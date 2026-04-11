.PHONY: build test lint install clean

build:
	go build -o bin/weather-goat-pp-cli ./cmd/weather-goat-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/weather-goat-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/weather-pp-mcp ./cmd/weather-pp-mcp

install-mcp:
	go install ./cmd/weather-pp-mcp

build-all: build build-mcp
