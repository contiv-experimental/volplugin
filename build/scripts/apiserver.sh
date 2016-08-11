#!/bin/bash
usage="$0 start|stop|rm"
if [ $# -ne 1  ]; then
    echo USAGE: $usage
    exit 1
fi

case $1 in
start)
    rm -f /tmp/apiserver-fifo

    set -e
    echo starting apiserver
    /usr/bin/contiv-vol-run.sh apiserver

    # now just sleep to keep the service up
    mkfifo "/tmp/apiserver-fifo"
    < "/tmp/apiserver-fifo"
    ;;

stop)
    echo stopping apiserver
    rm -f /tmp/apiserver-fifo
    docker stop apiserver
    ;;

rm)
    echo removing apiserver container
    rm -f /tmp/apiserver-fifo
    docker stop apiserver
    docker rm -v apiserver
    ;;

*)
    echo USAGE: $usage
    exit 1
    ;;

esac
