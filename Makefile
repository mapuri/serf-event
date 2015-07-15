.PHONY: checks check-format check-code deps unit-test

all: checks

deps:
	go get github.com/tools/godep
	go get github.com/golang/lint/golint

checks: deps check-format check-code unit-test
	
check-format:
	@echo "checking format..."
	test -z "$$(gofmt -l . | grep -v Godeps/_workspace/src/ | tee /dev/stderr)"
	@echo "done checking format..."

check-code:
	@echo "checking code..."
	test -z "$$(golint ./... | tee /dev/stderr)"
	go vet ./...
	@echo "done checking code..."

unit-test:
	godep go test -v ./...
