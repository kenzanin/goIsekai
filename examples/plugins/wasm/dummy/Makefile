.PHONY: build install clean

WASM     := dummy.wasm
DATA_DIR ?= app_data

## build: compile the dummy plugin to wasip1/wasm
build:
	GOOS=wasip1 GOARCH=wasm go build -o $(WASM) .

## install: build and copy into <DATA_DIR>/plugins (default app_data)
install: build
	mkdir -p ../../$(DATA_DIR)/plugins
	cp $(WASM) ../../$(DATA_DIR)/plugins/$(WASM)

## clean: remove the built wasm
clean:
	rm -f $(WASM)
