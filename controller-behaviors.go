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
	ValidateTx(state tree.Node, tx *Tx) error
}

type Indexer interface {
	IndexNode(relKeypath tree.Keypath, state tree.Node) (tree.Keypath, tree.Node, error)
}

type ResolverConstructor func(config tree.Node, internalState map[string]interface{}) (Resolver, error)
type ValidatorConstructor func(config tree.Node) (Validator, error)
type IndexerConstructor func(config tree.Node) (Indexer, error)

var resolverRegistry map[string]ResolverConstructor
var validatorRegistry map[string]ValidatorConstructor
var indexerRegistry map[string]IndexerConstructor

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
	indexerRegistry = map[string]IndexerConstructor{
		"indexer/keypath": NewKeypathIndexer,
		"indexer/js":      NewJSIndexer,
	}
}

type behaviorTree struct {
	validatorKeypaths []tree.Keypath
	validators        map[string]Validator
	resolverKeypaths  []tree.Keypath
	resolvers         map[string]Resolver
	indexers          map[string]map[string]Indexer
}

func newBehaviorTree() *behaviorTree {
	return &behaviorTree{
		validators: make(map[string]Validator),
		resolvers:  make(map[string]Resolver),
		indexers:   make(map[string]map[string]Indexer),
	}
}

func (t *behaviorTree) addResolver(keypath tree.Keypath, resolver Resolver) {
	if _, exists := t.resolvers[string(keypath)]; !exists {
		t.resolverKeypaths = append(t.resolverKeypaths, keypath)
		// @@TODO: sucks
		sort.Slice(t.resolverKeypaths, func(i, j int) bool { return bytes.Compare(t.resolverKeypaths[i], t.resolverKeypaths[j]) < 0 })
	}
	t.resolvers[string(keypath)] = resolver
}

func (t *behaviorTree) removeResolver(keypath tree.Keypath) {
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

func (t *behaviorTree) addValidator(keypath tree.Keypath, validator Validator) {
	if _, exists := t.validators[string(keypath)]; !exists {
		t.validatorKeypaths = append(t.validatorKeypaths, keypath)
		// @@TODO: sucks
		sort.Slice(t.validatorKeypaths, func(i, j int) bool { return bytes.Compare(t.validatorKeypaths[i], t.validatorKeypaths[j]) < 0 })
	}
	t.validators[string(keypath)] = validator
}

func (t *behaviorTree) removeValidator(keypath tree.Keypath) {
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

func (t *behaviorTree) addIndexer(keypath tree.Keypath, indexName tree.Keypath, indexer Indexer) {
	if _, exists := t.indexers[string(keypath)]; !exists {
		t.indexers[string(keypath)] = make(map[string]Indexer)
	}
	t.indexers[string(keypath)][string(indexName)] = indexer
}

func (t *behaviorTree) removeIndexer(keypath tree.Keypath, indexName tree.Keypath) {
	if _, exists := t.indexers[string(keypath)]; !exists {
		return
	}
	delete(t.indexers[string(keypath)], string(indexName))
}

func (t *behaviorTree) nearestResolverForKeypath(keypath tree.Keypath) (Resolver, tree.Keypath) {
	for i := len(t.resolverKeypaths) - 1; i >= 0; i-- {
		kp := t.resolverKeypaths[i]
		if keypath.StartsWith(kp) {
			return t.resolvers[string(kp)], kp
		}
	}
	return nil, nil
}

func (t *behaviorTree) nearestValidatorForKeypath(keypath tree.Keypath) (Validator, tree.Keypath) {
	for i := len(t.validatorKeypaths) - 1; i >= 0; i-- {
		kp := t.validatorKeypaths[i]
		if keypath.StartsWith(kp) {
			return t.validators[string(kp)], kp
		}
	}
	return nil, nil
}
