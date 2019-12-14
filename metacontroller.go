package redwood

import (
	"strings"
	"sync"

	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/ctx"
)

type Metacontroller interface {
	Ctx() *ctx.Context
	Start() error

	AddTx(tx *Tx) error
	FetchTx(stateURI string, txID ID) (*Tx, error)
	FetchTxs(stateURI string) TxIterator
	HaveTx(stateURI string, txID ID) bool

	KnownStateURIs() []string
	State(stateURI string, keypath []string, version *ID) (interface{}, []ID, error)
	MostRecentTxID(stateURI string) (ID, error) // @@TODO: should be .Leaves()

	RefResolver
	SetReceivedRefsHandler(handler ReceivedRefsHandler)
	OnDownloadedRef()

	DebugLockResolvers()
}

type metacontroller struct {
	*ctx.Context

	address Address

	controllers         map[string]Controller
	controllersMu       sync.RWMutex
	receivedRefsHandler ReceivedRefsHandler
	store               Store
	refStore            RefStore

	resolversLocked bool

	validStateURIs   map[string]struct{}
	validStateURIsMu sync.Mutex
}

var (
	ErrNoController = errors.New("no controller for that stateURI")
)

func NewMetacontroller(address Address, store Store, refStore RefStore) Metacontroller {
	return &metacontroller{
		Context:        &ctx.Context{},
		address:        address,
		controllers:    make(map[string]Controller),
		store:          store,
		refStore:       refStore,
		validStateURIs: make(map[string]struct{}),
	}
}

