package redwood

import (
	"sync"

	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/types"
)

type PeerStore interface {
	AddReachableAddresses(transportName string, reachableAt StringSet)
	AddVerifiedCredentials(transportName string, reachableAt StringSet, address types.Address, sigpubkey SigningPublicKey, encpubkey EncryptingPublicKey)
	MaybePeers() []peerTuple
	PeerTuples() []peerTuple
	PeersWithAddress(address types.Address) []*storedPeer
	PeersFromTransportWithAddress(transport string, address types.Address) []*storedPeer
	PeerReachableAt(transport string, reachableAt StringSet) *storedPeer
}

type peerStore struct {
	ctx.Logger

	muPeers          sync.RWMutex
	peers            map[peerTuple]*storedPeer
	peersWithAddress map[types.Address]map[peerTuple]*storedPeer
	maybePeers       map[peerTuple]struct{}
}

type peerTuple struct {
	TransportName string
	ReachableAt   string
}

func NewPeerStore(addr types.Address) *peerStore {
	s := &peerStore{
		Logger:           ctx.NewLogger("peer store " + addr.Pretty()),
		peers:            make(map[peerTuple]*storedPeer),
		peersWithAddress: make(map[types.Address]map[peerTuple]*storedPeer),
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

func (s *peerStore) AddVerifiedCredentials(transportName string, reachableAt StringSet, address types.Address, sigpubkey SigningPublicKey, encpubkey EncryptingPublicKey) {
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
		peer = &storedPeer{
			transportName: transportName,
			reachableAt:   NewStringSet(nil),
			address:       address,
		}
	}
	peer.address = address
	peer.sigpubkey = sigpubkey
	peer.encpubkey = encpubkey

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
}

func (s *peerStore) PeerReachableAt(transport string, reachableAt StringSet) *storedPeer {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	for ra := range reachableAt {
		peer, exists := s.peers[peerTuple{transport, ra}]
		if exists {
			return peer.Copy()
		}
	}
	return nil
}

func (s *peerStore) MaybePeers() []peerTuple {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	tuples := make([]peerTuple, len(s.maybePeers))
	i := 0
	for tuple := range s.maybePeers {
		tuples[i] = tuple
		i++
	}
	return tuples
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

func (s *peerStore) PeersWithAddress(address types.Address) []*storedPeer {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	var peers []*storedPeer
	if _, exists := s.peersWithAddress[address]; exists {
		for _, peer := range s.peersWithAddress[address] {
			peers = append(peers, peer.Copy())
		}
	}
	return peers
}

func (s *peerStore) PeersFromTransportWithAddress(transport string, address types.Address) []*storedPeer {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	var peers []*storedPeer
	if _, exists := s.peersWithAddress[address]; exists {
		for _, peer := range s.peersWithAddress[address] {
			if peer.transportName == transport {
				peers = append(peers, peer.Copy())
			}
		}
	}
	return peers
}

type storedPeer struct {
	transportName string
	reachableAt   StringSet
	address       types.Address
	sigpubkey     SigningPublicKey
	encpubkey     EncryptingPublicKey
}

func (p storedPeer) Address() types.Address {
	return p.address
}

func (p *storedPeer) SetAddress(addr types.Address) {
	p.address = addr
}

func (p storedPeer) PublicKeypairs() (SigningPublicKey, EncryptingPublicKey) {
	return p.sigpubkey, p.encpubkey
}

func (sp storedPeer) Tuples() []peerTuple {
	var tuples []peerTuple
	for reachableAt := range sp.reachableAt {
		tuples = append(tuples, peerTuple{sp.transportName, reachableAt})
	}
	return tuples
}

func (p *storedPeer) Copy() *storedPeer {
	if p == nil {
		return nil
	}
	return &storedPeer{
		transportName: p.transportName,
		reachableAt:   p.reachableAt.Copy(),
		address:       p.address,
		sigpubkey:     p.sigpubkey, // @@TODO: need copy
		encpubkey:     p.encpubkey, // @@TODO: need copy
	}
}
