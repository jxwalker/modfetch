# Simple build matrix and packaging targets

BINARY := modfetch
DIST := dist
VERSION ?= $(shell git describe --tags --always --dirty=-dev 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build test clean dist linux darwin macos-universal checksums

all: build

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/$(BINARY)

test:
	go test ./...

clean:
	rm -rf bin $(DIST)

$(DIST):
	mkdir -p $(DIST)

linux: $(DIST)
	# Linux amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags osusergo,netgo -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)_linux_amd64 ./cmd/$(BINARY)
	# Linux arm64
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags osusergo,netgo -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)_linux_arm64 ./cmd/$(BINARY)

darwin: $(DIST)
	# macOS amd64
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)_darwin_amd64 ./cmd/$(BINARY)
	# macOS arm64
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)_darwin_arm64 ./cmd/$(BINARY)

macos-universal: darwin
	bash scripts/build_macos_universal.sh $(DIST)/$(BINARY)_darwin_amd64 $(DIST)/$(BINARY)_darwin_arm64 $(DIST)/$(BINARY)_darwin_universal

checksums: $(DIST)
	# Generate SHA256 checksums for all artifacts in dist/
	cd $(DIST) && shasum -a 256 * > SHA256SUMS

release-dist: clean linux darwin checksums
	@echo "Artifacts in $(DIST)/ ready for release"

