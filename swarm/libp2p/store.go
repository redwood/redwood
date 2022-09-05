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
	Relays() PeerSet
	AddRelay(relayAddr string) error
	RemoveRelay(relayAddr string) error

	DebugPrint()
}

type store struct {
	log.Logger
	db     *state.DBTree
	data   storeData
	dataMu sync.RWMutex
}

type storeData struct {
	Relays PeerSet
}

type storeDataCodec struct {
	Relays Set[string] `tree:"relays"`
}

func NewStore(db *state.DBTree) (*store, error) {
	s := &store{
		Logger: log.NewLogger("libp2p"),
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
	for relayEncoded := range codec.Relays {
		relay, err := url.QueryUnescape(relayEncoded)
		if err != nil {
			s.Errorf("could not unescape relay '%v'", relayEncoded)
			continue
		}
		decoded = append(decoded, relay)
	}
	s.data.Relays, err = NewPeerSetFromStrings(decoded)
	if err != nil {
		return err
	}
	return nil
}

func (s *store) Relays() PeerSet {
	return s.data.Relays
}

func (s *store) AddRelay(relay string) error {
	err := s.data.Relays.AddString(relay)
	if err != nil {
		return err
	}

	node := s.db.State(true)
	defer node.Close()

	err = node.Set(s.keypathForRelay(relay), nil, true)
	if err != nil {
		return err
	}
	return node.Save()
}

func (s *store) RemoveRelay(relay string) error {
	err := s.data.Relays.RemoveString(relay)
	if err != nil {
		return err
	}

	node := s.db.State(true)
	defer node.Close()

	err = node.Delete(s.keypathForRelay(relay), nil)
	if err != nil {
		return err
	}
	return node.Save()
}

var (
	storeRootKeypath = state.Keypath("libp2p")
	relaysKeypath    = storeRootKeypath.Copy().Pushs("relays")
)

func (s *store) keypathForRelay(relay string) state.Keypath {
	return relaysKeypath.Copy().Pushs(url.QueryEscape(relay))
}

func (s *store) DebugPrint() {
	node := s.db.State(false)
	defer node.Close()
	node.NodeAt(storeRootKeypath, nil).DebugPrint(func(msg string, args ...interface{}) { fmt.Printf(msg, args...) }, true, 0)
}
