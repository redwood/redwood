package redwood

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/ctx"
)

type Store interface {
	Ctx() *ctx.Context
	Start() error

	AddTx(tx *Tx) error
	RemoveTx(txHash Hash) error
	FetchTxs() ([]Tx, error)
	HaveTx(txHash Hash) bool

	State() interface{}
	StateJSON() ([]byte, error)
	MostRecentTxHash() Hash

	SetResolver(keypath []string, resolver Resolver)
	SetValidator(keypath []string, validator Validator)
}

type store struct {
	*ctx.Context

	address          Address
	mu               sync.RWMutex
	txs              map[Hash]*Tx
	validTxs         map[Hash]*Tx
	resolverTree     resolverTree
	currentState     interface{}
	stateHistory     map[Hash]interface{}
	timeDAG          map[Hash]map[Hash]bool
	leaves           map[Hash]bool
	chMempool        chan *Tx
	mostRecentTxHash Hash

	persistence Persistence
}

func NewStore(address Address, genesisState interface{}, persistence Persistence) (Store, error) {
	s := &store{
		Context:          &ctx.Context{},
		address:          address,
		mu:               sync.RWMutex{},
		txs:              make(map[Hash]*Tx),
		validTxs:         make(map[Hash]*Tx),
		resolverTree:     resolverTree{},
		currentState:     genesisState,
		stateHistory:     make(map[Hash]interface{}),
		timeDAG:          make(map[Hash]map[Hash]bool),
		leaves:           make(map[Hash]bool),
		chMempool:        make(chan *Tx, 100),
		mostRecentTxHash: GenesisTxHash,
		persistence:      persistence,
	}

	return s, nil
}

func (s *store) Start() error {
	return s.CtxStart(
		// on startup,
		func() error {
			s.SetLogLabel(s.address.Pretty() + " store")

			s.SetResolver([]string{}, &dumbResolver{})
			s.SetValidator([]string{}, &permissionsValidator{})

			s.CtxAddChild(s.persistence.Ctx(), nil)

			err := s.persistence.Start()
			if err != nil {
				return err
			}

			go s.mempoolLoop()

			err = s.replayStoredTxs()
			if err != nil {
				return err
			}

			return nil
		},
		nil,
		nil,
		// on shutdown
		func() {},
	)
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

func (s *store) MostRecentTxHash() Hash {
	return s.mostRecentTxHash
}

func (s *store) SetResolver(keypath []string, resolver Resolver) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.resolverTree.addResolver(keypath, resolver)
}

func (s *store) SetValidator(keypath []string, validator Validator) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.resolverTree.addValidator(keypath, validator)
}

func (s *store) AddTx(tx *Tx) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ignore duplicates
	if _, exists := s.txs[tx.Hash()]; exists {
		s.Infof(0, "already know tx %v, skipping", tx.Hash().String())
		return nil
	}

	s.Infof(0, "new tx %v", tx.Hash().Pretty())

	// Store the tx (so we can ignore txs we've seen before)
	s.txs[tx.Hash()] = tx

	s.addToMempool(tx)
	return nil
}

