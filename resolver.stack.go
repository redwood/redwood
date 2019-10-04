package redwood

type stackResolver struct {
	state     interface{}
	resolvers []Resolver
}

func NewStackResolver(resolvers []Resolver) *stackResolver {
	return &stackResolver{resolvers: resolvers}
}

func (r *stackResolver) ResolveState(state interface{}, patch Patch) (interface{}, error) {
	var err error
	for _, resolver := range r.resolvers {
		state, err = resolver.ResolveState(state, patch)
		if err != nil {
			return nil, err
		}
	}
	return state, nil
}
