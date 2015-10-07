package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/contiv/volplugin/volcli"
)

var volmasterFlags = []cli.Flag{
	cli.StringFlag{
		Name:  "master",
		Usage: "address of volmaster process",
		Value: "127.0.0.1:8080",
	},
}

var flags = []cli.Flag{
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

func main() {
	app := cli.NewApp()

	app.Version = ""
	app.Flags = flags
	app.Usage = "Command volplugin and ceph infrastructure"
	app.ArgsUsage = "[subcommand] [arguments]"

	app.Commands = []cli.Command{
		{
			Name:  "tenant",
			Usage: "Manage Tenants",
			Subcommands: []cli.Command{
				{
					Name:        "upload",
					Flags:       flags,
					ArgsUsage:   "[tenant name]. accepts from stdin",
					Description: "Uploads a tenant to etcd. Accepts JSON for the tenant policy. Requires direct, unauthenticated access to etcd.",
					Usage:       "Upload a tenant to etcd",
					Action:      volcli.TenantUpload,
				},
				{
					Name:        "delete",
					Flags:       flags,
					ArgsUsage:   "[tenant name]",
					Description: "Permanently removes a tenant from etcd. Volumes that belong to the tenant are unaffected.",
					Usage:       "Delete a tenant",
					Action:      volcli.TenantDelete,
				},
				{
					Name:        "get",
					Flags:       flags,
					ArgsUsage:   "[tenant name]",
					Usage:       "Obtain the policy for a tenant",
					Description: "Gets the policy for a tenant from etcd.",
					Action:      volcli.TenantGet,
				},
				{
					Name:        "list",
					Flags:       flags,
					ArgsUsage:   "",
					Description: "Reads tenants and generates a newline-delimited list",
					Usage:       "List all tenants",
					Action:      volcli.TenantList,
				},
			},
		},
		{
			Name:  "volume",
			Usage: "Manage Volumes",
			Subcommands: []cli.Command{
				{
					Name: "create",
					Flags: append(flags, append(volmasterFlags, cli.StringSliceFlag{
						Name:  "opt",
						Usage: "Provide key=value options to create the volume",
					})...),
					ArgsUsage:   "[tenant name] [volume name]",
					Description: "This creates a logical volume. Calls out to the volmaster and sets the policy based on the tenant name provided.",
					Usage:       "Create a volume for a given tenant",
					Action:      volcli.VolumeCreate,
				},
				{
					Name:        "get",
					Flags:       flags,
					ArgsUsage:   "[tenant name] [volume name]",
					Usage:       "Get JSON configuration for a volume",
					Description: "Obtain the JSON configuration for the volume",
					Action:      volcli.VolumeGet,
				},
				{
					Name:        "list",
					Flags:       flags,
					ArgsUsage:   "[tenant name]",
					Description: "Given a tenant name, produces a newline-delimited list of volumes.",
					Usage:       "List all volumes for a given tenant",
					Action:      volcli.VolumeList,
				},
				{
					Name:        "list-all",
					ArgsUsage:   "",
					Description: "Produces a newline-delimited list of tenant/volume combinations.",
					Usage:       "List all volumes across tenants",
					Flags:       flags,
					Action:      volcli.VolumeListAll,
				},
				{
					Name:        "force-remove",
					ArgsUsage:   "[tenant name] [volume name]",
					Description: "Forcefully removes a volume without deleting or unmounting the underlying image",
					Usage:       "Forcefully remove a volume without removing the underlying image",
					Flags:       flags,
					Action:      volcli.VolumeForceRemove,
				},
				{
					Name:        "remove",
					ArgsUsage:   "[tenant name] [volume name]",
					Description: "Remove the volume for a tenant, deleting its contents.",
					Usage:       "Remove a volume and its contents",
					Flags:       append(flags, volmasterFlags...),
					Action:      volcli.VolumeRemove,
				},
			},
		},
		{
			Name:  "mount",
			Usage: "Manage Mounts",
			Subcommands: []cli.Command{
				{
					Name:        "list",
					Usage:       "List mounts",
					Description: "List the mounts the volmaster knows about, in newline-delimited form.",
					ArgsUsage:   "",
					Flags:       flags,
					Action:      volcli.MountList,
				},
				{
					Name:        "get",
					Usage:       "Get mount info",
					Description: "Obtains the information on a specified mount. Requires that you know the pool and image name.",
					ArgsUsage:   "[pool name] [volume name]",
					Flags:       flags,
					Action:      volcli.MountGet,
				},
				{
					Name:        "force-remove",
					ArgsUsage:   "[pool name] [volume name]",
					Usage:       "Forcefully remove mount information",
					Description: "Force-remove a mount. Use this to correct unmounting errors or failing hosts if necessary. Requires that you know the pool and image name.",
					Flags:       flags,
					Action:      volcli.MountForceRemove,
				},
			},
		},
	}

	app.Run(os.Args)
}
