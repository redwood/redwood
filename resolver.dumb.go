package redwood

import (
	"github.com/brynbellomy/redwood/tree"
	"github.com/brynbellomy/redwood/types"
)

type dumbResolver struct{}

func NewDumbResolver(config tree.Node, internalState map[string]interface{}) (Resolver, error) {
	return &dumbResolver{}, nil
}

func (r *dumbResolver) InternalState() map[string]interface{} {
	return map[string]interface{}{}
}

func (r *dumbResolver) ResolveState(state tree.Node, sender types.Address, txID types.ID, parents []types.ID, ps []Patch) error {
	for _, p := range ps {
		err := state.Set(p.Keypath, p.Range, p.Val)
		if err != nil {
			return err
		}
	}
	return nil
}
