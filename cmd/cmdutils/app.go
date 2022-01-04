package cmdutils

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/brynbellomy/klog"

	"redwood.dev/blob"
	"redwood.dev/errors"
	"redwood.dev/health"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/rpc"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/swarm/braidhttp"
	"redwood.dev/swarm/libp2p"
	"redwood.dev/swarm/protoauth"
	"redwood.dev/swarm/protoblob"
	"redwood.dev/swarm/protohush"
	"redwood.dev/swarm/prototree"
	"redwood.dev/tree"
	"redwood.dev/types"
	"redwood.dev/utils"
	"redwood.dev/utils/badgerutils"
)

type App struct {
	process.Process
	log.Logger

	Config Config
	TermUI *termUI

	ControllerHub       tree.ControllerHub
	TxStore             tree.TxStore
	KeyStore            identity.KeyStore
	PeerStore           swarm.PeerStore
	BlobStore           blob.Store
	AuthProto           protoauth.AuthProtocol
	BlobProto           protoblob.BlobProtocol
	HushProto           protohush.HushProtocol
	HushProtoStore      protohush.Store
	TreeProto           prototree.TreeProtocol
	TreeProtoStore      prototree.Store
	HTTPTransport       braidhttp.Transport
	Libp2pTransport     libp2p.Transport
	HTTPRPCServer       *http.Server
	HTTPRPCServerConfig rpc.HTTPConfig
	SharedStateDB       *state.DBTree

	Nurse *health.Nurse
}

func NewApp(name string, config Config) *App {
	if name == "" {
		name = "redwood"
	}
	return &App{
		Config:  config,
		Process: *process.New(name),
		Logger:  log.NewLogger(name),
	}
}

