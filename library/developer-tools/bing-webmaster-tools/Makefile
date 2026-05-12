.PHONY: build test lint install clean

build:
	go build -o bin/bing-webmaster-tools-pp-cli ./cmd/bing-webmaster-tools-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/bing-webmaster-tools-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/bing-webmaster-tools-pp-mcp ./cmd/bing-webmaster-tools-pp-mcp

install-mcp:
	go install ./cmd/bing-webmaster-tools-pp-mcp

build-all: build build-mcp
