package main

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
)

type M = map[string]interface{}

func main() {
	ctx := context.Background()

	var id1 ID
	var id2 ID

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

	c1 := NewConsumer(id1, 9123, NewStore(id1, genesisState))
	c2 := NewConsumer(id2, 9124, NewStore(id2, genesisState2))

	luaResolver, err := NewLuaResolver(`
function resolve_state(state, patch)
    state['heyo'] = 'zzzzz'
end
            `)
	if err != nil {
		panic(err)
	}

	c1.Store.RegisterResolverForKeypath([]string{"nested", "repo"}, NewStackResolver([]Resolver{
		NewDumbResolver(),
		luaResolver,
		&gitResolver{repoRoot: "/tmp/hihihi", branch: "master"},
	}))

	c2.AddPeer(ctx, "/ip4/0.0.0.0/tcp/9123/p2p/16Uiu2HAkyRshgAY4q6SwfW88zM845mkauUQneWL3vX7Yacd17SNx")

	err = c2.Subscribe(ctx, "braid://axon.science")
	if err != nil {
		panic(err)
	}

	time.Sleep(1 * time.Second)
	log.Infof("adding txs...")

	var (
		tx1 = Tx{
			ID:      RandomID(),
			Parents: []ID{GenesisTxID},
			From:    id1,
			URL:     "braid://axon.science",
			Patches: []Patch{
				{Keys: []string{"foo"}, Val: 123},
			},
		}

		tx2 = Tx{
			ID:      RandomID(),
			Parents: []ID{tx1.ID},
			From:    id1,
			URL:     "braid://axon.science",
			Patches: []Patch{
				{Keys: []string{"foo", "bar", "baz"}, Val: 123},
			},
		}

		tx3 = Tx{
			ID:      RandomID(),
			Parents: []ID{tx2.ID},
			From:    id1,
			URL:     "braid://axon.science",
			Patches: []Patch{
				{Keys: []string{"nested", "repo", "somefile.txt"}, Val: 123},
				{Keys: []string{"nested", "repo", "a-folder", "filez.txt"}, Val: []int{1, 2, 3}},
			},
		}
	)

	c1.AddTx(ctx, tx1)
	c1.AddTx(ctx, tx2)
	c1.AddTx(ctx, tx3)

	// Block forever
	ch := make(chan struct{})
	<-ch
}
