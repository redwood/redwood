// Code generated by mockery v2.13.1. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	process "redwood.dev/process"

	prototree "redwood.dev/swarm/prototree"

	swarm "redwood.dev/swarm"

	types "redwood.dev/types"
)

// TreeTransport is an autogenerated mock type for the TreeTransport type
type TreeTransport struct {
	mock.Mock
}

// AnnounceStateURIs provides a mock function with given fields: ctx, stateURIs
func (_m *TreeTransport) AnnounceStateURIs(ctx context.Context, stateURIs types.Set[string]) {
	_m.Called(ctx, stateURIs)
}

// Autoclose provides a mock function with given fields:
func (_m *TreeTransport) Autoclose() {
	_m.Called()
}

// AutocloseWithCleanup provides a mock function with given fields: closeFn
func (_m *TreeTransport) AutocloseWithCleanup(closeFn func()) {
	_m.Called(closeFn)
}

// Close provides a mock function with given fields:
func (_m *TreeTransport) Close() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Ctx provides a mock function with given fields:
func (_m *TreeTransport) Ctx() context.Context {
	ret := _m.Called()

	var r0 context.Context
	if rf, ok := ret.Get(0).(func() context.Context); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(context.Context)
		}
	}

	return r0
}

// Done provides a mock function with given fields:
func (_m *TreeTransport) Done() <-chan struct{} {
	ret := _m.Called()

	var r0 <-chan struct{}
	if rf, ok := ret.Get(0).(func() <-chan struct{}); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(<-chan struct{})
		}
	}

	return r0
}

// Go provides a mock function with given fields: ctx, name, fn
func (_m *TreeTransport) Go(ctx context.Context, name string, fn func(context.Context)) <-chan struct{} {
	ret := _m.Called(ctx, name, fn)

	var r0 <-chan struct{}
	if rf, ok := ret.Get(0).(func(context.Context, string, func(context.Context)) <-chan struct{}); ok {
		r0 = rf(ctx, name, fn)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(<-chan struct{})
		}
	}

	return r0
}

// Name provides a mock function with given fields:
func (_m *TreeTransport) Name() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// NewChild provides a mock function with given fields: ctx, name
func (_m *TreeTransport) NewChild(ctx context.Context, name string) *process.Process {
	ret := _m.Called(ctx, name)

	var r0 *process.Process
	if rf, ok := ret.Get(0).(func(context.Context, string) *process.Process); ok {
		r0 = rf(ctx, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*process.Process)
		}
	}

	return r0
}

// NewPeerConn provides a mock function with given fields: ctx, dialAddr
func (_m *TreeTransport) NewPeerConn(ctx context.Context, dialAddr string) (swarm.PeerConn, error) {
	ret := _m.Called(ctx, dialAddr)

	var r0 swarm.PeerConn
	if rf, ok := ret.Get(0).(func(context.Context, string) swarm.PeerConn); ok {
		r0 = rf(ctx, dialAddr)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(swarm.PeerConn)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, dialAddr)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// OnAckReceived provides a mock function with given fields: handler
func (_m *TreeTransport) OnAckReceived(handler prototree.AckReceivedCallback) {
	_m.Called(handler)
}

// OnP2PStateURIReceived provides a mock function with given fields: handler
func (_m *TreeTransport) OnP2PStateURIReceived(handler prototree.P2PStateURIReceivedCallback) {
	_m.Called(handler)
}

// OnPrivateTxReceived provides a mock function with given fields: handler
func (_m *TreeTransport) OnPrivateTxReceived(handler prototree.PrivateTxReceivedCallback) {
	_m.Called(handler)
}

// OnTxReceived provides a mock function with given fields: handler
func (_m *TreeTransport) OnTxReceived(handler prototree.TxReceivedCallback) {
	_m.Called(handler)
}

// OnWritableSubscriptionOpened provides a mock function with given fields: handler
func (_m *TreeTransport) OnWritableSubscriptionOpened(handler prototree.WritableSubscriptionOpenedCallback) {
	_m.Called(handler)
}

// ProcessTree provides a mock function with given fields:
func (_m *TreeTransport) ProcessTree() map[string]interface{} {
	ret := _m.Called()

	var r0 map[string]interface{}
	if rf, ok := ret.Get(0).(func() map[string]interface{}); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string]interface{})
		}
	}

	return r0
}

// ProvidersOfStateURI provides a mock function with given fields: ctx, stateURI
func (_m *TreeTransport) ProvidersOfStateURI(ctx context.Context, stateURI string) (<-chan prototree.TreePeerConn, error) {
	ret := _m.Called(ctx, stateURI)

	var r0 <-chan prototree.TreePeerConn
	if rf, ok := ret.Get(0).(func(context.Context, string) <-chan prototree.TreePeerConn); ok {
		r0 = rf(ctx, stateURI)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(<-chan prototree.TreePeerConn)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, stateURI)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SpawnChild provides a mock function with given fields: ctx, child
func (_m *TreeTransport) SpawnChild(ctx context.Context, child process.Spawnable) error {
	ret := _m.Called(ctx, child)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, process.Spawnable) error); ok {
		r0 = rf(ctx, child)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Start provides a mock function with given fields:
func (_m *TreeTransport) Start() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// State provides a mock function with given fields:
func (_m *TreeTransport) State() process.State {
	ret := _m.Called()

	var r0 process.State
	if rf, ok := ret.Get(0).(func() process.State); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(process.State)
	}

	return r0
}

type mockConstructorTestingTNewTreeTransport interface {
	mock.TestingT
	Cleanup(func())
}

// NewTreeTransport creates a new instance of TreeTransport. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewTreeTransport(t mockConstructorTestingTNewTreeTransport) *TreeTransport {
	mock := &TreeTransport{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
