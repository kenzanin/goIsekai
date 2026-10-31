# Build 1manga wasm plugin for goIsekai
# TinyGo-only build, no dependency on tools/build-wasm.sh

TINYGO ?= $(HOME)/opt/tinygo/bin/tinygo
WASMOPT ?= $(HOME)/opt/tinygo/bin/wasm-opt
GO126 ?= $(HOME)/.govman/versions/go1.26.9

export GO126
export GOROOT := $(GO126)
export GOTOOLCHAIN = local
export CGO_ENABLED = 0

.PHONY: all build clean fix

all: build

build: main.wasm

main.wasm: *.go
	$(TINYGO) build -target wasi -buildmode=c-shared -o main.wasm .
	$(WASMOPT) -O4 main.wasm -o main.wasm
	@echo "Built main.wasm: $$($(TINYGO) version | awk '{print $$3}')"

clean:
	rm -f main.wasm



fix:
	go1.26.9 fix ./**.go