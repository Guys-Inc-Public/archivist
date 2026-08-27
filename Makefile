BINARY  := archivist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
PKG     := github.com/Guys-Inc-Public/archivist/cmd/archivist
LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

.DEFAULT_GOAL := help
.PHONY: help build test cover lint fmt tidy round-trip clean

help: ## Show this help
	@awk 'BEGIN{FS=":.*##"} /^[a-z-]+:.*##/{printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build ./bin/archivist
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o bin/$(BINARY) $(PKG)

test: ## Run tests with the race detector
	go test -race ./...

cover: ## Run tests and open a coverage report
	go test -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out

lint: ## Run golangci-lint
	golangci-lint run

fmt: ## Format all Go source
	gofmt -l -w .

tidy: ## Tidy go.mod / go.sum
	go mod tidy

round-trip: build ## Build a repository and install from it with a real apt (needs root; run in a container)
	./script/round-trip.sh

clean: ## Remove build output
	rm -rf bin/ dist/ repo/ coverage.out
