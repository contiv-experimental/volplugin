start: install-ansible
	vagrant up

stop:
	vagrant destroy -f

restart: stop start

provision:
	vagrant provision

ssh:
	vagrant ssh mon0

golint:
	[ -n "`which golint`" ] || go get github.com/golang/lint/golint
	golint ./...


install-ansible:
	[ -n "`which ansible`" ] || pip install ansible

test: golint
	vagrant ssh mon0 -c 'sudo -i sh -c "cd /opt/golang/src/github.com/contiv/volplugin; godep go test -v ./..."'

build: golint
	vagrant ssh mon0 -c 'sudo -i sh -c "cd /opt/golang/src/github.com/contiv/volplugin; make run-build"'

run-volplugin:
	vagrant ssh mon0 -c 'sudo -i sh -c "cd /opt/golang/src/github.com/contiv/volplugin; make volplugin-start"'

run-volmaster:
	vagrant ssh mon0 -c 'sudo -i sh -c "cd /opt/golang/src/github.com/contiv/volplugin; make volmaster-start"'

run-build:
	godep go install -v ./volplugin/ ./volmaster/

volplugin-start: run-build
	pkill volplugin || exit 0
	sleep 1
	DEBUG=1 volplugin tenant1

volmaster-start: run-build
	pkill volmaster || exit 0
	sleep 1
	DEBUG=1 volmaster /etc/volmaster.json

reflex:
	@echo 'To use this task, `go get github.com/cespare/reflex`'
	reflex -r '.*\.go' make test
