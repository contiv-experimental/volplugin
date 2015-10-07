# volcli Reference

`volcli` controls the `volmaster`, which in turn is referenced by the
`volplugin` for local management of storage. Think of volcli as a tap into the
control plane.

## Top-Level Commands

These four commands present CRUD options on their respective sub-sections:

* `volcli tenant` manipulates tenant configuration
* `volcli volume` manipulates volumes. 
* `volcli mount` manipulates mounts.
* `volcli help` prints the help.
  * Note that for each subcommand, `volcli help [subcommand]` will print the
    help for that command. For multi-level commands, `volcli [subcommand] help
    [subcommand]` will work. Appending `--help` to any command will print the
    help as well.

## Tenant Commands

Typing `volcli tenant` without arguments will print help for these commands.

* `volcli tenant upload` takes a tenant name, and JSON configuration from standard input.
* `volcli tenant delete` removes a tenant. Its volumes and mounts will not be removed.
* `volcli tenant get` displays the JSON configuration for a tenant.
* `volcli tenant list` lists the tenants etcd knows about.

## Volume Commands

Typing `volcli volume` without arguments will print help for these commands.

* `volcli volume create` will forcefully create a volume just like it was created with
  `docker volume create`. Requires a tenant, and volume name.
* `volcli volume get` will retrieve the volume configuration for a given tenant/volume combination.
* `volcli volume list` will list all the volumes for a provided tenant.
* `volcli volume list-all` will list all volumes, across tenants.
* `volcli volume remove` will remove a volume given a tenant/volume
  combination, deleting the underlying data.  This operation may fail if the
  device is mounted, or expected to be mounted.
* `volcli volume force-remove`, given a tenant/volume combination, will remove
  the data from etcd but not perform any other operations. Use this option with
  caution.

## Mount Commands

Typing `volcli mount` without arguments will print help for these commands.

**Note:** `volcli mount` cannot control mounts -- this is managed by
`volplugin` which lives on each host. Eventually there will be support for
pushing operations down to the volplugin, but not yet.

* `volcli mount list` lists all known mounts in etcd.
* `volcli mount get` obtains specific information about a mount from etcd.
* `volcli mount force-remove` removes the contents from etcd, but does not
  attempt to perform any unmounting. This is useful for removing mounts that
  for some reason (e.g., host failure, which is not currently satsified by
  volplugin)
