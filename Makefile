.PHONY: all build test fmt vet clean

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

all: build

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/pm ./cmd/pm

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

clean:
	rm -f bin/pm
