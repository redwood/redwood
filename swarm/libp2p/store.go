package libp2p

import (
	"fmt"
	"net/url"
	"sync"

	"redwood.dev/errors"
	"redwood.dev/log"
	"redwood.dev/state"
	"redwood.dev/types"
)

//go:generate mockery --name Store --output ./mocks/ --case=underscore
type Store interface {
	StaticRelays() types.StringSet
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
	StaticRelays types.StringSet `tree:"staticRelays"`
}

var storeRootKeypath = state.Keypath("libp2p")

func NewStore(db *state.DBTree) (*store, error) {
	s := &store{
		Logger: log.NewLogger("libp2p store"),
		db:     db,
	}
	s.Infof(0, "opening libp2p store")
	err := s.loadData()
	return s, err
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

	var staticRelays []string
	for staticRelayEncoded := range s.data.StaticRelays {
		staticRelay, err := url.QueryUnescape(staticRelayEncoded)
		if err != nil {
			s.Errorf("could not unescape static relay '%v'", staticRelayEncoded)
			continue
		}
		staticRelays = append(staticRelays, staticRelay)
	}
	s.data.StaticRelays = types.NewStringSet(staticRelays)

	return nil
}

func (s *store) StaticRelays() types.StringSet {
	s.dataMu.RLock()
	defer s.dataMu.RUnlock()
	return s.data.StaticRelays.Copy()
}

func (s *store) AddStaticRelay(staticRelay string) error {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	s.data.StaticRelays.Add(staticRelay)

	node := s.db.State(true)
	defer node.Close()

	err := node.Set(s.keypathForStaticRelay(staticRelay), nil, true)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) RemoveStaticRelay(staticRelay string) error {
	s.dataMu.Lock()
	defer s.dataMu.Unlock()

	s.data.StaticRelays.Remove(staticRelay)

	node := s.db.State(true)
	defer node.Close()

	err := node.Delete(s.keypathForStaticRelay(staticRelay), nil)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) keypathForStaticRelay(staticRelay string) state.Keypath {
	return storeRootKeypath.Pushs("staticRelays").Pushs(url.QueryEscape(staticRelay))
}

func (s *store) DebugPrint() {
	node := s.db.State(false)
	defer node.Close()
	node.NodeAt(storeRootKeypath, nil).DebugPrint(func(msg string, args ...interface{}) { fmt.Printf(msg, args...) }, true, 0)
}
