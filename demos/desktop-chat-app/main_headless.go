// +build headless

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
	"redwood.dev/utils"

	rw "redwood.dev"
)

func main() {
	cliApp := cli.NewApp()
	// cliApp.Version = env.AppVersion

	configPath, err := rw.DefaultConfigPath("redwood-chat")
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
		app.configPath = c.String("config")
		app.devMode = c.Bool("dev")
		return app.Start()
	}

	err = cliApp.Run(os.Args)
	if err != nil {
		fmt.Println("error:", err)
		app.Close()
		os.Exit(1)
	}
	defer app.Close()

	<-utils.AwaitInterrupt()
}
