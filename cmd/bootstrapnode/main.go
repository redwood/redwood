package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/brynbellomy/klog"
	cryptop2p "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli"

	"redwood.dev/cmd/cmdutils"
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
	} `json:"datastore"`
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
				cfg.Datastore.Encryption.Key = datastoreKey.Bytes()
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

				bn := libp2p.NewBootstrapNode(
					config.Port,
					config.BootstrapPeers,
					p2pkey,
					config.DNSOverHTTPSURL,
					config.Datastore.Path,
					config.Datastore.Encryption,
				)
				err = bn.Start()
				if err != nil {
					return err
				}
				defer bn.Close()

				go func() {
					startREPL(bn)
					<-cmdutils.AwaitInterrupt()
					bn.Close()
				}()

				bn.Process.Go(nil, "repl (await termination)", func(ctx context.Context) {
					<-ctx.Done()
					err := os.Stdin.Close()
					if err != nil {
						panic(err)
					}
				})

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

type replCommand struct {
	Command  string
	HelpText string
	Handler  func(args []string, bn libp2p.BootstrapNode) error
}

var replCommands = []replCommand{
	{
		"peerid",
		"show the node's libp2p peer ID",
		func(args []string, bn libp2p.BootstrapNode) error {
			bn.Infof(0, "%v", bn.Libp2pPeerID())
			return nil
		},
	},
	{
		"peers",
		"show the peers known to the node",
		func(args []string, bn libp2p.BootstrapNode) error {
			var data [][]string

			for _, addrinfo := range bn.Peers() {
				connectedness := bn.Libp2pHost().Network().Connectedness(addrinfo.ID)
				conns := bn.Libp2pHost().Network().ConnsToPeer(addrinfo.ID)
				for _, ma := range addrinfo.Addrs {
					for _, conn := range conns {
						stat := conn.Stat()
						row := []string{addrinfo.ID.Pretty(), ma.String(), connectedness.String(), time.Since(stat.Opened).Round(1*time.Second).String() + " ago"}
						data = append(data, row)
					}
				}
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
			table.SetCenterSeparator("|")
			table.SetRowLine(true)
			table.SetHeader([]string{"Peer ID", "Addr", "Connectedness", "Conn opened"})
			table.SetAutoMergeCellsByColumnIndex([]int{0, 1})
			table.SetColumnColor(
				tablewriter.Colors{tablewriter.Bold, tablewriter.FgCyanColor},
				tablewriter.Colors{},
				tablewriter.Colors{},
				tablewriter.Colors{},
			)
			table.AppendBulk(data)
			table.Render()

			return nil
		},
	},
}

func startREPL(bn libp2p.BootstrapNode) {
	fmt.Println("Type \"help\" for a list of commands.")
	fmt.Println()

	commands := make(map[string]replCommand)
	for _, cmd := range replCommands {
		commands[cmd.Command] = cmd
	}

	var longestCommandLength int
	for _, cmd := range replCommands {
		if len(cmd.Command) > longestCommandLength {
			longestCommandLength = len(cmd.Command)
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
			bn.Error("enter a command")
			continue
		} else if parts[0] == "help" {
			fmt.Println("___ Commands _________")
			fmt.Println()
			for _, cmd := range replCommands {
				difference := longestCommandLength - len(cmd.Command)
				space := strings.Repeat(" ", difference+4)
				fmt.Printf("%v%v- %v\n", cmd.Command, space, cmd.HelpText)
			}
			continue
		}

		cmd, exists := commands[parts[0]]
		if !exists {
			bn.Error("unknown command")
			continue
		}

		err := cmd.Handler(parts[1:], bn)
		if err != nil {
			bn.Error(err)
		}
	}
}
