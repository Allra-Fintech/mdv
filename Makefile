BIN     := mdv
PREFIX  ?= $(HOME)/.local

.PHONY: build install uninstall clean format lint

build:
	go build -o $(BIN) ./cmd/mdv

install:
	go build -o $(PREFIX)/bin/$(BIN) ./cmd/mdv

uninstall:
	rm -f $(PREFIX)/bin/$(BIN)

clean:
	rm -f $(BIN)

format:
	go fmt ./...

lint:
	go vet ./...
