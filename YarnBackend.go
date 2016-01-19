package main

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/cloudfoundry-incubator/garden"
)

const NMHandler = "yarn-nide-manager"

type YarnBackend struct {
	RealBackend   garden.Backend
	yarnContainer *YarnContainer
}

func (s *YarnBackend) Start() error {
	return s.RealBackend.Start()
}

func (s *YarnBackend) Stop() {
	s.RealBackend.Stop()
}

func (s *YarnBackend) GraceTime(c garden.Container) time.Duration {
	return s.RealBackend.GraceTime(c)
}

func (s *YarnBackend) Ping() error {
	return s.RealBackend.Ping()
}

func (s *YarnBackend) Capacity() (garden.Capacity, error) {
	return s.RealBackend.Capacity()
}

func (s *YarnBackend) Create(c garden.ContainerSpec) (garden.Container, error) {
	if strings.HasPrefix(c.RootFSPath, "yarn://nodemanager") {
		s.yarnContainer = &YarnContainer{}
		return s.yarnContainer, nil
	}

	return s.RealBackend.Create(c)
}

func (s *YarnBackend) Destroy(handle string) error {
	if handle == NMHandler {
		s.yarnContainer.kill()
		s.yarnContainer = nil
	}
	return s.RealBackend.Destroy(handle)
}

func (s *YarnBackend) Containers(p garden.Properties) ([]garden.Container, error) {
	return s.RealBackend.Containers(p)
}

func removeHandle(handles []string) ([]string, bool) {
	for i, handle := range handles {
		if handle == NMHandler {
			handles[i] = handles[len(handles)-1]
			handles[len(handles)-1] = ""
			handles = handles[:len(handles)-1]
			return handles, true
		}

	}
	return handles, false
}

func (s *YarnBackend) BulkInfo(handles []string) (map[string]garden.ContainerInfoEntry, error) {
	handles, r := removeHandle(handles)

	ret, err := s.RealBackend.BulkInfo(handles)
	if err != nil {
		return ret, err
	}

	if r {
		info, err := s.yarnContainer.Info()
		var entry garden.ContainerInfoEntry
		entry.Info = info
		if err != nil {
			entry.Err = &garden.Error{err}
		}

		ret[NMHandler] = entry
	}

	return ret, nil
}

func (s *YarnBackend) BulkMetrics(handles []string) (map[string]garden.ContainerMetricsEntry, error) {
	handles, r := removeHandle(handles)

	ret, err := s.RealBackend.BulkMetrics(handles)
	if err != nil {
		return ret, err
	}

	if r {
		metrics, err := s.yarnContainer.Metrics()
		var entry garden.ContainerMetricsEntry
		entry.Metrics = metrics
		if err != nil {
			entry.Err = &garden.Error{err}
		}

		ret[NMHandler] = entry
	}

	return ret, nil
}

func (s *YarnBackend) Lookup(handle string) (garden.Container, error) {
	if handle == NMHandler {
		return s.yarnContainer, nil
	}
	return s.RealBackend.Lookup(handle)
}

var _ garden.Backend = &YarnBackend{}

type YarnContainer struct {
	Props garden.Properties

	limitsBandwith garden.BandwidthLimits
	limitsCpu      garden.CPULimits
	limitsDisk     garden.DiskLimits
	limitMemory    garden.MemoryLimits
	cmd            *exec.Cmd
}

var _ garden.Container = &YarnContainer{}

func (c *YarnContainer) Handle() string {
	return NMHandler
}

func (c *YarnContainer) Stop(kill bool) error {
	return nil
}
func (c *YarnContainer) Info() (garden.ContainerInfo, error) {
	return garden.ContainerInfo{
		State:         c.getState(),           // Either "active" or "stopped".
		Events:        []string{},             // List of events that occurred for the container. It currently includes only "oom" (Out Of Memory) event if it occurred.
		HostIP:        "",                     // The IP address of the gateway which controls the host side of the container's virtual ethernet pair.
		ContainerIP:   "",                     // The IP address of the container side of the container's virtual ethernet pair.
		ExternalIP:    "",                     //
		ContainerPath: "",                     // The path to the directory holding the container's files (both its control scripts and filesystem).
		ProcessIDs:    []string{},             // List of running processes.
		Properties:    c.Props,                // List of properties defined for the container.
		MappedPorts:   []garden.PortMapping{}, //
	}, nil

}

func (c *YarnContainer) kill() {
	if c.cmd != nil && c.cmd.ProcessState != nil && !c.cmd.ProcessState.Exited() {
		c.cmd.Process.Kill()
		c.cmd.Process.Release()
	}
}

