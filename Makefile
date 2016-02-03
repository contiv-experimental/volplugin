GUESTPREFIX=/opt/golang
GUESTGOPATH=$(GUESTPREFIX)/src/github.com/contiv/volplugin
GUESTBINPATH=$(GUESTPREFIX)/bin

start: install-ansible
	vagrant up
	make build

stop:
	vagrant destroy -f
	make clean

clean:
	rm -f *.vdi
	rm -f .vagrant/*.vmdk

update:
	vagrant box update || exit 0

restart: stop update start

provision:
	vagrant provision

ssh:
	vagrant ssh mon0

golint-host:
	[ -n "`which golint`" ] || go get github.com/golang/lint/golint
	golint ./...

golint:
	vagrant ssh mon0 -c "sudo -i sh -c 'cd $(GUESTGOPATH); http_proxy=${http_proxy} https_proxy=${https_proxy} make golint-host'"

# -composites=false is required to work around bug https://github.com/golang/go/issues/11394
govet-host:
	go tool vet -composites=false `find . -name '*.go' | grep -v Godeps`

govet:
	vagrant ssh mon0 -c "sudo -i sh -c 'cd $(GUESTGOPATH); http_proxy=${http_proxy} https_proxy=${https_proxy} make govet-host'"

install-ansible:
	[ -n "`which ansible`" ] || sudo pip install ansible

ci:
	GOPATH=/tmp/volplugin:${WORKSPACE} PATH="/tmp/volplugin/bin:/usr/local/go/bin:${PATH}" make test

godep:
	[ -n "`which godep`" ] || go get github.com/kr/godep

test: godep unit-test system-test

unit-test:
	vagrant ssh mon0 -c 'sudo -i sh -c "cd $(GUESTGOPATH); make unit-test-host"'

unit-test-host: godep golint-host govet-host
	godep go list ./... | HOST_TEST=1 GOGC=1000 xargs -I{} godep go test -v '{}' -coverprofile=$(GUESTPREFIX)/src/{}/cover.out -check.v

unit-test-nocoverage:
	vagrant ssh mon0 -c 'sudo -i sh -c "cd $(GUESTGOPATH); make unit-test-nocoverage-host"'

unit-test-nocoverage-host: golint-host govet-host
	HOST_TEST=1 GOGC=1000 godep go test -v ./... -check.v

build: golint govet
	vagrant ssh mon0 -c 'sudo -i sh -c "cd $(GUESTGOPATH); make run-build"'
	make run

docker: run-build
	docker build -t contiv/volplugin .

docker-push: docker
	docker push contiv/volplugin

run:
	@set -e; for i in $$(seq 0 2); do vagrant ssh mon$$i -c 'cd $(GUESTGOPATH) && make run-volplugin run-volmaster'; done
	vagrant ssh mon0 -c 'cd $(GUESTGOPATH) && make run-volsupervisor'

run-etcd:
	sudo systemctl start etcd

run-volplugin: run-etcd
	sudo pkill volplugin || exit 0
	sudo -E nohup bash -c '$(GUESTBINPATH)/volplugin --debug &>/tmp/volplugin.log &'

run-volsupervisor:
	sudo pkill volsupervisor || exit 0
	sudo -E nohup bash -c '$(GUESTBINPATH)/volsupervisor --debug &>/tmp/volsupervisor.log &'

run-volmaster:
	sudo pkill volmaster || exit 0
	sudo -E nohup bash -c '$(GUESTBINPATH)/volmaster --debug &>/tmp/volmaster.log &'

run-build: godep
	GOGC=1000 godep go install -v ./volcli/volcli/ ./volplugin/volplugin/ ./volmaster/volmaster/ ./volsupervisor/volsupervisor/

system-test: build godep
	rm -rf Godeps/_workspace/pkg
	godep go test -v -timeout 120m ./systemtests -check.v

reflex:
	@echo 'To use this task, `go get github.com/cespare/reflex`'

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

tar: clean-tar run-build
	@echo "v0.0.0-`date -u +%m-%d-%Y.%H-%M-%S.UTC`" > $(VERSION_FILE)
	@tar -jcf $(TAR_FILE) -C $(GOPATH)/bin volcli volmaster volplugin volsupervisor -C $(GOPATH)/src/github.com/contiv/volplugin contrib/completion/bash/volcli

clean-tar:
	@rm -f $(TAR_LOC)/*.$(TAR_EXT)

# GITHUB_USER and GITHUB_TOKEN are needed be set to run github-release
release: tar
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

vendor-ansible:
	git subtree pull --prefix ansible https://github.com/contiv/ansible HEAD
