package swarm

import (
	"fmt"
	"strings"
)

const (
	PeerState_Unknown = peerState_Unknown
	PeerState_Strike  = peerState_Strike
	PeerState_InUse   = peerState_InUse
)

func (entry peersMapEntry[P]) State() peerState {
	return entry.state
}

func (p *peerPool[P]) PrintPeers(label string) map[PeerDialInfo]peersMapEntry[P] {
	peers := p.CopyPeers()
	lines := []string{label + " ======================"}
	for key, val := range peers {
		lines = append(lines, fmt.Sprintf("  - %v %v", key, val))
	}
	lines = append(lines, "=======================")
	fmt.Println(strings.Join(lines, "\n"))
	return peers
}

func (p *peerPool[P]) CopyPeers() map[PeerDialInfo]peersMapEntry[P] {
	p.peersMu.Lock()
	defer p.peersMu.Unlock()
	m := make(map[PeerDialInfo]peersMapEntry[P])
	for dialInfo, entry := range p.peers {
		m[dialInfo] = entry
	}
	return m
}

type ConcretePeerStore = peerStore

// func NewPeerDetails(
// 	peerStore *peerStore,
// 	dialInfo PeerDialInfo,
// 	deviceUniqueID string,
// 	addresses types.Set[types.Address],
// 	sigpubkeys map[types.Address]*crypto.SigningPublicKey,
// 	encpubkeys map[types.Address]*crypto.AsymEncPubkey,
// 	stateURIs types.Set[string],
// 	lastContact types.Time,
// 	lastFailure types.Time,
// 	failures uint64,
// ) *peerDetails {
// 	e := endpoint{
// 		Dialinfo:    dialInfo,
// 		Lastcontact: lastContact,
// 		Lastfailure: lastFailure,
// 		Fails:       failures,
// 	}
// 	pd := peerDetails{
// 		peerStore,
// 		deviceUniqueID,
// 		addresses,
// 		sigpubkeys,
// 		encpubkeys,
// 		stateURIs,
// 		map[PeerDialInfo]*endpoint{
// 			dialInfo: &e,
// 		},
// 	}
// 	e.peerDetails = &pd
// 	return &pd
// }