func (c *YarnContainer) getState() string {
	// Either "active" or "stopped".
	if c.cmd != nil && c.cmd.ProcessState != nil && !c.cmd.ProcessState.Exited() {
		return "active"
	}
	return "stopped"
}

func (c *YarnContainer) getProcessIds() []string {
	// Either "active" or "stopped".
	return nil
}

func (c *YarnContainer) StreamIn(spec garden.StreamInSpec) error {
	return errors.New("leave me alone")
}
func (c *YarnContainer) StreamOut(spec garden.StreamOutSpec) (io.ReadCloser, error) {
	return nil, errors.New("leave me alone 2")
}
func (c *YarnContainer) LimitBandwidth(limits garden.BandwidthLimits) error {
	c.limitsBandwith = limits
	return nil
}
func (c *YarnContainer) CurrentBandwidthLimits() (garden.BandwidthLimits, error) {
	return c.limitsBandwith, nil
}
func (c *YarnContainer) LimitCPU(limits garden.CPULimits) error {
	c.limitsCpu = limits
	return nil
}
func (c *YarnContainer) CurrentCPULimits() (garden.CPULimits, error) {
	return c.limitsCpu, nil
}
func (c *YarnContainer) CurrentDiskLimits() (garden.DiskLimits, error) {
	return c.limitsDisk, nil
}
func (c *YarnContainer) LimitMemory(limits garden.MemoryLimits) error {
	c.limitMemory = limits
	return nil
}
func (c *YarnContainer) CurrentMemoryLimits() (garden.MemoryLimits, error) {
	return c.limitMemory, nil
}
func (c *YarnContainer) NetIn(hostPort, containerPort uint32) (uint32, uint32, error) {
	return 0, 0, nil
}
func (c *YarnContainer) NetOut(netOutRule garden.NetOutRule) error {
	return nil
}

type GardenProcess exec.Cmd

func (p *GardenProcess) ID() string {
	return fmt.Sprintf("%d", p.Process.Pid)
}

func (p *GardenProcess) Wait() (int, error) {

	if err := (*exec.Cmd)(p).Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			// http://stackoverflow.com/questions/10385551/get-exit-code-go
			// The program has exited with an exit code != 0

			// This works on both Unix and Windows. Although package
			// syscall is generally platform dependent, WaitStatus is
			// defined for both Unix and Windows and in both cases has
			// an ExitStatus() method with the same signature.
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				return status.ExitStatus(), nil
			}
		} else {
			return 0, err
		}
	}

	return 0, nil

}

func (p *GardenProcess) SetTTY(t garden.TTYSpec) error {
	return nil
}

func (p *GardenProcess) Signal(s garden.Signal) error {
	return (*exec.Cmd)(p).Process.Signal(syscall.Signal(int(s)))
}

func (c *YarnContainer) Run(garden.ProcessSpec, garden.ProcessIO) (garden.Process, error) {
	if c.cmd != nil {
		return nil, errors.New("one process is enough")
	}

	c.cmd = exec.Command("bash", "-c", "sleep 12345")
	err := c.cmd.Start()
	if err != nil {
		return nil, err
	}

	return (*GardenProcess)(c.cmd), nil
}
func (c *YarnContainer) Attach(processID string, io garden.ProcessIO) (garden.Process, error) {
	return nil, errors.New("can't attach")
}
func (c *YarnContainer) Metrics() (garden.Metrics, error) {
	m := garden.Metrics{}
	// make up lies:
	m.MemoryStat.Rss = 1
	m.CPUStat.Usage = 1
	m.CPUStat.User = 1
	m.CPUStat.System = 1
	m.DiskStat.TotalBytesUsed = 1
	m.DiskStat.TotalInodesUsed = 1
	m.DiskStat.ExclusiveBytesUsed = 1
	m.DiskStat.ExclusiveInodesUsed = 1

	m.NetworkStat.RxBytes = 1
	m.NetworkStat.TxBytes = 1

	return m, nil
}
func (c *YarnContainer) SetGraceTime(graceTime time.Duration) error {
	// ignore!
	return nil
}
func (c *YarnContainer) Properties() (garden.Properties, error) {
	return c.Props, nil
}
func (c *YarnContainer) Property(name string) (string, error) {
	return c.Props[name], nil
}
func (c *YarnContainer) SetProperty(name string, value string) error {
	return errors.New("set - props are read only. sorry!")
}
func (c *YarnContainer) RemoveProperty(name string) error {
	return errors.New("remove - props are read only. sorry!")
}
