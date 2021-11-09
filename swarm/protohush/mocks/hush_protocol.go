// Code generated by mockery v2.8.0. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	pb "redwood.dev/swarm/protohush/pb"

	process "redwood.dev/process"

	protohush "redwood.dev/swarm/protohush"

	types "redwood.dev/types"
)

// HushProtocol is an autogenerated mock type for the HushProtocol type
type HushProtocol struct {
	mock.Mock
}

// Autoclose provides a mock function with given fields:
func (_m *HushProtocol) Autoclose() {
	_m.Called()
}

// AutocloseWithCleanup provides a mock function with given fields: closeFn
func (_m *HushProtocol) AutocloseWithCleanup(closeFn func()) {
	_m.Called(closeFn)
}

// Close provides a mock function with given fields:
func (_m *HushProtocol) Close() error {
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
func (_m *HushProtocol) Ctx() context.Context {
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

// DecryptGroupMessage provides a mock function with given fields: msg
func (_m *HushProtocol) DecryptGroupMessage(msg pb.GroupMessage) error {
	ret := _m.Called(msg)

	var r0 error
	if rf, ok := ret.Get(0).(func(pb.GroupMessage) error); ok {
		r0 = rf(msg)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Done provides a mock function with given fields:
func (_m *HushProtocol) Done() <-chan struct{} {
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

// EncryptGroupMessage provides a mock function with given fields: sessionType, messageID, recipients, plaintext
func (_m *HushProtocol) EncryptGroupMessage(sessionType string, messageID string, recipients []types.Address, plaintext []byte) error {
	ret := _m.Called(sessionType, messageID, recipients, plaintext)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, string, []types.Address, []byte) error); ok {
		r0 = rf(sessionType, messageID, recipients, plaintext)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// EncryptIndividualMessage provides a mock function with given fields: sessionType, recipient, plaintext
func (_m *HushProtocol) EncryptIndividualMessage(sessionType string, recipient types.Address, plaintext []byte) error {
	ret := _m.Called(sessionType, recipient, plaintext)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, types.Address, []byte) error); ok {
		r0 = rf(sessionType, recipient, plaintext)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Go provides a mock function with given fields: ctx, name, fn
func (_m *HushProtocol) Go(ctx context.Context, name string, fn func(context.Context)) <-chan struct{} {
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
func (_m *HushProtocol) Name() string {
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
func (_m *HushProtocol) NewChild(ctx context.Context, name string) *process.Process {
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

// OnGroupMessageDecrypted provides a mock function with given fields: sessionType, handler
func (_m *HushProtocol) OnGroupMessageDecrypted(sessionType string, handler protohush.GroupMessageDecryptedCallback) {
	_m.Called(sessionType, handler)
}

// OnGroupMessageEncrypted provides a mock function with given fields: sessionType, handler
func (_m *HushProtocol) OnGroupMessageEncrypted(sessionType string, handler protohush.GroupMessageEncryptedCallback) {
	_m.Called(sessionType, handler)
}

// OnIndividualMessageDecrypted provides a mock function with given fields: sessionType, handler
func (_m *HushProtocol) OnIndividualMessageDecrypted(sessionType string, handler protohush.IndividualMessageDecryptedCallback) {
	_m.Called(sessionType, handler)
}

// ProcessTree provides a mock function with given fields:
func (_m *HushProtocol) ProcessTree() map[string]interface{} {
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

// ProposeIndividualSession provides a mock function with given fields: ctx, sessionType, recipient, epoch
func (_m *HushProtocol) ProposeIndividualSession(ctx context.Context, sessionType string, recipient types.Address, epoch uint64) error {
	ret := _m.Called(ctx, sessionType, recipient, epoch)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, types.Address, uint64) error); ok {
		r0 = rf(ctx, sessionType, recipient, epoch)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ProposeNextIndividualSession provides a mock function with given fields: ctx, sessionType, recipient
func (_m *HushProtocol) ProposeNextIndividualSession(ctx context.Context, sessionType string, recipient types.Address) error {
	ret := _m.Called(ctx, sessionType, recipient)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, types.Address) error); ok {
		r0 = rf(ctx, sessionType, recipient)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SpawnChild provides a mock function with given fields: ctx, child
func (_m *HushProtocol) SpawnChild(ctx context.Context, child process.Spawnable) error {
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
func (_m *HushProtocol) Start() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}