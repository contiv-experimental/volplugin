#! /bin/bash

set -e

#set GOPATH in CI environment
if [ "x${WORKSPACE}" != "x" ]; then
    export GOPATH=${WORKSPACE}
fi

for i in ceph nfs
do
  echo running ${i}-driver tests...
  USE_DRIVER="${i}" go test -v -timeout 240m ./systemtests -check.v -check.f "${TESTRUN}"
done
