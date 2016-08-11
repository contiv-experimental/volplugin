package main

import (
	"fmt"
	"os"

	"github.com/codegangsta/cli"
	"github.com/contiv/volplugin/volmigrate"
)

// version is provided by build
var version = ""

func main() {
	app := cli.NewApp()

	app.Version = version
	app.Flags = volmigrate.GlobalFlags
	app.Usage = "Run schema migrations"
	app.ArgsUsage = ""
	app.Commands = volmigrate.Commands

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n\n", err)
		os.Exit(1)
	}
}
