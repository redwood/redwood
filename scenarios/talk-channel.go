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
		"shrugisland": M{
			"talk0": M{
				// "messages": []interface{}{},
				"permissions": M{
					id1.String(): M{
						"^.*$": M{
							"read":  true,
							"write": true,
						},
					},
					id2.String(): M{
						"^.*$": M{
							"read":  true,
							"write": true,
						},
					},
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
		"shrugisland": M{
			"talk0": M{
				// "messages": []interface{}{},
				"permissions": M{
					id1.String(): M{
						"^.*$": M{
							"read":  true,
							"write": true,
						},
					},
					id2.String(): M{
						"^.*$": M{
							"read":  true,
							"write": true,
						},
					},
				},
			},
		},
	}

	c1 := rw.NewConsumer(id1, 21231, rw.NewStore(id1, genesisState))
	c2 := rw.NewConsumer(id2, 21241, rw.NewStore(id2, genesisState2))

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
	log.Infof("adding txs...")

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

	log.Infoln("sending tx 1")
	err = c1.AddTx(ctx, tx1)
	if err != nil {
		log.Errorf("zzz %+v", err)
	}
	log.Infoln("sending tx 2")
	err = c1.AddTx(ctx, tx2)
	if err != nil {
		log.Errorf("yyy %+v", err)
	}
	log.Infoln("sending tx 3")
	err = c1.AddTx(ctx, tx3)
	if err != nil {
		log.Errorf("xxx %+v", err)
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
