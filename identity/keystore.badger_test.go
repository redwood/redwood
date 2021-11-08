package identity_test

import (
	"encoding/json"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/status-im/doubleratchet"
	"github.com/stretchr/testify/require"

	"redwood.dev/errors"
	"redwood.dev/identity"
	"redwood.dev/internal/testutils"
	"redwood.dev/state"
	"redwood.dev/types"
)

func TestBadgerKeyStore_ErrorsWhenLocked(t *testing.T) {
	ks := identity.NewBadgerKeyStore(t.TempDir(), identity.InsecureScryptParams)

	_, err := ks.Identities()
	require.True(t, errors.Cause(err) == identity.ErrLocked)

	_, err = ks.PublicIdentities()
	require.True(t, errors.Cause(err) == identity.ErrLocked)

	_, err = ks.DefaultPublicIdentity()
	require.True(t, errors.Cause(err) == identity.ErrLocked)

	_, err = ks.IdentityWithAddress(types.Address{})
	require.True(t, errors.Cause(err) == identity.ErrLocked)

	_, err = ks.IdentityExists(types.Address{})
	require.True(t, errors.Cause(err) == identity.ErrLocked)

	_, err = ks.NewIdentity(true)
	require.True(t, errors.Cause(err) == identity.ErrLocked)

	_, err = ks.SignHash(types.Address{}, types.Hash{})
	require.True(t, errors.Cause(err) == identity.ErrLocked)

	_, err = ks.VerifySignature(types.Address{}, types.Hash{}, nil)
	require.True(t, errors.Cause(err) == identity.ErrLocked)

	_, err = ks.SealMessageFor(types.Address{}, nil, nil)
	require.True(t, errors.Cause(err) == identity.ErrLocked)

	_, err = ks.OpenMessageFrom(types.Address{}, nil, nil)
	require.True(t, errors.Cause(err) == identity.ErrLocked)
}

