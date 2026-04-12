.PHONY: build test lint install clean

build:
	go build -o bin/yahoo-finance-pp-cli ./cmd/yahoo-finance-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/yahoo-finance-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/yahoo-finance-pp-mcp ./cmd/yahoo-finance-pp-mcp

install-mcp:
	go install ./cmd/yahoo-finance-pp-mcp

build-all: build build-mcp
