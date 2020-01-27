package redwood

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/tree"
	"github.com/brynbellomy/redwood/types"
)

type Controller interface {
	Ctx() *ctx.Context
	Start() error

	AddTx(tx *Tx)
	HaveTx(txID types.ID) bool

	State(version *types.ID) tree.Node
	Leaves() map[types.ID]struct{}
	ResolverTree() *resolverTree
	SetResolverTree(tree *resolverTree)

	OnDownloadedRef()
}

type ReceivedRefsHandler func(refs []types.Hash)
type TxProcessedHandler func(c Controller, tx *Tx, state *tree.DBNode) error

type controller struct {
	*ctx.Context

	address      types.Address
	stateURI     string
	mu           sync.RWMutex
	txs          map[types.ID]*Tx
	validTxs     map[types.ID]*Tx
	resolverTree *resolverTree
	state        *tree.DBTree
	leaves       map[types.ID]struct{}

	chMempool     chan *Tx
	mempool       []*Tx
	onTxProcessed TxProcessedHandler

	chOnDownloadedRef chan struct{}
}

func NewController(address types.Address, stateURI string, dbRootPath string, txProcessedHandler TxProcessedHandler) (Controller, error) {
	stateURIClean := strings.NewReplacer(":", "_", "/", "_").Replace(stateURI)
	state, err := tree.NewDBTree(filepath.Join(dbRootPath, stateURIClean))
	if err != nil {
		return nil, err
	}

	c := &controller{
		Context:           &ctx.Context{},
		address:           address,
		stateURI:          stateURI,
		mu:                sync.RWMutex{},
		txs:               make(map[types.ID]*Tx),
		validTxs:          make(map[types.ID]*Tx),
		resolverTree:      newResolverTree(),
		state:             state,
		leaves:            make(map[types.ID]struct{}),
		chMempool:         make(chan *Tx, 100),
		chOnDownloadedRef: make(chan struct{}),
		onTxProcessed:     txProcessedHandler,
	}
	return c, nil
}

func (c *controller) Start() error {
	return c.CtxStart(
		// on startup,
		func() error {
			c.SetLogLabel(c.address.Pretty() + " controller")

			c.resolverTree.addResolver(tree.Keypath(nil), &dumbResolver{})
			go c.mempoolLoop()

			return nil
		},
		nil,
		nil,
		// on shutdown
		func() {
			err := c.state.Close()
			if err != nil {
				c.Errorf("error closing state db: %v", err)
			}
		},
	)
}

func (c *controller) OnDownloadedRef() {
	c.chOnDownloadedRef <- struct{}{}
}

func (c *controller) State(version *types.ID) tree.Node {
	if version == nil {
		version = &types.EmptyID
	}
	return c.state.RootNode(*version)
}

func (c *controller) Leaves() map[types.ID]struct{} {
	return c.leaves
}

func (c *controller) ResolverTree() *resolverTree {
	return c.resolverTree
}

func (c *controller) SetResolverTree(tree *resolverTree) {
	c.resolverTree = tree
}

func (c *controller) AddTx(tx *Tx) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Ignore duplicates
	if _, exists := c.txs[tx.ID]; exists {
		c.Infof(0, "already know tx %v, skipping", tx.ID.Pretty())
		return
	}

	c.Infof(0, "new tx %v", tx.ID.Pretty())

	// Store the tx (so we can ignore txs we've seen before)
	c.txs[tx.ID] = tx

	c.addToMempool(tx)
}

func (c *controller) addToMempool(tx *Tx) {
	select {
	case <-c.Context.Done():
	case c.chMempool <- tx:
	}
}

func (c *controller) mempoolLoop() {
	for {
		select {
		case <-c.Context.Done():
			return
		case tx := <-c.chMempool:
			c.mempool = append(c.mempool, tx)
			c.processMempool()
		case <-c.chOnDownloadedRef:
			c.processMempool()
		}
	}
}

func (c *controller) processMempool() {
	for {
		var anySucceeded bool
		var newMempool []*Tx

		for _, tx := range c.mempool {
			err := c.processMempoolTx(tx)
			if errors.Cause(err) == ErrNoParentYet || errors.Cause(err) == ErrMissingCriticalRefs {
				c.Infof(0, "readding to mempool %v (%v)", tx.ID.Pretty(), err)
				newMempool = append(newMempool, tx)
			} else if err != nil {
				c.Errorf("invalid tx %+v: %v", err, PrettyJSON(tx))
			} else {
				anySucceeded = true
				c.Infof(0, "tx added to chain (%v)", tx.ID.Pretty())
			}
		}
		c.mempool = newMempool
		if !anySucceeded {
			return
		}
	}
}

