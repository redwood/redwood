package redwood

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/nelson"
	"github.com/brynbellomy/redwood/tree"
	"github.com/brynbellomy/redwood/types"
)

type Controller interface {
	Ctx() *ctx.Context
	Start() error

	AddTx(tx *Tx, force bool) error
	HaveTx(txID types.ID) (bool, error)

	StateAtVersion(version *types.ID) tree.Node
	QueryIndex(version *types.ID, keypath tree.Keypath, indexName tree.Keypath, queryParam tree.Keypath, rng *tree.Range) (tree.Node, error)
	Leaves() ([]types.ID, error)

	OnNewState(fn func(tx *Tx))
}

type controller struct {
	*ctx.Context

	address  types.Address
	stateURI string

	controllerHub ControllerHub
	txStore       TxStore
	refStore      RefStore

	behaviorTree *behaviorTree

	states  *tree.DBTree
	indices *tree.DBTree

	newStateListeners   []func(tx *Tx)
	newStateListenersMu sync.RWMutex

	mempool Mempool
	addTxMu sync.Mutex
}

func NewController(
	address types.Address,
	stateURI string,
	stateDBRootPath string,
	controllerHub ControllerHub,
	txStore TxStore,
	refStore RefStore,
) (Controller, error) {
	stateURIClean := strings.NewReplacer(":", "_", "/", "_").Replace(stateURI)
	states, err := tree.NewDBTree(filepath.Join(stateDBRootPath, stateURIClean))
	if err != nil {
		return nil, err
	}

	indices, err := tree.NewDBTree(filepath.Join(stateDBRootPath, stateURIClean+"_indices"))
	if err != nil {
		return nil, err
	}

	c := &controller{
		Context:       &ctx.Context{},
		address:       address,
		stateURI:      stateURI,
		controllerHub: controllerHub,
		txStore:       txStore,
		refStore:      refStore,
		behaviorTree:  newBehaviorTree(),
		states:        states,
		indices:       indices,
	}
	c.mempool = NewMempool(address, c.processMempoolTx)
	return c, nil
}

func (c *controller) Start() error {
	return c.CtxStart(
		// on startup,
		func() error {
			c.SetLogLabel(c.address.Pretty() + " controller")

			// Add root resolver
			c.behaviorTree.addResolver(tree.Keypath(nil), &dumbResolver{})

			// Start mempool
			c.CtxAddChild(c.mempool.Ctx(), nil)
			err := c.mempool.Start()
			if err != nil {
				return err
			}

			// Listen for new refs
			c.refStore.OnRefsSaved(c.mempool.ForceReprocess)

			return nil
		},
		nil,
		nil,
		// on shutdown
		func() {
			err := c.states.Close()
			if err != nil {
				c.Errorf("error closing state db: %v", err)
			}
			err = c.indices.Close()
			if err != nil {
				c.Errorf("error closing index db: %v", err)
			}
		},
	)
}

func (c *controller) StateAtVersion(version *types.ID) tree.Node {
	return c.states.StateAtVersion(version, false)
}

func (c *controller) Leaves() ([]types.ID, error) {
	return c.txStore.Leaves(c.stateURI)
}

func (c *controller) AddTx(tx *Tx, force bool) error {
	c.addTxMu.Lock()
	defer c.addTxMu.Unlock()

	if !force {
		// Ignore duplicates
		exists, err := c.txStore.TxExists(tx.StateURI, tx.ID)
		if err != nil {
			return err
		} else if exists {
			c.Infof(0, "already know tx %v, skipping", tx.ID.Pretty())
			return nil
		}

		c.Infof(0, "new tx %v (%v)", tx.ID.Pretty(), tx.Hash().String())
	}

	// Store the tx (so we can ignore txs we've seen before)
	tx.Status = TxStatusInMempool
	err := c.txStore.AddTx(tx)
	if err != nil {
		return err
	}

	c.mempool.Add(tx)
	return nil
}

func (c *controller) Mempool() map[types.Hash]*Tx {
	return c.mempool.Get()
}

var (
	ErrNoParentYet         = errors.New("no parent yet")
	ErrPendingParent       = errors.New("parent pending validation")
	ErrInvalidParent       = errors.New("invalid parent")
	ErrInvalidSignature    = errors.New("invalid signature")
	ErrInvalidTx           = errors.New("invalid tx")
	ErrTxMissingParents    = errors.New("tx must have parents")
	ErrMissingCriticalRefs = errors.New("missing critical refs")
)

