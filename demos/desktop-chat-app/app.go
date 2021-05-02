package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brynbellomy/klog"
	"github.com/markbates/pkger"
	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"

	"redwood.dev/blob"
	"redwood.dev/config"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/rpc"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/swarm/braidhttp"
	"redwood.dev/swarm/libp2p"
	"redwood.dev/tree"
	"redwood.dev/types"
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
	go app.inputLoop()

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
	Handler  func(args []string, host swarm.Host) error
}{
	"libp2pid": {
		"list your libp2p peer ID",
		func(args []string, host swarm.Host) error {
			peerID := host.Transport("libp2p").(interface{ Libp2pPeerID() string }).Libp2pPeerID()
			host.Debugf("libp2p peer ID: %v", peerID)
			return nil
		},
	},
	"subscribe": {
		"subscribe",
		func(args []string, host swarm.Host) error {
			if len(args) < 1 {
				return errors.New("missing argument: state URI")
			}
			stateURI := args[0]
			sub, err := host.Subscribe(context.Background(), stateURI, swarm.SubscriptionType_Txs, nil, &swarm.FetchHistoryOpts{FromTxID: tree.GenesisTxID})
			if err != nil {
				return err
			}
			defer sub.Close()
			return nil
		},
	},
	"stateuris": {
		"list all known state URIs",
		func(args []string, host swarm.Host) error {
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
		func(args []string, host swarm.Host) error {
			if len(args) < 1 {
				return errors.New("missing argument: state URI")
			}

			stateURI := args[0]
			node, err := host.Controllers().StateAtVersion(stateURI, nil)
			if err != nil {
				return err
			}
			var keypath state.Keypath
			var rng *state.Range
			if len(args) > 1 {
				_, keypath, rng, err = tree.ParsePatchPath([]byte(args[1]))
				if err != nil {
					return err
				}
			}
			app.Debugf("stateURI: %v / keypath: %v / range: %v", stateURI, keypath, rng)
			node = node.NodeAt(keypath, rng)
			node.DebugPrint(app.Debugf, false, 0)
			fmt.Println(utils.PrettyJSON(node))
			return nil
		},
	},
	"blobs": {
		"list all blobs",
		func(args []string, host swarm.Host) error {
			blobIDsNeeded, err := host.BlobStore().RefsNeeded()
			if err != nil {
				return err
			}

			blobIDs, err := host.BlobStore().AllHashes()
			if err != nil {
				return err
			}

			var rows [][]string

			for _, id := range blobIDsNeeded {
				rows = append(rows, []string{id.String(), "(missing)"})
			}

			for _, id := range blobIDs {
				rows = append(rows, []string{id.String(), ""})
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
			table.SetCenterSeparator("|")
			table.SetRowLine(true)
			table.SetHeader([]string{"ID", "Status"})
			table.AppendBulk(rows)
			table.Render()

			return nil
		},
	},
	"peers": {
		"list all known peers",
		func(args []string, host swarm.Host) error {
			fmtPeerRow := func(addr, dialAddr string, lastContact, lastFailure time.Time, failures uint64, remainingBackoff time.Duration, stateURIs []string) []string {
				if len(addr) > 10 {
					addr = addr[:4] + "..." + addr[len(addr)-4:]
				}
				if len(dialAddr) > 30 {
					dialAddr = dialAddr[:30] + "..." + dialAddr[len(dialAddr)-6:]
				}
				lastContactStr := time.Now().Sub(lastContact).Round(1 * time.Second).String()
				if lastContact.IsZero() {
					lastContactStr = ""
				}
				lastFailureStr := time.Now().Sub(lastFailure).Round(1 * time.Second).String()
				if lastFailure.IsZero() {
					lastFailureStr = ""
				}
				failuresStr := fmt.Sprintf("%v", failures)
				remainingBackoffStr := remainingBackoff.Round(1 * time.Second).String()
				if remainingBackoff == 0 {
					remainingBackoffStr = ""
				}
				return []string{addr, dialAddr, lastContactStr, lastFailureStr, failuresStr, remainingBackoffStr, fmt.Sprintf("%v", stateURIs)}
			}

			var data [][]string
			for _, peer := range host.Peers() {
				for _, addr := range peer.Addresses() {
					data = append(data, fmtPeerRow(addr.Hex(), peer.DialInfo().DialAddr, peer.LastContact(), peer.LastFailure(), peer.Failures(), peer.RemainingBackoff(), peer.StateURIs().Slice()))
				}
				if len(peer.Addresses()) == 0 {
					data = append(data, fmtPeerRow("?", peer.DialInfo().DialAddr, peer.LastContact(), peer.LastFailure(), peer.Failures(), peer.RemainingBackoff(), peer.StateURIs().Slice()))
				}
			}

			sort.Slice(data, func(i, j int) bool {
				cmp := strings.Compare(data[i][0], data[j][0])
				if cmp == 0 {
					return strings.Compare(data[i][1], data[j][1]) < 0
				} else {
					return cmp < 0
				}
			})

			table := tablewriter.NewWriter(os.Stdout)
			table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
			table.SetCenterSeparator("|")
			table.SetRowLine(true)
			table.SetHeader([]string{"Address", "DialAddr", "LastContact", "LastFailure", "Failures", "Backoff", "StateURIs"})
			table.SetAutoMergeCellsByColumnIndex([]int{0, 1})
			table.SetColumnColor(
				tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
				tablewriter.Colors{},
				tablewriter.Colors{},
				tablewriter.Colors{},
				tablewriter.Colors{},
				tablewriter.Colors{},
				tablewriter.Colors{},
			)
			table.AppendBulk(data)
			table.Render()

			return nil
		},
	},
	"addpeer": {
		"add a peer",
		func(args []string, host swarm.Host) error {
			if len(args) < 2 {
				return errors.New("requires 2 arguments: addpeer <transport> <dial addr>")
			}
			host.AddPeer(swarm.PeerDialInfo{args[0], args[1]})
			return nil
		},
	},
	"rmallpeers": {
		"remove all peers",
		func(args []string, host swarm.Host) error {
			var toDelete []swarm.PeerDialInfo
			for _, peer := range host.Peers() {
				toDelete = append(toDelete, peer.DialInfo())
			}
			host.RemovePeers(toDelete)
			return nil
		},
	},
	"rmunverifiedpeers": {
		"remove peers who haven't been verified",
		func(args []string, host swarm.Host) error {
			var toDelete []swarm.PeerDialInfo
			for _, peer := range host.Peers() {
				if len(peer.Addresses()) == 0 {
					toDelete = append(toDelete, peer.DialInfo())
				}
			}
			host.RemovePeers(toDelete)
			return nil
		},
	},
	"rmfailedpeers": {
		"remove peers with more than a certain number of failures",
		func(args []string, host swarm.Host) error {
			if len(args) < 1 {
				return errors.New("requires 1 argument: rmfailedpeers <number of failures>")
			}

			num, err := strconv.Atoi(args[0])
			if err != nil {
				return errors.Wrap(err, "bad argument")
			}

			var toDelete []swarm.PeerDialInfo
			for _, peer := range host.Peers() {
				if peer.Failures() > uint64(num) {
					toDelete = append(toDelete, peer.DialInfo())
				}
			}
			host.RemovePeers(toDelete)
			return nil
		},
	},
	"set": {
		"set a keypath in a state tree",
		func(args []string, host swarm.Host) error {
			if len(args) < 3 {
				return errors.New("requires 3 arguments: set <state URI> <keypath> <JSON value>")
			}
			stateURI := args[0]
			keypath := state.Keypath(args[1])
			jsonVal := strings.Join(args[2:], " ")
			var val interface{}
			err := json.Unmarshal([]byte(jsonVal), &val)
			if err != nil {
				return err
			}
			err = host.SendTx(context.TODO(), tree.Tx{
				ID:       types.RandomID(),
				StateURI: stateURI,
				Patches: []tree.Patch{{
					Keypath: keypath,
					Val:     val,
				}},
			})
			return nil
		},
	},
}
