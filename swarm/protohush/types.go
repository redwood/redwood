package protohush

import (
	"bytes"

	"redwood.dev/crypto"
	"redwood.dev/errors"
	"redwood.dev/swarm/protohush/pb"
	"redwood.dev/types"
)

type (
	KeyBundle         = pb.KeyBundle
	PubkeyBundle      = pb.PubkeyBundle
	IndividualMessage = pb.IndividualMessage
	GroupMessage      = pb.GroupMessage
	X3DHPrivateKey    = pb.X3DHPrivateKey
	X3DHPublicKey     = pb.X3DHPublicKey
	RatchetPrivateKey = pb.RatchetPrivateKey
	RatchetPublicKey  = pb.RatchetPublicKey
)

var (
	RatchetPrivateKeyFromBytes = pb.RatchetPrivateKeyFromBytes
	RatchetPublicKeyFromBytes  = pb.RatchetPublicKeyFromBytes
	GenerateX3DHPrivateKey     = pb.GenerateX3DHPrivateKey
	GenerateRatchetPrivateKey  = pb.GenerateRatchetPrivateKey
)

type Session struct {
	MyAddress       types.Address  `tree:"myAddress"`
	MyBundleID      types.Hash     `tree:"myBundleID"`
	RemoteAddress   types.Address  `tree:"remoteAddress"`
	RemoteBundleID  types.Hash     `tree:"remoteBundleID"`
	EphemeralPubkey *X3DHPublicKey `tree:"ephemeralPubkey"`
	SharedKey       []byte         `tree:"sharedKey"`
	Revoked         bool           `tree:"revoked"`
	CreatedAt       uint64         `tree:"createdAt"`
}

func SessionID(aliceBundleID, bobBundleID types.Hash, ephemeralPubkey *X3DHPublicKey) types.Hash {
	if bytes.Compare(aliceBundleID.Bytes(), bobBundleID.Bytes()) > 0 {
		aliceBundleID, bobBundleID = bobBundleID, aliceBundleID
	}
	bs := bytes.Join([][]byte{
		aliceBundleID.Bytes(),
		bobBundleID.Bytes(),
		ephemeralPubkey.Bytes(),
	}, nil)
	return types.HashBytes(bs)
}

func (s Session) ID() types.Hash {
	return SessionID(s.MyBundleID, s.RemoteBundleID, s.EphemeralPubkey)
}

type SymEncKeyAndMessage struct {
	Key     crypto.SymEncKey `tree:"key"`
	Message crypto.SymEncMsg `tree:"message"`
}

type OutgoingGroupMessage struct {
	ID          string          `tree:"id"`
	MessageType string          `tree:"messageType"`
	Sender      types.Address   `tree:"sender"`
	Recipients  []types.Address `tree:"recipients"`
	Plaintext   []byte          `tree:"plaintext"`
}

type IncomingGroupMessage struct {
	ID          string       `tree:"id"`
	MessageType string       `tree:"messageType"`
	Message     GroupMessage `tree:"message"`
}

var (
	ErrRecipientIsSelf = errors.New("recipient is self")
)
