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
	"github.com/cloudfoundry-incubator/garden-linux/linux_backend"
	"github.com/pivotal-golang/lager"
)

const NMHandler = "yarn-nide-manager"

type YarnBackend struct {
	Logger        lager.Logger
	RealBackend   garden.Backend
	yarnContainer *YarnContainer
}

func (s *YarnBackend) Start() error {
	s.Logger.Debug("Start")
	return s.RealBackend.Start()
}

func (s *YarnBackend) Stop() {
	s.Logger.Debug("Stop")
	s.RealBackend.Stop()
}

func (s *YarnBackend) GraceTime(c garden.Container) time.Duration {
	s.Logger.Debug("Greace Time")
	return s.RealBackend.GraceTime(c)
}

func (s *YarnBackend) Ping() error {
	s.Logger.Debug("Ping")
	return s.RealBackend.Ping()
}

func (s *YarnBackend) Capacity() (garden.Capacity, error) {
	s.Logger.Debug("capacity")
	return s.RealBackend.Capacity()
}

func isRootFsOurs(rootFs string) bool {
	return strings.HasPrefix(rootFs, "yarn://nodemanager") || (strings.Contains(rootFs, "nodemanagermagic"))
}

func (s *YarnBackend) Create(c garden.ContainerSpec) (garden.Container, error) {
	s.Logger.Debug("Create called", lager.Data{"RootFSPath": c.RootFSPath})
	if isRootFsOurs(c.RootFSPath) {
		if s.yarnContainer != nil {
			s.Logger.Debug("container NOT created")
			return nil, errors.New("Can't create more than one.")
		}

		s.Logger.Debug("container created")
		s.yarnContainer = &YarnContainer{Logger: s.Logger.Session("yarncontainer")}
		return s.yarnContainer, nil
	}

	return s.RealBackend.Create(c)
}

func (s *YarnBackend) Destroy(handle string) error {
	s.Logger.Debug("Destroy", lager.Data{"handle": handle})
	if handle == NMHandler {
		if s.yarnContainer == nil {
			s.Logger.Debug("Yaron container should not be nil here")
			return nil
		}
		s.yarnContainer.kill()
		s.yarnContainer = nil
	}
	return s.RealBackend.Destroy(handle)
}

func (s *YarnBackend) Containers(p garden.Properties) ([]garden.Container, error) {
	s.Logger.Debug("Containers with props", lager.Data{"props": p})

	ret, err := s.RealBackend.Containers(p)
	if err != nil {
		return ret, err
	}
	if len(p) == 0 && s.yarnContainer != nil {
		ret = append(ret, s.yarnContainer)
	}
	return ret, nil
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
	s.Logger.Debug("BulkInfo")
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
	s.Logger.Debug("BulkInfo ret", lager.Data{"ret": ret})

	return ret, nil
}

func (s *YarnBackend) BulkMetrics(handles []string) (map[string]garden.ContainerMetricsEntry, error) {
	s.Logger.Debug("BulkMetrics")

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
	s.Logger.Debug("BulkMetrics ret", lager.Data{"ret": ret})

	return ret, nil
}

func (s *YarnBackend) Lookup(handle string) (garden.Container, error) {
	s.Logger.Debug("Lookup", lager.Data{"handle": handle})

	if handle == NMHandler && s.yarnContainer != nil {
		s.Logger.Debug("Lookup returned yarn container")

		return s.yarnContainer, nil
	}
	return s.RealBackend.Lookup(handle)
}

var _ garden.Backend = &YarnBackend{}

type YarnContainer struct {
	Logger lager.Logger
	Props  garden.Properties

	limitsBandwith garden.BandwidthLimits
	limitsCpu      garden.CPULimits
	limitsDisk     garden.DiskLimits
	limitMemory    garden.MemoryLimits
	cmd            *exec.Cmd
}

// so that GraceTime that uses real backend will work
var _ linux_backend.Container = &YarnContainer{}

var _ garden.Container = &YarnContainer{}

func (c *YarnContainer) Handle() string {
	c.Logger.Debug("Handle")
	return NMHandler
}

func (c *YarnContainer) Stop(kill bool) error {

	c.Logger.Debug("Stop")
	return nil
}
func (c *YarnContainer) Info() (garden.ContainerInfo, error) {
	c.Logger.Debug("Info")
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
	c.Logger.Debug("kill!!")
	if c.cmd != nil && c.cmd.ProcessState != nil && !c.cmd.ProcessState.Exited() {
		c.cmd.Process.Kill()
		c.cmd.Process.Wait()
	}
}

func (c *YarnContainer) getState() string {
	c.Logger.Debug("getState")
	// Either "active" or "stopped".
	if c.cmd != nil && c.cmd.ProcessState != nil && !c.cmd.ProcessState.Exited() {
		return "active"
	}
	return "stopped"
}

func (c *YarnContainer) getProcessIds() []string {
	var ret []string
	c.Logger.Debug("getprocid")
	if c.cmd != nil && c.cmd.Process != nil && (!c.cmd.ProcessState.Exited()) {
		ret = append(ret, fmt.Sprintf("%d", c.cmd.Process.Pid))
	}

	c.Logger.Debug("getprocid", lager.Data{"pids": ret})
	return ret
}

