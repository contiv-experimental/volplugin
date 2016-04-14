#! /bin/bash

set -e

#set GOPATH in CI environment
if [ "x${WORKSPACE}" != "x" ]; then
    export GOPATH=${WORKSPACE}
fi

echo running ceph-driver tests...
godep go test -v -timeout 240m ./systemtests -check.v

echo running test-driver tests...
USE_DRIVER="test" godep go test -v -timeout 240m ./systemtests -check.v -check.f systemtestSuite.TestVolCLI*
