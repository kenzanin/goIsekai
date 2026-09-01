# mangafire-plugin

Standalone Go/WASM manga source plugin for goIsekai (mangafire.to), built as
its own module — no dependency on the repo root. Exports the standard plugin
ABI: `contract_version`, `Init`, `Search`, `GetMangaDetail`, `GetChapterList`,
`GetPageList`, plus the `malloc`/`free`/`host_http_request` memory ABI.

## Build

    make build    # GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o dist/mangafire.wasm ./main.go

## Test

    make test     # go vet ./... && go test ./... (host-native VRF vector tests)
