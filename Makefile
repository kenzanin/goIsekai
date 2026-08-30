# goIsekai — build & quality gate.

BINARY := goisekai
## Wails v3: default path needs gtk4+webkitgtk-6.0; use the legacy 'gtk3' tag
## which builds against the installed webkit2gtk-4.1 (no separate webkit2_41 tag).
TAGS   := gtk3,production
PKGS   := ./internal/... ./pkg/... ./cmd/...

.PHONY: build dev devrun run check gen-frontend fmt fmt-web test race modernize lint lint-web test-reader test-frontend

## build: compile the desktop binary (requires webkit2gtk-4.1 on Linux via the gtk3 tag).
## Wails v3 needs CGO for webkit2gtk-4.1 (no separate webkit2_41 tag).
build: gen-frontend
	CGO_ENABLED=1 go build -tags $(TAGS) -o $(BINARY) ./cmd/goisekai

## dev: build with devtools enabled (Ctrl+Shift+F12 to open inspector).
dev: gen-frontend
	CGO_ENABLED=1 go build -tags gtk3 -o $(BINARY) ./cmd/goisekai

## devrun: build with devtools and launch.
devrun: dev
	./$(BINARY) -logLevel debug $(ARGS) 

## run: build and launch the app. Pass flags via ARGS, e.g.: make run ARGS=-logLevel=debug
run: build
	./$(BINARY) -logLevel debug $(ARGS)

## check: full quality gate — regenerate frontend, format, race tests, modernize, lint (Go + web).
check: gen-frontend fmt fmt-web race modernize lint lint-web

fmt:
	go fmt $(PKGS)

## gen-frontend: assemble cmd/goisekai/frontend/index.html from shell.html + views/ partials.
## Runs before build so go:embed always picks up partial edits.
gen-frontend:
	go run ./cmd/frontendgen

## fmt-web: format + auto-fix the frontend (Biome).
fmt-web:
	biome check --write cmd/goisekai/frontend

## lint-web: lint + format-check the frontend (Biome), read-only.
lint-web:
	biome check cmd/goisekai/frontend

test: gen-frontend
	CGO_ENABLED=1 go test -tags $(TAGS) $(PKGS)

race: gen-frontend
	CGO_ENABLED=1 go test -race -tags $(TAGS) $(PKGS)

modernize:
	## modernize has no -tags flag; go/packages honours GOFLAGS so it type-checks
	## the cmd package against the gtk3/webkit2gtk-4.1 path (not the default
	## webkitgtk-6.0 path unavailable on this host).
	CGO_ENABLED=1 GOFLAGS="-tags=$(TAGS)" modernize -fix $(PKGS)

lint:
	## typecheck linter must compile cmd (imports wails/v3 cgo) against the
	## gtk3/webkit2gtk-4.1 path; golangci-lint honours GOFLAGS.
	CGO_ENABLED=1 GOFLAGS="-tags=$(TAGS)" golangci-lint run $(PKGS)

## test-frontend: syntax-check + unit-test the pure JS frontend modules
## (no browser needed). Covers readhash (reader hash parsing/idempotency/next-chapter
## ordering) and format (image-byte sniffing + format/convert helpers).
test-frontend:
	node --check cmd/goisekai/frontend/lib/readhash.js
	node --check cmd/goisekai/frontend/lib/readhash.test.js
	node --check cmd/goisekai/frontend/lib/format.js
	node --check cmd/goisekai/frontend/lib/format.test.js
	node cmd/goisekai/frontend/lib/readhash.test.js
	node cmd/goisekai/frontend/lib/format.test.js

## test-reader: run just the reader hash-parsing/idempotency/next-chapter tests.
test-reader:
	node cmd/goisekai/frontend/lib/readhash.test.js
