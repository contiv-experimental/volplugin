<a href="http://1cea435f.ngrok.com/job/volplugin_CI/lastBuild/"><img src="http://1cea435f.ngrok.com/buildStatus/icon?job=volplugin_CI" /></a>

## volplugin: cluster-wide volume management for container ecosystems

volplugin uses a master/slave model to dynamically mount and maintain ceph RBD
images for containers, and manages actions to take on those images, such as
snapshots and filesystem types. It is still very alpha as of this writing.

### Prerequisites:

On the host, equivalent or greater:

* VirtualBox 5.0.2 or greater
* Vagrant 1.7.4
* Ansible 1.9.2
  * install with pip; you'll want to install `python-pip` and `python-dev` on
    ubuntu machines, then `sudo pip install ansible`. `brew install ansible`
    should do the right thing on OS X.
  * The make tooling in this repository will install it for you if it is not
    already installed, which requires `pip`. If you are not root, it may fail
    to perform this operation. The solution to this problem is to install
    ansible independently as described above.
* golang to run the system tests

Your guests will configure themselves.

### Usage instructions

Be sure to start the environment with `make start` before you continue with
these steps. You must have working vagrant, virtualbox, and ansible.

You will also want to `make ssh` to ssh into the `mon0` VM to follow along.
Eventually, this will be scripted/supervised and no longer necessary.

If you wish to test the cross-host functionality, ssh into `mon1` or `mon2`
with `vagrant ssh`. Then start at the "starting volplugin" (not volmaster)
step.

1. Start etcd: `etcd &>/dev/null &`
1. Upload tenant information: `volcli tenant upload tenant1 < /testdata/intent1.json`
1. Start the volmaster and volplugin:
  * <code>sudo \`which volmaster\` &</code>
  * <code>sudo \`which volplugin\` --master 192.168.24.10:8080 tenant1 &</code>
1. Add a docker volume with `pool/name` syntax:
  * `docker volume create -d tenant1 --name rbd/foo`
1. Run a container with the volume attached:
  * `docker run -it -v rbd/foo:/mnt ubuntu bash`
1. You should have a volume mounted at `/mnt`, pointing at a `/dev/rbd#`
   device. Exit the shell to unmount the device.

`volcli` has many applications including volume and mount management. Check it
out!

### Development Instructions 

Please read the `Makefile` for most targets. If you `make build` you will get
volmaster/volplugin/volcli installed on the guests, so `make run-build` if you
want a `go install`'d version of these programs on your host.

If you wish to run the tests, `make test`. The unit tests (`make unit-test`)
live throughout the codebase as `*_test` files. The system tests / integration
tests (`make system-test`) live in the `systemtests` directory.
