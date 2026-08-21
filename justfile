install:
	go install ./cmd/invoicer

build:
	go build -o bin/invoicer ./cmd/invoicer

test:
	go test ./...

default:
	just --list
