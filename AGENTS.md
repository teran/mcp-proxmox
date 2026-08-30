# AGENTS.md — Developer & AI-Agent Guide

**Language:** English. All code, comments, identifiers, and documentation are in
English.

Guidance for AI agents and human contributors working in this repository.

**Project:** `mcp-proxmox` — a local MCP server (Model Context Protocol) written in Go
that exposes the **Proxmox VE** and **Proxmox Backup Server (PBS)** REST APIs as
callable tools for LLM agents, IDEs, and assistants. Both backends are **optional**
and enabled only when an endpoint + API token is configured.

**Design authority:** see `SPEC.md`. This file is the operational / agent-facing
companion (always in English). Do not deviate from `SPEC.md`; if you find a
contradiction, raise it with the architect.

---

## 1. Roles and responsibilities

### Architect
- Owns the layered design (Clean Architecture / DDD) and the **dependency rule**
  (dependencies point inward only).
- Enforces the **no `internal`** package rule — every package lives at the repo
  root and is public (`domain/`, `application/`, `adapter/`, `cmd/`).
- Owns the two-gateway shape (`port.PVEGateway`, `port.PBSGateway`) and the
  `port.TokenSource` / transport seams (local stdio now, remote OAuth2 later).
- Reviews structural changes: new packages, cross-layer imports, port/interface
  boundaries. Maintains `SPEC.md` and the architecture invariants (verified by
  `go-arch-lint`).

### Developer
- Implements features following the layer rules: domain → application → adapter,
  with dependency inversion (application depends only on domain ports).
- Writes code covered by unit tests (target **95%+** on `cover-core`).
- Runs `golangci-lint`, `go test`, `go-arch-lint`, and gremlins locally before pushing.
- Registers MCP tools only in `adapter/mcp`; a backend's tools only when its
  service is non-nil. Read-only (query) tools are always registered for an
  enabled backend; mutation tools are registered only when `Deps.EnableMutations`
  is true (gate UX TBD — SPEC.md §2.5).

### QA
- Owns the test strategy: hermetic unit tests (port mocks), `httptest` mocks of
  the **real** Proxmox VE / PBS JSON APIs, and coverage gates.
- Ensures **95%+** coverage (`cover-core`) and acceptable gremlins mutation
  score (≥80% on `domain` and `application`).
- Reviews test quality, not just quantity; adds tests where mutation shows gaps.

### Security
- Reviews handling of secrets — Proxmox **API tokens** are **never** logged, never
  exposed in tool output, and never per-call parameters (they are read once per
  backend from config).
- Runs `gosec` and reviews HTTP clients for TLS, SSRF, body-close, and injection
  risks.
- Enforces **TLS verification is always on** — never an `insecure_skip_verify`
  switch; self-signed/custom CAs are supported only via a `ca_cert_path` per
  backend (used to build the client TLS `RootCAs`).
- Verifies the auth adapters don't leak credentials via error messages or logs.

