package crypto_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"redwood.dev/crypto"
	"redwood.dev/internal/testutils"
	"redwood.dev/state"
)

func TestSymEncKey(t *testing.T) {
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

func TestSymEnc_SaveToStateDB(t *testing.T) {
	db := testutils.SetupDBTree(t)
	defer db.DeleteDB()

	node := db.State(true)
	defer node.Close()

	type Blah struct {
		Key crypto.SymEncKey `tree:"key"`
		Msg crypto.SymEncMsg `tree:"msg"`
	}

	key, err := crypto.NewSymEncKey()
	require.NoError(t, err)

	plaintext := []byte(`foo bar baz`)
	ciphertext, err := key.Encrypt(plaintext)
	require.NoError(t, err)
	msg := crypto.SymEncMsgFromBytes(ciphertext.Bytes())

	b := Blah{key, msg}

	keypath := state.Keypath("foo")
	node.Set(keypath, nil, b)

	err = node.Save()
	require.NoError(t, err)
	node.Close()

	node = db.State(false)
	defer node.Close()

	var b2 Blah
	err = node.NodeAt(keypath, nil).Scan(&b2)
	require.NoError(t, err)

	require.Equal(t, b.Key, b2.Key)
	require.Equal(t, b.Msg, b2.Msg)
}
