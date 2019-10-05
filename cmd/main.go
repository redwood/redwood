package main

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"

	rw "github.com/brynbellomy/redwood"
)

type M = map[string]interface{}

func main() {
	ctx := context.Background()

	var id1 rw.ID
	var id2 rw.ID

	copy(id1[:], []byte("oneoneoneoneone"))
	copy(id2[:], []byte("twotwotwotwotwo"))

	genesisState := M{
		"permissions": M{
			id1.String(): M{
				"^.*$": M{
					"read":  true,
					"write": true,
				},
			},
		},
	}
	genesisState2 := M{
		"permissions": M{
			id1.String(): M{
				"^.*$": M{
					"read":  true,
					"write": true,
				},
			},
		},
	}

	c1 := rw.NewConsumer(id1, 21231, rw.NewStore(id1, genesisState))
	c2 := rw.NewConsumer(id2, 21241, rw.NewStore(id2, genesisState2))

	luaResolver, err := rw.NewLuaResolver(`
function resolve_state(state, patch)
    state['heyo'] = 'zzzzz'
end
            `)
	if err != nil {
		panic(err)
	}

	c1.Store.RegisterResolverForKeypath([]string{"nested", "repo"}, rw.NewStackResolver([]rw.Resolver{
		rw.NewDumbResolver(),
		luaResolver,
		// &gitResolver{repoRoot: "/tmp/hihihi", branch: "master"},
	}))

	// Connect the two consumers
	peerID := c1.Transport.(interface {
		Libp2pPeerID() string
	}).Libp2pPeerID()
	c2.AddPeer(ctx, "/ip4/0.0.0.0/tcp/21231/p2p/"+peerID)

	// Consumer 2 subscribes to a URL
	err = c2.Subscribe(ctx, "braid://axon.science")
	if err != nil {
		panic(err)
	}

	time.Sleep(1 * time.Second)


	var (
		tx1 = rw.Tx{
			ID:      rw.RandomID(),
			Parents: []rw.ID{rw.GenesisTxID},
			From:    id1,
			URL:     "braid://axon.science",
			Patches: []rw.Patch{
				{Keys: []string{"foo"}, Val: 123},
			},
		}

		tx2 = rw.Tx{
			ID:      rw.RandomID(),
			Parents: []rw.ID{tx1.ID},
			From:    id1,
			URL:     "braid://axon.science",
			Patches: []rw.Patch{
				{Keys: []string{"foo", "bar", "baz"}, Val: 123},
			},
		}

		tx3 = rw.Tx{
			ID:      rw.RandomID(),
			Parents: []rw.ID{tx2.ID},
			From:    id1,
			URL:     "braid://axon.science",
			Patches: []rw.Patch{
				{Keys: []string{"nested", "repo", "somefile.txt"}, Val: 123},
				{Keys: []string{"nested", "repo", "a-folder", "filez.txt"}, Val: []int{1, 2, 3}},
			},
		}
	)

	err = c1.AddTx(ctx, tx1)
	if err != nil {
		panic(err)
	}
	err = c1.AddTx(ctx, tx2)
	if err != nil {
		panic(err)
	}
	err = c1.AddTx(ctx, tx3)
	if err != nil {
		panic(err)
	}

	// Block forever
	ch := make(chan struct{})
	<-ch
}
