// This file was generated by counterfeiter
package fakes

import (
	"io"
	"sync"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden-linux/linux_backend"
)

type FakeResourcePool struct {
	SetupStub        func() error
	setupMutex       sync.RWMutex
	setupArgsForCall []struct{}
	setupReturns     struct {
		result1 error
	}
	AcquireStub        func(garden.ContainerSpec) (linux_backend.LinuxContainerSpec, error)
	acquireMutex       sync.RWMutex
	acquireArgsForCall []struct {
		arg1 garden.ContainerSpec
	}
	acquireReturns struct {
		result1 linux_backend.LinuxContainerSpec
		result2 error
	}
	RestoreStub        func(io.Reader) (linux_backend.LinuxContainerSpec, error)
	restoreMutex       sync.RWMutex
	restoreArgsForCall []struct {
		arg1 io.Reader
	}
	restoreReturns struct {
		result1 linux_backend.LinuxContainerSpec
		result2 error
	}
	ReleaseStub        func(linux_backend.LinuxContainerSpec) error
	releaseMutex       sync.RWMutex
	releaseArgsForCall []struct {
		arg1 linux_backend.LinuxContainerSpec
	}
	releaseReturns struct {
		result1 error
	}
	PruneStub        func(keep map[string]bool) error
	pruneMutex       sync.RWMutex
	pruneArgsForCall []struct {
		keep map[string]bool
	}
	pruneReturns struct {
		result1 error
	}
	MaxContainersStub        func() int
	maxContainersMutex       sync.RWMutex
	maxContainersArgsForCall []struct{}
	maxContainersReturns     struct {
		result1 int
	}
}

func (fake *FakeResourcePool) Setup() error {
	fake.setupMutex.Lock()
	fake.setupArgsForCall = append(fake.setupArgsForCall, struct{}{})
	fake.setupMutex.Unlock()
	if fake.SetupStub != nil {
		return fake.SetupStub()
	} else {
		return fake.setupReturns.result1
	}
}

func (fake *FakeResourcePool) SetupCallCount() int {
	fake.setupMutex.RLock()
	defer fake.setupMutex.RUnlock()
	return len(fake.setupArgsForCall)
}

func (fake *FakeResourcePool) SetupReturns(result1 error) {
	fake.SetupStub = nil
	fake.setupReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeResourcePool) Acquire(arg1 garden.ContainerSpec) (linux_backend.LinuxContainerSpec, error) {
	fake.acquireMutex.Lock()
	fake.acquireArgsForCall = append(fake.acquireArgsForCall, struct {
		arg1 garden.ContainerSpec
	}{arg1})
	fake.acquireMutex.Unlock()
	if fake.AcquireStub != nil {
		return fake.AcquireStub(arg1)
	} else {
		return fake.acquireReturns.result1, fake.acquireReturns.result2
	}
}

func (fake *FakeResourcePool) AcquireCallCount() int {
	fake.acquireMutex.RLock()
	defer fake.acquireMutex.RUnlock()
	return len(fake.acquireArgsForCall)
}

func (fake *FakeResourcePool) AcquireArgsForCall(i int) garden.ContainerSpec {
	fake.acquireMutex.RLock()
	defer fake.acquireMutex.RUnlock()
	return fake.acquireArgsForCall[i].arg1
}

func (fake *FakeResourcePool) AcquireReturns(result1 linux_backend.LinuxContainerSpec, result2 error) {
	fake.AcquireStub = nil
	fake.acquireReturns = struct {
		result1 linux_backend.LinuxContainerSpec
		result2 error
	}{result1, result2}
}

func (fake *FakeResourcePool) Restore(arg1 io.Reader) (linux_backend.LinuxContainerSpec, error) {
	fake.restoreMutex.Lock()
	fake.restoreArgsForCall = append(fake.restoreArgsForCall, struct {
		arg1 io.Reader
	}{arg1})
	fake.restoreMutex.Unlock()
	if fake.RestoreStub != nil {
		return fake.RestoreStub(arg1)
	} else {
		return fake.restoreReturns.result1, fake.restoreReturns.result2
	}
}

