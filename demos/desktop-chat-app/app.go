package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/brynbellomy/klog"
	"github.com/markbates/pkger"
	"github.com/pkg/errors"

	"redwood.dev/blob"
	"redwood.dev/cmd/cmdutils"
	"redwood.dev/config"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/rpc"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/swarm/braidhttp"
	"redwood.dev/swarm/libp2p"
	"redwood.dev/tree"
	"redwood.dev/utils"
)

var app = &appType{
	Logger:  log.NewLogger("app"),
	devMode: true,
}

type appType struct {
	startStopMu   sync.Mutex
	started       bool
	blobStore     blob.Store
	txStore       tree.TxStore
	host          swarm.Host
	db            *state.DBTree
	httpRPCServer *http.Server
	chLoggedOut   chan struct{}

	keyStore    identity.KeyStore
	password    string
	profileRoot string
	profileName string
	mnemonic    string

	// These are set once on startup and never change
	log.Logger
	configPath string
	devMode    bool
}

func (app *appType) Start() (err error) {
	app.startStopMu.Lock()
	defer app.startStopMu.Unlock()
	if app.started {
		return errors.New("already started")
	}
	defer func() {
		perr := recover()
		if err == nil && perr == nil {
			app.started = true
		}
	}()

	app.chLoggedOut = make(chan struct{})

	cfg, err := config.ReadConfigAtPath("redwood-chat", app.configPath)
	if os.IsNotExist(err) {
		err := os.MkdirAll(filepath.Dir(app.configPath), 0777|os.ModeDir)
		if err != nil {
			return err
		}
		cfg, err = config.ReadConfigAtPath("redwood-chat", app.configPath)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// Ignore the config file's data root and use our profileRoot + profileName
	cfg.Node.DataRoot = filepath.Join(app.profileRoot, app.profileName)

	if app.devMode {
		cfg.Node.DevMode = true
	}

	err = app.ensureDataDirs(cfg)
	if err != nil {
		return err
	}

	db, err := state.NewDBTree(filepath.Join(cfg.Node.DataRoot, "peers"))
	if err != nil {
		return err
	}
	app.db = db

	var (
		txStore       = tree.NewBadgerTxStore(cfg.TxDBRoot())
		keyStore      = identity.NewBadgerKeyStore(db, identity.DefaultScryptParams)
		blobStore     = blob.NewDiskStore(cfg.RefDataRoot(), db)
		peerStore     = swarm.NewPeerStore(db)
		controllerHub = tree.NewControllerHub(cfg.StateDBRoot(), txStore, blobStore)
	)
	app.keyStore = keyStore

	err = blobStore.Start()
	if err != nil {
		return err
	}
	app.blobStore = blobStore

	err = txStore.Start()
	if err != nil {
		return err
	}
	app.txStore = txStore

	var transports []swarm.Transport

	if cfg.P2PTransport.Enabled {
		var bootstrapPeers []string
		for _, bp := range cfg.Node.BootstrapPeers {
			if bp.Transport != "libp2p" {
				continue
			}
			bootstrapPeers = append(bootstrapPeers, bp.DialAddresses...)
		}

		libp2pTransport := libp2p.NewTransport(
			cfg.P2PTransport.ListenPort,
			cfg.P2PTransport.ReachableAt,
			bootstrapPeers,
			controllerHub,
			keyStore,
			blobStore,
			peerStore,
		)
		if err != nil {
			return err
		}
		transports = append(transports, libp2pTransport)
	}

	if cfg.HTTPTransport.Enabled {
		tlsCertFilename := filepath.Join(cfg.Node.DataRoot, "..", "server.crt")
		tlsKeyFilename := filepath.Join(cfg.Node.DataRoot, "..", "server.key")

		// var cookieSecret [32]byte
		// copy(cookieSecret[:], []byte(cfg.HTTPTransport.CookieSecret))

		httpTransport, err := braidhttp.NewTransport(
			cfg.HTTPTransport.ListenHost,
			cfg.HTTPTransport.ReachableAt,
			cfg.HTTPTransport.DefaultStateURI,
			controllerHub,
			keyStore,
			blobStore,
			peerStore,
			// cookieSecret,
			tlsCertFilename,
			tlsKeyFilename,
			cfg.Node.DevMode,
		)
		if err != nil {
			return err
		}
		transports = append(transports, httpTransport)
	}

	err = app.keyStore.Unlock(app.password, app.mnemonic)
	if err != nil {
		app.blobStore.Close()
		app.blobStore = nil

		app.txStore.Close()
		app.txStore = nil

		app.db.Close()
		app.db = nil
		return err
	}

	app.host, err = swarm.NewHost(transports, controllerHub, keyStore, blobStore, peerStore, cfg)
	if err != nil {
		return err
	}

	err = app.host.Start()
	if err != nil {
		panic(err)
	}

	pkger.Walk("/frontend/build", func(path string, info os.FileInfo, err error) error {
		app.Infof(0, "Serving %v", path)
		return nil
	})

	if cfg.HTTPRPC.Enabled {
		rwRPC := rpc.NewHTTPServer(app.host)
		server := &HTTPRPCServer{rwRPC, keyStore}
		app.httpRPCServer, err = rpc.StartHTTPRPC(server, cfg.HTTPRPC)
		if err != nil {
			return err
		}
		app.Infof(0, "http rpc server listening on %v", cfg.HTTPRPC.ListenHost)
	}

	for _, bootstrapPeer := range cfg.Node.BootstrapPeers {
		bootstrapPeer := bootstrapPeer
		go func() {
			app.Infof(0, "connecting to bootstrap peer: %v %v", bootstrapPeer.Transport, bootstrapPeer.DialAddresses)
			for _, dialAddr := range bootstrapPeer.DialAddresses {
				app.host.AddPeer(swarm.PeerDialInfo{TransportName: bootstrapPeer.Transport, DialAddr: dialAddr})
			}
		}()
	}

	go func() {
		time.Sleep(5 * time.Second)
		for stateURI := range cfg.Node.SubscribedStateURIs {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if app == nil || app.host == nil {
				return
			}

			sub, err := app.host.Subscribe(ctx, stateURI, 0, nil, nil)
			if err != nil {
				app.Errorf("error subscribing to %v: %v", stateURI, err)
				continue
			}
			sub.Close()
			app.Successf("subscribed to %v", stateURI)
		}
	}()

	klog.Info(utils.PrettyJSON(cfg))
	klog.Flush()

	app.initializeLocalState()
	go app.monitorForDMs()

	repl := cmdutils.NewREPL(app.host, app.Logger, cmdutils.DefaultREPLCommands)
	go repl.Start()
	defer repl.Stop()

	select {
	case <-repl.Done():
	case <-utils.AwaitInterrupt():
	}

	return nil
}

func (app *appType) monitorForDMs() {
	time.Sleep(5 * time.Second)

	sub := app.host.SubscribeStateURIs()
	defer sub.Close()

	for {
		stateURI, err := sub.Read()
		if err != nil {
			app.Debugf("error in stateURI subscription: %v", err)
			return
		} else if stateURI == "" {
			continue
		}

		if strings.HasPrefix(stateURI, "chat.p2p/private-") {
			roomName := stateURI[len("chat.p2p/"):]
			roomKeypath := state.Keypath("rooms").Pushs(roomName)
			var found bool
			func() {
				dmState, err := app.host.Controllers().StateAtVersion("chat.local/dms", nil)
				if err != nil {
					panic(err)
				}
				defer dmState.Close()

				found, err = dmState.Exists(roomKeypath)
				if err != nil {
					panic(err)
				}
			}()
			if !found {
				err := app.host.SendTx(context.TODO(), tree.Tx{
					StateURI: "chat.local/dms",
					Patches: []tree.Patch{{
						Keypath: roomKeypath,
						Val:     true,
					}},
				})
				if err != nil {
					panic(err)
				}
			}
		}
	}
}

func (app *appType) Close() {
	app.startStopMu.Lock()
	defer app.startStopMu.Unlock()
	if !app.started {
		fmt.Println("NOT STARTED")
		return
	}
	app.started = false

	if app.httpRPCServer != nil {
		err := app.httpRPCServer.Close()
		if err != nil {
			fmt.Println("error closing HTTP RPC server:", err)
		}
	}

	app.blobStore.Close()
	app.blobStore = nil

	app.txStore.Close()
	app.txStore = nil

	app.host.Close()
	app.host = nil

	app.db.Close()
	app.db = nil
}

func (app *appType) initializeLocalState() {
	f, err := pkger.Open("/sync9.js")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	_, sync9Sha3, err := app.host.AddRef(f)
	if err != nil {
		panic(err)
	}

	type M = map[string]interface{}

	app.ensureState("chat.local/servers", "value", M{
		"Merge-Type": M{
			"Content-Type": "resolver/js",
			"value": M{
				"src": M{
					"Content-Type": "link",
					"value":        "ref:sha3:" + sync9Sha3.Hex(),
				},
			},
		},
		"Validator": M{
			"Content-Type": "validator/permissions",
			"value": M{
				"*": M{
					"^.*$": M{
						"write": true,
					},
				},
			},
		},
		"value": M{},
	})

	app.ensureState("chat.local/dms", "rooms", M{
		"Merge-Type": M{
			"Content-Type": "resolver/js",
			"value": M{
				"src": M{
					"Content-Type": "link",
					"value":        "ref:sha3:" + sync9Sha3.Hex(),
				},
			},
		},
		"Validator": M{
			"Content-Type": "validator/permissions",
			"value": M{
				"*": M{
					"^.*$": M{
						"write": true,
					},
				},
			},
		},
		"rooms": M{},
	})

	app.ensureState("chat.local/address-book", "value", M{
		"Merge-Type": M{
			"Content-Type": "resolver/js",
			"value": M{
				"src": M{
					"Content-Type": "link",
					"value":        "ref:sha3:" + sync9Sha3.Hex(),
				},
			},
		},
		"Validator": M{
			"Content-Type": "validator/permissions",
			"value": M{
				"*": M{
					"^.*$": M{
						"write": true,
					},
				},
			},
		},
		"value": M{},
	})
}

func (app *appType) ensureState(stateURI string, checkKeypath string, value interface{}) {
	node, err := app.host.StateAtVersion(stateURI, nil)
	if err != nil && errors.Cause(err) != tree.ErrNoController {
		panic(err)
	} else if err == nil {
		defer node.Close()

		exists, err := node.Exists(state.Keypath(checkKeypath))
		if err != nil {
			panic(err)
		}
		if exists {
			return
		}
		node.Close()
	}

	err = app.host.SendTx(context.Background(), tree.Tx{
		StateURI: stateURI,
		ID:       tree.GenesisTxID,
		Patches:  []tree.Patch{{Val: value}},
	})
	if err != nil {
		panic(err)
	}
}

func (a *appType) ensureDataDirs(config *config.Config) error {
	err := os.MkdirAll(config.RefDataRoot(), 0777|os.ModeDir)
	if err != nil {
		return err
	}

	err = os.MkdirAll(config.TxDBRoot(), 0777|os.ModeDir)
	if err != nil {
		return err
	}

	err = os.MkdirAll(config.StateDBRoot(), 0777|os.ModeDir)
	if err != nil {
		return err
	}
	return nil
}
