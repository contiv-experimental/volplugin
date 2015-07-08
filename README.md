### Build Instructions

```
$ make start # builds and provisions VMs
$ make stop # tears down VMs.
$ make provision # provisions VMs with ansible
$ make ssh # ssh into the monitor host for volplugin testing
$ make build # build the binaries
$ make install-ansible # install ansible on the host (required for vagrant)
$ make test # run the unit tests
$ make volplugin # start the volplugin on the monitor host and hang (for logging)
$ make volplugin-start # start the volplugin on the local host
```
