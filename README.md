## volplugin: cluster-wide volume management for container ecosystems

volplugin uses a master/slave model to dynamically mount and maintain ceph RBD
devices. It is still very alpha as of this writing.

### Prerequisites:

On the host, equivalent or greater:

* VirtualBox 5.0.2 or greater
* Vagrant 1.7.4
* Ansible 1.9.2
  * install with pip; you'll want to install `python-pip` and `python-dev` on
    ubuntu machines, then `sudo pip install ansible`.
  * The make tooling in this repository will install it for you if it is not
    already installed. If you are not root, it may fail to perform this
    operation. The solution to this problem is to install ansible
    independently as described above.
* build-essential
* golang 1.4.x

Your guests will configure themselves.

### Usage instructions

Be sure to start the environment with `make start` before you continue with
these steps. You must have working vagrant, virtualbox, and ansible.

You will also want to `make ssh` to ssh into the `mon0` VM to follow along.

1. Start the volmaster and volplugin: `make run`. This will hang until you hit
   ^C which will stop the started processes.
1. Start a container with `make container`, or specify one manually:
 * `docker run -it -v <volname>:<volpath> --volume-driver tenant1 <image> <command>`
   * Example: `docker run -it -v tmp:/mnt --volume-driver tenant1 ubuntu bash`
1. You should have a volume mounted at your path, pointing at a `/dev/rbd#`
   device. Exit the shell to unmap the device.

### Build Instructions

```
# builds and provisions VMs
$ make start

# tears down VMs.
$ make stop

# provisions VMs with ansible
$ make provision

# ssh into the monitor host for volplugin testing
$ make ssh

# build the binaries in the guest
$ make build

# install ansible on the host (required for vagrant)
$ make install-ansible

# run the unit tests
$ make test

# start the volplugin on the monitor host and hang (for logging)
$ make run-volplugin

# start the volplugin on the local host
$ make volplugin-start

# start the volmaster on the monitor host and hang (for logging)
$ make run-volmaster

# start the volmaster on the local host
$ make volmaster-start
```
