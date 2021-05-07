package tree

import (
	"io"
	"strings"
	"sync"

	"github.com/pkg/errors"

	"redwood.dev/blob"
	"redwood.dev/log"
	"redwood.dev/state"
	"redwood.dev/types"
)

type ControllerHub interface {
	Start() error
	Close()

	AddTx(tx *Tx) error
	FetchTx(stateURI string, txID types.ID) (*Tx, error)
	FetchTxs(stateURI string, fromTxID types.ID) TxIterator

	EnsureController(stateURI string) (Controller, error)
	KnownStateURIs() ([]string, error)
	StateAtVersion(stateURI string, version *types.ID) (state.Node, error)
	QueryIndex(stateURI string, version *types.ID, keypath state.Keypath, indexName state.Keypath, queryParam state.Keypath, rng *state.Range) (state.Node, error)
	Leaves(stateURI string) ([]types.ID, error)

	IsPrivate(stateURI string) (bool, error)
	IsMember(stateURI string, addr types.Address) (bool, error)
	Members(stateURI string) ([]types.Address, error)

	BlobReader(refID blob.ID) (io.ReadCloser, int64, error)

	OnNewState(fn func(tx *Tx, root state.Node, leaves []types.ID))
}

type controllerHub struct {
	log.Logger
	chStop chan struct{}

	controllers   map[string]Controller
	controllersMu sync.RWMutex
	txStore       TxStore
	blobStore     blob.Store
	dbRootPath    string

	newStateListeners   []func(tx *Tx, root state.Node, leaves []types.ID)
	newStateListenersMu sync.RWMutex
}

var (
	ErrNoController = errors.New("no controller for that stateURI")
)

func NewControllerHub(dbRootPath string, txStore TxStore, blobStore blob.Store) ControllerHub {
	return &controllerHub{
		Logger:      log.NewLogger("controller hub"),
		chStop:      make(chan struct{}),
		controllers: make(map[string]Controller),
		dbRootPath:  dbRootPath,
		txStore:     txStore,
		blobStore:   blobStore,
	}
}

func (m *controllerHub) Start() (err error) {
	defer func() {
		if err != nil {
			m.Close()
		}
	}()

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
}

func (m *controllerHub) Close() {
	close(m.chStop)

	m.controllersMu.Lock()
	defer m.controllersMu.Unlock()
	for _, c := range m.controllers {
		c.Close()
	}
}

func (m *controllerHub) EnsureController(stateURI string) (Controller, error) {
	m.controllersMu.Lock()
	defer m.controllersMu.Unlock()

	ctrl := m.controllers[stateURI]
	if ctrl == nil {
		// Set up the controller
		var err error
		ctrl, err = NewController(stateURI, m.dbRootPath, m, m.txStore, m.blobStore)
		if err != nil {
			return nil, err
		}

		ctrl.OnNewState(m.notifyNewStateListeners)

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
			return errors.Wrapf(ErrInvalidPrivateRootKey, "got %v, expected %v", parts[len(parts)-1], tx.PrivateRootKey())
		}
	}

	ctrl, err := m.EnsureController(tx.StateURI)
	if err != nil {
		return err
	}
	return ctrl.AddTx(tx)
}

func (m *controllerHub) FetchTxs(stateURI string, fromTxID types.ID) TxIterator {
	return m.txStore.AllTxsForStateURI(stateURI, fromTxID)
}

func (m *controllerHub) FetchTx(stateURI string, txID types.ID) (*Tx, error) {
	return m.txStore.FetchTx(stateURI, txID)
}

func (m *controllerHub) StateAtVersion(stateURI string, version *types.ID) (state.Node, error) {
	m.controllersMu.RLock()
	defer m.controllersMu.RUnlock()

	ctrl := m.controllers[stateURI]
	if ctrl == nil {
		return nil, errors.Wrapf(ErrNoController, stateURI)
	}
	return ctrl.StateAtVersion(version), nil
}

func (m *controllerHub) QueryIndex(stateURI string, version *types.ID, keypath state.Keypath, indexName state.Keypath, queryParam state.Keypath, rng *state.Range) (state.Node, error) {
	m.controllersMu.RLock()
	defer m.controllersMu.RUnlock()

	ctrl := m.controllers[stateURI]
	if ctrl == nil {
		return nil, errors.Wrapf(ErrNoController, stateURI)
	}
	return ctrl.QueryIndex(version, keypath, indexName, queryParam, rng)
}

func (m *controllerHub) BlobReader(refID blob.ID) (io.ReadCloser, int64, error) {
	return m.blobStore.BlobReader(refID)
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

func (m *controllerHub) OnNewState(fn func(tx *Tx, root state.Node, leaves []types.ID)) {
	m.newStateListenersMu.Lock()
	defer m.newStateListenersMu.Unlock()
	m.newStateListeners = append(m.newStateListeners, fn)
}

func (m *controllerHub) notifyNewStateListeners(tx *Tx, root state.Node, leaves []types.ID) {
	m.newStateListenersMu.RLock()
	defer m.newStateListenersMu.RUnlock()

	var wg sync.WaitGroup
	wg.Add(len(m.newStateListeners))

	for _, handler := range m.newStateListeners {
		handler := handler
		go func() {
			defer wg.Done()
			handler(tx, root, leaves)
		}()
	}
	wg.Wait()
}
