# mcp-proxmox — Technical Specification

**Language:** English (fixed by the user — do not translate).
**Module:** `github.com/teran/mcp-proxmox`
**Go:** 1.27.3
**Status:** implemented — this document is the authoritative technical reference
and is kept in sync with the implementation.

This document is the authoritative technical reference for the `mcp-proxmox`
project: the source of truth for the intended architecture, tool registry, run
modes, authentication model, and quality requirements. The server is
**implemented**; `README.md`, `AGENTS.md`, and the CI configuration document the
operational reality and are kept in sync with this SPEC.

---

## 1. Overview and purpose

`mcp-proxmox` is a **Model Context Protocol (MCP) server** that exposes the
REST APIs of **Proxmox VE** and **Proxmox Backup Server (PBS)** as a set of
tools that MCP clients (LLM agents, IDEs, assistants) can call to inspect and
operate a Proxmox virtualization cluster and its backup server.

- **Proxmox VE** (`https://pve.example.com:8006`, path prefix `/api2/json`):
  nodes, QEMU virtual machines (VMs), LXC containers, storage, network,
  cluster/status, and tasks/logs.
- **Proxmox Backup Server** (`https://pbs.example.com:8007`, path prefix
  `/api2/json`): datastores, backups/snapshots, notes, verify, and prune.
- SDK: [`github.com/modelcontextprotocol/go-sdk v1.8.0`](https://github.com/modelcontextprotocol/go-sdk).
- Logging: `github.com/sirupsen/logrus` (all logs through logrus; the core only
  sees a minimal `port.AppLogger`).

Both backends are **optional**: each is *enabled only if the user supplies an
endpoint and an API token* for it (see §4). A backend that is not configured is
absent — its tools are not registered and nothing fails. Initially the server
runs **locally** over the **stdio** transport; the transport and authentication
boundaries are designed so a future **remote (HTTP) mode + OAuth2** can be added
later without rearchitecting (see §2.4 and §4).

> **Auth model note.** Both Proxmox APIs authenticate with **API tokens**, which
> are long-lived, server-issued credentials scoped by privilege separation. This
> is fundamentally different from the access/refresh-JWT scheme of `mcp-regcloud`,
> whose *local-only lock-in* was caused by a local config store needed to refresh
> short-lived tokens. Because a Proxmox API token is stateless and used verbatim
> on every request, the same `port.TokenSource` abstraction serves both local and
> future remote modes cleanly — this project addresses that explicitly (§2.4).

---

## 2. Architecture

`mcp-proxmox` follows **Clean Architecture / Domain-Driven Design** with a
**dependency rule that points inward only**. A hard customer constraint is that
**no `internal` package is used** — every package lives at the repo root and is
public, and the core (`domain` + `application`) must not depend on the MCP SDK,
the low-level `net/http`, logrus, or any Proxmox client.

### 2.1 Layers and responsibility

```
domain/model  ←  domain/port  ←  application  ←  adapter/*  ←  cmd/mcp-proxmox
     ↑              ↑              ↑              ↑
  (pure types)   (contracts)    (use cases)   (implementations)
```

| Layer | Path | Responsibility | External deps |
|---|---|---|---|
| Domain model | `domain/model` | Entities / value objects (`VM`, `LXCContainer`, `Node`, `Storage`, `Datastore`, `Backup`, `Task`, …) | std only (`encoding/json`, `time`) |
| Domain ports | `domain/port` | Interfaces (`PVEGateway`, `PBSGateway`, `TokenSource`, `AppLogger`) + sentinel errors + context helpers | std only |
| Application | `application/…` | Use cases / orchestration, thin services over the ports, `App` facade | `domain/*` only |
| Adapters | `adapter/…` | Concrete implementations: Proxmox/PBS HTTP clients, token sources, logging, MCP tool registry | depends on domain + (each) its own external lib |
| Composition root | `cmd/mcp-proxmox` | Flags, config, mode selection, wiring, startup | sees everything |

### 2.2 Dependency graph (imports)

```
cmd/mcp-proxmox ── application, adapter/{pve,pbs,token,logging,mcp},
                   domain/{model,port}, github.com/modelcontextprotocol/go-sdk/mcp

application ── domain/port, domain/model

adapter/pve      ── domain/port, domain/model     (resty.dev/v3, encoding/json — ONLY here)
adapter/pbs      ── domain/port, domain/model     (resty.dev/v3, encoding/json — ONLY here)
adapter/token    ── domain/port                   (os, context)
adapter/logging  ── domain/port                   (logrus, log/slog)
adapter/mcp      ── application, domain/{model,port}, go-sdk/mcp
domain/port      ── domain/model
domain/model     ── (std only)
```

There are **no import cycles**: ports never reference adapters; `application`
never imports `adapter/*`; outbound HTTP is done exclusively via
**`resty.dev/v3`** confined to `adapter/pve` and `adapter/pbs` (never a bare
`http.Client{}`/`http.NewRequestWithContext` — SPEC G9/N23); logrus is confined
to `adapter/logging` (plus select adapter files for trace/debug); the go-sdk is
confined to `adapter/mcp` and `cmd/mcp-proxmox`.

### 2.3 Package layout

```
mcp-proxmox/
├── cmd/mcp-proxmox/main.go      # composition root: flags, config, mode, wiring
├── domain/
│   ├── model/                   # entities: node.go, vm.go, lxc.go, storage.go,
│   │                            #   network.go, cluster.go, task.go,
│   │                            #   datastore.go, backup.go, verify.go, prune.go
│   └── port/                    # PVEGateway, PBSGateway, TokenSource,
│                                #   AppLogger/CtxLogger, sentinel errors,
│                                #   WithToken helpers
├── application/
│   ├── app.go                   # App facade + WithPVE/WithPBS options
│   ├── pve_service.go           # PVE use cases (nodes, vms, lxc, storage, ...)
│   ├── pbs_service.go           # PBS use cases (datastores, backups, verify, prune)
│   └── system_service.go        # ping, status
├── adapter/
│   ├── pve/                     # Proxmox VE HTTP client → port.PVEGateway
│   ├── pbs/                     # Proxmox Backup Server HTTP client → port.PBSGateway
│   ├── token/                   # StaticTokenSource (config API token) → port.TokenSource
│   ├── logging/                 # logrus setup + slog→logrus adapter + AppLogger
│   └── mcp/                     # go-sdk server assembly + tool registry
├── go.mod / go.sum
├── Dockerfile                   # minimal scratch runtime (consumes goreleaser artifact)
├── .goreleaser.yml              # cross-compile + semver version injection (ldflags)
├── .go-arch-lint.yml            # clean-architecture boundaries (go-arch-lint check)
├── .golangci.yml                # golangci-lint v2 + depguard clean-arch rule
├── gremlins.toml                # mutation-testing config (domain/application)
├── .github/workflows/ci.yml     # GitHub Actions CI (make lint/test/mutation/secrets/build)
├── .github/workflows/release.yml # GitHub Actions release (goreleaser on tags)
├── LICENSE                      # Apache-2.0, © 2026 Igor Shishkin
├── SPEC.md                      # this document
├── AGENTS.md                    # agent-facing developer/ops guide (present)
└── README.md                    # user-facing documentation (present)
```

### 2.4 Architectural decisions and rationale

**Clean architecture without `internal`.** Every package lives at the repo root
and is public; the core (`domain` + `application`) has zero dependencies on the
go-sdk, the low-level `net/http`, logrus, or any Proxmox client. This is a customer requirement
and is enforced by `.go-arch-lint.yml` (`go-arch-lint check`), not
by a hidden `internal/` directory. Dependencies point inward only through the
`domain/port` interfaces.

**Outer layer name: `adapter/`, not `infrastructure/`.** The outermost
implementation layer is deliberately named **`adapter/`** — the canonical Clean
Architecture term for *interface adapters* (HTTP clients/gateways, presenters,
controllers) — rather than the MCP-server skill's literal `infrastructure/`
label. This is a **recorded, deliberate decision**: the skill's
`infrastructure/` is understood to mean the same "outer ring of
implementations" layer, but we follow the standard Clean Architecture
vocabulary already used across the org's fleet (`mcp-regcloud`,
`mcp-entertainment`). `go-arch-lint` (`.go-arch-lint.yml`) and `depguard`
(`.golangci.yml`) are configured to this exact `adapter/` layout, so the
boundary is enforced structurally, not just by convention.

**Module namespace — decision.** The repository is hosted on **GitHub.com**
(`github.com/teran/mcp-proxmox`), so the Go module path
`github.com/teran/mcp-proxmox` is the repo's native namespace. We deliberately
keep this path for two reasons:
1. `teran` is the single maintainer's own identity and is used consistently
   across the org's fleet (`mcp-regcloud`, `mcp-entertainment`), so the
   path is a stable, already-established namespace.
2. Renaming the module would require rewriting every import across all packages
   plus the `pkg.go.dev`/badge references, and would provide no additional
   runtime security: this server is a **local stdio** tool with **no
   `go get` consumers** and never publishes to any public module proxy, so a
   different namespace would not change what is fetched or trusted.

**Decision:** keep `github.com/teran/mcp-proxmox` as the module path. This is a
**recorded, deliberate decision** rather than a silent choice. If the project is
ever made a `go get`-able library, shared across org boundaries, or published to
a public module proxy, the module path MUST be revisited as a dedicated refactor
commit — see §10.

**Two distinct gateways, one shared shape.** PVE and PBS are separate products
with different object models, so they are modelled as two separate secondary
ports — `port.PVEGateway` and `port.PBSGateway` — each implemented by its own
adapter (`adapter/pve`, `adapter/pbs`). They share `port.TokenSource`,
`port.AppLogger`, and the error-mapping conventions, but never share an entity
set. This mirrors the optional-capability requirement: a backend is injected
into `application.App` only when configured, so a backend can be absent without
touching the other.

**Optional enablement via optional injection.** The `App` facade exposes
`WithPVE(gw port.PVEGateway)` / `WithPBS(gw port.PBSGateway)` options. When a
backend is not configured, the corresponding option is simply not applied and
the service is nil; `adapter/mcp` registers the tools of a backend only when its
service is non-nil. The `ping`/`status` tools report which backends are active.
This is the same optional-injection pattern as `mcp-regcloud`'s
`WithS3Object`/`WithK8sClient` and `mcp-entertainment`'s dynamic tool
registration.

**Why the API-token auth model is remote-amenable (unlike regcloud).**
`mcp-regcloud` locked itself to **local stdio only** because its authentication
is an access/refresh JWT pair stored in a **local config store** — the server
must re-read and rewrite `~/.config/regcloud/cloud.yml` on every token refresh,
which cannot work in a multi-tenant remote deployment. Proxmox's **API tokens**
remove both blockers:

- the token is **stateless and long-lived** — it is not refreshed by the server;
- the token is supplied **by the operator / the MCP client**, not owned and
  mutated by the server.

Consequently auth is hidden behind `port.TokenSource` (§4.2), which returns a
ready-to-send `Authorization` header value. The **local** implementation reads
the token from config and returns `PVEAPIToken=<token>`. A **future remote**
implementation can swap in an `OAuth2TokenSource` (or relay a per-request
`Authorization` header, as `mcp-paperless-ngx` does) that returns
`Bearer <token>` — the gateways only consume the header string, never the token
origin. Transport is likewise a selection point in the composition root: today
`adapter/mcp` wires the go-sdk **stdio** server; a future remote mode wires the
go-sdk **Streamable HTTP** server. Neither change touches `domain` or
`application`. **Tradeoff:** this deliberately trades the simplicity of a
single local credential store for the extra work of defining the
`TokenSource`/transport seam now — but that seam is the entire point, and it
is small.

**Per-call vs from-config credentials.** Proxmox API tokens are long-lived,
privilege-scoped credentials configured once per backend (like `mcp-regcloud`'s
Reg.Ru JWT, and unlike its per-call S3 `access_key_id`/`secret_key`). The MCP
server reads **one token per backend from config** and re-uses it on every call;
tokens are **never** per-call parameters and **never** logged. Per-call
credential injection is reserved for a future remote/OAuth2 token source, not
for the tools themselves.

**Read vs write discipline.** PVE/PBS reads (`GET`) are idempotent and retried;
mutations (`POST`/`PUT`/`DELETE`) are **never** retried (§7.5).

### 2.5 Tool-surface phasing and the mutation gate

The tool surface is delivered **phased** and **gated**:

- **Read-only (query) tools are always available** for an enabled backend.
- **All mutations are explicitly gated** by the env var **`ENABLE_MUTATIONS`**
  (`adapter/config.Config.EnableMutations`, **default `false`**), read by
  envconfig and wired by the composition root (`cmd/mcp-proxmox/main.go`) into
  `adapter/mcp.Deps.EnableMutations` (and the `enableMutations` argument to
  `RegisterTools` / `registerPVETools` / `registerPBSTools`). `adapter/mcp`
  registers a mutation tool **only** when the flag is true; read-only tools are
  unaffected.
- Implemented mutations (when enabled): PVE — `pve_vm_create`, `pve_vm_resize`,
  `pve_vm_migrate`, `pve_ha_add`, `pve_vm_backup`; PBS — `pbs_verify_start`,
  `pbs_gc_start`, `pbs_prune_start`, `pbs_sync_start`. Further mutations
  (start/stop/reboot/shutdown/delete, LXC, restore/forget/notes-set) remain
  planned.
