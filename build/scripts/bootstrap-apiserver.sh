#!/bin/bash

set -e

docker rm -f apiserver &>/dev/null || :

## test for shared mount capability
if ! docker run -i -v /mnt:/mnt:shared alpine true &>/dev/null
then
    echo 1>&2 "Docker cannot run volplugin in its current state."
    echo 1>&2 "You must change docker's systemd MountFlags to equal 'shared'"
    echo 1>&2 'Then: `systemctl daemon-reload` and `systemctl restart docker`'
    echo 1>&2 "Otherwise volplugin will not be able to run effectively."
    exit 1
fi

set -x

docker run --net host --name apiserver \
    --privileged -i -d \
    -v /dev:/dev \
    -v /etc/ceph:/etc/ceph \
    -v /var/lib/ceph:/var/lib/ceph \
    -v /var/run/ceph:/var/run/ceph \
    -v /lib/modules:/lib/modules:ro \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v /mnt:/mnt:shared \
    contiv/volplugin apiserver
