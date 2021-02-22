package main

import (
	"bufio"
	"context"
	"fmt"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brynbellomy/klog"
	"github.com/markbates/pkger"
	"github.com/pkg/errors"
	"github.com/webview/webview"
	"redwood.dev"

	rw "redwood.dev"
	"redwood.dev/crypto"
	"redwood.dev/ctx"
	"redwood.dev/tree"
)

var app = &appType{
	Logger:     ctx.NewLogger("app"),
	configPath: "./node2.redwoodrc",
	devMode:    true,
}

type appType struct {
	ctx.Logger
	refStore      rw.RefStore
	txStore       rw.TxStore
	host          rw.Host
	peerDB        *tree.DBTree
	httpRPCServer *HTTPRPCServer
	chLoggedOut   chan struct{}

	// These are set once on startup and never change
	configPath string
	devMode    bool
}

func (app *appType) Start() error {
	app.chLoggedOut = make(chan struct{})

	config, err := rw.ReadConfigAtPath("redwood-webview", app.configPath)
	if os.IsNotExist(err) {
		err := os.MkdirAll(filepath.Dir(app.configPath), 0777|os.ModeDir)
		if err != nil {
			return err
		}
		config, err = rw.ReadConfigAtPath("redwood-webview", app.configPath)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	if app.devMode {
		config.Node.DevMode = true
	}

	err = ensureDataDirs(config)
	if err != nil {
		return err
	}

	signingKeypair, err := crypto.SigningKeypairFromHDMnemonic(config.Node.HDMnemonicPhrase, crypto.DefaultHDDerivationPath)
	if err != nil {
		return err
	}

	encryptingKeypair, err := crypto.GenerateEncryptingKeypair()
	if err != nil {
		return err
	}

	peerDB, err := tree.NewDBTree(filepath.Join(config.Node.DataRoot, "peers"))
	if err != nil {
		return err
	}
	app.peerDB = peerDB

	var (
		txStore       = rw.NewBadgerTxStore(config.TxDBRoot())
		refStore      = rw.NewRefStore(config.RefDataRoot())
		peerStore     = rw.NewPeerStore(peerDB)
		controllerHub = rw.NewControllerHub(config.StateDBRoot(), txStore, refStore)
	)

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

	var transports []rw.Transport

	if config.P2PTransport.Enabled {
		var bootstrapPeers []string
		for _, bp := range config.Node.BootstrapPeers {
			if bp.Transport != "libp2p" {
				continue
			}
			bootstrapPeers = append(bootstrapPeers, bp.DialAddresses...)
		}

		libp2pTransport, err := rw.NewLibp2pTransport(
			signingKeypair.Address(),
			config.P2PTransport.ListenPort,
			config.P2PTransport.ReachableAt,
			config.P2PTransport.KeyFile,
			encryptingKeypair,
			bootstrapPeers,
			controllerHub,
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

		var cookieSecret [32]byte
		copy(cookieSecret[:], []byte(config.HTTPTransport.CookieSecret))

		httpTransport, err := rw.NewHTTPTransport(
			config.HTTPTransport.ListenHost,
			config.HTTPTransport.ReachableAt,
			config.HTTPTransport.DefaultStateURI,
			controllerHub,
			refStore,
			peerStore,
			signingKeypair,
			encryptingKeypair,
			cookieSecret,
			tlsCertFilename,
			tlsKeyFilename,
			config.Node.DevMode,
		)
		if err != nil {
			return err
		}
		transports = append(transports, httpTransport)
	}

	app.host, err = rw.NewHost(signingKeypair, encryptingKeypair, transports, controllerHub, refStore, peerStore, config)
	if err != nil {
		return err
	}

	err = app.host.Start()
	if err != nil {
		panic(err)
	}

	pkger.Walk("/frontend/build", func(path string, info os.FileInfo, err error) error {
		fmt.Printf("IS NIL? app.Logger = %v\n", app.Logger)
		app.Infof(0, "Serving %v", path)
		return nil
	})

	if config.HTTPRPC.Enabled {
		rwRPC := rw.NewHTTPRPCServer(signingKeypair.Address(), app.host)
		app.httpRPCServer = &HTTPRPCServer{rwRPC, signingKeypair}

		err = rw.StartHTTPRPC(app.httpRPCServer, config.HTTPRPC)
		if err != nil {
			return err
		}
	}

	for _, bootstrapPeer := range config.Node.BootstrapPeers {
		bootstrapPeer := bootstrapPeer
		go func() {
			app.Infof(0, "connecting to bootstrap peer: %v %v", bootstrapPeer.Transport, bootstrapPeer.DialAddresses)
			for _, dialAddr := range bootstrapPeer.DialAddresses {
				app.host.AddPeer(rw.PeerDialInfo{TransportName: bootstrapPeer.Transport, DialAddr: dialAddr})
			}
		}()
	}

	go func() {
		time.Sleep(5 * time.Second)
		for stateURI := range config.Node.SubscribedStateURIs {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			sub, err := app.host.Subscribe(ctx, stateURI, 0, nil, nil)
			if err != nil {
				app.Errorf("error subscribing to %v: %v", stateURI, err)
				continue
			}
			sub.Close()
			app.Successf("subscribed to %v", stateURI)
		}
	}()

	klog.Info(rw.PrettyJSON(config))

	go app.initializeLocalState()

	// go startAPI(host, port)
	go app.inputLoop()
	// app.AttachInterruptHandler()
	// app.CtxWait()
	return nil
}

func (a *appType) Close() {
	a.refStore.Close()
	a.refStore = nil

	a.txStore.Close()
	a.txStore = nil

	// a.httpRPC.Close()

	a.host.Close()
	a.host = nil

	a.peerDB.Close()
	a.peerDB = nil
}

func (a *appType) initializeLocalState() {
	// pkger.Include("./sync9.js")
	f, err := pkger.Open("/sync9.js")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	_, _, err = a.host.AddRef(f)
	if err != nil {
		panic(err)
	}

	state, err := a.host.StateAtVersion("chat.local/servers", nil)
	if err != nil && errors.Cause(err) != rw.ErrNoController {
		panic(err)
	} else if err == nil {
		defer state.Close()

		exists, err := state.Exists(tree.Keypath("value"))
		if err != nil {
			panic(err)
		}
		if exists {
			return
		}
		state.Close()
	}

	type M = map[string]interface{}

	err = a.host.SendTx(context.Background(), rw.Tx{
		StateURI: "chat.local/servers",
		ID:       rw.GenesisTxID,
		Patches: []rw.Patch{{
			Val: M{
				"Merge-Type": M{
					"Content-Type": "resolver/dumb",
					"value":        M{},
				},
				"Validator": M{
					"Content-Type": "validator/permissions",
					"value": M{
						a.host.Address().Hex(): M{
							"^.*$": M{
								"write": true,
							},
						},
					},
				},
				"value": []interface{}{},
			},
		}},
	})
	if err != nil {
		panic(err)
	}
}

func startGUI(port uint) {
	debug := true
	w := webview.New(debug)
	defer w.Destroy()
	w.SetTitle("Minimal webview example")
	w.SetSize(800, 600, webview.HintNone)
	w.Navigate(fmt.Sprintf("http://localhost:%v/index.html", port))
	w.Run()
}

func ensureDataDirs(config *rw.Config) error {
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

func (a *appType) inputLoop() {
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
	Handler  func(args []string, host rw.Host) error
}{
	"stateuris": {
		"list all known state URIs",
		func(args []string, host rw.Host) error {
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
		func(args []string, host rw.Host) error {
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
				_, keypath, rng, err = rw.ParsePatchPath([]byte(args[1]))
				if err != nil {
					return err
				}
			}
			app.Debugf("stateURI: %v / keypath: %v / range: %v", stateURI, keypath, rng)
			state = state.NodeAt(keypath, rng)
			state.DebugPrint(app.Debugf, false, 0)
			fmt.Println(rw.PrettyJSON(state))
			return nil
		},
	},
	"peers": {
		"list all known peers",
		func(args []string, host rw.Host) error {
			for _, peer := range host.Peers() {
				fmt.Println("- ", peer.Addresses(), peer.DialInfo(), peer.LastContact())
			}
			return nil
		},
	},
	"addpeer": {
		"list all known peers",
		func(args []string, host rw.Host) error {
			if len(args) < 2 {
				return errors.New("requires two arguments: addpeer <transport> <dial addr>")
			}
			host.AddPeer(redwood.PeerDialInfo{args[0], args[1]})
			return nil
		},
	},
}
