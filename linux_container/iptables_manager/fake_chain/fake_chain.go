// This file was generated by counterfeiter
package fake_chain

import (
	"net"
	"sync"

	"github.com/cloudfoundry-incubator/garden-linux/linux_container/iptables_manager"
)

type FakeChain struct {
	SetupStub        func(containerID, bridgeName string, ip net.IP, network *net.IPNet) error
	setupMutex       sync.RWMutex
	setupArgsForCall []struct {
		containerID string
		bridgeName  string
		ip          net.IP
		network     *net.IPNet
	}
	setupReturns struct {
		result1 error
	}
	TeardownStub        func(containerID string) error
	teardownMutex       sync.RWMutex
	teardownArgsForCall []struct {
		containerID string
	}
	teardownReturns struct {
		result1 error
	}
}

func (fake *FakeChain) Setup(containerID string, bridgeName string, ip net.IP, network *net.IPNet) error {
	fake.setupMutex.Lock()
	fake.setupArgsForCall = append(fake.setupArgsForCall, struct {
		containerID string
		bridgeName  string
		ip          net.IP
		network     *net.IPNet
	}{containerID, bridgeName, ip, network})
	fake.setupMutex.Unlock()
	if fake.SetupStub != nil {
		return fake.SetupStub(containerID, bridgeName, ip, network)
	} else {
		return fake.setupReturns.result1
	}
}

func (fake *FakeChain) SetupCallCount() int {
	fake.setupMutex.RLock()
	defer fake.setupMutex.RUnlock()
	return len(fake.setupArgsForCall)
}

func (fake *FakeChain) SetupArgsForCall(i int) (string, string, net.IP, *net.IPNet) {
	fake.setupMutex.RLock()
	defer fake.setupMutex.RUnlock()
	return fake.setupArgsForCall[i].containerID, fake.setupArgsForCall[i].bridgeName, fake.setupArgsForCall[i].ip, fake.setupArgsForCall[i].network
}

func (fake *FakeChain) SetupReturns(result1 error) {
	fake.SetupStub = nil
	fake.setupReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeChain) Teardown(containerID string) error {
	fake.teardownMutex.Lock()
	fake.teardownArgsForCall = append(fake.teardownArgsForCall, struct {
		containerID string
	}{containerID})
	fake.teardownMutex.Unlock()
	if fake.TeardownStub != nil {
		return fake.TeardownStub(containerID)
	} else {
		return fake.teardownReturns.result1
	}
}

func (fake *FakeChain) TeardownCallCount() int {
	fake.teardownMutex.RLock()
	defer fake.teardownMutex.RUnlock()
	return len(fake.teardownArgsForCall)
}

func (fake *FakeChain) TeardownArgsForCall(i int) string {
	fake.teardownMutex.RLock()
	defer fake.teardownMutex.RUnlock()
	return fake.teardownArgsForCall[i].containerID
}

func (fake *FakeChain) TeardownReturns(result1 error) {
	fake.TeardownStub = nil
	fake.teardownReturns = struct {
		result1 error
	}{result1}
}

var _ iptables_manager.Chain = new(FakeChain)