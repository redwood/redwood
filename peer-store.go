package redwood

import (
	"sync"
	"time"

	"redwood.dev/ctx"
	"redwood.dev/types"
)

type PeerStore interface {
	AddDialInfos(dialInfos []PeerDialInfo)
	AddVerifiedCredentials(dialInfo PeerDialInfo, address types.Address, sigpubkey SigningPublicKey, encpubkey EncryptingPublicKey)
	UnverifiedPeers() []PeerDetails
	AllDialInfos() []PeerDialInfo
	PeerWithDialInfo(dialInfo PeerDialInfo) PeerDetails
	PeersWithAddress(address types.Address) []PeerDetails
	PeersFromTransportWithAddress(transportName string, address types.Address) []PeerDetails
	IsKnownPeer(dialInfo PeerDialInfo) bool
	OnNewUnverifiedPeer(fn func(dialInfo PeerDialInfo))
}

type peerStore struct {
	ctx.Logger

	muPeers          sync.RWMutex
	peers            map[PeerDialInfo]*peerDetails
	peersWithAddress map[types.Address]map[PeerDialInfo]*peerDetails
	unverifiedPeers  map[PeerDialInfo]struct{}

	newUnverifiedPeerHandler func(dialInfo PeerDialInfo)
}

type PeerDialInfo struct {
	TransportName string
	DialAddr      string
}

func NewPeerStore() *peerStore {
	s := &peerStore{
		Logger:           ctx.NewLogger("peerstore"),
		peers:            make(map[PeerDialInfo]*peerDetails),
		peersWithAddress: make(map[types.Address]map[PeerDialInfo]*peerDetails),
		unverifiedPeers:  make(map[PeerDialInfo]struct{}),
	}

	return s
}

func (s *peerStore) OnNewUnverifiedPeer(fn func(dialInfo PeerDialInfo)) {
	s.newUnverifiedPeerHandler = fn
}

func (s *peerStore) AddDialInfos(dialInfos []PeerDialInfo) {
	s.muPeers.Lock()
	defer s.muPeers.Unlock()

	for _, dialInfo := range dialInfos {
		if dialInfo.DialAddr == "" {
			continue
		}

		_, exists := s.peers[dialInfo]
		if !exists {
			s.peers[dialInfo] = &peerDetails{peerStore: s, dialInfo: dialInfo}
			s.unverifiedPeers[dialInfo] = struct{}{}
			s.newUnverifiedPeerHandler(dialInfo)
		}
	}
}

func (s *peerStore) AddVerifiedCredentials(
	dialInfo PeerDialInfo,
	address types.Address,
	sigpubkey SigningPublicKey,
	encpubkey EncryptingPublicKey,
) {
	s.muPeers.Lock()
	defer s.muPeers.Unlock()

	var peer *peerDetails
	if address != (types.Address{}) {
		if _, exists := s.peersWithAddress[address]; exists {
			peer = s.peersWithAddress[address][dialInfo]
		}
	}
	if peer == nil {
		if p, exists := s.peers[dialInfo]; exists {
			peer = p
		}
	}
	if peer == nil {
		peer = &peerDetails{
			peerStore: s,
			dialInfo:  dialInfo,
		}
	}

	peer.peerStore = s
	peer.address = address
	peer.sigpubkey = sigpubkey
	peer.encpubkey = encpubkey

	if _, exists := s.peersWithAddress[address]; !exists {
		s.peersWithAddress[address] = make(map[PeerDialInfo]*peerDetails)
	}
	s.peersWithAddress[address][dialInfo] = peer

	if dialInfo.DialAddr != "" {
		s.peers[dialInfo] = peer
	}

	if peer.address != (types.Address{}) {
		delete(s.unverifiedPeers, peer.dialInfo)
	}
}

func (s *peerStore) PeerWithDialInfo(dialInfo PeerDialInfo) PeerDetails {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	if dialInfo.DialAddr == "" {
		return nil
	}
	return s.peers[dialInfo]
}

func (s *peerStore) UnverifiedPeers() []PeerDetails {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	unverifiedPeers := make([]PeerDetails, len(s.unverifiedPeers))
	i := 0
	for dialInfo := range s.unverifiedPeers {
		unverifiedPeers[i] = s.peers[dialInfo]
		i++
	}
	return unverifiedPeers
}

func (s *peerStore) AllDialInfos() []PeerDialInfo {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	var dialInfos []PeerDialInfo
	for di := range s.peers {
		dialInfos = append(dialInfos, di)
	}
	return dialInfos
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
		for dialInfo, peer := range s.peersWithAddress[address] {
			if dialInfo.TransportName == transport {
				peers = append(peers, peer)
			}
		}
	}
	return peers
}

func (s *peerStore) IsKnownPeer(dialInfo PeerDialInfo) bool {
	s.muPeers.RLock()
	defer s.muPeers.RUnlock()

	for di := range s.peers {
		if di == dialInfo {
			return true
		}
	}
	return false
}

type PeerDetails interface {
	Address() types.Address
	DialInfo() PeerDialInfo
	PublicKeypairs() (SigningPublicKey, EncryptingPublicKey)

	UpdateConnStats(success bool)
	LastContact() time.Time
	LastFailure() time.Time
	Failures() int
}

type peerDetails struct {
	peerStore   *peerStore
	dialInfo    PeerDialInfo
	address     types.Address
	sigpubkey   SigningPublicKey
	encpubkey   EncryptingPublicKey
	lastContact time.Time
	lastFailure time.Time
	failures    int
}

func (p *peerDetails) Address() types.Address {
	p.peerStore.muPeers.RLock()
	defer p.peerStore.muPeers.RUnlock()
	return p.address
}

func (p *peerDetails) PublicKeypairs() (SigningPublicKey, EncryptingPublicKey) {
	p.peerStore.muPeers.RLock()
	defer p.peerStore.muPeers.RUnlock()
	return p.sigpubkey, p.encpubkey
}

func (p *peerDetails) DialInfo() PeerDialInfo {
	p.peerStore.muPeers.RLock()
	defer p.peerStore.muPeers.RUnlock()
	return p.dialInfo
}

func (p *peerDetails) UpdateConnStats(success bool) {
	p.peerStore.muPeers.Lock()
	defer p.peerStore.muPeers.Unlock()
	if success {
		p.lastContact = time.Now()
		p.failures = 0
	} else {
		p.lastFailure = time.Now()
		p.failures++
	}
}

func (p *peerDetails) LastContact() time.Time {
	p.peerStore.muPeers.RLock()
	defer p.peerStore.muPeers.RUnlock()
	return p.lastContact
}

func (p *peerDetails) LastFailure() time.Time {
	p.peerStore.muPeers.RLock()
	defer p.peerStore.muPeers.RUnlock()
	return p.lastFailure
}

func (p *peerDetails) Failures() int {
	p.peerStore.muPeers.RLock()
	defer p.peerStore.muPeers.RUnlock()
	return p.failures
}
