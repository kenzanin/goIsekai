# goIsekai — build & quality gate.

BINARY := goisekai
TAGS   := webkit2_41,production
PKGS   := ./internal/... ./pkg/... ./cmd/...

.PHONY: build check fmt test race modernize lint

## build: compile the desktop binary (requires webkit2gtk-4.1 on Linux).
build:
	go build -tags $(TAGS) -o $(BINARY) ./cmd/goisekai

## check: full quality gate — format, race tests, modernize, lint.
check: fmt race modernize lint

fmt:
	go fmt $(PKGS)

test:
	go test $(PKGS)

race:
	go test -race $(PKGS)

modernize:
	modernize -fix $(PKGS)

lint:
	golangci-lint run $(PKGS)
