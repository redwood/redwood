package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"
	"time"

	rw "github.com/brynbellomy/redwood"
	"github.com/brynbellomy/redwood/ctx"
	// "github.com/brynbellomy/redwood/remotestore"
)

type app struct {
	ctx.Context
}

func makeHost(signingKeypairHex string, port uint, dbfile, refStoreRoot, cookieSecretStr, tlsCertFilename, tlsKeyFilename string) rw.Host {
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
	// store := remotestore.NewClient("0.0.0.0:4567", signingKeypair.Address(), signingKeypair.SigningPrivateKey)
	refStore := rw.NewRefStore(refStoreRoot)
	controller, err := rw.NewController(signingKeypair.Address(), genesis, store, refStore)
	if err != nil {
		panic(err)
	}

	p2ptransport, err := rw.NewLibp2pTransport(signingKeypair.Address(), port, refStore)
	if err != nil {
		panic(err)
	}

	var cookieSecret [32]byte
	copy(cookieSecret[:], []byte(cookieSecretStr))
	httptransport, err := rw.NewHTTPTransport(signingKeypair.Address(), port+1, controller, refStore, signingKeypair, cookieSecret, tlsCertFilename, tlsKeyFilename)
	if err != nil {
		panic(err)
	}

	transport := &bothTransports{p2ptransport, httptransport}

	h, err := rw.NewHost(signingKeypair, encryptingKeypair, port, transport, controller, refStore)
	if err != nil {
		panic(err)
	}

	err = httptransport.Start()
	if err != nil {
		panic(err)
	}
	httptransport.SetTxHandler(h.OnTxReceived)
	httptransport.SetFetchHistoryHandler(h.OnFetchHistoryRequestReceived)

	return h
}

type bothTransports struct {
	rw.Transport
	httpTransport rw.Transport
}

func (t *bothTransports) Libp2pPeerID() string {
	return t.Transport.(interface{ Libp2pPeerID() string }).Libp2pPeerID()
}

func (t *bothTransports) ForEachSubscriberToURL(ctx context.Context, theURL string) (<-chan rw.Peer, error) {
	chHttp, err := t.httpTransport.ForEachSubscriberToURL(ctx, theURL)
	if err != nil {
		//return nil, err
	}
	chOther, err := t.Transport.ForEachSubscriberToURL(ctx, theURL)
	if err != nil {
		return nil, err
	}

	ch := make(chan rw.Peer)

	go func() {
		defer close(ch)

		if chHttp != nil {
			for p := range chHttp {
				ch <- p
			}
		}
		for p := range chOther {
			ch <- p
		}
	}()
	return ch, nil
}

