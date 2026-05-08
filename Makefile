GO ?= go
GOCACHE ?= /private/tmp/aitok-gocache
GOMODCACHE ?= /private/tmp/aitok-gomodcache

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
