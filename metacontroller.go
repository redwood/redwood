package redwood

import (
	"io"
	"strings"
	"sync"

	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/nelson"
	"github.com/brynbellomy/redwood/tree"
	"github.com/brynbellomy/redwood/types"
)

type Metacontroller interface {
	Ctx() *ctx.Context
	Start() error

	AddTx(tx *Tx) error
	FetchTx(stateURI string, txID types.ID) (*Tx, error)
	FetchTxs(stateURI string) TxIterator
	HaveTx(stateURI string, txID types.ID) (bool, error)

	StateURIExists(stateURI string) bool
	KnownStateURIs() []string
	StateAtVersion(stateURI string, version *types.ID) (tree.Node, error)
	QueryIndex(stateURI string, version *types.ID, keypath tree.Keypath, indexName tree.Keypath, queryParam tree.Keypath, rng *tree.Range) (tree.Node, error)
	Leaves(stateURI string) (map[types.ID]struct{}, error)

	SetReceivedRefsHandler(handler ReceivedRefsHandler)
	OnDownloadedRef()
	RefObjectReader(refID types.RefID) (io.ReadCloser, int64, error)

	DebugLockResolvers()
}

type metacontroller struct {
	*ctx.Context

	address types.Address

	controllers         map[string]Controller
	controllersMu       sync.RWMutex
	receivedRefsHandler ReceivedRefsHandler
	txStore             TxStore
	refStore            RefStore
	dbRootPath          string

	resolversLocked bool

	validStateURIs   map[string]struct{}
	validStateURIsMu sync.Mutex
}

var (
	ErrNoController = errors.New("no controller for that stateURI")

	MergeTypeKeypath = tree.Keypath("Merge-Type")
	ValidatorKeypath = tree.Keypath("Validator")
)

func NewMetacontroller(address types.Address, dbRootPath string, txStore TxStore, refStore RefStore) Metacontroller {
	return &metacontroller{
		Context:        &ctx.Context{},
		address:        address,
		controllers:    make(map[string]Controller),
		dbRootPath:     dbRootPath,
		txStore:        txStore,
		refStore:       refStore,
		validStateURIs: make(map[string]struct{}),
	}
}

