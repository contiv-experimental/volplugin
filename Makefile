CEPH_ANSIBLE_DIR=${PWD}/vendor/ceph-ansible

start: install-ansible
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

install-ansible:
	[[ -n `which ansible` ]] || sudo pip install ansible
