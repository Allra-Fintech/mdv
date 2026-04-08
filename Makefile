BIN     := mdv
PREFIX  ?= $(HOME)/.local

.PHONY: build install uninstall clean

build:
	go build -o $(BIN) ./cmd/mdv

install:
	go build -o $(PREFIX)/bin/$(BIN) ./cmd/mdv

uninstall:
	rm -f $(PREFIX)/bin/$(BIN)

clean:
	rm -f $(BIN)
