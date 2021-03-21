package crypto_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"redwood.dev/crypto"
)

func TestSigningKeypairFromHDMnemonic(t *testing.T) {
	mnemonic := "joke basic have athlete nurse tank snow uniform busy rural depend recall dinosaur glory elegant"
	keypair, err := crypto.SigningKeypairFromHDMnemonic(mnemonic, 0)
	require.NoError(t, err)
	require.Equal(t, "1c6e8d3d4e32f3c8e0bf1295a397ed5cda700888f8d289d602b15fdfd05a3f82", keypair.SigningPrivateKey.Hex())
}

func TestSigningPublicKey_JSON(t *testing.T) {
	keys, err := crypto.GenerateSigningKeypair()
	require.NoError(t, err)

	type X struct {
		Pubkey crypto.SigningPublicKey `json:"pubkey"`
	}

	var foo X
	foo.Pubkey = keys.SigningPublicKey

	bs, err := json.Marshal(foo)
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf(`{"pubkey":"%v"}`, foo.Pubkey.Hex()), string(bs))

	var bar X
	err = json.Unmarshal(bs, &bar)
	require.NoError(t, err)
	require.Equal(t, foo.Pubkey, bar.Pubkey)
}