func (c *controller) processMempoolTx(tx *Tx) processTxOutcome {
	err := c.tryApplyTx(tx)

	if err == nil {
		c.Successf("tx added to chain (%v) %v", tx.StateURI, tx.ID.Pretty())
		return processTxOutcome_Succeeded
	}

	switch errors.Cause(err) {
	case ErrTxMissingParents, ErrInvalidParent, ErrInvalidSignature, ErrInvalidTx:
		c.Errorf("invalid tx %v: %+v: %v", tx.ID.Pretty(), err, PrettyJSON(tx))
		return processTxOutcome_Failed

	case ErrPendingParent, ErrMissingCriticalRefs, ErrNoParentYet:
		c.Infof(0, "readding to mempool %v (%v)", tx.ID.Pretty(), err)
		return processTxOutcome_Retry

	default:
		c.Errorf("error processing tx %v: %+v: %v", tx.ID.Pretty(), err, PrettyJSON(tx))
		return processTxOutcome_Failed
	}
}

func (c *controller) tryApplyTx(tx *Tx) (err error) {
	defer annotate(&err, "stateURI=%v tx=%v", tx.StateURI, tx.ID.Pretty())

	//
	// Validate the tx's intrinsics
	//
	if len(tx.Parents) == 0 && tx.ID != GenesisTxID {
		return ErrTxMissingParents
	}

	for _, parentID := range tx.Parents {
		parentTx, err := c.txStore.FetchTx(tx.StateURI, parentID)
		if errors.Cause(err) == types.Err404 {
			return errors.Wrapf(ErrNoParentYet, "parent=%v", parentID.Pretty())
		} else if err != nil {
			return errors.Wrapf(err, "parent=%v", parentID.Pretty())
		} else if parentTx.Status == TxStatusInvalid {
			return errors.Wrapf(ErrInvalidParent, "parent=%v", parentID.Pretty())
		} else if parentTx.Status == TxStatusInMempool {
			return errors.Wrapf(ErrPendingParent, "parent=%v", parentID.Pretty())
		}
	}

	sigPubKey, err := RecoverSigningPubkey(tx.Hash(), tx.Sig)
	if err != nil {
		return errors.Wrap(ErrInvalidSignature, err.Error())
	} else if sigPubKey.VerifySignature(tx.Hash(), tx.Sig) == false {
		return ErrInvalidSignature
	} else if sigPubKey.Address() != tx.From {
		return errors.Wrapf(ErrInvalidSignature, "address doesn't match (expected=%v received=%v)", tx.From.Hex(), sigPubKey.Address().Hex())
	}

	state := c.states.StateAtVersion(nil, true)
	defer state.Close()

	//
	// Validate the tx's extrinsics
	//
	{
		// @@TODO: sort patches and use ordering to cut down on number of ops

		patches := tx.Patches
		for i := len(c.behaviorTree.validatorKeypaths) - 1; i >= 0; i-- {
			validatorKeypath := c.behaviorTree.validatorKeypaths[i]

			var unprocessedPatches []Patch
			var patchesTrimmed []Patch
			for _, patch := range patches {
				if patch.Keypath.StartsWith(validatorKeypath) {
					patchesTrimmed = append(patchesTrimmed, Patch{
						Keypath: patch.Keypath.RelativeTo(validatorKeypath),
						Range:   patch.Range,
						Val:     patch.Val,
					})
				} else {
					unprocessedPatches = append(unprocessedPatches, patch)
				}
			}

			txCopy := *tx
			txCopy.Patches = patchesTrimmed

			validator := c.behaviorTree.validators[string(validatorKeypath)]
			err := validator.ValidateTx(state.NodeAt(validatorKeypath, nil), &txCopy)
			if err != nil {
				// Mark the tx invalid and save it to the DB
				tx.Status = TxStatusInvalid
				err2 := c.txStore.AddTx(tx)
				if err2 != nil {
					return err2
				}
				return errors.Wrap(ErrInvalidTx, err.Error())
			}

			patches = unprocessedPatches
		}
	}

	//
	// Apply changes to the state tree
	//
	{
		// @@TODO: sort patches and use ordering to cut down on number of ops

		patches := tx.Patches
		for i := len(c.behaviorTree.resolverKeypaths) - 1; i >= 0; i-- {
			resolverKeypath := c.behaviorTree.resolverKeypaths[i]

			var unprocessedPatches []Patch
			var patchesTrimmed []Patch
			for _, patch := range patches {
				if patch.Keypath.StartsWith(resolverKeypath) {
					patchesTrimmed = append(patchesTrimmed, Patch{
						Keypath: patch.Keypath.RelativeTo(resolverKeypath),
						Range:   patch.Range,
						Val:     patch.Val,
					})
				} else {
					unprocessedPatches = append(unprocessedPatches, patch)
				}
			}
			if len(patchesTrimmed) == 0 {
				patches = unprocessedPatches
				continue
			}

			resolverState, err := state.CopyToMemory(resolverKeypath.Push(MergeTypeKeypath), nil)
			if err != nil && errors.Cause(err) != types.Err404 {
				return err
			}
			validatorState, err := state.CopyToMemory(resolverKeypath.Push(ValidatorKeypath), nil)
			if err != nil && errors.Cause(err) != types.Err404 {
				return err
			}

			stateToResolve := state.NodeAt(resolverKeypath, nil)

			stateToResolve.Diff().SetEnabled(false)
			err = state.Delete(resolverKeypath.Push(MergeTypeKeypath), nil)
			if err != nil && errors.Cause(err) != types.Err404 {
				return err
			}
			err = state.Delete(resolverKeypath.Push(ValidatorKeypath), nil)
			if err != nil && errors.Cause(err) != types.Err404 {
				return err
			}
			stateToResolve.Diff().SetEnabled(true)

			resolver := c.behaviorTree.resolvers[string(resolverKeypath)]
			err = resolver.ResolveState(stateToResolve, c.refStore, tx.From, tx.ID, tx.Parents, patchesTrimmed)
			if err != nil {
				return errors.Wrap(ErrInvalidTx, err.Error())
			}

			stateToResolve.Diff().SetEnabled(false)
			if resolverState != nil {
				err = stateToResolve.Set(MergeTypeKeypath, nil, resolverState)
				if err != nil {
					return err
				}
			}
			if validatorState != nil {
				err = stateToResolve.Set(ValidatorKeypath, nil, validatorState)
				if err != nil {
					return err
				}
			}
			stateToResolve.Diff().SetEnabled(true)

			patches = unprocessedPatches
		}
	}

	c.handleNewRefs(state)

	err = c.updateBehaviorTree(state)
	if err != nil {
		return err
	}

	err = state.Save()
	if err != nil {
		return err
	}

	if tx.Checkpoint {
		err = c.states.CopyVersion(tx.ID, tree.CurrentVersion)
		if err != nil {
			return err
		}
	}

	// Unmark parents as leaves
	for _, parentID := range tx.Parents {
		err := c.txStore.UnmarkLeaf(c.stateURI, parentID)
		if err != nil {
			return err
		}
	}

	// Mark this tx as a leaf
	err = c.txStore.MarkLeaf(c.stateURI, tx.ID)
	if err != nil {
		return err
	}

	// Mark the tx valid and save it to the DB
	tx.Status = TxStatusValid
	err = c.txStore.AddTx(tx)
	if err != nil {
		return err
	}

	c.notifyNewStateListeners(tx)

	return nil
}

