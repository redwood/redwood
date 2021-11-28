package tree

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"redwood.dev/blob"
	"redwood.dev/crypto"
	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/state"
	"redwood.dev/tree/nelson"
	"redwood.dev/utils"
	"redwood.dev/utils/badgerutils"
)

type Controller interface {
	process.Interface

	AddTx(tx Tx) error
	StateAtVersion(version *state.Version) state.Node
	QueryIndex(version *state.Version, keypath state.Keypath, indexName state.Keypath, queryParam state.Keypath, rng *state.Range) (state.Node, error)
	Leaves() ([]state.Version, error)
	OnNewState(fn NewStateCallback)
}

type controller struct {
	process.Process
	log.Logger

	stateURI        string
	stateDBRootPath string
	badgerOpts      badgerutils.OptsBuilder

	controllerHub ControllerHub
	txStore       TxStore
	blobStore     blob.Store

	behaviorTree *behaviorTree

	states  *state.VersionedDBTree
	indices *state.VersionedDBTree

	newStateListeners   []NewStateCallback
	newStateListenersMu sync.RWMutex

	mempool Mempool
	addTxMu sync.Mutex
}

type NewStateCallback func(tx Tx, state state.Node, leaves []state.Version)

var (
	MergeTypeKeypath = state.Keypath("Merge-Type")
	ValidatorKeypath = state.Keypath("Validator")
)

func NewController(
	stateURI string,
	stateDBRootPath string,
	badgerOpts badgerutils.OptsBuilder,
	controllerHub ControllerHub,
	txStore TxStore,
	blobStore blob.Store,
) (Controller, error) {
	c := &controller{
		Process:         *process.New("controller " + stateURI),
		Logger:          log.NewLogger("controller"),
		stateURI:        stateURI,
		stateDBRootPath: stateDBRootPath,
		badgerOpts:      badgerOpts,
		controllerHub:   controllerHub,
		txStore:         txStore,
		blobStore:       blobStore,
		behaviorTree:    newBehaviorTree(),
	}
	return c, nil
}

func (c *controller) Start() (err error) {
	defer func() {
		if err != nil {
			fmt.Println(err)
			c.Close()
		}
	}()
	err = c.Process.Start()
	if err != nil {
		return err
	}

	stateURIClean := strings.NewReplacer(":", "_", "/", "_").Replace(c.stateURI)
	states, err := state.NewVersionedDBTree(c.badgerOpts.ForPath(filepath.Join(c.stateDBRootPath, stateURIClean)))
	if err != nil {
		return err
	}
	c.states = states

	indices, err := state.NewVersionedDBTree(c.badgerOpts.ForPath(filepath.Join(c.stateDBRootPath, stateURIClean+"_indices")))
	if err != nil {
		return err
	}
	c.indices = indices

	// Add root resolver
	c.behaviorTree.addResolver(state.Keypath(nil), &dumbResolver{})

	// Start mempool
	c.mempool = NewMempool(c.processMempoolTx)
	err = c.Process.SpawnChild(context.TODO(), c.mempool)
	if err != nil {
		return err
	}

	stateURIs, err := c.txStore.KnownStateURIs()
	if err != nil {
		return err
	}

	// @@TODO: this is idiotic, fix it
	go func() {
		select {
		case <-c.Process.Done():
			return
		case <-time.After(5 * time.Second):
		}

		for _, stateURI := range stateURIs {
			iter := c.txStore.AllTxsForStateURI(stateURI, GenesisTxID)
			for {
				tx := iter.Next()
				if tx == nil {
					break
				}
				if tx.Status == TxStatusInMempool {
					c.mempool.Add(*tx)
				}
			}
		}
	}()

	// Listen for new blobs
	c.blobStore.OnBlobsSaved(c.mempool.ForceReprocess)

	return nil
}

func (c *controller) Close() error {
	if c.states != nil {
		err := c.states.Close()
		if err != nil {
			c.Errorf("error closing state db: %v", err)
		}
	}

	if c.indices != nil {
		err := c.indices.Close()
		if err != nil {
			c.Errorf("error closing index db: %v", err)
		}
	}

	return c.Process.Close()
}

