#!/bin/sh

set -e

if [ ! -n "`which ansible`" ]
then
cat <<'EOF'
Please install ansible: you can do this in a number of ways:

* OS X: Install homebrew from https://brew.sh and `brew install ansible`
* Debian/Ubuntu: `apt-get install ansible -y`
* CentOS: `yum install ansible -y`
EOF
  exit 1
fi
