WASM := dist/mangafire.wasm

.PHONY: test build clean

test:
	go vet ./...
	go test ./...

build:
	GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o $(WASM) ./main.go

clean:
	rm -rf dist