func (s *store) replayStoredTxs() error {
	iter := s.persistence.AllTxs()
	defer iter.Cancel()

	for {
		tx := iter.Next()
		if iter.Error() != nil {
			return iter.Error()
		} else if tx == nil {
			return nil
		}

		s.Warnf("found stored tx %v", tx.Hash())
		err := s.AddTx(tx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *store) addToMempool(tx *Tx) {
	select {
	case <-s.Context.Done():
	case s.chMempool <- tx:
	}
}

func (s *store) mempoolLoop() {
	for {
		select {
		case <-s.Context.Done():
			return
		case tx := <-s.chMempool:
			err := s.processMempoolTx(tx)
			if errors.Cause(err) == ErrNoParentYet {
				go func() {
					select {
					case <-s.Context.Done():
					case <-time.After(500 * time.Millisecond):
						s.addToMempool(tx)
					}
				}()
			} else if err != nil {
				s.Errorf("invalid tx %+v: %v", *tx, err)
			} else {
				s.Infof(0, "tx added to chain (%v)", tx.Hash().Pretty())
			}
		}
	}
}

func (s *store) processMempoolTx(tx *Tx) error {
	err := s.validateTxIntrinsics(tx)
	if err != nil {
		return err
	}

	//
	// Validate the tx's extrinsics
	//
	{
		validators := make(map[Validator][]Patch)
		validatorKeypaths := make(map[Validator][]string)
		for _, patch := range tx.Patches {
			v, idx := s.resolverTree.nearestValidatorForKeypath(patch.Keys)
			keys := make([]string, len(patch.Keys)-(idx))
			copy(keys, patch.Keys[idx:])
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
		s.validTxs[tx.Hash()] = tx
	}

	// Unmark parents as leaves
	for _, parentHash := range tx.Parents {
		delete(s.leaves, parentHash)
	}

	// @@TODO: add to timeDAG

	//
	// Apply changes to the state tree
	//
	{
		for _, p := range tx.Patches {
			var patch Patch = p
			var newState interface{}
			var err error
			for {
				resolver, currentResolverKeypathStartsAt := s.resolverTree.nearestResolverForKeypath(patch.Keys[:len(patch.Keys)-1])
				thisResolverKeypath := patch.Keys[:currentResolverKeypathStartsAt]
				thisResolverState := s.stateAtKeypath(thisResolverKeypath)

				patchCopy := patch
				patchCopy.Keys = patch.Keys[len(thisResolverKeypath):]

				newState, err = resolver.ResolveState(thisResolverState, tx.From, patchCopy)
				if err != nil {
					return err
				}

				if currentResolverKeypathStartsAt == 0 {
					break
				}

				patch = Patch{Keys: thisResolverKeypath, Val: newState}
			}
			s.currentState = newState
		}

		s.mostRecentTxHash = tx.Hash()

		// Walk the tree and initialize validators and resolvers
		// @@TODO: inefficient
		// @@TODO: breaks stateful resolvers
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
	}

	err = s.persistence.AddTx(tx)
	if err != nil {
		return err
	}

	// j, err := s.StateJSON()
	// if err != nil {
	// 	return err
	// }
	// s.Infof(0, "state = %v", string(j))
	// v, _ := valueAtKeypath(s.currentState.(map[string]interface{}), []string{"shrugisland", "talk0", "messages"})
	// s.Infof(0, "state = %v", string(PrettyJSON(v)))

	return nil
}

var (
	ErrNoParentYet           = errors.New("no parent yet")
	ErrInvalidSignature      = errors.New("invalid signature")
	ErrInvalidPrivateRootKey = errors.New("invalid private root key")
	ErrDuplicateGenesis      = errors.New("already have a genesis tx")
	ErrTxMissingParents      = errors.New("tx must have parents")
)

func (s *store) validateTxIntrinsics(tx *Tx) error {
	if len(tx.Parents) == 0 {
		return ErrTxMissingParents
	} else if len(s.validTxs) > 0 && len(tx.Parents) == 1 && tx.Parents[0] == GenesisTxHash {
		return ErrDuplicateGenesis
	}

	for _, parentHash := range tx.Parents {
		if _, exists := s.validTxs[parentHash]; !exists && parentHash.Pretty() != GenesisTxHash.Pretty() {
			return errors.Wrapf(ErrNoParentYet, "tx: %v", parentHash.Pretty())
		}
	}

	if tx.IsPrivate() {
		root := tx.PrivateRootKey()
		for _, p := range tx.Patches {
			if p.Keys[0] != root {
				return ErrInvalidPrivateRootKey
			}
		}
	}

	sigPubKey, err := RecoverSigningPubkey(tx.Hash(), tx.Sig)
	if err != nil {
		return errors.Wrap(ErrInvalidSignature, err.Error())
	} else if sigPubKey.VerifySignature(tx.Hash(), tx.Sig) == false {
		return errors.WithStack(ErrInvalidSignature)
	} else if sigPubKey.Address() != tx.From {
		return errors.Wrapf(ErrInvalidSignature, "address doesn't match (%v expected, %v received)", tx.From.Hex(), sigPubKey.Address().Hex())
	}

	return nil
}

func (s *store) stateAtKeypath(keypath []string) interface{} {
	if len(keypath) == 0 {
		return s.currentState
	} else if stateMap, isMap := s.currentState.(map[string]interface{}); isMap {
		val, _ := M(stateMap).GetValue(keypath...)
		return val
	}
	return nil
}

func (s *store) RemoveTx(txHash Hash) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.txs, txHash)

	return nil
}

func (s *store) HaveTx(txHash Hash) bool {
	_, have := s.txs[txHash]
	return have
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

func (s *store) getAncestors(hashes map[Hash]bool) map[Hash]bool {
	ancestors := map[Hash]bool{}

	var mark_ancestors func(id Hash)
	mark_ancestors = func(txHash Hash) {
		if !ancestors[txHash] {
			ancestors[txHash] = true
			for parentHash := range s.timeDAG[txHash] {
				mark_ancestors(parentHash)
			}
		}
	}
	for parentHash := range hashes {
		mark_ancestors(parentHash)
	}

	return ancestors
}
