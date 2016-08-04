GUESTPREFIX=/opt/golang
GUESTGOPATH=$(GUESTPREFIX)/src/github.com/contiv/volplugin
GUESTBINPATH=$(GUESTPREFIX)/bin

.PHONY: build

start: check-ansible
	# vagrant hits the file descriptor limit on OSX when running this task
	# 10240 is the max you can set on OSX and should be higher than the default on every other OS
	ulimit -n 10240; \
	if [ "x${PROVIDER}" = "x" ]; then vagrant up; else vagrant up --provider=${PROVIDER}; fi
	make run

big:
	BIG=1 vagrant up
	make run

demo:
	DEMO=1 make restart

stop:
	vagrant destroy -f
	make clean

clean:
	([ -n "$$(cat subnet_assignment.state)" ] && rm -rf /tmp/volplugin_vagrant_subnets/`cat subnet_assignment.state`) || :
	rm -f subnet_assignment.state
	rm -f *.vdi
	rm -f .vagrant/*.vmdk

clean-vms:
	@echo DO NOT USE THIS COMMAND UNLESS YOU ABSOLUTELY HAVE TO. PRESS CTRL-C NOW.
	#@sleep 20
	for i in $$(vboxmanage list vms | grep volplugin | awk '{ print $$2 }'); do vboxmanage controlvm "$$i" poweroff; vboxmanage unregistervm "$$i"; done
	make clean

update:
	vagrant box update || exit 0

restart: stop update start

reload:
	vagrant reload --provision

provision:
	vagrant provision

ssh:
	vagrant ssh mon0

checks:
	vagrant ssh mon0 -c "sudo -i sh -c 'cd $(GUESTGOPATH); ./build/scripts/checks.sh'"

check-ansible:
	@build/scripts/check-ansible.sh

ci:
	GOPATH=/tmp/volplugin:${WORKSPACE} PATH="/tmp/volplugin/bin:/usr/local/go/bin:${PATH}" make test

test: unit-test system-test

unit-test:
	vagrant ssh mon0 -c 'sudo -i sh -c "cd $(GUESTGOPATH); TESTRUN="${TESTRUN}" make unit-test-host"'

unit-test-host:
	go list ./... | grep -v vendor | HOST_TEST=1 GOGC=1000 xargs -I{} go test -v '{}' -coverprofile=$(GUESTPREFIX)/src/{}/cover.out -check.v -check.f "${TESTRUN}"

unit-test-nocoverage:
	vagrant ssh mon0 -c 'sudo -i sh -c "cd $(GUESTGOPATH); TESTRUN="${TESTRUN}" make unit-test-nocoverage-host"'

unit-test-nocoverage-host:
	HOST_TEST=1 GOGC=1000 go test -v ./... -check.v -check.f "${TESTRUN}"

build: checks
	vagrant ssh mon0 -c 'sudo -i sh -c "cd $(GUESTGOPATH); make run-build"'
	if [ ! -n "$$DEMO" ]; then for i in mon1 mon2; do vagrant ssh $$i -c 'sudo sh -c "pkill volplugin; pkill apiserver; pkill volsupervisor; mkdir -p /opt/golang/bin; cp /tmp/bin/* /opt/golang/bin"'; done; fi

docker: run-build
	docker build -t contiv/volplugin .

docker-push: docker
	docker push contiv/volplugin

clean-volplugin-containers:
	set -e; for i in $$(seq 0 $$(($$(vagrant status | grep -cE 'mon.*running') - 1))); do vagrant ssh mon$$i -c 'docker ps | grep "volplugin" | cut -d " " -f 1 | xargs docker rm -fv'; done

run: build
	set -e; for i in $$(seq 0 $$(($$(vagrant status | grep -cE 'mon.*running') - 1))); do vagrant ssh mon$$i -c 'cd $(GUESTGOPATH) && ./build/scripts/build-volplugin-containers.sh && make run-volplugin run-apiserver'; done
	vagrant ssh mon0 -c 'cd $(GUESTGOPATH) && make run-volsupervisor'
	sleep 10
	vagrant ssh mon0 -c 'volcli global upload < /testdata/globals/global1.json'

run-etcd:
	sudo systemctl start etcd

create-systemd-services:
	sudo cp '${GUESTGOPATH}/build/scripts/volplugin.service' /etc/systemd/system/
	sudo cp '${GUESTGOPATH}/build/scripts/volsupervisor.service' /etc/systemd/system/
	sudo cp '${GUESTGOPATH}/build/scripts/apiserver.service' /etc/systemd/system/
	sudo cp '${GUESTGOPATH}/build/scripts/volplugin.sh' /usr/bin/
	sudo cp '${GUESTGOPATH}/build/scripts/volsupervisor.sh' /usr/bin/
	sudo cp '${GUESTGOPATH}/build/scripts/apiserver.sh' /usr/bin/
	sudo systemctl daemon-reload

run-volplugin: run-etcd create-systemd-services
	sudo systemctl stop volplugin
	sudo systemctl start volplugin

run-volsupervisor:
	sudo systemctl stop volsupervisor
	sudo systemctl start volsupervisor

run-apiserver:
	sudo systemctl stop apiserver
	sudo systemctl start apiserver

run-build:
	GOGC=1000 go install -v \
		 -ldflags '-X main.version=$(if $(BUILD_VERSION),$(BUILD_VERSION),devbuild)' \
		 ./volcli/volcli/ ./volplugin/volplugin/ ./apiserver/apiserver/ ./volsupervisor/volsupervisor/
	cp $(GUESTBINPATH)/* bin/

system-test: system-test-ceph system-test-nfs

system-test-ceph: run
	USE_DRIVER=ceph TESTRUN="${TESTRUN}" ./build/scripts/systemtests.sh

system-test-nfs: run
	USE_DRIVER=nfs TESTRUN="${TESTRUN}" ./build/scripts/systemtests.sh

vendor-ansible:
	git subtree pull --prefix ansible https://github.com/contiv/ansible HEAD --squash

reflex:
	@echo 'To use this task, `go get github.com/cespare/reflex`'

reflex-run: reflex
	which reflex &>/dev/null && ulimit -n 2048 && reflex -r '.*\.go' make run

reflex-build: reflex
	which reflex &>/dev/null && ulimit -n 2048 && reflex -r '.*\.go' make build

reflex-test: reflex
	which reflex &>/dev/null && ulimit -n 2048 && reflex -r '.*\.go' make test

reflex-unit-test: reflex
	which reflex &>/dev/null && ulimit -n 2048 && reflex -r '.*\.go' make unit-test-nocoverage

# We are using date based versioning, so for consistent version during a build
# we evaluate and set the value of version once in a file and use it in 'tar'
# and 'release' targets.
NAME := volplugin
VERSION_FILE := /tmp/$(NAME)-version
VERSION := `cat $(VERSION_FILE)`
TAR_EXT := tar.bz2
TAR_FILENAME := $(NAME)-$(VERSION).$(TAR_EXT)
TAR_LOC := .
TAR_FILE := $(TAR_LOC)/$(TAR_FILENAME)

tar: clean-tar
	@echo "v0.0.0-`date -u +%m-%d-%Y.%H-%M-%S.UTC`" > $(VERSION_FILE)
	@tar -jcf $(TAR_FILE) -C ${PWD}/bin volcli apiserver volplugin volsupervisor -C ${PWD} contrib

clean-tar:
	@rm -f $(TAR_LOC)/*.$(TAR_EXT)

release:

# GITHUB_USER and GITHUB_TOKEN are needed be set to run github-release
release-github: tar
	@go get github.com/aktau/github-release
	@latest_tag=$$(git describe --tags `git rev-list --tags --max-count=1`); \
		comparison="$$latest_tag..HEAD"; \
		changelog=$$(git log $$comparison --oneline --no-merges --reverse); \
		if [ -z "$$changelog" ]; then echo "No new changes to release!"; exit 0; fi; \
		set -x; \
		( ( github-release -v release -p -r volplugin -t $(VERSION) -d "**Changelog**<br/>$$changelog" ) && \
		( github-release -v upload -r volplugin -t $(VERSION) -n $(TAR_FILENAME) -f $(TAR_FILE) || \
		github-release -v delete -r volplugin -t $(VERSION) ) ) || exit 1
	@make clean-tar
