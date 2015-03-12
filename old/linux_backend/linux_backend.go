package linux_backend

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden-linux/old/system_info"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Container
type Container interface {
	ID() string
	Properties() garden.Properties
	GraceTime() time.Duration

	Start() error

	Snapshot(io.Writer) error
	Cleanup()

	garden.Container
}

type ContainerPool interface {
	Setup() error
	Create(garden.ContainerSpec) (Container, error)
	Restore(io.Reader) (Container, error)
	Destroy(Container) error
	Prune(keep map[string]bool) error
	MaxContainers() int
}

//go:generate counterfeiter . ContainerRepository
type ContainerRepository interface {
	All() []Container
	Add(Container)
	FindByHandle(string) (Container, bool)
	Delete(Container)
}

type LinuxBackend struct {
	logger lager.Logger

	containerPool ContainerPool
	systemInfo    system_info.Provider
	snapshotsPath string

	containerRepo ContainerRepository
}

type HandleExistsError struct {
	Handle string
}

func (e HandleExistsError) Error() string {
	return fmt.Sprintf("handle already exists: %s", e.Handle)
}

type FailedToSnapshotError struct {
	OriginalError error
}

func (e FailedToSnapshotError) Error() string {
	return fmt.Sprintf("failed to save snapshot: %s", e.OriginalError)
}

func New(
	logger lager.Logger,
	containerPool ContainerPool,
	containerRepo ContainerRepository,
	systemInfo system_info.Provider,
	snapshotsPath string,
) *LinuxBackend {
	return &LinuxBackend{
		logger: logger.Session("backend"),

		containerPool: containerPool,
		systemInfo:    systemInfo,
		snapshotsPath: snapshotsPath,

		containerRepo: containerRepo,
	}
}

func (b *LinuxBackend) Setup() error {
	return b.containerPool.Setup()
}

func (b *LinuxBackend) Start() error {
	if b.snapshotsPath != "" {
		_, err := os.Stat(b.snapshotsPath)
		if err == nil {
			b.restoreSnapshots()
			os.RemoveAll(b.snapshotsPath)
		}

		err = os.MkdirAll(b.snapshotsPath, 0755)
		if err != nil {
			return err
		}
	}

	keep := map[string]bool{}

	containers := b.containerRepo.All()

	for _, container := range containers {
		keep[container.ID()] = true
	}

	return b.containerPool.Prune(keep)
}

func (b *LinuxBackend) Ping() error {
	return nil
}

func (b *LinuxBackend) Capacity() (garden.Capacity, error) {
	totalMemory, err := b.systemInfo.TotalMemory()
	if err != nil {
		return garden.Capacity{}, err
	}

	totalDisk, err := b.systemInfo.TotalDisk()
	if err != nil {
		return garden.Capacity{}, err
	}

	return garden.Capacity{
		MemoryInBytes: totalMemory,
		DiskInBytes:   totalDisk,
		MaxContainers: uint64(b.containerPool.MaxContainers()),
	}, nil
}

func (b *LinuxBackend) Create(spec garden.ContainerSpec) (garden.Container, error) {
	if spec.Handle != "" {
		_, exists := b.containerRepo.FindByHandle(spec.Handle)

		if exists {
			return nil, HandleExistsError{Handle: spec.Handle}
		}
	}

	container, err := b.containerPool.Create(spec)
	if err != nil {
		return nil, err
	}

	err = container.Start()
	if err != nil {
		return nil, err
	}

	b.containerRepo.Add(container)

	return container, nil
}

func (b *LinuxBackend) Destroy(handle string) error {
	container, found := b.containerRepo.FindByHandle(handle)

	if !found {
		return garden.ContainerNotFoundError{handle}
	}

	err := b.containerPool.Destroy(container)
	if err != nil {
		return err
	}

	b.containerRepo.Delete(container)

	return nil
}

func (b *LinuxBackend) Containers(filter garden.Properties) (containers []garden.Container, err error) {
	for _, container := range b.containerRepo.All() {
		if containerHasProperties(container, filter) {
			containers = append(containers, container)
		}
	}

	return containers, nil
}

func (b *LinuxBackend) Lookup(handle string) (garden.Container, error) {
	container, found := b.containerRepo.FindByHandle(handle)
	if !found {
		return nil, garden.ContainerNotFoundError{handle}
	}

	return container, nil
}

func (b *LinuxBackend) BulkInfo(handles []string) (map[string]garden.ContainerInfoEntry, error) {
	containers := b.containerRepo.All()
	infos := make(map[string]garden.ContainerInfoEntry)
	for _, container := range containers {
		if contains(handles, container.Handle()) {
			info, err := container.Info()
			infos[container.Handle()] = garden.ContainerInfoEntry{
				Info: info,
				Err:  err,
			}
		}
	}
	return infos, nil
}

func contains(handles []string, handle string) bool {
	for _, e := range handles {
		if e == handle {
			return true
		}
	}
	return false
}

func (b *LinuxBackend) GraceTime(container garden.Container) time.Duration {
	return container.(Container).GraceTime()
}

func (b *LinuxBackend) Stop() {
	for _, container := range b.containerRepo.All() {
		container.Cleanup()
		err := b.saveSnapshot(container)
		if err != nil {
			b.logger.Error("failed-to-save-snapshot", err, lager.Data{
				"container": container.ID(),
			})
		}
	}
}

func (b *LinuxBackend) restoreSnapshots() {
	sLog := b.logger.Session("restore")

	entries, err := ioutil.ReadDir(b.snapshotsPath)
	if err != nil {
		b.logger.Error("failed-to-read-snapshots", err, lager.Data{
			"from": b.snapshotsPath,
		})
	}

	for _, entry := range entries {
		snapshot := path.Join(b.snapshotsPath, entry.Name())

		lLog := sLog.Session("load", lager.Data{
			"snapshot": entry.Name(),
		})

		lLog.Debug("loading")

		file, err := os.Open(snapshot)
		if err != nil {
			lLog.Error("failed-to-open", err)
		}

		_, err = b.restore(file)
		if err != nil {
			lLog.Error("failed-to-restore", err)
		}
	}
}

func (b *LinuxBackend) saveSnapshot(container Container) error {
	if b.snapshotsPath == "" {
		return nil
	}

	b.logger.Info("save-snapshot", lager.Data{
		"container": container.ID(),
	})

	snapshotPath := path.Join(b.snapshotsPath, container.ID())

	snapshot, err := os.Create(snapshotPath)
	if err != nil {
		return &FailedToSnapshotError{err}
	}

	err = container.Snapshot(snapshot)
	if err != nil {
		return &FailedToSnapshotError{err}
	}

	return snapshot.Close()
}

func (b *LinuxBackend) restore(snapshot io.Reader) (garden.Container, error) {
	container, err := b.containerPool.Restore(snapshot)
	if err != nil {
		return nil, err
	}
	b.containerRepo.Add(container)
	return container, nil
}

func containerHasProperties(container Container, properties garden.Properties) bool {
	containerProps := container.Properties()

	for key, val := range properties {
		cval, ok := containerProps[key]
		if !ok {
			return false
		}

		if cval != val {
			return false
		}
	}

	return true
}
