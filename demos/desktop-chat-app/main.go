// +build !headless

package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/brynbellomy/klog"
	"github.com/urfave/cli"
	"github.com/webview/webview"

	"redwood.dev"
	"redwood.dev/utils"
)

func main() {
	cliApp := cli.NewApp()
	// cliApp.Version = env.AppVersion

	configPath, err := redwood.DefaultConfigPath("redwood-webview")
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
		cli.UintFlag{
			Name:  "port",
			Value: 54231,
			Usage: "port on which to serve UI assets",
		},
		cli.BoolFlag{
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

		if c.Bool("pprof") {
			go func() {
				http.ListenAndServe(":6060", nil)
			}()
			runtime.SetBlockProfileRate(int(time.Millisecond.Nanoseconds()) * 100)
		}

		port := c.Uint("port")
		// go startGUI(port)
		go startAPI(port)

		defer app.Close()

		<-utils.AwaitInterrupt()
		return nil
	}

	// Get .redwoodrc config
	config, err := redwood.ReadConfigAtPath("redwood-webview", app.configPath)
	if err != nil {
		panic(err)
	}

	// Create DataRoot if it doesn't exist
	splitDataRoot := strings.Split(config.Node.DataRoot, "/")
	app.mainDataRoot = "./" + splitDataRoot[1]
	if _, err := os.Stat(app.mainDataRoot); os.IsNotExist(err) {
		err := os.MkdirAll(app.mainDataRoot, 0777|os.ModeDir)
		if err != nil {
			panic(err)
		}
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
