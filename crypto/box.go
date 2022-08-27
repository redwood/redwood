package crypto

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"

	"golang.org/x/crypto/nacl/box"

	"redwood.dev/errors"
	"redwood.dev/types"
)

type (
	AsymEncPrivkey [NACL_BOX_KEY_LENGTH]byte
	AsymEncPubkey  [NACL_BOX_KEY_LENGTH]byte

	AsymEncKeypair struct {
		*AsymEncPrivkey
		*AsymEncPubkey
	}
)

const (
	NACL_BOX_KEY_LENGTH   = 32
	NACL_BOX_NONCE_LENGTH = 24
)

var (
	ErrCannotDecrypt = errors.New("cannot decrypt")
)

func GenerateAsymEncKeypair() (*AsymEncKeypair, error) {
	publicKey, privateKey, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &AsymEncKeypair{
		AsymEncPrivkey: (*AsymEncPrivkey)(privateKey),
		AsymEncPubkey:  (*AsymEncPubkey)(publicKey),
	}, nil
}

func AsymEncPubkeyFromBytes(bs []byte) (*AsymEncPubkey, error) {
	if len(bs) != NACL_BOX_KEY_LENGTH {
		return nil, errors.Errorf("bad length for AsymEncPubkey (%v)", len(bs))
	}
	var pk AsymEncPubkey
	copy(pk[:], bs)
	return &pk, nil
}

func AsymEncPubkeyFromHex(s string) (*AsymEncPubkey, error) {
	bs, err := hex.DecodeString(s)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return AsymEncPubkeyFromBytes(bs)
}

func (pubkey *AsymEncPubkey) Bytes() []byte {
	k := *pubkey
	return k[:]
}

func (pubkey *AsymEncPubkey) Hex() string {
	return hex.EncodeToString(pubkey.Bytes())
}

func (pubkey *AsymEncPubkey) String() string {
	return pubkey.Hex()
}

func (pubkey *AsymEncPubkey) UnmarshalBinary(bs []byte) (err error) {
	k, err := AsymEncPubkeyFromBytes(bs)
	if err != nil {
		return err
	}
	*pubkey = *k
	return nil
}

func (pubkey *AsymEncPubkey) MarshalBinary() ([]byte, error) {
	return pubkey.Bytes(), nil
}

func (pubkey *AsymEncPubkey) MarshalJSON() ([]byte, error) {
	return []byte(`"` + pubkey.Hex() + `"`), nil
}

func (pk *AsymEncPubkey) UnmarshalJSON(bs []byte) error {
	var s string
	err := json.Unmarshal(bs, &s)
	if err != nil {
		return err
	}
	k, err := AsymEncPubkeyFromHex(s)
	if err != nil {
		return err
	}
	*pk = *k
	return nil
}

func (pubkey *AsymEncPubkey) UnmarshalStateBytes(bs []byte) error {
	return pubkey.UnmarshalBinary(bs)
}

func (pubkey AsymEncPubkey) MarshalStateBytes() ([]byte, error) {
	return pubkey.MarshalBinary()
}

func (pk AsymEncPubkey) Copy() AsymEncPubkey {
	return pk
}

func (pk AsymEncPubkey) Marshal() ([]byte, error) {
	return pk.Bytes(), nil
}

func (pk AsymEncPubkey) MarshalTo(data []byte) (n int, err error) {
	copy(data, pk.Bytes())
	return len(pk), nil
}

func (pk *AsymEncPubkey) Unmarshal(data []byte) error {
	if len(data) != NACL_BOX_KEY_LENGTH {
		return errors.Errorf("bad length for crypto.AsymEncPubkey (%v)", len(data))
	}
	copy((*pk)[:], data)
	return nil
}

func (pk *AsymEncPubkey) UnmarshalText(bs []byte) error {
	bytes, err := hex.DecodeString(string(bs))
	if err != nil {
		return err
	}
	return pk.Unmarshal(bytes)
}

func (pk *AsymEncPubkey) Size() int { return NACL_BOX_KEY_LENGTH }

func (pk AsymEncPubkey) Compare(other AsymEncPubkey) int {
	return bytes.Compare(pk[:], other[:])
}

func (pk AsymEncPubkey) Equal(other AsymEncPubkey) bool {
	return bytes.Equal(pk[:], other[:])
}

func AsymEncPrivkeyFromBytes(bs []byte) *AsymEncPrivkey {
	var pk AsymEncPrivkey
	copy(pk[:], bs)
	return &pk
}

func AsymEncPrivkeyFromHex(s string) (*AsymEncPrivkey, error) {
	bs, err := hex.DecodeString(s)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var pk AsymEncPrivkey
	copy(pk[:], bs)
	return &pk, nil
}

func (privkey *AsymEncPrivkey) Bytes() []byte {
	bs := make([]byte, NACL_BOX_KEY_LENGTH)
	copy(bs, (*privkey)[:])
	return bs
}

func (privkey *AsymEncPrivkey) SealMessageFor(recipientPubKey *AsymEncPubkey, msg []byte) ([]byte, error) {
	// The shared key can be used to speed up processing when using the same
	// pair of keys repeatedly.
	var sharedEncryptKey [NACL_BOX_KEY_LENGTH]byte
	box.Precompute(&sharedEncryptKey, (*[NACL_BOX_KEY_LENGTH]byte)(recipientPubKey), (*[NACL_BOX_KEY_LENGTH]byte)(privkey))

	// You must use a different nonce for each message you encrypt with the
	// same key. Since the nonce here is 192 bits long, a random value
	// provides a sufficiently small probability of repeats.
	var nonce [NACL_BOX_NONCE_LENGTH]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return nil, err
	}

	// This encrypts msg and appends the result to the nonce.
	encrypted := box.SealAfterPrecomputation(nonce[:], msg, &nonce, &sharedEncryptKey)

	return encrypted, nil
}

func (privkey *AsymEncPrivkey) OpenMessageFrom(senderPublicKey *AsymEncPubkey, msgEncrypted []byte) ([]byte, error) {
	// The shared key can be used to speed up processing when using the same
	// pair of keys repeatedly.
	var sharedDecryptKey [NACL_BOX_KEY_LENGTH]byte
	box.Precompute(&sharedDecryptKey, (*[NACL_BOX_KEY_LENGTH]byte)(senderPublicKey), (*[NACL_BOX_KEY_LENGTH]byte)(privkey))

	// The recipient can decrypt the message using the shared key. When you
	// decrypt, you must use the same nonce you used to encrypt the message.
	// One way to achieve this is to store the nonce alongside the encrypted
	// message. Above, we prefixed the message with the nonce.
	var decryptNonce [NACL_BOX_NONCE_LENGTH]byte
	copy(decryptNonce[:], msgEncrypted[:NACL_BOX_NONCE_LENGTH])
	decrypted, ok := box.OpenAfterPrecomputation(nil, msgEncrypted[NACL_BOX_NONCE_LENGTH:], &decryptNonce, &sharedDecryptKey)
	if !ok {
		return nil, ErrCannotDecrypt
	}
	return decrypted, nil
}

func NewPopulatedAsymEncPubkey(_ gogoprotobufTest) *AsymEncPubkey {
	k, err := AsymEncPubkeyFromBytes(types.RandomBytes(32))
	if err != nil {
		panic(err)
	}
	return k
}
