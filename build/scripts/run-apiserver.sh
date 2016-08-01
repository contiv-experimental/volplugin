#!/bin/bash
docker run -it -v /var/run/docker.sock:/var/run/docker.sock contiv/volplugin-apiserver
