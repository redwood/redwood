package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"

	"github.com/brynbellomy/klog"
	"github.com/urfave/cli"

	"redwood.dev/cmd/cmdutils"
	"redwood.dev/process"
)

func init() {
	runtime.LockOSThread()
}

func main() {
	cliApp := cli.NewApp()
	// cliApp.Version = env.AppVersion

	configPath, err := cmdutils.DefaultConfigPath("hush")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	profileRoot, err := cmdutils.DefaultDataRoot("hush")
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
		// runtime.SetBlockProfileRate(int(time.Millisecond.Nanoseconds()) * 100)
		// runtime.SetCPUProfileRate(1000)

		var (
			profileRoot = c.String("root")
			configFile  = c.String("config")
			port        = c.Uint("port")
		)

		// Create the profileRoot if it doesn't exist
		if _, err := os.Stat(profileRoot); os.IsNotExist(err) {
			err := os.MkdirAll(profileRoot, 0777|os.ModeDir)
			if err != nil {
				return err
			}
		}
		fmt.Println("config file:", configFile)
		fmt.Println("profile root:", profileRoot)
		fmt.Println("port:", port)

		// go func() {
		// 	http.ListenAndServe("localhost:6060", nil)
		// }()

		masterProcess := process.New("root")

		err := masterProcess.Start()
		if err != nil {
			return err
		}
		defer masterProcess.Close()

		api := newAPI(port, configFile, profileRoot, masterProcess)
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
		err = gui.Start()
		if err != nil {
			return err
		}
		return nil
	}

	err = cliApp.Run(os.Args)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
