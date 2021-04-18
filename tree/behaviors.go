package tree

import (
	"bytes"
	"sort"

	"redwood.dev/blob"
	"redwood.dev/log"
	"redwood.dev/state"
	"redwood.dev/types"
)

type Resolver interface {
	ResolveState(node state.Node, blobStore blob.Store, sender types.Address, txID types.ID, parents []types.ID, patches []Patch) error
	InternalState() map[string]interface{}
}

type Validator interface {
	ValidateTx(node state.Node, tx *Tx) error
}

type Indexer interface {
	IndexNode(relKeypath state.Keypath, node state.Node) (state.Keypath, state.Node, error)
}

type ResolverConstructor func(config state.Node, internalState map[string]interface{}) (Resolver, error)
type ValidatorConstructor func(config state.Node) (Validator, error)
type IndexerConstructor func(config state.Node) (Indexer, error)

var resolverRegistry = map[string]ResolverConstructor{
	"resolver/dumb": NewDumbResolver,
	"resolver/lua":  NewLuaResolver,
	"resolver/js":   NewJSResolver,
	// "resolver/git":  NewGitResolver,
	//"resolver/stack": NewStackResolver,
}
var validatorRegistry = map[string]ValidatorConstructor{
	"validator/permissions": NewPermissionsValidator,
	// "stack":       NewStackValidator,
}
var indexerRegistry = map[string]IndexerConstructor{
	"indexer/keypath": NewKeypathIndexer,
	"indexer/js":      NewJSIndexer,
}

type behaviorTree struct {
	log.Logger
	validatorKeypaths []state.Keypath
	validators        map[string]Validator
	resolverKeypaths  []state.Keypath
	resolvers         map[string]Resolver
	indexers          map[string]map[string]Indexer
}

func newBehaviorTree() *behaviorTree {
	return &behaviorTree{
		Logger:     log.NewLogger(""),
		validators: make(map[string]Validator),
		resolvers:  make(map[string]Resolver),
		indexers:   make(map[string]map[string]Indexer),
	}
}

func (t *behaviorTree) copy() *behaviorTree {
	cp := &behaviorTree{
		validatorKeypaths: make([]state.Keypath, len(t.validatorKeypaths)),
		validators:        make(map[string]Validator, len(t.validators)),
		resolverKeypaths:  make([]state.Keypath, len(t.resolverKeypaths)),
		resolvers:         make(map[string]Resolver, len(t.resolvers)),
		indexers:          make(map[string]map[string]Indexer, len(t.indexers)),
	}
	for i, v := range t.validatorKeypaths {
		cp.validatorKeypaths[i] = v
	}
	for k, v := range t.validators {
		cp.validators[k] = v
	}
	for i, v := range t.resolverKeypaths {
		cp.resolverKeypaths[i] = v
	}
	for k, v := range t.resolvers {
		cp.resolvers[k] = v
	}
	for k, v := range t.indexers {
		cp.indexers[k] = make(map[string]Indexer, len(t.indexers[k]))
		for kk, vv := range v {
			cp.indexers[k][kk] = vv
		}
	}
	return cp
}

func (t *behaviorTree) debugPrint() {
	t.Debugf("BehaviorTree:\n----------------------------------------")
	for i := range t.validatorKeypaths {
		t.Debugf("  - validatorKeypaths[%v] = %v", i, t.validatorKeypaths[i])
	}
	for k := range t.validators {
		t.Debugf("  - validators[%v] = (%T) %v", k, t.validators[k], t.validators[k])
	}
	for i := range t.resolverKeypaths {
		t.Debugf("  - resolverKeypaths[%v] = %v", i, t.resolverKeypaths[i])
	}
	for k := range t.resolvers {
		t.Debugf("  - resolvers[%v] = (%T) %v", k, t.resolvers[k], t.resolvers[k])
	}
}

func (t *behaviorTree) addResolver(keypath state.Keypath, resolver Resolver) {
	if _, exists := t.resolvers[string(keypath)]; !exists {
		t.resolverKeypaths = append(t.resolverKeypaths, keypath)
		// @@TODO: sucks
		sort.Slice(t.resolverKeypaths, func(i, j int) bool { return bytes.Compare(t.resolverKeypaths[i], t.resolverKeypaths[j]) < 0 })
	}
	t.resolvers[string(keypath)] = resolver
}

func (t *behaviorTree) removeResolver(keypath state.Keypath) {
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

func (t *behaviorTree) addValidator(keypath state.Keypath, validator Validator) {
	if _, exists := t.validators[string(keypath)]; !exists {
		t.validatorKeypaths = append(t.validatorKeypaths, keypath)
		// @@TODO: sucks
		sort.Slice(t.validatorKeypaths, func(i, j int) bool { return bytes.Compare(t.validatorKeypaths[i], t.validatorKeypaths[j]) < 0 })
	}
	t.validators[string(keypath)] = validator
}

func (t *behaviorTree) removeValidator(keypath state.Keypath) {
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

func (t *behaviorTree) addIndexer(keypath state.Keypath, indexName state.Keypath, indexer Indexer) {
	if _, exists := t.indexers[string(keypath)]; !exists {
		t.indexers[string(keypath)] = make(map[string]Indexer)
	}
	t.indexers[string(keypath)][string(indexName)] = indexer
}

func (t *behaviorTree) removeIndexer(keypath state.Keypath, indexName state.Keypath) {
	if _, exists := t.indexers[string(keypath)]; !exists {
		return
	}
	delete(t.indexers[string(keypath)], string(indexName))
}

func (t *behaviorTree) nearestResolverForKeypath(keypath state.Keypath) (Resolver, state.Keypath) {
	for i := len(t.resolverKeypaths) - 1; i >= 0; i-- {
		kp := t.resolverKeypaths[i]
		if keypath.StartsWith(kp) {
			return t.resolvers[string(kp)], kp
		}
	}
	return nil, nil
}

func (t *behaviorTree) nearestValidatorForKeypath(keypath state.Keypath) (Validator, state.Keypath) {
	for i := len(t.validatorKeypaths) - 1; i >= 0; i-- {
		kp := t.validatorKeypaths[i]
		if keypath.StartsWith(kp) {
			return t.validators[string(kp)], kp
		}
	}
	return nil, nil
}
