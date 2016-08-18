#!/bin/bash

grep insecure-registry /usr/lib/systemd/system/docker.service &>/dev/null || sed -i 's/ExecStart=.*/& --insecure-registry=contiv-reg:5000/g' /usr/lib/systemd/system/docker.service && systemctl daemon-reload && systemctl restart docker
