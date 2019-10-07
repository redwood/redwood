package redwood

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/plan-systems/plan-core/tools/ctx"
)

type Store interface {
	AddTx(tx *Tx) error
	RemoveTx(txID ID) error
	FetchTxs() ([]Tx, error)

	State() interface{}
	StateJSON() ([]byte, error)
	MostRecentTxID() ID

	RegisterResolverForKeypath(keypath []string, resolver Resolver)
	RegisterValidatorForKeypath(keypath []string, validator Validator)
}

type store struct {
	ctx.Context

	ID             ID
	mu             sync.RWMutex
	txs            map[ID]*Tx
	validTxs       map[ID]*Tx
	resolverTree   resolverTree
	currentState   interface{}
	stateHistory   map[ID]interface{}
	timeDAG        map[ID]map[ID]bool
	leaves         map[ID]bool
	chMempool      chan *Tx
	mostRecentTxID ID
}

func NewStore(id ID, genesisState interface{}) (Store, error) {
	s := &store{
		ID:             id,
		mu:             sync.RWMutex{},
		txs:            make(map[ID]*Tx),
		validTxs:       make(map[ID]*Tx),
		resolverTree:   resolverTree{},
		currentState:   genesisState,
		stateHistory:   make(map[ID]interface{}),
		timeDAG:        make(map[ID]map[ID]bool),
		leaves:         make(map[ID]bool),
		chMempool:      make(chan *Tx, 100),
		mostRecentTxID: GenesisTxID,
	}

	err := s.Startup()

	return s, err
}

func (s *store) Startup() error {
	return s.CtxStart(
		s.ctxStartup,
		nil,
		nil,
		s.ctxStopping,
	)
}

func (s *store) ctxStartup() error {
	s.SetLogLabel(s.ID.Pretty()[:4] + " store")

	s.RegisterResolverForKeypath([]string{}, &dumbResolver{})
	s.RegisterValidatorForKeypath([]string{}, &permissionsValidator{})

	go s.mempoolLoop()

	return nil
}

func (s *store) ctxStopping() {
	// No op since c.Ctx will cancel as this ctx completes stopping
}

func (s *store) State() interface{} {
	return s.currentState
}

func (s *store) StateJSON() ([]byte, error) {
	bs, err := json.MarshalIndent(s.currentState, "", "    ")
	if err != nil {
		return nil, err
	}
	str := string(bs)
	str = strings.Replace(str, "\\n", "\n", -1)
	return []byte(str), nil
}

func (s *store) MostRecentTxID() ID {
	return s.mostRecentTxID
}

func (s *store) RegisterResolverForKeypath(keypath []string, resolver Resolver) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.resolverTree.addResolver(keypath, resolver)
}

func (s *store) RegisterValidatorForKeypath(keypath []string, validator Validator) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.resolverTree.addValidator(keypath, validator)
}

func (s *store) AddTx(tx *Tx) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ignore duplicates
	if _, exists := s.txs[tx.ID]; exists {
		return nil
	}

	s.Infof(0, "new tx %v", tx.ID)

	// Store the tx (so we can ignore txs we've seen before)
	s.txs[tx.ID] = tx

	s.addToMempool(tx)
	return nil
}

func (s *store) addToMempool(tx *Tx) {
	select {
	case <-s.Ctx.Done():
	case s.chMempool <- tx:
	}
}

func (s *store) mempoolLoop() {
	for {
		select {
		case <-s.Ctx.Done():
			return
		case tx := <-s.chMempool:
			err := s.processMempoolTx(tx)
			if errors.Cause(err) == ErrNoParentYet {
				s.addToMempool(tx)
			} else if err != nil {
				s.Errorf("invalid tx %+v: %v", *tx, err)
			} else {
				s.Infof(0, "tx added to chain (%v)", tx.ID.Pretty())
			}
		}
	}
}

