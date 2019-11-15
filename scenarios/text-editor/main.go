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
	peerStore := rw.NewPeerStore(signingKeypair.Address())
	controller, err := rw.NewController(signingKeypair.Address(), genesis, store, refStore)
	if err != nil {
		panic(err)
	}

	p2ptransport, err := rw.NewLibp2pTransport(signingKeypair.Address(), port, refStore, peerStore)
	if err != nil {
		panic(err)
	}

	var cookieSecret [32]byte
	copy(cookieSecret[:], []byte(cookieSecretStr))
	httptransport, err := rw.NewHTTPTransport(signingKeypair.Address(), port+1, controller, refStore, peerStore, signingKeypair, cookieSecret, tlsCertFilename, tlsKeyFilename)
	if err != nil {
		panic(err)
	}

	transports := []rw.Transport{p2ptransport, httptransport}

	h, err := rw.NewHost(signingKeypair, encryptingKeypair, port, transports, controller, refStore, peerStore)
	if err != nil {
		panic(err)
	}

	return h
}

func main() {
	flag.Parse()
	flag.Set("logtostderr", "true")
	flag.Set("v", "2")

	os.MkdirAll("/tmp/redwood/text-editor", 0700)

	host1 := makeHost("fad9c8855b740a0b7ed4c221dbad0f33a83a49cad6b3fe8d5817ac83d38b6a19", 21231, "/tmp/redwood/text-editor/badger1", "/tmp/redwood/text-editor/refs1", "cookiesecret1", "server1.crt", "server1.key")
	host2 := makeHost("deadbeef5b740a0b7ed4c22149cadbaddeadbeefd6b3fe8d5817ac83deadbeef", 21241, "/tmp/redwood/text-editor/badger2", "/tmp/redwood/text-editor/refs2", "cookiesecret2", "server2.crt", "server2.key")

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

	time.Sleep(2 * time.Second)

	// Both consumers subscribe to the URL
	anySucceeded, _ := host2.Subscribe(host2.Ctx(), "localhost:21231")
	if !anySucceeded {
		panic("host2 could not subscribe")
	}

	anySucceeded, _ = host1.Subscribe(host1.Ctx(), "localhost:21231")
	if !anySucceeded {
		panic("host1 could not subscribe")
	}

	sendTxs(host1, host2)

	app.AttachInterruptHandler()
	app.CtxWait()
}

func sendTxs(host1, host2 rw.Host) {
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

	// Setup talk channel using transactions
	var tx1 = rw.Tx{
		ID:      rw.IDFromString("one"),
		Parents: []rw.ID{rw.GenesisTxID},
		From:    host1.Address(),
		URL:     "localhost:21231",
		Patches: []rw.Patch{
			mustParsePatch(`.editor = {
				"text": "",
				"index": {
					"Content-Type": "text/html",
					"src": {
						"Content-Type": "link",
						"value": "ref:` + indexHTMLHash.String() + `"
					}
				},
				"Merge-Type": {
					"Content-Type": "resolver/js",
					"src": {
						"Content-Type": "link",
						"value": "ref:` + sync9Hash.String() + `"
					}
				},
				"Validator": {
					"Content-Type": "validator/permissions",
					"permissions": {
						"96216849c49358b10257cb55b28ea603c874b05e": {
							"^.*$": {
								"write": true
							}
						},
						"*": {
							"^\\.text.*": {
								"write": true
							}
						}
					}
				}
            }`),
		},
	}

	host1.Info(1, "sending tx to initialize talk channel...")
	sendTx(tx1)

	go func() {
		time.Sleep(2 * time.Second)
		host1.Controller().DebugLockResolvers()
	}()
}

func mustParsePatch(s string) rw.Patch {
	p, err := rw.ParsePatch(s)
	if err != nil {
		panic(err.Error() + ": " + s)
	}
	return p
}
