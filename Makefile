VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BINARY  := id-mcp
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build install test clean release help

## Build the MCP server binary
build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/id-mcp/
	@echo "Built: bin/$(BINARY) ($(VERSION))"

## Install the MCP binary to ~/.local/bin
install: build
	mkdir -p ~/.local/bin
	cp bin/$(BINARY) ~/.local/bin/$(BINARY)
	@echo "Installed: ~/.local/bin/$(BINARY)"

## Run tests
test:
	go test ./...

## Cross-compile for all platforms
release:
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64  ./cmd/id-mcp/
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64  ./cmd/id-mcp/
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64   ./cmd/id-mcp/
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64   ./cmd/id-mcp/
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe ./cmd/id-mcp/
	@echo "Release binaries built in dist/"

## Remove build artifacts
clean:
	rm -rf bin/ dist/

## Show help
help:
	@echo "id-mcp — Wordmade ID MCP Server"
	@echo ""
	@echo "  make build     Build binary"
	@echo "  make install   Build and install to ~/.local/bin"
	@echo "  make test      Run tests"
	@echo "  make release   Cross-compile for all platforms"
	@echo "  make clean     Remove build artifacts"
