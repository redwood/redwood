package prototree

import (
	"sync"

	"redwood.dev/log"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/types"
	"redwood.dev/utils"
)

//go:generate mockery --name Store --output ./mocks/ --case=underscore
type Store interface {
	SubscribedStateURIs() utils.StringSet
	AddSubscribedStateURI(stateURI string) error
	RemoveSubscribedStateURI(stateURI string) error
	MaxPeersPerSubscription() uint64
	SetMaxPeersPerSubscription(max uint64) error
	TxSeenByPeer(dialInfo swarm.PeerDialInfo, stateURI string, txID types.ID) bool
	MarkTxSeenByPeer(dialInfo swarm.PeerDialInfo, stateURI string, txID types.ID) error
}

type store struct {
	log.Logger
	db     *state.DBTree
	data   storeData
	muData sync.RWMutex
}

type storeData struct {
	SubscribedStateURIs     utils.StringSet
	MaxPeersPerSubscription uint64
	TxsSeenByPeers          map[string]map[string]map[string]map[types.ID]bool
}

type storeDataCodec struct {
	SubscribedStateURIs     []string                                         `tree:"subscribedStateURIs"`
	MaxPeersPerSubscription uint64                                           `tree:"maxPeersPerSubscription"`
	TxsSeenByPeers          map[string]map[string]map[string]map[string]bool `tree:"txsSeenByPeers"`
}

func NewStore(db *state.DBTree) (*store, error) {
	s := &store{
		Logger: log.NewLogger("prototree store"),
		db:     db,
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

	txsSeenByPeers := make(map[string]map[string]map[string]map[types.ID]bool)
	for transportName, x := range codec.TxsSeenByPeers {
		txsSeenByPeers[transportName] = make(map[string]map[string]map[types.ID]bool)
		for dialAddr, y := range x {
			txsSeenByPeers[transportName][dialAddr] = make(map[string]map[types.ID]bool)
			for stateURI, z := range y {
				txsSeenByPeers[transportName][dialAddr][stateURI] = make(map[types.ID]bool)
				for txIDStr, seen := range z {
					txID, err := types.IDFromHex(txIDStr)
					if err != nil {
						s.Errorf("while unmarshaling tx ID: %v", err)
						continue
					}
					txsSeenByPeers[transportName][dialAddr][stateURI][txID] = seen
				}
			}
		}
	}

	s.muData.Lock()
	defer s.muData.Unlock()
	s.data = storeData{
		SubscribedStateURIs:     utils.NewStringSet(codec.SubscribedStateURIs),
		MaxPeersPerSubscription: codec.MaxPeersPerSubscription,
		TxsSeenByPeers:          txsSeenByPeers,
	}
	return nil
}

func (s *store) saveData() error {
	txsSeenByPeers := make(map[string]map[string]map[string]map[string]bool)
	for transportName, x := range s.data.TxsSeenByPeers {
		txsSeenByPeers[transportName] = make(map[string]map[string]map[string]bool)
		for dialAddr, y := range x {
			txsSeenByPeers[transportName][dialAddr] = make(map[string]map[string]bool)
			for stateURI, z := range y {
				txsSeenByPeers[transportName][dialAddr][stateURI] = make(map[string]bool)
				for txID, seen := range z {
					txsSeenByPeers[transportName][dialAddr][stateURI][txID.Hex()] = seen
				}
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

func (s *store) SubscribedStateURIs() utils.StringSet {
	s.muData.RLock()
	defer s.muData.RUnlock()
	return s.data.SubscribedStateURIs.Copy()
}

func (s *store) AddSubscribedStateURI(stateURI string) error {
	s.muData.Lock()
	defer s.muData.Unlock()

	s.data.SubscribedStateURIs.Add(stateURI)
	return s.saveData()
}

func (s *store) RemoveSubscribedStateURI(stateURI string) error {
	s.muData.Lock()
	defer s.muData.Unlock()

	s.data.SubscribedStateURIs.Remove(stateURI)
	return s.saveData()
}

func (s *store) MaxPeersPerSubscription() uint64 {
	s.muData.RLock()
	defer s.muData.RUnlock()
	return s.data.MaxPeersPerSubscription
}

func (s *store) SetMaxPeersPerSubscription(max uint64) error {
	s.muData.Lock()
	defer s.muData.Unlock()
	s.data.MaxPeersPerSubscription = max
	return s.saveData()
}

func (s *store) TxSeenByPeer(dialInfo swarm.PeerDialInfo, stateURI string, txID types.ID) bool {
	if len(dialInfo.DialAddr) == 0 {
		return false
	}

	s.muData.RLock()
	defer s.muData.RUnlock()

	if _, exists := s.data.TxsSeenByPeers[dialInfo.TransportName]; !exists {
		return false
	}
	if _, exists := s.data.TxsSeenByPeers[dialInfo.TransportName][dialInfo.DialAddr]; !exists {
		return false
	}
	if _, exists := s.data.TxsSeenByPeers[dialInfo.TransportName][dialInfo.DialAddr][stateURI]; !exists {
		return false
	}
	return s.data.TxsSeenByPeers[dialInfo.TransportName][dialInfo.DialAddr][stateURI][txID]
}

func (s *store) MarkTxSeenByPeer(dialInfo swarm.PeerDialInfo, stateURI string, txID types.ID) error {
	if len(dialInfo.DialAddr) == 0 {
		return nil
	}

	s.muData.Lock()
	defer s.muData.Unlock()

	if _, exists := s.data.TxsSeenByPeers[dialInfo.TransportName]; !exists {
		s.data.TxsSeenByPeers[dialInfo.TransportName] = make(map[string]map[string]map[types.ID]bool)
	}
	if _, exists := s.data.TxsSeenByPeers[dialInfo.TransportName][dialInfo.DialAddr]; !exists {
		s.data.TxsSeenByPeers[dialInfo.TransportName][dialInfo.DialAddr] = make(map[string]map[types.ID]bool)
	}
	if _, exists := s.data.TxsSeenByPeers[dialInfo.TransportName][dialInfo.DialAddr][stateURI]; !exists {
		s.data.TxsSeenByPeers[dialInfo.TransportName][dialInfo.DialAddr][stateURI] = make(map[types.ID]bool)
	}
	s.data.TxsSeenByPeers[dialInfo.TransportName][dialInfo.DialAddr][stateURI][txID] = true

	return s.saveData()
}
