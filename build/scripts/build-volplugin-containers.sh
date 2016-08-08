#!/bin/bash
docker build -t contiv/volplugin .

# Ensure that docker is running with MountFlags=shared
sudo sed -i 's/MountFlags=slave/MountFlags=shared/g' /usr/lib/systemd/system/docker.service
sudo systemctl daemon-reload
sudo systemctl restart docker
sleep 5

