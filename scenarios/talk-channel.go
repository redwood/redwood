package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"time"

	rw "github.com/brynbellomy/redwood"
	"github.com/brynbellomy/redwood/ctx"
)

type M = map[string]interface{}

type app struct {
	ctx.Context
}

func makeHost(signingKeypairHex string, port uint, dbfile string) rw.Host {
	signingKeypair, err := rw.SigningKeypairFromHex(signingKeypairHex)
	if err != nil {
		panic(err)
	}

	encryptingKeypair, err := rw.GenerateEncryptingKeypair()
	if err != nil {
		panic(err)
	}

	genesisBytes, err := ioutil.ReadFile("genesis.json")
	if err != nil {
		panic(err)
	}

	var genesis map[string]interface{}
	err = json.Unmarshal(genesisBytes, &genesis)
	if err != nil {
		panic(err)
	}
	store := rw.NewBadgerStore(dbfile, signingKeypair.Address())
	controller, err := rw.NewController(signingKeypair.Address(), genesis, store)
	if err != nil {
		panic(err)
	}
	h, err := rw.NewHost(signingKeypair, encryptingKeypair, port, controller)
	if err != nil {
		panic(err)
	}
	return h
}

func main() {

	flag.Parse()
	flag.Set("logtostderr", "true")
	flag.Set("v", "2")

	host1 := makeHost("fad9c8855b740a0b7ed4c221dbad0f33a83a49cad6b3fe8d5817ac83d38b6a19", 21231, "/tmp/badger1")
	host2 := makeHost("deadbeef5b740a0b7ed4c22149cadbaddeadbeefd6b3fe8d5817ac83deadbeef", 21241, "/tmp/badger2")

	err := host1.Start()
	if err != nil {
		panic(err)
	}
	err = host2.Start()
	if err != nil {
		panic(err)
	}

	app := app{}
	app.CtxAddChild(host1.Ctx(), nil)
	app.CtxAddChild(host2.Ctx(), nil)
	app.CtxStart(
		func() error { return nil },
		nil,
		nil,
		nil,
	)

	// Connect the two consumers
	if libp2pTransport, is := host1.Transport().(interface{ Libp2pPeerID() string }); is {
		host2.AddPeer(host2.Ctx(), "/ip4/0.0.0.0/tcp/21231/p2p/"+libp2pTransport.Libp2pPeerID())
	} else {
		err := host2.AddPeer(host2.Ctx(), "localhost:21231")
		if err != nil {
			panic(err)
		}
		err = host1.AddPeer(host2.Ctx(), "localhost:21241")
		if err != nil {
			panic(err)
		}
	}

	time.Sleep(2 * time.Second)

	// Both consumers subscribe to the URL
	err = host2.Subscribe(host2.Ctx(), "localhost:21231")
	if err != nil {
		panic(err)
	}

	err = host1.Subscribe(host1.Ctx(), "localhost:21231")
	if err != nil {
		panic(err)
	}

	sendTxs(host1, host2)

	app.AttachInterruptHandler()
	app.CtxWait()
}

