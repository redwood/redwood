package pb

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/status-im/doubleratchet"

	"redwood.dev/crypto"
	"redwood.dev/errors"
	"redwood.dev/types"
)

func (id IndividualSessionID) String() string {
	return strings.Join([]string{
		id.SessionType,
		id.AliceAddr.Hex(),
		id.BobAddr.Hex(),
		fmt.Sprintf("%v", id.Epoch),
	}, "-")
}

func (id IndividualSessionID) Bytes() []byte {
	return []byte(id.String())
}

func (id IndividualSessionID) Hash() types.Hash {
	return types.HashBytes(id.Bytes())
}

func (id IndividualSessionID) MarshalText() ([]byte, error) {
	return id.Bytes(), nil
}

func (id *IndividualSessionID) UnmarshalText(bs []byte) error {
	parts := bytes.Split(bs, []byte("-"))
	if len(parts) != 4 {
		return errors.Errorf(`bad session ID: "%v"`, string(bs))
	}
	aliceAddr, err := types.AddressFromHex(string(parts[1]))
	if err != nil {
		return err
	}
	bobAddr, err := types.AddressFromHex(string(parts[2]))
	if err != nil {
		return err
	}
	epoch, err := strconv.ParseUint(string(parts[3]), 10, 64)
	if err != nil {
		return err
	}
	*id = IndividualSessionID{
		SessionType: string(parts[0]),
		AliceAddr:   aliceAddr,
		BobAddr:     bobAddr,
		Epoch:       epoch,
	}
	return nil
}

func (p DHPair) PrivateKey() doubleratchet.Key {
	return p.Private
}

func (p DHPair) PublicKey() doubleratchet.Key {
	return p.Public
}

func (p DHPair) AttestationHash() types.Hash {
	a := DHPubkeyAttestation{Pubkey: p.Public, Epoch: p.Epoch}
	bs, err := a.Marshal()
	if err != nil {
		panic("impossible")
	}
	// bs := bytes.Join([][]byte{p.Public, []byte(fmt.Sprintf("%v", p.Epoch))}, []byte("-"))
	return types.HashBytes(bs)
}

type SharedKey [32]byte

func SharedKeyFromBytes(bs []byte) (sk SharedKey) {
	copy(sk[:], bs)
	return sk
}

func GenerateSharedKey() (SharedKey, error) {
	var sk SharedKey
	n, err := rand.Read(sk[:])
	if err != nil {
		return SharedKey{}, err
	} else if n < len(SharedKey{}) {
		return SharedKey{}, errors.New("did not read enough entropy")
	}
	return sk, nil
}

func (t SharedKey) Bytes() []byte {
	return t[:]
}

func (t SharedKey) Marshal() ([]byte, error) { return t[:], nil }

func (t *SharedKey) MarshalTo(data []byte) (n int, err error) {
	copy(data, (*t)[:])
	return len(data), nil
}

func (t *SharedKey) Unmarshal(data []byte) error {
	*t = SharedKey{}
	copy((*t)[:], data)
	return nil
}

func (t *SharedKey) Size() int { return len(*t) }
func (t SharedKey) MarshalJSON() ([]byte, error) {
	return []byte(`"` + hex.EncodeToString(t[:]) + `"`), nil
}
func (t *SharedKey) UnmarshalJSON(data []byte) error {
	if len(data) < 3 {
		*t = SharedKey{}
		return nil
	}
	bs, err := hex.DecodeString(string(data[1 : len(data)-1]))
	if err != nil {
		return err
	}
	*t = SharedKeyFromBytes(bs)
	return nil
}
func (t SharedKey) Compare(other SharedKey) int { return bytes.Compare(t[:], other[:]) }
func (t SharedKey) Equal(other SharedKey) bool  { return bytes.Equal(t[:], other[:]) }

func NewPopulatedSharedKey(r randyHush) *SharedKey {
	sk, _ := GenerateSharedKey()
	return &sk
}

func (p IndividualSessionProposal) Hash() (types.Hash, error) {
	p2 := p
	p2.AliceSig = nil
	p2.RemoteDHPubkey = nil

	bs, err := p2.Marshal()
	if err != nil {
		return types.Hash{}, err
	}
	return types.HashBytes(bs), nil
}

func (a DHPubkeyAttestation) PubkeyHex() string {
	return hex.EncodeToString(a.Pubkey)
}

func (a DHPubkeyAttestation) Hash() (types.Hash, error) {
	withoutSig := DHPubkeyAttestation{Pubkey: a.Pubkey, Epoch: a.Epoch}
	bs, err := withoutSig.Marshal()
	if err != nil {
		return types.Hash{}, err
	}
	return types.HashBytes(bs), nil
}

func (a DHPubkeyAttestation) Address() (types.Address, error) {
	hash, err := a.Hash()
	if err != nil {
		return types.Address{}, err
	}
	pubkey, err := crypto.RecoverSigningPubkey(hash, a.Sig)
	if err != nil {
		return types.Address{}, err
	}
	return pubkey.Address(), nil
}

func IndividualMessageFromDoubleRatchetMessage(msg doubleratchet.Message, sessionHash types.Hash) IndividualMessage {
	return IndividualMessage{
		SessionHash: sessionHash,
		Header: IndividualMessage_Header{
			DhPubkey: msg.Header.DH,
			N:        msg.Header.N,
			Pn:       msg.Header.PN,
		},
		Ciphertext: msg.Ciphertext,
	}
}

func (m IndividualMessage) ToDoubleRatchetMessage() doubleratchet.Message {
	return doubleratchet.Message{
		Header: doubleratchet.MessageHeader{
			DH: m.Header.DhPubkey,
			N:  m.Header.N,
			PN: m.Header.Pn,
		},
		Ciphertext: m.Ciphertext,
	}
}

func (m IndividualMessage) Hash() (types.Hash, error) {
	bs, err := m.Marshal()
	if err != nil {
		return types.Hash{}, err
	}
	return types.HashBytes(bs), nil
}

func (m GroupMessage) Hash() (types.Hash, error) {
	m2 := m
	m2.Sig = nil

	bs, err := m2.Marshal()
	if err != nil {
		return types.Hash{}, err
	}
	return types.HashBytes(bs), nil
}

// func (p SharedKeyProposal) MarshalBinary() ([]byte, error) {
//     return (&pb.SharedKeyProposal{
//         SessionID: p.SessionID,
//         SharedKey: p.SharedKey[:],
//     }).Marshal()
// }

// func (p *SharedKeyProposal) UnmarshalBinary(bs []byte) error {
//     var proto pb.SharedKeyProposal
//     err := proto.Unmarshal(bs)
//     if err != nil {
//         return err
//     }
//     *p = SharedKeyProposal{
//         SessionID: proto.SessionID,
//         SharedKey: SharedKeyFromBytes(proto.SharedKey),
//     }
//     return nil
// }