func (fake *FakeResourcePool) RestoreCallCount() int {
	fake.restoreMutex.RLock()
	defer fake.restoreMutex.RUnlock()
	return len(fake.restoreArgsForCall)
}

func (fake *FakeResourcePool) RestoreArgsForCall(i int) io.Reader {
	fake.restoreMutex.RLock()
	defer fake.restoreMutex.RUnlock()
	return fake.restoreArgsForCall[i].arg1
}

func (fake *FakeResourcePool) RestoreReturns(result1 linux_backend.LinuxContainerSpec, result2 error) {
	fake.RestoreStub = nil
	fake.restoreReturns = struct {
		result1 linux_backend.LinuxContainerSpec
		result2 error
	}{result1, result2}
}

func (fake *FakeResourcePool) Release(arg1 linux_backend.LinuxContainerSpec) error {
	fake.releaseMutex.Lock()
	fake.releaseArgsForCall = append(fake.releaseArgsForCall, struct {
		arg1 linux_backend.LinuxContainerSpec
	}{arg1})
	fake.releaseMutex.Unlock()
	if fake.ReleaseStub != nil {
		return fake.ReleaseStub(arg1)
	} else {
		return fake.releaseReturns.result1
	}
}

func (fake *FakeResourcePool) ReleaseCallCount() int {
	fake.releaseMutex.RLock()
	defer fake.releaseMutex.RUnlock()
	return len(fake.releaseArgsForCall)
}

func (fake *FakeResourcePool) ReleaseArgsForCall(i int) linux_backend.LinuxContainerSpec {
	fake.releaseMutex.RLock()
	defer fake.releaseMutex.RUnlock()
	return fake.releaseArgsForCall[i].arg1
}

func (fake *FakeResourcePool) ReleaseReturns(result1 error) {
	fake.ReleaseStub = nil
	fake.releaseReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeResourcePool) Prune(keep map[string]bool) error {
	fake.pruneMutex.Lock()
	fake.pruneArgsForCall = append(fake.pruneArgsForCall, struct {
		keep map[string]bool
	}{keep})
	fake.pruneMutex.Unlock()
	if fake.PruneStub != nil {
		return fake.PruneStub(keep)
	} else {
		return fake.pruneReturns.result1
	}
}

func (fake *FakeResourcePool) PruneCallCount() int {
	fake.pruneMutex.RLock()
	defer fake.pruneMutex.RUnlock()
	return len(fake.pruneArgsForCall)
}

func (fake *FakeResourcePool) PruneArgsForCall(i int) map[string]bool {
	fake.pruneMutex.RLock()
	defer fake.pruneMutex.RUnlock()
	return fake.pruneArgsForCall[i].keep
}

func (fake *FakeResourcePool) PruneReturns(result1 error) {
	fake.PruneStub = nil
	fake.pruneReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeResourcePool) MaxContainers() int {
	fake.maxContainersMutex.Lock()
	fake.maxContainersArgsForCall = append(fake.maxContainersArgsForCall, struct{}{})
	fake.maxContainersMutex.Unlock()
	if fake.MaxContainersStub != nil {
		return fake.MaxContainersStub()
	} else {
		return fake.maxContainersReturns.result1
	}
}

func (fake *FakeResourcePool) MaxContainersCallCount() int {
	fake.maxContainersMutex.RLock()
	defer fake.maxContainersMutex.RUnlock()
	return len(fake.maxContainersArgsForCall)
}

func (fake *FakeResourcePool) MaxContainersReturns(result1 int) {
	fake.MaxContainersStub = nil
	fake.maxContainersReturns = struct {
		result1 int
	}{result1}
}

var _ linux_backend.ResourcePool = new(FakeResourcePool)
