.PHONY: install build test

install: build
	cp bin/invoicer $(GOPATH)/bin/invoicer

build:
	go build -o bin/invoicer ./cmd/invoicer

test:
	go test ./...