func TestBadgerKeyStore_Unlock(t *testing.T) {
	t.Run("empty keystore unlocks successfully", func(t *testing.T) {
		ks := identity.NewBadgerKeyStore(t.TempDir(), identity.InsecureScryptParams)
		defer ks.Close()

		err := ks.Unlock("password", "")
		require.NoError(t, err)
	})

	t.Run("empty keystore creates a default public identity when unlocked", func(t *testing.T) {
		ks := identity.NewBadgerKeyStore(t.TempDir(), identity.InsecureScryptParams)
		defer ks.Close()

		err := ks.Unlock("password", "")
		require.NoError(t, err)

		identities, err := ks.Identities()
		require.NoError(t, err)
		require.Len(t, identities, 1)

		id := identities[0]

		require.NotEqual(t, types.Address{}, id.Address())
		require.True(t, id.Public)

		pubIds, err := ks.PublicIdentities()
		require.NoError(t, err)
		require.Len(t, pubIds, 1)
		require.Equal(t, pubIds[0], id)

		defaultId, err := ks.DefaultPublicIdentity()
		require.NoError(t, err)
		require.Equal(t, defaultId, id)

		idWithAddr, err := ks.IdentityWithAddress(id.Address())
		require.NoError(t, err)
		require.Equal(t, idWithAddr, id)

		exists, err := ks.IdentityExists(id.Address())
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("will not unlock with an incorrect password", func(t *testing.T) {
		dir := t.TempDir()
		func() {
			ks := identity.NewBadgerKeyStore(dir, identity.InsecureScryptParams)
			defer ks.Close()

			err := ks.Unlock("password", "")
			require.NoError(t, err)
		}()

		ks := identity.NewBadgerKeyStore(dir, identity.InsecureScryptParams)
		err := ks.Unlock("alsdkjflsdkjf", "")
		require.Error(t, err)
	})

	t.Run("fetches existing identities from the DB", func(t *testing.T) {
		dir := t.TempDir()
		var ids []identity.Identity
		func() {
			ks := identity.NewBadgerKeyStore(dir, identity.InsecureScryptParams)
			defer ks.Close()

			err := ks.Unlock("password", "")
			require.NoError(t, err)

			id1, err := ks.DefaultPublicIdentity()
			require.NoError(t, err)

			id2, err := ks.NewIdentity(true)
			require.NoError(t, err)

			id3, err := ks.NewIdentity(false)
			require.NoError(t, err)

			ids, err = ks.Identities()
			require.NoError(t, err)
			require.Len(t, ids, 3)
			require.Equal(t, []identity.Identity{id1, id2, id3}, ids)
		}()

		ks := identity.NewBadgerKeyStore(dir, identity.InsecureScryptParams)
		err := ks.Unlock("password", "")
		require.NoError(t, err)

		expectedIds := ids

		ids, err = ks.Identities()
		require.NoError(t, err)
		require.Equal(t, expectedIds, ids)
	})
}

func TestBadgerKeyStore_NewIdentity(t *testing.T) {
	ks := identity.NewBadgerKeyStore(t.TempDir(), identity.InsecureScryptParams)
	err := ks.Unlock("password", "")
	require.NoError(t, err)
	defer ks.Close()

	ids, err := ks.Identities()
	require.NoError(t, err)

	id1 := ids[0]
	var id2 identity.Identity

	t.Run("creates public identities", func(t *testing.T) {
		id2, err = ks.NewIdentity(true)
		require.NoError(t, err)
		require.True(t, id2.Public)

		ids, err := ks.Identities()
		require.NoError(t, err)
		require.Len(t, ids, 2)
		require.Equal(t, []identity.Identity{id1, id2}, ids)

		pubIds, err := ks.PublicIdentities()
		require.NoError(t, err)
		require.Len(t, pubIds, 2)
		require.Equal(t, ids, pubIds)

		defaultPubId, err := ks.DefaultPublicIdentity()
		require.NoError(t, err)
		require.Equal(t, id1, defaultPubId)

		idWithAddr, err := ks.IdentityWithAddress(id2.Address())
		require.NoError(t, err)
		require.Equal(t, idWithAddr, id2)

		exists, err := ks.IdentityExists(id2.Address())
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("creates private identities", func(t *testing.T) {
		id3, err := ks.NewIdentity(false)
		require.NoError(t, err)
		require.False(t, id3.Public)

		ids, err := ks.Identities()
		require.NoError(t, err)
		require.Len(t, ids, 3)
		require.Equal(t, []identity.Identity{id1, id2, id3}, ids)

		pubIds, err := ks.PublicIdentities()
		require.NoError(t, err)
		require.Len(t, pubIds, 2)
		require.Equal(t, []identity.Identity{id1, id2}, pubIds)

		defaultPubId, err := ks.DefaultPublicIdentity()
		require.NoError(t, err)
		require.Equal(t, id1, defaultPubId)

		idWithAddr, err := ks.IdentityWithAddress(id3.Address())
		require.NoError(t, err)
		require.Equal(t, idWithAddr, id3)

		exists, err := ks.IdentityExists(id3.Address())
		require.NoError(t, err)
		require.True(t, exists)
	})
}

func TestBadgerKeyStore_SignHash(t *testing.T) {
	ks := identity.NewBadgerKeyStore(t.TempDir(), identity.InsecureScryptParams)
	err := ks.Unlock("password", "")
	require.NoError(t, err)
	defer ks.Close()

	ids, err := ks.Identities()
	require.NoError(t, err)

	id1 := ids[0]

	t.Run("signs with the correct identity", func(t *testing.T) {
		// Add some more identities
		id2, err := ks.NewIdentity(true)
		require.NoError(t, err)
		id3, err := ks.NewIdentity(false)
		require.NoError(t, err)

		hash := types.HashBytes([]byte("it should sign with the correct identity"))

		sig, err := ks.SignHash(id1.Address(), hash)
		require.NoError(t, err)

		valid := id1.SigKeypair.VerifySignature(hash, sig)
		require.True(t, valid)

		valid = id2.SigKeypair.VerifySignature(hash, sig)
		require.False(t, valid)

		valid = id3.SigKeypair.VerifySignature(hash, sig)
		require.False(t, valid)
	})
}

func TestBadgerKeyStore_MarshalsGethCryptoJSONToDB(t *testing.T) {
	db := testutils.SetupDBTree(t)
	defer db.DeleteDB()

	node := db.State(true)
	defer node.Close()

	bs := []byte(`{"cipher":"aes-128-ctr","ciphertext":"1d0839166e7a15b9c1333fc865d69858b22df26815ccf601b28219b6192974e1","cipherparams":{"iv":"8df6caa7ff1b00c4e871f002cb7921ed"},"kdf":"scrypt","kdfparams":{"dklen":32,"n":8,"p":16,"r":8,"salt":"e5e6ef3f4ea695f496b643ebd3f75c0aa58ef4070e90c80c5d3fb0241bf1595c"},"mac":"6d16dfde774845e4585357f24bce530528bc69f4f84e1e22880d34fa45c273e5"}`)

	var cryptoJSON keystore.CryptoJSON
	err := json.Unmarshal(bs, &cryptoJSON)
	require.NoError(t, err)

	err = node.Set(state.Keypath("blah"), nil, cryptoJSON)
	require.NoError(t, err)

	err = node.Save()
	require.NoError(t, err)

	node = db.State(false)
	defer node.Close()

	var cryptoJSON2 keystore.CryptoJSON
	err = node.NodeAt(state.Keypath("blah"), nil).Scan(&cryptoJSON2)
	require.NoError(t, err)
	require.Equal(t, cryptoJSON, cryptoJSON2)
}

func TestBadgerKeyStore_RatchetSessionStore(t *testing.T) {
	ks := identity.NewBadgerKeyStore(t.TempDir(), identity.InsecureScryptParams)
	err := ks.Unlock("password", "")
	require.NoError(t, err)
	defer ks.Close()

	var (
		sk = [32]byte{
			0xeb, 0x8, 0x10, 0x7c, 0x33, 0x54, 0x0, 0x20,
			0xe9, 0x4f, 0x6c, 0x84, 0xe4, 0x39, 0x50, 0x5a,
			0x2f, 0x60, 0xbe, 0x81, 0xa, 0x78, 0x8b, 0xeb,
			0x1e, 0x2c, 0x9, 0x8d, 0x4b, 0x4d, 0xc1, 0x40,
		}
		aliceSessionID = []byte("alice-session-id")
		bobSessionID   = []byte("bob-session-id")

		aliceMessage1 = []byte("hi bob")
		bobMessage1   = []byte("hi alice")
		aliceMessage2 = []byte("cool")
		bobMessage2   = []byte("neat")
	)

	err = ks.RatchetSessionStore().SaveSharedKey(aliceSessionID, sk[:])
	require.NoError(t, err)
	err = ks.RatchetSessionStore().SaveSharedKey(bobSessionID, sk[:])
	require.NoError(t, err)

	keyPair, err := doubleratchet.DefaultCrypto{}.GenerateDH()
	require.NoError(t, err)

	alice, err := doubleratchet.NewWithRemoteKey(aliceSessionID, sk[:], keyPair.PublicKey(), ks.RatchetSessionStore(), doubleratchet.WithKeysStorage(ks.RatchetKeyStore()))
	require.NoError(t, err)

	bob, err := doubleratchet.New(bobSessionID, sk[:], keyPair, ks.RatchetSessionStore(), doubleratchet.WithKeysStorage(ks.RatchetKeyStore()))
	require.NoError(t, err)

	msg, err := alice.RatchetEncrypt(aliceMessage1, nil)
	require.NoError(t, err)

	m, err := bob.RatchetDecrypt(msg, nil)
	require.NoError(t, err)
	require.Equal(t, aliceMessage1, m)

	msg, err = bob.RatchetEncrypt(bobMessage1, nil)
	require.NoError(t, err)

	m, err = alice.RatchetDecrypt(msg, nil)
	require.NoError(t, err)
	require.Equal(t, bobMessage1, m)

	alice2, err := doubleratchet.Load(aliceSessionID, ks.RatchetSessionStore())
	require.NoError(t, err)

	bob2, err := doubleratchet.Load(bobSessionID, ks.RatchetSessionStore())
	require.NoError(t, err)

	msg, err = alice2.RatchetEncrypt(aliceMessage2, nil)
	require.NoError(t, err)

	m, err = bob2.RatchetDecrypt(msg, nil)
	require.NoError(t, err)
	require.Equal(t, aliceMessage2, m)

	msg, err = bob2.RatchetEncrypt(bobMessage2, nil)
	require.NoError(t, err)

	m, err = alice2.RatchetDecrypt(msg, nil)
	require.NoError(t, err)
	require.Equal(t, bobMessage2, m)
}

func TestBadgerKeyStore_RatchetKeyStore(t *testing.T) {
	ks := identity.NewBadgerKeyStore(t.TempDir(), identity.InsecureScryptParams)
	err := ks.Unlock("password", "")
	require.NoError(t, err)
	defer ks.Close()

	rks := ks.RatchetKeyStore()

	var (
		sessionID = []byte("foo bar")
		pubKey    = doubleratchet.Key("asdfasdfasdffdsafdsafdsa")
		msgNum    = uint(12345)
		mk        = doubleratchet.Key("xyzzy zork xyzzy zork xyzzy zork xyzzy zork")
		keySeqNum = uint(4665)
	)

	err = rks.Put(sessionID, pubKey, msgNum, mk, keySeqNum)
	require.NoError(t, err)

	mk2, ok, err := rks.Get(pubKey, msgNum)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, mk, mk2)
}
