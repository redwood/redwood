package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"golang.org/x/net/context"

	"github.com/brynbellomy/klog"
	"github.com/urfave/cli"

	"redwood.dev/cmd/cmdutils"
	"redwood.dev/config"
	"redwood.dev/log"
	"redwood.dev/swarm/prototree"
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
		cli.StringSliceFlag{
			Name:  "subscribe, s",
			Usage: "state URIs to subscribe to immediately upon startup",
		},
	}

	cliApp.Action = func(c *cli.Context) error {
		passwordFile := c.String("password-file")
		configPath := c.String("config")
		gui := c.Bool("gui")
		dev := c.Bool("dev")
		stateURIs := c.StringSlice("subscribe")
		return run(configPath, passwordFile, gui, dev, stateURIs)
	}

	err = cliApp.Run(os.Args)
	if err != nil {
		fmt.Printf("error: %+v\n", err)
		os.Exit(1)
	}
}

const AppName = "redwood"

func run(configPath, passwordFile string, gui, dev bool, stateURIs []string) (err error) {
	defer utils.WithStack(&err)

	if passwordFile == "" {
		return errors.New("must specify --password-file flag")
	}

	passwordBytes, err := ioutil.ReadFile(passwordFile)
	if err != nil {
		return err
	}

	// Copy the default config and unmarshal the config file over it
	cfg := cmdutils.DefaultConfig(AppName)
	err = cmdutils.FindOrCreateConfigAtPath(&cfg, AppName, configPath)
	if err != nil {
		return err
	}

	cfg.KeyStore = cmdutils.KeyStoreConfig{
		Password:             string(passwordBytes),
		Mnemonic:             "",
		InsecureScryptParams: false,
	}

	cfg.DevMode = dev

	if gui {
		cfg.Mode = cmdutils.ModeTermUI
	} else {
		cfg.Mode = cmdutils.ModeREPL
	}

	app := cmdutils.NewApp(AppName, cfg)

	// Subscribe to any state URIs passed on the command line (mainly for demos)
	for _, stateURI := range stateURIs {
		sub, err := app.TreeProto.Subscribe(context.Background(), stateURI, prototree.SubscriptionType_Txs, nil, nil)
		if err != nil {
			return err
		}
		sub.Close()
	}

	go func() {
		err = app.Start()
		if err != nil {
			panic(err)
			// return err
		}
	}()

	fmt.Println("hi")

	<-app.Done()
	return nil
}