- **Default:** mutations are off (read-only + system vertical slice) until
  `ENABLE_MUTATIONS=true` is set.

**L11 `always_present` nuance.** All registered tools are **static per process
run**: the tool surface is fixed at startup — the `ENABLE_MUTATIONS` gate is a
**startup-time config**, not a per-request/caller dependency. Consequently every
registered tool is `always_present=true` (there are no conditionally-present or
caller-dependent tools), and the per-call access log (§5.1c) need not repeat
that fact for each tool.

---

## 3. Domains and capabilities

The tool surface is grouped into three domains — **Proxmox VE**, **Proxmox
Backup Server**, and **system** — each with a distinct tool name prefix.

### 3.1 Proxmox VE (prefix `pve_`)

Operates a PVE cluster over the REST API (`/api2/json`).

- **Nodes & cluster:** list nodes, node status (uptime, CPU, memory, storage
  totals), cluster status (quorum/health), cluster resources (VMs/CTs/storage
  across all nodes), and the next free VMID.
- **QEMU VMs:** list VMs on a node, VM config, VM status, create, resize, start,
  stop, reboot, shutdown, migrate, delete; backup a VM (vzdump).
- **LXC containers:** list containers on a node, container config, container
  status, create, start, stop, reboot, shutdown, delete.
- **Storage:** list cluster storage, storage status on a node.
- **Network:** list network interfaces on a node.
- **High availability:** register a new HA resource.
- **Tasks:** list tasks on a node, task status by UPID, task log by UPID.
- **Version:** PVE version info.

Mutation tools (create/resize/migrate/ha/backup) are registered only when
`ENABLE_MUTATIONS=true` (SPEC.md §2.5).

### 3.2 Proxmox Backup Server (prefix `pbs_`)

Operates a PBS instance over the REST API (`/api2/json`).

- **Datastores:** list datastores, datastore status (usage, size, free).
- **Backups/snapshots:** list snapshots in a datastore, get a single snapshot,
  restore a snapshot, forget (remove from the catalog) a snapshot.
- **Notes:** get and set the notes of a backup snapshot.
- **Verify:** start a verify job on a datastore, poll verify-job status.
- **Prune:** start a prune job on a datastore, poll prune-job status.
- **Garbage collection:** start a GC job on a datastore.
- **Sync:** start a one-off sync job on a datastore.
- **Version:** PBS version info.

Mutation tools (verify/gc/prune/sync start) are registered only when
`ENABLE_MUTATIONS=true` (SPEC.md §2.5).

### 3.3 System (no prefix)

- `ping` — liveness check; reports which backends (`pve`, `pbs`) are enabled.
- `status` — server status: version, transport, enabled backends.

---

## 4. Run mode and authentication

### 4.1 Run mode — local stdio first, remote later

The server runs **locally** over the **stdio** transport (stdin/stdout),
launched by an MCP client as a subprocess. This is the initial and default mode.

A future **remote mode** (Streamable HTTP) is designed for but not implemented
yet: the composition root selects the transport and token source; `domain` and
`application` are unaware of the transport. See §2.4.

**Transport decision (stdio now, HTTP later).** The task is a local companion
for managing a homelab Proxmox cluster from an editor/CLI: short-lived, launched
as a subprocess by the MCP client, no public listener, credentials supplied via
local environment variables. **STDIO** is therefore the right transport today —
it requires no TLS termination, no session management, and matches the MCP
client's model for local servers. A remote mode (Streamable HTTP) is only worth
adding when the server must serve remote/untrusted clients; that is out of scope
for this milestone, so the transport is chosen by the composition root behind an
interface to allow it later. **TLS is never implemented inside the server** — if
an HTTP mode is added, TLS termination is the reverse proxy's job (§8 security).

**Auth / OAuth2 decision (no OAuth2 now).** The server only trusts credentials
(Proxmox API tokens) that the operator supplies locally via environment
variables; it serves no untrusted remote clients. There is **no OAuth2** in the
local stdio mode — delegated authorization would add complexity with no remote
audience. The seam for it is preserved: auth is behind `port.TokenSource`
(§4.2), so a future remote mode can swap in an `OAuth2TokenSource` /
header-relay that returns `Bearer <token>` without touching `domain`/`application`.

| Transport (now) | Token source |
|---|---|
| stdio (stdin/stdout) | `StaticTokenSource` — API tokens from environment variables (`PVE_TOKEN` / `PBS_TOKEN`, per backend) |

### 4.2 Token abstraction — `port.TokenSource`

The origin of the `Authorization` header is hidden behind `port.TokenSource`:

```go
type TokenSource interface {
    AuthorizationHeader(ctx context.Context) (string, error)
}
```

- **Local:** `adapter/token.StaticTokenSource` reads the configured token and
  returns `PVEAPIToken=<user@realm!tokenid=uuid>` verbatim. PVE and PBS each get
  their own instance bound to their own configured token.
- **Future remote:** an `OAuth2TokenSource` (or a per-request header relay, as
  `mcp-paperless-ngx` does) can return `Bearer <token>` and perform the OAuth2
  flow. The gateways consume only the returned header string, so nothing else
  changes.

### 4.3 Proxmox authentication

The Proxmox VE and PBS REST APIs each accept authentication via an **API token**
sent as an HTTP header:

```
Authorization: PVEAPIToken=user@realm!tokenid=uuid
```

- The token string has the form `user@realm!tokenid=secret` and is generated in
  the Proxmox web UI ("API Tokens", with privilege separation per user).
- Token-based requests do **not** require the ticket/CSRF dance (that is only
  for username/password ticket auth via `POST /access/ticket` and the
  `PVEAuthCookie` + `CSRFPreventionToken`). This project uses **API tokens
  only**, so no CSRF handling is needed.
