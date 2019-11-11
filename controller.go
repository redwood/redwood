package redwood

import (
	"encoding/json"
	goerrors "errors"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/ctx"
)

type Controller interface {
	Ctx() *ctx.Context
	Start() error

	AddTx(tx *Tx) error
	FetchTxs() TxIterator
	HaveTx(txID ID) bool

	State(keypath []string, resolveRefs bool) (interface{}, error)
	MostRecentTxID() ID // @@TODO: should be .Leaves()

	SetResolver(keypath []string, resolver Resolver)
	SetValidator(keypath []string, validator Validator)

	SetReceivedRefsHandler(handler ReceivedRefsHandler)
}

type ReceivedRefsHandler func(refs []Hash)

type controller struct {
	*ctx.Context

	address        Address
	mu             sync.RWMutex
	txs            map[ID]*Tx
	validTxs       map[ID]*Tx
	resolverTree   resolverTree
	currentState   interface{}
	genesisState   interface{}
	leaves         map[ID]bool
	chMempool      chan *Tx
	mostRecentTxID ID

	onReceivedRefs func(refs []Hash)

	store    Store
	refStore RefStore
}

func NewController(address Address, genesisState interface{}, store Store, refStore RefStore) (Controller, error) {
	c := &controller{
		Context:        &ctx.Context{},
		address:        address,
		mu:             sync.RWMutex{},
		txs:            make(map[ID]*Tx),
		validTxs:       make(map[ID]*Tx),
		resolverTree:   resolverTree{},
		genesisState:   genesisState,
		currentState:   map[string]interface{}{},
		leaves:         make(map[ID]bool),
		chMempool:      make(chan *Tx, 100),
		mostRecentTxID: GenesisTxID,
		store:          store,
		refStore:       refStore,
	}

	return c, nil
}

func (c *controller) Start() error {
	return c.CtxStart(
		// on startup,
		func() error {
			c.SetLogLabel(c.address.Pretty() + " controller")

			c.SetResolver([]string{}, &dumbResolver{})
			// c.SetValidator([]string{}, &permissionsValidator{})

			c.CtxAddChild(c.store.Ctx(), nil)

			err := c.store.Start()
			if err != nil {
				return err
			}

			go c.mempoolLoop()

			err = c.AddTx(&Tx{
				ID:      GenesisTxID,
				Parents: []ID{},
				Patches: []Patch{{Val: c.genesisState}},
			})
			if err != nil {
				return err
			}

			err = c.replayStoredTxs()
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

func (c *controller) SetReceivedRefsHandler(handler ReceivedRefsHandler) {
	c.onReceivedRefs = handler
}

func (c *controller) State(keypath []string, resolveRefs bool) (interface{}, error) {
	val, exists := getValue(c.currentState, keypath)
	if !exists {
		return nil, nil
	}
	copied := DeepCopyJSValue(val)

	if resolveRefs {
		asMap, isMap := copied.(map[string]interface{})
		if isMap {
			resolved, _, err := c.resolveRefs(asMap)
			if err != nil {
				return nil, err
			}
			return resolved, nil
		}
	}
	return copied, nil
}

func (c *controller) resolveRefs(input interface{}) (interface{}, bool, error) {
	type resolution struct {
		keypath []string
		val     interface{}
	}
	resolutions := []resolution{}

	var anyMissing bool
	err := walkTree(input, func(keypath []string, val interface{}) error {
		link, exists := getString(val, []string{"link"})
		if !exists {
			return nil
		}

		linkType := DetermineLinkType(link)
		if linkType == LinkTypeRef {
			hash, err := HashFromHex(link[len("ref:"):])
			if err != nil {
				return err
			}

			objectReader, contentType, err := c.refStore.Object(hash)
			if goerrors.Is(err, os.ErrNotExist) {
				// If we don't have a given ref, we leave it as-is, but inform the caller.
				anyMissing = true
				return nil
			} else if err != nil {
				return err
			}
			defer objectReader.Close()

			bs, err := ioutil.ReadAll(objectReader)
			if err != nil {
				return err
			}

			switch {
			case contentType == "application/json",
				contentType == "application/js",
				contentType[:5] == "text/":
				resolutions = append(resolutions, resolution{keypath, string(bs)})

			case contentType[:6] == "image/":
				resolutions = append(resolutions, resolution{keypath, bs})

			default:
				panic("unknown content type")
			}

		} else if linkType == LinkTypePath {
			otherKeypath := strings.Split(link[1:], "/")
			val, err := c.State(otherKeypath, true)
			if err != nil {
				return err
			}
			resolutions = append(resolutions, resolution{keypath, val})
		}

		return nil
	})
	if err != nil {
		return nil, anyMissing, err
	}

	for _, res := range resolutions {
		if len(res.keypath) > 0 {
			setValueAtKeypath(input, res.keypath, res.val, true)
		} else {
			// This tends to come up when a browser is fetching individual resources that are refs
			return res.val, anyMissing, nil
		}
	}
	return input, anyMissing, nil
}

func (c *controller) StateJSON() []byte {
	bs, err := json.MarshalIndent(c.currentState, "", "    ")
	if err != nil {
		panic(err)
	}
	str := string(bs)
	str = strings.Replace(str, "\\n", "\n", -1)
	return []byte(str)
}

func (c *controller) MostRecentTxID() ID {
	return c.mostRecentTxID
}

func (c *controller) SetResolver(keypath []string, resolver Resolver) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.resolverTree.addResolver(keypath, resolver)
}

func (c *controller) SetValidator(keypath []string, validator Validator) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.resolverTree.addValidator(keypath, validator)
}

func (c *controller) AddTx(tx *Tx) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Ignore duplicates
	if _, exists := c.txs[tx.ID]; exists {
		c.Infof(0, "already know tx %v, skipping", tx.Hash().String())
		return nil
	}

	c.Infof(0, "new tx %v", tx.Hash().Pretty())

	// Store the tx (so we can ignore txs we've seen before)
	c.txs[tx.ID] = tx

	c.addToMempool(tx)
	return nil
}

