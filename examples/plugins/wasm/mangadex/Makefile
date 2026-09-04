.PHONY: build install clean

WASM     := mangadex.wasm
DATA_DIR ?= app_data

## build: compile the mangadex plugin to wasip1/wasm
build:
	GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o $(WASM) ./main.go

## install: build and copy into <DATA_DIR>/plugins (default app_data)
install: build
	mkdir -p ../../$(DATA_DIR)/plugins
	cp $(WASM) ../../$(DATA_DIR)/plugins/$(WASM)

## clean: remove the built wasm
clean:
	rm -f $(WASM)
