package main

import (
	"fmt"
	"os"

	"github.com/codegangsta/cli"
	"github.com/contiv/volplugin/volcli"
)

// version is provided by build
var version = ""

func main() {
	app := cli.NewApp()

	app.Version = version
	app.Flags = volcli.GlobalFlags
	app.Usage = "Command volplugin and ceph infrastructure"
	app.ArgsUsage = "[subcommand] [arguments]"
	app.Commands = volcli.Commands

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n\n", err)
		os.Exit(1)
	}
}
