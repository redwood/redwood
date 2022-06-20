package tree

import (
	"redwood.dev/blob"
	"redwood.dev/state"
	"redwood.dev/types"
)

type nullResolver struct{}

func (r nullResolver) ResolveState(node state.Node, blobStore blob.Store, sender types.Address, txID state.Version, parents []state.Version, patches []Patch) error {
	return nil
}

func (r nullResolver) InternalState() map[string]interface{} {
	return nil
}
