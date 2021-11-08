package tree

import (
	"redwood.dev/blob"
	"redwood.dev/state"
	"redwood.dev/types"
)

type dumbResolver struct{}

func NewDumbResolver(config state.Node, internalState map[string]interface{}) (Resolver, error) {
	return &dumbResolver{}, nil
}

func (r *dumbResolver) InternalState() map[string]interface{} {
	return map[string]interface{}{}
}

func (r *dumbResolver) ResolveState(node state.Node, blobStore blob.Store, sender types.Address, txID state.Version, parents []state.Version, ps []Patch) (err error) {
	for _, p := range ps {
		if p.Val != nil {
			err = node.Set(p.Keypath, p.Range, p.Val)
		} else {
			err = node.Delete(p.Keypath, p.Range)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
