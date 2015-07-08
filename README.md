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

# build the binaries
$ make build

# install ansible on the host (required for vagrant)
$ make install-ansible

# run the unit tests
$ make test

# start the volplugin on the monitor host and hang (for logging)
$ make volplugin

# start the volplugin on the local host
$ make volplugin-start
```