func (m *metacontroller) Start() error {
	return m.CtxStart(
		// on startup
		func() error {
			m.SetLogLabel(m.address.Pretty() + " metacontroller")

			m.CtxAddChild(m.txStore.Ctx(), nil)

			err := m.txStore.Start()
			if err != nil {
				return err
			}

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
	iter := m.txStore.AllTxs()
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

		if tx.Status == TxStatusValid {
			m.validStateURIsMu.Lock()
			m.validStateURIs[tx.URL] = struct{}{}
			m.validStateURIsMu.Unlock()
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
		ctrl, err = NewController(m.address, stateURI, m.dbRootPath, m.txStore, m.txProcessedHandler)
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

func (m *metacontroller) txProcessedHandler(c Controller, tx *Tx, state *tree.DBNode) error {
	err := m.txStore.AddTx(tx)
	if err != nil {
		m.Errorf("error adding tx to store: %+v", err)
		// @@TODO: is there anything else we can do here?
	}

	m.validStateURIsMu.Lock()
	m.validStateURIs[tx.URL] = struct{}{}
	m.validStateURIsMu.Unlock()

	// Walk the tree and initialize validators and resolvers
	// @@TODO: inefficient
	newBehaviorTree := newBehaviorTree()
	newBehaviorTree.addResolver(nil, &dumbResolver{})

	var refs []types.RefID
	defer func() {
		if m.receivedRefsHandler != nil {
				m.receivedRefsHandler(refs)
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
					return err
				} else if contentType != "link" {
					continue
				}

				linkStr, _, err := state.StringValue(keypath)
				if err != nil {
					return err
			}
			linkType, linkValue := nelson.DetermineLinkType(linkStr)
			if linkType == nelson.LinkTypeRef {
				var refID types.RefID
				err := refID.UnmarshalText([]byte(linkValue))
				if err != nil {
					return err
				}
				refs = append(refs, refID)
			}
		}
		}

		// Remove deleted resolvers and validators
		for kp := range diff.Removed {
			parentKeypath, key := tree.Keypath(kp).Pop()
			switch {
			case key.Equals(MergeTypeKeypath):
				c.BehaviorTree().removeResolver(parentKeypath)
			case key.Equals(ValidatorKeypath):
				c.BehaviorTree().removeValidator(parentKeypath)
			case parentKeypath.Part(-1).Equals(tree.Keypath("Indices")):
				//indicesKeypath, _ := parentKeypath.Pop()
				//c.BehaviorTree().removeIndexer()
			}

			for parentKeypath != nil {
				nextParentKeypath, key := parentKeypath.Pop()
				switch {
				case key.Equals(MergeTypeKeypath):
					err := m.initializeResolver(state, parentKeypath, c)
					if err != nil {
						return err
					}
				case key.Equals(ValidatorKeypath):
					err := m.initializeValidator(state, parentKeypath, c)
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
				err := m.initializeResolver(state, keypath, c)
				if err != nil {
					return err
				}

			case key.Equals(ValidatorKeypath):
				err := m.initializeValidator(state, keypath, c)
				if err != nil {
					return err
				}

			case key.Equals(tree.Keypath("Indices")):
				err := m.initializeIndexer(state, keypath, c)
				if err != nil {
					return err
				}
			}

			for parentKeypath != nil {
				nextParentKeypath, key := parentKeypath.Pop()
				switch {
				case key.Equals(MergeTypeKeypath):
					err := m.initializeResolver(state, parentKeypath, c)
					if err != nil {
						return err
					}
				case key.Equals(ValidatorKeypath):
					err := m.initializeValidator(state, parentKeypath, c)
					if err != nil {
						return err
					}
				}
				parentKeypath = nextParentKeypath
		}
	}

	return nil
}

func (m *metacontroller) initializeResolver(state *tree.DBNode, resolverKeypath tree.Keypath, c Controller) error {
	// Resolve any refs (to code) in the resolver config object.  We copy the config so
	// that we don't inject any refs into the state tree itself
	config, err := state.CopyToMemory(resolverKeypath, nil)
	if err != nil {
		return err
	}

	config, anyMissing, err := nelson.Resolve(config, m)
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
	oldResolver, oldResolverKeypath := c.BehaviorTree().nearestResolverForKeypath(resolverKeypath)
	if !oldResolverKeypath.Equals(resolverKeypath) {
		internalState = make(map[string]interface{})
	} else {
		internalState = oldResolver.InternalState()
	}

	resolver, err := ctor(config, internalState)
	if err != nil {
		return err
	}

	c.BehaviorTree().addResolver(resolverKeypath, resolver)
	return nil
}

func (m *metacontroller) initializeValidator(state *tree.DBNode, validatorKeypath tree.Keypath, c Controller) error {
	// Resolve any refs (to code) in the validator config object.  We copy the config so
	// that we don't inject any refs into the state tree itself
	config, err := state.CopyToMemory(validatorKeypath, nil)
	if err != nil {
		return err
	}

	config, anyMissing, err := nelson.Resolve(config, m)
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

	c.BehaviorTree().addValidator(validatorKeypath, validator)
	return nil
}

func (m *metacontroller) initializeIndexer(state *tree.DBNode, indexerConfigKeypath tree.Keypath, c Controller) error {
	// Resolve any refs (to code) in the indexer config object.  We copy the config so
	// that we don't inject any refs into the state tree itself
	indexConfigs, err := state.CopyToMemory(indexerConfigKeypath, nil)
	if err != nil {
		return err
	}

	subkeys := indexConfigs.Subkeys()

	for _, indexName := range subkeys {
		config, anyMissing, err := nelson.Resolve(indexConfigs.NodeAt(indexName, nil), m)
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

		c.BehaviorTree().addIndexer(indexerNodeKeypath, indexName, indexer)
	}
	return nil
}

func (m *metacontroller) StateURIExists(stateURI string) bool {
	m.validStateURIsMu.Lock()
	defer m.validStateURIsMu.Unlock()
	_, exists := m.validStateURIs[stateURI]
	return exists
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
	return m.txStore.AllTxsForStateURI(stateURI)
}

func (m *metacontroller) FetchTx(stateURI string, txID types.ID) (*Tx, error) {
	return m.txStore.FetchTx(stateURI, txID)
}

func (m *metacontroller) HaveTx(stateURI string, txID types.ID) (bool, error) {
	m.controllersMu.RLock()
	defer m.controllersMu.RUnlock()

	ctrl := m.controllers[stateURI]
	if ctrl == nil {
		return false, nil
	}
	return ctrl.HaveTx(txID)
}

func (m *metacontroller) StateAtVersion(stateURI string, version *types.ID) (tree.Node, error) {
	m.controllersMu.RLock()
	defer m.controllersMu.RUnlock()

	ctrl := m.controllers[stateURI]
	if ctrl == nil {
		return nil, errors.Wrapf(ErrNoController, stateURI)
	}
	return ctrl.StateAtVersion(version), nil
}

func (m *metacontroller) QueryIndex(stateURI string, version *types.ID, keypath tree.Keypath, indexName tree.Keypath, queryParam tree.Keypath, rng *tree.Range) (tree.Node, error) {
	m.controllersMu.RLock()
	defer m.controllersMu.RUnlock()

	ctrl := m.controllers[stateURI]
	if ctrl == nil {
		return nil, errors.Wrapf(ErrNoController, stateURI)
	}
	return ctrl.QueryIndex(version, keypath, indexName, queryParam, rng)
}

func (m *metacontroller) RefObjectReader(refID types.RefID) (io.ReadCloser, int64, error) {
	return m.refStore.Object(refID)
}

func (m *metacontroller) Leaves(stateURI string) (map[types.ID]struct{}, error) {
	m.controllersMu.RLock()
	defer m.controllersMu.RUnlock()

	ctrl := m.controllers[stateURI]
	if ctrl == nil {
		return nil, errors.Wrapf(ErrNoController, stateURI)
	}
	return ctrl.Leaves(), nil
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
