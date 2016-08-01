#!/bin/bash
usage="$0 start|stop"
if [ $# -ne 1  ]; then
    echo USAGE: $usage
    exit 1
fi

case $1 in
start)
    # Cleanup any existing instances
    count=$(docker ps -a | grep volplugin | grep -v apiserver | grep -v volsupervisor | wc -l)
    if [ "${count}" != 0 ]; then
        docker ps -a | grep volplugin | grep -v apiserver | grep -v volsupervisor | awk '{print $1}' | xargs docker rm -f
    fi
    rm -f /tmp/volplugin-fifo

    set -e
    echo starting volplugin
    cont_id=$(docker run --rm -i --name contiv_volplugin \
        -v /var/run/docker.sock:/var/run/docker.sock \
        contiv/volplugin-volplugin)

    # now just sleep to keep the service up
    mkfifo "/tmp/volplugin-fifo"
    < "/tmp/volplugin-fifo"
    ;;

stop)
    echo stopping volplugin
    count=$(docker ps -a | grep volplugin | grep -v apiserver | grep -v volsupervisor | wc -l)
    if [ "${count}" != 0 ]; then
        docker ps -a | grep volplugin | grep -v apiserver | grep -v volsupervisor | awk '{print $1}' | xargs docker rm -f
    fi
    rm -f /tmp/volplugin-fifo
    ;;

*)
    echo USAGE: $usage
    exit 1
    ;;

esac


