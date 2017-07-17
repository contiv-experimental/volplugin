#!/bin/bash

set -ex

if [ ! -x /opt/golang/bin/mc ]
then
  echo downloading minio client
  sudo curl -sSL -o /opt/golang/bin/mc https://dl.minio.io/client/mc/release/linux-amd64/mc
  sudo chmod +x /opt/golang/bin/mc
fi 

export DOCKER_HOST=tcp://localhost:2375

INTERFACE=enp0s8

docker rm -f minio || :

volcli volume remove policy1/minio || :

# configure a basic nfs mount that will be available on the first host
sudo mkdir -p /volplugin
echo '/volplugin *(rw,no_root_squash)' | sudo tee /etc/exports.d/basic.exports

# reap the ip address, this will be important later.
ip=$(tfip2 | grep ${INTERFACE}: | awk '{ print $3 }' | awk '{ print $1 }')

# create our volume. the mount= parameter tells it what to mount.
volcli volume create --opt mount=${ip}:/volplugin policy1/minio

docker run -it -p 9000:9000 -v policy1/minio:/export minio/minio server /export
