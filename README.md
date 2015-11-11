<a href="http://51633006.ngrok.com/job/volplugin_CI/lastBuild/"><img src="http://51633006.ngrok.com/buildStatus/icon?job=volplugin_CI" /></a>

# volplugin: cluster-wide ceph volume management for container ecosystems

**Note:** we have extended documentation for users
[here](http://volplugin-docs.s3-website-us-west-1.amazonaws.com/)

volplugin controls [Ceph](http://ceph.com/) RBD devices with a master/slave
model to orchestrate the mounting (and cross-host remounting) of volumes
scheduled with containers. You can control docker to mount these volumes with a
plugin, or you can (soon) use schedulers, as well as docker-compose to manage
your application alongside Ceph volumes.

volplugin currently only supports Docker volume plugins. First class scheduler support for:
[Kubernetes](https://github.com/kubernetes/kubernetes), [Swarm](https://github.com/docker/swarm),
and [Mesos](http://mesos.apache.org/) will be available before the first stable release.

The master/slave model allows us to support a number of features, such as:

* On-the-fly image creation and (re)mount from any Ceph source, by referencing
  a tenant and volume name.
* Multiple pool management
* Manage many kinds of filesystems, including providing mkfs commands.
* Snapshot frequency and pruning
* Ephemeral (removed on container teardown) volumes
* IOPS limiting (via blkio cgroup)

Currently planned, but unfinished features:

* Backup management (via shell commands/scripts with parameters)

volplugin is still alpha at the time of this writing; features and the API may
be extremely volatile and it is not suggested that you use this in production.

## Try it out

### Prerequisites:

On the host, equivalent or greater:

* [VirtualBox](https://virtualbox.org) 5.0.2 or greater
* [Vagrant](https://vagrantup.com) 1.7.4
* [Ansible](https://ansible.com) 1.9.2
  * install with pip; you'll want to install `python-pip` and `python-dev` on
    ubuntu machines, then `sudo pip install ansible`. `brew install ansible`
    should do the right thing on OS X.
  * The make tooling in this repository will install it for you if it is not
    already installed, which requires `pip`. If you are not root, it may fail
    to perform this operation. The solution to this problem is to install
    ansible independently as described above.
* [Go](https://golang.org) to run the system tests.

Your guests will configure themselves.

### Running the processes

Be sure to start and run the environment with `make start` before you
continue with these steps. You must have working vagrant, virtualbox, and
ansible.

These instructions ssh you into the `mon0` vm. If you wish to test the
cross-host functionality, ssh into `mon1` or `mon2` with `vagrant ssh`. Then
start at the "starting volplugin" (not volmaster) step.

1. Run the suite: `make run`.
1. SSH into the host: `make ssh`.
1. Upload tenant information: `volcli tenant upload tenant1 < /testdata/intent1.json`
1. Add a docker volume with `pool/name` syntax:
  * `docker volume create -d volplugin --name tenant1/foo`
1. Run a container with the volume attached:
  * `docker run -it -v tenant1/foo:/mnt ubuntu bash`
1. You should have a volume mounted at `/mnt`, pointing at a `/dev/rbd#`
   device. Exit the shell to unmount the device.

To use the volume again, either `docker volume create` it on another host and
start a container, or just do it again with a different container on the same
host. Your data will be there!

`volcli` has many applications including volume and mount management. Check it
out!

## Development Instructions 

Please read the `Makefile` for most targets. If you `make build` you will get
volmaster/volplugin/volcli installed on the guests, so `make run-build` if you
want a `go install`'d version of these programs on your host.
volmaster/volplugin **do not** run on anything but linux (you can use volcli,
however, on other platforms).

If you wish to run the tests, `make test`. The unit tests (`make unit-test`)
live throughout the codebase as `*_test` files. The system tests / integration
tests (`make system-test`) live in the `systemtests` directory.
