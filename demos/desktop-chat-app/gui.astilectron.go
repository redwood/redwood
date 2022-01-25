//go:build !webview
package main

import (
	"sync"

	"github.com/asticode/go-astikit"
	"github.com/asticode/go-astilectron"
	bootstrap "github.com/asticode/go-astilectron-bootstrap"
)

type GUI struct {
	closeOnce sync.Once
	chDone    chan struct{}
	api       *API
	window    *astilectron.Window
}

func newGUI(api *API) *GUI {
	return &GUI{
		api:    api,
		chDone: make(chan struct{}),
	}
}

// Vars injected via ldflags by bundler
var (
	AppName            string
	BuiltAt            string
	VersionAstilectron string
	VersionElectron    string
)

func (gui *GUI) Start() error {
	defer close(gui.chDone)

	return bootstrap.Run(bootstrap.Options{
		Asset:    Asset,
		AssetDir: AssetDir,
		AstilectronOptions: astilectron.Options{
			AppName: AppName,
			SingleInstance:     true,
			VersionAstilectron: VersionAstilectron,
			VersionElectron:    VersionElectron,
		},
		Debug: true,
		// Logger: log.New(log.Writer(), log.Prefix(), log.Flags()),
		MenuOptions: []*astilectron.MenuItemOptions{{
			Label: astikit.StrPtr("File"),
			SubMenu: []*astilectron.MenuItemOptions{
				{Role: astilectron.MenuItemRoleClose},
			},
		}},
		OnWait: func(_ *astilectron.Astilectron, ws []*astilectron.Window, _ *astilectron.Menu, _ *astilectron.Tray, _ *astilectron.Menu) error {
			gui.window = ws[0]
			return nil
		},
		RestoreAssets: RestoreAssets,
		Windows: []*bootstrap.Window{{
			Homepage: "http://localhost:54231/index.html",
			Options: &astilectron.WindowOptions{
				BackgroundColor: astikit.StrPtr("#333"),
				Center:          astikit.BoolPtr(true),
				Height:          astikit.IntPtr(700),
				Width:           astikit.IntPtr(700),
				Closable:        astikit.BoolPtr(true),
			},
		}},
	})
}

func (gui *GUI) Close() (err error) {
	return gui.window.Close()
	return nil
}
