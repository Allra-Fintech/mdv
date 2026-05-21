BIN     := mdv
PREFIX  ?= $(HOME)/.local
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build install uninstall clean format lint test test-unit test-integration

build:
	go build $(LDFLAGS) -o $(BIN) ./cmd/mdv

install:
	go build $(LDFLAGS) -o $(PREFIX)/bin/$(BIN) ./cmd/mdv

uninstall:
	rm -f $(PREFIX)/bin/$(BIN)

clean:
	rm -f $(BIN)

test-unit:
	go test ./...

test-integration:
	go test -tags integration ./...

test: test-unit test-integration

format:
	go fmt ./...

lint:
	go vet ./...
