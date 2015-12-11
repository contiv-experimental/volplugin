all: stop start test
	make stop

reflex-test: start
	reflex -r '\.go' make test

test: start golint
	godep go test -v ./... -check.v

golint:
	golint ./...

stop:
	vagrant destroy -f

start:
	vagrant up
