package main

import (
	"fmt"
	"os"

	"github.com/codegangsta/cli"
	"github.com/contiv/volplugin/config"
	"github.com/contiv/volplugin/volsupervisor"

	log "github.com/Sirupsen/logrus"
)

func start(ctx *cli.Context) {
	cfg, err := config.NewTopLevelConfig(ctx.String("prefix"), ctx.StringSlice("etcd"))
	if err != nil {
		log.Fatal(err)
	}

	dc := &volsupervisor.DaemonConfig{
		Config: cfg,
	}

	dc.Daemon()
}

func main() {
	app := cli.NewApp()
	app.Version = ""
	app.Usage = "Control many volplugins"
	app.Action = start
	app.Flags = []cli.Flag{
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