### DevOps
- Owns the CI pipeline (lint → arch → test → cover → mutation → build → sec).
- Maintains reproducible local run (goreleaser build / `go run ./cmd/mcp-proxmox`) over stdio.
- Owns **release/build via [goreleaser](https://goreleaser.com/)** (`.goreleaser.yml`):
  cross-compiles binaries/archives and auto-releases from git tags; the `Dockerfile`
  consumes the goreleaser artifact (it does **not** compile).
- Manages Go toolchain/dependency updates and gremlins/golangci-lint/goreleaser
  versions (use the latest available tool versions — do not pin unless a
  compatibility issue forces it).

---

## 2. Project rules and conventions

### Go
- Go **1.27.0** (matches the org's flagship `mcp-regcloud`; satisfies the go-sdk
  ≥ 1.24 requirement). Module name: `github.com/teran/mcp-proxmox`.
- Idiomatic Go: `gofmt`/`goimports`, errors wrapped with `%w`, no panics in library code.
- Use the official MCP SDK: `github.com/modelcontextprotocol/go-sdk v1.7.0` (import `.../mcp`).
- Logging: **logrus**; the core only ever sees `port.AppLogger` /
  `port.CtxLogger`. Use `Infof` for lifecycle events and `Warnf` for anomalies in
  use cases; tool handlers log errors via `Errorf` with the tool name
   (`"pve_vm_start: %v"`). `domain/model` never logs (pure types).
 - Log destination: the server runs over stdio (clients launch it as a subprocess
   and don't capture stderr), so when `LOG_LEVEL` is set, logs are written to
   `/tmp/mcp-proxmox.log` (`O_APPEND|O_CREATE|O_WRONLY`, mode `0600`) at that
   level; otherwise the default `info` level and default output are used. One
   `session_id` per MCP session is shared between the stdio transport and the
   request context; per-request `request_id` is added by `WithSession`.
 - Context-aware logging: application services log via `port.CtxLogger`, which
  type-asserts to the optional `port.RequestAwareLogger` and tags lines with the
  MCP `session_id`/`request_id` from `ctx`; it falls back to the plain `AppLogger`
  for test dummies. Tool handlers use the ctx-aware `toolLogger`
  (`adapter/mcp/tool_logger.go`).
- **Secrets never logged:** API tokens (the `PVEAPIToken`/`Authorization` header)
  are never logged — only the header *name* may appear at trace level, never its
  value. `ca_cert_path` content is never logged. Request/response bodies are never
  logged at a level that would include token material; trace-level request logging
  logs method + URL path only.
- Tool input schemas via `jsonschema:"..."` struct tags (`json` tag for the wire
  name, `jsonschema` tag for the human description, `omitempty` for optional fields).

### DDD / Clean Architecture — no `internal`
- Layers at repo root: `domain/`, `application/`, `adapter/`, `cmd/`.
- **Never** introduce an `internal/` directory (customer requirement).
- `domain/model` — pure types/value objects; std only (`encoding/json`, `time`).
- `domain/port` — contracts (`PVEGateway`, `PBSGateway`, `TokenSource`, `AppLogger`/
  `CtxLogger`) + sentinel errors + `WithToken`/`TokenFromContext` helpers; std only.
- `application/` — use cases / orchestration, thin services over the ports, `App`
  facade; depends only on `domain/*`.
- `adapter/*` — concrete implementations (Proxmox/PBS HTTP clients, token sources,
  logging, MCP tool registry); depends on domain + (each) its own external lib.
- `cmd/mcp-proxmox` — composition root: flags, config, mode selection, wiring.
- **Dependency rule:** dependencies point inward only. `application` never imports
  `adapter/*`; `net/http` is confined to `adapter/pve` and `adapter/pbs`; logrus is
  confined to `adapter/logging` (plus select adapter files for trace/debug); the
  go-sdk is confined to `adapter/mcp` and `cmd/mcp-proxmox`.
- Composition/DI wiring happens only in `cmd/` (the composition root).
- Structural boundaries are enforced by `go-arch-lint` + `depguard` + code review.

### Two gateways, one shared shape
- PVE and PBS are separate products with different object models — two separate
  secondary ports (`port.PVEGateway`, `port.PBSGateway`), each implemented by its
  own adapter (`adapter/pve`, `adapter/pbs`). They share `port.TokenSource`,
  `port.AppLogger`, and error-mapping conventions, but **never share an entity set**.

### Optional enablement via optional injection
- The `App` facade exposes `WithPVE(gw port.PVEGateway)` / `WithPBS(gw port.PBSGateway)`
  options. When a backend is not configured, the option is simply not applied and
  the service is nil; `adapter/mcp` registers a backend's tools only when its
  service is non-nil. `ping`/`status` are always registered and report which
  backends are active.

### Token / auth
- Auth is hidden behind `port.TokenSource`, which returns a ready-to-send
  `Authorization` header value:
  ```go
  type TokenSource interface {
      AuthorizationHeader(ctx context.Context) (string, error)
  }
  ```
- **Local:** `adapter/token.StaticTokenSource` reads the configured token and
  returns `PVEAPIToken=<user@realm!tokenid=uuid>` verbatim; PVE and PBS each get
  their own instance bound to their own configured token.
- **Future remote:** an `OAuth2TokenSource` (or a per-request header relay) can
  return `Bearer <token>`. The gateways consume only the returned header string.
- **API-token only** — no ticket/CSRF flow. Tokens are read once per backend from
  config and re-used on every call; they are **never** per-call parameters and
  **never** logged.

### Tests
- Coverage target: **95%+** across real code (`cover-core`, which excludes
  generated mocks and the `cmd/mcp-proxmox` composition root) — deliberately
  stricter than the org's 85% reference.
- Use mocks for `domain/port` interfaces in application/interface tests.
- **Hermetic tests:** no real network. `httptest` mocks of the **real** Proxmox VE
  and PBS JSON APIs, port mocks, and stubs. They always run as part of
  `go test ./...` — no build tag or env gate.
- Mutation testing via **gremlins**; aim for ≥80% mutation score on `domain` and
  `application` (generated mocks excluded).

### Config & secrets
- Configuration is read from **environment variables** via
  `kelseyhightower/envconfig` (`adapter/config`): `PVE_ENDPOINT`/`PVE_TOKEN`
  (/`PVE_CA_CERT_PATH`), `PBS_ENDPOINT`/`PBS_TOKEN` (/`PBS_CA_CERT_PATH`), and
  `LOG_LEVEL`. There is **no YAML/config file**.
- **Dynamic availability:** a backend is active only if **both** its `endpoint`
  and its `token` are present; otherwise it is **disabled** and its tools must
  **not** be registered. Nothing fails when a backend is absent.
- Secrets must **never** be committed, hard-coded, logged, or exposed in tool output.
- The env vars the server reads are listed in `adapter/config/config.go`
  (`config.Load()`); tokens are never logged and never per-call parameters.

### Quality gates
- `golangci-lint run` must pass with no findings (`.golangci.yml`, v2; linters
  incl. `errcheck`, `govet`, `staticcheck`, `ineffassign`, `unused`, `revive`,
  `depguard`, `nakedret`, plus `gosec`).
- `gosec` must pass.
- `govulncheck ./...` must report no vulnerabilities affecting the code.
- `go test -race ./...` must pass (race detector is part of CI).
- Coverage must meet the **95%** threshold on `cover-core`. **The CI pipeline
  fails the build when total `cover-core` is below 95%** — it is a hard gate,
  not just a local convention (both `.gitlab-ci.yml` and
  `.forgejo/workflows/ci.yml` enforce it).
- gremlins mutation score must not regress below 80% on `domain`/`application`.
- No secrets in code, config, logs, or tool output.

---

### TDD workflow

All fixes and features follow a **TDD workflow** with **isolated contexts**:
- **@qa** writes the tests (in an isolated context) against the `domain/port`
  contract and the architecture blueprint.
- **@developer** writes the implementation (in an isolated context) against the
  same contract.
- The two run in parallel; the task manager reconciles them and runs the quality
  gates (lint → arch → test → cover ≥95% → mutation → build → sec → govulncheck)
  before merge. Neither agent edits the other's files: QA owns `*_test.go`, the
  developer owns non-test `.go` code.

---

## 3. Commands

There is **no Makefile** — builds are done with **goreleaser** (`.goreleaser.yml`),
and the quality gates are run directly.

```bash
# build / release (goreleaser)
goreleaser build --snapshot --clean     # local snapshot build into dist/
goreleaser release --clean              # release from a git tag
go build ./...                          # quick compile check

# quality gates
golangci-lint run ./...                 # lint
go-arch-lint check                      # architecture / dependency rule
go test ./...                           # hermetic tests (no network)
go vet ./...
gofmt -l .                              # formatting check
gosec ./...                             # security scan
gremlins unleash --workers 4 --timeout-coefficient 50 ./application -E '.*mocks.*'
gremlins unleash --workers 4 --timeout-coefficient 50 ./domain   # mutation on core

# real-code coverage (excludes generated mocks and cmd/mcp-proxmox)
go test ./... -coverprofile=coverage.out
awk '!/\/cmd\/mcp-proxmox\//' coverage.out > coverage-core.out
go tool cover -func=coverage-core.out | tail -1
```

**Version/build:** the version is injected via ldflags from the nearest git tag
(`main.version={{ .Version }}` in `.goreleaser.yml`; default `v0.0.0-dev`) and
printed by the `-version` flag (`mcp-proxmox <version>`). The `Dockerfile`
consumes the goreleaser artifact from `dist/` — it does **not** compile; build
the binary first with `goreleaser build --snapshot --clean`.

---

## 4. Development workflow

### Branching
- Default branch: `main`.
- One feature/bugfix per branch, named: `feat/<slug>`, `fix/<slug>`, `test/<slug>`, `docs/<slug>`.
- Branch from latest `main`; keep branches short-lived.

### Commits
- Conventional commits: `feat:`, `fix:`, `test:`, `docs:`, `refactor:`, `chore:`.
- Keep commits focused; no unrelated changes.

### Merge request (MR) / review
- Open an MR (or PR on GitHub) against `main`.
- Fill in description: what, why, what was tested, coverage/mutation impact.
- Require **at least one approving review** from a human/agent with relevant role
  (architect for structural changes, security for auth, qa for tests).
- CI must be green: lint → arch → test → cover (95%) → mutation → build → sec.
- Address review comments; rebase/squash as appropriate; no force-push to shared branches.
- Do not merge with failing checks or below-threshold coverage/mutation.

### Review checklist (per role)
- **Architect:** layer boundaries intact? No `internal`? Dependency rule holds?
  Ports clean? Two-gateway shape respected?
- **Developer:** idiomatic Go, errors wrapped, no panics, no secrets, tool schemas complete.
- **QA:** coverage added for new logic; mutation reveals no surviving mutants in hot paths.
- **Security:** auth adapters don't leak tokens; HTTP client safe (TLS, bodyclose, SSRF).

---

## 5. Testing strategy summary

| Layer | Approach |
|---|---|
| `domain/model` | JSON serialization tests for every entity |
| `domain/port` | `WithToken`/`TokenFromContext`, `CtxLogger` fallback, error helpers |
| `application/*` | through `PVEGateway`/`PBSGateway` mocks (no network) |
| `adapter/pve` | `httptest` mock of the PVE API (assert method, `Authorization` header, URL path; canned JSON for nodes/vms/lxc/storage/tasks) |
| `adapter/pbs` | `httptest` mock of the PBS API (datastores/snapshots/verify/prune) |
| `adapter/token` | config parsing + header building cases (target 100%) |
| `adapter/logging` | level mapping, format, request-aware tagging |
| `adapter/mcp` | each tool: success + error via a gateway stub; assert `IsError` on failure; assert tools absent when a backend is disabled |

Table-driven cases cover success, API `errors`, `401 → ErrUnauthorized`,
`500 → retry then UpstreamError`, empty `data`, invalid JSON, missing token, and
(for `adapter/mcp`) backend-disabled tool absence.