func (m *metacontroller) Start() error {
	return m.CtxStart(
		// on startup
		func() error {
			m.SetLogLabel(m.address.Pretty() + " metacontroller")

			m.CtxAddChild(m.store.Ctx(), nil)

			err := m.store.Start()
			if err != nil {
				return err
			}

			//err = m.AddTx(&Tx{
			//    ID:      GenesisTxID,
			//    Parents: []ID{},
			//    Patches: []Patch{{Val: c.genesisState}},
			//})
			//if err != nil {
			//    return err
			//}

			err = m.replayStoredTxs()
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

func (m *metacontroller) replayStoredTxs() error {
	iter := m.store.AllTxs()
	defer iter.Cancel()

	for {
		tx := iter.Next()
		if iter.Error() != nil {
			return iter.Error()
		} else if tx == nil {
			return nil
		}

		m.Infof(0, "found stored tx %v", tx.Hash())
		err := m.AddTx(tx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *metacontroller) ensureController(stateURI string) (Controller, error) {
	m.controllersMu.Lock()
	defer m.controllersMu.Unlock()

	ctrl := m.controllers[stateURI]
	if ctrl == nil {
		// Set up the controller
		var err error
		ctrl, err = NewController(m.address, stateURI, m.txProcessedHandler)
		if err != nil {
			return nil, err
		}

		m.CtxAddChild(ctrl.Ctx(), nil)
		err = ctrl.Start()
		if err != nil {
			m.Errorf("error starting new controller: %v", err)
			return nil, err
		}

		m.controllers[stateURI] = ctrl
	}
	return ctrl, nil
}

func (m *metacontroller) txProcessedHandler(c Controller, tx *Tx, state interface{}) error {
	err := m.store.AddTx(tx)
	if err != nil {
		m.Errorf("error adding tx to store: %+v", err)
		// @@TODO: is there anything else we can do here?
	}

	func() {
		m.validStateURIsMu.Lock()
		defer m.validStateURIsMu.Unlock()
		m.validStateURIs[tx.URL] = struct{}{}
	}()

	if tx.Checkpoint {
		err = m.store.AddState(tx.ID, state)
		if err != nil {
			m.Errorf("error adding state to store: %+v", err)
			// @@TODO: is there anything else we can do here?
		}
	}

	// Notify the Host to start fetching any refs we don't have yet
	if m.receivedRefsHandler != nil {
		var refs []Hash
		err := WalkLinks(state, func(linkType LinkType, link string, keypath []string, val map[string]interface{}) error {
			if linkType == LinkTypeRef {
				hash, err := HashFromHex(link)
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

		m.receivedRefsHandler(refs)
	}

	// Walk the tree and initialize validators and resolvers
	// @@TODO: inefficient
	if !m.resolversLocked {
		newResolverTree := &resolverTree{}
		newResolverTree.addResolver([]string{}, &dumbResolver{})
		err = walkTree(state, func(keypath []string, val interface{}) error {
			resolverConfig, exists := getValue(val, []string{"Merge-Type"})
			if exists {
				// Resolve any refs (to code) in the resolver config object.  We deep copy the config so
				// that we don't inject any refs into the state tree itself
				config, anyMissing, err := m.ResolveRefs(DeepCopyJSValue(resolverConfig).(map[string]interface{}))
				if err != nil {
					return err
				} else if anyMissing {
					return ErrMissingCriticalRefs
				}

				// @@TODO: if the resolver type changes, this totally breaks everything
				var resolverInternalState map[string]interface{}
				oldResolverNode, depth := c.ResolverTree().nearestResolverNodeForKeypath(keypath)
				if depth != len(keypath) {
					resolverInternalState = make(map[string]interface{})
				} else {
					resolverInternalState = oldResolverNode.resolver.InternalState()
				}

				resolver, err := initResolverFromConfig(m, config.(*NelSON), resolverInternalState)
				if err != nil {
					return err
				}
				newResolverTree.addResolver(keypath, resolver)
			}

			validatorConfig, exists := getValue(val, []string{"Validator"})
			if exists {
				// Resolve any refs (to code) in the validator config object.  We deep copy the config so
				// that we don't inject any refs into the state tree itself
				config, anyMissing, err := m.ResolveRefs(DeepCopyJSValue(validatorConfig).(map[string]interface{}))
				if err != nil {
					return err
				} else if anyMissing {
					return ErrMissingCriticalRefs
				}

				validator, err := initValidatorFromConfig(m, config.(*NelSON))
				if err != nil {
					return err
				}
				newResolverTree.addValidator(keypath, validator)
			}

			return nil
		})
		if err != nil {
			return err
		}
		c.SetResolverTree(newResolverTree)
	}

	return nil
}

func (m *metacontroller) KnownStateURIs() []string {
	m.validStateURIsMu.Lock()
	defer m.validStateURIsMu.Unlock()

	var stateURIs []string
	for stateURI := range m.validStateURIs {
		stateURIs = append(stateURIs, stateURI)
	}
	return stateURIs
}

var (
	ErrInvalidPrivateRootKey = errors.New("invalid private root key")
)

func (m *metacontroller) AddTx(tx *Tx) error {
	if tx.IsPrivate() {
		parts := strings.Split(tx.URL, "/")
		if parts[len(parts)-1] != tx.PrivateRootKey() {
			return ErrInvalidPrivateRootKey
		}
	}

	ctrl, err := m.ensureController(tx.URL)
	if err != nil {
		return err
	}
	ctrl.AddTx(tx)
	return nil
}

func (m *metacontroller) FetchTxs(stateURI string) TxIterator {
	return m.store.AllTxsForStateURI(stateURI)
}

func (m *metacontroller) FetchTx(stateURI string, txID ID) (*Tx, error) {
	return m.store.FetchTx(stateURI, txID)
}

func (m *metacontroller) HaveTx(stateURI string, txID ID) bool {
	m.controllersMu.RLock()
	defer m.controllersMu.RUnlock()

	ctrl := m.controllers[stateURI]
	if ctrl == nil {
		return false
	}
	return ctrl.HaveTx(txID)
}

func (m *metacontroller) State(stateURI string, keypath []string, version *ID) (interface{}, []ID, error) {
	if version != nil {
		state, err := m.store.FetchState(*version)
		if err != nil {
			return nil, nil, err
		}

		tx, err := m.store.FetchTx(stateURI, *version)
		if err != nil {
			return nil, nil, err
		}

		sliced, exists := getValue(state, keypath)
		if !exists {
			return nil, nil, Err404
		}
		return sliced, tx.Parents, nil

	} else {
		m.controllersMu.RLock()
		defer m.controllersMu.RUnlock()

		ctrl := m.controllers[stateURI]
		if ctrl == nil {
			return nil, nil, errors.Wrapf(ErrNoController, stateURI)
		}
		state, leaves := ctrl.State(keypath)
		return state, leaves, nil
	}
}

func (m *metacontroller) ResolveRefs(input interface{}) (interface{}, bool, error) {
	m.controllersMu.RLock()
	defer m.controllersMu.RUnlock()

	var anyMissing bool
	newInput, err := mapTree(input, func(keypath []string, val interface{}) (interface{}, error) {
		if _, exists := getString(val, []string{"Content-Type"}); exists {
			n := m.ResolveNelSON(val)
			if !n.fullyResolved {
				anyMissing = true
			}
			return n, nil
		} else {
			return val, nil
		}
	})
	if err != nil {
		return nil, false, err
	}
	return newInput, anyMissing, nil
}

func (m *metacontroller) MostRecentTxID(stateURI string) (ID, error) {
	m.controllersMu.RLock()
	defer m.controllersMu.RUnlock()

	ctrl := m.controllers[stateURI]
	if ctrl == nil {
		return ID{}, errors.Wrapf(ErrNoController, stateURI)
	}
	return ctrl.MostRecentTxID(), nil
}

func (m *metacontroller) SetReceivedRefsHandler(handler ReceivedRefsHandler) {
	m.receivedRefsHandler = handler
}

func (m *metacontroller) OnDownloadedRef() {
	m.controllersMu.RLock()
	defer m.controllersMu.RUnlock()

	for _, ctrl := range m.controllers {
		ctrl.OnDownloadedRef()
	}
}

func (m *metacontroller) DebugLockResolvers() {
	m.resolversLocked = true
}
