// This file was generated by counterfeiter
package fake_pinger

import (
	"sync"

	"github.com/cloudfoundry-incubator/garden-linux/repository_fetcher"
	"github.com/docker/docker/registry"
)

type FakePinger struct {
	PingStub        func(*registry.Endpoint) (registry.RegistryInfo, error)
	pingMutex       sync.RWMutex
	pingArgsForCall []struct {
		arg1 *registry.Endpoint
	}
	pingReturns struct {
		result1 registry.RegistryInfo
		result2 error
	}
}

func (fake *FakePinger) Ping(arg1 *registry.Endpoint) (registry.RegistryInfo, error) {
	fake.pingMutex.Lock()
	fake.pingArgsForCall = append(fake.pingArgsForCall, struct {
		arg1 *registry.Endpoint
	}{arg1})
	fake.pingMutex.Unlock()
	if fake.PingStub != nil {
		return fake.PingStub(arg1)
	} else {
		return fake.pingReturns.result1, fake.pingReturns.result2
	}
}

func (fake *FakePinger) PingCallCount() int {
	fake.pingMutex.RLock()
	defer fake.pingMutex.RUnlock()
	return len(fake.pingArgsForCall)
}

func (fake *FakePinger) PingArgsForCall(i int) *registry.Endpoint {
	fake.pingMutex.RLock()
	defer fake.pingMutex.RUnlock()
	return fake.pingArgsForCall[i].arg1
}

func (fake *FakePinger) PingReturns(result1 registry.RegistryInfo, result2 error) {
	fake.PingStub = nil
	fake.pingReturns = struct {
		result1 registry.RegistryInfo
		result2 error
	}{result1, result2}
}

var _ repository_fetcher.Pinger = new(FakePinger)
