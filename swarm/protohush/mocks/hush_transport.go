// Code generated by mockery v2.8.0. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	process "redwood.dev/process"

	protohush "redwood.dev/swarm/protohush"

	swarm "redwood.dev/swarm"
)

// HushTransport is an autogenerated mock type for the HushTransport type
type HushTransport struct {
	mock.Mock
}

// Autoclose provides a mock function with given fields:
func (_m *HushTransport) Autoclose() {
	_m.Called()
}

// AutocloseWithCleanup provides a mock function with given fields: closeFn
func (_m *HushTransport) AutocloseWithCleanup(closeFn func()) {
	_m.Called(closeFn)
}

// Close provides a mock function with given fields:
func (_m *HushTransport) Close() error {
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
func (_m *HushTransport) Ctx() context.Context {
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
func (_m *HushTransport) Done() <-chan struct{} {
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
func (_m *HushTransport) Go(ctx context.Context, name string, fn func(context.Context)) <-chan struct{} {
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
func (_m *HushTransport) Name() string {
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
func (_m *HushTransport) NewChild(ctx context.Context, name string) *process.Process {
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
func (_m *HushTransport) NewPeerConn(ctx context.Context, dialAddr string) (swarm.PeerConn, error) {
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

// OnIncomingDHPubkeyAttestations provides a mock function with given fields: handler
func (_m *HushTransport) OnIncomingDHPubkeyAttestations(handler protohush.IncomingDHPubkeyAttestationsCallback) {
	_m.Called(handler)
}

// OnIncomingGroupMessage provides a mock function with given fields: handler
func (_m *HushTransport) OnIncomingGroupMessage(handler protohush.IncomingGroupMessageCallback) {
	_m.Called(handler)
}

// OnIncomingIndividualMessage provides a mock function with given fields: handler
func (_m *HushTransport) OnIncomingIndividualMessage(handler protohush.IncomingIndividualMessageCallback) {
	_m.Called(handler)
}

// OnIncomingIndividualSessionProposal provides a mock function with given fields: handler
func (_m *HushTransport) OnIncomingIndividualSessionProposal(handler protohush.IncomingIndividualSessionProposalCallback) {
	_m.Called(handler)
}

// OnIncomingIndividualSessionResponse provides a mock function with given fields: handler
func (_m *HushTransport) OnIncomingIndividualSessionResponse(handler protohush.IncomingIndividualSessionResponseCallback) {
	_m.Called(handler)
}

// ProcessTree provides a mock function with given fields:
func (_m *HushTransport) ProcessTree() map[string]interface{} {
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

// SpawnChild provides a mock function with given fields: ctx, child
func (_m *HushTransport) SpawnChild(ctx context.Context, child process.Spawnable) error {
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
func (_m *HushTransport) Start() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
