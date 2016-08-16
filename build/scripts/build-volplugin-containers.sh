#!/bin/bash

# In order for the new binaries to take effect we need to clean the older environment
# Otherwise the binaries stick to older container images
# repeated docker builds also lead to dangling images, which need to be cleaned too.
docker ps -a | grep -e volplugin -e apiserver -e volsupervisor | awk '{print $1}' | xargs docker rm -fv
docker rmi contiv/volplugin
docker rmi $(docker images -f "dangling=true" -q)

docker build -t contiv/volplugin .

# Ensure that docker is running with MountFlags=shared
sudo sed -i 's/MountFlags=slave/MountFlags=shared/g' /usr/lib/systemd/system/docker.service
sudo systemctl daemon-reload
sudo systemctl restart docker
sleep 5

