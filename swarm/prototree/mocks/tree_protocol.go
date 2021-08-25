// Code generated by mockery v2.8.0. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	process "redwood.dev/process"

	prototree "redwood.dev/swarm/prototree"

	state "redwood.dev/state"

	tree "redwood.dev/tree"
)

// TreeProtocol is an autogenerated mock type for the TreeProtocol type
type TreeProtocol struct {
	mock.Mock
}

// Autoclose provides a mock function with given fields:
func (_m *TreeProtocol) Autoclose() {
	_m.Called()
}

// Close provides a mock function with given fields:
func (_m *TreeProtocol) Close() error {
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
func (_m *TreeProtocol) Ctx() context.Context {
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
func (_m *TreeProtocol) Done() <-chan struct{} {
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

// Go provides a mock function with given fields: name, fn
func (_m *TreeProtocol) Go(name string, fn func(context.Context)) <-chan struct{} {
	ret := _m.Called(name, fn)

	var r0 <-chan struct{}
	if rf, ok := ret.Get(0).(func(string, func(context.Context)) <-chan struct{}); ok {
		r0 = rf(name, fn)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(<-chan struct{})
		}
	}

	return r0
}

// Name provides a mock function with given fields:
func (_m *TreeProtocol) Name() string {
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
func (_m *TreeProtocol) NewChild(ctx context.Context, name string) *process.Process {
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

// ProcessTree provides a mock function with given fields:
func (_m *TreeProtocol) ProcessTree() map[string]interface{} {
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
func (_m *TreeProtocol) ProvidersOfStateURI(ctx context.Context, stateURI string) <-chan prototree.TreePeerConn {
	ret := _m.Called(ctx, stateURI)

	var r0 <-chan prototree.TreePeerConn
	if rf, ok := ret.Get(0).(func(context.Context, string) <-chan prototree.TreePeerConn); ok {
		r0 = rf(ctx, stateURI)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(<-chan prototree.TreePeerConn)
		}
	}

	return r0
}

// SendTx provides a mock function with given fields: ctx, tx
func (_m *TreeProtocol) SendTx(ctx context.Context, tx tree.Tx) error {
	ret := _m.Called(ctx, tx)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, tree.Tx) error); ok {
		r0 = rf(ctx, tx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SpawnChild provides a mock function with given fields: ctx, child
func (_m *TreeProtocol) SpawnChild(ctx context.Context, child process.Spawnable) error {
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
func (_m *TreeProtocol) Start() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Subscribe provides a mock function with given fields: ctx, stateURI, subscriptionType, keypath, fetchHistoryOpts
func (_m *TreeProtocol) Subscribe(ctx context.Context, stateURI string, subscriptionType prototree.SubscriptionType, keypath state.Keypath, fetchHistoryOpts *prototree.FetchHistoryOpts) (prototree.ReadableSubscription, error) {
	ret := _m.Called(ctx, stateURI, subscriptionType, keypath, fetchHistoryOpts)

	var r0 prototree.ReadableSubscription
	if rf, ok := ret.Get(0).(func(context.Context, string, prototree.SubscriptionType, state.Keypath, *prototree.FetchHistoryOpts) prototree.ReadableSubscription); ok {
		r0 = rf(ctx, stateURI, subscriptionType, keypath, fetchHistoryOpts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(prototree.ReadableSubscription)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, prototree.SubscriptionType, state.Keypath, *prototree.FetchHistoryOpts) error); ok {
		r1 = rf(ctx, stateURI, subscriptionType, keypath, fetchHistoryOpts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SubscribeStateURIs provides a mock function with given fields:
func (_m *TreeProtocol) SubscribeStateURIs() (prototree.StateURISubscription, error) {
	ret := _m.Called()

	var r0 prototree.StateURISubscription
	if rf, ok := ret.Get(0).(func() prototree.StateURISubscription); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(prototree.StateURISubscription)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Unsubscribe provides a mock function with given fields: stateURI
func (_m *TreeProtocol) Unsubscribe(stateURI string) error {
	ret := _m.Called(stateURI)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(stateURI)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
