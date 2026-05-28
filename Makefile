GO ?= go
AITOK_CACHE_DIR ?= $(CURDIR)/.cache/aitok
GOCACHE ?= $(AITOK_CACHE_DIR)/go-build
GOMODCACHE ?= $(AITOK_CACHE_DIR)/go-mod
COMMIT_MSG_FILE ?= .git/COMMIT_EDITMSG
COMMIT_RANGE ?=

.PHONY: setup check test test-packages test-harness vet build run validate validate-pr-body commitlint commitlint-range

setup:
	git config core.hooksPath .githooks

check:
	@test -z "$$(gofmt -l $$(git ls-files '*.go'))"
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) vet ./...

test:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) test ./...

test-packages:
	@test -n "$(PKGS)" || (echo 'PKGS is required, for example: make test-packages PKGS="./internal/query ./internal/report"' >&2; exit 2)
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) test $(PKGS)

test-harness:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) test ./harness

vet:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) vet ./...

build:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) build ./cmd/aitok

run:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) run ./cmd/aitok $(ARGS)

validate-pr-body:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) run ./tools/validate-pr-body

commitlint:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) run ./tools/commitlint --edit "$(COMMIT_MSG_FILE)"

commitlint-range:
	@test -n "$(COMMIT_RANGE)" || (echo 'COMMIT_RANGE is required, for example: make commitlint-range COMMIT_RANGE="origin/main..HEAD"' >&2; exit 2)
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) run ./tools/commitlint --range "$(COMMIT_RANGE)"

validate: generate check test build

generate:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) generate ./internal/buildinfo/...
