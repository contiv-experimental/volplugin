package main

import (
	"fmt"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/volplugin/apiserver"
	"github.com/contiv/volplugin/config"

	"github.com/codegangsta/cli"
)

// version is provided by build
var version = ""

func start(ctx *cli.Context) {
	cfg, err := config.NewClient(ctx.String("prefix"), ctx.StringSlice("etcd"))
	if err != nil {
		logrus.Fatal(err)
	}

	d := &apiserver.DaemonConfig{
		Config:   cfg,
		MountTTL: ctx.Int("ttl"),
		Timeout:  time.Duration(ctx.Int("timeout")) * time.Minute,
	}

	d.Daemon(ctx.String("listen"))
}

func main() {
	app := cli.NewApp()
	app.Version = version
	app.Usage = "Control many volplugins"
	app.Action = start
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "listen",
			Usage:  "listen address for apiserver",
			EnvVar: "LISTEN",
			Value:  ":9005",
		},
		cli.StringFlag{
			Name:  "prefix",
			Usage: "prefix key used in etcd for namespacing",
			Value: "/volplugin",
		},
		cli.StringSliceFlag{
			Name:  "etcd",
			Usage: "URL for etcd",
			Value: &cli.StringSlice{"http://localhost:2379"},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n\n", err)
		os.Exit(1)
	}
}
