package swarm

import (
	"fmt"
	"strings"

	"redwood.dev/crypto"
	"redwood.dev/types"
)

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

type ConcretePeerStore = peerStore

func (p *peerStore) FetchAllPeerDetails() ([]*peerDetails, error) {
	pds, err := p.fetchAllPeerDetails()
	if err != nil {
		return nil, err
	}
	var s []*peerDetails
	for _, pd := range pds {
		s = append(s, pd)
	}
	return s, nil
}

func (p *peerStore) SavePeerDetails(peerDetails *peerDetails) error {
	return p.savePeerDetails(peerDetails)
}

func (p *peerStore) EnsurePeerDetails(dialInfo PeerDialInfo, deviceUniqueID string) (pd *peerDetails, knownPeer bool, needsSave bool) {
	return p.ensurePeerDetails(dialInfo, deviceUniqueID)
}

func NewPeerDetails(
	peerStore *peerStore,
	dialInfo PeerDialInfo,
	deviceUniqueID string,
	addresses types.Set[types.Address],
	sigpubkeys map[types.Address]*crypto.SigningPublicKey,
	encpubkeys map[types.Address]*crypto.AsymEncPubkey,
	stateURIs types.Set[string],
	lastContact types.Time,
	lastFailure types.Time,
	failures uint64,
) *peerDetails {
	e := endpoint{
		Dialinfo:    dialInfo,
		Lastcontact: lastContact,
		Lastfailure: lastFailure,
		Fails:       failures,
	}
	pd := peerDetails{
		peerStore,
		deviceUniqueID,
		addresses,
		sigpubkeys,
		encpubkeys,
		stateURIs,
		map[PeerDialInfo]*endpoint{
			dialInfo: &e,
		},
	}
	e.peerDetails = &pd
	return &pd
}
