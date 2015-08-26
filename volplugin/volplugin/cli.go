package main

import (
	"fmt"
	"os"

	"github.com/codegangsta/cli"
	"github.com/contiv/volplugin/volplugin"
)

func main() {
	app := cli.NewApp()
	app.Name = "volplugin"
	app.Version = "0.0.1"
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:   "debug",
			Usage:  "Turn on debug logging",
			EnvVar: "DEBUG",
		},
	}
	app.Action = run

	app.Run(os.Args)
}

func run(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		fmt.Printf("\nUsage: %s [tenant/driver name]\n\n", os.Args[0])
		cli.ShowAppHelp(ctx)
		os.Exit(1)
	}

	driverName := ctx.Args()[0]
	debug := ctx.Bool("debug")

	volplugin.Daemon(driverName, debug)
}
