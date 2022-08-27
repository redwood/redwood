package tree

import (
	"context"
	"fmt"
	"io"
	"sync"

	"redwood.dev/blob"
	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/state"
	"redwood.dev/types"
	"redwood.dev/utils"
	"redwood.dev/utils/badgerutils"
	. "redwood.dev/utils/generics"
)

type ControllerHub interface {
	process.Interface

	AddTx(tx Tx) error
	FetchTx(stateURI string, txID state.Version) (Tx, error)
	FetchValidTxsOrdered(stateURI string, fromTxID state.Version) TxIterator

	StateURIsWithData() (Set[string], error)
	IsStateURIWithData(stateURI string) (bool, error)
	OnNewStateURIWithData(fn NewStateURIWithDataCallback)
	StateAtVersion(stateURI string, version *state.Version) (state.Node, error)
	Leaves(stateURI string) ([]state.Version, error)

	BlobReader(refID blob.ID, rng *types.Range) (io.ReadCloser, int64, error)

	OnNewState(fn NewStateCallback)
	DebugPrint(stateURI string)
	DebugPrintAll()
}

type controllerHub struct {
	process.Process
	log.Logger

	controllers   map[string]Controller
	controllersMu sync.RWMutex
	txStore       TxStore
	blobStore     blob.Store
	dbRootPath    string
	badgerOpts    badgerutils.OptsBuilder

	dbTree *state.DBTree

	newStateListeners   []NewStateCallback
	newStateListenersMu sync.RWMutex
}

func NewControllerHub(dbRootPath string, txStore TxStore, blobStore blob.Store, badgerOpts badgerutils.OptsBuilder) ControllerHub {
	return &controllerHub{
		Process:     *process.New("controller hub"),
		Logger:      log.NewLogger("controller hub"),
		controllers: make(map[string]Controller),
		dbRootPath:  dbRootPath,
		badgerOpts:  badgerOpts,
		txStore:     txStore,
		blobStore:   blobStore,
	}
}

func (m *controllerHub) Start() (err error) {
	defer func() {
		if err != nil {
			fmt.Println(err)
			m.Close()
		}
	}()
	err = m.Process.Start()
	if err != nil {
		return err
	}

	dbTree, err := state.NewDBTree(m.badgerOpts.ForPath(m.dbRootPath))
	if err != nil {
		return err
	}
	m.dbTree = dbTree

	stateURIs, err := m.txStore.StateURIsWithData()
	if err != nil {
		return err
	}

	for stateURI := range stateURIs {
		_, err := m.ensureController(stateURI)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *controllerHub) ensureController(stateURI string) (Controller, error) {
	m.controllersMu.Lock()
	defer m.controllersMu.Unlock()

	ctrl := m.controllers[stateURI]
	if ctrl == nil {
		// Set up the controller
		// stateURIClean := strings.NewReplacer(":", "_", "/", "_").Replace(stateURI)

		var err error
		ctrl, err = NewController(stateURI, m.dbTree, m, m.txStore, m.blobStore)
		if err != nil {
			return nil, err
		}

		ctrl.OnNewState(m.notifyNewStateListeners)

		err = m.Process.SpawnChild(context.TODO(), ctrl)
		if err != nil {
			m.Errorf("error starting new controller: %v", err)
			return nil, err
		}

		m.controllers[stateURI] = ctrl
	}
	return ctrl, nil
}

func (m *controllerHub) StateURIsWithData() (Set[string], error) {
	return m.txStore.StateURIsWithData()
}

func (m *controllerHub) IsStateURIWithData(stateURI string) (bool, error) {
	return m.txStore.IsStateURIWithData(stateURI)
}

func (m *controllerHub) OnNewStateURIWithData(fn NewStateURIWithDataCallback) {
	m.txStore.OnNewStateURIWithData(fn)
}

func (m *controllerHub) AddTx(tx Tx) error {
	ctrl, err := m.ensureController(tx.StateURI)
	if err != nil {
		return err
	}
	return ctrl.AddTx(tx)
}

func (m *controllerHub) FetchValidTxsOrdered(stateURI string, fromTxID state.Version) TxIterator {
	return m.txStore.AllValidTxsForStateURIOrdered(stateURI, fromTxID)
}

func (m *controllerHub) FetchTx(stateURI string, txID state.Version) (Tx, error) {
	return m.txStore.FetchTx(stateURI, txID)
}

func (m *controllerHub) StateAtVersion(stateURI string, version *state.Version) (state.Node, error) {
	m.controllersMu.RLock()
	defer m.controllersMu.RUnlock()

	ctrl := m.controllers[stateURI]
	if ctrl == nil {
		return nil, errors.Wrapf(errors.Err404, stateURI)
	}
	return ctrl.StateAtVersion(version), nil
}

// func (m *controllerHub) QueryIndex(stateURI string, version *state.Version, keypath state.Keypath, indexName state.Keypath, queryParam state.Keypath, rng *state.Range) (state.Node, error) {
// 	m.controllersMu.RLock()
// 	defer m.controllersMu.RUnlock()

// 	ctrl := m.controllers[stateURI]
// 	if ctrl == nil {
// 		return nil, errors.Wrapf(errors.Err404, stateURI)
// 	}
// 	return ctrl.QueryIndex(version, keypath, indexName, queryParam, rng)
// }

func (m *controllerHub) BlobReader(refID blob.ID, rng *types.Range) (io.ReadCloser, int64, error) {
	return m.blobStore.BlobReader(refID, rng)
}

func (m *controllerHub) Leaves(stateURI string) ([]state.Version, error) {
	return m.txStore.Leaves(stateURI)
}

func (m *controllerHub) OnNewState(fn NewStateCallback) {
	m.newStateListenersMu.Lock()
	defer m.newStateListenersMu.Unlock()
	m.newStateListeners = append(m.newStateListeners, fn)
}

func (m *controllerHub) notifyNewStateListeners(tx Tx, root state.Node, leaves []state.Version, diff *state.Diff) {
	m.newStateListenersMu.RLock()
	defer m.newStateListenersMu.RUnlock()

	var wg sync.WaitGroup
	wg.Add(len(m.newStateListeners))

	for _, handler := range m.newStateListeners {
		handler := handler
		go func() {
			defer wg.Done()
			handler(tx, root, leaves, diff)
		}()
	}
	wg.Wait()
}

func (m *controllerHub) DebugPrint(stateURI string) {
	m.controllersMu.RLock()
	defer m.controllersMu.RUnlock()
	if m.controllers[stateURI] == nil {
		fmt.Println("no such controller")
		return
	}
	m.controllers[stateURI].DebugPrint()
}

func (m *controllerHub) DebugPrintAll() {
	m.dbTree.DebugPrint(utils.PrintfDebugPrinter, true, 0)
}