func (c *controller) StateAtVersion(version *state.Version) state.Node {
	return c.states.StateAtVersion(version, false)
}

func (c *controller) Leaves() ([]state.Version, error) {
	return c.txStore.Leaves(c.stateURI)
}

func (c *controller) AddTx(tx Tx) error {
	c.addTxMu.Lock()
	defer c.addTxMu.Unlock()

	// Ignore duplicates
	exists, err := c.txStore.TxExists(tx.StateURI, tx.ID)
	if err != nil {
		return err
	} else if exists {
		c.Infof(0, "already know tx %v %v, skipping", c.stateURI, tx.ID.Pretty())
		return nil
	}

	c.Infof(0, "new tx %v %v", c.stateURI, tx.ID.Pretty())

	// Store the tx (so we can ignore txs we've seen before)
	tx.Status = TxStatusInMempool
	err = c.txStore.AddTx(tx)
	if err != nil {
		return err
	}

	c.mempool.Add(tx)
	return nil
}

func (c *controller) Mempool() *txSortedSet {
	return c.mempool.Get()
}

var (
	ErrNoParentYet          = errors.New("no parent yet")
	ErrPendingParent        = errors.New("parent pending validation")
	ErrInvalidParent        = errors.New("invalid parent")
	ErrInvalidSignature     = errors.New("invalid signature")
	ErrInvalidTx            = errors.New("invalid tx")
	ErrTxMissingParents     = errors.New("tx must have parents")
	ErrMissingCriticalBlobs = errors.New("missing critical blobs")
	ErrSenderIsNotAMember   = errors.New("tx sender is not a member of state URI")
)

func (c *controller) processMempoolTx(tx Tx) processTxOutcome {
	err := c.tryApplyTx(tx)

	if err == nil {
		c.Successf("tx added to chain (%v) %v", tx.StateURI, tx.ID.Pretty())
		node := c.states.StateAtVersion(nil, false)
		defer node.Close()
		return processTxOutcome_Succeeded
	}

	switch errors.Cause(err) {
	case ErrTxMissingParents, ErrInvalidParent, ErrInvalidSignature, ErrInvalidTx:
		c.Errorf("invalid tx %v: %+v: %v", tx.ID.Pretty(), err, utils.PrettyJSON(tx))
		return processTxOutcome_Failed

	case ErrPendingParent, ErrMissingCriticalBlobs, ErrNoParentYet:
		c.Infof(0, "readding to mempool %v (%v)", tx.ID.Pretty(), err)
		return processTxOutcome_Retry

	default:
		c.Errorf("error processing tx %v: %+v: %v", tx.ID.Pretty(), err, utils.PrettyJSON(tx))
		return processTxOutcome_Failed
	}
}

