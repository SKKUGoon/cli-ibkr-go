.PHONY: build test check install
build:
	go build -trimpath -o bin/ibkr ./cmd/ibkr

test:
	go test -race ./...

check:
	test -z "$$(gofmt -l .)"
	go vet ./...
	go test -race ./...

install:
	go install ./cmd/ibkr
