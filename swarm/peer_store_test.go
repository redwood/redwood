package swarm_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"redwood.dev/internal/testutils"
	"redwood.dev/swarm"
)

func TestPeerStore_PeerDeviceAggregation(t *testing.T) {
	db := testutils.SetupDBTree(t)
	defer db.DeleteDB()

	p := swarm.NewPeerStore(db)
	var i int
	p.OnNewUnverifiedPeer(func(dialInfo swarm.PeerDialInfo) {
		i++
	})

	p.AddDialInfo(swarm.PeerDialInfo{"baz", "quux"}, "")
	require.Equal(t, 0, i)

	p.AddDialInfo(swarm.PeerDialInfo{"libp2p", "/ip4/10.0.0.7/tcp/21232/p2p/12D3KooWGQ6s2fgi5KBj4qNNjPb8ZLbD7xztNrtkHcbTWNNEbT2z"}, "12D3KooWGQ6s2fgi5KBj4qNNjPb8ZLbD7xztNrtkHcbTWNNEbT2z")
	require.Equal(t, 1, i)

	p.AddVerifiedCredentials(swarm.PeerDialInfo{"libp2p", "/ip4/10.0.0.7/tcp/21232/p2p/12D3KooWGQ6s2fgi5KBj4qNNjPb8ZLbD7xztNrtkHcbTWNNEbT2z"}, "12D3KooWGQ6s2fgi5KBj4qNNjPb8ZLbD7xztNrtkHcbTWNNEbT2z", testutils.RandomAddress(t), nil, nil)
	p.AddDialInfo(swarm.PeerDialInfo{"libp2p", "/ip4/10.0.0.7/tcp/21232/p2p/12D3KooWGQ6s2fgi5KBj4qNNjPb8ZLbD7xztNrtkHcbTWNNEbT2z"}, "12D3KooWGQ6s2fgi5KBj4qNNjPb8ZLbD7xztNrtkHcbTWNNEbT2z")
	p.AddDialInfo(swarm.PeerDialInfo{"libp2p", "/ip4/10.0.0.7/tcp/21232/p2p/12D3KooWGQ6s2fgi5KBj4qNNjPb8ZLbD7xztNrtkHcbTWNNEbT2z"}, "")
	p.AddDialInfo(swarm.PeerDialInfo{"libp2p", "/ip4/10.0.0.7/tcp/21232/p2p/12D3KooWGQ6s2fgi5KBj4qNNjPb8ZLbD7xztNrtkHcbTWNNEbT2z"}, "12D3KooWGQ6s2fgi5KBj4qNNjPb8ZLbD7xztNrtkHcbTWNNEbT2z")
	p.AddDialInfo(swarm.PeerDialInfo{"baz", "quux"}, "")
	p.AddDialInfo(swarm.PeerDialInfo{"baz", "quux"}, "12D3KooWGQ6s2fgi5KBj4qNNjPb8ZLbD7xztNrtkHcbTWNNEbT2z")
	p.AddDialInfo(swarm.PeerDialInfo{"libp2p", "/ip4/10.0.0.7/tcp/21232/p2p/12D3KooWGQ6s2fgi5KBj4qNNjPb8ZLbD7xztNrtkHcbTWNNEbT2z"}, "")
	p.AddDialInfo(swarm.PeerDialInfo{"libp2p", "/ip4/10.0.0.7/tcp/21232/p2p/12D3KooWGQ6s2fgi5KBj4qNNjPb8ZLbD7xztNrtkHcbTWNNEbT2z"}, "12D3KooWGQ6s2fgi5KBj4qNNjPb8ZLbD7xztNrtkHcbTWNNEbT2z")
	p.AddDialInfo(swarm.PeerDialInfo{"libp2p", "/ip4/10.0.0.7/tcp/21232/p2p/12D3KooWGQ6s2fgi5KBj4qNNjPb8ZLbD7xztNrtkHcbTWNNEbT2z"}, "")
	p.AddDialInfo(swarm.PeerDialInfo{"libp2p", "/ip4/10.0.0.7/tcp/21232/p2p/12D3KooWGQ6s2fgi5KBj4qNNjPb8ZLbD7xztNrtkHcbTWNNEbT2z"}, "12D3KooWGQ6s2fgi5KBj4qNNjPb8ZLbD7xztNrtkHcbTWNNEbT2z")
	p.AddDialInfo(swarm.PeerDialInfo{"baz", "quux"}, "")
	p.AddDialInfo(swarm.PeerDialInfo{"baz", "quux"}, "12D3KooWGQ6s2fgi5KBj4qNNjPb8ZLbD7xztNrtkHcbTWNNEbT2z")
	p.AddDialInfo(swarm.PeerDialInfo{"libp2p", "/ip4/10.0.0.7/tcp/21232/p2p/12D3KooWGQ6s2fgi5KBj4qNNjPb8ZLbD7xztNrtkHcbTWNNEbT2z"}, "")

	require.Equal(t, 1, i)
}

// func TestPeerStore_DB(t *testing.T) {
// 	db := testutils.SetupDBTree(t)
// 	defer db.DeleteDB()

// 	p := swarm.NewPeerStore(db)

// 	pds, err := p.FetchAllPeerDetails()
// 	require.NoError(t, err)
// 	require.Len(t, pds, 0)