func main() {

	flag.Parse()
	flag.Set("logtostderr", "true")
	flag.Set("v", "2")

	os.MkdirAll("/tmp/forest", 0700)

	host1 := makeHost("fad9c8855b740a0b7ed4c221dbad0f33a83a49cad6b3fe8d5817ac83d38b6a19", 21231, "/tmp/forest/badger1", "/tmp/forest/refs1", "cookiesecret1", "server1.crt", "server1.key")
	host2 := makeHost("deadbeef5b740a0b7ed4c22149cadbaddeadbeefd6b3fe8d5817ac83deadbeef", 21241, "/tmp/forest/badger2", "/tmp/forest/refs2", "cookiesecret2", "server2.crt", "server2.key")

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
		err = host1.AddPeer(host1.Ctx(), "localhost:21241")
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
	sync9, err := os.Open("../braidjs/dist/sync9-otto.js")
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

	// Setup talk channel using transactions
	var tx1 = rw.Tx{
		ID:      rw.IDFromString("one"),
		Parents: []rw.ID{rw.GenesisTxID},
		From:    host1.Address(),
		URL:     "localhost:21231",
		Patches: []rw.Patch{
			mustParsePatch(`.shrugisland = ` + deterministicJSON(`{
                "talk0": {
                    "index": {"link": "ref:`+indexHTMLHash.String()+`"},
                    "messages": [],
                    "resolver": {"type": "js", "src": { "link": "ref:`+sync9Hash.String()+`" }},
                    "validator": {"type": "permissions"},
                    "permissions": {
                        "96216849c49358b10257cb55b28ea603c874b05e": {
                            "^.*$": {
                                "read": true,
                                "write": true
                            }
                        },
                        "*": {
                            "^\\.permissions.*$": {
                                "read": true,
                                "write": false
                            },
                            "^\\.index.*$": {
                                "read": true,
                                "write": false
                            },
                            "^\\.messages.*": {
                                "read": true,
                                "write": true
                            }
                        }
                    }
                }
            }`)),
		},
	}

	host1.Info(1, "sending tx to initialize talk channel...")
	err = host1.SendTx(context.Background(), tx1)
	if err != nil {
		host1.Errorf("%+v", err)
	}

	time.Sleep(5 * time.Second)

	var (
		tx2 = rw.Tx{
			ID:      rw.IDFromString("two"),
			Parents: []rw.ID{tx1.ID},
			From:    host1.Address(),
			URL:     "localhost:21231",
			Patches: []rw.Patch{
				mustParsePatch(`.shrugisland.talk0.messages[0:0] = [{"text":"hello!","sender":"` + host1.Address().String() + `"}]`),
				// mustParsePatch(`.shrugisland.talk0.secretThing = {"welcome":"you've stumbled upon a secret place!","sensitiveInformation":[1,2,3]}`),
			},
		}

		tx3 = rw.Tx{
			ID:      rw.IDFromString("three"),
			Parents: []rw.ID{tx2.ID},
			From:    host2.Address(),
			URL:     "localhost:21231",
			Patches: []rw.Patch{
				mustParsePatch(`.shrugisland.talk0.messages[1:1] = [{"text":"well hello to you too","sender":"` + host2.Address().String() + `"}]`),
			},
		}

		tx4 = rw.Tx{
			ID:      rw.IDFromString("four"),
			Parents: []rw.ID{tx3.ID},
			From:    host1.Address(),
			URL:     "localhost:21231",
			Patches: []rw.Patch{
				mustParsePatch(`.shrugisland.talk0.messages[2:2] = [{"text":"who needs a meme?","sender":"` + host1.Address().String() + `","attachment":{"link":"ref:` + memeHash.String() + `"}}]`),
			},
		}
	)

	host1.Info(1, "sending tx 4...")
	err = host1.SendTx(context.Background(), tx4)
	if err != nil {
		host1.Errorf("xxx %+v", err)
	}
	host2.Info(1, "sending tx 3...")
	err = host2.SendTx(context.Background(), tx3)
	if err != nil {
		host2.Errorf("zzz %+v", err)
	}
	host1.Info(1, "sending tx 2...")
	err = host1.SendTx(context.Background(), tx2)
	if err != nil {
		host1.Errorf("yyy %+v", err)
	}

	// var (
	// 	recipients = []rw.Address{host1.Address(), host2.Address()}
	// 	tx5        = rw.Tx{
	// 		ID:      rw.IDFromString("five"),
	// 		Parents: []rw.ID{tx4.ID},
	// 		From:    host2.Address(),
	// 		URL:     "localhost:21231",
	// 		Patches: []rw.Patch{
	// 			mustParsePatch(`.` + rw.PrivateRootKeyForRecipients(recipients) + `.shrugisland.talk0.messages[0:0] = [{"text":"private message for you!"}]`),
	// 		},
	// 		Recipients: recipients,
	// 	}
	// )

	// err = host2.SendTx(context.Background(), tx5)
	// if err != nil {
	// 	host2.Errorf("qqq %+v", err)
	// }
}

func mustParsePatch(s string) rw.Patch {
	p, err := rw.ParsePatch(s)
	if err != nil {
		panic(err.Error() + ": " + s)
	}
	return p
}

func deterministicJSON(s string) string {
	var x interface{}
	err := json.Unmarshal([]byte(s), &x)
	if err != nil {
		panic(err)
	}

	bs, err := json.Marshal(x)
	if err != nil {
		panic(err)
	}
	return string(bs)
}