func (c *controller) handleNewRefs(state tree.Node) {
	var refs []types.RefID
	defer func() {
		if len(refs) > 0 {
			c.refStore.MarkRefsAsNeeded(refs)
		}
	}()

	diff := state.Diff()

	// Find all refs in the tree and notify the Host to start fetching them
	for kp := range diff.Added {
		keypath := tree.Keypath(kp)
		parentKeypath, key := keypath.Pop()
		switch {
		case key.Equals(nelson.ValueKey):
			contentType, err := nelson.GetContentType(state.NodeAt(parentKeypath, nil))
			if err != nil && errors.Cause(err) != types.Err404 {
				c.Errorf("error getting ref content type: %v", err)
				continue
			} else if contentType != "link" {
				continue
			}

			linkStr, _, err := state.StringValue(keypath)
			if err != nil {
				c.Errorf("error getting ref link value: %v", err)
				continue
			}
			linkType, linkValue := nelson.DetermineLinkType(linkStr)
			if linkType == nelson.LinkTypeRef {
				var refID types.RefID
				err := refID.UnmarshalText([]byte(linkValue))
				if err != nil {
					c.Errorf("error unmarshaling refID: %v", err)
					continue
				}
				refs = append(refs, refID)
			}
		}
	}
}

func (c *controller) updateBehaviorTree(state tree.Node) error {
	// Walk the tree and initialize validators and resolvers
	// @@TODO: inefficient
	newBehaviorTree := newBehaviorTree()
	newBehaviorTree.addResolver(nil, &dumbResolver{})

	diff := state.Diff()

	// Remove deleted resolvers and validators
	for kp := range diff.Removed {
		parentKeypath, key := tree.Keypath(kp).Pop()
		switch {
		case key.Equals(MergeTypeKeypath):
			c.behaviorTree.removeResolver(parentKeypath)
		case key.Equals(ValidatorKeypath):
			c.behaviorTree.removeValidator(parentKeypath)
		case parentKeypath.Part(-1).Equals(tree.Keypath("Indices")):
			//indicesKeypath, _ := parentKeypath.Pop()
			//c.behaviorTree.removeIndexer()
		}

		for parentKeypath != nil {
			nextParentKeypath, key := parentKeypath.Pop()
			switch {
			case key.Equals(MergeTypeKeypath):
				err := c.initializeResolver(state, parentKeypath)
				if err != nil {
					return err
				}
			case key.Equals(ValidatorKeypath):
				err := c.initializeValidator(state, parentKeypath)
				if err != nil {
					return err
				}
			}
			parentKeypath = nextParentKeypath
		}
	}

	// Attach added resolvers and validators
	for kp := range diff.Added {
		keypath := tree.Keypath(kp)
		parentKeypath, key := keypath.Pop()
		switch {
		case key.Equals(MergeTypeKeypath):
			err := c.initializeResolver(state, keypath)
			if err != nil {
				return err
			}

		case key.Equals(ValidatorKeypath):
			err := c.initializeValidator(state, keypath)
			if err != nil {
				return err
			}

		case key.Equals(tree.Keypath("Indices")):
			err := c.initializeIndexer(state, keypath)
			if err != nil {
				return err
			}
		}

		for parentKeypath != nil {
			nextParentKeypath, key := parentKeypath.Pop()
			switch {
			case key.Equals(MergeTypeKeypath):
				err := c.initializeResolver(state, parentKeypath)
				if err != nil {
					return err
				}
			case key.Equals(ValidatorKeypath):
				err := c.initializeValidator(state, parentKeypath)
				if err != nil {
					return err
				}
			}
			parentKeypath = nextParentKeypath
		}
	}
	return nil
}

