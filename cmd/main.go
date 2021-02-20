package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/brynbellomy/klog"
	"github.com/pkg/errors"
	"github.com/urfave/cli"

	rw "redwood.dev"
	"redwood.dev/crypto"
	"redwood.dev/ctx"
	"redwood.dev/tree"
)

var app = struct {
	ctx.Context
}{}

func main() {
	go func() {
		http.ListenAndServe(":6060", nil)
	}()
	runtime.SetBlockProfileRate(int(time.Millisecond.Nanoseconds()) * 100)

	cliApp := cli.NewApp()
	// cliApp.Version = env.AppVersion

	configRoot, err := rw.DefaultConfigRoot("redwood")
	if err != nil {
		app.Error(err)
		os.Exit(1)
	}

	cliApp.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config",
			Value: filepath.Join(configRoot, ".redwoodrc"),
			Usage: "location of config file",
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
		configPath := c.String("config")
		gui := c.Bool("gui")
		dev := c.Bool("dev")
		return run(configPath, gui, dev)
	}

	err = cliApp.Run(os.Args)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}

func run(configPath string, gui bool, dev bool) error {
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

	config, err := rw.ReadConfigAtPath("redwood", configPath)
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
	app.CtxAddChild(refStore.Ctx(), nil)

	err = txStore.Start()
	if err != nil {
		return err
	}
	app.CtxAddChild(txStore.Ctx(), nil)

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
		tlsCertFilename := filepath.Join(config.Node.DataRoot, "server.crt")
		tlsKeyFilename := filepath.Join(config.Node.DataRoot, "server.key")

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

	host, err := rw.NewHost(signingKeypair, encryptingKeypair, transports, controllerHub, refStore, peerStore, config)
	if err != nil {
		return err
	}

	err = host.Start()
	if err != nil {
		return err
	}

	if config.HTTPRPC.Enabled {
		httpRPC := rw.NewHTTPRPCServer(signingKeypair.Address(), host)

		err = rw.StartHTTPRPC(httpRPC, config.HTTPRPC)
		if err != nil {
			return err
		}
		app.CtxAddChild(httpRPC.Ctx(), nil)
	}

	for _, bootstrapPeer := range config.Node.BootstrapPeers {
		bootstrapPeer := bootstrapPeer
		go func() {
			app.Infof(0, "connecting to bootstrap peer: %v %v", bootstrapPeer.Transport, bootstrapPeer.DialAddresses)
			for _, dialAddr := range bootstrapPeer.DialAddresses {
				host.AddPeer(rw.PeerDialInfo{TransportName: bootstrapPeer.Transport, DialAddr: dialAddr})
			}
		}()
	}

	go func() {
		time.Sleep(5 * time.Second)
		app.Warnf("trying to subscribe %+v", config.Node.SubscribedStateURIs)
		for stateURI := range config.Node.SubscribedStateURIs {
			app.Warnf("trying to subscribe to %v", stateURI)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			sub, err := host.Subscribe(ctx, stateURI, rw.SubscriptionType_Txs, nil, nil)
			if err != nil {
				app.Errorf("error subscribing to %v: %v", stateURI, err)
				continue
			}
			sub.Close()
			app.Successf("subscribed to %v", stateURI)
		}
	}()

	app.CtxAddChild(host.Ctx(), nil)
	app.CtxStart(
		func() error { return nil },
		nil,
		nil,
		nil,
	)
	defer app.CtxStop("shutdown", nil)

	klog.Info(rw.PrettyJSON(config))

	if gui {
		go func() {
			for {
				select {
				case <-time.After(3 * time.Second):
					stateURIs, err := host.Controllers().KnownStateURIs()
					if err != nil {
						continue
					}

					termUI.Sidebar.SetStateURIs(stateURIs)
					states := make(map[string]string)
					for _, stateURI := range stateURIs {
						node, err := host.Controllers().StateAtVersion(stateURI, nil)
						if err != nil {
							panic(err)
						}
						states[stateURI] = rw.PrettyJSON(node)
					}
					termUI.StatePane.SetStates(states)
				case <-termUI.Done():
					return
				}
			}
		}()
		<-termUI.Done()

	} else {
		go inputLoop(host)
		app.AttachInterruptHandler()
		app.CtxWait()
	}
	return nil
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

func inputLoop(host rw.Host) {
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

		err := cmd.Handler(app.Ctx(), parts[1:], host)
		if err != nil {
			app.Error(err)
		}
	}
}

var replCommands = map[string]struct {
	HelpText string
	Handler  func(ctx context.Context, args []string, host rw.Host) error
}{
	"stateuris": {
		"list all known state URIs",
		func(ctx context.Context, args []string, host rw.Host) error {
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
		func(ctx context.Context, args []string, host rw.Host) error {
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
}
