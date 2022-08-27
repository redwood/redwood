package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/brynbellomy/klog"
	"github.com/urfave/cli"

	"redwood.dev/cmd/cmdutils"
	"redwood.dev/errors"
	"redwood.dev/types"
	"redwood.dev/utils"
	. "redwood.dev/utils/generics"
	"redwood.dev/vault"
)

type Config struct {
	DiskStorePath           string             `json:"diskStorePath"`
	JWTSecret               []byte             `json:"jwtSecret"`
	Port                    uint64             `json:"port"`
	DefaultUserCapabilities []vault.Capability `json:"defaultUserCapabilities"`
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
				jwtSecret := make([]byte, 32)
				_, err := rand.Read(jwtSecret)
				if err != nil {
					return err
				}

				var cfg Config
				cfg.DiskStorePath = ""
				cfg.JWTSecret = jwtSecret
				cfg.Port = 29292
				cfg.DefaultUserCapabilities = []vault.Capability{
					vault.Capability_Forbidden,
					vault.Capability_Fetch,
					vault.Capability_Store,
					vault.Capability_Admin,
				}
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

				vaultConfig := vault.Config{
					Port:                    config.Port,
					JWTSecret:               config.JWTSecret,
					DefaultUserCapabilities: config.DefaultUserCapabilities,
				}

				store := vault.NewDiskStore(config.DiskStorePath)
				rpcServer := vault.NewServer(vaultConfig, store)
				node := vault.NewNode(rpcServer, store)

				err = utils.EnsureDirAndMaxPerms(config.DiskStorePath, os.FileMode(0700))
				if err != nil {
					return err
				}

				err = node.Start()
				if err != nil {
					return err
				}
				defer node.Close()

				go func() {
					startREPL(node)
					<-cmdutils.AwaitInterrupt()
					node.Close()
				}()

				node.Process.Go(nil, "repl (await termination)", func(ctx context.Context) {
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
	Handler  func(args []string, vaultNode *vault.Node) error
}

var replCommands = []replCommand{
	{
		"collections",
		"show the vault's collections",
		func(args []string, vaultNode *vault.Node) error {
			collections, err := vaultNode.Collections()
			if err != nil {
				return err
			}
			for _, collection := range collections {
				fmt.Println("-", collection)
			}
			fmt.Println()
			return nil
		},
	},
	{
		"items",
		"show the items in a given collection",
		func(args []string, vaultNode *vault.Node) error {
			if len(args) < 1 {
				return errors.Errorf("expected 1 argument, got %v", len(args))
			}
			collectionID := args[0]
			var start, end uint64
			var err error
			if len(args) > 1 {
				start, err = strconv.ParseUint(args[1], 10, 64)
				if err != nil {
					return err
				}
			}
			if len(args) > 2 {
				end, err = strconv.ParseUint(args[2], 10, 64)
				if err != nil {
					return err
				}
			}

			items, err := vaultNode.Items(collectionID, time.Time{}, start, end)
			if err != nil {
				return err
			}
			for _, item := range items {
				fmt.Println("-", item.ItemID, item.Mtime)
			}
			fmt.Println()
			return nil
		},
	},
	{
		"allow",
		"add capabilities for a given user",
		func(args []string, vaultNode *vault.Node) error {
			if len(args) < 2 {
				return errors.Errorf("expected 2 or more arguments, got %v", len(args))
			}
			addr, err := types.AddressFromHex(args[0])
			if err != nil {
				return err
			}
			newCapabilities, err := MapWithError(args[1:], func(arg string) (c vault.Capability, err error) { return c, c.UnmarshalText([]byte(arg)) })
			if err != nil {
				return err
			}
			capabilities := vaultNode.UserCapabilities(addr)
			capabilities.AddAll(newCapabilities...)
			vaultNode.SetUserCapabilities(addr, capabilities.Slice())
			return nil
		},
	},
	{
		"deleteall",
		"remove all items in the vault",
		func(args []string, vaultNode *vault.Node) error {
			return vaultNode.DeleteAll()
		},
	},
}

func startREPL(vaultNode *vault.Node) {
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
			vaultNode.Error("enter a command")
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
			vaultNode.Error("unknown command")
			continue
		}

		err := cmd.Handler(parts[1:], vaultNode)
		if err != nil {
			vaultNode.Error(err)
		}
	}
}
