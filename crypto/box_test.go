package crypto_test

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
	"redwood.dev/crypto"
)

func TestEncryptingKeyConstructors(t *testing.T) {
	bytes := []byte("12345678901234567890123456789012")

	pubkey := crypto.EncryptingPublicKeyFromBytes(bytes)
	require.Equal(t, bytes, pubkey.Bytes())

	privkey := crypto.EncryptingPrivateKeyFromBytes(bytes)
	require.Equal(t, bytes, privkey.Bytes())

	hex := hex.EncodeToString(bytes)

	pubkey, err := crypto.EncryptingPublicKeyFromHex(hex)
	require.NoError(t, err)
	require.Equal(t, bytes, pubkey.Bytes())

	privkey, err = crypto.EncryptingPrivateKeyFromHex(hex)
	require.NoError(t, err)
	require.Equal(t, bytes, privkey.Bytes())
}
