package main

import (
	"context"
	"time"

	rw "github.com/brynbellomy/redwood"
)

type M = map[string]interface{}

func main() {
	ctx := context.Background()

	var id1 rw.ID
	var id2 rw.ID

	copy(id1[:], []byte("oneoneoneoneone"))
	copy(id2[:], []byte("twotwotwotwotwo"))

	genesisBytes, err := ioutil.ReadFile("genesis.json")
	if err != nil {
		panic(err)
	}

	var genesis1 map[string]interface{}
	var genesis2 map[string]interface{}
	err = json.Unmarshal(genesisBytes, &genesis1)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(genesisBytes, &genesis2)
	if err != nil {
		panic(err)
	}

	store1, err := rw.NewStore(id1, genesis1)
	if err != nil {
		panic(err)
	}
	store2, err := rw.NewStore(id2, genesis2)
	if err != nil {
		panic(err)
	}

	c1, err := rw.NewConsumer(id1, 21231, store1)
	if err != nil {
		panic(err)
	}

	c2, err := rw.NewConsumer(id2, 21241, store2)
	if err != nil {
		panic(err)
	}

	talkChannelResolver, err := rw.NewLuaResolverFromFile("talk-channel.lua")
	if err != nil {
		panic(err)
	}

	c1.Store.RegisterResolverForKeypath([]string{"shrugisland", "talk0"}, talkChannelResolver)
	c1.Store.RegisterValidatorForKeypath([]string{"shrugisland", "talk0"}, rw.NewStackValidator([]rw.Validator{
		&rw.IntrinsicsValidator{},
		&rw.PermissionsValidator{},
	}))

	c2.Store.RegisterResolverForKeypath([]string{"shrugisland", "talk0"}, talkChannelResolver)
	c2.Store.RegisterValidatorForKeypath([]string{"shrugisland", "talk0"}, rw.NewStackValidator([]rw.Validator{
		&rw.IntrinsicsValidator{},
		&rw.PermissionsValidator{},
	}))

	// Connect the two consumers
	peerID := c1.Transport.(interface{ Libp2pPeerID() string }).Libp2pPeerID()
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
				mustParsePatch(`.shrugisland.talk0.messages[0:0] = {"text":"hello!"}`),
			},
		}

		tx2 = rw.Tx{
			ID:      rw.RandomID(),
			Parents: []rw.ID{tx1.ID},
			From:    id1,
			URL:     "braid://axon.science",
			Patches: []rw.Patch{
				mustParsePatch(`.shrugisland.talk0.messages[1:1] = {"text":"well hello to you too"}`),
			},
		}

		tx3 = rw.Tx{
			ID:      rw.RandomID(),
			Parents: []rw.ID{tx2.ID},
			From:    id1,
			URL:     "braid://axon.science",
			Patches: []rw.Patch{
				mustParsePatch(`.shrugisland.talk0.messages[2:2] = {"text":"yoooo"}`),
			},
		}
	)

	c1.Info(1, "adding tx 1")
	err = c1.AddTx(ctx, tx1)
	if err != nil {
		c1.Errorf("zzz %+v", err)
	}
	c1.Info(1, "adding tx 2")
	err = c1.AddTx(ctx, tx2)
	if err != nil {
		c1.Errorf("yyy %+v", err)
	}
	c1.Info(1, "adding tx 3")
	err = c1.AddTx(ctx, tx3)
	if err != nil {
		c1.Errorf("xxx %+v", err)
	}

	// Block forever
	ch := make(chan struct{})
	<-ch
}

func mustParsePatch(s string) rw.Patch {
	p, err := rw.ParsePatch(s)
	if err != nil {
		panic(err)
	}
	return p
}

// genesisState := M{
//     "permissions": M{
//         id1.String(): M{
//             "^.*$": M{
//                 "read":  true,
//                 "write": true,
//             },
//         },
//     },
//     "shrugisland": M{
//         "talk0": M{
//             "permissions": M{
//                 id1.String(): M{
//                     "^.*$": M{
//                         "read":  true,
//                         "write": true,
//                     },
//                 },
//                 id2.String(): M{
//                     "^.*$": M{
//                         "read":  true,
//                         "write": true,
//                     },
//                 },
//             },
//         },
//     },
// }
// genesisState2 := M{
//     "permissions": M{
//         id1.String(): M{
//             "^.*$": M{
//                 "read":  true,
//                 "write": true,
//             },
//         },
//     },
//     "shrugisland": M{
//         "talk0": M{
//             "permissions": M{
//                 id1.String(): M{
//                     "^.*$": M{
//                         "read":  true,
//                         "write": true,
//                     },
//                 },
//                 id2.String(): M{
//                     "^.*$": M{
//                         "read":  true,
//                         "write": true,
//                     },
//                 },
//             },
//         },
//     },
// }
