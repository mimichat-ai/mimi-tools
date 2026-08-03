APP_NAME := mimi-tools
BINARY   := bin/mimi-tools
MAIN     := cmd/mimi-tools/main.go

# Version info (overridable, defaults to git describe)
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILDTIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS   := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILDTIME)

.PHONY: lint
lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Please install it with: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2" && exit 1)
	golangci-lint run ./...

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: run
run:
	go run $(MAIN)

.PHONY: test
test:
	go test -race -v -count=1 ./...

.PHONY: build
build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) $(MAIN)

.PHONY: ci
ci: lint test build

# ─── Android variant ────────────────────────────────────────────────────────
# Build and run the Android development sandbox image (Dockerfile.android)
# Requires podman (or docker). See DEPLOYMENT-ANDROID.md for details.

ANDROID_IMAGE := mimi-tools-android:latest

.PHONY: build-android
build-android:
	podman build -f Dockerfile.android -t $(ANDROID_IMAGE) .

.PHONY: run-android
run-android:
	podman run -it --rm -p 2334:2333 -v /projects:/projects $(ANDROID_IMAGE)