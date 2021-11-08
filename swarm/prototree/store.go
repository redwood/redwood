package prototree

import (
	"sync"

	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/state"
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
}

type store struct {
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
	SubscribedStateURIs     utils.StringSet
	MaxPeersPerSubscription uint64
	TxsSeenByPeers          map[string]map[string]map[types.ID]bool
}

type storeDataCodec struct {
	SubscribedStateURIs     []string                              `tree:"subscribedStateURIs"`
	MaxPeersPerSubscription uint64                                `tree:"maxPeersPerSubscription"`
	TxsSeenByPeers          map[string]map[string]map[string]bool `tree:"txsSeenByPeers"`
}

func NewStore(db *state.DBTree) (*store, error) {
	s := &store{
		Logger:                      log.NewLogger("prototree store"),
		db:                          db,
		subscribedStateURIListeners: make(map[*subscribedStateURIListener]struct{}),
	}
	err := s.loadData()
	return s, err
}

func (s *store) loadData() error {
	node := s.db.State(false)
	defer node.Close()

	var codec storeDataCodec
	err := node.NodeAt(state.Keypath("prototree"), nil).Scan(&codec)
	if err != nil {
		return err
	}

	txsSeenByPeers := make(map[string]map[string]map[types.ID]bool)
	for deviceSpecificID, x := range codec.TxsSeenByPeers {
		txsSeenByPeers[deviceSpecificID] = make(map[string]map[types.ID]bool)
		for stateURI, y := range x {
			txsSeenByPeers[deviceSpecificID][stateURI] = make(map[types.ID]bool)
			for txIDStr, seen := range y {
				txID, err := types.IDFromHex(txIDStr)
				if err != nil {
					s.Errorf("while unmarshaling tx ID: %v", err)
					continue
				}
				txsSeenByPeers[deviceSpecificID][stateURI][txID] = seen
			}
		}
	}

	s.dataMu.Lock()
	defer s.dataMu.Unlock()
	s.data = storeData{
		SubscribedStateURIs:     utils.NewStringSet(codec.SubscribedStateURIs),
		MaxPeersPerSubscription: codec.MaxPeersPerSubscription,
		TxsSeenByPeers:          txsSeenByPeers,
	}
	return nil
}

func (s *store) saveData() error {
	txsSeenByPeers := make(map[string]map[string]map[string]bool)
	for deviceSpecificID, y := range s.data.TxsSeenByPeers {
		txsSeenByPeers[deviceSpecificID] = make(map[string]map[string]bool)
		for stateURI, z := range y {
			txsSeenByPeers[deviceSpecificID][stateURI] = make(map[string]bool)
			for txID, seen := range z {
				txsSeenByPeers[deviceSpecificID][stateURI][txID.Hex()] = seen
			}
		}
	}

	codec := storeDataCodec{
		SubscribedStateURIs:     s.data.SubscribedStateURIs.Slice(),
		MaxPeersPerSubscription: s.data.MaxPeersPerSubscription,
		TxsSeenByPeers:          txsSeenByPeers,
	}

	node := s.db.State(true)
	defer node.Close()

	err := node.Set(state.Keypath("prototree"), nil, codec)
	if err != nil {
		return err
	}
	return node.Save()
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
	return s.saveData()
}

func (s *store) RemoveSubscribedStateURI(stateURI string) error {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	s.data.SubscribedStateURIs.Remove(stateURI)
	return s.saveData()
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
	return s.saveData()
}

func (s *store) TxSeenByPeer(deviceSpecificID, stateURI string, txID types.ID) bool {
	if len(deviceSpecificID) == 0 {
		return false
	}

	s.dataMu.RLock()
	defer s.dataMu.RUnlock()

	if _, exists := s.data.TxsSeenByPeers[deviceSpecificID]; !exists {
		return false
	}
	if _, exists := s.data.TxsSeenByPeers[deviceSpecificID][stateURI]; !exists {
		return false
	}
	return s.data.TxsSeenByPeers[deviceSpecificID][stateURI][txID]
}

func (s *store) MarkTxSeenByPeer(deviceSpecificID, stateURI string, txID types.ID) error {
	if len(deviceSpecificID) == 0 {
		return nil
	}

	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	if _, exists := s.data.TxsSeenByPeers[deviceSpecificID]; !exists {
		s.data.TxsSeenByPeers[deviceSpecificID] = make(map[string]map[types.ID]bool)
	}
	if _, exists := s.data.TxsSeenByPeers[deviceSpecificID][stateURI]; !exists {
		s.data.TxsSeenByPeers[deviceSpecificID][stateURI] = make(map[types.ID]bool)
	}
	s.data.TxsSeenByPeers[deviceSpecificID][stateURI][txID] = true

	return s.saveData()
}
