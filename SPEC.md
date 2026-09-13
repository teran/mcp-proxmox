# mcp-proxmox — Technical Specification

**Language:** English (fixed by the user — do not translate).
**Module:** `github.com/teran/mcp-proxmox`
**Go:** 1.27.0
**Status:** design snapshot (authoritative technical reference; implementation to follow).

This document is the authoritative technical reference for the `mcp-proxmox`
project: the source of truth for the intended architecture, tool registry, run
modes, authentication model, and quality requirements. It is written *before*
the implementation so the design decisions are fixed up front; `README.md`,
`AGENTS.md`, and CI configuration will be derived from it.

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
- SDK: [`github.com/modelcontextprotocol/go-sdk v1.7.0`](https://github.com/modelcontextprotocol/go-sdk).
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
├── .gitlab-ci.yml               # GitLab pipeline (lint → arch → test → cover → mutation → build)
├── .forgejo/workflows/ci.yml    # Forgejo Actions CI
├── LICENSE                      # Apache-2.0, © 2026 Igor Shishkin
├── SPEC.md                      # this document
├── AGENTS.md                    # derived later
└── README.md                    # user-facing documentation (derived later)
```

### 2.4 Architectural decisions and rationale

**Clean architecture without `internal`.** Every package lives at the repo root
and is public; the core (`domain` + `application`) has zero dependencies on the
go-sdk, the low-level `net/http`, logrus, or any Proxmox client. This is a customer requirement
and is enforced by `.go-arch-lint.yml` (`go-arch-lint check`), not
by a hidden `internal/` directory. Dependencies point inward only through the
`domain/port` interfaces.

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
| `PBS_TOKEN` | PBS API token (`user@pbs!tokenid=uuid`); enables PBS when non-empty |
| `PBS_CA_CERT_PATH` | optional custom CA (PEM) for the PBS endpoint; empty = system roots |
| `LOG_LEVEL` | log level `trace\|debug\|info\|warn\|error`; **when set, logging is enabled** (to `LOG_FILENAME`); when unset, logging is **disabled** (L2) |
| `LOG_FILENAME` | log file path used when `LOG_LEVEL` is set (default `/tmp/mcp-proxmox.log`, mode `0600`) (L3) |
| `LOG_FORMAT` | log format `text` (default) \| `json` (L4) |

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

When logging is enabled at **debug/trace**, the server middleware emits a
structured per-request line for every incoming MCP call with fields: `tool`
(name, for `tools/call`), `source` (`"STDIO"`), `duration_ms`, `outcome`
(`ok`/`error`), and (only at trace) `args` — with sensitive keys redacted — plus
`in_bytes`/`out_bytes`. Every line carries `session_id`/`request_id` from `ctx`.
For outbound upstream requests, each gateway **forwards the `request_id` as an
`X-Request-ID` header** and logs method/path/status/duration/in_bytes/out_bytes
with the same `request_id` (SPEC §5.5).

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

---

## 6. Tool registry (complete)

**46 tools** are registered. Naming uses the domain prefixes `pve_` and `pbs_`;
system tools (`ping`, `status`) have no prefix. `query` = read (idempotent,
retried); `mutation` = write (never retried).

> The exact registry may be trimmed or extended during implementation, but this
> list is the agreed, defensible surface. Every endpoint below exists in the
> real Proxmox VE / PBS REST APIs (`/api2/json`).

### 6.0 Tool metadata (Annotations & Instructions) — M4/N11

Every registered tool carries MCP **Annotations (hints)** and per-tool
**Instructions**, set via the go-sdk `Tool` fields:

- `Title` — a short human-readable display title (e.g. "List PVE nodes").
- `readOnlyHint`, `destructiveHint`, `idempotentHint`, `openWorldHint`.
- `instructions` — natural-language guidance (inputs, ordering, side effects,
  precautions) for the model, encoded into the SDK `Description` (the field the
  go-sdk treats as the model hint).

The hints follow a fixed mapping (`adapter/mcp/helpers.go` `roTool`/`sysTool`):

| Tool family | readOnly | destructive | idempotent | openWorld |
|---|---|---|---|---|
| read / query | `true` | `false` | `true` | `false` |
| write / update (planned, gated) | `false` | `false` | `true` | `false` |
| delete (planned, gated) | `false` | `true` | `false` | `false` |

The currently-registered surface is entirely read-only (system + PVE + PBS
queries), so all tools are `readOnly=true, destructive=false, idempotent=true,
openWorld=false`. Mutation tools behind the gate (§2.5) will follow the write /
delete rows above.

### 6.1 System (2)

| # | Tool | Domain | Type | Description |
|---|---|---|---|---|
| 1 | `ping` | system | query | liveness; reports enabled backends (`pve`, `pbs`) |
| 2 | `status` | system | query | server status: version, transport, enabled backends |

### 6.2 Proxmox VE — nodes, cluster & HA (6)

| # | Tool | Type | Endpoint |
|---|---|---|---|
| 3 | `pve_node_list` | query | `GET /nodes` |
| 4 | `pve_node_status` | query | `GET /nodes/{node}/status` |
| 5 | `pve_cluster_status` | query | `GET /cluster/status` |
| 6 | `pve_cluster_resources` | query | `GET /cluster/resources` |
| 7 | `pve_nextid` | query | `GET /cluster/nextid` |
| 8 | `pve_ha_add` | mutation | `POST /cluster/ha/resources` |

### 6.3 Proxmox VE — QEMU VMs (12)

| # | Tool | Type | Endpoint |
|---|---|---|---|
| 9 | `pve_vm_list` | query | `GET /nodes/{node}/qemu` |
| 10 | `pve_vm_get` | query | `GET /nodes/{node}/qemu/{vmid}/config` |
| 11 | `pve_vm_status` | query | `GET /nodes/{node}/qemu/{vmid}/status/current` |
| 12 | `pve_vm_create` | mutation | `POST /nodes/{node}/qemu` |
| 13 | `pve_vm_resize` | mutation | `PUT /nodes/{node}/qemu/{vmid}/resize` |
| 14 | `pve_vm_start` | mutation | `POST /nodes/{node}/qemu/{vmid}/status/start` |
| 15 | `pve_vm_stop` | mutation | `POST /nodes/{node}/qemu/{vmid}/status/stop` |
| 16 | `pve_vm_reboot` | mutation | `POST /nodes/{node}/qemu/{vmid}/status/reboot` |
| 17 | `pve_vm_shutdown` | mutation | `POST /nodes/{node}/qemu/{vmid}/status/shutdown` |
| 18 | `pve_vm_migrate` | mutation | `POST /nodes/{node}/qemu/{vmid}/migrate` |
| 19 | `pve_vm_backup` | mutation | `POST /nodes/{node}/vzdump` |
| 20 | `pve_vm_delete` | mutation | `DELETE /nodes/{node}/qemu/{vmid}` |

### 6.4 Proxmox VE — LXC containers (9)

| # | Tool | Type | Endpoint |
|---|---|---|---|
| 21 | `pve_lxc_list` | query | `GET /nodes/{node}/lxc` |
| 22 | `pve_lxc_get` | query | `GET /nodes/{node}/lxc/{vmid}/config` |
| 23 | `pve_lxc_status` | query | `GET /nodes/{node}/lxc/{vmid}/status/current` |
| 24 | `pve_lxc_create` | mutation | `POST /nodes/{node}/lxc` |
| 25 | `pve_lxc_start` | mutation | `POST /nodes/{node}/lxc/{vmid}/status/start` |
| 26 | `pve_lxc_stop` | mutation | `POST /nodes/{node}/lxc/{vmid}/status/stop` |
| 27 | `pve_lxc_reboot` | mutation | `POST /nodes/{node}/lxc/{vmid}/status/reboot` |
| 28 | `pve_lxc_shutdown` | mutation | `POST /nodes/{node}/lxc/{vmid}/status/shutdown` |
| 29 | `pve_lxc_delete` | mutation | `DELETE /nodes/{node}/lxc/{vmid}` |

### 6.5 Proxmox VE — storage & network (3)

| # | Tool | Type | Endpoint |
|---|---|---|---|
| 30 | `pve_storage_list` | query | `GET /storage` |
| 31 | `pve_storage_get` | query | `GET /nodes/{node}/storage/{storage}/status` |
| 32 | `pve_network_list` | query | `GET /nodes/{node}/network` |

### 6.6 Proxmox VE — tasks & version (4)

| # | Tool | Type | Endpoint |
|---|---|---|---|
| 33 | `pve_task_list` | query | `GET /nodes/{node}/tasks` |
| 34 | `pve_task_status` | query | `GET /nodes/{node}/tasks/{upid}/status` |
| 35 | `pve_task_log` | query | `GET /nodes/{node}/tasks/{upid}/log` |
| 36 | `pve_version` | query | `GET /version` |

### 6.7 PBS — datastores (2)

| # | Tool | Type | Endpoint |
|---|---|---|---|
| 37 | `pbs_datastore_list` | query | `GET /admin/datastore` |
| 38 | `pbs_datastore_status` | query | `GET /admin/datastore/{store}/status` |

### 6.8 PBS — backups/snapshots & notes (6)

| # | Tool | Type | Endpoint |
|---|---|---|---|
| 39 | `pbs_backup_list` | query | `GET /admin/datastore/{store}/snapshot` |
| 40 | `pbs_backup_get` | query | `GET /admin/datastore/{store}/snapshot/{snapshot}` |
| 41 | `pbs_backup_restore` | mutation | `POST /admin/datastore/{store}/snapshot/{snapshot}/restore` |
| 42 | `pbs_backup_forget` | mutation | `DELETE /admin/datastore/{store}/snapshot/{snapshot}` |
| 43 | `pbs_backup_notes_get` | query | `GET /admin/datastore/{store}/snapshot/{snapshot}/notes` |
| 44 | `pbs_backup_notes_set` | mutation | `PUT /admin/datastore/{store}/snapshot/{snapshot}/notes` |

### 6.9 PBS — verify, GC, prune, sync & version (7)

| # | Tool | Type | Endpoint |
|---|---|---|---|
| 45 | `pbs_verify_start` | mutation | `POST /admin/datastore/{store}/verify` |
| 46 | `pbs_verify_status` | query | `GET /admin/datastore/{store}/verify/{upid}` |
| 47 | `pbs_gc_start` | mutation | `POST /admin/datastore/{store}/gc` |
| 48 | `pbs_prune_start` | mutation | `POST /admin/datastore/{store}/prune` |
| 49 | `pbs_prune_status` | query | `GET /admin/datastore/{store}/prune/{upid}` |
| 50 | `pbs_sync_start` | mutation | `POST /admin/datastore/{store}/sync` |
| 51 | `pbs_version` | query | `GET /version` |

**Total: 51 tools** (system 2, PVE 34, PBS 15).

> **Implemented mutations (behind `ENABLE_MUTATIONS=true`, §2.5):**
> PVE — `pve_vm_create`, `pve_vm_resize`, `pve_vm_migrate`, `pve_ha_add`,
> `pve_vm_backup`. PBS — `pbs_verify_start`, `pbs_gc_start`, `pbs_prune_start`,
> `pbs_sync_start`. The remaining mutation rows (start/stop/reboot/shutdown/
> delete, LXC, restore/forget/notes-set) are planned and not yet registered.

> **Restore/forget semantics (PBS):** `pbs_backup_restore` restores a snapshot
> and returns the resulting task UPID; `pbs_backup_forget` removes a snapshot
> from the datastore catalog. Both are mutations and are **never retried**.

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
  efficacy ≥ 80% (config in `gremlins.toml`; generated mocks excluded).

### 8.4 Build and quality gates

There is **no Makefile**. Builds use **goreleaser** (`.goreleaser.yml`);
quality gates run directly:

```bash
golangci-lint run ./...   # lint
go-arch-lint check        # architecture / dependency rule
go test ./...             # hermetic tests
go vet ./... && gofmt -l .
gosec ./...               # security
gremlins unleash --workers 4 --timeout-coefficient 50 ./application -E '.*mocks.*'
gremlins unleash --workers 4 --timeout-coefficient 50 ./domain
```

The CI pipeline (`.gitlab-ci.yml`, `.forgejo/workflows/ci.yml`) runs, and
**fails the build on any violation**: lint (`golangci-lint`), architecture
(`go-arch-lint`), **`go test -race`**, **gosec**, **govulncheck**, a **hard
coverage gate** — total `cover-core` below **95%** fails the pipeline — and a
**mutation-testing (gremlins) hard gate** on `domain` + `application` that
**fails the build on surviving mutants** (no `allow_failure` /
`continue-on-error`; C7/C8, N15/N19). Releases are built and published by
`goreleaser release --clean` on git tags. All findings are fixed, never
suppressed (no blanket `#nosec` / default excludes).

### 8.5 TDD workflow

All fixes and features follow a **TDD workflow** with **isolated contexts**:

- **@qa** writes the tests (isolated context) against the `domain/port` contract
  and the architecture blueprint; QA owns `*_test.go`.
- **@developer** writes the implementation (isolated context) against the same
  contract; the developer owns non-test `.go` code.
- The two run in parallel; the task manager reconciles and runs the quality
  gates (lint → arch → test → cover ≥95% → mutation → build → sec → govulncheck)
  before merge. Neither agent edits the other's files.

---

## 9. Development workflow (summary)

- Default branch `main`; feature/bugfix branches named `feat/<slug>`, `fix/<slug>`,
  `test/<slug>`, `docs/<slug>`; conventional commits.
- Open an MR against `main`; at least one approving review; CI green:
  lint → arch → test → cover (95%) → mutation → build → sec.
- Do not merge with below-threshold coverage/mutation or failing checks.

---

## 10. Open assumptions (to confirm before implementation)

1. **Go version:** `1.27.0` (matches the org's flagship `mcp-regcloud`; satisfies
   the go-sdk ≥ 1.24 requirement). Confirm the installed toolchain.
2. **Module name:** `github.com/teran/mcp-proxmox`.
3. **MCP SDK:** `github.com/modelcontextprotocol/go-sdk v1.7.0`.
4. **Config:** environment variables via `kelseyhightower/envconfig`
   (`PVE_ENDPOINT`/`PVE_TOKEN`/`PVE_CA_CERT_PATH`, `PBS_ENDPOINT`/`PBS_TOKEN`/
   `PBS_CA_CERT_PATH`, `LOG_LEVEL`); **no YAML file**. A backend is enabled only
   when both its endpoint and token are non-empty.
5. **Tool surface — phased/gated:** read-only (query) tools are **always**
   available. All mutations are gated behind the explicit **`ENABLE_MUTATIONS`**
   env opt-in (default `false`, SPEC.md §2.5; the seam is
   `Deps.EnableMutations` in `adapter/mcp`, honored during tool registration).
   The milestone default surface is read-only + system (`ping`/`status`).
6. **Exact tool list:** the 51-tool registry in §6 is the agreed surface; it may
   be trimmed/extended during implementation (e.g. more PVE config options).
7. **API-token only** (no ticket/CSRF flow) — matches the stated requirement.
8. **Tokens from config, not per-call** — confirmed in §2.4; revisit only if a
   remote/OAuth2 relay is added.
