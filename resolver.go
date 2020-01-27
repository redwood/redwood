package redwood

import (
	"bytes"
	"sort"

	"github.com/brynbellomy/redwood/tree"
	"github.com/brynbellomy/redwood/types"
)

type Resolver interface {
	ResolveState(state tree.Node, sender types.Address, txID types.ID, parents []types.ID, patches []Patch) error
	InternalState() map[string]interface{}
}

type Validator interface {
	ValidateTx(state tree.Node, txs, validTxs map[types.ID]*Tx, tx *Tx) error
}

type ResolverConstructor func(config tree.Node, internalState map[string]interface{}) (Resolver, error)
type ValidatorConstructor func(config tree.Node) (Validator, error)

var resolverRegistry map[string]ResolverConstructor
var validatorRegistry map[string]ValidatorConstructor

func init() {
	validatorRegistry = map[string]ValidatorConstructor{
		"validator/permissions": NewPermissionsValidator,
		// "stack":       NewStackValidator,
	}
	resolverRegistry = map[string]ResolverConstructor{
		"resolver/dumb": NewDumbResolver,
		"resolver/lua":  NewLuaResolver,
		"resolver/js":   NewJSResolver,
		//"resolver/stack": NewStackResolver,
	}
}

type resolverTree struct {
	validatorKeypaths []tree.Keypath
	validators        map[string]Validator
	resolverKeypaths  []tree.Keypath
	resolvers         map[string]Resolver
}

func newResolverTree() *resolverTree {
	return &resolverTree{
		validators: make(map[string]Validator),
		resolvers:  make(map[string]Resolver),
	}
}

func (t *resolverTree) addResolver(keypath tree.Keypath, resolver Resolver) {
	if _, exists := t.resolvers[string(keypath)]; !exists {
		t.resolverKeypaths = append(t.resolverKeypaths, keypath)
		// @@TODO: sucks
		sort.Slice(t.resolverKeypaths, func(i, j int) bool { return bytes.Compare(t.resolverKeypaths[i], t.resolverKeypaths[j]) < 0 })
	}
	t.resolvers[string(keypath)] = resolver
}

func (t *resolverTree) removeResolver(keypath tree.Keypath) {
	if _, exists := t.resolvers[string(keypath)]; !exists {
		return
	}
	delete(t.resolvers, string(keypath))
	var idx int
	for i, kp := range t.resolverKeypaths {
		if kp.Equals(keypath) {
			idx = i
			break
		}
	}
	copy(t.resolverKeypaths[idx:], t.resolverKeypaths[idx+1:])
	t.resolverKeypaths = t.resolverKeypaths[:len(t.resolverKeypaths)-1]
}

func (t *resolverTree) addValidator(keypath tree.Keypath, validator Validator) {
	if _, exists := t.validators[string(keypath)]; !exists {
		t.validatorKeypaths = append(t.validatorKeypaths, keypath)
		// @@TODO: sucks
		sort.Slice(t.validatorKeypaths, func(i, j int) bool { return bytes.Compare(t.validatorKeypaths[i], t.validatorKeypaths[j]) < 0 })
	}
	t.validators[string(keypath)] = validator
}

func (t *resolverTree) removeValidator(keypath tree.Keypath) {
	if _, exists := t.validators[string(keypath)]; !exists {
		return
	}
	delete(t.validators, string(keypath))
	var idx int
	for i, kp := range t.validatorKeypaths {
		if kp.Equals(keypath) {
			idx = i
			break
		}
	}
	copy(t.validatorKeypaths[idx:], t.validatorKeypaths[idx+1:])
	t.validatorKeypaths = t.validatorKeypaths[:len(t.validatorKeypaths)-1]
}

func (t *resolverTree) nearestResolverForKeypath(keypath tree.Keypath) (Resolver, tree.Keypath) {
	for i := len(t.resolverKeypaths) - 1; i >= 0; i-- {
		kp := t.resolverKeypaths[i]
		if keypath.StartsWith(kp) {
			return t.resolvers[string(kp)], kp
		}
	}
	return nil, nil
}

func (t *resolverTree) nearestValidatorForKeypath(keypath tree.Keypath) (Validator, tree.Keypath) {
	for i := len(t.validatorKeypaths) - 1; i >= 0; i-- {
		kp := t.validatorKeypaths[i]
		if keypath.StartsWith(kp) {
			return t.validators[string(kp)], kp
		}
	}
	return nil, nil
}
