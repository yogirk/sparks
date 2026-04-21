.PHONY: build test test-race test-cover lint fmt vet tidy install clean help

BIN := sparks
PKG := ./...

help:
	@echo "Sparks dev targets:"
	@echo "  make build       - build ./sparks binary in repo root"
	@echo "  make install     - go install ./cmd/sparks"
	@echo "  make test        - go test ./..."
	@echo "  make test-race   - go test -race ./..."
	@echo "  make test-cover  - go test with coverage report"
	@echo "  make lint        - go vet + staticcheck (if installed)"
	@echo "  make fmt         - gofmt -w ."
	@echo "  make tidy        - go mod tidy"
	@echo "  make clean       - remove binary + coverage artifacts"

build:
	go build -o $(BIN) ./cmd/sparks

install:
	go install ./cmd/sparks

test:
	go test $(PKG)

test-race:
	go test -race $(PKG)

test-cover:
	go test -coverprofile=cover.out $(PKG)
	go tool cover -func=cover.out | tail -1

lint:
	go vet $(PKG)
	@command -v staticcheck >/dev/null 2>&1 && staticcheck $(PKG) || echo "staticcheck not installed (optional)"

fmt:
	gofmt -w .

tidy:
	go mod tidy

clean:
	rm -f $(BIN) cover.out

.PHONY: sync-contract verify-contract

sync-contract:
	cp sparks-contracts.md internal/contract/contract.md

verify-contract:
	@diff -q sparks-contracts.md internal/contract/contract.md > /dev/null && echo "contract: in sync" || (echo "contract drift! run: make sync-contract" && exit 1)
