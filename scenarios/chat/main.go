package main

import (
	"context"
	"flag"
	"os"
	"time"

	rw "github.com/brynbellomy/redwood"
	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/scenarios/demoutils"
)

type app struct {
	ctx.Context
}

func main() {

	flag.Parse()
	flag.Set("logtostderr", "true")
	flag.Set("v", "2")

	os.MkdirAll("/tmp/redwood", 0700)

	// Make two Go hosts that will communicate with one another over libp2p
	host1 := demoutils.MakeHost("fad9c8855b740a0b7ed4c221dbad0f33a83a49cad6b3fe8d5817ac83d38b6a19", 21231, "/tmp/redwood/badger1", "/tmp/redwood/refs1", "cookiesecret1", "server1.crt", "server1.key")
	host2 := demoutils.MakeHost("deadbeef5b740a0b7ed4c22149cadbaddeadbeefd6b3fe8d5817ac83deadbeef", 21241, "/tmp/redwood/badger2", "/tmp/redwood/refs2", "cookiesecret2", "server2.crt", "server2.key")

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

	// Connect the two peers
	libp2pTransport := host1.Transport("libp2p").(interface{ Libp2pPeerID() string })
	host2.AddPeer(host2.Ctx(), "libp2p", rw.NewStringSet([]string{"/ip4/0.0.0.0/tcp/21231/p2p/" + libp2pTransport.Libp2pPeerID()}))
	//err = host2.AddPeer(host2.Ctx(), "http", "https://localhost:21232")
	//if err != nil {
	//    panic(err)
	//}
	//err = host1.AddPeer(host1.Ctx(), "http", "https://localhost:21242")
	//if err != nil {
	//    panic(err)
	//}

	// Both consumers subscribe to the channel URI
	ctx, _ := context.WithTimeout(context.Background(), 120*time.Second)
	go func() {
		anySucceeded, _ := host2.Subscribe(ctx, "localhost:21231/chat")
		if !anySucceeded {
			panic("host2 could not subscribe")
		}
	}()

	go func() {
		anySucceeded, _ := host1.Subscribe(ctx, "localhost:21231/chat")
		if !anySucceeded {
			panic("host1 could not subscribe")
		}
	}()

	// Now, let's construct our chat room with a few transactions
	sendTxs(host1, host2)

	app.AttachInterruptHandler()
	app.CtxWait()
}

