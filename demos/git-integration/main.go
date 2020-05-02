package main

import (
	"context"
	"flag"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	"github.com/brynbellomy/klog"

	rw "github.com/brynbellomy/redwood"
	"github.com/brynbellomy/redwood/ctx"
	"github.com/brynbellomy/redwood/demos/demoutils"
	"github.com/brynbellomy/redwood/types"
	// "github.com/brynbellomy/redwood/remotestore"
)

type app struct {
	ctx.Context
}

func main() {
	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()

	flagset := flag.NewFlagSet("", flag.ContinueOnError)
	klog.InitFlags(flagset)
	flagset.Set("logtostderr", "true")
	flagset.Set("v", "2")
	klog.SetFormatter(&klog.FmtConstWidth{
		FileNameCharWidth: 24,
		UseColor:          true,
	})

	// Make two Go hosts that will communicate with one another over libp2p
	host1 := demoutils.MakeHost("fad9c8855b740a0b7ed4c221dbad0f33a83a49cad6b3fe8d5817ac83d38b6a19", 21231, "localhost:21231/gitdemo", "cookiesecret1", "server1.crt", "server1.key")
	host2 := demoutils.MakeHost("deadbeef5b740a0b7ed4c22149cadbaddeadbeefd6b3fe8d5817ac83deadbeef", 21241, "localhost:21231/gitdemo", "cookiesecret2", "server2.crt", "server2.key")

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
	ctx, _ := context.WithTimeout(context.Background(), 120*time.Second)
	go func() {
		anySucceeded, _ := host2.Subscribe(ctx, "localhost:21231/gitdemo")
		if !anySucceeded {
			panic("host2 could not subscribe")
		}
		anySucceeded, _ = host2.Subscribe(ctx, "localhost:21231/git")
		if !anySucceeded {
			panic("host2 could not subscribe")
		}
		anySucceeded, _ = host2.Subscribe(ctx, "localhost:21231/git-reflog")
		if !anySucceeded {
			panic("host2 could not subscribe")
		}
	}()

	go func() {
		anySucceeded, _ := host1.Subscribe(ctx, "localhost:21231/gitdemo")
		if !anySucceeded {
			panic("host1 could not subscribe")
		}
		anySucceeded, _ = host1.Subscribe(ctx, "localhost:21231/git")
		if !anySucceeded {
			panic("host1 could not subscribe")
		}
		anySucceeded, _ = host1.Subscribe(ctx, "localhost:21231/git-reflog")
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
	indexHTML, err := os.Open("./repo/index.html")
	if err != nil {
		panic(err)
	}
	indexHTMLHash, err := host1.AddRef(indexHTML, "text/html")
	if err != nil {
		panic(err)
	}
	scriptJS, err := os.Open("./repo/script.js")
	if err != nil {
		panic(err)
	}
	scriptJSHash, err := host1.AddRef(scriptJS, "application/javascript")
	if err != nil {
		panic(err)
	}
	readme, err := os.Open("./repo/README.md")
	if err != nil {
		panic(err)
	}
	readmeHash, err := host1.AddRef(readme, "text/markdown")
	if err != nil {
		panic(err)
	}
	redwoodJpg, err := os.Open("./repo/redwood.jpg")
	if err != nil {
		panic(err)
	}
	redwoodJpgHash, err := host1.AddRef(redwoodJpg, "image/jpeg")
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

	// If you alter the contents of the ./repo subdirectory, you'll need to determine the
	// git commit hash of the first commit again, and then tweak these variables.  Otherwise,
	// you'll get a "bad object" error from git.
	commit1Hash := "e7098ece1ce234fac2c8c4f5ffdf049d5266f9f2"
	commit1Timestamp := "2020-05-01T19:00:45-05:00"

	commit1RepoTxID, err := types.IDFromHex(commit1Hash)
	if err != nil {
		panic(err)
	}

	//
	// Setup our git repo's 3 backing channels using 3 transactions.
	//
	var (
		// The "gitdemo" channel simply contains an index.html page for viewing the current state of the repo.
		genesisDemo = rw.Tx{
			ID:      rw.GenesisTxID,
			Parents: []types.ID{},
			From:    host1.Address(),
			URL:     "localhost:21231/gitdemo",
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
							}
						}
					},
					"providers": [
						"localhost:21231",
						"localhost:21241"
					],
					"index.html": {
						"Content-Type": "link",
						"value": "state:localhost:21231/git/files/index.html"
					}
				}`),
			},
		}

		// The "git" channel stores the file tree for each commit.  The files are stored under a "files" key,
		// and other important metadata are stored under the other keys.  The permissions validator allows
		// any user to write to these keys, but you'll want to set up your own repo to be more restrictive.
		genesisRepo = rw.Tx{
			ID:      rw.GenesisTxID,
			Parents: []types.ID{},
			From:    host1.Address(),
			URL:     "localhost:21231/git",
			Patches: []rw.Patch{
				mustParsePatch(` = {
					"Validator": {
						"Content-Type": "validator/permissions",
						"value": {
							"96216849c49358b10257cb55b28ea603c874b05e": {
								"^.*$": {
									"write": true
								}
							},
							"*": {
								"^\\.message.*": {
									"write": true
								},
								"^\\.timestamp.*": {
									"write": true
								},
								"^\\.author.*": {
									"write": true
								},
								"^\\.committer.*": {
									"write": true
								},
								"^\\.files.*": {
									"write": true
								}
							}
						}
					}
				}`),
			},
		}

		// The "git-reflog" channel contains a single "refs" key which stores the current commit
		// for each ref (i.e. each branch or tag).
		genesisReflog = rw.Tx{
			ID:      rw.GenesisTxID,
			Parents: []types.ID{},
			From:    host1.Address(),
			URL:     "localhost:21231/git-reflog",
			Patches: []rw.Patch{
				mustParsePatch(` = {
					"Validator": {
						"Content-Type": "validator/permissions",
						"value": {
							"96216849c49358b10257cb55b28ea603c874b05e": {
								"^.*$": {
									"write": true
								}
							},
							"*": {
								"^\\.refs.*": {
									"write": true
								}
							}
						}
					}
				}`),
			},
		}

		// Finally, we submit two transactions (one to "git" and one to "git-reflog") that simulate what
		// would happen if a user were to push their first commit to the repo.  The repo is now able to be
		// cloned using the command "git clone redwood://localhost:21231/git"
		commit1Repo = rw.Tx{
			ID:         commit1RepoTxID,
			Parents:    []types.ID{genesisRepo.ID},
			From:       host1.Address(),
			URL:        "localhost:21231/git",
			Checkpoint: true,
			Patches: []rw.Patch{
				mustParsePatch(`.message = "First commit\n"`),
				mustParsePatch(`.timestamp = "` + commit1Timestamp + `"`),
				mustParsePatch(`.author = {
					"email": "bryn.bellomy@gmail.com",
					"name": "Bryn Bellomy",
					"timestamp": "` + commit1Timestamp + `"
				}`),
				mustParsePatch(`.committer = {
					"email": "bryn.bellomy@gmail.com",
					"name": "Bryn Bellomy",
					"timestamp": "` + commit1Timestamp + `"
				}`),
				mustParsePatch(`.files = {
					"README.md": {
						"Content-Type": "link",
						"mode": 33188,
						"value": "ref:` + readmeHash.Hex() + `"
					},
					"redwood.jpg": {
						"Content-Type": "link",
						"mode": 33188,
						"value": "ref:` + redwoodJpgHash.Hex() + `"
					},
					"index.html": {
						"Content-Type": "link",
						"mode": 33188,
						"value": "ref:` + indexHTMLHash.Hex() + `"
					},
					"script.js": {
						"Content-Type": "link",
						"mode": 33188,
						"value": "ref:` + scriptJSHash.Hex() + `"
					}
				}`),
			},
		}

		commit1Reflog = rw.Tx{
			ID:         types.RandomID(),
			Parents:    []types.ID{genesisReflog.ID},
			From:       host1.Address(),
			URL:        "localhost:21231/git-reflog",
			Checkpoint: true,
			Patches: []rw.Patch{
				mustParsePatch(`.refs = {
					"heads": {
						"master": "` + commit1Hash + `"
					}
				}`),
			},
		}
	)

	sendTx(genesisDemo)
	sendTx(genesisRepo)
	sendTx(genesisReflog)
	sendTx(commit1Repo)
	sendTx(commit1Reflog)
}

func mustParsePatch(s string) rw.Patch {
	p, err := rw.ParsePatch([]byte(s))
	if err != nil {
		panic(err.Error() + ": " + s)
	}
	return p
}
