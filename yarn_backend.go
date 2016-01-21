package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/pivotal-golang/lager"

	"github.com/vito/houdini"
)

const NMHandler = "yarn-nide-manager"

type YarnBackend struct {
	Logger         lager.Logger
	RealBackend    garden.Backend
	houdiniBackend garden.Backend
	depot          string
}

var _ garden.Backend = &YarnBackend{}

func NewYarnBackend(logger lager.Logger, realBackend garden.Backend) *YarnBackend {
	depot := "/tmp"
	houdiniBackend := houdini.NewBackend(depot)

	return &YarnBackend{Logger: logger, RealBackend: realBackend, houdiniBackend: houdiniBackend, depot: depot}
}

func (s *YarnBackend) Start() error {
	s.Logger.Debug("Start")
	err := s.RealBackend.Start()
	if err != nil {
		return err
	}

	err = s.houdiniBackend.Start()
	if err != nil {
		s.RealBackend.Stop()
		return err
	}
	return nil
}

func (s *YarnBackend) Stop() {
	s.Logger.Debug("Stop")
	s.RealBackend.Stop()
	s.houdiniBackend.Start()
}

func (s *YarnBackend) GetBacekndForContainer(handle string) (garden.Backend, error) {
	if _, err := s.RealBackend.Lookup(handle); err == nil {
		return s.RealBackend, nil
	}
	if _, err := s.houdiniBackend.Lookup(handle); err == nil {
		return s.houdiniBackend, nil
	}

	return nil, errors.New("no backend for container")
}

func (s *YarnBackend) GraceTime(c garden.Container) time.Duration {
	s.Logger.Debug("Greace Time")
	backend, err := s.GetBacekndForContainer(c.Handle())
	if err != nil {
		return 0
	}
	return backend.GraceTime(c)
}

func (s *YarnBackend) Ping() error {
	s.Logger.Debug("Ping")

	if err := s.RealBackend.Ping(); err != nil {
		return err
	}

	return s.houdiniBackend.Ping()

}

func (s *YarnBackend) Capacity() (garden.Capacity, error) {
	s.Logger.Debug("capacity")
	return s.RealBackend.Capacity()
}

func isOurContainer(c garden.ContainerSpec) bool {

	return strings.HasPrefix(c.RootFSPath, "yarn://nodemanager") || (strings.Contains(c.RootFSPath, "nodemanagermagic"))
}

func (s *YarnBackend) Create(c garden.ContainerSpec) (garden.Container, error) {
	s.Logger.Debug("Create called", lager.Data{"RootFSPath": c.RootFSPath})
	if isOurContainer(c) {
		cnt, err := s.houdiniBackend.Create(c)
		if err != nil {
			return cnt, err
		}
		// fragile!! copied from hodini..
		cntdir := filepath.Join(s.depot, cnt.Handle())
		// make the tmp dir for diego
		os.MkdirAll(filepath.Join(cntdir, "tmp"), 0755)

		err = s.HandleBindMounts(c, cntdir)
		return cnt, err
	}

	return s.RealBackend.Create(c)
}

func (s *YarnBackend) Destroy(handle string) error {
	s.Logger.Debug("Destroy", lager.Data{"handle": handle})
	backend, err := s.GetBacekndForContainer(handle)
	if err != nil {
		return err
	}
	return backend.Destroy(handle)

}

func (s *YarnBackend) BulkInfo(handles []string) (map[string]garden.ContainerInfoEntry, error) {
	s.Logger.Debug("BulkInfo")

	// fragile!!!!
	// after checking source code:
	// garden impl ignores handles it doesnlt know
	// and houdini impl is trivial.
	// so this works:

	return s.RealBackend.BulkInfo(handles)
}

func (s *YarnBackend) BulkMetrics(handles []string) (map[string]garden.ContainerMetricsEntry, error) {
	s.Logger.Debug("BulkInfo")
	// see comment in BulkMetrics
	return s.RealBackend.BulkMetrics(handles)

}

