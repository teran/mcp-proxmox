# Makefile — build-system interface (R07) for mcp-proxmox.
#
# CI binds to these targets instead of raw language-specific commands, so the
# same targets work across CI providers (GitLab CI + Forgejo Actions).
#
# Standard interface (R07): build, test, lint.
#   build  — produce binary artifacts (goreleaser snapshot) into dist/
#   test   — hermetic unit tests (-race) + coverage >= 95% gate (C1)
#   lint   — golangci-lint + vet + gofmt + go-arch-lint + gosec + govulncheck
#
# Dedicated hard-gate jobs (NOT part of the standard interface; CI runs them
# as separate stages/jobs so failures are isolated):
#   mutation — gremlins mutation testing, hard gate (C2/C8)
#   secrets  — gitleaks secret scan over the full git history (C3/N28)
#
# This is a Local (stdio-only) server. `e2e` is an OPTIONAL, harness-backed
# target (build-tagged `//go:build e2e`, requires Docker): it runs an in-repo
# emulator of the Proxmox VE / PBS REST APIs in a container via
# github.com/teran/go-docker-testsuite and drives the real server over stdio.
# It is NOT part of the standard interface and does not run in the default
# `go test ./...` pass. There is no CI-published container image, so
# `container-image` stays intentionally UNDECLARED (N31 — a CI job must never
# invoke an undeclared optional target). See SPEC.md §4.1 / §9.

GO        ?= go
SHELL     := bash
COVER_OUT := coverage.out
COVER_CORE := coverage-core.out

.PHONY: build test lint mutation secrets e2e

## build — produce binary artifacts via goreleaser (single artifact in dist/)
build:
	goreleaser build --snapshot --clean

## test — hermetic unit tests (-race) + coverage >= 95% gate (C1)
test:
	$(GO) test -race ./...
	$(GO) test ./... -coverprofile=$(COVER_OUT)
	awk '!/\/cmd\/mcp-proxmox\//' $(COVER_OUT) > $(COVER_CORE)
	cov=$$($(GO) tool cover -func=$(COVER_CORE) | tail -1 | awk '{print $$NF}' | tr -d '%'); \
		echo "cover-core: $$cov%"; \
		if ! awk -v c="$$cov" 'BEGIN { exit !(c >= 95) }'; then \
			echo "coverage $$cov% is below the 95% gate"; \
			exit 1; \
		fi

## lint — static analysis, formatting, arch + security + vulnerability scans
lint:
	golangci-lint run ./...
	$(GO) vet ./...
	unformatted=$$(gofmt -l .); \
		if [ -n "$$unformatted" ]; then \
			echo "gofmt required for the following files:"; \
			echo "$$unformatted"; \
			exit 1; \
		fi
	go-arch-lint check
	gosec ./...
	govulncheck ./...

## mutation — gremlins mutation testing on core packages; HARD GATE (C2/C8)
mutation:
	gremlins unleash --workers 4 --timeout-coefficient 50 --threshold-efficacy=80 --threshold-mcover=80 ./application -E '.*mocks.*'
	gremlins unleash --workers 4 --timeout-coefficient 50 --threshold-efficacy=80 --threshold-mcover=80 ./domain

## secrets — gitleaks scan over the FULL git history; HARD GATE (C3/N28)
secrets:
	gitleaks detect --source . --redact --verbose

## e2e — harness-backed end-to-end suite (//go:build e2e; requires Docker)
e2e:
	$(GO) test -tags e2e ./e2e/...
