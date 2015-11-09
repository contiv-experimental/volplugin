package volcli

import "github.com/codegangsta/cli"

// VolmasterFlags contains the flags specific to volmasters
var VolmasterFlags = []cli.Flag{
	cli.StringFlag{
		Name:  "volmaster",
		Usage: "address of volmaster process",
		Value: "127.0.0.1:8080",
	},
}

// GlobalFlags are required global flags for the operation of volcli.
var GlobalFlags = []cli.Flag{
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

// Commands is the data structure which describes the command heirarchy
// for volcli.
var Commands = []cli.Command{
	{
		Name:  "tenant",
		Usage: "Manage Tenants",
		Subcommands: []cli.Command{
			{
				Name:        "upload",
				ArgsUsage:   "[tenant name]. accepts from stdin",
				Description: "Uploads a tenant to etcd. Accepts JSON for the tenant policy. Requires direct, unauthenticated access to etcd.",
				Usage:       "Upload a tenant to etcd",
				Action:      TenantUpload,
			},
			{
				Name:        "delete",
				ArgsUsage:   "[tenant name]",
				Description: "Permanently removes a tenant from etcd. Volumes that belong to the tenant are unaffected.",
				Usage:       "Delete a tenant",
				Action:      TenantDelete,
			},
			{
				Name:        "get",
				ArgsUsage:   "[tenant name]",
				Usage:       "Obtain the policy for a tenant",
				Description: "Gets the policy for a tenant from etcd.",
				Action:      TenantGet,
			},
			{
				Name:        "list",
				ArgsUsage:   "",
				Description: "Reads tenants and generates a newline-delimited list",
				Usage:       "List all tenants",
				Action:      TenantList,
			},
		},
	},
	{
		Name:  "volume",
		Usage: "Manage Volumes",
		Subcommands: []cli.Command{
			{
				Name: "create",
				Flags: append(VolmasterFlags, cli.StringSliceFlag{
					Name:  "opt",
					Usage: "Provide key=value options to create the volume",
				}),
				ArgsUsage:   "[tenant name] [volume name]",
				Description: "This creates a logical volume. Calls out to the volmaster and sets the policy based on the tenant name provided.",
				Usage:       "Create a volume for a given tenant",
				Action:      VolumeCreate,
			},
			{
				Name:        "get",
				ArgsUsage:   "[tenant name] [volume name]",
				Usage:       "Get JSON configuration for a volume",
				Description: "Obtain the JSON configuration for the volume",
				Action:      VolumeGet,
			},
			{
				Name:        "list",
				ArgsUsage:   "[tenant name]",
				Description: "Given a tenant name, produces a newline-delimited list of volumes.",
				Usage:       "List all volumes for a given tenant",
				Action:      VolumeList,
			},
			{
				Name:        "list-all",
				ArgsUsage:   "",
				Description: "Produces a newline-delimited list of tenant/volume combinations.",
				Usage:       "List all volumes across tenants",
				Action:      VolumeListAll,
			},
			{
				Name:        "force-remove",
				ArgsUsage:   "[tenant name] [volume name]",
				Description: "Forcefully removes a volume without deleting or unmounting the underlying image",
				Usage:       "Forcefully remove a volume without removing the underlying image",
				Action:      VolumeForceRemove,
			},
			{
				Name:        "remove",
				ArgsUsage:   "[tenant name] [volume name]",
				Description: "Remove the volume for a tenant, deleting its contents.",
				Usage:       "Remove a volume and its contents",
				Flags:       VolmasterFlags,
				Action:      VolumeRemove,
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
				Action:      MountList,
			},
			{
				Name:        "get",
				Usage:       "Get mount info",
				Description: "Obtains the information on a specified mount. Requires that you know the pool and image name.",
				ArgsUsage:   "[pool name] [volume name]",
				Action:      MountGet,
			},
			{
				Name:        "force-remove",
				ArgsUsage:   "[pool name] [volume name]",
				Usage:       "Forcefully remove mount information",
				Description: "Force-remove a mount. Use this to correct unmounting errors or failing hosts if necessary. Requires that you know the pool and image name.",
				Action:      MountForceRemove,
			},
		},
	},
}
