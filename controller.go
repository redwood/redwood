package redwood

import (
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

	AddTx(tx *Tx) error
	HaveTx(txID types.ID) (bool, error)

	StateAtVersion(version *types.ID) tree.Node
	QueryIndex(version *types.ID, keypath tree.Keypath, indexName tree.Keypath, queryParam tree.Keypath, rng *tree.Range) (tree.Node, error)
	Leaves() map[types.ID]struct{}
	BehaviorTree() *behaviorTree
	SetBehaviorTree(tree *behaviorTree)

	OnDownloadedRef()
}

type ReceivedRefsHandler func(refs []types.RefID)
type TxProcessedHandler func(c Controller, tx *Tx, state *tree.DBNode) error

type controller struct {
	*ctx.Context

	address  types.Address
	stateURI string

	txStore  TxStore
	refStore RefStore

	behaviorTree *behaviorTree

	states  *tree.DBTree
	indices *tree.DBTree
	leaves  map[types.ID]struct{}

	mempool       Mempool
	onTxProcessed TxProcessedHandler
	addTxMu       sync.Mutex
}

func NewController(
	address types.Address,
	stateURI string,
	stateDBRootPath string,
	txStore TxStore,
	refStore RefStore,
	txProcessedHandler TxProcessedHandler,
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
		txStore:       txStore,
		refStore:      refStore,
		behaviorTree:  newBehaviorTree(),
		states:        states,
		indices:       indices,
		leaves:        make(map[types.ID]struct{}),
		onTxProcessed: txProcessedHandler,
	}
	c.mempool = NewMempool(address, c.processMempoolTx)
	return c, nil
}

func (c *controller) Start() error {
	return c.CtxStart(
		// on startup,
		func() error {
			c.SetLogLabel(c.address.Pretty() + " controller")

			c.behaviorTree.addResolver(tree.Keypath(nil), &dumbResolver{})

			c.CtxAddChild(c.mempool.Ctx(), nil)
			err := c.mempool.Start()
			if err != nil {
				return err
			}

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

func (c *controller) OnDownloadedRef() {
	c.mempool.ForceReprocess()
}

func (c *controller) StateAtVersion(version *types.ID) tree.Node {
	return c.states.StateAtVersion(version, false)
}

func (c *controller) Leaves() map[types.ID]struct{} {
	return c.leaves
}

func (c *controller) BehaviorTree() *behaviorTree {
	return c.behaviorTree
}

func (c *controller) SetBehaviorTree(tree *behaviorTree) {
	c.behaviorTree = tree
}

func (c *controller) AddTx(tx *Tx) error {
	c.addTxMu.Lock()
	defer c.addTxMu.Unlock()

	// Ignore duplicates
	exists, err := c.txStore.TxExists(tx.StateURI, tx.ID)
	if err != nil {
		return err
	} else if exists {
		c.Infof(0, "already know tx %v, skipping", tx.ID.Pretty())
		return nil
	}

	c.Infof(0, "new tx %v (%v)", tx.ID.Pretty(), tx.Hash().String())

	// Store the tx (so we can ignore txs we've seen before)
	tx.Status = TxStatusInMempool
	err = c.txStore.AddTx(tx)
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

			resolver := c.behaviorTree.resolvers[string(resolverKeypath)]
			err = resolver.ResolveState(state.NodeAt(resolverKeypath, nil), c.refStore, tx.From, tx.ID, tx.Parents, patchesTrimmed)
			if err != nil {
				return errors.Wrap(ErrInvalidTx, err.Error())
			}

			patches = unprocessedPatches
		}
	}

	err = c.onTxProcessed(c, tx, state)
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
		delete(c.leaves, parentID)
	}

	// Mark this tx as a leaf
	c.leaves[tx.ID] = struct{}{}

	// Mark the tx valid and save it to the DB
	tx.Status = TxStatusValid
	err = c.txStore.AddTx(tx)
	if err != nil {
		return err
	}
	return nil
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

//func (c *controller) getAncestors(hashes map[Hash]bool) map[Hash]bool {
//    ancestors := map[Hash]bool{}
//
//    var mark_ancestors func(id Hash)
//    mark_ancestors = func(txHash Hash) {
//        if !ancestors[txHash] {
//            ancestors[txHash] = true
//            for parentHash := range c.timeDAG[txHash] {
//                mark_ancestors(parentHash)
//            }
//        }
//    }
//    for parentHash := range hashes {
//        mark_ancestors(parentHash)
//    }
//
//    return ancestors
//}
