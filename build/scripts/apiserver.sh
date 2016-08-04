#!/bin/bash
usage="$0 start|stop"
if [ $# -ne 1  ]; then
    echo USAGE: $usage
    exit 1
fi

case $1 in
start)
    # clean up any running instances
    count=$(docker ps -a | grep apiserver | wc -l)
    if [ "${count}" != 0 ]; then
        docker ps -a | grep apiserver | awk '{print $1}' | xargs docker rm -f
    fi
    rm -f /tmp/apiserver-fifo

    set -e
    echo starting apiserver
    docker run --rm -i --name contiv_apiserver \
        -v /var/run/docker.sock:/var/run/docker.sock \
        contiv/volplugin-apiserver

    # now just sleep to keep the service up
    mkfifo "/tmp/apiserver-fifo"
    < "/tmp/apiserver-fifo"
    ;;

stop)
    echo stopping apiserver
    count=$(docker ps -a | grep apiserver | wc -l)
    if [ "${count}" != 0 ]; then
        docker ps -a | grep apiserver | awk '{print $1}' | xargs docker rm -f
    fi
    rm -f /tmp/apiserver-fifo

    ;;

*)
    echo USAGE: $usage
    exit 1
    ;;

esac


