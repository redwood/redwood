package prototree

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/state"
	"redwood.dev/tree"
	"redwood.dev/types"
	"redwood.dev/utils"
)

//go:generate mockery --name Store --output ./mocks/ --case=underscore
type Store interface {
	SubscribedStateURIs() types.StringSet
	AddSubscribedStateURI(stateURI string) error
	RemoveSubscribedStateURI(stateURI string) error
	OnNewSubscribedStateURI(handler func(stateURI string)) (unsubscribe func())

	MaxPeersPerSubscription() uint64
	SetMaxPeersPerSubscription(max uint64) error

	TxSeenByPeer(deviceUniqueID, stateURI string, txID state.Version) bool
	MarkTxSeenByPeer(deviceUniqueID, stateURI string, txID state.Version) error
	PruneTxSeenRecordsOlderThan(threshold time.Duration) error

	DebugPrint()
}

type store struct {
	process.Process
	log.Logger

	db *state.DBTree

	data   storeData
	dataMu sync.RWMutex

	subscribedStateURIListeners   map[*subscribedStateURIListener]struct{}
	subscribedStateURIListenersMu sync.RWMutex
}

type subscribedStateURIListener struct {
	handler func(stateURI string)
}

type storeData struct {
	SubscribedStateURIs     types.StringSet                                       `tree:"subscribedStateURIs"`
	MaxPeersPerSubscription uint64                                                `tree:"maxPeersPerSubscription"`
	TxsSeenByPeers          map[string]map[tree.StateURI]map[state.Version]uint64 `tree:"txsSeenByPeers"`
	EncryptedTxs            map[tree.StateURI]map[state.Version]EncryptedTx       `tree:"encryptedTxs"`
}

var storeRootKeypath = state.Keypath("prototree")

func NewStore(db *state.DBTree) (*store, error) {
	s := &store{
		Process:                     *process.New("prototree store"),
		Logger:                      log.NewLogger("prototree store"),
		db:                          db,
		subscribedStateURIListeners: make(map[*subscribedStateURIListener]struct{}),
	}
	s.Infof(0, "opening prototree store")
	err := s.loadData()
	return s, err
}

func (s *store) Start() error {
	err := s.Process.Start()
	if err != nil {
		return err
	}

	pruneTxsTask := NewPruneTxsTask(5*time.Minute, s)
	err = s.Process.SpawnChild(nil, pruneTxsTask)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) loadData() error {
	node := s.db.State(false)
	defer node.Close()

	err := node.NodeAt(storeRootKeypath, nil).Scan(&s.data)
	if errors.Cause(err) == errors.Err404 {
		// do nothing
	} else if err != nil {
		return err
	}
	return nil
}

func (s *store) SubscribedStateURIs() types.StringSet {
	s.dataMu.RLock()
	defer s.dataMu.RUnlock()
	return s.data.SubscribedStateURIs.Copy()
}

func (s *store) AddSubscribedStateURI(stateURI string) error {
	func() {
		s.subscribedStateURIListenersMu.RLock()
		defer s.subscribedStateURIListenersMu.RUnlock()
		for listener := range s.subscribedStateURIListeners {
			listener.handler(stateURI)
		}
	}()

	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	s.data.SubscribedStateURIs.Add(stateURI)

	node := s.db.State(true)
	defer node.Close()

	err := node.Set(s.keypathForSubscribedStateURI(stateURI), nil, true)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) RemoveSubscribedStateURI(stateURI string) error {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	s.data.SubscribedStateURIs.Remove(stateURI)

	node := s.db.State(true)
	defer node.Close()

	err := node.Delete(s.keypathForSubscribedStateURI(stateURI), nil)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) keypathForSubscribedStateURI(stateURI string) state.Keypath {
	return storeRootKeypath.Pushs("subscribedStateURIs").Pushs(url.QueryEscape(stateURI))
}

func (s *store) OnNewSubscribedStateURI(handler func(stateURI string)) (unsubscribe func()) {
	s.subscribedStateURIListenersMu.Lock()
	defer s.subscribedStateURIListenersMu.Unlock()

	listener := &subscribedStateURIListener{handler: handler}
	s.subscribedStateURIListeners[listener] = struct{}{}

	return func() {
		s.subscribedStateURIListenersMu.Lock()
		defer s.subscribedStateURIListenersMu.Unlock()
		delete(s.subscribedStateURIListeners, listener)
	}
}

