#!/bin/sh

set -e

help() {
cat <<EOF
$(basename $0): <version> <release notes file> <tarball>

Release a version of volplugin.
EOF

  exit 1
}

if [ -z "$1" -o -z "$2" -o -z "$3" ]
then
  help
fi

if [ -z "$GITHUB_TOKEN" ]
then
  echo 1>&2 "Github token is missing; please set GITHUB_TOKEN"
  exit 1
fi

echo "Retrieving github-release"
GOPATH=$HOME go get -u github.com/aktau/github-release
PATH=$HOME/bin:$PATH

echo "Pushing release tag for version $1"
git push git@github.com:contiv/volplugin $1

set -x
github-release release -u contiv -r volplugin --tag $1 --name "Contiv Storage release $1" --description "$(cat $2)"
github-release upload -u contiv -r volplugin --tag $1 --name "64-bit Linux release $1" --file $3

docker push contiv/volplugin:$1
docker push contiv/volplugin-autorun:$1
