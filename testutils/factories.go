package testutils

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"redwood.dev/crypto"
	"redwood.dev/types"
)

func RandomAddress(t *testing.T) types.Address {
	t.Helper()

	bs := make([]byte, len(types.Address{}))
	n, err := rand.Read(bs)
	require.NoError(t, err)
	require.Equal(t, 20, n)

	return types.AddressFromBytes(bs)
}

func RandomSigningPublicKey(t *testing.T) crypto.SigningPublicKey {
	t.Helper()

	k, err := crypto.GenerateSigningKeypair()
	require.NoError(t, err)
	return k.SigningPublicKey
}

func RandomEncryptingPublicKey(t *testing.T) crypto.EncryptingPublicKey {
	t.Helper()

	k, err := crypto.GenerateEncryptingKeypair()
	require.NoError(t, err)
	return k.EncryptingPublicKey
}

func RandomTime(t *testing.T) time.Time {
	t.Helper()

	n := rand.Intn(1613680441*2) + 1613680441
	return time.Unix(int64(n), 0)
}

func RandomDuration(t *testing.T, max time.Duration) time.Duration {
	return time.Duration(rand.Intn(int(max)))
}

func RandomString(t *testing.T, n int) string {
	t.Helper()

	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}
