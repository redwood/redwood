package redwood

import (
	"sync"

	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/ctx"
)

type Controller interface {
	Ctx() *ctx.Context
	Start() error

	AddTx(tx *Tx)
	HaveTx(txID ID) bool

	State(keypath []string) (interface{}, []ID)
	MostRecentTxID() ID // @@TODO: should be .Leaves()
	ResolverTree() *resolverTree
	SetResolverTree(tree *resolverTree)

	OnDownloadedRef()
}

type RefResolver interface {
	ResolveRefs(input interface{}) (interface{}, bool, error)
}

type ReceivedRefsHandler func(refs []Hash)
type TxProcessedHandler func(c Controller, tx *Tx, state interface{}) error

type controller struct {
	*ctx.Context

	address        Address
	stateURI       string
	mu             sync.RWMutex
	txs            map[ID]*Tx
	validTxs       map[ID]*Tx
	resolverTree   *resolverTree
	currentState   interface{}
	leaves         map[ID]bool
	mostRecentTxID ID

	chMempool     chan *Tx
	mempool       []*Tx
	onTxProcessed TxProcessedHandler

	chOnDownloadedRef chan struct{}
}

func NewController(address Address, stateURI string, txProcessedHandler TxProcessedHandler) (Controller, error) {
	c := &controller{
		Context:           &ctx.Context{},
		address:           address,
		stateURI:          stateURI,
		mu:                sync.RWMutex{},
		txs:               make(map[ID]*Tx),
		validTxs:          make(map[ID]*Tx),
		resolverTree:      &resolverTree{},
		currentState:      map[string]interface{}{},
		leaves:            make(map[ID]bool),
		chMempool:         make(chan *Tx, 100),
		chOnDownloadedRef: make(chan struct{}),
		mostRecentTxID:    GenesisTxID,
		onTxProcessed:     txProcessedHandler,
	}
	return c, nil
}

func (c *controller) Start() error {
	return c.CtxStart(
		// on startup,
		func() error {
			c.SetLogLabel(c.address.Pretty() + " controller")

			c.resolverTree.addResolver([]string{}, &dumbResolver{})
			go c.mempoolLoop()

			return nil
		},
		nil,
		nil,
		// on shutdown
		func() {},
	)
}

func (c *controller) OnDownloadedRef() {
	c.chOnDownloadedRef <- struct{}{}
}

func (c *controller) State(keypath []string) (interface{}, []ID) {
	// @@TODO: mutex
	val, exists := getValue(c.currentState, keypath)
	if !exists {
		return nil, nil
	}
	leaves := []ID{c.mostRecentTxID}
	return DeepCopyJSValue(val), leaves
}

func (c *controller) MostRecentTxID() ID {
	return c.mostRecentTxID
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

	//
	// Validate the tx's extrinsics
	//
	{
		validators, validatorKeypaths, _ := c.resolverTree.groupPatchesByValidator(tx.Patches)

		for validator, patches := range validators {
			if len(patches) == 0 {
				continue
			}

			txCopy := *tx
			txCopy.Patches = patches

			c.mu.RLock()
			err := validator.ValidateTx(c.stateAtKeypath(validatorKeypaths[validator]), c.txs, c.validTxs, txCopy)
			if err != nil {
				c.mu.RUnlock()
				return err
			}
			c.mu.RUnlock()
		}
	}

	//
	// Apply changes to the state tree
	//
	var newState interface{}
	{
		var processNode func(node *resolverTreeNode, localState interface{}, patches []Patch) []Patch
		processNode = func(node *resolverTreeNode, localState interface{}, patches []Patch) []Patch {
			localStateMap, isMap := localState.(map[string]interface{})
			if !isMap {
				localStateMap = make(map[string]interface{})
			}

			newPatches := []Patch{}
			for key, child := range node.subkeys {
				// Trim patches to be relative to this child's keypath
				patchesTrimmed := make([]Patch, 0)
				for _, p := range patches {
					if len(p.Keys) > 0 && p.Keys[0] == key {
						pcopy := p.Copy()
						patchesTrimmed = append(patchesTrimmed, Patch{Keys: pcopy.Keys[1:], Range: pcopy.Range, Val: pcopy.Val})
					}
				}

				// Process the patches for the child node into (hopefully) fewer patches and then queue them up for processing at this node
				processed := processNode(child, localStateMap[key], patchesTrimmed)
				for i := range processed {
					processed[i].Keys = append([]string{key}, processed[i].Keys...)
				}

				newPatches = append(newPatches, processed...)
			}

			// Also queue up any patches that weren't the responsibility of our child nodes
			for _, p := range patches {
				if len(p.Keys) == 0 {
					newPatches = append(newPatches, p.Copy())
				} else if _, exists := node.subkeys[p.Keys[0]]; !exists {
					newPatches = append(newPatches, p.Copy())
				}
			}

			if node.resolver != nil {
				// If this is a node with a resolver, process this set of patches into a single patch for our parent
				newState, err := node.resolver.ResolveState(localStateMap, tx.From, tx.ID, tx.Parents, newPatches)
				if err != nil {
					panic(err)
				}
				return []Patch{{Keys: node.keypath, Val: newState}}
			} else {
				// If this node isn't a resolver, just return the patches our children gave us
				return newPatches
			}
		}
		finalPatches := processNode(c.resolverTree.root, c.currentState, tx.Patches)
		if len(finalPatches) != 1 {
			panic("noooo")
		}

		newState = finalPatches[0].Val
	}

	err = c.onTxProcessed(c, tx, newState)
	if err != nil {
		return err
	}

	// Unmark parents as leaves
	for _, parentID := range tx.Parents {
		delete(c.leaves, parentID)
	}

	// @@TODO: add to timeDAG
	c.mostRecentTxID = tx.ID

	// Finally, set current state
	c.currentState = newState
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

func (c *controller) stateAtKeypath(keypath []string) interface{} {
	if len(keypath) == 0 {
		return c.currentState
	} else if stateMap, isMap := c.currentState.(map[string]interface{}); isMap {
		val, _ := getValue(stateMap, keypath)
		return val
	}
	return nil
}

func (c *controller) HaveTx(txID ID) bool {
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
