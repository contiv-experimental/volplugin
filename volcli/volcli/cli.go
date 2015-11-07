package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/contiv/volplugin/volcli"
)

func main() {
	app := cli.NewApp()

	app.Version = ""
	app.Flags = volcli.GlobalFlags
	app.Usage = "Command volplugin and ceph infrastructure"
	app.ArgsUsage = "[subcommand] [arguments]"
	app.Commands = volcli.Commands

	app.Run(os.Args)
}
