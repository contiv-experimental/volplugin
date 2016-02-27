package main

import (
	"fmt"
	"os"
	"time"

	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/volmaster"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

func start(ctx *cli.Context) {
	if ctx.Bool("debug") {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug logging enabled")
	}

	cfg, err := config.NewTopLevelConfig(ctx.String("prefix"), ctx.StringSlice("etcd"))
	if err != nil {
		log.Fatal(err)
	}

	d := &volmaster.DaemonConfig{
		Config:   cfg,
		MountTTL: ctx.Int("ttl"),
		Timeout:  time.Duration(ctx.Int("timeout")) * time.Minute,
	}

	d.Daemon(ctx.Bool("debug"), ctx.String("listen"))
}

func main() {
	app := cli.NewApp()
	app.Version = ""
	app.Usage = "Control many volplugins"
	app.Action = start
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:   "debug",
			Usage:  "turn on debugging",
			EnvVar: "DEBUG",
		},
		cli.StringFlag{
			Name:   "listen",
			Usage:  "listen address for volmaster",
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
		cli.IntFlag{
			Name:  "ttl",
			Usage: "Set ttl of written locks; in seconds",
			Value: 300,
		},
		cli.IntFlag{
			Name:  "timeout",
			Usage: "Set timeout for ceph commands; in minutes",
			Value: 5,
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n\n", err)
		os.Exit(1)
	}
}
