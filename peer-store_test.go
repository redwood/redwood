package redwood_test

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"redwood.dev"
	"redwood.dev/testutils"
	"redwood.dev/types"
	"redwood.dev/utils"
)

func TestPeerStore_DB(t *testing.T) {
	db := testutils.SetupDBTree(t)
	defer db.DeleteDB()

	p := redwood.NewPeerStore(db)

	pds, err := p.FetchAllPeerDetails()
	require.NoError(t, err)
	require.Len(t, pds, 0)

	pd1 := redwood.NewPeerDetails(
		p,
		redwood.PeerDialInfo{TransportName: "http", DialAddr: "http://asdf.dev:1234"},
		testutils.RandomAddress(t),
		testutils.RandomSigningPublicKey(t),
		testutils.RandomEncryptingPublicKey(t),
		utils.NewStringSet([]string{"asdf.dev/registry", "asdf.dev/users", "blah.org/foobar"}),
		testutils.RandomTime(t),
		testutils.RandomTime(t),
		123,
	)
	err = p.SavePeerDetails(pd1)
	require.NoError(t, err)

	pds, err = p.FetchAllPeerDetails()
	require.NoError(t, err)
	require.Len(t, pds, 1)

	requirePeerDetailsEqual(t, pd1, pds[0])

	pd2 := redwood.NewPeerDetails(
		p,
		redwood.PeerDialInfo{TransportName: "libp2p", DialAddr: "/ip4/123.456.789.12/tcp/21231/p2p/16Uiu2HAmBn4mSAKEErkYbCWhmgYLwYckRTb4RwDDRrQEFg6ewJAK"},
		testutils.RandomAddress(t),
		testutils.RandomSigningPublicKey(t),
		testutils.RandomEncryptingPublicKey(t),
		utils.NewStringSet([]string{"foo.bar/blah", "hello.xyz/asdfasdf"}),
		testutils.RandomTime(t),
		testutils.RandomTime(t),
		456,
	)
	err = p.SavePeerDetails(pd2)
	require.NoError(t, err)

	pd3 := redwood.NewPeerDetails(
		p,
		redwood.PeerDialInfo{TransportName: "libp2p", DialAddr: "/dns4/redwood.dev/tcp/21231/p2p/16Uiu2HAmBn4mSAKEErkYbCWhmgYLwYckRTb4RwEcjaEeiFJKAdjDOF"},
		testutils.RandomAddress(t),
		testutils.RandomSigningPublicKey(t),
		testutils.RandomEncryptingPublicKey(t),
		utils.NewStringSet([]string{"foo.bar/blah", "hello.xyz/asdfasdf", "foobar.xyz/xyzzy"}),
		testutils.RandomTime(t),
		testutils.RandomTime(t),
		789,
	)
	err = p.SavePeerDetails(pd3)
	require.NoError(t, err)

	pds, err = p.FetchAllPeerDetails()
	require.NoError(t, err)
	require.Len(t, pds, 3)

	expected := []redwood.PeerDetails{pd1, pd2, pd3}
	sort.Slice(expected, func(i, j int) bool {
		hash_i := types.HashBytes([]byte(expected[i].DialInfo().TransportName + ":" + expected[i].DialInfo().DialAddr)).Hex()
		hash_j := types.HashBytes([]byte(expected[j].DialInfo().TransportName + ":" + expected[j].DialInfo().DialAddr)).Hex()
		return hash_i < hash_j
	})

	for i := range pds {
		requirePeerDetailsEqual(t, expected[i], pds[i])
	}
}

func requirePeerDetailsEqual(t *testing.T, pd1, pd2 redwood.PeerDetails) {
	t.Helper()

	require.Equal(t, pd1.DialInfo(), pd2.DialInfo())
	require.Equal(t, pd1.Address(), pd2.Address())
	sigpubkey1, encpubkey1 := pd1.PublicKeys()
	sigpubkey2, encpubkey2 := pd2.PublicKeys()
	require.Equal(t, sigpubkey1, sigpubkey2)
	require.Equal(t, encpubkey1, encpubkey2)
	require.True(t, pd1.StateURIs().Equal(pd2.StateURIs()))
	require.True(t, pd1.LastContact().Equal(pd2.LastContact()))
	require.True(t, pd1.LastFailure().Equal(pd2.LastFailure()))
	require.Equal(t, pd1.Failures(), pd2.Failures())
}
