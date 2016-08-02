[![Build-Status][Build-Status-Image]][Build-Status-URL] [![ReportCard][ReportCard-Image]][ReportCard-URL]

# volplugin: cluster-wide volume management for container ecosystems

**Note**: Most of this documentation is about the open source project. If you
came to try Contiv Storage, [read our documentation](http://contiv.github.io/documents/storage/index.html).

volplugin controls [Ceph](http://ceph.com/) RBD or NFS devices, in a way that
makes them easy to use for devs with docker, and flexible to configure for ops.
Reference your volumes with docker from anywhere your storage is available, and
they are located and mounted. Works great with [Compose](https://github.com/docker/compose) and
[Swarm](https://github.com/docker/swarm), now [Mesos](https://www.mesosphere.com) too!

Our profiles system makes instantiating lots of similar class volumes a snap,
allowing for a variety of use cases:

* Give your dev teams full-stack dev environments (complete with state) that
  arrive on demand. They can configure them.
* Scale your stateful containers in a snap with our snapshot facilities, just
  `volcli volume snapshot copy` and refer to the volume immediately. Anywhere. (Ceph only)
* Container crashed? Host died? volplugin's got you. Just re-init your
  container on another host with the same volume name.

volplugin currently only supports Docker volume plugins. First class scheduler support for:
[Kubernetes](https://github.com/kubernetes/kubernetes) and
[Mesos](http://mesos.apache.org/) will be available before the first stable
release.

* On-the-fly image creation and (re)mount from any Ceph source, by referencing
  a policy and volume name.
* Manage many kinds of filesystems, including providing mkfs commands.
* Snapshot frequency and pruning. Also copy snapshots to new volumes!
* Ephemeral (removed on container teardown) volumes
* BPS limiting (via blkio cgroup)

volplugin is still alpha at the time of this writing; features and the API may
be extremely volatile and it is not suggested that you use this in production.

## Try it out

This will start the suite of volplugin tools in containers from the
`contiv/volplugin` image. It will do the work of configuring docker for you.  Note that you must have
a working ceph environment that volplugin can already use. If not, please refer
to the [development instructions](http://contiv.github.io/documents/gettingStarted/storage/storage.html)
for how you can build one.


```
$ docker run -it -v /var/run/docker.sock:/var/run/docker.sock contiv/volplugin-autorun
```

If you get an error like "mountpoint / is not a shared mount", set
`MountFlags=shared` in your systemd unit file for docker. It will most likely
be set to `slave` instead.

## Development Instructions 

Our [Getting Started instructions](http://contiv.github.io/documents/gettingStarted/storage/storage.html)
should be the first thing you read. The prerequisites are absolutely necessary.

Please see our [CONTRIBUTING](https://github.com/contiv/volplugin/blob/master/CONTRIBUTING.md)
document as well.

Please read the `Makefile` for most targets. If you `make build` you will get
apiserver/volplugin/volcli installed on the guests, so `make run-build` if you
want a `go install`'d version of these programs on your host.
apiserver/volplugin **do not** run on anything but linux (you can use volcli,
however, on other platforms).

`make start` will start the development environment. `make stop` stops, and
`make restart` rebuilds it.

If you wish to run the tests, `make test`. The unit tests (`make unit-test`)
live throughout the codebase as `*_test` files. The system tests / integration
tests (`make system-test`) live in the `systemtests` directory.  Note that `make system-test`
**will not** successfully run on OSX due to dependencies on unavailable libraries.

[ReportCard-URL]: https://goreportcard.com/report/github.com/contiv/volplugin
[ReportCard-Image]: https://goreportcard.com/badge/github.com/contiv/volplugin
[Build-Status-URL]: http://contiv.ngrok.io/job/Volplugin%20Push%20Build%20Master
[Build-Status-Image]: http://contiv.ngrok.io/buildStatus/icon?job=Volplugin%20Push%20Build%20Master
