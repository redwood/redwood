package redwood

import (
	"io"
	"strings"
	"sync"

	"github.com/pkg/errors"

	"redwood.dev/ctx"
	"redwood.dev/tree"
	"redwood.dev/types"
)

type ControllerHub interface {
	Ctx() *ctx.Context
	Start() error

	AddTx(tx *Tx, force bool) error
	FetchTx(stateURI string, txID types.ID) (*Tx, error)
	FetchTxs(stateURI string, fromTxID types.ID) TxIterator
	HaveTx(stateURI string, txID types.ID) (bool, error)

	EnsureController(stateURI string) (Controller, error)
	KnownStateURIs() ([]string, error)
	StateAtVersion(stateURI string, version *types.ID) (tree.Node, error)
	QueryIndex(stateURI string, version *types.ID, keypath tree.Keypath, indexName tree.Keypath, queryParam tree.Keypath, rng *tree.Range) (tree.Node, error)
	Leaves(stateURI string) ([]types.ID, error)

	IsPrivate(stateURI string) (bool, error)
	IsMember(stateURI string, addr types.Address) (bool, error)
	Members(stateURI string) ([]types.Address, error)

	RefObjectReader(refID types.RefID) (io.ReadCloser, int64, error)

	OnNewState(fn func(tx *Tx, state tree.Node, leaves []types.ID))
}

type controllerHub struct {
	*ctx.Context

	controllers   map[string]Controller
	controllersMu sync.RWMutex
	txStore       TxStore
	refStore      RefStore
	dbRootPath    string

	newStateListeners   []func(tx *Tx, state tree.Node, leaves []types.ID)
	newStateListenersMu sync.RWMutex
}

var (
	ErrNoController = errors.New("no controller for that stateURI")
)

func NewControllerHub(dbRootPath string, txStore TxStore, refStore RefStore) ControllerHub {
	return &controllerHub{
		Context:     &ctx.Context{},
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
			m.SetLogLabel("controller hub")

			stateURIs, err := m.txStore.KnownStateURIs()
			if err != nil {
				return err
			}

			for _, stateURI := range stateURIs {
				_, err := m.EnsureController(stateURI)
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

func (m *controllerHub) EnsureController(stateURI string) (Controller, error) {
	m.controllersMu.Lock()
	defer m.controllersMu.Unlock()

	ctrl := m.controllers[stateURI]
	if ctrl == nil {
		// Set up the controller
		var err error
		ctrl, err = NewController(stateURI, m.dbRootPath, m, m.txStore, m.refStore)
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

func (m *controllerHub) AddTx(tx *Tx, force bool) error {
	if tx.IsPrivate() {
		parts := strings.Split(tx.StateURI, "/")
		if parts[len(parts)-1] != tx.PrivateRootKey() {
			return ErrInvalidPrivateRootKey
		}
	}

	ctrl, err := m.EnsureController(tx.StateURI)
	if err != nil {
		return err
	}
	return ctrl.AddTx(tx, force)
}

func (m *controllerHub) FetchTxs(stateURI string, fromTxID types.ID) TxIterator {
	return m.txStore.AllTxsForStateURI(stateURI, fromTxID)
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

func (m *controllerHub) IsPrivate(stateURI string) (bool, error) {
	m.controllersMu.RLock()
	defer m.controllersMu.RUnlock()

	ctrl := m.controllers[stateURI]
	if ctrl == nil {
		return false, errors.Wrapf(ErrNoController, stateURI)
	}
	return ctrl.IsPrivate()
}

func (m *controllerHub) IsMember(stateURI string, addr types.Address) (bool, error) {
	m.controllersMu.RLock()
	defer m.controllersMu.RUnlock()

	ctrl := m.controllers[stateURI]
	if ctrl == nil {
		return false, errors.Wrapf(ErrNoController, stateURI)
	}
	return ctrl.IsMember(addr)
}

func (m *controllerHub) Members(stateURI string) ([]types.Address, error) {
	m.controllersMu.RLock()
	defer m.controllersMu.RUnlock()

	ctrl := m.controllers[stateURI]
	if ctrl == nil {
		return nil, errors.Wrapf(ErrNoController, stateURI)
	}
	return ctrl.Members(), nil
}

func (m *controllerHub) OnNewState(fn func(tx *Tx, state tree.Node, leaves []types.ID)) {
	m.newStateListenersMu.Lock()
	defer m.newStateListenersMu.Unlock()
	m.newStateListeners = append(m.newStateListeners, fn)
}

func (m *controllerHub) notifyNewStateListeners(tx *Tx, state tree.Node, leaves []types.ID) {
	m.newStateListenersMu.RLock()
	defer m.newStateListenersMu.RUnlock()

	var wg sync.WaitGroup
	wg.Add(len(m.newStateListeners))

	for _, handler := range m.newStateListeners {
		handler := handler
		go func() {
			defer wg.Done()
			handler(tx, state, leaves)
		}()
	}
	wg.Wait()
}
