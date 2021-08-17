// +build !headless

package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/brynbellomy/klog"
	"github.com/urfave/cli"
	"github.com/webview/webview"

	"redwood.dev/cmd/cmdutils"
	"redwood.dev/process"
)

func init() {
	runtime.LockOSThread()
}

func main() {
	cliApp := cli.NewApp()
	// cliApp.Version = env.AppVersion

	configPath, err := cmdutils.DefaultConfigPath("redwood-chat")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	profileRoot, err := cmdutils.DefaultDataRoot("redwood-chat")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	cliApp.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config",
			Value: configPath,
			Usage: "location of config file",
		},
		cli.StringFlag{
			Name:  "root",
			Value: profileRoot,
			Usage: "location of the data root containing all profiles",
		},
		cli.UintFlag{
			Name:  "port",
			Value: 54231,
			Usage: "port on which to serve UI assets",
		},
		cli.StringFlag{
			Name:  "pprof",
			Value: "6060",
			Usage: "enable pprof",
		},
		cli.BoolFlag{
			Name:  "dev",
			Usage: "enable dev mode",
		},
	}

	cliApp.Action = func(c *cli.Context) error {
		flagset := flag.NewFlagSet("", flag.ContinueOnError)
		klog.InitFlags(flagset)
		flagset.Set("logtostderr", "true")
		flagset.Set("v", "2")
		klog.SetFormatter(&klog.FmtConstWidth{
			FileNameCharWidth: 24,
			UseColor:          true,
		})
		klog.Flush()

		go func() {
			http.ListenAndServe(":"+c.String("pprof"), nil)
		}()
		runtime.SetBlockProfileRate(int(time.Millisecond.Nanoseconds()) * 100)

		profileRoot := c.String("root")

		// Create the profileRoot if it doesn't exist
		if _, err := os.Stat(profileRoot); os.IsNotExist(err) {
			err := os.MkdirAll(profileRoot, 0777|os.ModeDir)
			if err != nil {
				return err
			}
		}
		fmt.Println("profile root:", profileRoot)

		masterProcess := process.New("root")

		err := masterProcess.Start()
		if err != nil {
			return err
		}
		defer masterProcess.Close()

		api := newAPI(c.Uint("port"), c.String("config"), profileRoot, masterProcess)
		gui := newGUI(api)

		err = masterProcess.SpawnChild(context.TODO(), api)
		if err != nil {
			return err
		}

		go func() {
			defer masterProcess.Close()
			defer gui.Close()
			select {
			case <-cmdutils.AwaitInterrupt():
			case <-api.Done():
			case <-gui.chDone:
			}
		}()
		gui.Start()
		return nil
	}

	err = cliApp.Run(os.Args)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}

type GUI struct {
	closeOnce sync.Once
	chDone    chan struct{}

	api     *API
	webview webview.WebView
}

func newGUI(api *API) *GUI {
	return &GUI{
		api:    api,
		chDone: make(chan struct{}),
	}
}

func (gui *GUI) Start() error {
	defer close(gui.chDone)

	debug := true
	gui.webview = webview.New(debug)
	gui.webview.SetTitle("Minimal webview example")
	gui.webview.SetSize(800, 600, webview.HintNone)
	gui.webview.Navigate(fmt.Sprintf("http://localhost:%v/index.html", gui.api.port))
	gui.webview.Run()
	return nil
}

func (gui *GUI) Close() (err error) {
	gui.closeOnce.Do(func() {
		gui.webview.Destroy()
	})
	return nil
}

// MUON STUFF
// cfg := &muon.Config{
//  Title:          "asdf",
//  Height:         800,
//  Width:          600,
//  Titled:         true,
//  Resizeable:     true,
//  FilesystemPath: "../projects/forest/lib",
// }

// m := muon.New(cfg, api.server)
// err := m.Start()
// if err != nil {
//  panic(err)
// }

// ULTRALIGHT STUFF
// app := ultralight.NewApp()
// defer app.Destroy()

// win := app.NewWindow(1024, 768, false, "Ultralight Browser")
// defer win.Destroy()

// ovl := win.Overlay(0)
// ovl.Resize(win.Width(), UI_HEIGHT)

// ovl.View().OnConsoleMessage(func(source ultralight.MessageSource, level ultralight.MessageLevel,
//  message string, line uint, col uint, sourceId string) {
//  fmt.Printf("CONSOLE source=%v level=%v id=%q line=%c col=%v %v\n",
//      source, level, sourceId, line, col, message)
// })

// ui := &UI{win: win, ovl: ovl, tabWidth: win.Width(), tabHeight: win.Height() - UI_HEIGHT, tabs: map[int]*Tab{}}

// view := ovl.View()

// ovl.View().LoadURL("file:///assets/ui.html")
// app.Run()
