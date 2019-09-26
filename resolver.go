package main

type Resolver interface {
	ResolveState(p Patch) (interface{}, error)
}

type resolverTree struct {
	root *resolverTreeNode
}

type resolverTreeNode struct {
	resolver Resolver
	subkeys  map[string]*resolverTreeNode
}

func (t *resolverTree) addResolver(keypath []string, resolver Resolver) {
	if t.root == nil {
		t.root = &resolverTreeNode{subkeys: map[string]*resolverTreeNode{}}
	}

	current := t.root

	for {
		if len(keypath) == 0 {
			current.resolver = resolver
			return
		}

		key := keypath[0]
		keypath = keypath[1:]

		if current.subkeys[key] == nil {
			current.subkeys[key] = &resolverTreeNode{subkeys: map[string]*resolverTreeNode{}}
		}
		current = current.subkeys[key]
	}
}

func (t *resolverTree) resolverForKeypath(keypath []string) (Resolver, []string) {
	currentKeypath := keypath
	current := t.root
	currentResolver := current.resolver
	currentResolverKeypathStartsAt := 0
	i := 0

	for {
		key := currentKeypath[0]
		currentKeypath = currentKeypath[1:]

		current = current.subkeys[key]
		if current == nil {
			break
		} else if len(currentKeypath) == 0 {
			break
		} else if current.resolver != nil {
			currentResolver = current.resolver
			currentResolverKeypathStartsAt = i
		}

		i++
	}

	parentResolverKeys := keypath[:currentResolverKeypathStartsAt]
	return currentResolver, parentResolverKeys
}
