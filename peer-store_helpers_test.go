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
	address types.Address,
	sigpubkey crypto.SigningPublicKey,
	encpubkey crypto.EncryptingPublicKey,
	stateURIs utils.StringSet,
	lastContact time.Time,
	lastFailure time.Time,
	failures uint64,
) *peerDetails {
	return &peerDetails{
		peerStore,
		dialInfo,
		address,
		sigpubkey,
		encpubkey,
		stateURIs,
		lastContact,
		lastFailure,
		failures,
	}
}
