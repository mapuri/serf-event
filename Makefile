.PHONY: checks check-format check-code deps unit-test

all: checks

deps:
	go get github.com/tools/godep
	go get github.com/golang/lint/golint

checks: deps check-format check-code unit-test
	
check-format:
	@echo "checking format..."
	test -z "$(golint . | grep -v Godeps/_workspace/src/)"

check-code:
	@echo "checking lint..."
	test -z "$(golint ./...)"
	go vet ./...

unit-test:
	godep go test -v ./...
