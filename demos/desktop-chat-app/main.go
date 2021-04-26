// +build !headless

package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"time"

	"github.com/brynbellomy/klog"
	"github.com/urfave/cli"
	"github.com/webview/webview"

	"redwood.dev/config"
	"redwood.dev/utils"
)

func main() {
	cliApp := cli.NewApp()
	// cliApp.Version = env.AppVersion

	configPath, err := config.DefaultConfigPath("redwood-chat")
	if err != nil {
		app.Error(err)
		os.Exit(1)
	}

	profileRoot, err := config.DefaultDataRoot("redwood-chat")
	if err != nil {
		app.Error(err)
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

		// if c.Bool("pprof") {
		go func() {
			http.ListenAndServe(":"+c.String("pprof"), nil)
		}()
		runtime.SetBlockProfileRate(int(time.Millisecond.Nanoseconds()) * 100)
		// }

		app.configPath = c.String("config")
		app.profileRoot = c.String("root")

		// Create the profileRoot if it doesn't exist
		if _, err := os.Stat(app.profileRoot); os.IsNotExist(err) {
			err := os.MkdirAll(app.profileRoot, 0777|os.ModeDir)
			if err != nil {
				panic(err)
			}
		}

		port := c.Uint("port")
		go startGUI(port)
		go startAPI(port)

		defer app.Close()

		<-utils.AwaitInterrupt()
		return nil
	}

	err = cliApp.Run(os.Args)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}

func startGUI(port uint) {
	debug := true
	w := webview.New(debug)
	defer w.Destroy()
	w.SetTitle("Minimal webview example")
	w.SetSize(800, 600, webview.HintNone)
	w.Navigate(fmt.Sprintf("http://localhost:%v/index.html", port))
	w.Run()
}
