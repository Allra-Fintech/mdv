BIN     := mdv
PREFIX  ?= $(HOME)/.local

.PHONY: build install uninstall clean format lint test test-unit test-integration

build:
	go build -o $(BIN) ./cmd/mdv

install:
	go build -o $(PREFIX)/bin/$(BIN) ./cmd/mdv

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
