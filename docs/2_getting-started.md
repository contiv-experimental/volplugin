# Getting Started

## Clone and build the project

Please see the [prerequisites in the README](https://github.com/contiv/volplugin/blob/master/README.md#prerequisites)
before attempting these instructions.

### On Linux (without a VM):

Clone and build the project: 

* `git clone github.com/contiv/volplugin`
* `make run-build`
  * This will install some utilities for building the software in your
    `$GOPATH`, as well as the `volmaster`, `volplugin` and `volcli`
    utilities.

### Everywhere else (with a VM):

* `git clone github.com/contiv/volplugin`
* `make start build`

The build and each binary will be on the VM in `/opt/golang/bin`.

## Install Dependencies

* [etcd release notes and install instructions](https://github.com/coreos/etcd/releases/tag/v2.2.0)
  * We currently support versions 2.0 and up.
* [Ceph](http://docs.ceph.com/docs/master/start/)
  * If you have not installed Ceph before, a quick installation guide [is here](http://docs.ceph.com/docs/master/start/)
  * Ceph can be a complicated beast to install. If this is your first time
    using the project, please be aware there are pre-baked VMs that will work
    for you on any unix operating system. [See the README for more information](https://github.com/contiv/volplugin/blob/master/README.md#running-the-processes).

## Configure Services

Ensure ceph is fully operational, and that the `rbd` tool works as root.

1. Start etcd: `etcd &>/dev/null &`
1. Upload a tenant policy with `volcli tenant upload tenant1`. It accepts the
   policy from stdin.
   * You can find some examples of policy in
     [systemtests/testdata](https://github.com/contiv/volplugin/tree/master/systemtests/testdata).
   * If you just want a quick start without configuring it yourself: 
     * `cat systemtests/testdata/intent1.json | volcli tenant upload tenant1`
1. Start volmaster (as root): `volmaster &`
   * volmaster has a debug mode as well, but it's really noisy, so avoid using
     it with background processes. Volplugin currently runs on port 8080, but
     this will be variable in the future.
1. Start volplugin in debug mode (as root): `volplugin --debug &`
   * If you run volplugin on multiple hosts, you can use the `--master` flag to
     provide a ip:port pair to connect to over http. By default it connects to
     `127.0.0.1:8080`.

## Run Stuff!

Let's start a container with a volume.

1. Create a volume that refers to the volplugin driver:
   `docker volume create -d volplugin tenant1/test`
   * `test` is the name of your volume, and it lives under tenant `tenant1`,
     which you uploaded with `volcli tenant upload`
1. Run a container that uses it: `docker run -it -v tenant1/test:/mnt ubuntu bash`
1. Run `mount | grep /mnt` in the container, you should see the `/dev/rbd#`
   attached to that directory.
   * Once you have a multi-host setup, anytime the volume is not mounted, it
     can be mounted on any host that has a connected rbd client available, and
     volplugin running.

