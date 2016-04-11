#!/bin/bash

usage="$0 <ifname> <vip>"
if [ $# -ne 2 ]; then
    echo USAGE: $usage
    exit 1
fi

set -x -e

intf=$1
vip=$2
    
/sbin/ip link add name ${intf}_0 type dummy

/sbin/ip addr add ${vip} dev ${intf}_0

/sbin/ip link set dev ${intf}_0 up
