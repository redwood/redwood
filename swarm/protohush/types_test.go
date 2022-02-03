package protohush_test

import (
	"encoding/hex"
	"fmt"
	"testing"

	"redwood.dev/internal/testutils"
	"redwood.dev/swarm/protohush"
	"redwood.dev/types"
)

func TestEncryptedIndividualSessionProposal_Hash(t *testing.T) {
	addr := types.RandomAddress()
	bs := testutils.RandomBytes(t, 32)
	p := protohush.EncryptedIndividualSessionProposal{
		AliceAddr:         addr,
		EncryptedProposal: bs,
	}

	fmt.Println(addr)
	fmt.Println(hex.EncodeToString(bs))

	fmt.Println(p.Hash(), addr, hex.EncodeToString(bs))
	fmt.Println(p.Hash(), addr, hex.EncodeToString(bs))
	fmt.Println(p.Hash(), addr, hex.EncodeToString(bs))
}
