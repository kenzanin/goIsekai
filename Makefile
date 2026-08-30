# goIsekai — build & quality gate (HTTP server + browser UI, pure Go).

BINARY := goisekai
PKGS   := ./internal/... ./pkg/... ./cmd/...

.PHONY: build dev devrun run open check fmt fmt-web test race modernize lint lint-web clean

## build: compile the server binary (pure Go, CGO-free, cross-compilable).
build:
	CGO_ENABLED=0 go build -o $(BINARY) ./cmd/goisekai

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
	CGO_ENABLED=0 go test -race $(PKGS)

modernize:
	CGO_ENABLED=0 modernize -fix $(PKGS)

lint:
	CGO_ENABLED=0 golangci-lint run $(PKGS)

## clean: remove the server binary.
clean:
	rm -f $(BINARY)
