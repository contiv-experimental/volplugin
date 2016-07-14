#!/bin/bash

set -e

docker rm -f apiserver volplugin volsupervisor &>/dev/null || :

## test for shared mount capability
if ! docker run -it -v /mnt:/mnt:shared alpine true &>/dev/null
then
  echo 1>&2 "Docker cannot run volplugin in its current state."
  echo 1>&2 "You must change docker's systemd MountFlags to equal 'shared'"
  echo 1>&2 'Then: `systemctl daemon-reload` and `systemctl restart docker`'
  echo 1>&2 "Otherwise volplugin will not be able to run effectively."
  exit 1
fi

set -x

docker run --net host --name apiserver \
  --privileged -it -d \
  -v /dev:/dev \
  -v /etc/ceph:/etc/ceph \
  -v /var/lib/ceph:/var/lib/ceph \
  -v /lib/modules:/lib/modules:ro \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /mnt:/mnt:shared \
  contiv/volplugin apiserver

set +x

sleep 1

if [ ! -n "${NO_UPLOAD}" ]
then
  set -x
  docker exec -i apiserver volcli policy upload policy1 < /policy.json
  docker exec -i apiserver volcli global upload < /global.json
fi

set -x

docker run --net host --name volsupervisor \
  -itd --privileged \
  -v /lib/modules:/lib/modules:ro \
  -v /etc/ceph:/etc/ceph \
  -v /var/lib/ceph:/var/lib/ceph \
  contiv/volplugin volsupervisor

set +x
sleep 1
set -x

docker run --net host --name volplugin \
  --privileged -it -d \
  -v /dev:/dev \
  -v /lib/modules:/lib/modules:ro \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /run/docker/plugins:/run/docker/plugins \
  -v /mnt:/mnt:shared \
  -v /etc/ceph:/etc/ceph \
  -v /var/lib/ceph:/var/lib/ceph \
  contiv/volplugin volplugin
