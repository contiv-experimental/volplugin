#!/bin/bash
usage="$0 start|stop"
if [ $# -ne 1  ]; then
    echo USAGE: $usage
    exit 1
fi

case $1 in
start)
    rm -f /tmp/volplugin-fifo

    set -e
    echo starting volplugin
    /usr/bin/contiv-vol-run.sh volplugin

    # now just sleep to keep the service up
    mkfifo "/tmp/volplugin-fifo"
    < "/tmp/volplugin-fifo"
    ;;

stop)
    echo stopping volplugin
    rm -f /tmp/volplugin-fifo
    docker rm -f volplugin
    ;;

*)
    echo USAGE: $usage
    exit 1
    ;;

esac


