package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/brynbellomy/klog"
	cryptop2p "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/urfave/cli"

	"redwood.dev/crypto"
	"redwood.dev/state"
	"redwood.dev/swarm/libp2p"
)

type Config struct {
	Port      uint   `json:"port"`
	P2PKey    string `json:"p2pKey"`
	Datastore struct {
		Path       string                 `json:"path"`
		Encryption state.EncryptionConfig `json:"encryption"`
	} `json: "datastore"`
	BootstrapPeers  []string `json:"bootstrapPeers"`
	DNSOverHTTPSURL string   `json:"dnsOverHTTPSURL"`
}

func main() {
	cliApp := cli.NewApp()

	cliApp.Commands = []cli.Command{
		{
			Name: "genconfig",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "config, c",
					Usage: "path to the config file",
				},
			},
			Action: func(c *cli.Context) error {
				p2pkey, _, err := cryptop2p.GenerateKeyPair(cryptop2p.Ed25519, 0)
				if err != nil {
					return err
				}
				datastoreKey, err := crypto.NewSymEncKey()
				if err != nil {
					return err
				}
				p2pkeyBytes, err := cryptop2p.MarshalPrivateKey(p2pkey)
				if err != nil {
					return err
				}

				var cfg Config
				cfg.Port = 21231
				cfg.P2PKey = cryptop2p.ConfigEncodeKey(p2pkeyBytes)
				cfg.Datastore.Path = "./data"
				cfg.Datastore.Encryption.Key = datastoreKey
				cfg.Datastore.Encryption.KeyRotationInterval = 24 * time.Hour
				cfg.BootstrapPeers = []string{}
				cfg.DNSOverHTTPSURL = ""
				configBytes, err := json.MarshalIndent(cfg, "", "    ")
				if err != nil {
					return err
				}
				return ioutil.WriteFile(c.String("config"), configBytes, 0600)
			},
		},
		{
			Name: "start",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "config, c",
					Usage: "path to the config file",
				},
			},
			Action: func(c *cli.Context) error {
				flagset := flag.NewFlagSet("", flag.ContinueOnError)
				klog.InitFlags(flagset)
				flagset.Set("logtostderr", "true")
				flagset.Set("v", "2")
				klog.SetFormatter(&klog.FmtConstWidth{
					FileNameCharWidth: 24,
					UseColor:          true,
				})

				configBytes, err := ioutil.ReadFile(c.String("config"))
				if err != nil {
					return err
				}

				var config Config
				err = json.Unmarshal(configBytes, &config)
				if err != nil {
					return err
				}

				p2pkeyBytes, err := cryptop2p.ConfigDecodeKey(config.P2PKey)
				if err != nil {
					return err
				}
				p2pkey, err := cryptop2p.UnmarshalPrivateKey(p2pkeyBytes)
				if err != nil {
					return err
				}

				bn := libp2p.NewBootstrapNode(config.Port, config.BootstrapPeers, p2pkey, config.DNSOverHTTPSURL, config.Datastore.Path, config.Datastore.Encryption)
				err = bn.Start()
				if err != nil {
					return err
				}
				defer bn.Close()
				select {}
			},
		},
	}

	err := cliApp.Run(os.Args)
	if err != nil {
		fmt.Printf("error: %+v\n", err)
		os.Exit(1)
	}
}
