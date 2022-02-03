package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"io"

	"golang.org/x/crypto/nacl/box"

	"redwood.dev/errors"
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

func AsymEncPubkeyFromBytes(bs []byte) *AsymEncPubkey {
	var pk AsymEncPubkey
	copy(pk[:], bs)
	return &pk
}

func AsymEncPubkeyFromHex(s string) (*AsymEncPubkey, error) {
	bs, err := hex.DecodeString(s)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var pk AsymEncPubkey
	copy(pk[:], bs)
	return &pk, nil
}

func (pubkey *AsymEncPubkey) Bytes() []byte {
	bs := make([]byte, NACL_BOX_KEY_LENGTH)
	copy(bs, (*pubkey)[:])
	return bs
}

func (pubkey *AsymEncPubkey) Hex() string {
	return hex.EncodeToString(pubkey.Bytes())
}

func (pubkey *AsymEncPubkey) String() string {
	return pubkey.Hex()
}

func (pubkey *AsymEncPubkey) UnmarshalBinary(bs []byte) error {
	k := AsymEncPubkeyFromBytes(bs)
	*pubkey = *k
	return nil
}

func (pubkey *AsymEncPubkey) MarshalBinary() ([]byte, error) {
	return pubkey.Bytes(), nil
}

func (pubkey *AsymEncPubkey) MarshalJSON() ([]byte, error) {
	return []byte(`"` + pubkey.Hex() + `"`), nil
}

func (pubkey *AsymEncPubkey) UnmarshalStateBytes(bs []byte) error {
	return pubkey.UnmarshalBinary(bs)
}

func (pubkey AsymEncPubkey) MarshalStateBytes() ([]byte, error) {
	return pubkey.MarshalBinary()
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
