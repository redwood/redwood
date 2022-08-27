package libp2p

import (
	"fmt"
	"net/url"
	"sync"

	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/state"
	. "redwood.dev/utils/generics"
)

//go:generate mockery --name Store --output ./mocks/ --case=underscore
type Store interface {
	StaticRelays() PeerSet
	AddStaticRelay(relayAddr string) error
	RemoveStaticRelay(relayAddr string) error

	DebugPrint()
}

type store struct {
	log.Logger
	db     *state.DBTree
	data   storeData
	dataMu sync.RWMutex
}

type storeData struct {
	StaticRelays PeerSet
}

type storeDataCodec struct {
	StaticRelays Set[string] `tree:"staticRelays"`
}

func NewStore(db *state.DBTree) (*store, error) {
	s := &store{
		Logger: log.NewLogger("libp2p store"),
		db:     db,
	}
	s.Infof("opening libp2p store")
	err := s.loadData()
	return s, err
}

func (s *store) loadData() error {
	node := s.db.State(false)
	defer node.Close()

	var codec storeDataCodec
	err := node.NodeAt(storeRootKeypath, nil).Scan(&codec)
	if errors.Cause(err) == errors.Err404 {
		// do nothing
	} else if err != nil {
		return err
	}

	var decoded []string
	for staticRelayEncoded := range codec.StaticRelays {
		staticRelay, err := url.QueryUnescape(staticRelayEncoded)
		if err != nil {
			s.Errorf("could not unescape static relay '%v'", staticRelayEncoded)
			continue
		}
		decoded = append(decoded, staticRelay)
	}
	s.data.StaticRelays, err = NewPeerSetFromStrings(decoded)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) StaticRelays() PeerSet {
	// s.dataMu.RLock()
	// defer s.dataMu.RUnlock()
	return s.data.StaticRelays
}

func (s *store) AddStaticRelay(staticRelay string) error {
	// s.dataMu.Lock()
	// defer s.dataMu.Unlock()

	err := s.data.StaticRelays.AddString(staticRelay)
	if err != nil {
		return err
	}

	node := s.db.State(true)
	defer node.Close()

	err = node.Set(s.keypathForStaticRelay(staticRelay), nil, true)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) RemoveStaticRelay(staticRelay string) error {
	// s.dataMu.Lock()
	// defer s.dataMu.Unlock()

	err := s.data.StaticRelays.RemoveString(staticRelay)
	if err != nil {
		return err
	}

	node := s.db.State(true)
	defer node.Close()

	err = node.Delete(s.keypathForStaticRelay(staticRelay), nil)
	if err != nil {
		return err
	}
	return node.Save()
}

var (
	storeRootKeypath    = state.Keypath("libp2p")
	staticRelaysKeypath = storeRootKeypath.Copy().Pushs("staticRelays")
)

func (s *store) keypathForStaticRelay(staticRelay string) state.Keypath {
	return staticRelaysKeypath.Copy().Pushs(url.QueryEscape(staticRelay))
}

func (s *store) DebugPrint() {
	node := s.db.State(false)
	defer node.Close()
	node.NodeAt(storeRootKeypath, nil).DebugPrint(func(msg string, args ...interface{}) { fmt.Printf(msg, args...) }, true, 0)
}
