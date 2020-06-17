package redwood

import (
	"sync"
	"time"

	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/types"
)

type PeerStore interface {
	AddReachableAddresses(transportName string, reachableAt StringSet)
	AddVerifiedCredentials(transportName string, reachableAt StringSet, address types.Address, sigpubkey SigningPublicKey, encpubkey EncryptingPublicKey)
	MaybePeers() []PeerDetails
	PeerDialInfos() []PeerDialInfo
	PeersWithAddress(address types.Address) []PeerDetails
	PeersFromTransportWithAddress(transportName string, address types.Address) []PeerDetails
	PeerReachableAt(transportName string, reachableAt StringSet) PeerDetails
}

type peerStore struct {
	ctx.Logger

	muPeers          sync.RWMutex
	peers            map[PeerDialInfo]*peerDetails
	peersWithAddress map[types.Address]map[PeerDialInfo]*peerDetails
	maybePeers       map[PeerDialInfo]*peerDetails
}

type PeerDialInfo struct {
	TransportName string
	ReachableAt   string
}

func NewPeerStore() *peerStore {
	s := &peerStore{
		Logger:           ctx.NewLogger("peerstore"),
		peers:            make(map[PeerDialInfo]*peerDetails),
		peersWithAddress: make(map[types.Address]map[PeerDialInfo]*peerDetails),
		maybePeers:       make(map[PeerDialInfo]*peerDetails),
	}

	return s
}

func (s *peerStore) AddReachableAddresses(transportName string, reachableAt StringSet) {
	s.muPeers.Lock()
	defer s.muPeers.Unlock()

	for ra := range reachableAt {
		key := PeerDialInfo{transportName, ra}
		_, exists := s.peers[key]
		if !exists {
			s.maybePeers[key] = &peerDetails{transportName: transportName, reachableAt: reachableAt.Copy()}
		}
	}
}

func (s *peerStore) AddVerifiedCredentials(
	transportName string,
	reachableAt StringSet,
	address types.Address,
	sigpubkey SigningPublicKey,
	encpubkey EncryptingPublicKey,
) {
	s.muPeers.Lock()
	defer s.muPeers.Unlock()

	var peer *peerDetails
	if address != (types.Address{}) {
		for _, p := range s.peersWithAddress[address] {
			peer = p
			break
		}
	}
	if peer == nil {
		for ra := range reachableAt {
			if p, exists := s.peers[PeerDialInfo{transportName, ra}]; exists {
				peer = p
				break
			}
		}
	}
	if peer == nil {
		peer = &peerDetails{
			transportName: transportName,
			reachableAt:   NewStringSet(nil),
			address:       address,
		}

		// Coalesce the maybePeers into a single verified peer
		for ra := range reachableAt {
			otherPeer := s.maybePeers[PeerDialInfo{transportName, ra}]
			if otherPeer != nil {
				otherPeer.RLock()
				if otherPeer.failures > peer.failures {
					peer.failures = otherPeer.failures
				}
				if otherPeer.lastContact.After(peer.lastContact) {
					peer.lastContact = otherPeer.lastContact
				}
				if otherPeer.lastFailure.After(peer.lastFailure) {
					peer.lastFailure = otherPeer.lastFailure
				}
				otherPeer.RUnlock()
			}
			delete(s.maybePeers, PeerDialInfo{transportName, ra})
		}
	}
	peer.Lock()
	defer peer.Unlock()
	peer.address = address
	peer.sigpubkey = sigpubkey
	peer.encpubkey = encpubkey

	if _, exists := s.peersWithAddress[address]; !exists {
		s.peersWithAddress[address] = make(map[PeerDialInfo]*peerDetails)
	}

	if len(reachableAt) > 0 {
		for ra := range reachableAt {
			tuple := PeerDialInfo{transportName, ra}
			s.peers[tuple] = peer
			s.peersWithAddress[address][tuple] = peer
			peer.reachableAt.Add(ra)
		}
	} else {
		// @@TODO: this seems like a potentially very bad idea.  maybe remove the inner map entirely, change it to a slice, and iterate over it to check for dupes
		tuple := PeerDialInfo{TransportName: transportName}
		s.peersWithAddress[address][tuple] = peer
	}

	if peer.address != (types.Address{}) {
		for ra := range peer.reachableAt {
			delete(s.maybePeers, PeerDialInfo{transportName, ra})
		}
	}
}

