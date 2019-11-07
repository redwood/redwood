package redwood

import (
	"github.com/pkg/errors"
)

type stackResolver struct {
	resolver
	state     interface{}
	resolvers []Resolver
}

func (r *stackResolver) InternalState() map[string]interface{} {
	return nil
}

func NewStackResolver(params map[string]interface{}, internalState map[string]interface{}) (Resolver, error) {
	children, exists := getSlice(params, []string{"children"})
	if !exists {
		return nil, errors.New("stack resolver needs an array 'children' param")
	}

	var resolvers []Resolver
	for i := range children {
		config, is := children[i].(map[string]interface{})
		if !is {
			return nil, errors.New("stack resolver found something that didn't look like a resolver config")
		}

		resolver, err := initResolverFromConfig(config, nil) // @@TODO: stack resolver internal state...?
		if err != nil {
			return nil, err
		}

		resolvers = append(resolvers, resolver)
	}

	return &stackResolver{resolvers: resolvers}, nil
}

func (r *stackResolver) ResolveState(state interface{}, sender Address, txID ID, parents []ID, patches []Patch) (interface{}, error) {
	var err error
	for _, resolver := range r.resolvers {
		state, err = resolver.ResolveState(state, sender, txID, parents, patches)
		if err != nil {
			return nil, err
		}
	}
	return state, nil
}