func (s *store) MaxPeersPerSubscription() uint64 {
	s.dataMu.RLock()
	defer s.dataMu.RUnlock()
	return s.data.MaxPeersPerSubscription
}

func (s *store) SetMaxPeersPerSubscription(max uint64) error {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	s.data.MaxPeersPerSubscription = max

	node := s.db.State(true)
	defer node.Close()

	err := node.Set(s.keypathForMaxPeersPerSubscription(), nil, max)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) keypathForMaxPeersPerSubscription() state.Keypath {
	return storeRootKeypath.Pushs("maxPeersPerSubscription")
}

func (s *store) TxSeenByPeer(deviceUniqueID string, stateURI string, txID state.Version) bool {
	if len(deviceUniqueID) == 0 {
		return false
	}

	s.dataMu.RLock()
	defer s.dataMu.RUnlock()

	if _, exists := s.data.TxsSeenByPeers[deviceUniqueID]; !exists {
		return false
	}
	if _, exists := s.data.TxsSeenByPeers[deviceUniqueID][tree.StateURI(stateURI)]; !exists {
		return false
	}
	_, exists := s.data.TxsSeenByPeers[deviceUniqueID][tree.StateURI(stateURI)][txID]
	return exists
}

func (s *store) MarkTxSeenByPeer(deviceUniqueID string, stateURI string, txID state.Version) error {
	if len(deviceUniqueID) == 0 {
		return nil
	}

	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	now := uint64(time.Now().UTC().Unix())

	if _, exists := s.data.TxsSeenByPeers[deviceUniqueID]; !exists {
		s.data.TxsSeenByPeers[deviceUniqueID] = make(map[tree.StateURI]map[state.Version]uint64)
	}
	if _, exists := s.data.TxsSeenByPeers[deviceUniqueID][tree.StateURI(stateURI)]; !exists {
		s.data.TxsSeenByPeers[deviceUniqueID][tree.StateURI(stateURI)] = make(map[state.Version]uint64)
	}
	s.data.TxsSeenByPeers[deviceUniqueID][tree.StateURI(stateURI)][txID] = now

	node := s.db.State(true)
	defer node.Close()

	err := node.Set(s.keypathForTxSeenByPeer(deviceUniqueID, tree.StateURI(stateURI), txID), nil, now)
	if err != nil {
		return err
	}

	return node.Save()
}

func (s *store) keypathForTxSeenByPeer(deviceUniqueID string, stateURI tree.StateURI, txID state.Version) state.Keypath {
	return storeRootKeypath.Pushs("txsSeenByPeers").Pushs(deviceUniqueID).Pushs(url.QueryEscape(string(stateURI))).Pushs(txID.Hex())
}

type pruneTxsTask struct {
	process.PeriodicTask
	log.Logger

	interval time.Duration
	store    Store
}

func NewPruneTxsTask(
	interval time.Duration,
	store Store,
) *pruneTxsTask {
	t := &pruneTxsTask{
		Logger: log.NewLogger("prototree store"),
		store:  store,
	}
	t.PeriodicTask = *process.NewPeriodicTask("PruneTxsTask", utils.NewStaticTicker(interval), t.pruneTxs)
	return t
}

func (t *pruneTxsTask) pruneTxs(ctx context.Context) {
	err := t.store.PruneTxSeenRecordsOlderThan(t.interval)
	if err != nil {
		t.Errorf("while pruning prototree store txs: %v", err)
	}
}

func (s *store) PruneTxSeenRecordsOlderThan(threshold time.Duration) error {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	node := s.db.State(true)
	defer node.Close()

	s.Debugf("pruning prototree store txs")

	for deviceUniqueID, y := range s.data.TxsSeenByPeers {
		for stateURI, z := range y {
			for txID, whenSeen := range z {
				t := time.Unix(int64(whenSeen), 0)
				if t.Add(threshold).Before(time.Now().UTC()) {
					delete(s.data.TxsSeenByPeers[deviceUniqueID][stateURI], txID)

					err := node.Delete(s.keypathForTxSeenByPeer(deviceUniqueID, stateURI, txID), nil)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return node.Save()
}
func (s *store) DebugPrint() {
	node := s.db.State(false)
	defer node.Close()
	node.NodeAt(storeRootKeypath, nil).DebugPrint(func(msg string, args ...interface{}) { fmt.Printf(msg, args...) }, true, 0)
}