// 	addr1 := testutils.RandomAddress(t)
// 	pd1 := swarm.NewPeerDetails(
// 		p,
// 		swarm.PeerDialInfo{TransportName: "http", DialAddr: "http://asdf.dev:1234"},
// 		"peer1",
// 		types.NewAddressSet([]types.Address{addr1}),
// 		map[types.Address]*crypto.SigningPublicKey{addr1: testutils.RandomSigningPublicKey(t)},
// 		map[types.Address]*crypto.AsymEncPubkey{addr1: testutils.RandomAsymEncPubkey(t)},
// 		types.NewStringSet([]string{"asdf.dev/registry", "asdf.dev/users", "blah.org/foobar"}),
// 		types.Time(testutils.RandomTime(t)),
// 		types.Time(testutils.RandomTime(t)),
// 		123,
// 	)
// 	err = p.SavePeerDetails(pd1)
// 	require.NoError(t, err)

// 	pds, err = p.FetchAllPeerDetails()
// 	require.NoError(t, err)
// 	require.Len(t, pds, 1)

// 	requirePeerDevicesEqual(t, pd1, pds[0])

// 	addr2 := testutils.RandomAddress(t)
// 	pd2 := swarm.NewPeerDetails(
// 		p,
// 		swarm.PeerDialInfo{TransportName: "libp2p", DialAddr: "/ip4/123.456.789.12/tcp/21231/p2p/16Uiu2HAmBn4mSAKEErkYbCWhmgYLwYckRTb4RwDDRrQEFg6ewJAK"},
// 		"peer2",
// 		types.NewAddressSet([]types.Address{addr2}),
// 		map[types.Address]*crypto.SigningPublicKey{addr2: testutils.RandomSigningPublicKey(t)},
// 		map[types.Address]*crypto.AsymEncPubkey{addr2: testutils.RandomAsymEncPubkey(t)},
// 		types.NewStringSet([]string{"foo.bar/blah", "hello.xyz/asdfasdf"}),
// 		types.Time(testutils.RandomTime(t)),
// 		types.Time(testutils.RandomTime(t)),
// 		456,
// 	)
// 	err = p.SavePeerDetails(pd2)
// 	require.NoError(t, err)

// 	addr3 := testutils.RandomAddress(t)
// 	pd3 := swarm.NewPeerDetails(
// 		p,
// 		swarm.PeerDialInfo{TransportName: "libp2p", DialAddr: "/dns4/redwood.dev/tcp/21231/p2p/16Uiu2HAmBn4mSAKEErkYbCWhmgYLwYckRTb4RwEcjaEeiFJKAdjDOF"},
// 		"peer3",
// 		types.NewAddressSet([]types.Address{addr3}),
// 		map[types.Address]*crypto.SigningPublicKey{addr3: testutils.RandomSigningPublicKey(t)},
// 		map[types.Address]*crypto.AsymEncPubkey{addr3: testutils.RandomAsymEncPubkey(t)},
// 		types.NewStringSet([]string{"foo.bar/blah", "hello.xyz/asdfasdf", "foobar.xyz/xyzzy"}),
// 		types.Time(testutils.RandomTime(t)),
// 		types.Time(testutils.RandomTime(t)),
// 		789,
// 	)
// 	err = p.SavePeerDetails(pd3)
// 	require.NoError(t, err)
// 	requireExpectedPeers(t, p, []swarm.PeerInfo{pd1, pd2, pd3})

// 	err = p.RemovePeers([]string{pd2.DeviceUniqueID()})
// 	require.NoError(t, err)
// 	requireExpectedPeers(t, p, []swarm.PeerInfo{pd1, pd3})

// 	err = p.RemovePeers([]string{pd1.DeviceUniqueID(), pd3.DeviceUniqueID()})
// 	require.NoError(t, err)
// 	requireExpectedPeers(t, p, []swarm.PeerInfo{})
// }

// func requireExpectedPeers(t *testing.T, p *swarm.ConcretePeerStore, expected []swarm.PeerInfo) {
// 	t.Helper()

// 	pds, err := p.FetchAllPeerDetails()
// 	require.NoError(t, err)
// 	require.Len(t, pds, len(expected))

// 	expectedMap := make(map[string]swarm.PeerInfo)
// 	for _, x := range expected {
// 		expectedMap[x.DeviceUniqueID()] = x
// 	}
// 	for _, pd := range pds {
// 		requirePeerDevicesEqual(t, expectedMap[pd.DeviceUniqueID()], pd)
// 	}
// }

// func requirePeerDevicesEqual(t *testing.T, pd1, pd2 swarm.PeerInfo) {
// 	t.Helper()

// 	endpoints1 := pd1.Endpoints()
// 	endpoints2 := pd2.Endpoints()

// 	for i := range endpoints1 {
// 		e1, e2 := endpoints1[i], endpoints2[i]
// 		require.Equal(t, e1.DialInfo(), e2.DialInfo())
// 		require.Equal(t, e1.Addresses()[0], e2.Addresses()[0])
// 		sigpubkey1, encpubkey1 := e1.PublicKeys(e1.Addresses()[0])
// 		sigpubkey2, encpubkey2 := e2.PublicKeys(e2.Addresses()[0])
// 		require.Equal(t, sigpubkey1, sigpubkey2)
// 		require.Equal(t, encpubkey1, encpubkey2)
// 		require.True(t, e1.StateURIs().Equal(e2.StateURIs()))
// 		require.True(t, e1.LastContact().Equal(e2.LastContact()))
// 		require.True(t, e1.LastFailure().Equal(e2.LastFailure()))
// 		require.Equal(t, e1.Failures(), e2.Failures())
// 	}
// }
