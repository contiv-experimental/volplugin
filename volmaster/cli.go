package main

import (
	"fmt"
	"os"
	"path"

	"github.com/contiv/volplugin/config"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

func start(ctx *cli.Context) {
	if ctx.Bool("debug") {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug logging enabled")
	}

	cfg := config.NewTopLevelConfig(ctx.String("prefix"), ctx.StringSlice("etcd"))

	if err := cfg.Sync(); err != nil {
		errExit(ctx, err)
	}

	daemon(cfg, ctx.Bool("debug"), ctx.String("listen"))
}

func main() {
	basePath := path.Base(os.Args[0])

	app := cli.NewApp()
	app.Version = "0.0.1"
	app.Usage = fmt.Sprintf("Control many volplugins", basePath)
	app.Name = basePath
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