func (app *App) Start() error {
	err := app.Process.Start()
	if err != nil {
		return err
	}

	cfg := app.Config

	if cfg.Nurse.Enabled {
		app.Nurse = health.NewNurse(health.NurseConfig{
			ProfileRoot:          cfg.Nurse.ProfileRoot,
			PollInterval:         cfg.Nurse.PollInterval,
			GatherDuration:       cfg.Nurse.GatherDuration,
			MaxProfileSize:       cfg.Nurse.MaxProfileSize,
			CPUProfileRate:       cfg.Nurse.CPUProfileRate,
			MemProfileRate:       cfg.Nurse.MemProfileRate,
			BlockProfileRate:     cfg.Nurse.BlockProfileRate,
			MutexProfileFraction: cfg.Nurse.MutexProfileFraction,
			MemThreshold:         cfg.Nurse.MemThreshold,
			GoroutineThreshold:   cfg.Nurse.GoroutineThreshold,
		})
		err = app.Process.SpawnChild(nil, app.Nurse)
		if err != nil {
			return err
		}
	}

	if cfg.Mode == ModeTermUI {
		app.TermUI = NewTermUI()
		app.TermUI.Start()

		flagset := flag.NewFlagSet("", flag.ContinueOnError)
		klog.InitFlags(flagset)
		flagset.Set("v", "2")
		flagset.Set("log_file", "/tmp/asdf") // This is necessary to keep the logger in "single mode" -- otherwise logs will be duplicated
		klog.SetOutput(app.TermUI.LogPane)
		klog.SetFormatter(&FmtConstWidth{
			FileNameCharWidth: 24,
			UseColor:          true,
		})

	} else {
		flagset := flag.NewFlagSet("", flag.ContinueOnError)
		klog.InitFlags(flagset)
		flagset.Set("logtostderr", "true")
		flagset.Set("v", "2")
		klog.SetFormatter(&klog.FmtConstWidth{
			FileNameCharWidth: 24,
			UseColor:          true,
		})
	}

	err = app.EnsureDataDirs(cfg)
	if err != nil {
		return err
	}

	var badgerOpts badgerutils.OptsBuilder

	// Initialize the KeyStore
	{
		scryptParams := identity.DefaultScryptParams
		if cfg.KeyStore.InsecureScryptParams {
			scryptParams = identity.InsecureScryptParams
		}
		app.KeyStore = identity.NewBadgerKeyStore(badgerOpts.ForPath(cfg.KeyStoreRoot()), scryptParams)
		err = app.KeyStore.Unlock(cfg.KeyStore.Password, cfg.KeyStore.Mnemonic)
		if err != nil {
			app.Errorf("while unlocking keystore: %+v", err)
			return err
		}
		defer closeIfError(&err, app.KeyStore)

		if len(cfg.Libp2pTransport.Key) > 0 {
			keyBytes, err := base64.StdEncoding.DecodeString(cfg.Libp2pTransport.Key)
			if err != nil {
				return err
			}

			if !libp2p.IsValidKey(keyBytes) {
				app.Errorf("invalid libp2p key")
				return errors.New("invalid libp2p key")
			}

			err = app.KeyStore.SaveExtraUserData("libp2p:p2pkey", hex.EncodeToString(keyBytes))
			if err != nil {
				app.Errorf("while saving libp2p key: %+v", err)
				return err
			}
		}
	}

	// All DBs other than the keystore are encrypted at rest
	badgerOpts = badgerOpts.WithEncryption(app.KeyStore.LocalSymEncKey().Bytes(), 24*time.Hour)

	// Initialize the config DB shared by various packages
	db, err := state.NewDBTree(badgerOpts.ForPath(filepath.Join(cfg.DataRoot, "shared")))
	if err != nil {
		app.Errorf("while opening shared db: %+v", err)
		return err
	}
	defer closeIfError(&err, db)
	app.SharedStateDB = db

	app.PeerStore = swarm.NewPeerStore(app.SharedStateDB)

	app.BlobStore = blob.NewBadgerStore(badgerOpts.ForPath(cfg.BlobDataRoot()))
	err = app.BlobStore.Start()
	if err != nil {
		app.Errorf("while opening blob store: %+v", err)
		return err
	}

	// Initialize the tree protocol
	if cfg.TreeProtocol.Enabled {
		app.TxStore = tree.NewBadgerTxStore(badgerOpts.ForPath(cfg.TxDBRoot()))

		err = app.TxStore.Start()
		if err != nil {
			app.Errorf("while opening tx store: %+v", err)
			return err
		}

		app.ControllerHub = tree.NewControllerHub(cfg.StateDBRoot(), app.TxStore, app.BlobStore, badgerOpts)
		err = app.Process.SpawnChild(context.TODO(), app.ControllerHub)
		if err != nil {
			app.Errorf("while starting controller hub: %+v", err)
			return err
		}
	}

	// Make self-signed TLS certs as a backup
	tlsCert, err := utils.MakeSelfSignedX509Certificate()
	if err != nil {
		return err
	}
	tlsCerts := []tls.Certificate{*tlsCert}

	var transports []swarm.Transport
	{
		if cfg.Libp2pTransport.Enabled {
			var bootstrapPeers []string
			for _, bp := range cfg.BootstrapPeers {
				if bp.Transport != "libp2p" {
					continue
				}
				bootstrapPeers = append(bootstrapPeers, bp.DialAddresses...)
			}

			libp2pTransport, err := libp2p.NewTransport(
				cfg.Libp2pTransport.ListenPort,
				cfg.Libp2pTransport.ReachableAt,
				bootstrapPeers,
				cfg.Libp2pTransport.StaticRelays,
				filepath.Join(cfg.DataRoot, libp2p.TransportName),
				cfg.DNSOverHTTPSURL,
				app.ControllerHub,
				app.KeyStore,
				app.BlobStore,
				app.PeerStore,
			)
			if err != nil {
				app.Errorf("while creating libp2p transport: %+v", err)
				return err
			}
			defer closeIfError(&err, libp2pTransport)

			app.Libp2pTransport = libp2pTransport
			transports = append(transports, app.Libp2pTransport)
		}

		if cfg.BraidHTTPTransport.Enabled {
			tlsCertFilename := filepath.Join(cfg.BraidHTTPTransport.TLSCertFile)
			tlsKeyFilename := filepath.Join(cfg.BraidHTTPTransport.TLSKeyFile)

			httpTransport, err := braidhttp.NewTransport(
				cfg.BraidHTTPTransport.ListenHost,
				cfg.BraidHTTPTransport.ListenHostSSL,
				types.NewStringSet(nil), // @@TODO
				cfg.BraidHTTPTransport.DefaultStateURI,
				app.ControllerHub,
				app.KeyStore,
				app.BlobStore,
				app.PeerStore,
				tlsCertFilename,
				tlsKeyFilename,
				tlsCerts,
				[]byte(cfg.JWTSecret),
				cfg.DevMode,
			)
			if err != nil {
				app.Errorf("while creating braid-http transport: %+v", err)
				return err
			}
			defer closeIfError(&err, httpTransport)

			app.HTTPTransport = httpTransport
			transports = append(transports, app.HTTPTransport)
		}
	}

	var protocols []process.Interface

	if cfg.AuthProtocol.Enabled {
		app.AuthProto = protoauth.NewAuthProtocol(transports, app.KeyStore, app.PeerStore)
		protocols = append(protocols, app.AuthProto)
	}

	if cfg.BlobProtocol.Enabled {
		app.BlobProto = protoblob.NewBlobProtocol(transports, app.BlobStore)
		protocols = append(protocols, app.BlobProto)
	}

	if cfg.HushProtocol.Enabled {
		app.HushProtoStore = protohush.NewStore(app.SharedStateDB)
		app.HushProto = protohush.NewHushProtocol(transports, app.HushProtoStore, app.KeyStore, app.PeerStore)
		protocols = append(protocols, app.HushProto)
	}

	if cfg.TreeProtocol.Enabled {
		app.TreeProtoStore, err = prototree.NewStore(app.SharedStateDB)
		if err != nil {
			app.Errorf("while opening prototree store: %+v", err)
			return err
		}

		err = app.TreeProtoStore.SetMaxPeersPerSubscription(cfg.TreeProtocol.MaxPeersPerSubscription)
		if err != nil {
			app.Errorf("while setting max peers per subscription: %+v", err)
			return err
		}

		app.TreeProto = prototree.NewTreeProtocol(
			transports,
			app.HushProto,
			app.ControllerHub,
			app.TxStore,
			app.KeyStore,
			app.PeerStore,
			app.TreeProtoStore,
		)
		protocols = append(protocols, app.TreeProto)
	}

	for _, transport := range transports {
		app.Infof(0, "starting %v", transport.Name())
		err = app.Process.SpawnChild(nil, transport)
		if err != nil {
			app.Errorf("while starting %v transport: %+v", transport.Name(), err)
			return err
		}
	}

	for _, protocol := range protocols {
		app.Infof(0, "starting %v", protocol.Name())
		err = app.Process.SpawnChild(nil, protocol)
		if err != nil {
			return err
		}
	}

	if cfg.HTTPRPC.Enabled {
		rwRPC := rpc.NewHTTPServer([]byte(cfg.JWTSecret), app.AuthProto, app.BlobProto, app.TreeProto, app.PeerStore, app.KeyStore, app.BlobStore, app.ControllerHub)
		var server interface{}
		if cfg.HTTPRPC.Server != nil {
			server = cfg.HTTPRPC.Server(rwRPC)
		} else {
			server = rwRPC
		}
		app.HTTPRPCServer, err = rpc.StartHTTPRPC(server, cfg.HTTPRPC, []byte(cfg.JWTSecret))
		if err != nil {
			return err
		}
		app.Infof(0, "http rpc server listening on %v", cfg.HTTPRPC.ListenHost)
	}

	for _, bootstrapPeer := range cfg.BootstrapPeers {
		bootstrapPeer := bootstrapPeer
		_ = app.Process.Go(nil, "", func(ctx context.Context) {
			app.Infof(0, "adding bootstrap peer %v %v", bootstrapPeer.Transport, bootstrapPeer.DialAddresses)
			for _, dialAddr := range bootstrapPeer.DialAddresses {
				app.PeerStore.AddDialInfo(swarm.PeerDialInfo{TransportName: bootstrapPeer.Transport, DialAddr: dialAddr}, "")
			}
		})
		if err != nil {
			return err
		}
	}

	klog.Info(utils.PrettyJSON(cfg))
	klog.Flush()

	switch cfg.Mode {
	case ModeREPL:
		go func() {
			prompt := ">"
			if cfg.REPLConfig.Prompt != "" {
				prompt = cfg.REPLConfig.Prompt
			}
			app.startREPL(prompt, cfg.REPLConfig.Commands)
			<-AwaitInterrupt()
			app.Process.Close()
		}()

		app.Process.Go(nil, "repl (await termination)", func(ctx context.Context) {
			<-ctx.Done()
			err := os.Stdin.Close()
			if err != nil {
				panic(err)
			}
		})

	case ModeTermUI:
		app.Process.Go(nil, "termui", func(ctx context.Context) {
			for {
				select {
				case <-time.After(3 * time.Second):
					// @@TODO: use stateURI subscriptions for this
					stateURIs, err := app.ControllerHub.KnownStateURIs()
					if err != nil {
						continue
					}

					app.TermUI.Sidebar.SetStateURIs(stateURIs)
					states := make(map[string]string)
					for _, stateURI := range stateURIs {
						node, err := app.ControllerHub.StateAtVersion(stateURI, nil)
						if err != nil {
							panic(err)
						}
						states[stateURI] = utils.PrettyJSON(node)
					}
					app.TermUI.StatePane.SetStates(states)
				case <-app.TermUI.Done():
					return
				}
			}
		})

	case ModeHeadless:
		<-AwaitInterrupt()
	}

	return nil
}

