package main

import (
	"fmt"
	"os"
	"time"

	"github.com/codegangsta/cli"
	"github.com/contiv/volplugin/volplugin"
)

var host string

func init() {
	var err error
	host, err = os.Hostname()
	if err != nil {
		panic("Could not retrieve hostname")
	}
}

func main() {
	app := cli.NewApp()
	app.Version = ""
	app.Usage = "Mount and manage Ceph RBD for containers"
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:   "debug",
			Usage:  "Turn on debug logging",
			EnvVar: "DEBUG",
		},
		cli.StringFlag{
			Name:   "master",
			Usage:  "Set the volmaster host:port",
			EnvVar: "MASTER",
			Value:  "localhost:9005",
		},
		cli.StringFlag{
			Name:   "host-label",
			Usage:  "Set the internal hostname",
			EnvVar: "HOSTLABEL",
			Value:  host,
		},
		cli.IntFlag{
			Name:  "ttl",
			Usage: "Set the timeout for refreshing mount point data to the volmaster",
			Value: 300,
		},
		cli.IntFlag{
			Name:  "timeout",
			Usage: "Set timeout for ceph commands; in minutes",
			Value: 5,
		},
	}
	app.Action = run

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n\n", err)
		os.Exit(1)
	}
}

func run(ctx *cli.Context) {
	dc := &volplugin.DaemonConfig{
		Debug:   ctx.Bool("debug"),
		TTL:     ctx.Int("ttl"),
		Master:  ctx.String("master"),
		Host:    ctx.String("host-label"),
		Timeout: time.Duration(ctx.Int("timeout")) * time.Minute,
	}

	if err := dc.Daemon(); err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n\n", err)
		os.Exit(1)
	}
}
