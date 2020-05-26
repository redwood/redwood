package main

import (
	"context"
	"flag"
	"os"
	"time"

	"github.com/brynbellomy/klog"

	rw "github.com/brynbellomy/redwood"
	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/demos/demoutils"
	"github.com/brynbellomy/redwood/types"
)

type app struct {
	ctx.Context
}

func main() {
	flagset := flag.NewFlagSet("", flag.ContinueOnError)
	klog.InitFlags(flagset)
	flagset.Set("logtostderr", "true")
	flagset.Set("v", "2")
	klog.SetFormatter(&klog.FmtConstWidth{
		FileNameCharWidth: 24,
		UseColor:          true,
	})

	// Make two Go hosts that will communicate with one another over libp2p
	host1 := demoutils.MakeHost("fad9c8855b740a0b7ed4c221dbad0f33a83a49cad6b3fe8d5817ac83d38b6a19", 21231, "localhost:21231/editor", "cookiesecret1", "server1.crt", "server1.key")
	host2 := demoutils.MakeHost("deadbeef5b740a0b7ed4c22149cadbaddeadbeefd6b3fe8d5817ac83deadbeef", 21241, "localhost:21231/editor", "cookiesecret2", "server2.crt", "server2.key")

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

	// Both consumers subscribe to the StateURI
	ctx, _ := context.WithTimeout(context.Background(), 120*time.Second)
	go func() {
		anySucceeded, _ := host2.Subscribe(ctx, "localhost:21231/editor")
		if !anySucceeded {
			panic("host2 could not subscribe")
		}
	}()

	go func() {
		anySucceeded, _ := host1.Subscribe(ctx, "localhost:21231/editor")
		if !anySucceeded {
			panic("host1 could not subscribe")
		}
	}()

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
	_, sync9Hash, err := host1.AddRef(sync9)
	if err != nil {
		panic(err)
	}
	indexHTML, err := os.Open("./index.html")
	if err != nil {
		panic(err)
	}
	_, indexHTMLHash, err := host1.AddRef(indexHTML)
	if err != nil {
		panic(err)
	}

	// These are just convenience utils
	hostsByAddress := map[types.Address]rw.Host{
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

	//
	// Setup our text editor using a single transaction.  This channel has:
	//   - a string containing the document's text (at the keypath ".text.value")
	//   - an index.html page (for interacting with the editor from a web browser)
	//   - a "sync9" merge resolver (which is good at intelligently merging concurrent updates
	//       from multiple users).  Notice that we uploaded the Javascript code for this resolver
	//       to the node above ^, and we're now referring to it by its hash.
	//   - a "permissions" validator (which says that any user may write to the .text key)
	//
	var (
		genesisTx = rw.Tx{
			ID:       rw.GenesisTxID,
			Parents:  []types.ID{},
			From:     host1.Address(),
			StateURI: "localhost:21231/editor",
			Patches: []rw.Patch{
				mustParsePatch(` = {
					"text": {
						"value": "",

                        "Validator": {
                            "Content-Type": "validator/permissions",
                            "value": {
                                "*": {
                                    "^\\.value.*": {
                                        "write": true
                                    }
                                }
                            }
                        },
						"Merge-Type": {
							"Content-Type": "resolver/js",
							"value": {
								"src": {
									"Content-Type": "link",
									"value": "ref:sha3:` + sync9Hash.String() + `"
								}
							}
						}
					},
					"index.html": {
						"Content-Type": "text/html",
						"value": {
							"Content-Type": "link",
							"value": "ref:sha3:` + indexHTMLHash.String() + `"
						}
					},
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
								"^\\.text\\.value.*": {
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
	)

	host1.Info(1, "sending tx to initialize text editor channel...")
	sendTx(genesisTx)

	//go func() {
	//    time.Sleep(2 * time.Second)
	//    host1.Controller().DebugLockResolvers()
	//}()
}

func mustParsePatch(s string) rw.Patch {
	p, err := rw.ParsePatch([]byte(s))
	if err != nil {
		panic(err.Error() + ": " + s)
	}
	return p
}
