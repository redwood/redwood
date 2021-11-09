package protohush

import (
	"bytes"

	"redwood.dev/errors"
	"redwood.dev/swarm/protohush/pb"
	"redwood.dev/types"
)

type SharedKey = pb.SharedKey
type IndividualSessionID = pb.IndividualSessionID
type IndividualSessionProposal = pb.IndividualSessionProposal
type IndividualSessionApproval = pb.IndividualSessionApproval
type DHPair = pb.DHPair
type DHPubkeyAttestation = pb.DHPubkeyAttestation
type IndividualMessage = pb.IndividualMessage
type IndividualMessageHeader = pb.IndividualMessage_Header

// type GroupSessionID = pb.GroupSessionID
// type GroupSessionProposal = pb.GroupSessionProposal
type GroupMessage = pb.GroupMessage
type GroupMessage_EncryptionKey = pb.GroupMessage_EncryptionKey

var GenerateSharedKey = pb.GenerateSharedKey
var IndividualMessageFromDoubleRatchetMessage = pb.IndividualMessageFromDoubleRatchetMessage

type EncryptedIndividualSessionProposal struct {
	AliceAddr         types.Address `tree:"aliceAddr"`
	EncryptedProposal []byte        `tree:"encryptedProposal"`
}

func (p EncryptedIndividualSessionProposal) Hash() types.Hash {
	return types.HashBytes(append(p.AliceAddr.Bytes(), p.EncryptedProposal...))
}

type IndividualMessageIntent struct {
	ID          types.ID      `tree:"id"`
	SessionType string        `tree:"sessionType"`
	Recipient   types.Address `tree:"recipient"`
	Plaintext   []byte        `tree:"plaintext"`
}

type GroupMessageIntent struct {
	SessionType string          `tree:"sessionType"`
	ID          string          `tree:"id"`
	Recipients  []types.Address `tree:"recipients"`
	Plaintext   []byte          `tree:"plaintext"`
}

func addrsSorted(alice, bob types.Address) (types.Address, types.Address) {
	if bytes.Compare(alice.Bytes(), bob.Bytes()) < 0 {
		return alice, bob
	}
	return bob, alice
}

var (
	ErrRecipientIsSelf = errors.New("recipient is self")
)
