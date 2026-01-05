.PHONY: all build test lint fmt clean install

VERSION ?= dev
LDFLAGS = -ldflags "-X main.version=$(VERSION)"

all: fmt lint test build

build:
	go build $(LDFLAGS) -o ralph ./cmd/ralph

test:
	go test ./...

lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

clean:
	rm -f ralph
	rm -rf dist/

install:
	go install $(LDFLAGS) ./cmd/ralph
