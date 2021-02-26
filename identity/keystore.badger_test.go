package identity_test

import (
	"encoding/json"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"

	"github.com/stretchr/testify/require"

	"redwood.dev/testutils"
	"redwood.dev/tree"
)

func TestBadgerKeyStore_MarshalsGethCryptoJSONToDB(t *testing.T) {
	db := testutils.SetupDBTree(t)
	defer db.DeleteDB()

	state := db.State(true)
	defer state.Close()

	bs := []byte(`{"cipher":"aes-128-ctr","ciphertext":"1d0839166e7a15b9c1333fc865d69858b22df26815ccf601b28219b6192974e1","cipherparams":{"iv":"8df6caa7ff1b00c4e871f002cb7921ed"},"kdf":"scrypt","kdfparams":{"dklen":32,"n":8,"p":16,"r":8,"salt":"e5e6ef3f4ea695f496b643ebd3f75c0aa58ef4070e90c80c5d3fb0241bf1595c"},"mac":"6d16dfde774845e4585357f24bce530528bc69f4f84e1e22880d34fa45c273e5"}`)

	var cryptoJSON keystore.CryptoJSON
	err := json.Unmarshal(bs, &cryptoJSON)
	require.NoError(t, err)

	err = state.Set(tree.Keypath("blah"), nil, cryptoJSON)
	require.NoError(t, err)

	err = state.Save()
	require.NoError(t, err)

	state = db.State(false)
	defer state.Close()

	var cryptoJSON2 keystore.CryptoJSON
	err = state.NodeAt(tree.Keypath("blah"), nil).Scan(&cryptoJSON2)
	require.NoError(t, err)
	require.Equal(t, cryptoJSON, cryptoJSON2)
}