func (c *controller) replayStoredTxs() error {
	iter := c.store.AllTxs()
	defer iter.Cancel()

	for {
		tx := iter.Next()
		if iter.Error() != nil {
			return iter.Error()
		} else if tx == nil {
			return nil
		}

		c.Infof(0, "found stored tx %v", tx.Hash())
		err := c.AddTx(tx)
		if err != nil {
			return err
		}
	}
	return nil
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
			err := c.processMempoolTx(tx)
			if errors.Cause(err) == ErrNoParentYet || errors.Cause(err) == ErrMissingCriticalRefs {
				go func() {
					select {
					case <-c.Context.Done():
					case <-time.After(500 * time.Millisecond):
						c.Infof(0, "readding to mempool %v (%v)", tx.ID.Pretty(), err)
						c.addToMempool(tx)
					}
				}()
			} else if err != nil {
				c.Errorf("invalid tx %+v: %+v", *tx, err)
			} else {
				c.Infof(0, "tx added to chain (%v)", tx.Hash().Pretty())
			}
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

			err := validator.ValidateTx(c.stateAtKeypath(validatorKeypaths[validator]), c.txs, c.validTxs, txCopy)
			if err != nil {
				return err
			}
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

		// Notify the Host to start fetching any refs we don't have yet
		var refs []Hash
		err = walkTree(newState, func(keypath []string, val interface{}) error {
			linkStr, valIsString := val.(string)
			if len(keypath) > 0 && keypath[len(keypath)-1] == "link" && valIsString {
				if DetermineLinkType(linkStr) == LinkTypeRef {
					hash, err := HashFromHex(linkStr[len("ref:"):])
					if err != nil {
						return err
					}
					refs = append(refs, hash)
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		c.onReceivedRefs(refs)

		// Unmark parents as leaves
		for _, parentID := range tx.Parents {
			delete(c.leaves, parentID)
		}

		// @@TODO: add to timeDAG
		c.mostRecentTxID = tx.ID

		// Walk the tree and initialize validators and resolvers
		// @@TODO: inefficient
		newResolverTree := resolverTree{}
		newResolverTree.addResolver([]string{}, &dumbResolver{})
		newResolverTree.addValidator([]string{}, &permissionsValidator{})
		err = walkTree(newState, func(keypath []string, val interface{}) error {
			resolverConfigMap, exists := getMap(val, []string{"resolver"})
			if !exists {
				return nil
			}

			// Resolve any refs (to code) in the resolver config object.  We deep copy the config so
			// that we don't inject any refs into the state tree itself
			config, anyMissing, err := c.resolveRefs(DeepCopyJSValue(resolverConfigMap).(map[string]interface{}))
			if err != nil {
				return err
			} else if anyMissing {
				return ErrMissingCriticalRefs
			}

			var resolverInternalState map[string]interface{}
			oldResolverNode, depth := c.resolverTree.nearestResolverNodeForKeypath(keypath)
			if depth != len(keypath) {
				resolverInternalState = make(map[string]interface{})
			} else {
				resolverInternalState = oldResolverNode.resolver.InternalState()
			}

			resolver, err := initResolverFromConfig(config.(map[string]interface{}), resolverInternalState)
			if err != nil {
				return err
			}
			newResolverTree.addResolver(keypath, resolver)

			validatorConfig, exists := getMap(val, []string{"validator"})
			if !exists {
				return nil
			}
			validator, err := initValidatorFromConfig(validatorConfig)
			if err != nil {
				return err
			}
			newResolverTree.addValidator(keypath, validator)

			return nil
		})
		if err != nil {
			return err
		}
		c.resolverTree = newResolverTree
	}

	// Finally, set current state
	c.currentState = newState
	// c.Warnf("state ~> %v", PrettyJSON(newState))

	tx.Valid = true
	c.validTxs[tx.ID] = tx

	err = c.store.AddTx(tx)
	if err != nil {
		return err
	}

	// j, err := c.StateJSON()
	// if err != nil {
	// 	return err
	// }
	// c.Infof(0, "state = %v", string(j))
	// v, _ := getValue(c.currentState.(map[string]interface{}), []string{"shrugisland", "talk0", "messages"})
	// c.Infof(0, "state = %v", string(PrettyJSON(v)))

	return nil
}

var (
	ErrNoParentYet           = errors.New("no parent yet")
	ErrMissingCriticalRefs   = errors.New("missing critical refs")
	ErrInvalidSignature      = errors.New("invalid signature")
	ErrInvalidPrivateRootKey = errors.New("invalid private root key")
	ErrTxMissingParents      = errors.New("tx must have parents")
)

func (c *controller) validateTxIntrinsics(tx *Tx) error {
	if len(tx.Parents) == 0 && tx.ID != GenesisTxID {
		return ErrTxMissingParents
	}

	for _, parentID := range tx.Parents {
		if _, exists := c.validTxs[parentID]; !exists && parentID != GenesisTxID {
			return errors.Wrapf(ErrNoParentYet, "tx: %v", parentID.Pretty())
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
	_, have := c.txs[txID]
	return have
}

func (c *controller) FetchTxs() TxIterator {
	return c.store.AllTxs()
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
