#!/bin/sh

echo "Fetching connwait..."
[ -n "`which connwait`" ] || sudo -E $(which go) get github.com/erikh/connwait

echo "Fetching tfip2"
[ -n "`which tfip2`" ] || sudo -E $(which go) get github.com/erikh/tfip2
