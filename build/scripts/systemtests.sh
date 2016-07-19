#! /bin/bash

set -e

if [ ! -n "${USE_DRIVER}" ]
then
  export USE_DRIVER=ceph
fi

#set GOPATH in CI environment
if [ "x${WORKSPACE}" != "x" ]; then
    export GOPATH=${WORKSPACE}
fi

for i in ceph nfs
do
  echo running ${USE_DRIVER}-driver tests...
  go test -v -timeout 240m ./systemtests -check.v -check.f "${TESTRUN}"
done
