// This file was generated by counterfeiter
package fakes

import (
	"sync"

	"github.com/cloudfoundry-incubator/garden-linux/linux_backend"
)

type FakeHealthChecker struct {
	HealthCheckStub        func() error
	healthCheckMutex       sync.RWMutex
	healthCheckArgsForCall []struct{}
	healthCheckReturns     struct {
		result1 error
	}
}

func (fake *FakeHealthChecker) HealthCheck() error {
	fake.healthCheckMutex.Lock()
	fake.healthCheckArgsForCall = append(fake.healthCheckArgsForCall, struct{}{})
	fake.healthCheckMutex.Unlock()
	if fake.HealthCheckStub != nil {
		return fake.HealthCheckStub()
	} else {
		return fake.healthCheckReturns.result1
	}
}

func (fake *FakeHealthChecker) HealthCheckCallCount() int {
	fake.healthCheckMutex.RLock()
	defer fake.healthCheckMutex.RUnlock()
	return len(fake.healthCheckArgsForCall)
}

func (fake *FakeHealthChecker) HealthCheckReturns(result1 error) {
	fake.HealthCheckStub = nil
	fake.healthCheckReturns = struct {
		result1 error
	}{result1}
}

var _ linux_backend.HealthChecker = new(FakeHealthChecker)
