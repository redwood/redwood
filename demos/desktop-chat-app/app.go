package main

import (
	"bufio"
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

	"redwood.dev"
	"redwood.dev/ctx"
	"redwood.dev/identity"
	"redwood.dev/tree"
)

var app = &appType{
	Logger:  ctx.NewLogger("app"),
	devMode: true,
}

type appType struct {
	startStopMu   sync.Mutex
	started       bool
	refStore      redwood.RefStore
	txStore       redwood.TxStore
	host          redwood.Host
	db            *tree.DBTree
	httpRPCServer *http.Server
	chLoggedOut   chan struct{}

	keyStore    identity.KeyStore
	password    string
	profileRoot string
	profileName string
	mnemonic    string

	// These are set once on startup and never change
	ctx.Logger
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

	config, err := redwood.ReadConfigAtPath("redwood-webview", app.configPath)
	if os.IsNotExist(err) {
		err := os.MkdirAll(filepath.Dir(app.configPath), 0777|os.ModeDir)
		if err != nil {
			return err
		}
		config, err = redwood.ReadConfigAtPath("redwood-webview", app.configPath)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// Ignore the config file's data root and use our profileRoot + profileName
	config.Node.DataRoot = filepath.Join(app.profileRoot, app.profileName)

	if app.devMode {
		config.Node.DevMode = true
	}

	err = app.ensureDataDirs(config)
	if err != nil {
		return err
	}

	db, err := tree.NewDBTree(filepath.Join(config.Node.DataRoot, "peers"))
	if err != nil {
		return err
	}
	app.db = db

	var (
		txStore       = redwood.NewBadgerTxStore(config.TxDBRoot())
		keyStore      = identity.NewBadgerKeyStore(db, identity.DefaultScryptParams)
		refStore      = redwood.NewRefStore(config.RefDataRoot())
		peerStore     = redwood.NewPeerStore(db)
		controllerHub = redwood.NewControllerHub(config.StateDBRoot(), txStore, refStore)
	)
	app.keyStore = keyStore

	err = refStore.Start()
	if err != nil {
		return err
	}
	app.refStore = refStore

	err = txStore.Start()
	if err != nil {
		return err
	}
	app.txStore = txStore

	var transports []redwood.Transport

	if config.P2PTransport.Enabled {
		var bootstrapPeers []string
		for _, bp := range config.Node.BootstrapPeers {
			if bp.Transport != "libp2p" {
				continue
			}
			bootstrapPeers = append(bootstrapPeers, bp.DialAddresses...)
		}

		libp2pTransport := redwood.NewLibp2pTransport(
			config.P2PTransport.ListenPort,
			config.P2PTransport.ReachableAt,
			config.P2PTransport.KeyFile,
			bootstrapPeers,
			controllerHub,
			keyStore,
			refStore,
			peerStore,
		)
		if err != nil {
			return err
		}
		transports = append(transports, libp2pTransport)
	}

	if config.HTTPTransport.Enabled {
		tlsCertFilename := filepath.Join(config.Node.DataRoot, "..", "server.crt")
		tlsKeyFilename := filepath.Join(config.Node.DataRoot, "..", "server.key")

		// var cookieSecret [32]byte
		// copy(cookieSecret[:], []byte(config.HTTPTransport.CookieSecret))

		httpTransport, err := redwood.NewHTTPTransport(
			config.HTTPTransport.ListenHost,
			config.HTTPTransport.ReachableAt,
			config.HTTPTransport.DefaultStateURI,
			controllerHub,
			keyStore,
			refStore,
			peerStore,
			// cookieSecret,
			tlsCertFilename,
			tlsKeyFilename,
			config.Node.DevMode,
		)
		if err != nil {
			return err
		}
		transports = append(transports, httpTransport)
	}

	err = app.keyStore.Unlock(app.password, app.mnemonic)
	if err != nil {
		app.refStore.Close()
		app.refStore = nil

		app.txStore.Close()
		app.txStore = nil

		app.db.Close()
		app.db = nil
		return err
	}

	app.host, err = redwood.NewHost(transports, controllerHub, keyStore, refStore, peerStore, config)
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

	if config.HTTPRPC.Enabled {
		rwRPC := redwood.NewHTTPRPCServer(app.host)
		rpc := &HTTPRPCServer{rwRPC, keyStore}
		app.httpRPCServer, err = redwood.StartHTTPRPC(rpc, config.HTTPRPC)
		if err != nil {
			return err
		}
		app.Infof(0, "http rpc server listening on %v", config.HTTPRPC.ListenHost)
	}

	for _, bootstrapPeer := range config.Node.BootstrapPeers {
		bootstrapPeer := bootstrapPeer
		go func() {
			app.Infof(0, "connecting to bootstrap peer: %v %v", bootstrapPeer.Transport, bootstrapPeer.DialAddresses)
			for _, dialAddr := range bootstrapPeer.DialAddresses {
				app.host.AddPeer(redwood.PeerDialInfo{TransportName: bootstrapPeer.Transport, DialAddr: dialAddr})
			}
		}()
	}

	// go func() {
	// time.Sleep(5 * time.Second)
	for stateURI := range config.Node.SubscribedStateURIs {
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
	// }()

	klog.Info(redwood.PrettyJSON(config))
	klog.Flush()

	app.initializeLocalState()
	go app.monitorForDMs()
	go app.inputLoop()

	return nil
}

func (app *appType) monitorForDMs() {
	// time.Sleep(5 * time.Second)

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
			roomKeypath := tree.Keypath("rooms").Pushs(roomName)
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
				err := app.host.SendTx(context.TODO(), redwood.Tx{
					StateURI: "chat.local/dms",
					Patches: []redwood.Patch{{
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

	app.refStore.Close()
	app.refStore = nil

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
	state, err := app.host.StateAtVersion(stateURI, nil)
	if err != nil && errors.Cause(err) != redwood.ErrNoController {
		panic(err)
	} else if err == nil {
		defer state.Close()

		exists, err := state.Exists(tree.Keypath(checkKeypath))
		if err != nil {
			panic(err)
		}
		if exists {
			return
		}
		state.Close()
	}

	err = app.host.SendTx(context.Background(), redwood.Tx{
		StateURI: stateURI,
		ID:       redwood.GenesisTxID,
		Patches:  []redwood.Patch{{Val: value}},
	})
	if err != nil {
		panic(err)
	}
}

func (a *appType) ensureDataDirs(config *redwood.Config) error {
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

func (app *appType) inputLoop() {
	fmt.Println("Type \"help\" for a list of commands.")
	fmt.Println()

	var longestCommandLength int
	for cmd := range replCommands {
		if len(cmd) > longestCommandLength {
			longestCommandLength = len(cmd)
		}
	}

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("> ")

		if !scanner.Scan() {
			break
		}

		line := scanner.Text()
		parts := strings.Split(line, " ")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}

		if len(parts) < 1 {
			app.Error("enter a command")
			continue
		} else if parts[0] == "help" {
			fmt.Println("___ Commands _________")
			fmt.Println()
			for cmd, info := range replCommands {
				difference := longestCommandLength - len(cmd)
				space := strings.Repeat(" ", difference+4)
				fmt.Printf("%v%v- %v\n", cmd, space, info.HelpText)
			}
			continue
		}

		cmd, exists := replCommands[parts[0]]
		if !exists {
			app.Error("unknown command")
			continue
		}

		err := cmd.Handler(parts[1:], app.host)
		if err != nil {
			app.Error(err)
		}
	}
}

var replCommands = map[string]struct {
	HelpText string
	Handler  func(args []string, host redwood.Host) error
}{
	"stateuris": {
		"list all known state URIs",
		func(args []string, host redwood.Host) error {
			stateURIs, err := host.Controllers().KnownStateURIs()
			if err != nil {
				return err
			}
			if len(stateURIs) == 0 {
				fmt.Println("no known state URIs")
			} else {
				for _, stateURI := range stateURIs {
					fmt.Println("- ", stateURI)
				}
			}
			return nil
		},
	},
	"state": {
		"print the current state tree",
		func(args []string, host redwood.Host) error {
			if len(args) < 1 {
				return errors.New("missing argument: state URI")
			}

			stateURI := args[0]
			state, err := host.Controllers().StateAtVersion(stateURI, nil)
			if err != nil {
				return err
			}
			var keypath tree.Keypath
			var rng *tree.Range
			if len(args) > 1 {
				_, keypath, rng, err = redwood.ParsePatchPath([]byte(args[1]))
				if err != nil {
					return err
				}
			}
			app.Debugf("stateURI: %v / keypath: %v / range: %v", stateURI, keypath, rng)
			state = state.NodeAt(keypath, rng)
			state.DebugPrint(app.Debugf, false, 0)
			fmt.Println(redwood.PrettyJSON(state))
			return nil
		},
	},
	"peers": {
		"list all known peers",
		func(args []string, host redwood.Host) error {
			for _, peer := range host.Peers() {
				fmt.Println("- ", peer.Addresses(), peer.DialInfo(), peer.LastContact())
			}
			return nil
		},
	},
	"addpeer": {
		"list all known peers",
		func(args []string, host redwood.Host) error {
			if len(args) < 2 {
				return errors.New("requires two arguments: addpeer <transport> <dial addr>")
			}
			host.AddPeer(redwood.PeerDialInfo{args[0], args[1]})
			return nil
		},
	},
}
