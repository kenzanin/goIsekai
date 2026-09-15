# goIsekai — build & quality gate (HTTP server + browser UI, pure Go).
#
# `just` on its own runs the first recipe, `build`. `just --list` shows
# everything with its one-line description.

# The server binary.
binary := "goisekai"

# Everything `go` and `golangci-lint` run over.
pkgs := "./internal/... ./pkg/... ./cmd/..."

# The `check` gate scope: production Go code only. Test files are skipped by
# the linter; the Lua and web assets have their own recipes.
prod := "./internal/... ./pkg/... ./cmd/..."

# Where `install-lua`, `install-js` and `install-info` copy into. Override per run:
#   just plugins_dir=/tmp/plugins install-lua kaliscan
plugins_dir := "app_data/plugins"
info_dir := "app_data/info"

# The frontend, and the compiled assets inside it.
frontend_dir := "cmd/goisekai/frontend"
lib_dir := "cmd/goisekai/frontend/lib"

# compile the server binary (pure Go, CGO-free, cross-compilable)
build: css br
    CGO_ENABLED=0 go build -o {{binary}} ./cmd/goisekai

# compile the embedded stylesheet from the templates, if npx is available
css:
    #!/usr/bin/env sh
    set -eu
    if ! command -v npx >/dev/null 2>&1; then
        echo "npx not found — keeping the committed tailwind.css"
        exit 0
    fi
    # tailwindcss 3.4 is pinned, so its bundled caniuse-lite snapshot ages past
    # the 6 months browserslist tolerates and it nags on every build. The data
    # only decides which CSS prefixes autoprefixer emits; pinning it keeps the
    # output reproducible instead of tracking whatever npm cached today.
    export BROWSERSLIST_IGNORE_OLD_DATA=1
    printf '@tailwind base;\n@tailwind components;\n@tailwind utilities;\n' > {{lib_dir}}/input.css
    npx --yes tailwindcss@3.4.17 -c tailwind.config.js \
        -i {{lib_dir}}/input.css -o {{lib_dir}}/tailwind.css
    rm -f {{lib_dir}}/input.css {{lib_dir}}/input.css.br

# pre-compress the frontend JS/CSS to .br (needs the `brotli` CLI; runs before every build)
br:
    #!/usr/bin/env sh
    set -eu
    for f in {{lib_dir}}/*.js {{lib_dir}}/*.css; do
        brotli -f -q 11 "$f" -o "$f.br"
    done

# quality gate over production Go code only (tests: `just test`, web: `just lint-web`)
check: fmt-prod modernize lint-prod

# build, then serve at debug log level; `just run -port 9099` passes args to the server
run *args: build
    ./{{binary}} -logLevel debug {{args}}

# build and launch (alias of `run`, kept because the docs name it)
devrun *args: build
    ./{{binary}} -logLevel debug {{args}}

# build (alias, so `just dev` still works like it did under make)
dev: build

# open the default browser at the server URL (default 127.0.0.1:8080)
open:
    xdg-open http://127.0.0.1:8080

# run every test in the repo
test:
    CGO_ENABLED=0 go test {{pkgs}}

# run every test under the race detector
race:
    CGO_ENABLED=1 go test -race {{pkgs}}

# format every Go package, tests included
fmt:
    go fmt {{pkgs}}

# format production Go packages (which is all of them; kept for the gate name)
fmt-prod:
    go fmt {{prod}}

# apply the stdlib/language modernizations for the Go version in go.mod
modernize:
    CGO_ENABLED=0 modernize -fix {{pkgs}}

# lint every Go package
lint:
    CGO_ENABLED=0 golangci-lint run {{pkgs}}

# lint production Go code, skipping *_test.go
lint-prod:
    CGO_ENABLED=0 golangci-lint run --tests=false {{prod}}

# format and auto-fix the frontend (Biome)
fmt-web:
    biome check --write {{frontend_dir}}

# lint and format-check the frontend (Biome), read-only
lint-web:
    biome check {{frontend_dir}}

# format the Lua templates with Stylua
fmt-lua:
    stylua internal/templates/

# lint the Lua templates with Luacheck
lint-lua:
    luacheck internal/templates/ --codes --no-unused --no-unused-args

# remove the server binary
clean:
    rm -f {{binary}}

# copy a Lua plugin folder into the plugins dir: `just install-lua kaliscan`
install-lua plugin:
    #!/usr/bin/env sh
    set -eu
    src="examples/plugins/lua/{{plugin}}"
    if [ ! -f "$src/main.lua" ]; then
        echo "$src/main.lua not found"
        exit 1
    fi
    mkdir -p {{plugins_dir}}
    rm -rf {{plugins_dir}}/{{plugin}}
    cp -r "$src" {{plugins_dir}}/{{plugin}}
    echo "installed: {{plugins_dir}}/{{plugin}}/main.lua (restart the server to load it)"

# copy a JS plugin folder into the plugins dir: `just install-js mangzio`
install-js plugin:
    #!/usr/bin/env sh
    set -eu
    src="examples/plugins/js/{{plugin}}"
    if [ ! -f "$src/main.js" ]; then
        echo "$src/main.js not found"
        exit 1
    fi
    mkdir -p {{plugins_dir}}
    rm -rf {{plugins_dir}}/{{plugin}}
    cp -r "$src" {{plugins_dir}}/{{plugin}}
    echo "installed: {{plugins_dir}}/{{plugin}}/main.js (restart the server to load it)"

# copy an info (metadata enrichment) script folder into the info dir: `just install-info mangadex`
install-info info:
    #!/usr/bin/env sh
    set -eu
    src="examples/info/{{info}}"
    if [ ! -f "$src/main.lua" ]; then
        echo "$src/main.lua not found"
        exit 1
    fi
    mkdir -p {{info_dir}}
    rm -rf {{info_dir}}/{{info}}
    cp -r "$src" {{info_dir}}/{{info}}
    echo "installed: {{info_dir}}/{{info}}/main.lua (restart the server to load it)"
