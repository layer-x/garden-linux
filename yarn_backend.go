package main

import (
	"errors"
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
}

var _ garden.Backend = &YarnBackend{}

func NewYarnBackend(logger lager.Logger, realBackend garden.Backend) *YarnBackend {
	depot := "/tmp"
	houdiniBackend := houdini.NewBackend(depot)

	return &YarnBackend{Logger: logger, RealBackend: realBackend, houdiniBackend: houdiniBackend}
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
		return s.houdiniBackend.Create(c)
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

func (s *YarnBackend) Lookup(handle string) (garden.Container, error) {
	s.Logger.Debug("Lookup", lager.Data{"handle": handle})
	backend, err := s.GetBacekndForContainer(handle)
	if err != nil {
		return nil, err
	}
	return backend.Lookup(handle)
}
