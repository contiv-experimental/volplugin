# Architecture

"volplugin", despite the name, is actually a suite of components:

`volmaster` is the master process. It exists to coordinate the volplugins in a
way that safely manages container volumes. It talks to `etcd` to keep its
state.

`volplugin` is the slave process. It exists to bridge the state management
between `volmaster` and `docker`, and to mount volumes on specific hosts.

`volcli` is a utility for managing `volmaster`'s data. It makes both REST calls
into the volmaster and additionally can write directly to etcd.

## Organizational Architecture

`volmaster` is completely stateless, and can be run multi-host for redundancy.
`volmaster` needs both root permissions, and capability to manipulate RBD
images with the `rbd` tool.

`volsupervisor` handles scheduled and supervised tasks such as snapshotting. It
may only be deployed on one host at a time.

`volplugin` needs to run on every host that will be running containers. Upon
start, it will create a unix socket in the appropriate plugin path so that
docker recognizes it. This creates a driver named `volplugin`.

`volcli` is a management tool and can live anywhere that has access to the etcd
cluster and volmaster.

## Security Architecture

There is none currently. This is still an alpha, security will be a beta
target.

## Network Architecture

`volmaster`, by default, listens on `0.0.0.0:8080`. It provides a REST
interface to each of its operations that are used both by `volplugin` and
`volcli`. It connects to etcd at `127.0.0.1:2379`, which you can change by
supplying `--etcd` one or more times.

`volsupervisor` needs root, connections to etcd, and access to ceph `rbd` tools
as admin.

`volplugin` contacts the volmaster but listens on no network ports (it uses a
unix socket as described above, to talk to docker). It by default connects to
the volmaster at `127.0.0.1:8080` and must be supplied the `--master` switch to
talk to a remote `volmaster`.

`volcli` talks to both `volmaster` and `etcd` to communicate various state and
operations to the system.
