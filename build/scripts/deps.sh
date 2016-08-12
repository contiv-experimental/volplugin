#!/bin/sh

echo "Fetching connwait..."
[ -n "`which connwait`" ] || sudo -E $(which go) get github.com/erikh/connwait
