#!/bin/sh

set -e

if [ -z "$1" ]
then
  echo 1>&2 "Please supply a version"
  exit 1
fi

BUILD_VERSION=$1 make build

dir=/tmp/volplugin-${1}

rm -rf $dir
mkdir $dir
cp bin/volmigrate bin/apiserver bin/volcli bin/volplugin bin/volsupervisor $dir

cd /tmp
tar cvjf ${OLDPWD}/$(basename $dir).tar.bz2 volplugin-${1}
cd $OLDPWD

git tag $1

echo "Tag $1 has been created but not pushed; push it if you're sure you're ready to release!"
echo "Your tarball is in $PWD/$(basename $dir).tar.bz2!"
