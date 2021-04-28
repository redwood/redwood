package swarm_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"redwood.dev/crypto"
	"redwood.dev/internal/testutils"
	"redwood.dev/swarm"
	"redwood.dev/types"
	"redwood.dev/utils"
)

func TestPeerStore_DB(t *testing.T) {
	db := testutils.SetupDBTree(t)
	defer db.DeleteDB()

	p := swarm.NewPeerStore(db)

	pds, err := p.FetchAllPeerDetails()
	require.NoError(t, err)
	require.Len(t, pds, 0)

	addr1 := testutils.RandomAddress(t)
	pd1 := swarm.NewPeerDetails(
		p,
		swarm.PeerDialInfo{TransportName: "http", DialAddr: "http://asdf.dev:1234"},
		utils.NewAddressSet([]types.Address{addr1}),
		map[types.Address]crypto.SigningPublicKey{addr1: testutils.RandomSigningPublicKey(t)},
		map[types.Address]crypto.EncryptingPublicKey{addr1: testutils.RandomEncryptingPublicKey(t)},
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

	addr2 := testutils.RandomAddress(t)
	pd2 := swarm.NewPeerDetails(
		p,
		swarm.PeerDialInfo{TransportName: "libp2p", DialAddr: "/ip4/123.456.789.12/tcp/21231/p2p/16Uiu2HAmBn4mSAKEErkYbCWhmgYLwYckRTb4RwDDRrQEFg6ewJAK"},
		utils.NewAddressSet([]types.Address{addr2}),
		map[types.Address]crypto.SigningPublicKey{addr2: testutils.RandomSigningPublicKey(t)},
		map[types.Address]crypto.EncryptingPublicKey{addr2: testutils.RandomEncryptingPublicKey(t)},
		utils.NewStringSet([]string{"foo.bar/blah", "hello.xyz/asdfasdf"}),
		testutils.RandomTime(t),
		testutils.RandomTime(t),
		456,
	)
	err = p.SavePeerDetails(pd2)
	require.NoError(t, err)

	addr3 := testutils.RandomAddress(t)
	pd3 := swarm.NewPeerDetails(
		p,
		swarm.PeerDialInfo{TransportName: "libp2p", DialAddr: "/dns4/redwood.dev/tcp/21231/p2p/16Uiu2HAmBn4mSAKEErkYbCWhmgYLwYckRTb4RwEcjaEeiFJKAdjDOF"},
		utils.NewAddressSet([]types.Address{addr3}),
		map[types.Address]crypto.SigningPublicKey{addr3: testutils.RandomSigningPublicKey(t)},
		map[types.Address]crypto.EncryptingPublicKey{addr3: testutils.RandomEncryptingPublicKey(t)},
		utils.NewStringSet([]string{"foo.bar/blah", "hello.xyz/asdfasdf", "foobar.xyz/xyzzy"}),
		testutils.RandomTime(t),
		testutils.RandomTime(t),
		789,
	)
	err = p.SavePeerDetails(pd3)
	require.NoError(t, err)
	requireExpectedPeers(t, p, []swarm.PeerDetails{pd1, pd2, pd3})

	err = p.RemovePeers([]swarm.PeerDialInfo{pd2.DialInfo()})
	require.NoError(t, err)
	requireExpectedPeers(t, p, []swarm.PeerDetails{pd1, pd3})

	err = p.RemovePeers([]swarm.PeerDialInfo{pd1.DialInfo(), pd3.DialInfo()})
	require.NoError(t, err)
	requireExpectedPeers(t, p, []swarm.PeerDetails{})
}

func requireExpectedPeers(t *testing.T, p *swarm.ConcretePeerStore, expected []swarm.PeerDetails) {
	t.Helper()

	pds, err := p.FetchAllPeerDetails()
	require.NoError(t, err)
	require.Len(t, pds, len(expected))

	expectedMap := make(map[swarm.PeerDialInfo]swarm.PeerDetails)
	for _, x := range expected {
		expectedMap[x.DialInfo()] = x
	}
	for _, pd := range pds {
		requirePeerDetailsEqual(t, expectedMap[pd.DialInfo()], pd)
	}
}

func requirePeerDetailsEqual(t *testing.T, pd1, pd2 swarm.PeerDetails) {
	t.Helper()

	require.Equal(t, pd1.DialInfo(), pd2.DialInfo())
	require.Equal(t, pd1.Addresses()[0], pd2.Addresses()[0])
	sigpubkey1, encpubkey1 := pd1.PublicKeys(pd1.Addresses()[0])
	sigpubkey2, encpubkey2 := pd2.PublicKeys(pd2.Addresses()[0])
	require.Equal(t, sigpubkey1, sigpubkey2)
	require.Equal(t, encpubkey1, encpubkey2)
	require.True(t, pd1.StateURIs().Equal(pd2.StateURIs()))
	require.True(t, pd1.LastContact().Equal(pd2.LastContact()))
	require.True(t, pd1.LastFailure().Equal(pd2.LastFailure()))
	require.Equal(t, pd1.Failures(), pd2.Failures())
}
