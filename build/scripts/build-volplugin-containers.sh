#!/bin/bash
docker build -t contiv/volplugin .
docker build -t contiv/volplugin-apiserver -f Dockerfile.apiserver .
docker build -t contiv/volplugin-volsupervisor -f Dockerfile.volsupervisor .
docker build -t contiv/volplugin-volplugin -f Dockerfile.volplugin .

# Ensure that docker is running with MountFlags=shared
sudo sed -i 's/MountFlags=slave/MountFlags=shared/g' /usr/lib/systemd/system/docker.service
sleep 1
sudo systemctl daemon-reload
sudo systemctl restart docker
sleep 2

