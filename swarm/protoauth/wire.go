package protoauth

import (
	"crypto/rand"
	"encoding/hex"

	"redwood.dev/errors"
)

type ChallengeMsg []byte

func GenerateChallengeMsg() ([]byte, error) {
	challengeMsg := make([]byte, 128)
	_, err := rand.Read(challengeMsg)
	return challengeMsg, err
}

func (c ChallengeMsg) MarshalJSON() ([]byte, error) {
	return []byte(`"` + hex.EncodeToString(c) + `"`), nil
}

func (c *ChallengeMsg) UnmarshalJSON(bs []byte) error {
	bs, err := hex.DecodeString(string(bs[1 : len(bs)-1]))
	if err != nil {
		return errors.WithStack(err)
	}
	*c = bs
	return nil
}

type ChallengeIdentityResponse struct {
	Signature     []byte `json:"signature"`
	AsymEncPubkey []byte `json:"encryptingPublicKey"`
}
