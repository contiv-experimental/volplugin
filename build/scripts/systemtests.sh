#! /bin/bash

set -e

if [ ! -n "${USE_DRIVER}" ]
then
  export USE_DRIVER=ceph
fi

echo running ${USE_DRIVER}-driver tests...
go test -v -timeout 240m ./systemtests -check.v -check.f "${TESTRUN}"
