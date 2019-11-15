package redwood

import (
	"sync"

	"github.com/brynbellomy/redwood/ctx"
)

type PeerStore interface {
	AddReachableAddresses(transportName string, reachableAt StringSet)
	AddVerifiedCredentials(transportName string, reachableAt StringSet, address Address, sigpubkey SigningPublicKey, encpubkey EncryptingPublicKey)
	PeerTuples() []peerTuple
	PeersWithAddress(address Address) []*storedPeer
}

type peerStore struct {
	ctx.Logger

	muPeers          sync.RWMutex
	peers            map[peerTuple]*storedPeer
	peersWithAddress map[Address]map[peerTuple]*storedPeer
	maybePeers       map[peerTuple]struct{}
}

type peerTuple struct {
	TransportName string
	ReachableAt   string
}

type storedPeer struct {
	transportName string
	reachableAt   StringSet
	address       Address
	sigpubkey     SigningPublicKey
	encpubkey     EncryptingPublicKey
}

func NewPeerStore(addr Address) *peerStore {
	s := &peerStore{
		Logger:           ctx.NewLogger("peer store " + addr.Pretty()),
		peers:            make(map[peerTuple]*storedPeer),
		peersWithAddress: make(map[Address]map[peerTuple]*storedPeer),
		maybePeers:       make(map[peerTuple]struct{}),
	}

	return s
}

func peerTuples(peer Peer) []peerTuple {
	var tuples []peerTuple
	transportName := peer.Transport().Name()
	for reachableAt := range peer.ReachableAt() {
		tuples = append(tuples, peerTuple{transportName, reachableAt})
	}
	return tuples
}

func (s *peerStore) AddReachableAddresses(transportName string, reachableAt StringSet) {
	s.muPeers.Lock()
	defer s.muPeers.Unlock()

	for ra := range reachableAt {
		key := peerTuple{transportName, ra}
		_, exists := s.peers[key]
		if !exists {
			s.maybePeers[key] = struct{}{}
		}
	}
}

func (s *peerStore) AddVerifiedCredentials(transportName string, reachableAt StringSet, address Address, sigpubkey SigningPublicKey, encpubkey EncryptingPublicKey) {
	s.muPeers.Lock()
	defer s.muPeers.Unlock()

	if len(reachableAt) == 0 {
		return
	}

	var peer *storedPeer
	for ra := range reachableAt {
		if p, exists := s.peers[peerTuple{transportName, ra}]; exists {
			peer = p
			break
		}
	}
	if peer == nil {
		peer = &storedPeer{transportName: transportName, reachableAt: NewStringSet(nil)}
	}

	if _, exists := s.peersWithAddress[address]; !exists {
		s.peersWithAddress[address] = make(map[peerTuple]*storedPeer)
	}

	for ra := range reachableAt {
		tuple := peerTuple{transportName, ra}
		delete(s.maybePeers, tuple)
		s.peers[tuple] = peer
		s.peersWithAddress[address][tuple] = peer
		peer.reachableAt.Add(ra)
	}

	peer.address = address
	peer.sigpubkey = sigpubkey
	peer.encpubkey = encpubkey
}

func (s *peerStore) PeerTuples() []peerTuple {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	var peers []peerTuple
	for tuple := range s.peers {
		peers = append(peers, tuple)
	}
	for tuple := range s.maybePeers {
		peers = append(peers, tuple)
	}
	return peers
}

func (s *peerStore) PeersWithAddress(address Address) []*storedPeer {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	var peers []*storedPeer
	if _, exists := s.peersWithAddress[address]; exists {
		for _, peer := range s.peersWithAddress[address] {
			peers = append(peers, peer)
		}
	}
	return peers
}

func (sp *storedPeer) Tuples() []peerTuple {
	var tuples []peerTuple
	for reachableAt := range sp.reachableAt {
		tuples = append(tuples, peerTuple{sp.transportName, reachableAt})
	}
	return tuples
}
