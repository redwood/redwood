package redwood

import (
	"github.com/pkg/errors"
)

type Resolver interface {
	ResolveState(state interface{}, sender Address, txID ID, parents []ID, patches []Patch) (interface{}, error)
	Keypath() []string
	SetKeypath(keypath []string)
	InternalState() map[string]interface{}
}

type Validator interface {
	ValidateTx(state interface{}, txs, validTxs map[ID]*Tx, tx Tx) error
}

type resolver struct {
	keypath []string
}

func (r *resolver) Keypath() []string {
	return r.keypath
}

func (r *resolver) SetKeypath(keypath []string) {
	r.keypath = keypath
}

type ResolverConstructor func(params map[string]interface{}, internalState map[string]interface{}) (Resolver, error)
type ValidatorConstructor func(params map[string]interface{}) (Validator, error)

var resolverRegistry map[string]ResolverConstructor
var validatorRegistry map[string]ValidatorConstructor

func init() {
	validatorRegistry = map[string]ValidatorConstructor{
		"permissions": NewPermissionsValidator,
		// "stack":       NewStackValidator,
	}
	resolverRegistry = map[string]ResolverConstructor{
		"dumb":  NewDumbResolver,
		"stack": NewStackResolver,
		"lua":   NewLuaResolver,
		"js":    NewJSResolver,
	}
}

func initResolverFromConfig(config map[string]interface{}, internalState map[string]interface{}) (Resolver, error) {
	typ, exists := getString(config, []string{"type"})
	if !exists {
		return nil, errors.New("cannot init resolver without a 'type' param")
	}
	ctor, exists := resolverRegistry[typ]
	if !exists {
		return nil, errors.Errorf("unknown resolver type '%v'", typ)
	}
	return ctor(config, internalState)
}

func initValidatorFromConfig(config map[string]interface{}) (Validator, error) {
	typ, exists := getString(config, []string{"type"})
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
	depth     int
	keypath   []string
}

func (t *resolverTree) addResolver(keypath []string, resolver Resolver) {
	node := t.ensureNodeExists(keypath)
	node.resolver = resolver
	resolver.SetKeypath(keypath)
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
	depth := len(keypath)

	for {
		if len(keypath) == 0 {
			return current
		}

		key := keypath[0]
		keypath = keypath[1:]

		if current.subkeys[key] == nil {
			nodeKeypath := make([]string, len(keypath))
			copy(nodeKeypath, keypath)
			current.subkeys[key] = &resolverTreeNode{subkeys: map[string]*resolverTreeNode{}, depth: depth, keypath: nodeKeypath}
		}
		current = current.subkeys[key]
	}
}

func (t *resolverTree) deepestNodeInKeypathWhere(keypath []string, condition func(*resolverTreeNode) bool) (*resolverTreeNode, int) {
	remaining := keypath
	current := t.root
	ancestor := (*resolverTreeNode)(nil)
	closestAncestorKeypathIdx := -1
	i := 0

	for {
		if current == nil {
			break
		}
		if condition(current) == true {
			closestAncestorKeypathIdx = i
			ancestor = current
		}
		if len(remaining) == 0 {
			break
		}

		key := remaining[0]
		remaining = remaining[1:]

		current = current.subkeys[key]

		i++
	}

	return ancestor, closestAncestorKeypathIdx
}

func (t *resolverTree) nearestResolverNodeForKeypath(keypath []string) (*resolverTreeNode, int) {
	return t.deepestNodeInKeypathWhere(keypath, func(node *resolverTreeNode) bool { return node.resolver != nil })
}

func (t *resolverTree) nearestValidatorNodeForKeypath(keypath []string) (*resolverTreeNode, int) {
	return t.deepestNodeInKeypathWhere(keypath, func(node *resolverTreeNode) bool { return node.validator != nil })
}

func (t *resolverTree) groupPatchesByValidator(patches []Patch) (map[Validator][]Patch, map[Validator][]string, []Patch) {
	validators := make(map[Validator][]Patch)
	validatorKeypaths := make(map[Validator][]string)
	var notValidated []Patch
	for _, patch := range patches {
		node, idx := t.nearestValidatorNodeForKeypath(patch.Keys)
		if node == nil {
			notValidated = append(notValidated, patch)
			continue
		}
		v := node.validator
		keys := make([]string, len(patch.Keys)-(idx))
		copy(keys, patch.Keys[idx:])
		p := patch
		p.Keys = keys

		validators[v] = append(validators[v], p)
		validatorKeypaths[v] = patch.Keys[:idx]
	}
	return validators, validatorKeypaths, notValidated
}
