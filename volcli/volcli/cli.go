package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/contiv/volplugin/volcli"
)

func main() {
	app := cli.NewApp()

	app.Commands = []cli.Command{
		{
			Name: "tenant",
			Subcommands: []cli.Command{
				{
					Name:   "upload",
					Action: volcli.TenantUpload,
				},
				{
					Name: "delete",
				},
				{
					Name: "get",
				},
			},
		},
	}

	app.Run(os.Args)
}
