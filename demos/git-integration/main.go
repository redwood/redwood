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
	host1 := demoutils.MakeHost("fad9c8855b740a0b7ed4c221dbad0f33a83a49cad6b3fe8d5817ac83d38b6a19", 21231, "somegitprovider.org/gitdemo", "cookiesecret1", "server1.crt", "server1.key")
	host2 := demoutils.MakeHost("deadbeef5b740a0b7ed4c22149cadbaddeadbeefd6b3fe8d5817ac83deadbeef", 21241, "somegitprovider.org/gitdemo", "cookiesecret2", "server2.crt", "server2.key")

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

	// Connect the two peers using libp2p
	libp2pTransport := host1.Transport("libp2p").(interface{ Libp2pPeerID() string })
	host2.AddPeer(host2.Ctx(), "libp2p", rw.NewStringSet([]string{"/ip4/0.0.0.0/tcp/21231/p2p/" + libp2pTransport.Libp2pPeerID()}))

	time.Sleep(2 * time.Second)

	// Both consumers subscribe to the URL
	ctx, _ := context.WithTimeout(context.Background(), 120*time.Second)
	go func() {
		anySucceeded, _ := host2.Subscribe(ctx, "somegitprovider.org/gitdemo")
		if !anySucceeded {
			panic("host2 could not subscribe")
		}
	}()

	go func() {
		anySucceeded, _ := host1.Subscribe(ctx, "somegitprovider.org/gitdemo")
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
	commit1Hash := "09c69f61e0e5c433e8b7c8be2124aeb1e558f13f"
	commit1Timestamp := "2020-05-07T17:01:31-05:00"

	commit1RepoTxID, err := types.IDFromHex(commit1Hash)
	if err != nil {
		panic(err)
	}

	//
	// Setup our git repo's state tree.
	//
	var (
		// The "gitdemo" channel contains:
		//   - A link to the current worktree so that we can browse it like a regular website.
		//   - All of the commit data that Git expects.  The files are stored under a "files" key,
		//         and other important metadata are stored under the other keys.
		//   - A mapping of refs (usually, branches) to commit hashes.
		//   - A permissions validator that allows anyone to write to the repo but tries to keep
		//         people from writing to the wrong keys.
		genesisDemo = rw.Tx{
			ID:      rw.GenesisTxID,
			Parents: []types.ID{},
			From:    host1.Address(),
			URL:     "somegitprovider.org/gitdemo",
			Patches: []rw.Patch{
				mustParsePatch(` = {
                    "demo": {
                        "Content-Type": "link",
                        "value": "state:somegitprovider.org/gitdemo/refs/heads/master/worktree"
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
                                "^\\.refs\\..*": {
                                    "write": true
                                },
                                "^\\.commits\\.[a-f0-9]+\\.parents\\..*": {
                                    "write": true
                                },
                                "^\\.commits\\.[a-f0-9]+\\.message\\..*": {
                                    "write": true
                                },
                                "^\\.commits\\.[a-f0-9]+\\.timestamp\\..*": {
                                    "write": true
                                },
                                "^\\.commits\\.[a-f0-9]+\\.author\\..*": {
                                    "write": true
                                },
                                "^\\.commits\\.[a-f0-9]+\\.committer\\..*": {
                                    "write": true
                                },
                                "^\\.commits\\.[a-f0-9]+\\.files\\..*": {
                                    "write": true
                                }
                            }
						}
					},
                    "refs": {
                        "heads": {}
                    },
                    "commits": {}
				}`),
			},
		}

		// Finally, we submit two transactions (one to "git" and one to "git-reflog") that simulate what
		// would happen if a user were to push their first commit to the repo.  The repo is now able to be
		// cloned using the command "git clone redwood://localhost:21231/git"
		commit1Repo = rw.Tx{
			ID:         commit1RepoTxID,
			Parents:    []types.ID{genesisDemo.ID},
			From:       host1.Address(),
			URL:        "somegitprovider.org/gitdemo",
			Checkpoint: true,
			Patches: []rw.Patch{
				mustParsePatch(`.commits.` + commit1Hash + `.message = "First commit\n"`),
				mustParsePatch(`.commits.` + commit1Hash + `.timestamp = "` + commit1Timestamp + `"`),
				mustParsePatch(`.commits.` + commit1Hash + `.author = {
					"email": "bryn.bellomy@gmail.com",
					"name": "Bryn Bellomy",
					"timestamp": "` + commit1Timestamp + `"
				}`),
				mustParsePatch(`.commits.` + commit1Hash + `.committer = {
					"email": "bryn.bellomy@gmail.com",
					"name": "Bryn Bellomy",
					"timestamp": "` + commit1Timestamp + `"
				}`),
				mustParsePatch(`.commits.` + commit1Hash + `.files = {
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
				mustParsePatch(`.refs.heads.master.HEAD = "` + commit1Hash + `"`),
				mustParsePatch(`.refs.heads.master.worktree = {
                    "Content-Type": "link",
                    "value": "state:somegitprovider.org/gitdemo/commits/` + commit1Hash + `/files"
                }`),
			},
		}
	)

	sendTx(genesisDemo)
	sendTx(commit1Repo)
}

func mustParsePatch(s string) rw.Patch {
	p, err := rw.ParsePatch([]byte(s))
	if err != nil {
		panic(err.Error() + ": " + s)
	}
	return p
}
