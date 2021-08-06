package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/brynbellomy/klog"
	"github.com/pkg/errors"
	"github.com/urfave/cli"

	"redwood.dev/blob"
	"redwood.dev/config"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/rpc"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/swarm/braidhttp"
	"redwood.dev/swarm/libp2p"
	"redwood.dev/swarm/protoauth"
	"redwood.dev/swarm/protoblob"
	"redwood.dev/swarm/prototree"
	"redwood.dev/tree"
	"redwood.dev/utils"
)

var logger = log.NewLogger("redwood")

func main() {
	go func() {
		http.ListenAndServe(":6060", nil)
	}()
	runtime.SetBlockProfileRate(int(time.Millisecond.Nanoseconds()) * 100)

	defer klog.Flush()

	cliApp := cli.NewApp()
	// cliApp.Version = env.AppVersion

	configRoot, err := config.DefaultConfigRoot("redwood")
	if err != nil {
		logger.Error(err)
		os.Exit(1)
	}

	cliApp.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "c, config",
			Value: filepath.Join(configRoot, ".redwoodrc"),
			Usage: "location of config file",
		},
		cli.StringFlag{
			Name:  "p, password-file",
			Usage: "location of password file",
		},
		cli.BoolFlag{
			Name:  "gui",
			Usage: "enable CLI GUI",
		},
		cli.BoolFlag{
			Name:  "dev",
			Usage: "enable dev mode",
		},
	}

	cliApp.Action = func(c *cli.Context) error {
		passwordFile := c.String("password-file")
		configPath := c.String("config")
		gui := c.Bool("gui")
		dev := c.Bool("dev")
		return run(configPath, passwordFile, gui, dev)
	}

	err = cliApp.Run(os.Args)
	if err != nil {
		fmt.Printf("error: %+v\n", err)
		os.Exit(1)
	}
}