func (c *controller) processMempoolTx(tx *Tx) error {
	err := c.validateTxIntrinsics(tx)
	if err != nil {
		return err
	}

	err = c.state.Update(types.EmptyID, func(state *tree.DBNode) error {
		state.ResetDiff()

		//
		// Validate the tx's extrinsics
		//
		{
			// @@TODO: sort patches and use ordering to cut down on number of ops

			patches := tx.Patches
			for i := len(c.resolverTree.validatorKeypaths) - 1; i >= 0; i-- {
				validatorKeypath := c.resolverTree.validatorKeypaths[i]

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
				err := c.resolverTree.validators[string(validatorKeypath)].ValidateTx(state.AtKeypath(validatorKeypath, nil), c.txs, c.validTxs, &txCopy)
				if err != nil {
					return err
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
			for i := len(c.resolverTree.resolverKeypaths) - 1; i >= 0; i-- {
				resolverKeypath := c.resolverTree.resolverKeypaths[i]

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

				stateToResolve := state.AtKeypath(resolverKeypath, nil)
				resolverConfig, err := state.CopyToMemory(resolverKeypath.Push(MergeTypeKeypath), nil)
				if err != nil && errors.Cause(err) != types.Err404 {
					return err
				}
				if resolverConfig != nil {
					state.Diff().SetEnabled(false)
					err = stateToResolve.Delete(MergeTypeKeypath, nil)
					if err != nil {
						return err
					}
					state.Diff().SetEnabled(true)
				}

				err = c.resolverTree.resolvers[string(resolverKeypath)].ResolveState(stateToResolve, tx.From, tx.ID, tx.Parents, patchesTrimmed)
				if err != nil {
					return err
				}

				if resolverConfig != nil {
					state.Diff().SetEnabled(false)
					resolverConfigVal, _, err := resolverConfig.Value(nil, nil)
					if err != nil {
						return err
					}
					err = stateToResolve.Set(MergeTypeKeypath, nil, resolverConfigVal)
					if err != nil {
						return err
					}
					state.Diff().SetEnabled(true)
				}

				patches = unprocessedPatches
			}
		}

		err = c.onTxProcessed(c, tx, state)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	if tx.Checkpoint {
		err = c.state.CopyVersion(tx.ID, types.EmptyID)
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

	// @@TODO: add to timeDAG

	//c.Warnf("state (%v) ~> %v", c.stateURI, PrettyJSON(newState))

	tx.Valid = true
	c.validTxs[tx.ID] = tx

	return nil
}

var (
	ErrNoParentYet         = errors.New("no parent yet")
	ErrMissingCriticalRefs = errors.New("missing critical refs")
	ErrInvalidSignature    = errors.New("invalid signature")
	ErrTxMissingParents    = errors.New("tx must have parents")
)

func (c *controller) validateTxIntrinsics(tx *Tx) error {
	if len(tx.Parents) == 0 && tx.ID != GenesisTxID {
		return ErrTxMissingParents
	}

	for _, parentID := range tx.Parents {
		if _, exists := c.validTxs[parentID]; !exists && parentID != GenesisTxID {
			return errors.Wrapf(ErrNoParentYet, "parent tx: %v", parentID.Hex())
		}
	}

	if tx.ID != GenesisTxID {
		sigPubKey, err := RecoverSigningPubkey(tx.Hash(), tx.Sig)
		if err != nil {
			return errors.Wrap(ErrInvalidSignature, err.Error())
		} else if sigPubKey.VerifySignature(tx.Hash(), tx.Sig) == false {
			return errors.Wrapf(ErrInvalidSignature, "cannot be verified")
		} else if sigPubKey.Address() != tx.From {
			return errors.Wrapf(ErrInvalidSignature, "address doesn't match (%v expected, %v received)", tx.From.Hex(), sigPubKey.Address().Hex())
		}
	}

	return nil
}

func (c *controller) HaveTx(txID types.ID) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, have := c.txs[txID]
	return have
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
