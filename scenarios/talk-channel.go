package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
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

	flag.Parse()
	flag.Set("logtostderr", "true")
	flag.Set("v", "2")

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

	signingKeypair1, err := rw.SigningKeypairFromHex("fad9c8855b740a0b7ed4c221dbad0f33a83a49cad6b3fe8d5817ac83d38b6a19")
	if err != nil {
		panic(err)
	}
	signingKeypair2, err := rw.SigningKeypairFromHex("deadbeef5b740a0b7ed4c22149cadbaddeadbeefd6b3fe8d5817ac83deadbeef")
	if err != nil {
		panic(err)
	}

	encryptingKeypair1, err := rw.GenerateEncryptingKeypair()
	if err != nil {
		panic(err)
	}
	encryptingKeypair2, err := rw.GenerateEncryptingKeypair()
	if err != nil {
		panic(err)
	}

	fmt.Println("account 1:", signingKeypair1.Address())
	fmt.Println("account 2:", signingKeypair2.Address())

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

	store1, err := rw.NewStore(signingKeypair1.Address(), genesis1)
	if err != nil {
		panic(err)
	}
	store2, err := rw.NewStore(signingKeypair2.Address(), genesis2)
	if err != nil {
		panic(err)
	}

	c1, err := rw.NewHost(signingKeypair1, encryptingKeypair1, 21231, store1)
	if err != nil {
		panic(err)
	}

	c2, err := rw.NewHost(signingKeypair2, encryptingKeypair2, 21241, store2)
	if err != nil {
		panic(err)
	}

	n.CtxAddChild(c1, nil)
	n.CtxAddChild(c2, nil)

	// Connect the two consumers
	// peerID := c1.Transport.(interface{ Libp2pPeerID() string }).Libp2pPeerID()
	// c2.AddPeer(c2.Ctx, "/ip4/0.0.0.0/tcp/21231/p2p/"+peerID)

	// Consumer 2 subscribes to a URL
	err = c2.Subscribe(c2.Ctx, "axon.science:21231")
	if err != nil {
		panic(err)
	}

	time.Sleep(1 * time.Second)

	// Setup talk channel using transactions
	var tx1 = rw.Tx{
		ID:      rw.RandomID(),
		Parents: []rw.ID{rw.GenesisTxID},
		From:    c1.Address(),
		URL:     "axon.science:21231",
		Patches: []rw.Patch{
			mustParsePatch(`.shrugisland.talk0.permissions = {
                "96216849c49358b10257cb55b28ea603c874b05e": {
                    "^.*$": {
                        "read": true,
                        "write": true
                    }
                },
                "*": {
                    "^.*$": {
                        "read": true,
                        "write": true
                    }
                }
            }`),
			mustParsePatch(`.shrugisland.talk0.messages = []`),
			mustParsePatch(`.shrugisland.talk0.validator = {"type": "permissions"}`),
			mustParsePatch(`.shrugisland.talk0.resolver = {
                    "type":"lua",
                    "src":"
                        function resolve_state(state, sender, patch)
                            if state == nil then
                                state = {}
                            end

                            if patch:RangeStart() ~= -1 and patch:RangeStart() == patch:RangeEnd() then
                                local msg = {
                                    text = patch.Val['text'],
                                    sender = sender,
                                }
                                if state['messages'] == nil then
                                    state['messages'] = { msg }

                                elseif patch:RangeStart() <= #state['messages'] then
                                    state['messages'][ #state['messages'] + 1 ] = msg

                                end
                            else
                                error('invalid')
                            end
                            return state
                        end
                    "
                }`),
			mustParsePatch(`.shrugisland.talk0.index = "
                    <html>
                    <head>
                        <style>
                            * {
                                font-family: 'Consolas', 'Ubuntu Mono', 'Monaco', 'Courier New', Courier, sans-serif;
                            }
                        </style>
                    </head>
                    <body>
                        <div id='my-address'></div>
                        <br/>
                        <div>
                            Log into a new address with a seed phrase:<br/>
                            <input id='input-mnemonic'/>&nbsp;
                            <button id='btn-login'>Go</button>
                        </div>
                        <br/>
                        <br/>

                        <div id='container'></div>
                        <div>
                            <input id='input-text' />
                            <button id='btn-send'>Send</button>
                        </div>
                    </body>

                    <script src='/braid.js'></script>
                    <script>

                        //
                        // Identity stuff
                        //

                        var identity = Braid.randomIdentity()
                        function refreshIdentityUI() {
                            document.getElementById('my-address').innerHTML = '<strong>Your address:</strong> ' + identity.address
                        }

                        refreshIdentityUI()

                        var inputMnemonic = document.getElementById('input-mnemonic')
                        document.getElementById('btn-login').addEventListener('click', () => {
                            identity = Braid.identityFromMnemonic(inputMnemonic.value)
                            refreshIdentityUI()
                        })


                        //
                        // Chat stuff
                        //

                        var mostRecentTxID = null
                        var messages = []
                        function refreshChatUI() {
                            var container = document.getElementById('container')
                            var html = ''
                            for (let msg of messages) {
                                html += '<div><b>' + msg.sender.substr(0, 6) + ':</b> ' + msg.text + '</div>'
                            }
                            container.innerHTML = html
                        }

                        Braid.get('/shrugisland/talk0/messages', (err, update) => {
                            if (err) {
                                throw new Error(err)
                            }
                            messages = update.data
                            mostRecentTxID = update.mostRecentTxID
                            refreshChatUI()
                        })

                        refreshChatUI()

                        var inputText = document.getElementById('input-text')
                        document.getElementById('btn-send').addEventListener('click', () => {
                            Braid.put({
                                id: Braid.util.randomID(),
                                parents: [ mostRecentTxID ],
                                url: 'axon.science:21231',
                                patches: [
                                    '.shrugisland.talk0.messages[' + messages.length + ':' + messages.length + '] = ' + JSON.stringify({text: inputText.value}),
                                ],
                            }, identity)
                        })
                    </script>
                    </html>
                "`),
		},
	}

	c1.Info(1, "sending tx to initialize talk channel...")
	err = c1.AddTx(context.Background(), tx1)
	if err != nil {
		c1.Errorf("%+v", err)
	}

	var (
		tx2 = rw.Tx{
			ID:      rw.RandomID(),
			Parents: []rw.ID{tx1.ID},
			From:    c1.Address(),
			URL:     "axon.science:21231",
			Patches: []rw.Patch{
				mustParsePatch(`.shrugisland.talk0.messages[0:0] = {"text":"hello!"}`),
			},
		}

		tx3 = rw.Tx{
			ID:      rw.RandomID(),
			Parents: []rw.ID{tx2.ID},
			From:    c1.Address(),
			URL:     "axon.science:21231",
			Patches: []rw.Patch{
				mustParsePatch(`.shrugisland.talk0.messages[1:1] = {"text":"well hello to you too"}`),
			},
		}

		tx4 = rw.Tx{
			ID:      rw.RandomID(),
			Parents: []rw.ID{tx3.ID},
			From:    c1.Address(),
			URL:     "axon.science:21231",
			Patches: []rw.Patch{
				mustParsePatch(`.shrugisland.talk0.messages[2:2] = {"text":"yoooo"}`),
			},
		}
	)

	c1.Info(1, "sending tx 4...")
	err = c1.AddTx(context.Background(), tx4)
	if err != nil {
		c1.Errorf("xxx %+v", err)
	}
	c1.Info(1, "sending tx 3...")
	err = c1.AddTx(context.Background(), tx3)
	if err != nil {
		c1.Errorf("zzz %+v", err)
	}
	c1.Info(1, "sending tx 2...")
	err = c1.AddTx(context.Background(), tx2)
	if err != nil {
		c1.Errorf("yyy %+v", err)
	}

	rw.NewHTTPServer(c1)

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