func (c *controller) initializeResolver(state tree.Node, resolverConfigKeypath tree.Keypath) error {
	// Resolve any refs (to code) in the resolver config object.  We copy the config so
	// that we don't inject any refs into the state tree itself
	config, err := state.CopyToMemory(resolverConfigKeypath, nil)
	if err != nil {
		return err
	}

	config, anyMissing, err := nelson.Resolve(config, c.controllerHub)
	if err != nil {
		return err
	} else if anyMissing {
		return errors.WithStack(ErrMissingCriticalRefs)
	}

	contentType, err := nelson.GetContentType(config)
	if err != nil {
		return err
	} else if contentType == "" {
		return errors.New("cannot initialize resolver without a 'Content-Type' key")
	}

	ctor, exists := resolverRegistry[contentType]
	if !exists {
		return errors.Errorf("unknown resolver type '%v'", contentType)
	}

	// @@TODO: if the resolver type changes, this totally breaks everything
	var internalState map[string]interface{}
	oldResolver, oldResolverKeypath := c.behaviorTree.nearestResolverForKeypath(resolverConfigKeypath)
	if !oldResolverKeypath.Equals(resolverConfigKeypath) {
		internalState = make(map[string]interface{})
	} else {
		internalState = oldResolver.InternalState()
	}

	resolver, err := ctor(config, internalState)
	if err != nil {
		return err
	}

	resolverNodeKeypath, _ := resolverConfigKeypath.Pop()

	c.behaviorTree.addResolver(resolverNodeKeypath, resolver)
	return nil
}

func debugPrint(inFormat string, args ...interface{}) {
	fmt.Printf(inFormat, args...)
}