func sendTxs(host1, host2 rw.Host) {
	// Before sending any transactions, we upload some resources we're going to need
	// into the RefStore of the node.  These resources can be referred to in the state
	// tree by their hash.
	sync9, err := os.Open("./sync9-otto.js")
	if err != nil {
		panic(err)
	}
	sync9Hash, err := host1.AddRef(sync9, "application/js")
	if err != nil {
		panic(err)
	}
	indexHTML, err := os.Open("./index.html")
	if err != nil {
		panic(err)
	}
	indexHTMLHash, err := host1.AddRef(indexHTML, "text/html")
	if err != nil {
		panic(err)
	}
	meme, err := os.Open("./meme.jpg")
	if err != nil {
		panic(err)
	}
	memeHash, err := host1.AddRef(meme, "image/jpg")
	if err != nil {
		panic(err)
	}

	hostsByAddress := map[rw.Address]rw.Host{
		host1.Address(): host1,
		host2.Address(): host2,
	}

	sendTx := func(tx rw.Tx) {
		host := hostsByAddress[tx.From]
		err := host.SendTx(context.Background(), tx)
		if err != nil {
			host.Errorf("%+v", err)
		}
	}

	var (
		//
		// First, we set up the outermost merge resolver and the validator, which essentially
		// say that the "god" user (the one who initiated the channel) is permitted to modify
		// anything anywhere in the state tree.
		//
		genesisTx = rw.Tx{
			ID:      rw.GenesisTxID,
			Parents: []rw.ID{},
			From:    host1.Address(),
			URL:     "localhost:21231/chat",
			Patches: []rw.Patch{
				mustParsePatch(` = {
					"Merge-Type": {
						"Content-Type": "resolver/dumb",
						"value": {}
					},
					"Validator": {
						"Content-Type": "validator/permissions",
						"value": {
							"96216849c49358b10257cb55b28ea603c874b05e": {
								"^.*$": {
									"write": true
								}
							},
							"*": {
								"^\\.private-.*": {
									"write": true
								}
							}
						}
					},
					"providers": [
						"localhost:21231",
						"localhost:21241"
					]
				}`),
			},
		}

		//
		// Then, we set up the chat room itself.  It has:
		//   - an array of messages
		//   - an index.html page (for interacting with the chat from a web browser)
		//   - a "sync9" merge resolver (which is good at intelligently merging concurrent updates
		//       from multiple users).  Notice that we uploaded the Javascript code for this resolver
		//       to the node above ^, and we're now referring to it by its hash.
		//   - a "permissions" validator (which says that any user may write to the .messages key)
		//
		tx1 = rw.Tx{
			ID:      rw.RandomID(),
			Parents: []rw.ID{rw.GenesisTxID},
			From:    host1.Address(),
			URL:     "localhost:21231/chat",
			Patches: []rw.Patch{
				mustParsePatch(`.talk0 = {
					"messages": [],
					"index.html": {
						"Content-Type": "text/html",
						"value": {
							"Content-Type": "link",
							"value": "ref:` + indexHTMLHash.String() + `"
						}
					},
					"Merge-Type": {
						"Content-Type": "resolver/js",
						"value": {
							"src": {
								"Content-Type": "link",
								"value": "ref:` + sync9Hash.String() + `"
							}
						}
					},
					"Validator": {
						"Content-Type": "validator/permissions",
						"value": {
							"96216849c49358b10257cb55b28ea603c874b05e": {
								"^.*$": {
									"write": true
								}
							},
							"*": {
								"^\\.messages.*": {
									"write": true
								}
							}
						}
					}
				}`),
			},
		}
	)

	host1.Info(1, "sending tx to initialize talk channel...")
	sendTx(genesisTx)
	sendTx(tx1)

	//
	// Now, let's set up a place to store user profiles.  The permissions validator
	// in this part of the tree uses the special ${sender} token, which allows us to
	// declare that any user may only write to the keypath corresponding to their own
	// public key.
	//
	var (
		ptx1 = rw.Tx{
			ID:      rw.IDFromString("ptx1"),
			Parents: []rw.ID{tx1.ID},
			From:    host1.Address(),
			URL:     "localhost:21231/chat",
			Patches: []rw.Patch{
				mustParsePatch(`.users = {
					"Validator": {
						"Content-Type": "validator/permissions",
						"value": {
							"*": {
								"^\\.${sender}.*$": {
									"write": true 
								}
							}
						}
					}
				}`),
			},
		}

		ptx2 = rw.Tx{
			ID:      rw.IDFromString("ptx2"),
			Parents: []rw.ID{ptx1.ID},
			From:    host1.Address(),
			URL:     "localhost:21231/chat",
			Patches: []rw.Patch{
				mustParsePatch(`.users.` + host1.Address().Hex() + ` = {
                    "name": "Paul Stamets",
                    "occupation": "Astromycologist"
                }`),
			},
		}

		// Here, we also add a private portion of Paul Stamets' user profile.  Only he can view this data.
		ptx3recipients = []rw.Address{host1.Address()}
		ptx3           = rw.Tx{
			ID:      rw.GenesisTxID,
			Parents: []rw.ID{},
			From:    host1.Address(),
			URL:     "localhost:21231/" + rw.PrivateRootKeyForRecipients(ptx3recipients),
			Patches: []rw.Patch{
				mustParsePatch(`.profile = {
                    "public": {
						"Content-Type": "link",
						"value": "state:localhost:21231/chat/users/` + host1.Address().Hex() + `"
                    },
                    "secrets": {
                        "catName": "Stanley",
                        "favoriteDonutShape": "toroidal"
                    }
                }`),
			},
			Recipients: ptx3recipients,
		}
	)

	sendTx(ptx1)
	sendTx(ptx2)
	sendTx(ptx3)

	//
	// Here, we add a few initial messages to the chat.
	//
	var (
		tx2 = rw.Tx{
			ID:      rw.IDFromString("two"),
			Parents: []rw.ID{ptx2.ID},
			From:    host1.Address(),
			URL:     "localhost:21231/chat",
			Patches: []rw.Patch{
				mustParsePatch(`.talk0.messages[0:0] = [{"text":"hello!","sender":"` + host1.Address().String() + `"}]`),
			},
		}

		tx3 = rw.Tx{
			ID:      rw.IDFromString("three"),
			Parents: []rw.ID{tx2.ID},
			From:    host1.Address(),
			URL:     "localhost:21231/chat",
			Patches: []rw.Patch{
				mustParsePatch(`.talk0.messages[1:1] = [{"text":"well hello to you too","sender":"` + host2.Address().String() + `"}]`),
			},
		}

		tx4 = rw.Tx{
			ID:      rw.IDFromString("four"),
			Parents: []rw.ID{tx3.ID},
			From:    host1.Address(),
			URL:     "localhost:21231/chat",
			Patches: []rw.Patch{
				mustParsePatch(`.talk0.messages[2:2] = [{
					"text": "who needs a meme?",
					"sender": "` + host1.Address().String() + `",
					"attachment": {
						"Content-Type": "image/jpg",
						"value": {
							"Content-Type": "link",
							"value":"ref:` + memeHash.String() + `"
						}
					}
				}]`),
			},
		}
	)

	sendTx(tx4)
	sendTx(tx3)
	sendTx(tx2)
}

func mustParsePatch(s string) rw.Patch {
	p, err := rw.ParsePatch(s)
	if err != nil {
		panic(err.Error() + ": " + s)
	}
	return p
}
