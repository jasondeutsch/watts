GO ?= go
INSTALL_DIR ?= /usr/local/bin
SUDO ?= sudo

BINARY := bin/watts

.PHONY: build test install

build:
	mkdir -p bin
	$(GO) build -o "$(BINARY)" ./cmd/cli

test:
	$(GO) test ./...

install: build
	$(SUDO) mkdir -p "$(INSTALL_DIR)"
	$(SUDO) install -m 755 "$(BINARY)" "$(INSTALL_DIR)/watts"
