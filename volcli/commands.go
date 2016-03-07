package volcli

import "github.com/codegangsta/cli"

// VolmasterFlags contains the flags specific to volmasters
var VolmasterFlags = []cli.Flag{
	cli.StringFlag{
		Name:  "volmaster",
		Usage: "address of volmaster process",
		Value: "127.0.0.1:9005",
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
		Name:  "global",
		Usage: "Manage Global Configuration",
		Subcommands: []cli.Command{
			{
				Name:        "upload",
				ArgsUsage:   "accepts configuration from stdin",
				Description: "Uploads a the global configuration to etcd. Accepts JSON.",
				Usage:       "Upload a policy to etcd",
				Action:      GlobalUpload,
			},
			{
				Name:        "get",
				Flags:       VolmasterFlags,
				ArgsUsage:   "[policy name]",
				Usage:       "Obtain the global configuration",
				Description: "Gets the global configuration from etcd",
				Action:      GlobalGet,
			},
		},
	},
	{
		Name:  "policy",
		Usage: "Manage Policies",
		Subcommands: []cli.Command{
			{
				Name:        "upload",
				Flags:       VolmasterFlags,
				ArgsUsage:   "[policy name]. accepts from stdin",
				Description: "Uploads a policy to etcd. Accepts JSON. Requires direct, unauthenticated access to etcd.",
				Usage:       "Upload a policy to etcd",
				Action:      PolicyUpload,
			},
			{
				Name:        "delete",
				ArgsUsage:   "[policy name]",
				Description: "Permanently removes a policy from etcd. Volumes that belong to the policy are unaffected.",
				Usage:       "Delete a policy",
				Action:      PolicyDelete,
			},
			{
				Name:        "get",
				ArgsUsage:   "[policy name]",
				Usage:       "Obtain the policy",
				Description: "Gets the policy for a policy from etcd.",
				Action:      PolicyGet,
			},
			{
				Name:        "list",
				ArgsUsage:   "",
				Description: "Reads policies and generates a newline-delimited list",
				Usage:       "List all policies",
				Action:      PolicyList,
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
				ArgsUsage:   "[policy name]/[volume name]",
				Description: "This creates a logical volume. Calls out to the volmaster and sets the policy based on the policy name provided.",
				Usage:       "Create a volume for a given policy",
				Action:      VolumeCreate,
			},
			{
				Name:        "get",
				ArgsUsage:   "[policy name]/[volume name]",
				Usage:       "Get JSON configuration for a volume",
				Description: "Obtain the JSON configuration for the volume",
				Action:      VolumeGet,
			},
			{
				Name:        "list",
				ArgsUsage:   "[policy name]",
				Description: "Given a policy name, produces a newline-delimited list of volumes.",
				Usage:       "List all volumes for a given policy",
				Action:      VolumeList,
			},
			{
				Name:        "list-all",
				ArgsUsage:   "",
				Description: "Produces a newline-delimited list of policy/volume combinations.",
				Usage:       "List all volumes across policies",
				Action:      VolumeListAll,
			},
			{
				Name:        "force-remove",
				ArgsUsage:   "[policy name]/[volume name]",
				Description: "Forcefully removes a volume without deleting or unmounting the underlying image",
				Usage:       "Forcefully remove a volume without removing the underlying image",
				Action:      VolumeForceRemove,
			},
			{
				Name:        "remove",
				ArgsUsage:   "[policy name]/[volume name]",
				Description: "Remove the volume for a policy, deleting its contents.",
				Usage:       "Remove a volume and its contents",
				Flags:       VolmasterFlags,
				Action:      VolumeRemove,
			},
		},
	},
	{
		Name:  "use",
		Usage: "Manage Uses (hosts consuming resources)",
		Subcommands: []cli.Command{
			{
				Name:        "list",
				Usage:       "List uses",
				Description: "List the uses the volmaster knows about, in newline-delimited form.",
				ArgsUsage:   "",
				Action:      UseList,
			},
			{
				Name:        "get",
				Usage:       "Get use info",
				Description: "Obtains the information on a specified use. Requires that you know the policy and image name.",
				ArgsUsage:   "[policy name]/[volume name]",
				Action:      UseGet,
			},
			{
				Name:        "force-remove",
				ArgsUsage:   "[policy name]/[volume name]",
				Usage:       "Forcefully remove use information",
				Description: "Force-remove a use. Use this to correct unuseing errors or failing hosts if necessary. Requires that you know the policy and image name.",
				Action:      UseTheForce,
			},
		},
	},
}
