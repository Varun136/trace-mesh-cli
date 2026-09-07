APP_NAME := tm
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
BUILD_DIR := ./bin
LDFLAGS := -s -w -X tracemesh/cmd.version=$(VERSION) -X tracemesh/cmd.commit=$(COMMIT) -X tracemesh/cmd.date=$(DATE)

.PHONY: all build build-all test clean install uninstall lint vet fmt tidy release help

all: lint vet test build

## build: Build the binary for the current platform
build:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME) ./cmd/tm

## build-all: Build binaries for all supported platforms
build-all: build-linux build-darwin build-windows

build-linux:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)_linux_amd64 ./cmd/tm
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)_linux_arm64 ./cmd/tm
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)_linux_arm ./cmd/tm

build-darwin:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)_darwin_amd64 ./cmd/tm
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)_darwin_arm64 ./cmd/tm

build-windows:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)_windows_amd64.exe ./cmd/tm
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)_windows_arm64.exe ./cmd/tm

## test: Run all tests
test:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

## test-integration: Run integration tests only
test-integration:
	go test -v -race ./test/...

## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

## install: Install the binary to GOPATH/bin
install:
	CGO_ENABLED=0 go install -ldflags="$(LDFLAGS)" ./cmd/tm

## uninstall: Remove the binary from GOPATH/bin
uninstall:
	rm -f $(GOPATH)/bin/$(APP_NAME)

## lint: Run golangci-lint
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found; running go vet instead"; \
		go vet ./...; \
	fi

## vet: Run go vet
vet:
	go vet ./...

## fmt: Format Go source files
fmt:
	go fmt ./...

## tidy: Tidy go modules
tidy:
	go mod tidy

## release: Create a release snapshot (requires goreleaser)
release:
	goreleaser release --snapshot --clean

## help: Show this help message
help:
	@echo "Available targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