func (c *controller) tryApplyTx(tx Tx) (err error) {
	defer errors.Annotate(&err, "stateURI=%v tx=%v", tx.StateURI, tx.ID.Pretty())

	//
	// Validate the tx's intrinsics
	//
	if len(tx.Parents) == 0 && tx.ID != GenesisTxID {
		return ErrTxMissingParents
	}

	for _, parentID := range tx.Parents {
		parentTx, err := c.txStore.FetchTx(tx.StateURI, parentID)
		if errors.Cause(err) == errors.Err404 {
			return errors.Wrapf(ErrNoParentYet, "parent=%v", parentID.Pretty())
		} else if err != nil {
			return errors.Wrapf(err, "parent=%v", parentID.Pretty())
		} else if parentTx.Status == TxStatusInvalid {
			return errors.Wrapf(ErrInvalidParent, "parent=%v", parentID.Pretty())
		} else if parentTx.Status == TxStatusInMempool {
			return errors.Wrapf(ErrPendingParent, "parent=%v", parentID.Pretty())
		}
	}

	sigPubKey, err := crypto.RecoverSigningPubkey(tx.Hash(), tx.Sig)
	if err != nil {
		return errors.Wrap(ErrInvalidSignature, err.Error())
	} else if sigPubKey.VerifySignature(tx.Hash(), tx.Sig) == false {
		return ErrInvalidSignature
	} else if sigPubKey.Address() != tx.From {
		return errors.Wrapf(ErrInvalidSignature, "address doesn't match (expected=%v received=%v)", tx.From.Hex(), sigPubKey.Address().Hex())
		// } else if c.isPrivate && !c.members.Contains(sigPubKey.Address()) {
		// 	return errors.Wrapf(ErrSenderIsNotAMember, "tx=%v stateURI=%v sender=%v", tx.ID, tx.StateURI, sigPubKey.Address())
		// @@TODO
	}

	root := c.states.StateAtVersion(nil, true)
	defer root.Close()

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
						Keypath:   patch.Keypath.RelativeTo(validatorKeypath),
						Range:     patch.Range,
						ValueJSON: patch.ValueJSON,
					})
				} else {
					unprocessedPatches = append(unprocessedPatches, patch)
				}
			}

			txCopy := tx
			txCopy.Patches = patchesTrimmed

			validator := c.behaviorTree.validators[string(validatorKeypath)]
			err := validator.ValidateTx(root.NodeAt(validatorKeypath, nil), &txCopy)
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
						Keypath:   patch.Keypath.RelativeTo(resolverKeypath),
						Range:     patch.Range,
						ValueJSON: patch.ValueJSON,
					})
				} else {
					unprocessedPatches = append(unprocessedPatches, patch)
				}
			}
			if len(patchesTrimmed) == 0 {
				patches = unprocessedPatches
				continue
			}

			resolverState, err := root.CopyToMemory(resolverKeypath.Push(MergeTypeKeypath), nil)
			if err != nil && errors.Cause(err) != errors.Err404 {
				return err
			}
			validatorState, err := root.CopyToMemory(resolverKeypath.Push(ValidatorKeypath), nil)
			if err != nil && errors.Cause(err) != errors.Err404 {
				return err
			}

			stateToResolve := root.NodeAt(resolverKeypath, nil)

			stateToResolve.Diff().SetEnabled(false)
			err = root.Delete(resolverKeypath.Push(MergeTypeKeypath), nil)
			if err != nil && errors.Cause(err) != errors.Err404 {
				return err
			}
			err = root.Delete(resolverKeypath.Push(ValidatorKeypath), nil)
			if err != nil && errors.Cause(err) != errors.Err404 {
				return err
			}
			stateToResolve.Diff().SetEnabled(true)

			resolver := c.behaviorTree.resolvers[string(resolverKeypath)]
			err = resolver.ResolveState(stateToResolve, c.blobStore, tx.From, tx.ID, tx.Parents, patchesTrimmed)
			if err != nil {
				return errors.Wrapf(ErrInvalidTx, "%+v", err)
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

	c.handleNewBlobs(root)

	err = c.updateBehaviorTree(root)
	if err != nil {
		return err
	}

	err = root.Save()
	if err != nil {
		return err
	}

	if tx.Checkpoint {
		err = c.states.CopyVersion(tx.ID, state.CurrentVersion)
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

	leaves, err := c.txStore.Leaves(c.stateURI)
	if err != nil {
		return err
	}

	root = c.states.StateAtVersion(nil, false)
	defer root.Close()
	c.notifyNewStateListeners(tx, root, leaves)

	return nil
}

func (c *controller) handleNewBlobs(root state.Node) {
	var blobs []blob.ID
	defer func() {
		if len(blobs) > 0 {
			c.blobStore.MarkBlobsAsNeeded(blobs)
		}
	}()

	diff := root.Diff()

	// Find all blobs in the tree and notify the Host to start fetching them
	for kp := range diff.Added {
		keypath := state.Keypath(kp)
		parentKeypath, key := keypath.Pop()
		switch {
		case key.Equals(nelson.ValueKey):
			contentType, err := nelson.GetContentType(root.NodeAt(parentKeypath, nil))
			if err != nil && errors.Cause(err) != errors.Err404 {
				c.Errorf("error getting ref content type: %v", err)
				continue
			} else if contentType != "link" {
				continue
			}

			linkStr, _, err := root.StringValue(keypath)
			if err != nil {
				c.Errorf("error getting ref link value: %v", err)
				continue
			}
			linkType, linkValue := nelson.DetermineLinkType(linkStr)
			if linkType == nelson.LinkTypeBlob {
				var blobID blob.ID
				err := blobID.UnmarshalText([]byte(linkValue))
				if err != nil {
					c.Errorf("error unmarshaling blobID: %v", err)
					continue
				}
				blobs = append(blobs, blobID)
			}
		}
	}
}

func (c *controller) updateBehaviorTree(root state.Node) error {
	// Walk the tree and initialize validators and resolvers (@@TODO: inefficient)

	// We need to be able to roll back in case of error, so we make a copy
	newBehaviorTree := c.behaviorTree.copy()

	diff := root.Diff()

	// Remove deleted resolvers and validators
	for kp := range diff.Removed {
		parentKeypath, key := state.Keypath(kp).Pop()
		switch {
		case key.Equals(MergeTypeKeypath):
			c.behaviorTree.removeResolver(parentKeypath)
		case key.Equals(ValidatorKeypath):
			c.behaviorTree.removeValidator(parentKeypath)
		case parentKeypath.Part(-1).Equals(state.Keypath("Indices")):
			//indicesKeypath, _ := parentKeypath.Pop()
			//c.behaviorTree.removeIndexer()
		}

		for parentKeypath != nil {
			nextParentKeypath, key := parentKeypath.Pop()
			switch {
			case key.Equals(MergeTypeKeypath):
				err := c.initializeResolver(newBehaviorTree, root, parentKeypath)
				if err != nil {
					return err
				}
			case key.Equals(ValidatorKeypath):
				err := c.initializeValidator(newBehaviorTree, root, parentKeypath)
				if err != nil {
					return err
				}
			}
			parentKeypath = nextParentKeypath
		}
	}

	// Attach added resolvers and validators
	for kp := range diff.Added {
		keypath := state.Keypath(kp)
		parentKeypath, key := keypath.Pop()
		switch {
		case key.Equals(MergeTypeKeypath):
			err := c.initializeResolver(newBehaviorTree, root, keypath)
			if err != nil {
				return err
			}

		case key.Equals(ValidatorKeypath):
			err := c.initializeValidator(newBehaviorTree, root, keypath)
			if err != nil {
				return err
			}

		case key.Equals(state.Keypath("Indices")):
			err := c.initializeIndexer(newBehaviorTree, root, keypath)
			if err != nil {
				return err
			}
		}

		for parentKeypath != nil {
			nextParentKeypath, key := parentKeypath.Pop()
			switch {
			case key.Equals(MergeTypeKeypath):
				err := c.initializeResolver(newBehaviorTree, root, parentKeypath)
				if err != nil {
					return err
				}
			case key.Equals(ValidatorKeypath):
				err := c.initializeValidator(newBehaviorTree, root, parentKeypath)
				if err != nil {
					return err
				}
			}
			parentKeypath = nextParentKeypath
		}
	}
	c.behaviorTree = newBehaviorTree
	return nil
}

func (c *controller) initializeResolver(behaviorTree *behaviorTree, root state.Node, resolverConfigKeypath state.Keypath) error {
	// Resolve any blobs (to code) in the resolver config object.  We copy the config so
	// that we don't inject any blobs into the state tree itself
	config, err := root.CopyToMemory(resolverConfigKeypath, nil)
	if err != nil {
		return err
	}

	config, anyMissing, err := nelson.Resolve(config, c.controllerHub, c.blobStore)
	if err != nil {
		return err
	} else if anyMissing {
		return errors.WithStack(ErrMissingCriticalBlobs)
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
	oldResolver, oldResolverKeypath := behaviorTree.nearestResolverForKeypath(resolverConfigKeypath)
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

	behaviorTree.addResolver(resolverNodeKeypath, resolver)
	return nil
}

func debugPrint(inFormat string, args ...interface{}) {
	fmt.Printf(inFormat, args...)
}

func (c *controller) initializeValidator(behaviorTree *behaviorTree, root state.Node, validatorConfigKeypath state.Keypath) error {
	// Resolve any blobs (to code) in the validator config object.  We copy the config so
	// that we don't inject any blobs into the state tree itself
	config, err := root.CopyToMemory(validatorConfigKeypath, nil)
	if err != nil {
		return err
	}

	config, anyMissing, err := nelson.Resolve(config, c.controllerHub, c.blobStore)
	if err != nil {
		return err
	} else if anyMissing {
		return errors.WithStack(ErrMissingCriticalBlobs)
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

	behaviorTree.addValidator(validatorNodeKeypath, validator)
	return nil
}

func (c *controller) initializeIndexer(behaviorTree *behaviorTree, root state.Node, indexerConfigKeypath state.Keypath) error {
	// Resolve any blobs (to code) in the indexer config object.  We copy the config so
	// that we don't inject any blobs into the state tree itself
	indexConfigs, err := root.CopyToMemory(indexerConfigKeypath, nil)
	if err != nil {
		return err
	}

	subkeys := indexConfigs.Subkeys()

	for _, indexName := range subkeys {
		config, anyMissing, err := nelson.Resolve(indexConfigs.NodeAt(indexName, nil), c.controllerHub, c.blobStore)
		if err != nil {
			return err
		} else if anyMissing {
			return errors.WithStack(ErrMissingCriticalBlobs)
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

		behaviorTree.addIndexer(indexerNodeKeypath, indexName, indexer)
	}
	return nil
}

func (c *controller) OnNewState(fn NewStateCallback) {
	c.newStateListenersMu.Lock()
	defer c.newStateListenersMu.Unlock()
	c.newStateListeners = append(c.newStateListeners, fn)
}

func (c *controller) notifyNewStateListeners(tx Tx, root state.Node, leaves []state.Version) {
	c.newStateListenersMu.RLock()
	defer c.newStateListenersMu.RUnlock()

	var wg sync.WaitGroup
	wg.Add(len(c.newStateListeners))

	for _, handler := range c.newStateListeners {
		handler := handler
		go func() {
			defer wg.Done()
			handler(tx, root, leaves)
		}()
	}
	wg.Wait()
}

func (c *controller) QueryIndex(version *state.Version, keypath state.Keypath, indexName state.Keypath, queryParam state.Keypath, rng *state.Range) (node state.Node, err error) {
	defer errors.Annotate(&err, "keypath=%v index=%v index_arg=%v rng=%v", keypath, indexName, queryParam, rng)

	indexNode := c.indices.IndexAtVersion(version, keypath, indexName, false)

	exists, err := indexNode.Exists(queryParam)
	if err != nil {
		return nil, err

	} else if !exists {
		indexNode.Close()
		indexNode = c.indices.IndexAtVersion(version, keypath, indexName, true)

		indices, exists := c.behaviorTree.indexers[string(keypath)]
		if !exists {
			return nil, errors.Err404
		}
		indexer, exists := indices[string(indexName)]
		if !exists {
			return nil, errors.Err404
		}

		if version == nil {
			version = &state.CurrentVersion
		}

		nodeToIndex, err := c.states.StateAtVersion(version, false).NodeAt(keypath, nil).CopyToMemory(nil, nil)
		if err != nil {
			return nil, err
		}

		nodeToIndex, err = nelson.FirstNonFrameNode(nodeToIndex, 10)
		if err != nil {
			return nil, err
		}

		err = c.indices.BuildIndex(version, nodeToIndex, indexName, indexer)
		if err != nil {
			return nil, err
		}

		indexNode = c.indices.IndexAtVersion(version, keypath, indexName, false)

		exists, err = indexNode.Exists(queryParam)
		if err != nil {
			return nil, err
		} else if !exists {
			return nil, errors.Err404
		}
	}

	return indexNode.NodeAt(queryParam, rng), nil
}