- **TLS verification is always on.** There is no `insecure_skip_verify` switch;
  self-signed/custom CAs are supported only by adding a `ca_cert_path` per
  backend in the config (used to build the client's TLS `RootCAs`).

### 4.4 Configuration (`adapter/config`, environment variables via `envconfig`)

Configuration is read from **environment variables** using
[`kelseyhightower/envconfig`](https://github.com/kelseyhightower/envconfig) —
there is **no YAML/config file**. A backend is **enabled only if both its
`endpoint` and `token` are present**; if either is empty, the backend is
disabled and its tools are not registered.

| Variable | Purpose |
|---|---|
| `PVE_ENDPOINT` | PVE API endpoint (e.g. `https://pve.example.com:8006`); enables PVE when non-empty |
| `PVE_TOKEN` | PVE API token (`user@realm!tokenid=uuid`); enables PVE when non-empty |
| `PVE_CA_CERT_PATH` | optional custom CA (PEM) for the PVE endpoint; empty = system roots |
| `PBS_ENDPOINT` | PBS API endpoint (e.g. `https://pbs.example.com:8007`); enables PBS when non-empty |
| `PBS_TOKEN` | PBS API token (`user@pbs!tokenid:secret` — `:` separator, `=` normalized to `:`); enables PBS when non-empty |
| `PBS_CA_CERT_PATH` | optional custom CA (PEM) for the PBS endpoint; empty = system roots |
| `LOG_LEVEL` | log level `trace\|debug\|info\|warn\|error`; **when set, logging is enabled** (to `LOG_FILENAME`); when unset, logging is **disabled** (L2) |
| `LOG_FILENAME` | log file path used when `LOG_LEVEL` is set (default `/tmp/mcp-proxmox.log`, mode `0600`) (L3) |
| `LOG_FORMAT` | log format `text` (default) \| `json` (L4) |
| `ENABLE_MUTATIONS` | register mutation tools (default `false`). When `true`, mutation tools are registered for enabled backends; when `false`, only read-only query tools are exposed (§2.5) |
| `HTTP_TIMEOUT` | upstream API call timeout for both PVE and PBS (default `2m`; `Go` duration string such as `90s`, `3m`) (X3) |

These are the **only** environment variables the server reads. The tokens are
read once per backend from the environment and re-used on every call; they are
**never** logged and **never** per-call parameters (see §2.4). The loader lives
in `adapter/config/config.go` (`config.Load()` → `config.Config` with
`PVEEnabled()` / `PBSEnabled()` predicates).

### 4.5 Gateway configuration

Each gateway adapter exposes a config struct. Outbound HTTP is performed
exclusively via **`resty.dev/v3`** (SPEC G9/N23); the `HTTPClient` field is a
test-only seam whose transport/timeout the Resty client is derived from.

```go
type Config struct {
    Endpoint    string            // e.g. https://pve.example.com:8006
    TokenSource port.TokenSource  // returns the Authorization header
    HTTPClient  *http.Client      // TEST seam only: transport/timeout Resty derives from (default 30s)
    CACertPath  string            // optional custom CA (PEM); empty = system roots
    Logger      port.AppLogger
    Retries     int               // 2 (read-only idempotent requests)
    RetryBackoff time.Duration    // 200ms
}
```

### 4.6 Version and build

The binary embeds **build metadata** via ldflags (B2): `appName`, `appVersion`,
`appCommitHash`, `appTimestamp` (plus `version`). `.goreleaser.yml` injects them
as `-X main.appName=... -X main.appVersion={{ .Version }}
-X main.appCommitHash={{ .ShortCommit }} -X main.appTimestamp={{ .Date }}`.
A plain `go build` without ldflags uses defaults (`mcp-proxmox`,
`v0.0.0-dev`, `unknown`, `unknown`). The `-version` flag prints
`mcp-proxmox <version>` and exits 0.

The container image build is **isolated**: the `Dockerfile` does not compile —
it consumes a goreleaser artifact from `dist/mcp-proxmox`, producing a minimal
`FROM scratch` runtime with only the static binary and the CA bundle.

**Container image vs. Local deployment type (B3/N17).** The MCP-server skill's
B3/N17 says a container image is **only** for a **Remote/Hybrid** (HTTP) server,
never a Local stdio-only server. This server is **Local (stdio)** and its image
is **not** built or published by any CI/CD pipeline (no `container-image` job,
no registry push — R1/R3/R4 do not apply). The `Dockerfile` is kept purely as a
**local convenience packaging** for running the stdio binary in an isolated
runtime (`docker run` with env vars for endpoint/token), which is useful for
development/sandboxing and is how some clients prefer to launch a subprocess.
This is a **recorded, deliberate deviation from the letter of B3/N17**: the
image exists for operator convenience only, is never released, and does not turn
the server into a Remote/Hybrid one. If a remote HTTP mode is ever implemented
(§4.1), the image becomes a first-class release artifact and R1/R3/R4/N14 apply
in full.

**Observability N/A (Local).** The O-series criteria (O01–O04 — the internal
observability endpoint on `:8081` exposing **Prometheus** metrics, **pprof**,
and liveness/readiness **probes**) are **NOT applicable**: this is a **Local
(stdio-only)** server with **no HTTP transport and no network listener**, so
there is no `:8081`-style internal endpoint on which to expose
Prometheus/pprof/probes. The O-series rationale (O01/O04/N32) applies to
**Remote/Hybrid** servers only; for a Local stdio server these criteria are N/A.
Process observability is instead covered by the stdio file log sink
(§5.1b/§5.1c) and upstream request logging.

### 4.7 Data and state model (X1–X5)

**Stateless (X1).** This server is **stateless**: there is no persistent state
held outside a single session. Each tool call maps to one self-contained
upstream HTTP request against the Proxmox VE / PBS REST APIs, and the upstream
cluster holds all state. The only per-process data is:

- the configuration and API tokens, read **once** at startup from environment
  variables (`adapter/config`) and held in memory for the process lifetime —
  never persisted, never refreshed, never mutated by the server;
- the `session_id` / per-request `request_id`, used **only** for log
  correlation (§5.2), not as business state.

There is no database, no on-disk store, no cache, and no cross-session data.
Consequently the data/state criteria that apply to stateful servers do not
apply here:

- **Write idempotency (X2).** Mutating tools are gated behind `ENABLE_MUTATIONS`
  (§2.5) and map to single upstream calls. Idempotency is a property of the
  **upstream** Proxmox API (e.g. `pve_nextid` search-before-create for VM/CT
  creation, HA-add by resource key) — the server issues one request and does not
  create duplicates itself. Read-only tools set `idempotentHint=true` (X2/N25).
- **Upstream connection & limits (X3).** Upstream calls use an explicit timeout
  (default 2m, configurable via `HTTP_TIMEOUT`) and context; on failure
  (including `429`) the error is returned
  to the model **and logged** — there is **no silent retry** except a bounded
  retry with backoff for **idempotent read-only** requests, and the underlying
  error is always logged (§7.5).
- **Persistence (X4).** Not applicable — the server is stateless.
- **Concurrency (X5).** Not applicable — there are no file/DB writes to
  serialize; the server issues independent HTTP requests per call.

### 4.8 No user-supplied local filesystem access (S04)

This is a **Local stdio server** that performs **no user-supplied local
filesystem access**. It never opens, reads, writes, lists, or walks any local
path named by a tool argument — no tool argument references a path the server
accesses on its own host. The **only** file I/O the process ever performs is:

- reading `ca_cert_path` (**per backend**, `PVE_CA_CERT_PATH` /
  `PBS_CA_CERT_PATH`) to build the TLS `RootCAs` for the outbound HTTPS client;
- writing `LOG_FILENAME` (default `/tmp/mcp-proxmox.log`) when `LOG_LEVEL` is
  set.

Because no tool argument names a local path the server touches,
`ALLOW_DIRS` / `ALLOW_SYMLINKS` (and any client-side filesystem sandboxing) are
**N/A** for this server — there is nothing to sandbox on the server side.

### 4.9 Security — domain security audit (S14)

Produced by the `security` agent per `security-audit.md`; layered on the generic
S01–S13 baseline this SPEC applies throughout (TLS always on — S01; secret
redaction in output/logs — S02/L05; no user-supplied filesystem access — S04;
read-only-by-default / least privilege — S10; injection — S08/S11; HITL — S12;
supply chain — S13). This subsection records the **domain-specific** audit for a
Proxmox control-plane tool. Its presence satisfies **S14**; its absence is the
**N38** violation.

#### Domain security audit (S14)

**1. Audit inputs**

- **Purpose:** local stdio MCP server exposing the Proxmox VE and PBS REST APIs
  as callable tools so an LLM agent / IDE can inspect **and** (when
  `ENABLE_MUTATIONS=true`) operate a Proxmox virtualization cluster and its
  backup server. Primary job: **control-plane proxy / state-modifying operator**
  over two external management APIs.
- **Callers:** the local MCP client / model host (editor, CLI, agent runtime)
  that launches the server as a subprocess over **stdio**. In local mode these
  callers hold the full privilege of the configured API tokens (they can delete
  VMs, forget snapshots, start backups). No unauthenticated / network callers.
- **External systems:** **PVE** (`https://pve…:8006/api2/json`) and **PBS**
  (`https://pbs…:8007/api2/json`) over HTTPS, authenticated with long-lived,
  privilege-scoped **API tokens**. Upstream responses are treated as
  **untrusted** (S07/S9).
- **Data:** API tokens (credentials); VM / LXC / HA / storage / network /
  cluster configs; backup/snapshot metadata and notes; verify/prune/gc task
  status and logs; PVE/PBS version and node/cluster status.
- **Transport & deployment:** **Local (stdio)**, no network listener, no TLS
  inside the server (in any future remote mode TLS termination is the reverse
  proxy's job, §8); config and tokens supplied via environment variables.
- **Language / runtime:** Go 1.27.3; supply-chain gates `govulncheck`/gosec
  (S13) apply.

**2. Trust boundaries & threat model**

| Zone | What crosses the boundary | From → To |
|---|---|---|
| Model host / MCP client | user/model-supplied **tool arguments** (`vmid`, `node`, `snapshot`, `upid`, `store`, …) | client → server (stdio) |
| Server process (`adapter/mcp` → `application` → `adapter/pve\|pbs`) | validated args, `Authorization: PVEAPIToken=…` header | server → upstream |
| Upstream **PVE / PBS** | upstream JSON responses (untrusted, S07), task UPIDs | upstream → server |
| Local filesystem | `ca_cert_path` (read-only, per backend), `LOG_FILENAME` (append, `0600`) | server → fs |
| Credentials store (env) | `PVE_TOKEN` / `PBS_TOKEN` read once at startup | env → server memory |

Domain threat model (STRIDE applied to *this* server's boundaries):

- **Compromised / malicious model host** (highest realistic risk — the caller is
  the component most likely to be driven by an adversarial prompt):
  - *Spoofing / Tampering / DoS of the virtualization estate:* issue tool calls
    to delete or alter the **wrong** VM/CT/snapshot, restore a bad snapshot,
    register an HA resource, or launch a destructive backup/prune/GC —
    a **control-plane** version of prompt-injection / tool-abuse (relates to X02
    and S12). Impact: **data loss, environment outage, pruning of the last good
    backup copy**.
  - *Information disclosure:* request configs of any VM/CT/storage/snapshot the
    token can read (S02/S10 limiting).
  - *Argument injection:* forge `vmid`/`node`/`snapshot`/`upid`/`store` values
    interpolated into URL path segments to hit unintended endpoints or trigger
    path-injection (S08/S11 hardening; `url.PathEscape`).
- **Malicious upstream PVE/PBS response:**
  - *Tampering / info-leak via output:* a hostile upstream returns crafted JSON
    that reaches the model through tool output; mitigated by strict
    `outputSchema` validation + `sanitizeOutput` (S9/S2) and treating upstream as
    untrusted (S07).
  - *Task-status spoofing:* a forged UPID/status could mislead the model into
    believing a prune/backup completed (integrity of operations).
- **Hostile local filesystem:**
  - *Token leak via log:* `LOG_FILENAME` must never capture token values
    (L05/S2); the token is never logged, only the header *name* at trace.
  - *`ca_cert_path` tampering:* a compromised `ca_cert_path` file could inject a
    CA that trusts a MITM — mitigated by S01 "TLS verification always on" and
    treating the path as operator-controlled config, never a tool argument (S04).
- **Local stdio protocol attacker:** the stdio channel is the trusted client
  channel; any process able to write to the server's stdin holds the same power
  as the model host — reinforcing that the server is a **local, single-user
  trust** boundary with **no network exposure**.

**3. Data sensitivity classification → C-I-A**

| Data class | Sensitivity | Confidentiality | Integrity | Availability |
|---|---|---|---|---|
| API tokens (`PVE_TOKEN`/`PBS_TOKEN`, `Authorization` header) | **Secret** | redact (S02/L05); never logged, never in tool output; read once, in-memory (X1) | — | must remain available for authorized reads |
| Infra / control-plane config of VMs, CTs, HA, storage, network, cluster | **High** | least-privilege token (S10) | **authorized-op-only writes; idempotency (X02) on non-idempotent mutations; no wrong-resource ops** | operations must not be disrupted by destructive ops |
| Backup/snapshot metadata, notes, verify/prune/GC task status | **High** (backups are the last line of defense) | least privilege (S10) | **no double-execution of prune/GC/verify/backup (X02)** | destructive `prune`/`forget`/`gc` must be gated (S12) |
| PVE/PBS version, node/cluster status, datastore usage | **Public** | treat as untrusted (S07); still schema-validated | — | — |
| `ca_cert_path` (PEM), `LOG_FILENAME` | **Secret** / **config** | cert content never logged (L05) | TLS integrity (S01) | — |

Derived stances: tokens and `ca_cert_path` must **never be persisted or logged**
(the server is stateless, X1, so **no encryption-at-rest** applies); destructive/
unsafe ops (`*_delete`, restore/rollback, prune, forget, gc, backup) require
`destructiveHint` + HITL (S12) and the `ENABLE_MUTATIONS` gate (default off, §2.5).

**4. Domain-specific attack vectors (beyond generic S01–S13)**

| Vector | Threat | Impact | Domain-specific requirement |
|---|---|---|---|
| **Per-operation authz on destructive ops** (`pve_vm_delete`, `pve_lxc_delete`, snapshot delete, restore/rollback, HA) | Compromised/malicious model host issues a delete/alter call against the **wrong** resource — spoofed `vmid`/`node`/`snapshot` | Irreversible VM/CT deletion, snapshot loss, bad restore, HA misconfiguration; control-plane **availability/integrity loss** | **MUST (new):** destructive tools carry `destructiveHint=true` and require HITL (S12); mutations registered only under `ENABLE_MUTATIONS` (§2.5); per-tool `instructions` state the resource identity and require confirming the target before a destructive call |
| **Command/path-injection via `vmid`/`node`/`snapshot`/`upid`/`store` interpolated into URL path segments** | Attacker-controlled value embedded in the URL path reaches a different or malformed upstream endpoint | SSRF-adjacent path confusion, hitting unintended PVE/PBS endpoints, malformed requests (S11 hardening) | **MUST (hardens S08/S11):** every path-segment value strictly validated against the upstream schema and **URL-path-escaped with `url.PathEscape`** before interpolation; no raw concatenation; reject non-conforming values at `inputSchema` (S8) |
| **Non-idempotent mutations double-execution** (`pve_vm_create`, `pve_vm_backup`, `pbs_verify_start`, `pbs_gc_start`, `pbs_prune_start`, `pbs_sync_start`, `pbs_task_stop`) | A client/transport retry or a model retry re-issues a mutation → duplicate create, parallel backup, double prune/GC | Duplicate resources, wasted I/O, corrupted prune/verify window | **MUST (cross-ref X02, §4.7):** mutations are **never retried** (§7.5); rely on upstream idempotency where present (`pve_nextid` search-before-create, HA key); set `idempotentHint` only when the upstream op is truly idempotent |
| **Token confidentiality across output/log** | API token leaks through tool output or log lines (S02/L05) | Full credential compromise of the PVE/PBS estate | **MUST (hardens S02/L05):** `sanitizeOutput`/`sanitizeArgs` redact token-like keys; tokens never enter domain models, tool output, or log lines — only the header *name* at trace; `gitleaks` scans history (S05) |
| **TLS / CA handling** | Disabling verification or trusting a forged CA (S01) | MITM of the control-plane API, token theft in transit | **MUST (hardens S01):** TLS verification **always on**, no `insecure_skip_verify`; custom CA only via operator `ca_cert_path` building the client `RootCAs`; never a tool argument (S04) |
| **Upstream response integrity** | Forged/tampered upstream JSON reaching the model (S07/S9) | Model acts on false VM/snapshot/task state, e.g. believing a prune finished | **MUST (hardens S7/S9):** upstream treated as untrusted; every output validated against per-tool `outputSchema` and passed through `sanitizeOutput`; failures returned as `IsError` |

**5. Filled auditor checklist (security-audit.md §6)** — all **YES**:

- [x] Trust zones and boundaries enumerated and documented (audit §2).
- [x] A concrete threat model for this domain recorded, not a generic list
      (audit §2: malicious model host, hostile upstream, hostile filesystem).
- [x] Every data class classified with C-I-A requirements (audit §3).
- [x] Domain-specific attack vectors identified beyond generic S01–S13
      (audit §4: destructive-op authz, path-injection, non-idempotent
      double-execution, token leak, TLS/CA, upstream integrity).
- [x] Each vector mapped to a concrete requirement (hardened S-code or new MUST)
      (audit §4).
- [x] Sensitive data redaction (S02) and no-secrets-in-logs (L05) confirmed for
      this domain's data (tokens/CA cert).
- [x] Destructive/irreversible operations flagged `destructiveHint` with HITL
      (S12) where the domain demands it (delete/restore/prune/forget/gc/backup).
- [x] Mutating tools carry a real data-level idempotency mechanism (X02) where
      double-execution is harmful (never-retry + upstream idempotency, §7.5).
- [x] Filesystem access scoped per S04 and extended for this domain's file
      classes (`ca_cert_path` read-only, `LOG_FILENAME` `0600` append; no tool
      argument names a path).
- [x] The audit is produced by the `security` agent and embedded verbatim (S14).

---

## 5. Logging

All logging goes through **logrus** (`github.com/sirupsen/logrus`).

### 5.1 Components (`adapter/logging`)

- `NewLogger(level, format)` → `*logrus.Logger` (format `text` default or `json`).
  The text formatter uses `FullTimestamp: true` so lines carry absolute
  wall-clock time instead of the relative `INFO[0002]` elapsed duration —
  convenient for correlating logs across sessions/requests.
- `NewAppLogger(l)` → `port.AppLogger` (`Debugf/Infof/Warnf/Errorf`). This is the
  **only** logging interface the core sees — the core never imports logrus.
- `NewSlogLogger(l)` → `*slog.Logger`. go-sdk accepts only a `*slog.Logger`, so a
  `slog.Handler` bridges the SDK's internal logs into the same logrus journal.
- `WithSession(ctx, l)` → `*logrus.Entry` carrying `session_id` and per-request
  `request_id` from `ctx` (via `port.SessionIDFromContext` /
  `port.RequestIDFromContext`), so request-scoped lines are tagged consistently
  without repeating the fields at every call site.

### 5.1b Log destination (stdio-friendly file sink)

The server runs over **stdio**, so MCP clients launch it as a subprocess and
typically do **not** capture its stderr. To keep the journal inspectable:

- **Disabled by default (L2).** When `LOG_LEVEL` is **unset**, the logger
  discards all output (`io.Discard`) — no logs are emitted anywhere, so nothing
  pollutes the stdio protocol channel.
- When `LOG_LEVEL` is set (any non-empty value), logs are **enabled** and written
  to **`LOG_FILENAME`** (default `/tmp/mcp-proxmox.log`,
  `O_APPEND|O_CREATE|O_WRONLY`, mode `0600`) at that level, in `LOG_FORMAT`
  (text or JSON). This is the file sink that makes stdio-mode logs easy to tail:
  `tail -f /tmp/mcp-proxmox.log`.
- **Banner (B5/L6).** When enabled, the **very first** log line is the startup
  banner: `Starting {appName}/{appVersion} (commit: {appCommit}; built at
  {appTimestamp}) ...`, using the build metadata from §4.6. No banner is emitted
  when logging is disabled.
- A single **session ID** is created per MCP session (one process run in stdio
  mode) and shared between the stdio transport connection (so the SDK logs a
  non-empty `session_id` on connect/disconnect) and the request context (so tool
  and application log lines carry the same `session_id`). Per-request
  `request_id` is added by `WithSession`.

### 5.1c Per-request tool-call logging (L8)

When logging is enabled (always in HTTP mode; in stdio only when `LOG_LEVEL` is
set — L2), the server middleware emits an **access log at `info`** for every
incoming MCP call — **unconditionally, no `debug`/`trace` gate**. The info line
carries `tool` (name, for `tools/call`), `source` (`"STDIO"`), `duration_ms`,
`outcome` (`ok`/`error`), and the **redacted** `args` (`sanitizeArgs` strips
sensitive keys per S02, so no secret can reach the journal). Byte-size metrics
(`in_bytes`/`out_bytes`) are confined to a separate **trace**-gated line
(emitted only at `LOG_LEVEL=trace`). Every line carries `session_id`/`request_id`
from `ctx`. For outbound upstream requests, each gateway **forwards the
`request_id` as an `X-Request-ID` header** and logs method/path/status/duration/
in_bytes/out_bytes with the same `request_id` (SPEC §5.5).

### 5.2 Context-aware logging (`port.CtxLogger`)

Application services log via `port.CtxLogger` (built in `application.New` via
`port.NewCtxLogger(log)`), which type-asserts the underlying logger to the
optional `port.RequestAwareLogger` and tags lifecycle lines with the MCP
`session_id` / `request_id` from `ctx`; it falls back to the plain `AppLogger`
for test dummies. Tool handlers use a ctx-aware `toolLogger`
(`adapter/mcp/tool_logger.go`) so error lines carry the same IDs.

### 5.3 Logging conventions

- Use cases log via `Infof` (lifecycle events) and `Warnf` (anomalies).
- Errors are propagated by returning `error`; `adapter/mcp` tools log them via
  `Errorf` with the tool name as context (`"pve_vm_start: %v"`).
- `domain/model` never logs (pure types).

### 5.4 Secrets and sensitive data are never logged

- **API tokens** (the `PVEAPIToken`/`Authorization` header) are never logged —
  only the header *name* may appear at trace level, never its value.
- **`ca_cert_path`** content is never logged.
- Request/response bodies are never logged at a level that would include token
  material; trace-level request logging logs method + URL path only.

**Output-path redaction & sanitization (S2/S9).** Every tool result passes
through `wrapOutput` → `sanitizeOutput` (`adapter/mcp/helpers.go`) before
reaching the MCP client:

- **S2 (no secret leakage):** any map key whose name looks sensitive (`token`,
  `secret`, `api_key`, `auth`, `password`, `credential`, `fingerprint`, …) is
  replaced with `[redacted]` in tool output. The typed domain models returned by
  tools carry no secret fields — API tokens never enter the model or the output
  path — so this is a defensive guarantee for any map-based output.
- **S9 (output sanitization & validation):** every tool result is validated
  against a per-tool `outputSchema` derived from the tool's typed output type.
  `roTool`, `mutTool` and `sysTool` (`adapter/mcp/helpers.go`) set `Tool.OutputSchema`
  via `outputSchema[Out]()` — generated with the same `jsonschema.ForType`
  reflection path the go-sdk uses for input schemas, and shaped to match the
  wrapping `wrapOutput` produces (slices/arrays → `{"items":...}`, scalars →
  `{"value":...}`, structs/maps pass through). Tools whose handler returns
  `Out == any` (the erased shape) advertise a permissive schema that accepts any
  JSON, since the runtime shape is unknown. Before a result reaches the client it
  is wrapped (`wrapOutput`), passed through `sanitizeOutput` (redacting sensitive
  map keys per S2 and stripping ANSI/control bytes per S9), and validated against
  the derived `outputSchema`; a non-conforming result is logged via the
  `toolLogger` and returned as an error (the MCP result is flagged `IsError`)
  instead of emitting non-conforming output. The guarantee is therefore
  wrap + redact + strip + schema validation.

Input arguments are likewise redacted in log lines (`sanitizeArgs`, §5.1c) using
the same sensitive-key policy.

---

## 6. Tool registry (complete)

With both backends enabled and `ENABLE_MUTATIONS=true` (SPEC §2.5), **74 tools**
are registered. Naming uses the domain prefixes `pve_` and `pbs_`; system tools
(`ping`, `status`) have no prefix. `query` = read (idempotent, retried);
`mutation` = write (never retried). Read-only query tools are always registered
for an enabled backend; mutation tools are registered **only** when
`ENABLE_MUTATIONS=true` — with the default (mutations disabled) **35 tools** are
registered (system 2, PVE 21, PBS 12).

> This list is the implemented, defensible surface — it is derived directly from
> the tool-registration code in `adapter/mcp`. Every endpoint below exists in the
> real Proxmox VE / PBS REST APIs (`/api2/json`).

### 6.0 Tool metadata (Annotations & Instructions) — M4/N11

Every registered tool carries MCP **Annotations (hints)** and per-tool
**Instructions**, set via the go-sdk `Tool` fields:

- `Title` — a short human-readable display title (e.g. "List PVE nodes").
- `readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`.
- `instructions` — natural-language guidance (inputs, ordering, side effects,
  precautions) for the model, encoded into the SDK `Description` (the field the
  go-sdk treats as the model hint).

The hints follow a fixed mapping (`adapter/mcp/helpers.go` `roTool`/`mutTool`):

| Tool family | readOnly | destructive | idempotent | openWorld |
|---|---|---|---|---|
| read / query | `true` | `false` | `true` | `false` |
| write / update | `false` | `false` | `true` | `false` |
| delete | `false` | `true` | `false` | `false` |

Every tool also carries its **own per-tool `Title` and `instructions`**, passed
at registration (`roTool(s, name, title, desc, instr, ...)` — see
`adapter/mcp/pve_tools.go`). `instructions` is natural-language guidance for the
model (expected inputs, ordering, side effects, precautions) and is encoded into
the SDK `Description` so it is surfaced to the client as the model hint.

**Example — `pve_node_list` (§6.2):**

```json
{
  "name": "pve_node_list",
  "title": "List PVE nodes",
  "description": "List the Proxmox VE cluster nodes.\n\nInstructions: Returns all cluster nodes and their status. No arguments; read-only and idempotent — safe to call repeatedly.",
  "annotations": { "readOnlyHint": true, "idempotentHint": true, "openWorldHint": false, "destructiveHint": false }
}
```

The registry tables in §6.1–6.10 list each tool's name / domain / type / endpoint
for brevity; every row is registered with the title + instructions + the
annotations from the mapping table above (M4/N9).

The annotation mapping is realized on the full surface: query tools are
`readOnly=true, destructive=false, idempotent=true, openWorld=false`; mutation
tools (registered behind the gate, §2.5) follow the write / delete rows above,
with destructive mutations (delete / restore / rollback / GC / prune) carrying
`destructive=true`.

### 6.1 System (2)

| # | Tool | Domain | Type | Description |
|---|---|---|---|---|
| 1 | `ping` | system | query | liveness; reports enabled backends (`pve`, `pbs`) |
| 2 | `status` | system | query | server status: version, transport, enabled backends |

### 6.2 Proxmox VE — nodes, cluster & HA (7)

| # | Tool | Type | Endpoint |
|---|---|---|---|
| 3 | `pve_node_list` | query | `GET /nodes` |
| 4 | `pve_node_status` | query | `GET /nodes/{node}/status` |
| 5 | `pve_cluster_status` | query | `GET /cluster/status` |
| 6 | `pve_cluster_resources` | query | `GET /cluster/resources` |
| 7 | `pve_nextid` | query | `GET /cluster/nextid` |
| 8 | `pve_backup_job_list` | query | `GET /cluster/backup` |
| 9 | `pve_ha_add` | mutation | `POST /cluster/ha/resources` |

### 6.3 Proxmox VE — backup jobs (3)

| # | Tool | Type | Endpoint |
|---|---|---|---|
| 10 | `pve_backup_job_create` | mutation | `POST /cluster/backup` |
| 11 | `pve_backup_job_update` | mutation | `PUT /cluster/backup/{id}` |
| 12 | `pve_backup_job_delete` | mutation | `DELETE /cluster/backup/{id}` |

### 6.4 Proxmox VE — QEMU VMs (22)

| # | Tool | Type | Endpoint |
|---|---|---|---|
| 13 | `pve_vm_list` | query | `GET /nodes/{node}/qemu` |
| 14 | `pve_vm_get` | query | `GET /nodes/{node}/qemu/{vmid}/config` |
| 15 | `pve_vm_status` | query | `GET /nodes/{node}/qemu/{vmid}/status/current` |
| 16 | `pve_vm_snapshot_list` | query | `GET /nodes/{node}/qemu/{vmid}/snapshot` |
| 17 | `pve_vm_create` | mutation | `POST /nodes/{node}/qemu` |
| 18 | `pve_vm_resize` | mutation | `PUT /nodes/{node}/qemu/{vmid}/resize` |
| 19 | `pve_vm_migrate` | mutation | `POST /nodes/{node}/qemu/{vmid}/migrate` |
| 20 | `pve_vm_backup` | mutation | `POST /nodes/{node}/vzdump` |
| 21 | `pve_vm_start` | mutation | `POST /nodes/{node}/qemu/{vmid}/status/start` |
| 22 | `pve_vm_stop` | mutation | `POST /nodes/{node}/qemu/{vmid}/status/stop` |
| 23 | `pve_vm_shutdown` | mutation | `POST /nodes/{node}/qemu/{vmid}/status/shutdown` |
| 24 | `pve_vm_reboot` | mutation | `POST /nodes/{node}/qemu/{vmid}/status/reboot` |
| 25 | `pve_vm_reset` | mutation | `POST /nodes/{node}/qemu/{vmid}/status/reset` |
| 26 | `pve_vm_suspend` | mutation | `POST /nodes/{node}/qemu/{vmid}/status/suspend` |
| 27 | `pve_vm_resume` | mutation | `POST /nodes/{node}/qemu/{vmid}/status/resume` |
| 28 | `pve_vm_delete` | mutation | `DELETE /nodes/{node}/qemu/{vmid}` |
| 29 | `pve_vm_clone` | mutation | `POST /nodes/{node}/qemu/{vmid}/clone` |
| 30 | `pve_vm_snapshot_create` | mutation | `POST /nodes/{node}/qemu/{vmid}/snapshot` |
| 31 | `pve_vm_snapshot_delete` | mutation | `DELETE /nodes/{node}/qemu/{vmid}/snapshot/{snapname}` |
| 32 | `pve_vm_snapshot_rollback` | mutation | `POST /nodes/{node}/qemu/{vmid}/snapshot/{snapname}/rollback` |
| 33 | `pve_vm_restore` | mutation | `POST /nodes/{node}/qemu` (qmrestore, `archive` param) |
| 34 | `pve_vm_set` | mutation | `PUT /nodes/{node}/qemu/{vmid}/config` |

### 6.5 Proxmox VE — LXC containers (14)

| # | Tool | Type | Endpoint |
|---|---|---|---|
| 35 | `pve_lxc_list` | query | `GET /nodes/{node}/lxc` |
| 36 | `pve_lxc_get` | query | `GET /nodes/{node}/lxc/{vmid}/config` |
| 37 | `pve_lxc_status` | query | `GET /nodes/{node}/lxc/{vmid}/status/current` |
| 38 | `pve_lxc_snapshot_list` | query | `GET /nodes/{node}/lxc/{vmid}/snapshot` |
| 39 | `pve_lxc_start` | mutation | `POST /nodes/{node}/lxc/{vmid}/status/start` |
| 40 | `pve_lxc_stop` | mutation | `POST /nodes/{node}/lxc/{vmid}/status/stop` |
| 41 | `pve_lxc_shutdown` | mutation | `POST /nodes/{node}/lxc/{vmid}/status/shutdown` |
| 42 | `pve_lxc_reboot` | mutation | `POST /nodes/{node}/lxc/{vmid}/status/reboot` |
| 43 | `pve_lxc_delete` | mutation | `DELETE /nodes/{node}/lxc/{vmid}` |
| 44 | `pve_lxc_clone` | mutation | `POST /nodes/{node}/lxc/{vmid}/clone` |
| 45 | `pve_lxc_snapshot_create` | mutation | `POST /nodes/{node}/lxc/{vmid}/snapshot` |
| 46 | `pve_lxc_snapshot_delete` | mutation | `DELETE /nodes/{node}/lxc/{vmid}/snapshot/{snapname}` |
| 47 | `pve_lxc_snapshot_rollback` | mutation | `POST /nodes/{node}/lxc/{vmid}/snapshot/{snapname}/rollback` |
| 48 | `pve_lxc_set` | mutation | `PUT /nodes/{node}/lxc/{vmid}/config` |

### 6.6 Proxmox VE — storage & network (3)

| # | Tool | Type | Endpoint |
|---|---|---|---|
| 49 | `pve_storage_list` | query | `GET /storage` |
| 50 | `pve_storage_get` | query | `GET /nodes/{node}/storage/{storage}/status` |
| 51 | `pve_network_list` | query | `GET /nodes/{node}/network` |

### 6.7 Proxmox VE — tasks & version (4)

| # | Tool | Type | Endpoint |
|---|---|---|---|
| 52 | `pve_task_list` | query | `GET /nodes/{node}/tasks` |
| 53 | `pve_task_status` | query | `GET /nodes/{node}/tasks/{upid}/status` |
| 54 | `pve_task_log` | query | `GET /nodes/{node}/tasks/{upid}/log` |
| 55 | `pve_version` | query | `GET /version` |

### 6.8 PBS — datastores (3)

| # | Tool | Type | Endpoint |
|---|---|---|---|
| 56 | `pbs_datastore_list` | query | `GET /admin/datastore` |
| 57 | `pbs_datastore_status` | query | `GET /admin/datastore/{store}/status` |
| 58 | `pbs_datastore_config_get` | query | `GET /admin/datastore/{store}/config` |

### 6.9 PBS — backups/snapshots & files (6)

| # | Tool | Type | Endpoint |
|---|---|---|---|
| 59 | `pbs_backup_list` | query | `GET /admin/datastore/{store}/snapshot` |
| 60 | `pbs_backup_get` | query | `GET /admin/datastore/{store}/snapshot/{backup-id}` (group) |
| 61 | `pbs_backup_notes_get` | query | `GET /admin/datastore/{store}/backup/{backup-id}/{backup-type}/notes` |
| 62 | `pbs_backup_files_list` | query | `GET /admin/datastore/{store}/backup/{backup-type}/{backup-id}/{snapshot}` |
| 63 | `pbs_backup_restore_file` | mutation | `POST /admin/datastore/{store}/backup/{backup-type}/{backup-id}/{snapshot}/restore` (single file) |
| 64 | `pbs_backup_vm_restore` | mutation | `POST /admin/datastore/{store}/backup/{backup-type}/{backup-id}/{snapshot}/restore` (VM) |

### 6.10 PBS — verify, GC, prune, sync, tasks & version (10)

| # | Tool | Type | Endpoint |
|---|---|---|---|
| 65 | `pbs_verify_status` | query | `GET /nodes/localhost/tasks/{upid}/status` |
| 66 | `pbs_prune_status` | query | `GET /nodes/localhost/tasks/{upid}/status` |
| 67 | `pbs_task_status` | query | `GET /nodes/localhost/tasks/{upid}/status` |
| 68 | `pbs_task_log` | query | `GET /nodes/localhost/tasks/{upid}/log` |
| 69 | `pbs_version` | query | `GET /version` |
| 70 | `pbs_verify_start` | mutation | `POST /admin/datastore/{store}/verify` |
| 71 | `pbs_gc_start` | mutation | `POST /admin/datastore/{store}/gc` |
| 72 | `pbs_prune_start` | mutation | `POST /admin/datastore/{store}/prune` |
| 73 | `pbs_sync_start` | mutation | `POST /admin/datastore/{store}/sync` |
| 74 | `pbs_task_stop` | mutation | `DELETE /nodes/localhost/tasks/{upid}` |

**Total: 74 tools** with `ENABLE_MUTATIONS=true` (system 2, PVE 53, PBS 19).
With `ENABLE_MUTATIONS` unset (default `false`), only the read-only surface is
registered: **35 tools** (system 2, PVE 21, PBS 12).

> **Mutation gate:** every `mutation`-typed row above is registered only when
> `ENABLE_MUTATIONS=true` (§2.5); when unset, only the `query` rows are exposed.
> Destructive mutations (delete / restore / rollback / GC / prune) additionally
> carry `destructive=true` and are **never retried**.

> **PBS restore semantics:** `pbs_backup_restore_file` restores a single file out
> of a backup snapshot to a target path on the PBS filesystem; `pbs_backup_vm_restore`
> restores a full VM backup snapshot into a PVE datastore and returns the task
> UPID. Both are mutations and are **never retried**.

> **Optional enablement recap:** all `pve_*` tools are registered only when the
> PVE backend is configured; all `pbs_*` tools only when PBS is configured.
> `ping`/`status` are always registered.

---

## 7. Error handling and retries

### 7.1 Sentinel errors (`domain/port/errors.go`)

```go
var (
    ErrUnauthorized = errors.New("proxmox: unauthorized") // HTTP 401
    ErrForbidden    = errors.New("proxmox: forbidden")    // HTTP 403
    ErrNotFound     = errors.New("proxmox: not found")    // HTTP 404
)
```

### 7.2 Typed adapter errors (`adapter/pve/errors.go`, `adapter/pbs/errors.go`)

- `UpstreamError{Op, Status, Err}` — transport / HTTP 5xx / decode / marshal /
  request errors. **The only errors that are retried.**
- `APIError{Messages}` — semantic errors from the Proxmox JSON body
  (`{"errors": {...}}` or the `data`/`message` error envelope).
- `EmptyDataError` — the API returned an empty/`null` `data` for a getter.

Getters do **not** return a silently-zero struct when the entity is absent; a
missing VM/storage/datastore surfaces as an explicit not-found error.

### 7.3 HTTP → error mapping (`Gateway.do`)

| Condition | Result |
|---|---|
| 401 | `port.ErrUnauthorized` |
| 403 | `port.ErrForbidden` |
| 404 | `port.ErrNotFound` |
| 5xx | `*UpstreamError{Op:"http", Status}` |
| API `errors`/error envelope present | `*APIError` |
| empty/`null` `data` for a getter | `*EmptyDataError` |
| transport/decode/marshal | `*UpstreamError` |

### 7.4 Token-source failure

If the token source cannot produce a header (missing/empty configured token on
an enabled backend), the gateway returns a `port.ErrUnauthorized`-style
misconfiguration error before any request is sent.

### 7.5 Retries

Retries (`Gateway.withRetry`) apply **only to idempotent reads** (queries, i.e.
`GET`) and **never to mutations** (to avoid repeating a write). Policy: `N`
attempts (`Config.Retries = 2`) with linear backoff (`RetryBackoff * attempt`,
default 200 ms); only `*UpstreamError` (5xx and transport) is retried — sentinels
and `*APIError` abort immediately. The loop honors `ctx.Done()`.

### 7.6 Error → response mapping (A1)

Tool handlers (`adapter/mcp/helpers.go`) return typed errors to the go-sdk. The
go-sdk distinguishes two cases:

- **Tool-execution errors** (all of this server's typed errors below) are
  embedded in the MCP `CallToolResult` with `IsError=true` and the message in
  its content — per the MCP spec, tool failures are a **successful JSON-RPC
  response carrying an error result**, not a JSON-RPC error. This is why every
  tool's failure is flagged `IsError` in the adapter/mcp tests.
- **Protocol-level errors** — invalid input against `inputSchema`
  (`additionalProperties:false`) and `*jsonrpc.Error` returns — map to JSON-RPC
  error codes at the transport layer.

The typed-error → observable-outcome mapping:

| Error type | MCP / JSON-RPC outcome | Meaning |
|---|---|---|
| `port.ErrUnauthorized` | `IsError=true` (HTTP 401) | request not authorized (bad/missing token) |
| `port.ErrForbidden` | `IsError=true` (HTTP 403) | request forbidden |
| `port.ErrNotFound` | `IsError=true` (HTTP 404) | requested resource not found |
| `*EmptyDataError` | `IsError=true` | entity absent / empty `data` on a getter |
| `*APIError` (semantic) | `IsError=true` | upstream returned a semantic error payload |
| `*UpstreamError` (5xx/transport) | `IsError=true` | upstream failure (5xx / network / decode / marshal) |
| token-source misconfiguration | `IsError=true` | enabled backend has no usable token |
| input validation (`InvalidParams`) | JSON-RPC `-32602` | tool args failed `inputSchema` (S8) |
| method not found | JSON-RPC `-32601` | unknown RPC method (SDK) |

The standard JSON-RPC codes (`-32700` parse, `-32600` invalid request,
`-32601` method not found, `-32602` invalid params, `-32603` internal error)
are used by the SDK for protocol-level conditions. `port.HTTPStatus` is logged
on upstream failures so the HTTP status remains observable alongside the
`IsError` result.

---

## 8. Quality requirements

### 8.1 Test hermeticity and per-layer strategy

No test touches the real network. Tests use `httptest` mocks of the **real**
Proxmox VE and PBS JSON APIs, port mocks, and stubs.

| Layer | Approach |
|---|---|
| `domain/model` | JSON serialization tests for every entity |
| `domain/port` | `WithToken`/`TokenFromContext`, `CtxLogger` fallback, error helpers |
| `application/*` | through `PVEGateway`/`PBSGateway` mocks (no network) |
| `adapter/pve` | `httptest` mock of the PVE API (assert method, `Authorization` header, URL path; return canned JSON for nodes/vms/lxc/storage/tasks) |
| `adapter/pbs` | `httptest` mock of the PBS API (datastores/snapshots/verify/prune) |
| `adapter/token` | config parsing + header building cases (target 100%) |
| `adapter/logging` | level mapping, format, request-aware tagging |
| `adapter/mcp` | each tool: success + error via a gateway stub; assert `IsError` on failure; assert tools absent when a backend is disabled |

### 8.2 Coverage

Design target: **95%+** overall — stricter than this org's reference projects
(which target 85%); this project's target is deliberately 95%+ (customer
requirement). The table-driven tests cover success, API `errors`, `401 →
ErrUnauthorized`, `500 → retry then UpstreamError`, empty `data`, invalid JSON,
missing token, and (for `adapter/mcp`) backend-disabled tool absence.

`cover-core` reports real-code coverage excluding generated mocks and the
`go test ./... -coverprofile=coverage.out` filtered via awk into
`coverage-core.out` (excludes generated mocks and the `cmd/mcp-proxmox`
composition root), so the 95%+ gate is measured on the logic that matters.
Mutation testing (gremlins) targets `domain` and `application`.

### 8.3 Lint and conventions

- `gofmt` / `go vet` clean; `golangci-lint` v2 per `.golangci.yml` with
  `errcheck`, `govet`, `staticcheck`, `ineffassign`, `unused`, `revive`,
  `depguard`, `nakedret`, plus `gosec` (security).
- Comments and code in English; package-level doc comments on exported types.
- No `internal` package (customer requirement); all packages public.
- `domain/*` and `application/*` must not import the MCP SDK, logrus, low-level `net/http`, resty,
  or any Proxmox client (enforced by review, by `.go-arch-lint.yml` /
  `go-arch-lint check`, and by the `depguard` clean-arch rule in
  `.golangci.yml`). outbound HTTP is done exclusively via `resty.dev/v3` confined to `adapter/pve` + `adapter/pbs`;
  logrus to `adapter/logging`; go-sdk to `adapter/mcp` + `cmd/mcp-proxmox`.
 - **Mutation testing:** `gremlins unleash` on `domain` + `application` with
  **efficacy ≥ 80% AND mutant-coverage ≥ 80%** (config in `gremlins.toml`:
  `efficacy = 80.0`, `mutant-coverage = 80.0`; generated mocks excluded). The
  Makefile `mutation` target passes `--threshold-efficacy=80 --threshold-mcover=80`.

### 8.4 Build and quality gates

The build system exposes the standard interface (R7) via **`Makefile`**:
`make build` (goreleaser), `make test` (hermetic `-race` tests + coverage ≥ 95%
gate), `make lint` (golangci-lint + vet + gofmt + go-arch-lint + gosec +
govulncheck), plus the dedicated hard-gate targets `make mutation` (gremlins)
and `make secrets` (gitleaks). CI binds to these `make` targets, so the same
interface works across providers. Direct tool invocations:

```bash
make lint      # golangci-lint, vet, gofmt, go-arch-lint, gosec, govulncheck
make test      # go test -race + coverage >= 95% (cover-core) gate
make mutation  # gremlins --threshold-efficacy=80 --threshold-mcover=80 (hard gate)
make secrets   # gitleaks detect over the full git history (hard gate)
make build     # goreleaser snapshot into dist/
make e2e       # hermetic e2e via go-docker-testsuite against e2e/emulator (requires Docker)
```

The CI pipeline runs on **a single provider — GitHub Actions**
(`.github/workflows/ci.yml`):

- **R6 — CI provider decision.** The repository is hosted on **GitHub.com**
  (`github.com/teran/mcp-proxmox`), whose native CI is **GitHub Actions**
  (`.github/workflows/ci.yml`); the release workflow
  (`.github/workflows/release.yml`) publishes binaries to **GitHub Releases** on
  tags. **GitHub Actions is the single, explicit CI provider** (R6): it is the
  repo host's native CI and the only pipeline maintained.
- **N31 / R7.** The workflow never invokes a language-specific command directly;
  each job calls a `make` target (`make lint`/`test`/`mutation`/`build`, and
  `make secrets`/`gitleaks` in its dedicated job). `make e2e` is declared and
  run in a dedicated `e2e` job (against the `e2e/emulator` Docker container via
  `go-docker-testsuite`); the `container-image` target is **not** declared (this
  is a Local stdio-only server).

The GitHub Actions pipeline **fails the build on any violation**: lint (`golangci-lint`), architecture
(`go-arch-lint`), **`go test -race`**, **gosec**, **govulncheck**, a **hard
coverage gate** — total `cover-core` below **95%** fails the pipeline — a
**secret scan** (`gitleaks`) over the full git history, and a
**mutation-testing (gremlins) hard gate** on `domain` + `application` that
**fails the build on surviving mutants** or below-threshold efficacy/mutant
coverage (80/80; no `allow_failure` / `continue-on-error`; C7/C8, N15/N19).
Releases are built and published by `goreleaser release --clean` on git tags.
All findings are fixed, never suppressed (no blanket `#nosec` / default
excludes).

### 8.5 TDD workflow

All fixes and features follow a **TDD workflow** with **isolated contexts**:

- **@qa** writes the tests (isolated context) against the `domain/port` contract
  and the architecture blueprint; QA owns `*_test.go`.
- **@developer** writes the implementation (isolated context) against the same
  contract; the developer owns non-test `.go` code.
- The two run in parallel; the task manager reconciles and runs the quality
  gates (lint → arch → test → cover ≥95% → mutation → build → sec → govulncheck)
  before merge. Neither agent edits the other's files.

### 8.6 e2e decision (C9GO / T2 / C4)

**A hermetic e2e suite is present.** The Go profile (C9GO) recommends e2e via
the `go-docker-testsuite` harness (`make e2e`, `//go:build e2e`) to cover the
full tool-handler → upstream path. This project implements it **without needing
a live Proxmox cluster**:

- **The upstream is emulated in-container.** `e2e/emulator` is an in-repo
  emulator of the Proxmox VE / PBS REST APIs that serves the **real** JSON
  payloads on `/api2/json`. The `go-docker-testsuite` harness runs the emulator
  as a Docker container, so e2e stays hermetic and CI-provisionable.
- **The real binary is driven over stdio.** The suite builds the actual
  `cmd/mcp-proxmox` binary, launches it as a subprocess pointed at the emulator,
  and drives the MCP protocol to assert a **real tool round-trip** (handler →
  adapter → emulator → response → tool output).

The suite is **build-tagged `//go:build e2e`** — it is excluded from the default
`go test ./...` unit pass (**T2/N30** satisfy the "build-tagged" requirement and
keep hermetic unit tests fast). It is run via the dedicated `make e2e` target
and a **dedicated `e2e` CI job** in `.github/workflows/ci.yml` (**C4/C9GO**;
requires Docker, provided by the job runner). The hermetic unit + coverage ≥95%
(**C1**) + mutation (**C2**) gates remain the hard gates; the e2e job
complements, never replaces, them.

---

## 9. Development workflow (summary)

- Default branch `master` (R2); feature/bugfix branches named `feat/<slug>`, `fix/<slug>`,
  `test/<slug>`, `docs/<slug>`; conventional commits.
- Open an MR against `master`; at least one approving review; CI green:
  lint → arch → test → cover (95%) → mutation → build → sec.
- Do not merge with below-threshold coverage/mutation or failing checks.

---

## 10. Open assumptions (to confirm before implementation)

1. **Go version:** `1.27.3` (matches the org's flagship `mcp-regcloud`; satisfies
   the go-sdk ≥ 1.24 requirement). Confirm the installed toolchain.
2. **Module name:** `github.com/teran/mcp-proxmox` — the repo's native namespace
   on **GitHub.com**, recorded in §2.4. If the server ever becomes a
   `go get`-able library or is published to a public module proxy, revisit the
   module path as a dedicated refactor.
3. **MCP SDK:** `github.com/modelcontextprotocol/go-sdk v1.8.0`.
4. **Config:** environment variables via `kelseyhightower/envconfig`
   (`PVE_ENDPOINT`/`PVE_TOKEN`/`PVE_CA_CERT_PATH`, `PBS_ENDPOINT`/`PBS_TOKEN`/
   `PBS_CA_CERT_PATH`, `LOG_LEVEL`); **no YAML file**. A backend is enabled only
   when both its endpoint and token are non-empty.
5. **Tool surface — phased/gated:** read-only (query) tools are **always**
   available. All mutations are gated behind the explicit **`ENABLE_MUTATIONS`**
   env opt-in (default `false`, SPEC.md §2.5; the seam is
   `Deps.EnableMutations` in `adapter/mcp`, honored during tool registration).
   The milestone default surface is read-only + system (`ping`/`status`).
6. **Exact tool list:** the tool registry in §6 is the agreed, implemented
   surface (74 tools with mutations enabled, 35 read-only by default); it may be
   trimmed/extended in later features (e.g. more PVE config options).
7. **API-token only** (no ticket/CSRF flow) — matches the stated requirement.
8. **Tokens from config, not per-call** — confirmed in §2.4; revisit only if a
   remote/OAuth2 relay is added.

## 11. TODO / Future work

Tracked follow-ups (not yet implemented). Each is a self-contained piece of work
with its own branch/PR, following the TDD workflow (§8.5).

1. **Pagination for large datastore snapshot listings (`pbs_backup_list` /
   `pbs_backup_get`).** The current implementation issues a single
   `GET /admin/datastore/{store}/snapshots` and returns the **entire** list
   in one response. On a datastore with many snapshots (observed: 132 snapshots
   for a single VM, and `pbs_backup_list` returning a multi-hundred-KB payload)
   this is slow and unbounded. PBS supports query-parameter paging
   (`limit`, and time-window `start`/`end` filters on `backup-time`); add
   pagination / windowing to avoid returning the whole set and to bound the
   response size. Applies the same reasoning as the `pbs_backup_files_list`
   fix (epoch `backup-time`).
2. **Fix `RestoreFile` / `RestoreVMBackup` endpoints.** Both still build the
   same wrong path as the (now fixed) `ListBackupFiles` did
   (`/admin/datastore/{store}/backups/{type}/{id}/{snapshot}/files` and
   `.../restore`), which returns 404 against a real PBS. Per the PBS source,
   these are `POST /admin/datastore/{store}/files` (single file) and
   `POST /admin/datastore/{store}/restore` (VM), with the same
   `backup-type`/`backup-id`/`backup-time` (epoch) parameters. After fixing,
   the legacy `backupFilesPath` helper can be removed entirely.
3. **Fix PBS task / verify / prune status endpoints.** `pbs_verify_status`,
   `pbs_prune_status`, `pbs_task_status` and `pbs_task_log` all return 404
   against a real PBS because they hit non-existent paths:
   `GET /admin/tasks/{upid}`, `GET /admin/tasks/{upid}/log`,
   `GET /admin/datastore/{store}/verify/{upid}`,
   `GET /admin/datastore/{store}/prune/{upid}`. PBS is a single node and tasks
   live under the node API. Confirmed live (a running verify job), the correct
   request is, e.g.:
   `GET /api2/json/nodes/pbs01/tasks/UPID%3Apbs01%3A...%3Ateran%40homelab!mcp%3A/status`
   i.e. `/nodes/{node}/tasks/{url-encoded-upid}/status` (and `/log` for the
   log; node = the PBS node, `localhost` also resolves). The UPID must be
   URL-encoded as a path segment (`:` → `%3A`, `@` → `%40`, `!` left literal) —
   not `url.PathEscape`'d and not appended raw. Verify/prune status are read
   through the same task endpoints (no separate `/verify/{upid}` /
   `/prune/{upid}`). Fix the gateway paths accordingly (task list is
   `GET /nodes/{node}/tasks`).

---

## Integration tests

This section defines the **live-cluster integration test plan** for `mcp-proxmox`. It complements (and does not replace) the hermetic unit/coverage/mutation gates (§8). It runs against a **real Proxmox VE cluster** and a **real Proxmox Backup Server (PBS)**, driving every one of the **74 tools** (§6) through the actual MCP stdio server with `ENABLE_MUTATIONS=true`. Every mutation is executed only against a brand-new, dedicated, throwaway resource; **no pre-existing resource is ever touched**.

> **Relationship to known-broken PBS tools (§11).** Seven PBS tools are currently broken against a real PBS. For these, the integration test **doubles as the fix-verification**: a failing run confirms the defect is still present, and a passing run proves the fix (the corrected endpoint is stated in the assertion). They are flagged ⚠️.

### I.1 Overview & isolation guarantee

**Isolation is the top priority of this plan.** All state-mutating tools operate **only** on a single brand-new dedicated throwaway VM named `int-test-<uuid>`, plus the additional dedicated throwaway objects required to cover the full 74-tool surface (a clone VM, two restore-target VMs, one LXC container, one backup job, one HA resource — all `int-test-<uuid>-*` prefixed). The plan **MUST NOT**:

- use, start, stop, reboot, reset, suspend, resize, migrate, clone, snapshot, roll back, back up, restore, or **delete** any **existing** VM, LXC, or HA resource;
- alter, update, or delete any **existing** storage, datastore, backup job, backup group, snapshot, or node;
- target any existing VMID/LXC ID — the test VM/CT always uses a VMID freshly allocated via `pve_nextid` (or an explicit `int-test` range) and verified absent in `pve_vm_list`/`pve_lxc_list`/`pve_cluster_resources` before any mutation.

Every created object carries the `int-test-<uuid>` name/tag so it is identifiable at a glance and cleanable in a single pass. The **only** objects created during the run are:

| Object | Created by | Removed by |
|---|---|---|
| `int-test-<uuid>` VM | `pve_vm_create` (Phase 1) | `pve_vm_delete` (Phase 9) |
| `int-test-<uuid>-snap1` VM snapshot | `pve_vm_snapshot_create` (Phase 3) | `pve_vm_snapshot_delete` (Phase 9) |
| `int-test-<uuid>-clone` VM | `pve_vm_clone` (Phase 6) | `pve_vm_delete` (Phase 9) |
| `int-test-<uuid>-restore` VM | `pve_vm_restore` (Phase 6) | `pve_vm_delete` (Phase 9) |
| `int-test-<uuid>-pbsrestore` VM | `pbs_backup_vm_restore` (Phase 6) | `pve_vm_delete` (Phase 9) |
| `int-test-<uuid>-lxc` container | out-of-band `pct create` (Phase 7) | `pve_lxc_delete` (Phase 9) |
| `int-test-<uuid>-lxc-snap` LXC snapshot | `pve_lxc_snapshot_create` (Phase 7) | `pve_lxc_snapshot_delete` (Phase 9) |
| `int-test-<uuid>-job` backup job | `pve_backup_job_create` (Phase 2) | `pve_backup_job_delete` (Phase 9) |
| `vm:<int-test-vmid>` HA resource | `pve_ha_add` (Phase 2) | out-of-band `ha-manager` (Phase 9) |
| PBS backup group `vm/<int-test-vmid>` | `pve_vm_backup` (Phase 3) | out-of-band PBS `forget` (Phase 9) |

### I.2 Prerequisites

| Requirement | Value / guidance |
|---|---|
| PVE cluster | Real Proxmox VE cluster reachable at `PVE_ENDPOINT` (e.g. `https://pve.example.com:8006`), **≥ 2 nodes** (`NODE_A`, `NODE_B`) so `pve_vm_migrate` is testable |
| PVE token | `PVE_TOKEN` — an API token (`user@realm!tokenid=uuid`) with full privileges (create/delete VMs & CTs, resize/migrate, vzdump, HA, config) on the **dedicated** nodes; TLS on, custom CA via `PVE_CA_CERT_PATH` if self-signed |
| PBS | `PBS_ENDPOINT` (e.g. `https://pbs.example.com:8007`) + `PBS_TOKEN` (`user@pbs!tokenid:secret`); a writable datastore (`store`) the token can list/verify/prune/gc |
| Mutations | `ENABLE_MUTATIONS=true` (otherwise only 35 read-only tools are registered — this plan needs all 74) |
| Logging | `LOG_LEVEL=info` (and optionally `trace`) so task/polling behavior is inspectable; `LOG_FILENAME` default is fine |
| Timeout | `HTTP_TIMEOUT` should be raised (e.g. `5m`) for long-running backups/restores/GC |
| Storage | An available disk storage (e.g. `local-lvm`) for the VM disks and a **backup storage** reachable by PVE (`pve_storage_list` in Phase 0) that feeds the PBS datastore |
| Network | An available bridge (e.g. `vmbr0`) for `net0` |
| Tooling | The MCP client / runner to drive the stdio server, plus SSH/shell access to `NODE_A` (for the out-of-band `pct` LXC create and `ha-manager`/`proxmox-backup-client` cleanup) |

**Recommendations (safety):** prefer a **test-only PVE cluster / PBS datastore** if available; otherwise point the token at an **isolated node** and a **dedicated scratch PBS datastore** so that the datastore-wide destructive PBS tools (`pbs_gc_start`, `pbs_prune_start`) only affect throwaway data. Reserve a small VMID range (e.g. 9000–9050) exclusively for `int-test` resources.

### I.3 Naming & tagging convention

Every object created by the run uses the prefix **`int-test-<uuid>`** where `<uuid>` is a single random UUID generated at run start (e.g. `int-test-3f9a…`):

- VM names: `int-test-<uuid>`, `int-test-<uuid>-clone`, `int-test-<uuid>-restore`, `int-test-<uuid>-pbsrestore`
- VM snapshots: `int-test-<uuid>-snap1`, `…-snap2`
- LXC: `int-test-<uuid>-lxc`; its snapshot: `int-test-<uuid>-lxc-snap`
- Backup job ID: `int-test-<uuid>-job`
- HA resource SID: `vm:<int-test-vmid>`
- PBS file-restore target: `/int-test-<uuid>/…` inside the datastore
- VM tags: set `tags=int-test-<uuid>` on created VMs/CTs so `pve_cluster_resources` / `pve_vm_get` confirms provenance.

The cleanup phase (`I.7`) discovers leftover objects **by this prefix only**, so it can never match a production resource.

### I.4 Scope & coverage accounting (all 74 tools)

System 2 + PVE read 21 + PVE write 32 + PBS read 12 + PBS write 7 = **74 tools**, all covered. Tools marked ⚠️ are the known-broken PBS tools from §11 (the run verifies the §11 fixes). Each phase table row lists: **Tool · Phase · Input (on int-test objects) · Expected · PASS/FAIL assertion**.

> **Node tracking after migration.** `pve_vm_migrate` (Phase 3) moves the VM from `NODE_A` to `NODE_B`. From that point every VM tool in later phases must pass `node=NODE_B`. The matrix says `node=host` and the runner substitutes the current host (`NODE_A` before Phase 3, `NODE_B` after).

### I.5 Phased test matrix

#### Phase 0 — System & discovery (read-only baseline)

Establishes the environment, picks `NODE_A`/`NODE_B`, the disk/backup storage, the network bridge, the PBS datastore, and reserves the int-test VMID. **No mutations.**

| # | Tool | Input | Expected | PASS assertion |
|---|---|---|---|---|
| 1 | `ping` | — | `{ "pve": true, "pbs": true }` | both backends reported enabled |
| 2 | `status` | — | version, transport (`stdio`), enabled backends | `pve` and `pbs` present; version non-empty |
| 3 | `pve_node_list` | — | all cluster nodes | **≥ 2 nodes**; capture `NODE_A`, `NODE_B` |
| 4 | `pve_node_status` | `node=NODE_A` | node status object | `online`; cpu/mem fields present |
| 5 | `pve_cluster_status` | — | quorum/health entries | quorum present; `pve_cluster_resources` reachable |
| 6 | `pve_cluster_resources` | — | VMs/CTs/storage across nodes | `int-test-<uuid>` VMID **absent** (isolation pre-check) |
| 7 | `pve_nextid` | — | a free VMID | numeric ID ≥ 100; reserve as `INT_TEST_VMID` |
| 8 | `pve_version` | — | PVE version object | `version`/`release` non-empty |
| 9 | `pve_storage_list` | — | cluster storage list | includes disk storage (e.g. `local-lvm`) and a **backup storage** |
| 10 | `pve_storage_get` | `node=NODE_A, storage=<disk-storage>` | storage status | `type`, `total`/`free` present |
| 11 | `pve_network_list` | `node=NODE_A` | network interfaces | includes a bridge (e.g. `vmbr0`) |
| 12 | `pve_vm_list` | `node=NODE_A` | VM list | `INT_TEST_VMID` **absent** |
| 13 | `pve_lxc_list` | `node=NODE_A` | CT list | no `int-test-*` container present |
| 14 | `pve_backup_job_list` | — | configured backup jobs | no `int-test-*` job present |
| 15 | `pve_task_list` | `node=NODE_A, limit=10` | task list | valid UPIDs returned (source for later) |
| 16 | `pbs_datastore_list` | — | datastores | `store` present |
| 17 | `pbs_datastore_status` | `store=<store>` | datastore status | `size`/`used`/`avail` present |
| 18 | `pbs_datastore_config_get` | `store=<store>` | datastore config | `path`, `keep-*` retention present |
| 19 | `pbs_version` | — | PBS version object | `version` non-empty |

#### Phase 1 — Create the dedicated int-test VM

| # | Tool | Input | Expected | PASS assertion |
|---|---|---|---|---|
| 20 | `pve_vm_create` | `node=NODE_A, vmid=INT_TEST_VMID, name=int-test-<uuid>, cores=1, memory=512, scsi0=<disk>:4, net0=vmbr0, ostype=l26, boot=order=scsi0, tags=int-test-<uuid>` | `{ "vmid": INT_TEST_VMID }` | VMID equals the reserved ID |
| 21 | `pve_vm_get` | `node=NODE_A, vmid=INT_TEST_VMID` | VM config | `name == int-test-<uuid>`, `cores == 1`, `memory == 512` |
| 22 | `pve_vm_status` | `node=NODE_A, vmid=INT_TEST_VMID` | status | `status == stopped` |
| 23 | `pve_task_list` | `node=NODE_A, vmid=INT_TEST_VMID, typefilter=qmcreate` | create task present | a `qmcreate` UPID exists |
| 24 | `pve_task_status` | `node=NODE_A, upid=<qmcreate UPID>` | task status | `status == stopped/OK`; `exitstatus == OK` |
| 25 | `pve_task_log` | `node=NODE_A, upid=<qmcreate UPID>` | log lines | non-empty log; contains expected create messages |

#### Phase 2 — Update config, resize, HA, backup job (int-test VM + int-test backup job)

| # | Tool | Input | Expected | PASS assertion |
|---|---|---|---|---|
| 26 | `pve_vm_set` | `node=NODE_A, vmid=INT_TEST_VMID, config={ "onboot": "0", "description": "int-test", "tags": "int-test-<uuid>" }` | success (no error) | `IsError == false`; re-`pve_vm_get` shows `onboot=0`, `description=int-test` |
| 27 | `pve_vm_resize` | `node=NODE_A, vmid=INT_TEST_VMID, disk=scsi0, size_gb=6` | success | `IsError == false`; re-`pve_vm_get` shows `scsi0` size grew from 4G to 6G |
| 28 | `pve_ha_add` | `sid=vm:<INT_TEST_VMID>, type=vm, nodes=[NODE_A,NODE_B], comment=int-test-<uuid>` | success | `IsError == false` (HA resource registered) |
| 29 | `pve_backup_job_create` | `id=int-test-<uuid>-job, storage=<backup-storage>, schedule="0 0 * * 0", vmid=<INT_TEST_VMID>, mode=snapshot, comment=int-test-<uuid>` | `{ "id": "int-test-<uuid>-job" }` | job ID returned / present in list |
| 30 | `pve_backup_job_update` | `id=int-test-<uuid>-job, mode=suspend, comment=int-test-<uuid>-updated` | `{ "id": ... }` | `IsError == false`; re-list shows updated fields |
| 31 | `pve_backup_job_list` | — | job list | `int-test-<uuid>-job` present |

#### Phase 3 — Snapshot, rollback, migrate, backup (user-required step 2)

VM is still **stopped** for snapshot/rollback/offline-migrate. After migrate the VM lives on `NODE_B`.

| # | Tool | Input | Expected | PASS assertion |
|---|---|---|---|---|
| 32 | `pve_vm_snapshot_create` | `node=NODE_A, vmid=INT_TEST_VMID, snapname=int-test-<uuid>-snap1, vmstate=false, description=int-test` | `{ "upid": ... }` | task UPID returned; poll `pve_task_status` to OK |
| 33 | `pve_vm_snapshot_list` | `node=NODE_A, vmid=INT_TEST_VMID` | snapshot list | contains `int-test-<uuid>-snap1` |
| 34 | `pve_vm_snapshot_rollback` | `node=NODE_A, vmid=INT_TEST_VMID, snapname=int-test-<uuid>-snap1` | `{ "upid": ... }` | `IsError == false`; task completes OK (VM still has the snapshot) |
| 35 | `pve_vm_migrate` | `node=NODE_A, vmid=INT_TEST_VMID, target=NODE_B, online=false` | success | `IsError == false`; `pve_vm_status node=NODE_B` shows the VM now on `NODE_B` |
| 36 | `pve_vm_backup` | `node=NODE_B, vmid=INT_TEST_VMID, storage=<backup-storage>, mode=snapshot, compress=zstd` | `{ "upid": ... }` | UPID returned; poll `pve_task_status` until `exitstatus == OK`; backup now present on PBS |

#### Phase 4 — View backups, files, snapshots (user-required step 3; read-only)

Requires the Phase 3 backup to exist. `backup_time` is the epoch read from the `pbs_backup_list` row.

| # | Tool | Input | Expected | PASS assertion |
|---|---|---|---|---|
| 37 | `pbs_backup_list` ⚠️ | `store=<store>` | backup snapshot list | contains `vm/<INT_TEST_VMID>`; **fix-verification:** after §11.1 pagination lands, returns a **bounded** page (not one unbounded multi-hundred-KB payload) with paging/`backup-time` window respected |
| 38 | `pbs_backup_get` ⚠️ | `store=<store>, backup_id=<INT_TEST_VMID>` | group snapshots | returns the int-test VM's snapshot(s); **fix-verification** for §11.1 |
| 39 | `pbs_backup_notes_get` | `store=<store>, backup_id=<INT_TEST_VMID>, backup_type=vm` | group notes | `IsError == false`; notes/comment fields returned (empty ok) |
| 40 | `pbs_backup_files_list` | `store=<store>, backup_type=vm, backup_id=<INT_TEST_VMID>, backup_time=<epoch>` | file list | non-empty file list for the int-test backup (e.g. `drive-scsi0.img.fw`/`.dat`) |
| 41 | `pve_vm_snapshot_list` | `node=NODE_B, vmid=INT_TEST_VMID` | snapshot list | `int-test-<uuid>-snap1` still present after migration |
| 42 | `pve_vm_get` | `node=NODE_B, vmid=INT_TEST_VMID` | VM config | config matches; `name == int-test-<uuid>` |
| 43 | `pve_vm_status` | `node=NODE_B, vmid=INT_TEST_VMID` | status | `status == stopped` |

#### Phase 5 — Start, reboot, soft/hard stop, suspend/resume/reset (user-required step 4; VM on NODE_B)

Order matters: **start → reboot → reset → suspend → resume → shutdown (soft) → start → stop (hard)**. Sequence so each lifecycle transition is exercised once while the VM is in a valid state.

| # | Tool | Input | Expected | PASS assertion |
|---|---|---|---|---|
| 44 | `pve_vm_start` | `node=NODE_B, vmid=INT_TEST_VMID` | `{ "upid": ... }` | task completes; `pve_vm_status` → `running` |
| 45 | `pve_vm_reboot` | `node=NODE_B, vmid=INT_TEST_VMID, timeout=30` | `{ "upid": ... }` | `IsError == false`; task OK; status `running` |
| 46 | `pve_vm_reset` | `node=NODE_B, vmid=INT_TEST_VMID` | `{ "upid": ... }` | `IsError == false`; task OK; status `running` |
| 47 | `pve_vm_suspend` | `node=NODE_B, vmid=INT_TEST_VMID, to_disk=true` | `{ "upid": ... }` | `IsError == false`; status `paused` |
| 48 | `pve_vm_resume` | `node=NODE_B, vmid=INT_TEST_VMID` | `{ "upid": ... }` | `IsError == false`; status `running` |
| 49 | `pve_vm_shutdown` | `node=NODE_B, vmid=INT_TEST_VMID, timeout=30` | `{ "upid": ... }` | task OK; status `stopped` (graceful) |
| 50 | `pve_vm_start` | `node=NODE_B, vmid=INT_TEST_VMID` | `{ "upid": ... }` | status `running` (re-start for hard stop) |
| 51 | `pve_vm_stop` | `node=NODE_B, vmid=INT_TEST_VMID` | `{ "upid": ... }` | task OK; status `stopped` (hard stop). **End of Phase 5: VM stopped** |

#### Phase 6 — Clone & restore (creates dedicated int-test restore targets)

| # | Tool | Input | Expected | PASS assertion |
|---|---|---|---|---|
| 52 | `pve_vm_clone` | `node=NODE_B, vmid=INT_TEST_VMID, newid=<CLONE_VMID>, name=int-test-<uuid>-clone, full=true` | `{ "upid": ... }` | task OK; `pve_vm_list` shows `int-test-<uuid>-clone` |
| 53 | `pve_vm_restore` | `node=NODE_B, archive=<path of int-test vzdump archive>, vmid=<RESTORE_VMID>, unique=true` | `{ "upid": ... }` | task OK; a new `int-test-<uuid>-restore` VM exists |
| 54 | `pbs_backup_vm_restore` ⚠️ | `store=<store>, backup_type=vm, backup_id=<INT_TEST_VMID>, snapshot=<ISO from pbs_backup_list>, target=<disk-storage>, vmid=<PBSRESTORE_VMID>` | `{ "upid": ... }` | **fix-verification (§11.2):** no 404 — request hits `POST /admin/datastore/{store}/restore`; returns a task UPID; a `int-test-<uuid>-pbsrestore` VM is created on PVE |
| 55 | `pbs_backup_restore_file` ⚠️ | `store=<store>, backup_type=vm, backup_id=<INT_TEST_VMID>, snapshot=<ISO>, path=drive-scsi0.img.fw, target=/int-test-<uuid>/out.fw` | success | **fix-verification (§11.2):** no 404 — request hits `POST /admin/datastore/{store}/files`; target file created under the `int-test-<uuid>` path on PBS |

#### Phase 7 — LXC tool set (dedicated int-test container)

There is **no `pve_lxc_create` tool** in the registry (SPEC §6.5), so a single dedicated container `int-test-<uuid>-lxc` is created **out-of-band** via `pct create <LXC_VMID> <template> --hostname int-test-<uuid>-lxc` on `NODE_A`. All 14 LXC tools then run against it only.

| # | Tool | Input | Expected | PASS assertion |
|---|---|---|---|---|
| 56 | `pve_lxc_list` | `node=NODE_A` | CT list | `int-test-<uuid>-lxc` present |
| 57 | `pve_lxc_get` | `node=NODE_A, vmid=<LXC_VMID>` | CT config | `hostname == int-test-<uuid>-lxc` |
| 58 | `pve_lxc_status` | `node=NODE_A, vmid=<LXC_VMID>` | CT status | `status == stopped` |
| 59 | `pve_lxc_set` | `node=NODE_A, vmid=<LXC_VMID>, config={ "description": "int-test", "onboot": "0" }` | success | `IsError == false`; re-`pve_lxc_get` shows the values |
| 60 | `pve_lxc_snapshot_create` | `node=NODE_A, vmid=<LXC_VMID>, snapname=int-test-<uuid>-lxc-snap` | `{ "upid": ... }` | task OK |
| 61 | `pve_lxc_snapshot_list` | `node=NODE_A, vmid=<LXC_VMID>` | snapshot list | contains `int-test-<uuid>-lxc-snap` |
| 62 | `pve_lxc_snapshot_rollback` | `node=NODE_A, vmid=<LXC_VMID>, snapname=int-test-<uuid>-lxc-snap` | `{ "upid": ... }` | `IsError == false`; task OK |
| 63 | `pve_lxc_start` | `node=NODE_A, vmid=<LXC_VMID>` | `{ "upid": ... }` | status `running` |
| 64 | `pve_lxc_reboot` | `node=NODE_A, vmid=<LXC_VMID>` | `{ "upid": ... }` | `IsError == false`; status `running` |
| 65 | `pve_lxc_shutdown` | `node=NODE_A, vmid=<LXC_VMID>, timeout=30` | `{ "upid": ... }` | status `stopped` |
| 66 | `pve_lxc_stop` | `node=NODE_A, vmid=<LXC_VMID>` | `{ "upid": ... }` | `IsError == false`; status `stopped` |
| 67 | `pve_lxc_clone` | `node=NODE_A, vmid=<LXC_VMID>, newid=<LXC_CLONE_VMID>, hostname=int-test-<uuid>-lxc-clone` | `{ "upid": ... }` | task OK; `pve_lxc_list` shows the clone |
| 68 | `pve_lxc_snapshot_delete` | `node=NODE_A, vmid=<LXC_VMID>, snapname=int-test-<uuid>-lxc-snap` | `{ "upid": ... }` | `IsError == false`; snapshot gone from list |
| 69 | `pve_lxc_delete` | `node=NODE_A, vmid=<LXC_CLONE_VMID>, purge=true` | `{ "upid": ... }` | clone container removed |

> **Assumption:** the clone (`LXC_CLONE_VMID`) is deleted here (69); the source `int-test-<uuid>-lxc` container is deleted in Phase 9 so `pve_lxc_delete` is exercised once in each phase boundary. If the operator prefers a single LXC delete, merge 69 into Phase 9 and skip one — coverage of `pve_lxc_delete` is what matters.

#### Phase 8 — PBS jobs: verify / GC / prune / sync / task-stop (mutation + status)

`pbs_verify_start` yields a UPID that is polled with `pbs_verify_status`/`pbs_task_status`/`pbs_task_log` and cancelled with `pbs_task_stop` (these four task/verify/prune status tools are the §11.3 known-broken group — **fix-verification**: they must now hit `GET /nodes/{node}/tasks/{url-encoded-upid}/status` and `/log`, not the old `/admin/...` paths).

| # | Tool | Input | Expected | PASS assertion |
|---|---|---|---|---|
| 70 | `pbs_verify_start` | `store=<store>` | `{ "upid": ... }` | UPID returned (verify job started on the datastore) |
| 71 | `pbs_verify_status` ⚠️ | `upid=<verify UPID>` | task status | **fix-verification (§11.3):** no 404 — `GET /nodes/{node}/tasks/{upid}/status`; returns `status`/`UPID` |
| 72 | `pbs_task_status` ⚠️ | `upid=<verify UPID>` | task status | no 404 — `/nodes/{node}/tasks/{upid}/status`; status present |
| 73 | `pbs_task_log` ⚠️ | `upid=<verify UPID>, limit=50` | log lines | no 404 — `/nodes/{node}/tasks/{upid}/log`; non-empty log |
| 74 | `pbs_task_stop` | `upid=<verify UPID>` | success | no 404 — `DELETE /nodes/{node}/tasks/{upid}`; task cancelled (or already stopped) |
| 75 | `pbs_prune_start` | `store=<store>` | `{ "upid": ... }` | UPID returned |
| 76 | `pbs_prune_status` ⚠️ | `upid=<prune UPID>` | task status | no 404 — `/nodes/{node}/tasks/{upid}/status` |
| 77 | `pbs_gc_start` | `store=<store>` | `{ "upid": ... }` | UPID returned |
| 78 | `pbs_sync_start` | `store=<store>, remote=<configured remote>` | `{ "upid": ... }` or documented APIError | PASS = request reaches `POST /admin/datastore/{store}/sync` (a **semantic** error about an unconfigured remote is acceptable; a 404 is a FAIL) |

> **Destructive note (safety):** `pbs_gc_start` (removes unreferenced datastore chunks) and `pbs_prune_start` (removes snapshots per retention) are **datastore-wide**. On a shared datastore they are **smoke-tested only** (start → UPID returned → verify the int-test backup group is still listed by `pbs_backup_get`). On a dedicated scratch datastore they are run to completion and the datastore is discarded afterwards.

#### Phase 9 — Cleanup & verification

Removes every `int-test-*` object, oldest/most-nested first, then verifies nothing remains.

| # | Tool | Input | Expected | PASS assertion |
|---|---|---|---|---|
| 79 | `pve_vm_snapshot_delete` | `node=NODE_B, vmid=INT_TEST_VMID, snapname=int-test-<uuid>-snap1` | `{ "upid": ... }` | task OK; snapshot gone |
| 80 | `pve_vm_delete` | `node=NODE_B, vmid=<PBSRESTORE_VMID>, purge=true` | `{ "upid": ... }` | restore-target VM removed |
| 81 | `pve_vm_delete` | `node=NODE_B, vmid=<RESTORE_VMID>, purge=true` | `{ "upid": ... }` | restore-target VM removed |
| 82 | `pve_vm_delete` | `node=NODE_B, vmid=<CLONE_VMID>, purge=true` | `{ "upid": ... }` | clone VM removed |
| 83 | `pve_vm_delete` | `node=NODE_B, vmid=INT_TEST_VMID, purge=true` | `{ "upid": ... }` | **main int-test VM removed** (final step 5) |
| 84 | `pve_lxc_delete` | `node=NODE_A, vmid=<LXC_VMID>, purge=true` | `{ "upid": ... }` | int-test LXC removed |
| 85 | `pve_backup_job_delete` | `id=int-test-<uuid>-job` | `{ "deleted": true }` | job gone from `pve_backup_job_list` |
| 86 | (out-of-band) `ha-manager` | remove `vm:<INT_TEST_VMID>` | removed | HA resource gone (no `pve_ha_remove` tool exists) |
| 87 | (out-of-band) PBS `forget` | forget `vm/<INT_TEST_VMID>` group | removed | PBS backup group gone (no PBS `forget` tool exists) |
| 88 | (out-of-band) rm | remove `/int-test-<uuid>/` on PBS | removed | file-restore target removed |
| 89 | `pve_cluster_resources` | — | resource list | **no** `int-test-*` resource remains |
| 90 | `pve_vm_list` / `pve_lxc_list` | `node=NODE_A` and `node=NODE_B` | lists | no `int-test-*` VM/CT on either node |
| 91 | `pve_backup_job_list` | — | job list | no `int-test-*` job |
| 92 | `pbs_backup_list` ⚠️ | `store=<store>` | snapshot list | no `vm/<INT_TEST_VMID>` group remains |

### I.6 Ordering & dependencies

- **Phase 0 → 1:** create requires `NODE_A`, the disk storage, the bridge, and a reserved VMID (from `pve_nextid`).
- **Phase 1 → 2:** update/resize/HA/backup-job all require the VM to exist.
- **Phase 2 → 3:** snapshot/rollback require the VM (HA-add must precede nothing, but backup-job creation is independent); migrate requires a second node.
- **Phase 3 → 4:** `pbs_backup_list`/`get`/`files_list` and the VM snapshot view require the backup (Phase 3) and the snapshot to exist.
- **Phase 4 → 5:** lifecycle requires the VM to be present and (for rollback/snapshot-free ops) stopped/running per the sequence.
- **Phase 5 → 6:** clone/restore require a stopped source VM and an existing backup archive.
- **Phase 7:** LXC tools are independent of the VM, but share the node; run after Phase 6 to avoid contention.
- **Phase 8:** PBS status tools require a UPID produced by a PBS job start (Phase 8 itself); `pbs_task_status`/`pbs_task_log` reuse the verify UPID.
- **Phase 9:** must be the **last** phase — delete removes every object created in Phases 1–8 and runs the final zero-leftover verification.

**Hard rule enforcement in ordering:** no mutation anywhere references a VMID/LXC ID other than those allocated by this plan; every deletion in Phase 9 targets an object created earlier in this run (verified by the `int-test-<uuid>` name/tag/prefix).

### I.7 Cleanup

Explicit removal, nested/derived resources first, in this order:

1. **VM snapshots** before the VM: `pve_vm_snapshot_delete` (79).
2. **All dedicated VMs:** `pve_vm_delete` for the PBS-restore target (80), qmrestore target (81), clone (82), and the main `int-test` VM (83) — always with `purge=true`.
3. **The dedicated LXC container:** `pve_lxc_delete` (84); its clone was already deleted in Phase 7 (69) and its snapshot in Phase 7 (68).
4. **The backup job:** `pve_backup_job_delete` (85).
5. **Out-of-band removals** (no corresponding MCP tool exists): the HA resource `vm:<INT_TEST_VMID>` via `ha-manager` (86); the PBS backup group `vm/<INT_TEST_VMID>` via `proxmox-backup-client forget` (87); the `/int-test-<uuid>/` file-restore path (88).
6. **Verification of zero leftovers:** re-run `pve_cluster_resources`, `pve_vm_list`/`pve_lxc_list` on both nodes, `pve_backup_job_list`, and `pbs_backup_list`, asserting **no** `int-test-*`/`vm/<INT_TEST_VMID>` remains (89–92). If any `int-test-*` object is found, remove it by its prefix before declaring the run clean.

> A leftover check is **mandatory**, not advisory: because every object carries the `int-test-<uuid>` prefix, the sweep can unambiguously identify and remove only test objects and can never match production resources.

### I.8 Safety rollback

If a destructive step fails mid-way, the runner must:

1. **Stop immediately** and take an inventory snapshot of `int-test-*` objects (via `pve_cluster_resources`, `pve_vm_list`, `pve_lxc_list`, `pve_backup_job_list`, `pbs_backup_list`).
2. **Never retry a failed mutation blindly** — mutations are not idempotent (§7.5); a retry could double-create. Inspect the returned error (`IsError`, HTTP status) and the logged `request_id` first.
3. **Resume at the failed phase, not from the start** — earlier phases that succeeded (e.g. VM created) should be reused, not re-created.
4. **If cleanup itself fails** (e.g. `pve_vm_delete` errors because the VM is locked by a running task), resolve the lock first (poll `pve_task_status`; stop the task), then retry the delete. Do **not** force-delete or mutate any other resource.
5. **Confirm the guard invariant** before every destructive call: the target `vmid`/`lxc`/`snapname`/`job id`/`upid`/`store` must equal a value captured earlier in this run from the matrix — a VMID/LXC ID that was **not** allocated by this run is an immediate abort condition.
6. **No existing VM/LXC is ever a target** — enforced by (5): every destructive target must match the `int-test-<uuid>` prefix or a run-allocated VMID. If a target cannot be positively identified as run-created, the operation is aborted and escalated to the operator.

### I.9 Success criteria

The integration run **passes** only when all of the following hold:

1. **All read tools return valid structured output** — every one of the 21 PVE read tools, 12 PBS read tools, and 2 system tools returns well-formed JSON that validates against its output schema (no `IsError`, no empty/`null` data where an entity is expected).
2. **All 32 PVE mutations and 7 PBS mutations complete against int-test objects only** — each returns the expected UPID/ID/success and the resulting state is confirmed (VM created→migrated→backed-up→snapshotted→lifecycled→deleted; clone/restore-targets cleaned up; LXC lifecycle completes; backup job created/updated/deleted).
3. **All known-broken PBS tools now pass** ⚠️ — `pbs_backup_list`, `pbs_backup_get`, `pbs_backup_restore_file`, `pbs_backup_vm_restore`, `pbs_verify_status`, `pbs_prune_status`, `pbs_task_status`, `pbs_task_log` return **200/OK against the corrected endpoints** (per §11.2/§11.3; the legacy paths that returned 404 no longer do), and the pagination fix (§11.1) returns bounded pages on a populated datastore.
4. **Zero leftover state** — after Phase 9 no `int-test-*` VM/CT/clone/restore-target/snapshot/backup-job/HA-resource/PBS-group/path remains on either node or the PBS datastore, and no pre-existing resource was altered (verified by comparing Phase 0 discovery against Phase 9 discovery).
5. **Task-driven tools verified end-to-end** — every mutation that returns a UPID (`start`/`stop`/`shutdown`/`reboot`/`reset`/`suspend`/`resume`/`backup`/`migrate`/`clone`/`restore`/`snapshot_*`/PBS job starts) had its task polled to a terminal `OK` state via `pve_task_status`/`pbs_task_status` where applicable.

**Reporting:** the run emits a per-tool table with `# · tool · phase · PASS/FAIL · observed`. Any FAIL on a non-⚠️ tool is a regression to fix before merge; any FAIL on a ⚠️ tool confirms the corresponding §11 defect is still open.