func run(configPath, passwordFile string, gui, dev bool) (err error) {
	defer utils.WithStack(&err)

	if passwordFile == "" {
		return errors.New("must specify --password-file flag")
	}

	var termUI *termUI
	if gui {
		termUI = NewTermUI()
		go termUI.Start()
		defer termUI.Stop()

		flagset := flag.NewFlagSet("", flag.ContinueOnError)
		klog.InitFlags(flagset)
		flagset.Set("v", "2")
		flagset.Set("log_file", "/tmp/asdf") // This is necessary to keep the logger in "single mode" -- otherwise logs will be duplicated
		klog.SetOutput(termUI.LogPane)
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

	klog.Flush()
	defer klog.Flush()

	passwordBytes, err := ioutil.ReadFile(passwordFile)
	if err != nil {
		return err
	}

	config, err := config.ReadConfigAtPath("redwood", configPath)
	if err != nil {
		return err
	}

	if dev {
		config.Node.DevMode = true
	}

	err = ensureDataDirs(config)
	if err != nil {
		return err
	}

	keyStore := identity.NewBadgerKeyStore(config.KeyStoreRoot(), identity.DefaultScryptParams)
	err = keyStore.Unlock(string(passwordBytes), "")
	if err != nil {
		return err
	}

	encryptionConfig := &state.EncryptionConfig{
		Key:                 keyStore.LocalSymEncKey(),
		KeyRotationInterval: 24 * time.Hour, // @@TODO: make configurable
	}

	db, err := state.NewDBTree(filepath.Join(config.Node.DataRoot, "shared"), encryptionConfig)
	if err != nil {
		return err
	}
	defer closeIfError(&err, db)

	var (
		txStore       = tree.NewBadgerTxStore(config.TxDBRoot(), encryptionConfig)
		blobStore     = blob.NewBadgerStore(config.BlobDataRoot(), encryptionConfig)
		peerStore     = swarm.NewPeerStore(db)
		controllerHub = tree.NewControllerHub(config.StateDBRoot(), txStore, blobStore, encryptionConfig)
	)

	err = blobStore.Start()
	if err != nil {
		return err
	}
	defer closeIfError(&err, blobStore)

	err = txStore.Start()
	if err != nil {
		return err
	}
	defer closeIfError(&err, txStore)

	err = controllerHub.Start()
	if err != nil {
		return err
	}
	defer closeIfError(&err, controllerHub)

	var transports []swarm.Transport

	if config.P2PTransport.Enabled {
		var bootstrapPeers []string
		for _, bp := range config.Node.BootstrapPeers {
			if bp.Transport != "libp2p" {
				continue
			}
			bootstrapPeers = append(bootstrapPeers, bp.DialAddresses...)
		}

		libp2pTransport := libp2p.NewTransport(
			config.P2PTransport.ListenPort,
			config.P2PTransport.ReachableAt,
			bootstrapPeers,
			controllerHub,
			keyStore,
			blobStore,
			peerStore,
		)
		transports = append(transports, libp2pTransport)
	}

	if config.HTTPTransport.Enabled {
		tlsCertFilename := filepath.Join(config.Node.DataRoot, "server.crt")
		tlsKeyFilename := filepath.Join(config.Node.DataRoot, "server.key")

		httpTransport, err := braidhttp.NewTransport(
			config.HTTPTransport.ListenHost,
			config.HTTPTransport.ReachableAt,
			config.HTTPTransport.DefaultStateURI,
			controllerHub,
			keyStore,
			blobStore,
			peerStore,
			tlsCertFilename,
			tlsKeyFilename,
			config.Node.DevMode,
		)
		if err != nil {
			return err
		}
		transports = append(transports, httpTransport)
	}

	for _, transport := range transports {
		err := transport.Start()
		if err != nil {
			return err
		}
	}

	authProto := protoauth.NewAuthProtocol(transports, keyStore, peerStore)
	blobProto := protoblob.NewBlobProtocol(transports, blobStore)
	treeProto := prototree.NewTreeProtocol(transports, controllerHub, txStore, keyStore, peerStore, config)

	for _, proto := range []swarm.Protocol{authProto, blobProto, treeProto} {
		proto.Start()
	}

	if config.HTTPRPC.Enabled {
		httpRPC := rpc.NewHTTPServer(authProto, blobProto, treeProto, peerStore, keyStore, controllerHub)

		rpcServer, err := rpc.StartHTTPRPC(httpRPC, config.HTTPRPC)
		if err != nil {
			return err
		}
		defer rpcServer.Close()
	}

	for _, bootstrapPeer := range config.Node.BootstrapPeers {
		bootstrapPeer := bootstrapPeer
		go func() {
			logger.Infof(0, "connecting to bootstrap peer: %v %v", bootstrapPeer.Transport, bootstrapPeer.DialAddresses)
			for _, dialAddr := range bootstrapPeer.DialAddresses {
				peerStore.AddDialInfos([]swarm.PeerDialInfo{{TransportName: bootstrapPeer.Transport, DialAddr: dialAddr}})
			}
		}()
	}

	go func() {
		time.Sleep(5 * time.Second)
		logger.Warnf("trying to subscribe %+v", config.Node.SubscribedStateURIs)
		for stateURI := range config.Node.SubscribedStateURIs {
			logger.Warnf("trying to subscribe to %v", stateURI)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			sub, err := treeProto.Subscribe(ctx, stateURI, prototree.SubscriptionType_Txs, nil, nil)
			if err != nil {
				logger.Errorf("error subscribing to %v: %v", stateURI, err)
				continue
			}
			sub.Close()
			logger.Successf("subscribed to %v", stateURI)
		}
	}()

	klog.Info(utils.PrettyJSON(config))

	if gui {
		go func() {
			for {
				select {
				case <-time.After(3 * time.Second):
					stateURIs, err := controllerHub.KnownStateURIs()
					if err != nil {
						continue
					}

					termUI.Sidebar.SetStateURIs(stateURIs)
					states := make(map[string]string)
					for _, stateURI := range stateURIs {
						node, err := controllerHub.StateAtVersion(stateURI, nil)
						if err != nil {
							panic(err)
						}
						states[stateURI] = utils.PrettyJSON(node)
					}
					termUI.StatePane.SetStates(states)
				case <-termUI.Done():
					return
				}
			}
		}()
		<-termUI.Done()

	} else {
		go inputLoop(controllerHub, peerStore)
		<-utils.AwaitInterrupt()
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

func ensureDataDirs(config *config.Config) error {
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

func inputLoop(controllerHub tree.ControllerHub, peerStore swarm.PeerStore) {
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
			logger.Error("enter a command")
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
			logger.Error("unknown command")
			continue
		}

		err := cmd.Handler(context.Background(), parts[1:], controllerHub, peerStore)
		if err != nil {
			logger.Error(err)
		}
	}
}

var replCommands = map[string]struct {
	HelpText string
	Handler  func(ctx context.Context, args []string, controllerHub tree.ControllerHub, peerStore swarm.PeerStore) error
}{
	"stateuris": {
		"list all known state URIs",
		func(ctx context.Context, args []string, controllerHub tree.ControllerHub, peerStore swarm.PeerStore) error {
			stateURIs, err := controllerHub.KnownStateURIs()
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
		func(ctx context.Context, args []string, controllerHub tree.ControllerHub, peerStore swarm.PeerStore) error {
			if len(args) < 1 {
				return errors.New("missing argument: state URI")
			}

			stateURI := args[0]
			node, err := controllerHub.StateAtVersion(stateURI, nil)
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
			logger.Debugf("stateURI: %v / keypath: %v / range: %v", stateURI, keypath, rng)
			node = node.NodeAt(keypath, rng)
			node.DebugPrint(logger.Debugf, false, 0)
			fmt.Println(utils.PrettyJSON(node))
			return nil
		},
	},
	"peers": {
		"list all known peers",
		func(ctx context.Context, args []string, controllerHub tree.ControllerHub, peerStore swarm.PeerStore) error {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.Debug)
			defer w.Flush()
			fmt.Fprintf(w, "Addrs\tDial info\tLast contact\tLast failure\tFailures\n")
			for _, peer := range peerStore.Peers() {
				var lastContact, lastFailure string
				if !peer.LastContact().IsZero() {
					lastContact = time.Now().Sub(peer.LastContact()).String()
				}
				if !peer.LastFailure().IsZero() {
					lastFailure = time.Now().Sub(peer.LastFailure()).String()
				}
				fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\n", peer.Addresses(), peer.DialInfo(), lastContact, lastFailure, peer.Failures())
			}
			return nil
		},
	},
	"addpeer": {
		"list all known peers",
		func(ctx context.Context, args []string, controllerHub tree.ControllerHub, peerStore swarm.PeerStore) error {
			if len(args) < 2 {
				return errors.New("requires two arguments: addpeer <transport> <dial addr>")
			}
			peerStore.AddDialInfos([]swarm.PeerDialInfo{{args[0], args[1]}})
			return nil
		},
	},
}
