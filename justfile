install:
	go install ./cmd/invoicer

build:
	go build -o bin/invoicer ./cmd/invoicer

[no-exit-message]
dev *ARGS:
	air {{ARGS}}

test:
	go test ./...

default:
	just --list
