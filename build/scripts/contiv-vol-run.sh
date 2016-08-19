#!/bin/bash

if [ -f /tmp/contiv-registry ]; then
	registry=$(cat /tmp/contiv-registry)
fi

set -e

# Check if the container already exists
if docker inspect -f {{.State.Running}} $1 &>/dev/null
then
    # if the container is already running -> return
    if docker inspect -f {{.State.Running}} $1 | grep "true" &>/dev/null
    then
        echo $1 "container is already running"
        exit 0
    fi

    # if the container is in stopped state -> `docker start` it
    if docker inspect -f {{.State.Running}} $1 | grep "false" &>/dev/null
    then
        echo $1 "container exists. Restarting it."
        docker start $1
        exit 0
    fi
fi


## test for shared mount capability
if ! grep "MountFlags" /lib/systemd/system/docker.service | grep shared &>/dev/null
then
  echo 1>&2 "Docker cannot run volplugin in its current state."
  echo 1>&2 "You must change docker's systemd MountFlags to equal 'shared'"
  echo 1>&2 'Then: `systemctl daemon-reload` and `systemctl restart docker`'
  echo 1>&2 "Otherwise volplugin will not be able to run effectively."
  exit 1
fi

set -x

case "$1" in
"apiserver" )
    docker run --net host --name apiserver \
        --privileged -i -d \
        -v /dev:/dev \
        -v /etc/ceph:/etc/ceph \
        -v /var/lib/ceph:/var/lib/ceph \
        -v /lib/modules:/lib/modules:ro \
        -v /var/run/docker.sock:/var/run/docker.sock \
        -v /mnt:/mnt:shared \
        ${registry}contiv/volplugin apiserver
    ;;

"volsupervisor" )
    docker run --net host --name volsupervisor \
        -id --privileged \
        -v /lib/modules:/lib/modules:ro \
        -v /etc/ceph:/etc/ceph \
        -v /var/lib/ceph:/var/lib/ceph \
        ${registry}contiv/volplugin volsupervisor
    ;;

"volplugin" )
    docker run --net host --name volplugin \
        --privileged -i -d \
        -v /dev:/dev \
        -v /lib/modules:/lib/modules:ro \
        -v /var/run/docker.sock:/var/run/docker.sock \
        -v /run/docker/plugins:/run/docker/plugins \
        -v /mnt:/mnt:shared \
        -v /etc/ceph:/etc/ceph \
        -v /var/lib/ceph:/var/lib/ceph \
        -v /var/run/ceph:/var/run/ceph \
        -v /sys/fs/cgroup:/sys/fs/cgroup \
        ${registry}contiv/volplugin volplugin
    ;;

* )
    echo "Unknown option:" "$1"
    ;;
esac
