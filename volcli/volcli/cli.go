package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/contiv/volplugin/volcli"
)

var flags = []cli.Flag{
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

func main() {
	app := cli.NewApp()

	app.Flags = flags

	app.Commands = []cli.Command{
		{
			Name: "tenant",
			Subcommands: []cli.Command{
				{
					Name:   "upload",
					Flags:  flags,
					Action: volcli.TenantUpload,
				},
				{
					Name:   "delete",
					Flags:  flags,
					Action: volcli.TenantDelete,
				},
				{
					Name:   "get",
					Flags:  flags,
					Action: volcli.TenantGet,
				},
				{
					Name:   "list",
					Flags:  flags,
					Action: volcli.TenantList,
				},
			},
		},
	}

	app.Run(os.Args)
}
