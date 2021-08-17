package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/brynbellomy/klog"
	"github.com/urfave/cli"

	"redwood.dev/swarm/libp2p"
)

func main() {
	cliApp := cli.NewApp()

	cliApp.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "p, port",
			Value: "21231",
			Usage: "the port to listen on",
		},
		cli.StringSliceFlag{
			Name:  "b, bootstrap-peer",
			Usage: "bootstrap peer",
		},
		cli.StringFlag{
			Name:  "d, datastore-path",
			Value: "./data",
			Usage: "the path at which to store peer information",
		},
		cli.StringFlag{
			Name:  "dns-over-https",
			Usage: "the URL of a DNS-over-HTTPS server",
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

		port := c.Uint("port")
		bootstrapPeers := c.StringSlice("bootstrap-peer")
		dohDNSResolverURL := c.String("dns-over-https")
		datastorePath := c.String("datastore-path")
		bn := libp2p.NewBootstrapNode(port, bootstrapPeers, dohDNSResolverURL, datastorePath)
		err := bn.Start()
		if err != nil {
			return err
		}
		select {}
	}

	err := cliApp.Run(os.Args)
	if err != nil {
		fmt.Printf("error: %+v\n", err)
		os.Exit(1)
	}
}
