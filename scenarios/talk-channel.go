package main

import (
	"encoding/json"
	"io/ioutil"
	"time"

	"github.com/plan-systems/plan-core/tools/ctx"

	rw "github.com/brynbellomy/redwood"
)

type M = map[string]interface{}

type testNode struct {
	ctx.Context
}

func (n *testNode) onStartup() error {
	return nil
}

func main() {

	n := testNode{}
	n.CtxStart(
		n.onStartup,
		nil,
		nil,
		nil,
	)

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

	n.CtxAddChild(c1, nil)
	n.CtxAddChild(c2, nil)

	// Setup talk channel using transactions
	{
		var (
			tx1 = rw.Tx{
				ID:      rw.RandomID(),
				Parents: []rw.ID{rw.GenesisTxID},
				From:    id1,
				URL:     "braid://axon.science",
				Patches: []rw.Patch{
					mustParsePatch(`.shrugisland.talk0.validator = {
                        "type":"stack",
                        "children":[
                            {"type": "intrinsics"},
                            {"type": "permissions"}
                        ]
                    }`),
					mustParsePatch(`.shrugisland.talk0.resolver = {
                        "type":"lua",
                        "src":"
                            function resolve_state(state, patch)
                                if state == nil then
                                    state = {}
                                end

                                if patch:RangeStart() ~= -1 and patch:RangeStart() == patch:RangeEnd() then
                                    if state['messages'] == nil then
                                        state['messages'] = { patch.Val }

                                    elseif patch:RangeStart() <= #state['messages'] then
                                        -- state:Get('messages'):Insert(patch.Val)
                                        state['messages'][ #state['messages'] + 1 ] = patch.Val

                                    end
                                else
                                    error('invalid')
                                end
                                return state
                            end
                        "
                    }`),
				},
			}
		)

		c1.Info(1, "sending tx to initialize talk channel...")
		err = c1.AddTx(tx1)
		if err != nil {
			c1.Errorf("%+v", err)
		}
	}

	// talkChannelResolver, err := rw.NewLuaResolverFromFile("talk-channel.lua")
	// if err != nil {
	// 	panic(err)
	// }

	// c1.Store.RegisterResolverForKeypath([]string{"shrugisland", "talk0"}, talkChannelResolver)
	// c1.Store.RegisterValidatorForKeypath([]string{"shrugisland", "talk0"}, rw.NewStackValidator([]rw.Validator{
	// 	&rw.IntrinsicsValidator{},
	// 	&rw.PermissionsValidator{},
	// }))

	// c2.Store.RegisterResolverForKeypath([]string{"shrugisland", "talk0"}, talkChannelResolver)
	// c2.Store.RegisterValidatorForKeypath([]string{"shrugisland", "talk0"}, rw.NewStackValidator([]rw.Validator{
	// 	&rw.IntrinsicsValidator{},
	// 	&rw.PermissionsValidator{},
	// }))

	// Connect the two consumers
	peerID := c1.Transport.(interface{ Libp2pPeerID() string }).Libp2pPeerID()
	c2.AddPeer(c2.Ctx, "/ip4/0.0.0.0/tcp/21231/p2p/"+peerID)

	// Consumer 2 subscribes to a URL
	err = c2.Subscribe(c2.Ctx, "braid://axon.science")
	if err != nil {
		panic(err)
	}

	time.Sleep(1 * time.Second)

	var (
		tx2 = rw.Tx{
			ID:      rw.RandomID(),
			Parents: []rw.ID{rw.GenesisTxID},
			From:    id1,
			URL:     "braid://axon.science",
			Patches: []rw.Patch{
				mustParsePatch(`.shrugisland.talk0.messages[0:0] = {"text":"hello!"}`),
			},
		}

		tx3 = rw.Tx{
			ID:      rw.RandomID(),
			Parents: []rw.ID{tx2.ID},
			From:    id1,
			URL:     "braid://axon.science",
			Patches: []rw.Patch{
				mustParsePatch(`.shrugisland.talk0.messages[1:1] = {"text":"well hello to you too"}`),
			},
		}

		tx4 = rw.Tx{
			ID:      rw.RandomID(),
			Parents: []rw.ID{tx3.ID},
			From:    id1,
			URL:     "braid://axon.science",
			Patches: []rw.Patch{
				mustParsePatch(`.shrugisland.talk0.messages[2:2] = {"text":"yoooo"}`),
			},
		}
	)

	c1.Info(1, "sending tx 1...")
	err = c1.AddTx(tx2)
	if err != nil {
		c1.Errorf("zzz %+v", err)
	}
	c1.Info(1, "sending tx 2...")
	err = c1.AddTx(tx3)
	if err != nil {
		c1.Errorf("yyy %+v", err)
	}
	c1.Info(1, "sending tx 3...")
	err = c1.AddTx(tx4)
	if err != nil {
		c1.Errorf("xxx %+v", err)
	}

	n.AttachInterruptHandler()
	n.CtxWait()

}

func mustParsePatch(s string) rw.Patch {
	p, err := rw.ParsePatch(s)
	if err != nil {
		panic(err.Error() + ": " + s)
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
