#!/bin/bash

usage="$0 <ifname> <vip>"
if [ $# -ne 2 ]; then
    echo USAGE: $usage
    exit 1
fi

set -x -e

intf=$1

/sbin/ip link del dev ${intf}_0
