package main

import (
	"fmt"
	"os"

	"github.com/codegangsta/cli"
	"github.com/contiv/volplugin/volplugin"
)

var host string

// version is provided by build
var version = ""

func init() {
	var err error
	host, err = os.Hostname()
	if err != nil {
		panic("Could not retrieve hostname")
	}
}

func main() {
	app := cli.NewApp()
	app.Version = version
	app.Usage = "Mount and manage Ceph RBD for containers"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "plugin-name",
			Value: "volcontiv",
			Usage: "Name of plugin presented to docker, et al.",
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
		cli.StringFlag{
			Name:   "host-label",
			Usage:  "Set the internal hostname",
			EnvVar: "HOSTLABEL",
			Value:  host,
		},
	}
	app.Action = run

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n\n", err)
		os.Exit(1)
	}
}

func run(ctx *cli.Context) {
	dc := volplugin.NewDaemonConfig(ctx)

	if err := dc.Daemon(); err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n\n", err)
		os.Exit(1)
	}
}
