package main

import (
	"bytes"
	"context"
	_ "embed"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"path/filepath"
	"strings"
	"time"

	"redwood.dev/cmd/cmdutils"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/redwood.js/embed/sync9"
	"redwood.dev/rpc"
	"redwood.dev/state"
	"redwood.dev/tree"
	"redwood.dev/utils"
)

type App struct {
	*cmdutils.App
	log.Logger

	started       bool
	httpRPCServer *http.Server

	profileRoot string
	profileName string
	password    string
	mnemonic    string
	configPath  string
	devMode     bool
}

const AppName = "redwood-chat"

func newApp(password, mnemonic, profileRoot, profileName, configPath string, devMode bool, masterProcess process.ProcessTreer) (*App, error) {
	app := &App{
		Logger:      log.NewLogger("app"),
		profileRoot: profileRoot,
		profileName: profileName,
		password:    password,
		mnemonic:    mnemonic,
		configPath:  configPath,
		devMode:     devMode,
	}

	// Copy the default config and unmarshal the config file over it
	cfg := cmdutils.DefaultConfig(AppName)
	err := cmdutils.FindOrCreateConfigAtPath(&cfg, AppName, configPath)
	if err != nil {
		return nil, err
	}

	cfg.Mode = cmdutils.ModeREPL
	cfg.DevMode = devMode
	cfg.DataRoot = filepath.Join(profileRoot, profileName)

	app.Infof(0, "profile: %v", cfg.DataRoot)

	cfg.KeyStore = cmdutils.KeyStoreConfig{
		Password:             password,
		Mnemonic:             mnemonic,
		InsecureScryptParams: false,
	}

	if cfg.HTTPRPC == nil {
		cfg.HTTPRPC = &rpc.HTTPConfig{}
	}
	if cfg.HTTPRPC.ListenHost == "" {
		cfg.HTTPRPC.ListenHost = "127.0.0.1:8081"
	}
	cfg.HTTPRPC.Enabled = true
	cfg.HTTPRPC.Server = func(innerServer *rpc.HTTPServer) interface{} {
		return &HTTPRPCServer{innerServer, app}
	}

	cfg.REPLConfig.Commands = append(cfg.REPLConfig.Commands, cmdutils.REPLCommand{
		Command:  "ps",
		HelpText: "display the current process tree",
		Handler: func(args []string, _ *cmdutils.App) error {
			app.Infof(0, "processes:\n%v", utils.PrettyJSON(masterProcess.ProcessTree()))
			return nil
		},
	})

	err = cfg.Save()
	if err != nil {
		return nil, err
	}

	app.App = cmdutils.NewApp(AppName, cfg)

	app.Info(0, utils.PrettyJSON(cfg))

	return app, nil
}

func (app *App) Start() error {
	err := app.App.Start()
	if err != nil {
		return err
	}

	app.initializeLocalState()
	app.monitorForDMs()

	return nil
}

func (app *App) Close() error {
	return app.Process.Close()
}

func (app *App) monitorForDMs() {
	app.Process.Go("monitorForDMs", func(ctx context.Context) {
		time.Sleep(5 * time.Second)

		sub := app.TreeProto.SubscribeStateURIs()
		defer sub.Close()

		for {
			select {
			case <-ctx.Done():
			default:
			}

			stateURI, err := sub.Read(ctx)
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
					dmState, err := app.ControllerHub.StateAtVersion("chat.local/dms", nil)
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
					err := app.TreeProto.SendTx(ctx, tree.Tx{
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
	})
}

func (app *App) initializeLocalState() {
	_, sync9Sha3, err := app.BlobStore.StoreBlob(ioutil.NopCloser(bytes.NewReader(sync9.RedwoodResolverSrc)))
	if err != nil {
		panic(err)
	}

	type M = map[string]interface{}

	app.EnsureInitialState("chat.local/servers", "value", M{
		"Merge-Type": M{
			"Content-Type": "resolver/js",
			"value": M{
				"src": M{
					"Content-Type": "link",
					"value":        "blob:sha3:" + sync9Sha3.Hex(),
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

	app.EnsureInitialState("chat.local/dms", "rooms", M{
		"Merge-Type": M{
			"Content-Type": "resolver/js",
			"value": M{
				"src": M{
					"Content-Type": "link",
					"value":        "blob:sha3:" + sync9Sha3.Hex(),
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

	app.EnsureInitialState("chat.local/address-book", "value", M{
		"Merge-Type": M{
			"Content-Type": "resolver/js",
			"value": M{
				"src": M{
					"Content-Type": "link",
					"value":        "blob:sha3:" + sync9Sha3.Hex(),
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
