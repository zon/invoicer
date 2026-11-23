version := `sh scripts/version.sh`

install:
	go install ./cmd/invoicer

build:
	go build -ldflags "-X github.com/zon/invoicer/internal/cli.Version={{version}}" -o bin/invoicer ./cmd/invoicer

[no-exit-message]
dev *ARGS:
	air {{ARGS}}

test:
	go test ./...

default:
	just --list