func (c *YarnContainer) StreamIn(spec garden.StreamInSpec) error {
	c.Logger.Debug("Strean In error", lager.Data{"spec": spec})
	return errors.New("leave me alone")
}
func (c *YarnContainer) StreamOut(spec garden.StreamOutSpec) (io.ReadCloser, error) {
	c.Logger.Debug("StreamOut In error", lager.Data{"spec": spec})
	return nil, errors.New("leave me alone 2")
}
func (c *YarnContainer) LimitBandwidth(limits garden.BandwidthLimits) error {
	c.Logger.Debug("LimitBandwidth", lager.Data{"limits": limits})
	c.limitsBandwith = limits
	return nil
}
func (c *YarnContainer) CurrentBandwidthLimits() (garden.BandwidthLimits, error) {
	c.Logger.Debug("CurrentBandwidthLimits")
	return c.limitsBandwith, nil
}
func (c *YarnContainer) LimitCPU(limits garden.CPULimits) error {
	c.Logger.Debug("LimitCPU", lager.Data{"limits": limits})
	c.limitsCpu = limits
	return nil
}
func (c *YarnContainer) CurrentCPULimits() (garden.CPULimits, error) {
	c.Logger.Debug("CurrentCPULimits")
	return c.limitsCpu, nil
}
func (c *YarnContainer) CurrentDiskLimits() (garden.DiskLimits, error) {
	c.Logger.Debug("CurrentDiskLimits")
	return c.limitsDisk, nil
}
func (c *YarnContainer) LimitMemory(limits garden.MemoryLimits) error {
	c.Logger.Debug("LimitMemory", lager.Data{"limits": limits})
	c.limitMemory = limits
	return nil
}
func (c *YarnContainer) CurrentMemoryLimits() (garden.MemoryLimits, error) {
	c.Logger.Debug("CurrentMemoryLimits")
	return c.limitMemory, nil
}
func (c *YarnContainer) NetIn(hostPort, containerPort uint32) (uint32, uint32, error) {
	c.Logger.Debug("NetIn", lager.Data{"hostPort": hostPort, "containerPort": containerPort})
	return 0, 0, nil
}
func (c *YarnContainer) NetOut(netOutRule garden.NetOutRule) error {
	c.Logger.Debug("NetOut", lager.Data{"netOutRule": netOutRule})
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

func makeCmd(ps garden.ProcessSpec, pi garden.ProcessIO) *exec.Cmd {
	cmd := exec.Command(ps.Path, ps.Args...)

	cmd.Env = ps.Env
	cmd.Stdin = pi.Stdin
	cmd.Stdout = pi.Stdout
	cmd.Stderr = pi.Stderr
	cmd.Dir = ps.Dir

	return cmd
}

func (c *YarnContainer) Run(ps garden.ProcessSpec, pi garden.ProcessIO) (garden.Process, error) {
	c.Logger.Debug("Run")
	if c.cmd != nil {
		c.Logger.Debug("one process is enough")
		return nil, errors.New("one process is enough")
	}

	c.Logger.Debug("running bash")

	c.cmd = makeCmd(ps, pi)
	err := c.cmd.Start()
	if err != nil {
		c.Logger.Debug("start command error", lager.Data{"err": err})
		return nil, err
	}

	return (*GardenProcess)(c.cmd), nil
}

func (c *YarnContainer) Attach(processID string, io garden.ProcessIO) (garden.Process, error) {
	c.Logger.Debug("Attach")
	return nil, errors.New("can't attach")
}

func (c *YarnContainer) Metrics() (garden.Metrics, error) {
	c.Logger.Debug("Metrics")
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
	c.Logger.Debug("SetGreaceTime")
	// ignore!
	return nil
}
func (c *YarnContainer) Properties() (garden.Properties, error) {
	c.Logger.Debug("Properties")
	return c.Props, nil
}
func (c *YarnContainer) Property(name string) (string, error) {
	c.Logger.Debug("Property")
	return c.Props[name], nil
}
func (c *YarnContainer) SetProperty(name string, value string) error {
	c.Logger.Debug("SetProperty")
	return errors.New("set - props are read only. sorry!")
}
func (c *YarnContainer) RemoveProperty(name string) error {
	c.Logger.Debug("RemoveProperty")
	return errors.New("remove - props are read only. sorry!")
}

func (c *YarnContainer) ID() string {
	c.Logger.Debug("ID")
	return NMHandler
}
func (c *YarnContainer) HasProperties(props garden.Properties) bool {
	c.Logger.Debug("HasProperties", lager.Data{"props": props})
	return false
}
func (c *YarnContainer) GraceTime() time.Duration {
	c.Logger.Debug("GraceTime")
	return time.Duration(0)
}
func (c *YarnContainer) Start() error {
	c.Logger.Debug("Start")
	_, err := c.Run(garden.ProcessSpec{}, garden.ProcessIO{})
	return err
}
func (c *YarnContainer) Snapshot(io.Writer) error {
	c.Logger.Debug("Snapshot")
	return errors.New("no snapshots here!!")
}
func (c *YarnContainer) ResourceSpec() linux_backend.LinuxContainerSpec {
	c.Logger.Debug("ResourceSpec")
	return linux_backend.LinuxContainerSpec{ID: NMHandler, State: linux_backend.State(c.getState())}
}
func (c *YarnContainer) Restore(linux_backend.LinuxContainerSpec) error {
	c.Logger.Debug("Restore")
	return errors.New("no restore here!!")
}
func (c *YarnContainer) Cleanup() error {
	c.Logger.Debug("Cleanup")
	return nil
}
func (c *YarnContainer) LimitDisk(limits garden.DiskLimits) error {
	c.Logger.Debug("LimitDisk", lager.Data{"limits": limits})
	c.limitsDisk = limits
	return nil
}