func (s *YarnBackend) Containers(p garden.Properties) ([]garden.Container, error) {
	s.Logger.Debug("Containers with props", lager.Data{"props": p})

	ret, err := s.RealBackend.Containers(p)
	if err != nil {
		return ret, err
	}
	ret2, err := s.houdiniBackend.Containers(p)
	if err != nil {
		return ret2, err
	}

	return append(ret, ret2...), nil
}

func (s *YarnBackend) Lookup(handle string) (garden.Container, error) {
	s.Logger.Debug("Lookup", lager.Data{"handle": handle})
	backend, err := s.GetBacekndForContainer(handle)
	if err != nil {
		return nil, err
	}
	cnt, err := backend.Lookup(handle)
	if err != nil {
		return cnt, err
	}

	if s.houdiniBackend == backend {
		// fragile!! copied from hodini..
		cntdir := filepath.Join(s.depot, cnt.Handle())

		return &wrapperContainer{cnt, cntdir}, nil
	}

	return cnt, nil
}

type wrapperContainer struct {
	c      garden.Container
	cntdir string
}

func (w *wrapperContainer) Handle() string {
	return w.c.Handle()
}
func (w *wrapperContainer) Stop(kill bool) error {
	return w.c.Stop(kill)
}
func (w *wrapperContainer) Info() (garden.ContainerInfo, error) {
	return w.c.Info()
}
func (w *wrapperContainer) StreamIn(spec garden.StreamInSpec) error {
	return w.c.StreamIn(spec)
}
func (w *wrapperContainer) StreamOut(spec garden.StreamOutSpec) (io.ReadCloser, error) {
	return w.c.StreamOut(spec)
}
func (w *wrapperContainer) LimitBandwidth(limits garden.BandwidthLimits) error {
	return w.c.LimitBandwidth(limits)
}
func (w *wrapperContainer) CurrentBandwidthLimits() (garden.BandwidthLimits, error) {
	return w.c.CurrentBandwidthLimits()
}
func (w *wrapperContainer) LimitCPU(limits garden.CPULimits) error {
	return w.c.LimitCPU(limits)
}
func (w *wrapperContainer) CurrentCPULimits() (garden.CPULimits, error) {
	return w.c.CurrentCPULimits()
}
func (w *wrapperContainer) CurrentDiskLimits() (garden.DiskLimits, error) {
	return w.c.CurrentDiskLimits()
}
func (w *wrapperContainer) LimitMemory(limits garden.MemoryLimits) error {
	return w.c.LimitMemory(limits)
}
func (w *wrapperContainer) CurrentMemoryLimits() (garden.MemoryLimits, error) {
	return w.c.CurrentMemoryLimits()
}
func (w *wrapperContainer) NetIn(hostPort, containerPort uint32) (uint32, uint32, error) {
	return w.c.NetIn(hostPort, containerPort)
}
func (w *wrapperContainer) NetOut(netOutRule garden.NetOutRule) error {
	return w.c.NetOut(netOutRule)
}
func (w *wrapperContainer) Run(p1 garden.ProcessSpec, p2 garden.ProcessIO) (garden.Process, error) {
	if strings.HasPrefix(p1.Path, "/tmp") {
		p1.Path = filepath.Join(w.cntdir, p1.Path)
	}
	return w.c.Run(p1, p2)
}
func (w *wrapperContainer) Attach(processID string, io garden.ProcessIO) (garden.Process, error) {
	return w.c.Attach(processID, io)
}
func (w *wrapperContainer) Metrics() (garden.Metrics, error) {
	return w.c.Metrics()
}
func (w *wrapperContainer) SetGraceTime(graceTime time.Duration) error {
	return w.c.SetGraceTime(graceTime)
}
func (w *wrapperContainer) Properties() (garden.Properties, error) {
	return w.c.Properties()
}
func (w *wrapperContainer) Property(name string) (string, error) {
	return w.c.Property(name)
}
func (w *wrapperContainer) SetProperty(name string, value string) error {
	return w.c.SetProperty(name, value)
}
func (w *wrapperContainer) RemoveProperty(name string) error {
	return w.c.RemoveProperty(name)
}
