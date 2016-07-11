#!/bin/sh	

out=$(find . -name '*.go' -type f | xargs gofmt -l -s | grep -v vendor)

if [ $(echo -n "${out}" | wc -l) != 0 ]
then
  echo 2>&1 "gofmt errors in:"
  echo 2>&1 "${out}"
  exit 1
fi