func (s *store) processMempoolTx(tx *Tx) error {
	err := s.validateTxIntrinsics(tx)
	if err != nil {
		return err
	}

	// Validate the tx
	validators := make(map[Validator][]Patch)
	validatorKeypaths := make(map[Validator][]string)
	for _, patch := range tx.Patches {
		v, idx := s.resolverTree.validatorForKeypath(patch.Keys)
		keys := make([]string, len(patch.Keys)-(idx+1))
		copy(keys, patch.Keys[idx+1:])
		p := patch
		p.Keys = keys

		validators[v] = append(validators[v], p)
		validatorKeypaths[v] = patch.Keys[:idx]
	}

	for validator, patches := range validators {
		if len(patches) == 0 {
			continue
		}

		txCopy := *tx
		txCopy.Patches = patches

		err := validator.Validate(s.stateAtKeypath(validatorKeypaths[validator]), s.txs, s.validTxs, txCopy)
		if err != nil {
			return err
		}
	}

	tx.Valid = true
	s.validTxs[tx.ID] = tx
	s.mostRecentTxID = tx.ID

	// Unmark parents as leaves
	for _, parentID := range tx.Parents {
		delete(s.leaves, parentID)
	}

	// @@TODO: add to timeDAG

	// Apply its changes to the state tree
	for _, p := range tx.Patches {
		var patch Patch = p
		var newState interface{}
		var err error
		for {
			resolver, currentResolverKeypathStartsAt := s.resolverTree.resolverForKeypath(patch.Keys)
			parentResolverKeypath := patch.Keys[:currentResolverKeypathStartsAt]
			thisResolverKeypath := patch.Keys[:currentResolverKeypathStartsAt]

			thisResolverState := s.stateAtKeypath(thisResolverKeypath)

			patchCopy := patch
			patchCopy.Keys = patchCopy.Keys[len(parentResolverKeypath):]

			newState, err = resolver.ResolveState(thisResolverState, tx.From, patchCopy)
			if err != nil {
				return err
			}

			if len(parentResolverKeypath) == 0 {
				break
			}

			patch = Patch{Keys: parentResolverKeypath, Val: newState}
		}
		s.currentState = newState
	}

	// Walk the tree and initialize validators and resolvers
	// @@TODO: inefficient
	s.resolverTree = resolverTree{}
	s.resolverTree.addResolver([]string{}, &dumbResolver{})
	s.resolverTree.addValidator([]string{}, &permissionsValidator{})
	err = walkTree(s.currentState, func(keypath []string, val interface{}) error {
		m, isMap := val.(map[string]interface{})
		if !isMap {
			return nil
		}

		resolverConfig, exists := M(m).GetMap("resolver")
		if !exists {
			return nil
		}
		resolver, err := initResolverFromConfig(resolverConfig)
		if err != nil {
			return err
		}
		s.resolverTree.addResolver(keypath, resolver)

		validatorConfig, exists := M(m).GetMap("validator")
		if !exists {
			return nil
		}
		validator, err := initValidatorFromConfig(validatorConfig)
		if err != nil {
			return err
		}
		s.resolverTree.addValidator(keypath, validator)

		return nil
	})
	if err != nil {
		return err
	}

	j, err := s.StateJSON()
	if err != nil {
		return err
	}
	s.Infof(0, "state = %v", string(j))

	// Save historical state

	return nil
}

var (
	ErrNoParentYet      = errors.New("no parent yet")
	ErrInvalidSignature = errors.New("invalid signature")
)

func (s *store) validateTxIntrinsics(tx *Tx) error {
	if len(tx.Parents) == 0 {
		return errors.New("tx must have parents")
	} else if len(s.validTxs) > 0 && len(tx.Parents) == 1 && tx.Parents[0] == GenesisTxID {
		return errors.New("already have a genesis tx")
	}

	for _, parentID := range tx.Parents {
		if _, exists := s.validTxs[parentID]; !exists && parentID.Pretty() != GenesisTxID.Pretty() {
			return errors.Wrapf(ErrNoParentYet, "txid: %v", parentID.Pretty())
		}
	}

	hash, err := tx.Hash()
	if err != nil {
		return errors.WithStack(err)
	}

	pk, err := RecoverPubkey(hash, tx.Sig)
	if err != nil {
		return errors.Wrap(ErrInvalidSignature, err.Error())
	} else if VerifySignature(pk, hash, tx.Sig) == false {
		return errors.Wrap(ErrInvalidSignature, err.Error())
	} else if pk.Address() != tx.From {

		return errors.Wrapf(ErrInvalidSignature, "address doesn't match (%v expected, %v received)", tx.From, pk.Address())
	}

	return nil
}

func (s *store) stateAtKeypath(keypath []string) interface{} {
	current := s.currentState
	for _, key := range keypath {
		asMap, isMap := current.(map[string]interface{})
		if !isMap {
			return nil
		}
		current = asMap[key]
	}
	return current
}

func (s *store) RemoveTx(txID ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.txs, txID)

	return nil
}

func (s *store) FetchTxs() ([]Tx, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var txs []Tx
	for _, tx := range s.txs {
		txs = append(txs, *tx)
	}

	return txs, nil
}

func (s *store) getAncestors(vids map[ID]bool) map[ID]bool {
	ancestors := map[ID]bool{}

	var mark_ancestors func(id ID)
	mark_ancestors = func(id ID) {
		if !ancestors[id] {
			ancestors[id] = true
			for parentID := range s.timeDAG[id] {
				mark_ancestors(parentID)
			}
		}
	}
	for parentID := range vids {
		mark_ancestors(parentID)
	}

	return ancestors
}