func (s *peerStore) PeerReachableAt(transport string, reachableAt StringSet) PeerDetails {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	if len(reachableAt) == 0 {
		return nil
	}

	for ra := range reachableAt {
		peer, exists := s.peers[PeerDialInfo{transport, ra}]
		if exists {
			return peer
		}
	}

	for ra := range reachableAt {
		peer, exists := s.maybePeers[PeerDialInfo{transport, ra}]
		if exists {
			return peer
		}
	}
	return nil
}

func (s *peerStore) MaybePeers() []PeerDetails {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	maybePeers := make([]PeerDetails, len(s.maybePeers))
	i := 0
	for _, peer := range s.maybePeers {
		maybePeers[i] = peer
		i++
	}
	return maybePeers
}

func (s *peerStore) PeerDialInfos() []PeerDialInfo {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	var peers []PeerDialInfo
	for tuple := range s.peers {
		peers = append(peers, tuple)
	}
	for tuple := range s.maybePeers {
		peers = append(peers, tuple)
	}
	return peers
}

func (s *peerStore) PeersWithAddress(address types.Address) []PeerDetails {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	var peers []PeerDetails
	if _, exists := s.peersWithAddress[address]; exists {
		for _, peer := range s.peersWithAddress[address] {
			peers = append(peers, peer)
		}
	}
	return peers
}

func (s *peerStore) PeersFromTransportWithAddress(transport string, address types.Address) []PeerDetails {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	var peers []PeerDetails
	if _, exists := s.peersWithAddress[address]; exists {
		for _, peer := range s.peersWithAddress[address] {
			peer.RLock()
			if peer.transportName == transport {
				peers = append(peers, peer)
			}
			peer.RUnlock()
		}
	}
	return peers
}

type PeerDetails interface {
	Address() types.Address
	TransportName() string
	ReachableAt() StringSet
	DialInfos() []PeerDialInfo
	PublicKeypairs() (SigningPublicKey, EncryptingPublicKey)

	UpdateConnStats(success bool)
	LastContact() time.Time
	LastFailure() time.Time
	Failures() int
}

type peerDetails struct {
	sync.RWMutex

	transportName string
	reachableAt   StringSet
	address       types.Address
	sigpubkey     SigningPublicKey
	encpubkey     EncryptingPublicKey

	lastContact time.Time
	lastFailure time.Time
	failures    int
}

func (p *peerDetails) TransportName() string {
	p.RLock()
	defer p.RUnlock()
	return p.transportName
}

func (p *peerDetails) ReachableAt() StringSet {
	p.RLock()
	defer p.RUnlock()
	return p.reachableAt.Copy()
}

func (p *peerDetails) DialInfos() []PeerDialInfo {
	p.RLock()
	defer p.RUnlock()

	var tuples []PeerDialInfo
	for reachableAt := range p.reachableAt {
		tuples = append(tuples, PeerDialInfo{p.transportName, reachableAt})
	}
	return tuples
}

func (p *peerDetails) Address() types.Address {
	p.RLock()
	defer p.RUnlock()
	return p.address
}

func (p *peerDetails) PublicKeypairs() (SigningPublicKey, EncryptingPublicKey) {
	p.RLock()
	defer p.RUnlock()
	// @@TODO: needs copy
	return p.sigpubkey, p.encpubkey
}

func (p *peerDetails) UpdateConnStats(success bool) {
	p.Lock()
	defer p.Unlock()
	if success {
		p.lastContact = time.Now()
		p.failures = 0
	} else {
		p.lastFailure = time.Now()
		p.failures++
	}
}

func (p *peerDetails) LastContact() time.Time {
	p.RLock()
	defer p.RUnlock()
	return p.lastContact
}

func (p *peerDetails) LastFailure() time.Time {
	p.RLock()
	defer p.RUnlock()
	return p.lastFailure
}

func (p *peerDetails) Failures() int {
	p.RLock()
	defer p.RUnlock()
	return p.failures
}
