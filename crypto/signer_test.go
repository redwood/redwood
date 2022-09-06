package crypto_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"redwood.dev/crypto"
	"redwood.dev/internal/testutils"
	"redwood.dev/types"
)

func TestSigKeypairFromHDMnemonic(t *testing.T) {
	mnemonic := "joke basic have athlete nurse tank snow uniform busy rural depend recall dinosaur glory elegant"
	keypair, err := crypto.SigKeypairFromHDMnemonic(mnemonic, 0)
	require.NoError(t, err)
	require.Equal(t, "1c6e8d3d4e32f3c8e0bf1295a397ed5cda700888f8d289d602b15fdfd05a3f82", keypair.SigningPrivateKey.Hex())
}

func TestSigning(t *testing.T) {
	keys, err := crypto.GenerateSigKeypair()
	require.NoError(t, err)

	hash := types.HashBytes(types.RandomBytes(100))
	sig, err := keys.SignHash(hash)
	require.NoError(t, err)

	pubkey, err := crypto.RecoverSigningPubkey(hash, sig)
	require.NoError(t, err)

	require.Equal(t, keys.Address(), pubkey.Address())

	keys.Address()
}

func TestChallengeMsg_MarshalUnmarshal(t *testing.T) {
	challenge, err := crypto.GenerateChallengeMsg()
	require.NoError(t, err)

	bs, err := challenge.Marshal()
	require.NoError(t, err)

	var challenge2 crypto.ChallengeMsg
	err = challenge2.Unmarshal(bs)
	require.NoError(t, err)
	require.Equal(t, challenge, challenge2)
}

func TestSignature_MarshalUnmarshal(t *testing.T) {
	id := testutils.RandomIdentity(t)

	bs := types.RandomBytes(32)
	sig, err := id.SignHash(types.HashBytes(bs))
	require.NoError(t, err)

	sigBytes, err := sig.Marshal()
	require.NoError(t, err)

	var sig2 crypto.Signature
	err = sig2.Unmarshal(sigBytes)
	require.NoError(t, err)
	require.Equal(t, sig, sig2)
}
