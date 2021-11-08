package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
)

const (
	AES128_KEY_LENGTH  = 16
	AES256_KEY_LENGTH  = 32
	AES_GCM_NONCE_SIZE = 12
)

// AES-256 symmetric encryption key
type SymEncKey [AES256_KEY_LENGTH]byte

func NewSymEncKey() (SymEncKey, error) {
	var key SymEncKey
	_, err := io.ReadFull(rand.Reader, key[:])
	if err != nil {
		return SymEncKey{}, err
	}
	return key, nil
}

func SymEncKeyFromBytes(bs []byte) SymEncKey {
	var k SymEncKey
	copy(k[:], bs)
	return k
}

func (key SymEncKey) Bytes() []byte {
	return key[:]
}

func (key SymEncKey) Encrypt(plaintext []byte) (SymEncMsg, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return SymEncMsg{}, err
	}
	aesgcm, err := cipher.NewGCMWithNonceSize(block, AES_GCM_NONCE_SIZE)
	if err != nil {
		return SymEncMsg{}, err
	}

	// @@TODO: Never use more than 2^32 random nonces with a given key because of the risk of a repeat.
	nonce := make([]byte, aesgcm.NonceSize())
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return SymEncMsg{}, err
	}
	msg := SymEncMsg{
		Ciphertext: aesgcm.Seal(nil, nonce, plaintext, nil),
		Nonce:      nonce,
	}
	return msg, nil
}

func (key SymEncKey) Decrypt(msg SymEncMsg) ([]byte, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return aesgcm.Open(nil, msg.Nonce, msg.Ciphertext, nil)
}

func (key SymEncKey) MarshalJSON() ([]byte, error) {
	return []byte(`"` + hex.EncodeToString(key[:]) + `"`), nil
}

func (key *SymEncKey) UnmarshalJSON(hexBytes []byte) error {
	if len(hexBytes) != (32*2)+2 {
		return errors.New("bad symmetric encryption key length")
	}
	bytes, err := hex.DecodeString(string(hexBytes[1:65]))
	if err != nil {
		return err
	}
	copy((*key)[:], bytes)
	return nil
}

// An AES-256 symmetrically encrypted message
type SymEncMsg struct {
	Nonce      []byte
	Ciphertext []byte
}

func SymEncMsgFromBytes(bs []byte) SymEncMsg {
	var msg SymEncMsg
	nonceLen := binary.LittleEndian.Uint64(bs[0:8])
	msg.Nonce = make([]byte, nonceLen)
	copy(msg.Nonce, bs[8:8+nonceLen])
	ciphertextLen := binary.LittleEndian.Uint64(bs[8+nonceLen : 16+nonceLen])
	msg.Ciphertext = make([]byte, ciphertextLen)
	copy(msg.Ciphertext, bs[16+nonceLen:16+nonceLen+ciphertextLen])
	return msg
}

func (msg SymEncMsg) Bytes() []byte {
	bytes := make([]byte, 8+len(msg.Nonce)+8+len(msg.Ciphertext))
	binary.LittleEndian.PutUint64(bytes[0:8], uint64(len(msg.Nonce)))
	copy(bytes[8:], msg.Nonce)
	offset := 8 + len(msg.Nonce)
	binary.LittleEndian.PutUint64(bytes[offset:offset+8], uint64(len(msg.Ciphertext)))
	copy(bytes[offset+8:], msg.Ciphertext)
	return bytes
}
