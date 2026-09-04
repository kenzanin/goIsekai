# goIsekai — build & quality gate (HTTP server + browser UI, pure Go).

BINARY := goisekai
PKGS   := ./internal/... ./pkg/... ./cmd/...

.PHONY: build dev devrun run open check fmt fmt-web test race modernize lint lint-web br clean

## build: compile the server binary (pure Go, CGO-free, cross-compilable).
build: br
	CGO_ENABLED=0 go build -o $(BINARY) ./cmd/goisekai

## br: pre-compress frontend JS/CSS to .br (brotli) for embedded serving.
## Requires the `brotli` CLI. Run automatically before every build.
br:
	@for f in cmd/goisekai/frontend/lib/*.js cmd/goisekai/frontend/lib/*.css; do \
		brotli -f -q 11 "$$f" -o "$$f.br"; \
	done

## dev: build at debug log level default (nothing special needed anymore).
dev: build

## devrun: build and launch with debug logging.
devrun: dev
	./$(BINARY) -logLevel debug $(ARGS)

## run: build and launch the server (auto-opens browser).
run: build
	./$(BINARY) -logLevel debug $(ARGS)

## open: open the default browser at the server URL (default 127.0.0.1:8080).
open:
	xdg-open http://127.0.0.1:8080

## check: full quality gate — format, race tests, modernize, lint (Go + web).
check: fmt fmt-web race modernize lint lint-web

fmt:
	go fmt $(PKGS)

## fmt-web: format + auto-fix the frontend (Biome).
fmt-web:
	biome check --write cmd/goisekai/frontend

## lint-web: lint + format-check the frontend (Biome), read-only.
lint-web:
	biome check cmd/goisekai/frontend

test:
	CGO_ENABLED=0 go test $(PKGS)

race:
	CGO_ENABLED=1 go test -race $(PKGS)

modernize:
	CGO_ENABLED=0 modernize -fix $(PKGS)

lint:
	CGO_ENABLED=0 golangci-lint run $(PKGS)

## clean: remove the server binary.
clean:
	rm -f $(BINARY)

## install-lua: copy a Lua plugin folder into the configured plugins dir.
## Usage: make install-lua PLUGIN=kaliscan
PLUGINS_DIR ?= app_data/plugins
install-lua:
	@test -f plugins/lua/$(PLUGIN)/main.lua || { echo "plugins/lua/$(PLUGIN)/main.lua not found"; exit 1; }
	mkdir -p $(PLUGINS_DIR)
	rm -rf $(PLUGINS_DIR)/$(PLUGIN)
	cp -r plugins/lua/$(PLUGIN) $(PLUGINS_DIR)/$(PLUGIN)
	@echo "installed: $(PLUGINS_DIR)/$(PLUGIN)/main.lua (restart the server to load it)"
