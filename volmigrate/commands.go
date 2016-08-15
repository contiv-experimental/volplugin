package volmigrate

import "github.com/codegangsta/cli"

// GlobalFlags are required global flags for the operation of volmigrate.
var GlobalFlags = []cli.Flag{
	cli.BoolFlag{
		Name:  "silent",
		Usage: "disables prompting before running migrations",
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
}

// Commands is the data structure which describes the command hierarchy for volmigrate.
var Commands = []cli.Command{
	{
		Name:        "list",
		ArgsUsage:   "",
		Usage:       "Lists all schema migration versions",
		Description: "Produces a newline-delimited list of all available schema migration versions",
		Action:      ListMigrations,
	},
	{
		Name:        "run",
		ArgsUsage:   "[version]",
		Usage:       "Runs schema migrations",
		Description: "Runs one or all pending schema migrations",
		Action:      RunMigrations,
	},
	{
		Name:        "version",
		ArgsUsage:   "",
		Usage:       "See the current schema version",
		Description: "See the current schema version",
		Action:      ShowVersions,
	},
}
