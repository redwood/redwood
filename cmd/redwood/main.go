package main

import (
	"context"
	"fmt"
	"io/ioutil"
	_ "net/http/pprof"
	"os"
	"path/filepath"

	"github.com/brynbellomy/klog"
	"github.com/urfave/cli"

	"redwood.dev/cmd/cmdutils"
	"redwood.dev/errors"
	"redwood.dev/log"
)

var logger = log.NewLogger("redwood")

func main() {
	defer klog.Flush()

	cliApp := cli.NewApp()
	// cliApp.Version = env.AppVersion

	configRoot, err := cmdutils.DefaultConfigRoot("redwood")
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
		cli.StringFlag{
			Name:  "k, libp2p-key-file",
			Usage: "location of libp2p key file",
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

	cliApp.Action = run

	err = cliApp.Run(os.Args)
	if err != nil {
		fmt.Printf("error: %+v\n", err)
		os.Exit(1)
	}
}

const AppName = "redwood"

func run(c *cli.Context) (err error) {
	defer errors.AddStack(&err)

	var (
		passwordFile = c.String("password-file")
		configPath   = c.String("config")
		gui          = c.Bool("gui")
		dev          = c.Bool("dev")
		stateURIs    = c.StringSlice("subscribe")
	)

	if passwordFile == "" {
		return errors.New("must specify --password-file flag")
	}
	passwordBytes, err := ioutil.ReadFile(passwordFile)
	if err != nil {
		return err
	}

	fmt.Println("reading config at", configPath)

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
		err := app.TreeProto.Subscribe(context.Background(), stateURI)
		if err != nil {
			return err
		}
	}

	go func() {
		err = app.Start()
		if err != nil {
			panic(fmt.Sprintf("%+v", err))
		}
	}()

	fmt.Println("hi")

	<-app.Done()
	return nil
}
