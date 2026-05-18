BINARY      := confluence-cli
PKG         := github.com/angelmsger/confluence-cli
CONSTANTS   := $(PKG)/pkg/constants
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_TIME  := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -s -w \
	-X '$(CONSTANTS).Version=$(VERSION)' \
	-X '$(CONSTANTS).Commit=$(COMMIT)' \
	-X '$(CONSTANTS).BuildTime=$(BUILD_TIME)'

# Install destination: prefer `go env GOBIN`, fall back to $GOPATH/bin.
INSTALL_DIR := $(shell go env GOBIN)
ifeq ($(INSTALL_DIR),)
INSTALL_DIR := $(shell go env GOPATH)/bin
endif

# Where the companion Skill is copied by `make install-skill`.
SKILL_DIR ?= $(HOME)/.claude/skills

.PHONY: build test e2e e2e-live lint fmt vet cross docs install install-skill tidy clean

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/confluence-cli

test:
	go test ./...

e2e: build
	./scripts/e2e.sh

e2e-live: build
	CONFLUENCE_E2E_LIVE=1 ./scripts/e2e.sh

lint: fmt vet

fmt:
	gofmt -l -w .

vet:
	go vet ./...

tidy:
	go mod tidy

cross:
	VERSION=$(VERSION) COMMIT=$(COMMIT) ./scripts/build.sh

# Regenerate the CLI reference under docs/cli/ from the cobra command tree.
docs:
	go run ./cmd/gen-docs

install: build
	mkdir -p $(INSTALL_DIR)
	cp bin/$(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "installed $(INSTALL_DIR)/$(BINARY)"

# Install the companion Skill by copying it into the Claude Code skills dir.
# Prefer `npx skills add` when Node is available — see docs/installation.md.
install-skill:
	mkdir -p $(SKILL_DIR)
	rm -rf $(SKILL_DIR)/confluence
	cp -R skills/confluence $(SKILL_DIR)/confluence
	@echo "installed skill -> $(SKILL_DIR)/confluence"

clean:
	rm -rf bin dist
