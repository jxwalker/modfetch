# Simple build matrix and packaging targets

BINARY := modfetch
DIST := dist
VERSION ?= $(shell git describe --tags --always --dirty=-dev 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

GOLANGCI_BIN ?= $(shell command -v golangci-lint 2>/dev/null)

.PHONY: all build test clean dist linux darwin macos-universal checksums fmt fmt-check vet lint ci

all: build

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/$(BINARY)

test:
	go test ./...

fmt:
	go fmt ./...

# Fails if files need formatting
fmt-check:
	@echo "Checking gofmt..."
	@diff -u <(echo -n) <(gofmt -s -l .) || (echo "Run 'make fmt' to format the above files" && exit 1)

vet:
	go vet ./...

lint:
ifdef GOLANGCI_BIN
	$(GOLANGCI_BIN) run ./...
else
	@echo "golangci-lint not found. Install: https://golangci-lint.run/ or run via CI."
	@exit 1
endif

ci: vet fmt-check test
	@echo "CI checks completed"

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

