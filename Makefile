CEPH_ANSIBLE_DIR=${PWD}/vendor/ceph-ansible

start:
	vagrant up

stop:
	vagrant destroy -f

restart: stop start

provision:
	vagrant provision

ssh:
	vagrant ssh mon0

build:
	godep go install ./
