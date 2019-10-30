package redwood

import (
	"encoding/json"
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
	RemoveTx(txHash Hash) error
	FetchTxs() ([]Tx, error)
	HaveTx(txHash Hash) bool

	State(keypath []string, resolveRefs bool) (interface{}, error)
	StateJSON() []byte
	MostRecentTxHash() Hash

	SetResolver(keypath []string, resolver Resolver)
	SetValidator(keypath []string, validator Validator)

	SetReceivedRefsHandler(handler ReceivedRefsHandler)
}

type ReceivedRefsHandler func(refs []Hash)

type controller struct {
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

	onReceivedRefs func(refs []Hash)

	store    Store
	refStore RefStore
}

func NewController(address Address, genesisState interface{}, store Store, refStore RefStore) (Controller, error) {
	c := &controller{
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
		store:            store,
		refStore:         refStore,
	}

	return c, nil
}

func (c *controller) Start() error {
	return c.CtxStart(
		// on startup,
		func() error {
			c.SetLogLabel(c.address.Pretty() + " controller")

			c.SetResolver([]string{}, &dumbResolver{})
			c.SetValidator([]string{}, &permissionsValidator{})

			c.CtxAddChild(c.store.Ctx(), nil)

			err := c.store.Start()
			if err != nil {
				return err
			}

			go c.mempoolLoop()

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
	val, exists := M(c.currentState.(map[string]interface{})).GetValue(keypath...)
	if !exists {
		return nil, nil
	}

	copied := DeepCopyJSValue(val)

	if resolveRefs {
		asMap, isMap := copied.(map[string]interface{})
		if isMap {
			resolved, err := c.resolveRefs(asMap)
			if err != nil {
				return nil, err
			}
			return resolved, nil
		}
	}

	return copied, nil
}

func (c *controller) resolveRefs(m map[string]interface{}) (interface{}, error) {
	type resolution struct {
		keypath []string
		val     interface{}
	}
	resolutions := []resolution{}

	err := walkTree(m, func(keypath []string, val interface{}) error {
		asMap, isMap := val.(map[string]interface{})
		if !isMap {
			return nil
		}

		link, exists := M(asMap).GetString("link")
		if !exists {
			return nil
		}

		hash, err := HashFromHex(link[len("ref:"):])
		if err != nil {
			return err
		}

		objectReader, contentType, err := c.refStore.Object(hash)
		if errors.Cause(err) == os.ErrNotExist {
			// If we don't have a given ref, we just don't fill it in
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
		return nil
	})
	if err != nil {
		return nil, err
	}

	for _, res := range resolutions {
		if len(res.keypath) > 0 {
			M(m).SetValue(res.keypath, res.val)
		} else {
			// This tends to come up when a browser is fetching individual resources that are refs
			return res.val, nil
		}
	}
	return m, nil
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

func (c *controller) MostRecentTxHash() Hash {
	return c.mostRecentTxHash
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
	if _, exists := c.txs[tx.Hash()]; exists {
		c.Infof(0, "already know tx %v, skipping", tx.Hash().String())
		return nil
	}

	c.Infof(0, "new tx %v", tx.Hash().Pretty())

	// Store the tx (so we can ignore txs we've seen before)
	c.txs[tx.Hash()] = tx

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
						c.Infof(0, "readding to mempool %v (%v)", tx.Hash(), err)
						c.addToMempool(tx)
					}
				}()
			} else if err != nil {
				c.Errorf("invalid tx %+v: %v", *tx, err)
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
		validators := make(map[Validator][]Patch)
		validatorKeypaths := make(map[Validator][]string)
		for _, patch := range tx.Patches {
			v, idx := c.resolverTree.nearestValidatorForKeypath(patch.Keys)
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

			err := validator.Validate(c.stateAtKeypath(validatorKeypaths[validator]), c.txs, c.validTxs, txCopy)
			if err != nil {
				return err
			}
		}
	}

	//
	// Check incoming patches to see if any refs will be modified (necessitating that we fetch them before updating the state tree)
	//
	{
		var resolverRefs []Hash

	CheckPatchesForRefs:
		for _, p := range tx.Patches {
			var foundResolverKey bool

			for i, key := range p.Keys {
				if key == "resolver" || key == "validator" {
					foundResolverKey = true
					continue
				}
				if foundResolverKey && key == "link" && i == len(p.Keys)-1 {
					if linkStr, isString := p.Val.(string); isString {
						hash, err := HashFromHex(linkStr[len("ref:"):])
						if err != nil {
							return err
						}
						resolverRefs = append(resolverRefs, hash)
						continue CheckPatchesForRefs
					}
				}
			}
			if foundResolverKey {
				err := walkTree(p.Val, func(keypath []string, val interface{}) error {
					linkStr, valIsString := val.(string)
					if len(keypath) > 0 && keypath[len(keypath)-1] == "link" && valIsString {
						hash, err := HashFromHex(linkStr[len("ref:"):])
						if err != nil {
							return err
						}
						resolverRefs = append(resolverRefs, hash)
					}
					return nil
				})
				if err != nil {
					return err
				}
			}
		}

		c.onReceivedRefs(resolverRefs)

		var missingRefs bool
		for _, refHash := range resolverRefs {
			if !c.refStore.HaveObject(refHash) {
				missingRefs = true
				break
			}
		}

		if missingRefs {
			return ErrMissingCriticalRefs
		}
	}

	tx.Valid = true
	c.validTxs[tx.Hash()] = tx

	//
	// Apply changes to the state tree
	//
	{
		var processNode func(node *resolverTreeNode, localState interface{}, patches []Patch) []Patch
		processNode = func(node *resolverTreeNode, localState interface{}, patches []Patch) []Patch {
			localStateMap, isMap := localState.(map[string]interface{})
			if !isMap {
				localStateMap = make(map[string]interface{})
			}
			newPatches := []Patch{}
			for key, child := range node.subkeys {
				patchesTrimmed := make([]Patch, 0)
				for _, p := range patches {
					if len(p.Keys) > 0 && p.Keys[0] == key {
						patchesTrimmed = append(patchesTrimmed, Patch{Keys: p.Keys[1:], Range: p.Range, Val: p.Val})
					}
				}
				processed := processNode(child, localStateMap[key], patchesTrimmed)
				for i := range processed {
					processed[i].Keys = append([]string{key}, processed[i].Keys...)
				}
				newPatches = append(newPatches, processed...)
			}
			for _, p := range patches {
				if len(p.Keys) == 0 {
					continue
				}
				if _, exists := node.subkeys[p.Keys[0]]; !exists {
					newPatches = append(newPatches, p)
				}
			}

			if node.resolver != nil {
				var err error
				newState, err := node.resolver.ResolveState(localStateMap, tx.From, tx.Hash(), tx.Parents, newPatches)
				if err != nil {
					panic(err)
				}
				return []Patch{{Keys: node.keypath, Val: newState}}
			} else {
				return newPatches
			}
		}
		finalPatches := processNode(c.resolverTree.root, c.currentState, tx.Patches)
		if len(finalPatches) > 1 {
			panic("noooo")
		}
		nextState := finalPatches[0].Val

		// Set current state
		c.currentState = nextState

		// Notify the Host to start fetching any refs we don't have yet
		var refs []Hash
		err = walkTree(c.currentState, func(keypath []string, val interface{}) error {
			linkStr, valIsString := val.(string)
			if len(keypath) > 0 && keypath[len(keypath)-1] == "link" && valIsString {
				hash, err := HashFromHex(linkStr[len("ref:"):])
				if err != nil {
					return err
				}
				refs = append(refs, hash)
			}
			return nil
		})
		if err != nil {
			return err
		}
		c.onReceivedRefs(refs)

		// Unmark parents as leaves
		for _, parentHash := range tx.Parents {
			delete(c.leaves, parentHash)
		}

		// @@TODO: add to timeDAG

		// msgs, _ := M(c.currentState.(map[string]interface{})).GetValue("shrugisland", "talk0", "messages")
		// c.Warnf("state ~> %v", PrettyJSON(msgs))
		// c.Warnf("controller state ~>", string(c.StateJSON()))

		c.mostRecentTxHash = tx.Hash()

		// Walk the tree and initialize validators and resolvers
		// @@TODO: inefficient
		newResolverTree := resolverTree{}
		newResolverTree.addResolver([]string{}, &dumbResolver{})
		newResolverTree.addValidator([]string{}, &permissionsValidator{})
		err = walkTree(c.currentState, func(keypath []string, val interface{}) error {
			m, isMap := val.(map[string]interface{})
			if !isMap {
				return nil
			}
			resolverConfigMap, exists := M(m).GetMap("resolver")
			if !exists {
				return nil
			}

			// Don't actually inject the refs into the state tree
			config, err := c.resolveRefs(DeepCopyJSValue(resolverConfigMap).(map[string]interface{}))
			if err != nil {
				return err
			}

			var resolverInternalState map[string]interface{}
			oldResolver, depth := c.resolverTree.nearestResolverForKeypath(keypath)
			if depth != len(keypath) {
				resolverInternalState = make(map[string]interface{})
			} else {
				resolverInternalState = oldResolver.InternalState()
			}

			resolver, err := initResolverFromConfig(config.(map[string]interface{}), resolverInternalState)
			if err != nil {
				return err
			}
			newResolverTree.addResolver(keypath, resolver)

			validatorConfig, exists := M(m).GetMap("validator")
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

	err = c.store.AddTx(tx)
	if err != nil {
		return err
	}

	// j, err := c.StateJSON()
	// if err != nil {
	// 	return err
	// }
	// c.Infof(0, "state = %v", string(j))
	// v, _ := valueAtKeypath(c.currentState.(map[string]interface{}), []string{"shrugisland", "talk0", "messages"})
	// c.Infof(0, "state = %v", string(PrettyJSON(v)))

	return nil
}

var (
	ErrNoParentYet           = errors.New("no parent yet")
	ErrMissingCriticalRefs   = errors.New("missing critical refs")
	ErrInvalidSignature      = errors.New("invalid signature")
	ErrInvalidPrivateRootKey = errors.New("invalid private root key")
	ErrDuplicateGenesis      = errors.New("already have a genesis tx")
	ErrTxMissingParents      = errors.New("tx must have parents")
)

func (c *controller) validateTxIntrinsics(tx *Tx) error {
	if len(tx.Parents) == 0 {
		return ErrTxMissingParents
	} else if len(c.validTxs) > 0 && len(tx.Parents) == 1 && tx.Parents[0] == GenesisTxHash {
		return ErrDuplicateGenesis
	}

	for _, parentHash := range tx.Parents {
		if _, exists := c.validTxs[parentHash]; !exists && parentHash.Pretty() != GenesisTxHash.Pretty() {
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
		return errors.Wrapf(ErrInvalidSignature, "cannot be verified")
	} else if sigPubKey.Address() != tx.From {
		return errors.Wrapf(ErrInvalidSignature, "address doesn't match (%v expected, %v received)", tx.From.Hex(), sigPubKey.Address().Hex())
	}

	return nil
}

func (c *controller) stateAtKeypath(keypath []string) interface{} {
	if len(keypath) == 0 {
		return c.currentState
	} else if stateMap, isMap := c.currentState.(map[string]interface{}); isMap {
		val, _ := M(stateMap).GetValue(keypath...)
		return val
	}
	return nil
}

func (c *controller) RemoveTx(txHash Hash) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.txs, txHash)

	return nil
}

func (c *controller) HaveTx(txHash Hash) bool {
	_, have := c.txs[txHash]
	return have
}

func (c *controller) FetchTxs() ([]Tx, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var txs []Tx
	for _, tx := range c.txs {
		txs = append(txs, *tx)
	}

	return txs, nil
}

func (c *controller) getAncestors(hashes map[Hash]bool) map[Hash]bool {
	ancestors := map[Hash]bool{}

	var mark_ancestors func(id Hash)
	mark_ancestors = func(txHash Hash) {
		if !ancestors[txHash] {
			ancestors[txHash] = true
			for parentHash := range c.timeDAG[txHash] {
				mark_ancestors(parentHash)
			}
		}
	}
	for parentHash := range hashes {
		mark_ancestors(parentHash)
	}

	return ancestors
}
