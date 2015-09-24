start: download-docker install-ansible
	vagrant up

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
	vagrant ssh mon0 -c "sudo -i sh -c 'cd /opt/golang/src/github.com/contiv/volplugin; make golint-host'"

download-docker:
	curl https://master.dockerproject.org/linux/amd64/docker -o ansible/docker

install-ansible:
	[ -n "`which ansible`" ] || pip install ansible

test: unit-test system-test

unit-test: golint
	vagrant ssh mon0 -c 'sudo -i sh -c "cd /opt/golang/src/github.com/contiv/volplugin; HOST_TEST=1 godep go test -v ./..."'

build: golint
	@for i in $$(seq 0 2); do vagrant ssh mon$$i -c 'sudo -i sh -c "cd /opt/golang/src/github.com/contiv/volplugin; make run-build"'; done

run:
	vagrant ssh mon0 -c 'sudo -i sh -c "cd /opt/golang/src/github.com/contiv/volplugin; make run-build; (make volplugin-start &); make volmaster-start"'

run-volplugin:
	vagrant ssh mon0 -c 'sudo -i sh -c "cd /opt/golang/src/github.com/contiv/volplugin; make run-build volplugin-start"'

run-volmaster:
	vagrant ssh mon0 -c 'sudo -i sh -c "cd /opt/golang/src/github.com/contiv/volplugin; make run-build volmaster-start"'

run-build:
	godep go install -v ./volcli/volcli/ ./volplugin/volplugin/ ./volmaster

system-test: build
	go test -v ./systemtests

container:
	vagrant ssh mon0 -c 'sudo docker run -it --volume-driver tenant1 -v tmp:/mnt ubuntu bash'

volplugin-start:
	pkill volplugin || exit 0
	sleep 1
	volplugin --debug tenant1

volmaster-start:
	pkill volmaster || exit 0
	sleep 1
	volmaster /etc/volmaster.json

reflex:
	@echo 'To use this task, `go get github.com/cespare/reflex`'
	which reflex &>/dev/null && ulimit -n 2048 && reflex -r '.*\.go' make test system-test
