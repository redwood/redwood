// +build !headless

package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/brynbellomy/klog"
	"github.com/urfave/cli"

	rw "redwood.dev"
)

func main() {
	cliApp := cli.NewApp()
	// cliApp.Version = env.AppVersion

	configPath, err := rw.DefaultConfigPath("redwood-webview")
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

		// configPath := c.String("config")

		if c.Bool("pprof") {
			go func() {
				http.ListenAndServe(":6060", nil)
			}()
			runtime.SetBlockProfileRate(int(time.Millisecond.Nanoseconds()) * 100)
		}

		// dev := c.Bool("dev")
		port := c.Uint("port")
		// return run(configPath, enablePprof, dev, port)
		// go startGUI(port)
		startAPI(port)
		return nil
	}

	go waitForCtrlC()

	err = cliApp.Run(os.Args)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}

func waitForCtrlC() {
	sigInbox := make(chan os.Signal, 1)

	signal.Notify(sigInbox, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	count := 0
	firstTime := int64(0)

	//timer := time.NewTimer(30 * time.Second)

	for range sigInbox {
		count++
		curTime := time.Now().Unix()

		// Prevent un-terminated ^c character in terminal
		fmt.Println()

		if count == 1 {
			firstTime = curTime

			if app != nil {
				app.Close()
			}
			os.Exit(-1)
		} else {
			if curTime > firstTime+3 {
				fmt.Println("\nReceived interrupt before graceful shutdown, terminating...")
				os.Exit(-1)
			}
		}
	}
}
