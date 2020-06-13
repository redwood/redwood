package redwood

import (
	"io"
	"strings"
	"sync"

	"github.com/pkg/errors"

	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/tree"
	"github.com/brynbellomy/redwood/types"
)

type ControllerHub interface {
	Ctx() *ctx.Context
	Start() error

	AddTx(tx *Tx) error
	FetchTx(stateURI string, txID types.ID) (*Tx, error)
	FetchTxs(stateURI string) TxIterator
	HaveTx(stateURI string, txID types.ID) (bool, error)

	KnownStateURIs() ([]string, error)
	StateAtVersion(stateURI string, version *types.ID) (tree.Node, error)
	QueryIndex(stateURI string, version *types.ID, keypath tree.Keypath, indexName tree.Keypath, queryParam tree.Keypath, rng *tree.Range) (tree.Node, error)
	Leaves(stateURI string) ([]types.ID, error)

	RefObjectReader(refID types.RefID) (io.ReadCloser, int64, error)

	OnNewState(fn func(tx *Tx))
}

type controllerHub struct {
	*ctx.Context

	address types.Address

	controllers   map[string]Controller
	controllersMu sync.RWMutex
	txStore       TxStore
	refStore      RefStore
	dbRootPath    string

	newStateListeners   []func(tx *Tx)
	newStateListenersMu sync.RWMutex
}

var (
	ErrNoController = errors.New("no controller for that stateURI")

	MergeTypeKeypath = tree.Keypath("Merge-Type")
	ValidatorKeypath = tree.Keypath("Validator")
)

func NewControllerHub(address types.Address, dbRootPath string, txStore TxStore, refStore RefStore) ControllerHub {
	return &controllerHub{
		Context:     &ctx.Context{},
		address:     address,
		controllers: make(map[string]Controller),
		dbRootPath:  dbRootPath,
		txStore:     txStore,
		refStore:    refStore,
	}
}

func (m *controllerHub) Start() error {
	return m.CtxStart(
		// on startup
		func() error {
			m.SetLogLabel(m.address.Pretty() + " controllerHub")

			stateURIs, err := m.txStore.KnownStateURIs()
			if err != nil {
				return err
			}

			for _, stateURI := range stateURIs {
				_, err := m.ensureController(stateURI)
				if err != nil {
					return err
				}
			}

			return nil
		},
		nil,
		nil,
		// on shutdown
		func() {},
	)
}

func (m *controllerHub) ensureController(stateURI string) (Controller, error) {
	m.controllersMu.Lock()
	defer m.controllersMu.Unlock()

	ctrl := m.controllers[stateURI]
	if ctrl == nil {
		// Set up the controller
		var err error
		ctrl, err = NewController(m.address, stateURI, m.dbRootPath, m.txStore, m.refStore)
		if err != nil {
			return nil, err
		}

		ctrl.OnNewState(m.notifyNewStateListeners)

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

func (m *controllerHub) KnownStateURIs() ([]string, error) {
	return m.txStore.KnownStateURIs()
}

var (
	ErrInvalidPrivateRootKey = errors.New("invalid private root key")
)

func (m *controllerHub) AddTx(tx *Tx) error {
	if tx.IsPrivate() {
		parts := strings.Split(tx.StateURI, "/")
		if parts[len(parts)-1] != tx.PrivateRootKey() {
			return ErrInvalidPrivateRootKey
		}
	}

	ctrl, err := m.ensureController(tx.StateURI)
	if err != nil {
		return err
	}
	ctrl.AddTx(tx)
	return nil
}

func (m *controllerHub) FetchTxs(stateURI string) TxIterator {
	return m.txStore.AllTxsForStateURI(stateURI)
}

func (m *controllerHub) FetchTx(stateURI string, txID types.ID) (*Tx, error) {
	return m.txStore.FetchTx(stateURI, txID)
}

func (m *controllerHub) HaveTx(stateURI string, txID types.ID) (bool, error) {
	m.controllersMu.RLock()
	defer m.controllersMu.RUnlock()

	ctrl := m.controllers[stateURI]
	if ctrl == nil {
		return false, nil
	}
	return ctrl.HaveTx(txID)
}

func (m *controllerHub) StateAtVersion(stateURI string, version *types.ID) (tree.Node, error) {
	m.controllersMu.RLock()
	defer m.controllersMu.RUnlock()

	ctrl := m.controllers[stateURI]
	if ctrl == nil {
		return nil, errors.Wrapf(ErrNoController, stateURI)
	}
	return ctrl.StateAtVersion(version), nil
}

func (m *controllerHub) QueryIndex(stateURI string, version *types.ID, keypath tree.Keypath, indexName tree.Keypath, queryParam tree.Keypath, rng *tree.Range) (tree.Node, error) {
	m.controllersMu.RLock()
	defer m.controllersMu.RUnlock()

	ctrl := m.controllers[stateURI]
	if ctrl == nil {
		return nil, errors.Wrapf(ErrNoController, stateURI)
	}
	return ctrl.QueryIndex(version, keypath, indexName, queryParam, rng)
}

func (m *controllerHub) RefObjectReader(refID types.RefID) (io.ReadCloser, int64, error) {
	return m.refStore.Object(refID)
}

func (m *controllerHub) Leaves(stateURI string) ([]types.ID, error) {
	return m.txStore.Leaves(stateURI)
}

func (m *controllerHub) OnNewState(fn func(tx *Tx)) {
	m.newStateListenersMu.Lock()
	defer m.newStateListenersMu.Unlock()
	m.newStateListeners = append(m.newStateListeners, fn)
}

func (m *controllerHub) notifyNewStateListeners(tx *Tx) {
	m.newStateListenersMu.RLock()
	defer m.newStateListenersMu.RUnlock()

	var wg sync.WaitGroup
	wg.Add(len(m.newStateListeners))

	for _, handler := range m.newStateListeners {
		handler := handler
		go func() {
			defer wg.Done()
			handler(tx)
		}()
	}
	wg.Wait()
}
