package swarm

import (
	"fmt"
	"strings"
	"time"

	"redwood.dev/crypto"
	"redwood.dev/types"
	"redwood.dev/utils"
)

var NewPeerPool = newPeerPool

const (
	PeerState_Unknown = peerState_Unknown
	PeerState_Strike  = peerState_Strike
	PeerState_InUse   = peerState_InUse
)

func (entry peersMapEntry) State() peerState {
	return entry.state
}

func (p *peerPool) PrintPeers(label string) map[PeerDialInfo]peersMapEntry {
	peers := p.CopyPeers()
	lines := []string{label + " ======================"}
	for key, val := range peers {
		lines = append(lines, fmt.Sprintf("  - %v %v", key, val))
	}
	lines = append(lines, "=======================")
	fmt.Println(strings.Join(lines, "\n"))
	return peers
}

func (p *peerPool) CopyPeers() map[PeerDialInfo]peersMapEntry {
	p.peersMu.Lock()
	defer p.peersMu.Unlock()
	m := make(map[PeerDialInfo]peersMapEntry)
	for dialInfo, entry := range p.peers {
		m[dialInfo] = entry
	}
	return m
}

func (p *peerStore) FetchAllPeerDetails() ([]*peerDetails, error) {
	return p.fetchAllPeerDetails()
}

func (p *peerStore) FetchPeerDetails(dialInfo PeerDialInfo) (*peerDetails, error) {
	return p.fetchPeerDetails(dialInfo)
}

func (p *peerStore) SavePeerDetails(peerDetails *peerDetails) error {
	return p.savePeerDetails(peerDetails)
}

func NewPeerDetails(
	peerStore *peerStore,
	dialInfo PeerDialInfo,
	addresses utils.AddressSet,
	sigpubkeys map[types.Address]crypto.SigningPublicKey,
	encpubkeys map[types.Address]crypto.EncryptingPublicKey,
	stateURIs utils.StringSet,
	lastContact time.Time,
	lastFailure time.Time,
	failures uint64,
) *peerDetails {
	return &peerDetails{
		peerStore,
		dialInfo,
		addresses,
		sigpubkeys,
		encpubkeys,
		stateURIs,
		lastContact,
		lastFailure,
		failures,
	}
}
