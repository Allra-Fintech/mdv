BIN     := mdview
PREFIX  ?= $(HOME)/.local

.PHONY: build install uninstall clean

build:
	go build -o $(BIN) .

install:
	go build -o $(PREFIX)/bin/$(BIN) .

uninstall:
	rm -f $(PREFIX)/bin/$(BIN)

clean:
	rm -f $(BIN)
