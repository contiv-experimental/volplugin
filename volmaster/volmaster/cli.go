package main

import (
	"os"

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

	cfg := config.NewTopLevelConfig(ctx.String("prefix"), ctx.StringSlice("etcd"))

	volmaster.Daemon(cfg, ctx.Bool("debug"), ctx.String("listen"))
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
			Value:  ":8080",
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
	app.Run(os.Args)
}
