MODULE_NAME := mcp-proxmox
BINARY := bin/mcp-proxmox

# Version injected via ldflags from the nearest git tag (semver), with a
# v0.0.0-dev fallback. Override explicitly: make build VERSION=v1.2.3
VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null || echo v0.0.0-dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build run release snapshot test cover cover-core cover-html lint fix sec arch mutation format tidy ci clean image

build: ## build the server binary
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/mcp-proxmox

run: ## run the server over STDIO
	go run ./cmd/mcp-proxmox

release: ## goreleaser full release (from git tags)
	goreleaser release --clean

snapshot: ## goreleaser local snapshot build (no publish)
	goreleaser release --snapshot --clean

test: ## run tests with coverage
	go test ./... -cover

cover: ## run tests and print coverage by function
	go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out

cover-core: ## coverage of real code (excludes generated mocks and the cmd composition root)
	go test ./... -coverprofile=coverage.out
	awk '!/\/domain\/port\/mocks\// && !/\/cmd\/mcp-proxmox\//' coverage.out > coverage-core.out
	go tool cover -func=coverage-core.out | tail -1

cover-html: ## open HTML coverage report
	go tool cover -html=coverage.out

lint: ## run golangci-lint
	golangci-lint run ./...

fix: ## auto-fix lint issues
	golangci-lint run --fix ./...

sec: ## run gosec
	gosec ./...

arch: ## check architecture / dependency rules
	go-arch-lint check

mutation: ## run gremlins mutation testing on domain + application
	gremlins unleash --workers 4 --timeout-coefficient 50 ./application
	gremlins unleash --workers 4 --timeout-coefficient 50 ./domain -E '.*mocks.*'

format: ## format code
	gofmt -w . && goimports -w .

tidy: ## tidy go modules
	go mod tidy

image: ## build the linux/amd64 binary with goreleaser, then the scratch image
	GOOS=linux GOARCH=amd64 goreleaser build --snapshot --clean \
		--single-target --id mcp-proxmox --output dist/mcp-proxmox
	docker build -t $(MODULE_NAME) .

clean: ## remove build artifacts
	rm -rf bin dist coverage.out coverage-core.out

ci: lint arch test cover mutation build ## full CI pipeline
