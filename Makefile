.PHONY: build test vet fmt-check lint-content run-cli run-web audit migrate e2e seed clean

GO ?= go
BIN_DIR ?= bin

build:
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/sagescore     ./cmd/sagescore
	$(GO) build -o $(BIN_DIR)/sagescore-web ./cmd/sagescore-web

test:
	$(GO) test ./... -race -count=1

vet:
	$(GO) vet ./...

fmt-check:
	@out=$$($(GO) run mvdan.cc/gofumpt@latest -l . 2>/dev/null || gofmt -l .); \
	if [ -n "$$out" ]; then \
		echo "gofmt violations:"; echo "$$out"; exit 1; \
	fi

lint-content:
	$(GO) run ./scripts/lint-content .

run-cli: build
	$(BIN_DIR)/sagescore

run-web: build
	$(BIN_DIR)/sagescore-web

# Phase 1+ — runs an audit against DOMAIN.
audit: build
	@if [ -z "$(DOMAIN)" ]; then echo "usage: make audit DOMAIN=example.com"; exit 2; fi
	$(BIN_DIR)/sagescore audit $(DOMAIN)

# Phase 2+ — applies migrations (currently AutoMigrate at server start).
migrate:
	@echo "(Phase 2: migrations run automatically at server startup via GORM AutoMigrate)"

# Phase 2+ — end-to-end test against a synthetic in-repo site.
e2e:
	@echo "(Phase 2: e2e harness lands with the web service)"

# Phase 4 — pre-seeds 200 founder-curated audits against production.
seed:
	@echo "(Phase 4: seed run against production)"

clean:
	rm -rf $(BIN_DIR)
