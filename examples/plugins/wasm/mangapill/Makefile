# Build the Mangapill TinyGo wasm plugin for goIsekai.
# Mirrors examples/plugins/wasm/mangabuddy/Makefile (TinyGo 0.43.0 only).

TINYGO ?= $(HOME)/opt/tinygo/bin/tinygo
WASMOPT ?= $(HOME)/opt/tinygo/bin/wasm-opt
GO126 ?= $(HOME)/sdk/go1.26.9

export GO126
export GOROOT := $(GO126)
export GOTOOLCHAIN = local
export CGO_ENABLED = 0

.PHONY: all build clean lint

all: build

build: main.wasm

main.wasm: main.go abi.go hostcall.go text.go text_host.go
	$(TINYGO) build -target wasi -buildmode=c-shared -o main.wasm .
	$(WASMOPT) -O4 main.wasm -o main.wasm
	@echo "Built main.wasm: $$($(TINYGO) version | awk '{print $$3}')"

clean:
	rm -f main.wasm

lint:
	go vet .
	@fmtout=$$(gofmt -l .); if [ -n "$$fmtout" ]; then echo "gofmt: $$fmtout"; exit 1; fi
	@echo "lint OK"

fix: **.go
	go fix ./**.go
