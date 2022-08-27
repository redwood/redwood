package protoauth

import (
	"bytes"

	"redwood.dev/crypto"
	"redwood.dev/types"
)

type ChallengeIdentityResponse struct {
	Challenge     crypto.ChallengeMsg  `json:"challenge"`
	Signature     crypto.Signature     `json:"signature"`
	AsymEncPubkey crypto.AsymEncPubkey `json:"encryptingPublicKey"`
}

func (r ChallengeIdentityResponse) Hash() types.Hash {
	bs := bytes.Join([][]byte{
		r.Challenge.Bytes(),
		r.AsymEncPubkey.Bytes(),
	}, nil)
	return types.HashBytes(bs)
}
