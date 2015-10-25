start: download-docker install-ansible
	vagrant up
	make build

stop:
	vagrant destroy -f

update:
	vagrant box update

restart: stop update download-docker start

provision: download-docker
	vagrant provision

ssh:
	vagrant ssh mon0

golint-host:
	[ -n "`which golint`" ] || go get github.com/golang/lint/golint
	golint ./...

golint:
	vagrant ssh mon0 -c "sudo -i sh -c 'cd /opt/golang/src/github.com/contiv/volplugin; http_proxy=${http_proxy} https_proxy=${https_proxy} make golint-host'"

download-docker:
	curl https://master.dockerproject.org/linux/amd64/docker -o ansible/docker

install-ansible:
	[ -n "`which ansible`" ] || pip install ansible

ci:
	GOPATH=/tmp/volplugin:/var/jenkins/workspace/volplugin_CI PATH="/tmp/volplugin/bin:/usr/local/go/bin:${PATH}" make test

godep:
	[ -n "`which godep`" ] || go get github.com/kr/godep

test: godep unit-test system-test

unit-test: golint
	vagrant ssh mon0 -c 'sudo -i sh -c "cd /opt/golang/src/github.com/contiv/volplugin; HOST_TEST=1 godep go test -v ./... -check.v"'

build: golint
	@set -e; for i in $$(seq 0 2); do vagrant ssh mon$$i -c 'sudo -i sh -c "cd /opt/golang/src/github.com/contiv/volplugin; make run-build"'; done

run:
	@set -e; for i in $$(seq 0 2); do vagrant ssh mon$$i -c 'cd /opt/golang/src/github.com/contiv/volplugin && make run-volplugin run-volmaster'; done
	vagrant ssh mon0 -c 'cd /opt/golang/src/github.com/contiv/volplugin && make run-volsupervisor'

run-etcd:
	sudo systemctl start etcd

run-volplugin: run-etcd
	sudo pkill volplugin || exit 0
	sudo -E setsid bash -c '/opt/golang/bin/volplugin --debug &>/tmp/volplugin.log &'

run-volsupervisor:
	sudo pkill volsupervisor || exit 0
	sudo -E setsid bash -c '/opt/golang/bin/volsupervisor --debug &>/tmp/volsupervisor.log &'

run-volmaster:
	sudo pkill volmaster || exit 0
	sudo -E setsid bash -c '/opt/golang/bin/volmaster --debug &>/tmp/volmaster.log &'

run-build: godep
	godep go install -v ./volcli/volcli/ ./volplugin/volplugin/ ./volmaster/volmaster/ ./volsupervisor/volsupervisor/

system-test: build godep
	rm -rf Godeps/_workspace/pkg
	godep go test -v ./systemtests -check.v

reflex:
	@echo 'To use this task, `go get github.com/cespare/reflex`'

reflex-build: reflex
	which reflex &>/dev/null && ulimit -n 2048 && reflex -r '.*\.go' make build

reflex-test: reflex
	which reflex &>/dev/null && ulimit -n 2048 && reflex -r '.*\.go' make test

reflex-unit-test: reflex
	which reflex &>/dev/null && ulimit -n 2048 && reflex -r '.*\.go' make unit-test

reflex-docs: reflex install-docs
	# for some reason reflex and the build-docs task don't play nicely
	which reflex &>/dev/null && ulimit -n 2048 && reflex -r 'docs/.*\.md' node docs.js

install-docs:
	@echo "To install the packages required for documentation generation, you need npm."
	npm install mdoc
	npm install marked

start-doc-server:
	go run docs/docs-server.go dist &
	@echo "You should now be able to generate and view the docs at http://localhost:8080"

stop-doc-server:
	pkill -f docs-server

build-docs:
	@echo "To build the documentation, run install-docs first."
	rm -rf dist
	mkdir -p dist
	node docs.js

publish-docs: build-docs
	cd dist && s3cmd sync --delete-removed --recursive * s3://volplugin-docs
