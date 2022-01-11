package prototree

import (
	"fmt"
	"strings"

	"redwood.dev/errors"
	"redwood.dev/state"
	"redwood.dev/tree"
	"redwood.dev/types"
)

type ACL interface {
	TypeOf(stateURI string) StateURIType
	MembersOf(stateURI string) (types.AddressSet, error)
	HasReadAccess(stateURI string, keypath state.Keypath, addresses types.AddressSet) (bool, error)
}

type StateURIType int

const (
	StateURIType_Invalid StateURIType = iota
	StateURIType_DeviceLocal
	// StateURIType_UserLocal
	StateURIType_Private
	StateURIType_Public
)

func (t StateURIType) String() string {
	switch t {
	case StateURIType_Invalid:
		return "invalid"
	case StateURIType_DeviceLocal:
		return "device local"
	case StateURIType_Private:
		return "private"
	case StateURIType_Public:
		return "public"
	default:
		return fmt.Sprintf("<ERR: bad state uri type: %d>", t)
	}
}

type DefaultACL struct {
	ControllerHub tree.ControllerHub
}

var DefaultACLMembersKeypath = state.Keypath("Members")

var _ ACL = DefaultACL{}

func (acl DefaultACL) TypeOf(stateURI string) StateURIType {
	parts := strings.Split(stateURI, "/")
	if len(parts) != 2 || len(parts[1]) == 0 {
		return StateURIType_Invalid
	}
	hostParts := strings.Split(parts[0], ".")
	if len(hostParts) < 2 || len(hostParts[0]) == 0 || len(hostParts[1]) == 0 {
		return StateURIType_Invalid
	}
	switch hostParts[1] {
	case "local":
		return StateURIType_DeviceLocal
	case "p2p":
		return StateURIType_Private
	default:
		return StateURIType_Public
	}
}

func (acl DefaultACL) MembersOf(stateURI string) (types.AddressSet, error) {
	addrs := types.NewAddressSet(nil)

	state, err := acl.ControllerHub.StateAtVersion(stateURI, nil)
	if errors.Cause(err) == tree.ErrNoController {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	defer state.Close()

	iter := state.ChildIterator(DefaultACLMembersKeypath, true, 10)
	defer iter.Close()

	for iter.Rewind(); iter.Valid(); iter.Next() {
		addrHex := iter.Node().Keypath().Part(-1).String()

		addr, err := types.AddressFromHex(addrHex)
		if err != nil {
			return nil, err
		}
		addrs.Add(addr)
	}
	return addrs, nil
}

func (acl DefaultACL) HasReadAccess(stateURI string, keypath state.Keypath, addresses types.AddressSet) (bool, error) {
	switch acl.TypeOf(stateURI) {
	case StateURIType_Invalid:
		return false, errors.Errorf(`bad state URI: "%v"`, stateURI)
	case StateURIType_DeviceLocal:
		// @@TODO: credentials store to hold UCAN/JWT chains
		return true, nil
	case StateURIType_Private:
		return acl.isMemberOfPrivateStateURI(stateURI, addresses)
	case StateURIType_Public:
		return true, nil
	default:
		panic("invariant violation")
	}
}

func (acl DefaultACL) isMemberOfPrivateStateURI(stateURI string, addresses types.AddressSet) (bool, error) {
	state, err := acl.ControllerHub.StateAtVersion(stateURI, nil)
	if err != nil {
		return false, err
	}
	defer state.Close()

	for addr := range addresses {
		ok, err := state.Exists(DefaultACLMembersKeypath.Pushs(addr.Hex()))
		if err != nil {
			continue
		} else if !ok {
			continue
		}
		return true, nil
	}
	return false, nil
}
