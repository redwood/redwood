package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/brynbellomy/klog"
	"github.com/pkg/errors"
	"github.com/urfave/cli"

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

	db, err := state.NewDBTree(filepath.Join(config.Node.DataRoot, "shared"))
	if err != nil {
		return err
	}

	var (
		txStore       = tree.NewBadgerTxStore(config.TxDBRoot())
		keyStore      = identity.NewBadgerKeyStore(db, identity.DefaultScryptParams)
		blobStore     = blob.NewDiskStore(config.RefDataRoot(), db)
		peerStore     = swarm.NewPeerStore(db)
		controllerHub = tree.NewControllerHub(config.StateDBRoot(), txStore, blobStore)
	)

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

	host, err := swarm.NewHost(transports, controllerHub, keyStore, blobStore, peerStore, config)
	if err != nil {
		return err
	}

	err = keyStore.Unlock(string(passwordBytes), "")
	if err != nil {
		return err
	}

	err = blobStore.Start()
	if err != nil {
		return err
	}
	defer blobStore.Close()

	err = txStore.Start()
	if err != nil {
		return err
	}
	defer txStore.Close()

	err = host.Start()
	if err != nil {
		return err
	}
	defer host.Close()

	if config.HTTPRPC.Enabled {
		httpRPC := rpc.NewHTTPServer(host)

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
				host.AddPeer(swarm.PeerDialInfo{TransportName: bootstrapPeer.Transport, DialAddr: dialAddr})
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

			sub, err := host.Subscribe(ctx, stateURI, swarm.SubscriptionType_Txs, nil, nil)
			if err != nil {
				logger.Errorf("error subscribing to %v: %v", stateURI, err)
				continue
			}
			sub.Close()
			logger.Successf("subscribed to %v", stateURI)
		}
	}()

	var ui interface {
		Start()
		Stop()
		Done() <-chan struct{}
	}
	if gui {
		ui = cmdutils.NewTermUI(host)
	} else {
		ui = cmdutils.NewREPL(host, logger, cmdutils.DefaultREPLCommands)
	}

	go ui.Start()
	defer ui.Stop()

	logger.Info(0, utils.PrettyJSON(config))

	select {
	case <-ui.Done():
	case <-utils.AwaitInterrupt():
	}
	return nil
}

func ensureDataDirs(config *config.Config) error {
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
