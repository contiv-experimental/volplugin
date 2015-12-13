all: stop start test
	make stop

deps:
	go get github.com/tools/godep
	go get github.com/golang/lint/golint

reflex-test: start
	reflex -r '\.go' make test

test: deps start golint
	godep go test -v ./... -check.v

golint: deps
	golint ./...

stop:
	vagrant destroy -f

start:
	vagrant up
