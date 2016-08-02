#!/bin/bash
set -e
set -x

cd /tmp/
TEMP_GOPATH=/tmp/$(mktemp -d tempgopath.XXXXXXXXX)
mkdir -p $TEMP_GOPATH/src/github.com/contiv/volplugin
tar -c --exclude "*.vdi" --exclude subnet_assignment.state $GOPATH/src/github.com/contiv/volplugin | tar -x --strip-components=5 -C $TEMP_GOPATH/src/github.com/contiv/
export GOPATH=$TEMP_GOPATH
cd $GOPATH/src/github.com/contiv/volplugin
go get github.com/tools/godep
$GOPATH/bin/godep restore -v
rm -rf $TEMP_GOPATH
echo "vendored deps were restored"
