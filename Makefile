GO ?= go
AITOK_CACHE_DIR ?= $(CURDIR)/.cache/aitok
GOCACHE ?= $(AITOK_CACHE_DIR)/go-build
GOMODCACHE ?= $(AITOK_CACHE_DIR)/go-mod
COMMIT_MSG_FILE ?= .git/COMMIT_EDITMSG

.PHONY: setup check test test-harness vet build validate validate-pr-body commitlint

setup:
	git config core.hooksPath .githooks

check:
	@test -z "$$(gofmt -l $$(git ls-files '*.go'))"
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) vet ./...

test:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) test ./...

test-harness:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) test ./harness

vet:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) vet ./...

build:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) build ./cmd/aitok

validate-pr-body:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) run ./tools/validate-pr-body

commitlint:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) run ./tools/commitlint --edit "$(COMMIT_MSG_FILE)"

validate: check test build
