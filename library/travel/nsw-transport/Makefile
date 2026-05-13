.PHONY: build test lint install clean

build:
	go build -o bin/nsw-transport-pp-cli ./cmd/nsw-transport-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/nsw-transport-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/nsw-transport-pp-mcp ./cmd/nsw-transport-pp-mcp

install-mcp:
	go install ./cmd/nsw-transport-pp-mcp

build-all: build build-mcp
