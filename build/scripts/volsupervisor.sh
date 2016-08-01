#!/bin/bash
usage="$0 start|stop"
if [ $# -ne 1  ]; then
    echo USAGE: $usage
    exit 1
fi

case $1 in
start)

    # clean up any running instances
    count=$(docker ps -a | grep volsupervisor | wc -l) 
    if [ "${count}" != 0 ]; then
        docker ps -a | grep volsupervisor | awk '{print $1}' | xargs docker rm -f
    fi
    rm -f /tmp/volsupervisor-fifo

    set -e
    echo starting volsupervisor
    cont_id=$(docker run --rm -i --name contiv_volsupervisor \
        -v /var/run/docker.sock:/var/run/docker.sock \
        contiv/volplugin-volsupervisor)

    # now just sleep to keep the service up
    mkfifo "/tmp/volsupervisor-fifo"
    < "/tmp/volsupervisor-fifo"
    ;;

stop)
    echo stopping volsupervisor
    rm -f /tmp/volsupervisor-fifo
    count=$(docker ps -a | grep volsupervisor | wc -l) 
    if [ "${count}" != 0 ]; then
        docker ps -a | grep volsupervisor | awk '{print $1}' | xargs docker rm -f
    fi

    ;;

*)
    echo USAGE: $usage
    exit 1
    ;;

esac