func (c *controller) initializeValidator(state tree.Node, validatorConfigKeypath tree.Keypath) error {
	// Resolve any refs (to code) in the validator config object.  We copy the config so
	// that we don't inject any refs into the state tree itself
	config, err := state.CopyToMemory(validatorConfigKeypath, nil)
	if err != nil {
		return err
	}

	config, anyMissing, err := nelson.Resolve(config, c.controllerHub)
	if err != nil {
		return err
	} else if anyMissing {
		return errors.WithStack(ErrMissingCriticalRefs)
	}

	contentType, err := nelson.GetContentType(config)
	if err != nil {
		return err
	} else if contentType == "" {
		return errors.New("cannot initialize validator without a 'Content-Type' key")
	}

	ctor, exists := validatorRegistry[contentType]
	if !exists {
		return errors.Errorf("unknown validator type '%v'", contentType)
	}

	validator, err := ctor(config)
	if err != nil {
		return err
	}

	validatorNodeKeypath, _ := validatorConfigKeypath.Pop()

	c.behaviorTree.addValidator(validatorNodeKeypath, validator)
	return nil
}

func (c *controller) initializeIndexer(state tree.Node, indexerConfigKeypath tree.Keypath) error {
	// Resolve any refs (to code) in the indexer config object.  We copy the config so
	// that we don't inject any refs into the state tree itself
	indexConfigs, err := state.CopyToMemory(indexerConfigKeypath, nil)
	if err != nil {
		return err
	}

	subkeys := indexConfigs.Subkeys()

	for _, indexName := range subkeys {
		config, anyMissing, err := nelson.Resolve(indexConfigs.NodeAt(indexName, nil), c.controllerHub)
		if err != nil {
			return err
		} else if anyMissing {
			return errors.WithStack(ErrMissingCriticalRefs)
		}

		contentType, err := nelson.GetContentType(config)
		if err != nil {
			return err
		} else if contentType == "" {
			return errors.New("cannot initialize indexer without a 'Content-Type' key")
		}

		ctor, exists := indexerRegistry[contentType]
		if !exists {
			return errors.Errorf("unknown indexer type '%v'", contentType)
		}

		indexer, err := ctor(config)
		if err != nil {
			return err
		}

		indexerNodeKeypath, _ := indexerConfigKeypath.Pop()

		c.behaviorTree.addIndexer(indexerNodeKeypath, indexName, indexer)
	}
	return nil
}

func (c *controller) OnNewState(fn func(tx *Tx)) {
	c.newStateListenersMu.Lock()
	defer c.newStateListenersMu.Unlock()
	c.newStateListeners = append(c.newStateListeners, fn)
}

func (c *controller) notifyNewStateListeners(tx *Tx) {
	c.newStateListenersMu.RLock()
	defer c.newStateListenersMu.RUnlock()

	var wg sync.WaitGroup
	wg.Add(len(c.newStateListeners))

	for _, handler := range c.newStateListeners {
		handler := handler
		go func() {
			defer wg.Done()
			handler(tx)
		}()
	}
	wg.Wait()
}

func (c *controller) HaveTx(txID types.ID) (bool, error) {
	return c.txStore.TxExists(c.stateURI, txID)
}

func (c *controller) QueryIndex(version *types.ID, keypath tree.Keypath, indexName tree.Keypath, queryParam tree.Keypath, rng *tree.Range) (node tree.Node, err error) {
	defer annotate(&err, "keypath=%v index=%v index_arg=%v rng=%v", keypath, indexName, queryParam, rng)

	indexNode := c.indices.IndexAtVersion(version, keypath, indexName, false)

	exists, err := indexNode.Exists(queryParam)
	if err != nil {
		return nil, err

	} else if !exists {
		indexNode.Close()
		indexNode = c.indices.IndexAtVersion(version, keypath, indexName, true)

		indices, exists := c.behaviorTree.indexers[string(keypath)]
		if !exists {
			return nil, types.Err404
		}
		indexer, exists := indices[string(indexName)]
		if !exists {
			return nil, types.Err404
		}

		if version == nil {
			version = &tree.CurrentVersion
		}

		nodeToIndex, err := c.states.StateAtVersion(version, false).NodeAt(keypath, nil).CopyToMemory(nil, nil)
		if err != nil {
			return nil, err
		}

		nodeToIndex, relKeypath, err := nelson.Unwrap(nodeToIndex)
		if err != nil {
			return nil, err
		}

		err = c.indices.BuildIndex(version, relKeypath, nodeToIndex, indexName, indexer)
		if err != nil {
			return nil, err
		}

		indexNode = c.indices.IndexAtVersion(version, keypath, indexName, false)

		exists, err = indexNode.Exists(queryParam)
		if err != nil {
			return nil, err
		} else if !exists {
			return nil, types.Err404
		}
	}

	return indexNode.NodeAt(queryParam, rng), nil
}