func sendTxs(host1, host2 rw.Host) {
	// Setup talk channel using transactions
	var tx1 = rw.Tx{
		ID:      rw.IDFromString("one"),
		Parents: []rw.Hash{rw.GenesisTxHash},
		From:    host1.Address(),
		URL:     "localhost:21231",
		Patches: []rw.Patch{
			mustParsePatch(`.shrugisland.talk0.permissions = {
                "96216849c49358b10257cb55b28ea603c874b05e": {
                    "^.*$": {
                        "read": true,
                        "write": true
                    }
                },
                "*": {
                    "^\\.messages.*$": {
                        "read": true,
                        "write": true
                    }
                }
            }`),
			mustParsePatch(`.shrugisland.talk0.messages = []`),
			mustParsePatch(`.shrugisland.talk0.validator = {"type": "permissions"}`),
			mustParsePatch(`.shrugisland.talk0.resolver = {"type":"lua", "src":"
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
                        error('only patches that append messages to the end of the channel are allowed')
                    end
                    return state
                end
            "}`),
			mustParsePatch(`.shrugisland.talk0.index = "
                    <html>
                    <head>
                        <style>
                            * {
                                font-family: 'Consolas', 'Ubuntu Mono', 'Monaco', 'Courier New', Courier, sans-serif;
                            }
                            #debug-state {
                                background: #eaeaea;
                                font-size: 0.7rem;
                                padding: 10px;
                                border-radius: 5px;
                                margin-left: 40px;
                            }
                        </style>
                    </head>
                    <body>
                        <div style='display: flex'>
                            <div>
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
                            </div>
                            <div>
                                <code>
                                    <pre id='debug-state'>
                                    </pre>
                                </code>
                            </div>
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

                        var mostRecentTxHash = null
                        var messages = []
                        function refreshChatUI() {
                            var container = document.getElementById('container')
                            var html = ''
                            for (let msg of messages) {
                                html += '<div><b>' + msg.sender.substr(0, 6) + ':</b> ' + msg.text + '</div>'
                            }
                            container.innerHTML = html
                        }

                        var debugStateElem = document.getElementById('debug-state')
                        Braid.get('/', (err, update) => {
                            if (err) {
                                throw new Error(err)
                            }
                            console.log(update.data)
                            var j = JSON.stringify(update.data, null, 4)
                            j = j.replace(/\\\\n/g, '\\n')
                                 .replace(/</g, '&lt;')
                                 .replace(/>/g, '&gt;')
                            console.log('j ~>', j)

                            debugStateElem.innerHTML = j
                        })

                        Braid.get('/shrugisland/talk0/messages', (err, update) => {
                            if (err) {
                                throw new Error(err)
                            }
                            messages = update.data
                            mostRecentTxHash = update.mostRecentTxHash
                            refreshChatUI()
                        })

                        refreshChatUI()

                        var inputText = document.getElementById('input-text')
                        document.getElementById('btn-send').addEventListener('click', () => {
                            Braid.put({
                                id: Braid.util.randomID(),
                                parents: [ mostRecentTxHash ],
                                url: 'localhost:21231',
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

	host1.Info(1, "sending tx to initialize talk channel...")
	err := host1.SendTx(context.Background(), tx1)
	if err != nil {
		host1.Errorf("%+v", err)
	}

	var (
		tx2 = rw.Tx{
			ID:      rw.IDFromString("two"),
			Parents: []rw.Hash{tx1.Hash()},
			From:    host1.Address(),
			URL:     "localhost:21231",
			Patches: []rw.Patch{
				mustParsePatch(`.shrugisland.talk0.messages[0:0] = {"text":"hello!"}`),
			},
		}

		tx3 = rw.Tx{
			ID:      rw.IDFromString("three"),
			Parents: []rw.Hash{tx2.Hash()},
			From:    host1.Address(),
			URL:     "localhost:21231",
			Patches: []rw.Patch{
				mustParsePatch(`.shrugisland.talk0.messages[1:1] = {"text":"well hello to you too"}`),
			},
		}

		tx4 = rw.Tx{
			ID:      rw.IDFromString("four"),
			Parents: []rw.Hash{tx3.Hash()},
			From:    host2.Address(),
			URL:     "localhost:21231",
			Patches: []rw.Patch{
				mustParsePatch(`.shrugisland.talk0.messages[2:2] = {"text":"yoooo"}`),
			},
		}
	)

	host2.Info(1, "sending tx 4...")
	err = host2.SendTx(context.Background(), tx4)
	if err != nil {
		host2.Errorf("xxx %+v", err)
	}
	host1.Info(1, "sending tx 3...")
	err = host1.SendTx(context.Background(), tx3)
	if err != nil {
		host1.Errorf("zzz %+v", err)
	}
	host1.Info(1, "sending tx 2...")
	err = host1.SendTx(context.Background(), tx2)
	if err != nil {
		host1.Errorf("yyy %+v", err)
	}

	var (
		recipients = []rw.Address{host1.Address(), host2.Address()}
		tx5        = rw.Tx{
			ID:      rw.IDFromString("four"),
			Parents: []rw.Hash{tx4.Hash()},
			From:    host2.Address(),
			URL:     "localhost:21231",
			Patches: []rw.Patch{
				mustParsePatch(`.` + rw.PrivateRootKeyForRecipients(recipients) + `.shrugisland.talk0.messages[2:2] = {"text":"private message for you!"}`),
			},
			Recipients: recipients,
		}
	)

	err = host2.SendTx(context.Background(), tx5)
	if err != nil {
		host2.Errorf("yyy %+v", err)
	}
}

func mustParsePatch(s string) rw.Patch {
	p, err := rw.ParsePatch(s)
	if err != nil {
		panic(err.Error() + ": " + s)
	}
	return p
}
