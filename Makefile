CEPH_ANSIBLE_DIR=${PWD}/vendor/ceph-ansible

start:
	vagrant up

stop:
	vagrant destroy -f

restart: stop start

ssh:
	vagrant ssh mon0
