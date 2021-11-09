package crypto_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"redwood.dev/crypto"
)

func TestAES(t *testing.T) {
	var (
		plaintext = []byte("foo bar")
	)

	key, err := crypto.NewSymEncKey()
	require.NoError(t, err)

	ciphertext, err := key.Encrypt(plaintext)
	require.NoError(t, err)

	msg := crypto.SymEncMsgFromBytes(ciphertext.Bytes())

	key2 := crypto.SymEncKeyFromBytes(key.Bytes())

	got, err := key2.Decrypt(msg)
	require.NoError(t, err)
	require.Equal(t, plaintext, got)
}
