.PHONY: test-up test-provision test-cleanup

test-up:
	vagrant up

test-provision:
	vagrant provision

test-cleanup:
	CONTIV_ANSIBLE_PLAYBOOK="./cleanup.yml" CONTIV_ANSIBLE_TAGS="all" vagrant provision
