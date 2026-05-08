GO ?= go
AITOK_CACHE_DIR ?= /tmp/aitok-cache
GOCACHE ?= $(AITOK_CACHE_DIR)/go-build
GOMODCACHE ?= $(AITOK_CACHE_DIR)/go-mod

.PHONY: check test test-harness vet build validate validate-pr-body

check:
	@test -z "$$(gofmt -l .)"
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

validate: check test build
