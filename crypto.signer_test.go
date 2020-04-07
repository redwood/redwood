package redwood

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSigningKeypairFromHDMnemonic(t *testing.T) {
	mnemonic := "joke basic have athlete nurse tank snow uniform busy rural depend recall dinosaur glory elegant"
	keypair, err := SigningKeypairFromHDMnemonic(mnemonic, DefaultHDDerivationPath)
	require.NoError(t, err)

	require.Equal(t, "1c6e8d3d4e32f3c8e0bf1295a397ed5cda700888f8d289d602b15fdfd05a3f82", keypair.SigningPrivateKey.String())
}
