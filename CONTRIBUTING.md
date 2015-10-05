# Contributing to volplugin

Patches and feature additions are totally welcome to volplugin! For now, please
see the [README](https://github.com/contiv/volplugin/blob/master/README.md) for
build and test instructions.

If you have a patch to contribute to volplugin, please follow these guidelines:

* Sign your commits with `git commit -s`. Note that we live under an Apache
  license.
* Your comments should detail the directory they are affecting, as below. This
  makes it much simpler than `git log --stat` to determine what areas of a
  project a specific commit is referencing.
  * Example: `librbd: changed a widget`
* Please squash your commits per fix/feature, with a single commit message.

If you have trouble with the appropriate git commands to handle these
requirements, please let us know! We're happy to help.

## Environment

We currently test against the docker [master branch](https://master.dockerproject.org)
and this will be downloaded on `make start` to be injected into the VMs by ansible.

## Note about VMs

Unfortunately, as of this writing, due to security concerns we do not publish
the source to tools to build the VM images. These are built by us and can be
found on [Atlas](https://atlas.hashicorp.com/contiv/boxes/centos71-netplugin) for
auditing if you have a concern. 

## System Tests

Our system tests are an integration suite that runs from the host, unlike the
unit tests which run on the guest(s). As a result system tests have unique
requirements. The system tests also are responsible for setting up volplugin,
volmaster, etcd and other dependencies.

* Must run on all (reasonably supported) platforms for development, e.g.:
  * Mac OS X
  * Linux (ubuntu and centos)
  * Windows *should* work :)
* Each test is responsible for the test setup and teardown of each resource
  * see the `rebootstrap()` function in the system tests (and the tests
    themselves for usage explanations).
  * there are also numerous utility functions in [systemtests/util_test.go](https://github.com/contiv/volplugin/blob/master/systemtests/util_test.go)
    which should be used in any test.
