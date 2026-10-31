# Build kunmanga_online wasm plugin for goIsekai
# TinyGo-only build, no dependency on tools/build-wasm.sh

TINYGO ?= $(HOME)/opt/tinygo/bin/tinygo
WASMOPT ?= $(HOME)/opt/tinygo/bin/wasm-opt
GO126 ?= $(HOME)/sdk/go1.26.9

export GO126
export GOROOT := $(GO126)
export GOTOOLCHAIN = local
export CGO_ENABLED = 0
# TinyGo shells out to `go` for module metadata, so the pinned toolchain's bin
# must be on PATH — GOROOT alone is not enough.
export PATH := $(GO126)/bin:$(PATH)

.PHONY: all build clean lint fix

all: build

build: main.wasm

main.wasm: *.go
	$(TINYGO) build -target wasi -buildmode=c-shared -o main.wasm .
	$(WASMOPT) -O4 main.wasm -o main.wasm
	@echo "Built main.wasm: $$($(TINYGO) version | awk '{print $$3}')"

clean:
	rm -f main.wasm

lint:
	@fmtout=$$(gofmt -l .); if [ -n "$$fmtout" ]; then echo "gofmt: $$fmtout"; exit 1; fi
	@echo "lint OK"

# go1.26.9 (the pinned toolchain, already first on PATH) rewrites old APIs.


fix:
	go1.26.9 fix ./**.go