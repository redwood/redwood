package redwood

import (
	"time"

	"redwood.dev/crypto"
	"redwood.dev/types"
	"redwood.dev/utils"
)

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