func closeIfError(err *error, x interface{}) {
	type closer interface {
		Close()
	}
	type closerWithError interface {
		Close() error
	}
	if *err != nil {
		switch x := x.(type) {
		case closer:
			x.Close()
		case closerWithError:
			x.Close()
		}
	}
}

func (app *App) Close() error {
	app.Infof(0, "shutting down")

	app.Infof(0, "killing rpc")
	if app.HTTPRPCServer != nil {
		err := app.HTTPRPCServer.Close()
		if err != nil {
			fmt.Println("error closing HTTP RPC server:", err)
		}
	}

	app.Infof(0, "killing keystore")
	if app.KeyStore != nil {
		app.KeyStore.Close()
		app.KeyStore = nil
	}

	app.Infof(0, "killing blobstore")
	if app.BlobStore != nil {
		app.BlobStore.Close()
		app.BlobStore = nil
	}

	app.Infof(0, "killing txstore")
	if app.TxStore != nil {
		app.TxStore.Close()
		app.TxStore = nil
	}

	app.Infof(0, "killing shared state db")
	if app.SharedStateDB != nil {
		app.SharedStateDB.Close()
		app.SharedStateDB = nil
	}

	return app.Process.Close()
}

func (app *App) EnsureInitialState(stateURI string, checkKeypath string, value interface{}) {
	node, err := app.ControllerHub.StateAtVersion(stateURI, nil)
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

	valueBytes, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}

	err = app.TreeProto.SendTx(context.Background(), tree.Tx{
		StateURI: stateURI,
		ID:       tree.GenesisTxID,
		Patches:  []tree.Patch{{ValueJSON: valueBytes}},
	})
	if err != nil {
		panic(err)
	}
}

func (a *App) EnsureDataDirs(config Config) error {
	err := os.MkdirAll(config.BlobDataRoot(), 0777|os.ModeDir)
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

func (app *App) startREPL(prompt string, replCommands REPLCommands) {
	fmt.Println("Type \"help\" for a list of commands.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf(prompt)

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
			fmt.Println(replCommands.Help())
			fmt.Println()
			continue
		}

		err := replCommands.Handle(nil, parts, app)
		if err != nil {
			app.Errorf("%+v", err)
		}
	}
}

func AwaitInterrupt() <-chan struct{} {
	chDone := make(chan struct{})

	go func() {
		sigInbox := make(chan os.Signal, 1)

		signal.Notify(sigInbox, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

		count := 0
		firstTime := int64(0)

		for range sigInbox {
			count++
			curTime := time.Now().Unix()

			// Prevent un-terminated ^c character in terminal
			fmt.Println()

			if count == 1 {
				firstTime = curTime
				close(chDone)

			} else {
				if curTime > firstTime+3 {
					fmt.Println("\nReceived interrupt before graceful shutdown, terminating...")
					klog.Flush()
					os.Exit(-1)
				}
			}
		}
	}()

	return chDone
}
