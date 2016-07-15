## Contiv Storage Release <version>

Contiv Storage is the all-singing, all-dancing volumes system for Swarm and
Mesos. It provides a locking and policies subsystem for all volume types
including Ceph and NFS. The locking subsystem is used to ensure that two of the
same volume cannot be scheduled at the same time, while the policies system is
used to enforce volume compliance. Contiv Storage is best suited to large
clusters and labs.

This beta release version of Contiv Storage brings the latest features and
bugfixes.

<release notes>

You can get volplugin below, or if you wish to run it in containers you can try
the [contiv/volplugin-autorun](https://hub.docker.com/r/contiv/volplugin-autorun) image:

```
docker run -it -v /var/run/docker.sock:/var/run/docker.sock contiv/volplugin-autorun:<version>
```

Note that you *must* have an existing ceph installation for this to work!

If you just wish to configure volplugin itself, you can pull
[contiv/volplugin](https://hub.docker.com/r/contiv/volplugin). It contains
volcli, volplugin, apiserver, and volsupervisor and is tagged by release
version.

```
docker pull contiv/volplugin:<version>
```
