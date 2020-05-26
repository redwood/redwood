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

func (r *dumbResolver) ResolveState(state tree.Node, refStore RefStore, sender types.Address, txID types.ID, parents []types.ID, ps []Patch) (err error) {
	for _, p := range ps {
		if p.Val != nil {
			err = state.Set(p.Keypath, p.Range, p.Val)
		} else {
			err = state.Delete(p.Keypath, p.Range)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
