package redwood

import (
	"github.com/pkg/errors"
)

type Resolver interface {
	ResolveState(state interface{}, sender ID, patch Patch) (interface{}, error)
}

type Validator interface {
	Validate(state interface{}, txs map[ID]Tx, tx Tx) error
}

type ResolverConstructor func(params map[string]interface{}) (Resolver, error)
type ValidatorConstructor func(params map[string]interface{}) (Validator, error)

var resolverRegistry map[string]ResolverConstructor
var validatorRegistry map[string]ValidatorConstructor

func init() {
	validatorRegistry = map[string]ValidatorConstructor{
		"intrinsics":  NewIntrinsicsValidator,
		"permissions": NewPermissionsValidator,
		"stack":       NewStackValidator,
	}
	resolverRegistry = map[string]ResolverConstructor{
		"dumb":  NewDumbResolver,
		"stack": NewStackResolver,
		"lua":   NewLuaResolver,
	}
}

func initResolverFromConfig(config map[string]interface{}) (Resolver, error) {
	typ, exists := M(config).GetString("type")
	if !exists {
		return nil, errors.New("cannot init resolver without a 'type' param")
	}
	ctor, exists := resolverRegistry[typ]
	if !exists {
		return nil, errors.Errorf("unknown resolver type '%v'", typ)
	}
	return ctor(config)
}

func initValidatorFromConfig(config map[string]interface{}) (Validator, error) {
	typ, exists := M(config).GetString("type")
	if !exists {
		return nil, errors.New("cannot init validator without a 'type' param")
	}
	ctor, exists := validatorRegistry[typ]
	if !exists {
		return nil, errors.Errorf("unknown validator type '%v'", typ)
	}
	return ctor(config)
}

type resolverTree struct {
	root *resolverTreeNode
}

type resolverTreeNode struct {
	resolver  Resolver
	validator Validator
	subkeys   map[string]*resolverTreeNode
}

func (t *resolverTree) addResolver(keypath []string, resolver Resolver) {
	node := t.ensureNodeExists(keypath)
	node.resolver = resolver
}

func (t *resolverTree) addValidator(keypath []string, validator Validator) {
	node := t.ensureNodeExists(keypath)
	node.validator = validator
}

func (t *resolverTree) ensureNodeExists(keypath []string) *resolverTreeNode {
	if t.root == nil {
		t.root = &resolverTreeNode{subkeys: map[string]*resolverTreeNode{}}
	}

	current := t.root

	for {
		if len(keypath) == 0 {
			return current
		}

		key := keypath[0]
		keypath = keypath[1:]

		if current.subkeys[key] == nil {
			current.subkeys[key] = &resolverTreeNode{subkeys: map[string]*resolverTreeNode{}}
		}
		current = current.subkeys[key]
	}
}

func (t *resolverTree) closestAncestorOfKeypathWhere(keypath []string, condition func(*resolverTreeNode) bool) (*resolverTreeNode, int) {
	remaining := keypath
	current := t.root
	ancestor := (*resolverTreeNode)(nil)
	closestAncestorKeypathIdx := -1
	i := 0

	for {
		if current == nil {
			break
		} else if len(remaining) == 0 {
			break
		}
		if condition(current) == true {
			closestAncestorKeypathIdx = i
			ancestor = current
		}

		key := remaining[0]
		remaining = remaining[1:]

		current = current.subkeys[key]

		i++
	}

	return ancestor, closestAncestorKeypathIdx
}

func (t *resolverTree) resolverForKeypath(keypath []string) (Resolver, int) {
	node, idx := t.closestAncestorOfKeypathWhere(keypath, func(node *resolverTreeNode) bool { return node.resolver != nil })
	return node.resolver, idx
}

func (t *resolverTree) validatorForKeypath(keypath []string) (Validator, int) {
	node, idx := t.closestAncestorOfKeypathWhere(keypath, func(node *resolverTreeNode) bool { return node.validator != nil })
	return node.validator, idx
}
