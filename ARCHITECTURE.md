# mcp-proxmox — Application-Level Architecture Blueprint

**Language:** English.
**Authoritative source:** [`SPEC.md`](./SPEC.md). This document is the
application-level blueprint that turns the spec into a concrete, stable contract
so a developer and a QA engineer can implement and test in parallel without
re-deriving the design. Where the locked grill decisions refine `SPEC.md`, the
grill decisions win (and `SPEC.md` has been updated to match).

> **Milestone scope.** This blueprint fixes the **read-only (query) surface +
> system tools (`ping`/`status`)** as the vertical slice. All mutations are gated
> off (gate UX TBD). A developer implements the RO tools and gateway bodies; QA
> writes the hermetic tests. Both work against the interfaces and layouts below.

---

## 1. Package layout (final)

```
mcp-proxmox/
├── cmd/mcp-proxmox/main.go      # composition root: flags, envconfig, wiring, session, gate
├── domain/
│   ├── model/                   # pure types (std only): node, vm, lxc, storage,
│   │                            #   network, cluster, task, datastore, backup,
│   │                            #   verify, prune, version, status
│   └── port/                    # PVEGateway, PBSGateway, TokenSource, AppLogger,
│                                #   CtxLogger, sentinel errors, WithToken helpers
├── application/
│   ├── app.go                   # App facade + WithPVE/WithPBS/New options
│   ├── pve_service.go           # PVE use cases (read-only)
│   ├── pbs_service.go           # PBS use cases (read-only)
│   └── system_service.go        # ping, status
├── adapter/
│   ├── config/                  # envconfig loader + Config struct
│   ├── pve/                     # Proxmox VE HTTP client → port.PVEGateway
│   ├── pbs/                     # Proxmox Backup Server HTTP client → port.PBSGateway
│   ├── token/                   # StaticTokenSource → port.TokenSource
│   ├── logging/                 # logrus setup, AppLogger, slog→logrus, WithSession
│   └── mcp/                     # go-sdk server assembly + tool registry
├── go.mod / go.sum              # module github.com/teran/mcp-proxmox, go 1.27.0
├── .golangci.yml, .go-arch-lint.yml, gremlins.toml,
├── Dockerfile, .goreleaser.yml, .dockerignore, .gitignore,
├── .gitlab-ci.yml, .forgejo/workflows/ci.yml
├── SPEC.md (authoritative), AGENTS.md, README.md, ARCHITECTURE.md (this file)
└── LICENSE
```

**No `internal` package** (customer requirement). Every package is public and
lives at the repo root. Dependency rule points **inward only**:

```
domain/model ← domain/port ← application ← adapter/* ← cmd/mcp-proxmox
```

Constraints enforced by `go-arch-lint check` (`.go-arch-lint.yml`) and the `depguard`
rule in `.golangci.yml`:

- `domain/*`, `application/*` never import go-sdk, `net/http`, logrus,
  envconfig, or any Proxmox client.
- `net/http` is confined to `adapter/pve` + `adapter/pbs`.
- logrus is confined to `adapter/logging` (+ select adapter files for
  trace/debug).
- the go-sdk is confined to `adapter/mcp` + `cmd/mcp-proxmox`.

---

## 2. The ports (domain/port) — the stable contract

### 2.1 `port.TokenSource`
```go
type TokenSource interface {
    // AuthorizationHeader returns a ready-to-send Authorization header value,
    // e.g. "PVEAPIToken=user@realm!tokenid=uuid". Never logs the token.
    AuthorizationHeader(ctx context.Context) (string, error)
}
```
- **Local:** `adapter/token.StaticTokenSource` returns `PVEAPIToken=<token>`.
- **Future remote:** an `OAuth2TokenSource` / per-request header relay returns
  `Bearer <token>`. `port.WithToken`/`port.TokenFromContext` exist for that
  relay seam. Gateways consume only the header string.

### 2.2 `port.AppLogger` / `port.CtxLogger`
Mirrors `mcp-regcloud`: `AppLogger` has `Debugf/Infof/Warnf/Errorf`. The optional
`RequestAwareLogger` adds `*fContext(ctx, ...)` variants that tag lines with
`session_id`/`request_id` via `logging.WithSession`. `port.NewCtxLogger(l)`
wraps any `AppLogger` and falls back to the plain logger for test dummies.

### 2.3 `port.PVEGateway` (read-only surface)
```go
type PVEGateway interface {
    // Nodes & cluster
    ListNodes(ctx) ([]model.Node, error)                    // GET /nodes
    GetNodeStatus(ctx, node) (*model.NodeStatus, error)     // GET /nodes/{node}/status
    GetClusterStatus(ctx) ([]model.ClusterStatus, error)    // GET /cluster/status
    GetClusterResources(ctx) ([]model.ClusterResource, error)// GET /cluster/resources
    GetNextID(ctx) (int, error)                             // GET /cluster/nextid

    // QEMU VMs
    ListVMs(ctx, node) ([]model.VM, error)                  // GET /nodes/{node}/qemu
    GetVMConfig(ctx, node, vmid) (*model.VMConfig, error)   // GET .../{vmid}/config
    GetVMStatus(ctx, node, vmid) (*model.VMStatus, error)   // GET .../{vmid}/status/current

    // LXC containers
    ListLXCs(ctx, node) ([]model.LXC, error)
    GetLXCConfig(ctx, node, vmid) (*model.LXCConfig, error)
    GetLXCStatus(ctx, node, vmid) (*model.LXCStatus, error)

    // Storage & network
    ListStorage(ctx) ([]model.Storage, error)
    GetStorageStatus(ctx, node, storage) (*model.StorageStatus, error)
    ListNetwork(ctx, node) ([]model.NetworkInterface, error)

    // Tasks & version
    ListTasks(ctx, node, opts port.TaskListOptions) ([]model.Task, error)
    GetTaskStatus(ctx, node, upid) (*model.TaskStatus, error)
    GetTaskLog(ctx, node, upid, limit) ([]model.TaskLogEntry, error)
    GetPVEVersion(ctx) (*model.PVEVersion, error)
}
```

### 2.4 `port.PBSGateway` (read-only surface)
```go
type PBSGateway interface {
    ListDatastores(ctx) ([]model.Datastore, error)          // GET /admin/datastore
    GetDatastoreStatus(ctx, store) (*model.DatastoreStatus, error)
    ListBackups(ctx, store) ([]model.Backup, error)         // GET .../snapshot
    GetBackup(ctx, store, snapshot) (*model.Backup, error)
    GetBackupNotes(ctx, store, snapshot) (*model.BackupNotes, error)
    GetVerifyStatus(ctx, store, upid) (*model.VerifyStatus, error)
    GetPruneStatus(ctx, store, upid) (*model.PruneStatus, error)
    GetPBSVersion(ctx) (*model.PBSVersion, error)
}
```

> **Mutations are intentionally NOT on the RO interfaces yet.** The mutation
> methods (start/stop/create/migrate/delete/restore/forget/notes-set/verify-start/
> prune-start) will be added to these interfaces when the gate is implemented —
> that is an additive, non-breaking change, so the RO contract below stays stable.

### 2.5 Sentinel errors (`domain/port/errors.go`)
```go
var (
    ErrUnauthorized = errors.New("proxmox: unauthorized") // HTTP 401
    ErrForbidden    = errors.New("proxmox: forbidden")    // HTTP 403
    ErrNotFound     = errors.New("proxmox: not found")    // HTTP 404
)
```
Gateway-specific typed errors (`UpstreamError`, `APIError`) live in
`adapter/pve/errors.go` and `adapter/pbs/errors.go` (SPEC.md §7).

---

## 3. Application layer (`application/`)

Thin use-case services over the ports, plus the `App` facade:

```go
type App struct {
    PVE    *PVEService    // nil unless WithPVE applied
    PBS    *PBSService    // nil unless WithPBS applied
    System *SystemService // always present
    log    port.AppLogger
}

func New(log port.AppLogger, version string, opts ...Option) *App
func WithPVE(gw port.PVEGateway) Option  // wire only when PVE enabled
func WithPBS(gw port.PBSGateway) Option  // wire only when PBS enabled
```

- `PVEService` / `PBSService` expose one method per gateway method (thin
  delegation), so `adapter/mcp` talks to the application layer, and QA tests the
  services against port mocks.
- `SystemService` implements `Ping(ctx) map[string]bool` and
  `Status(ctx) model.ServerStatus` and reports which backends are enabled by
  checking `pve`/`pbs` nil-ness (set by `New` after options are applied).
- The system `status` tool reports the injected `version` and transport
  `"stdio"`.

**Optional enablement via optional injection** (SPEC.md §2.3): if a backend is
not configured, its `With*` option is simply not applied; `adapter/mcp`
registers that backend's tools only when its service is non-nil.

---

## 4. Configuration (`adapter/config`)

Environment variables via `kelseyhightower/envconfig` — **no YAML file**:

| Field | Env var | Notes |
|---|---|---|
| `PVEEndpoint` | `PVE_ENDPOINT` | enables PVE when non-empty **and** token set |
| `PVEToken` | `PVE_TOKEN` | API token; never logged |
| `PVECACertPath` | `PVE_CA_CERT_PATH` | optional custom CA (PEM) |
| `PBSEndpoint` | `PBS_ENDPOINT` | enables PBS when non-empty **and** token set |
| `PBSToken` | `PBS_TOKEN` | API token; never logged |
| `PBSCACertPath` | `PBS_CA_CERT_PATH` | optional custom CA (PEM) |
| `LogLevel` | `LOG_LEVEL` | trace\|debug\|info\|warn\|error; when set → file sink |

```go
func Load() (Config, error)                       // envconfig.Process("", &c)
func (c Config) PVEEnabled() bool                 // endpoint != "" && token != ""
func (c Config) PBSEnabled() bool
```

The composition root builds a `StaticTokenSource` per enabled backend and passes
it as the gateway's `port.TokenSource`.

---

## 5. Mutation gate + remote/OAuth2 seams

### 5.1 Mutation gate (TBD UX, clean seam)
All mutations are **explicitly opt-in** via a single boolean seam that threads
from the composition root into tool registration:

```go
// adapter/mcp
type Deps struct {
    // ...
    EnableMutations bool // gates ALL mutation tools
}
func RegisterTools(s *mcpSDK.Server, app *application.App, log toolLogger, enableMutations bool)
```

- `adapter/mcp/register.go` calls `registerPVETools(s, app, log, enableMutations)`
  and `registerPBSTools(...)`; each registers its **read-only tools always** and
  its **mutation tools only when `enableMutations` is true**.
- `cmd/mcp-proxmox/main.go` sets `enableMutations := false` for this milestone
  (read-only + system only). The final gating UX — env flag, config key, or CLI
  flag — is TBD; the seam is `Deps.EnableMutations`, and no other code changes
  when the UX is decided.
- `pve_vm_migrate` is flagged as the **next mutation** to add behind this gate.

### 5.2 Remote / OAuth2 seam
- **Token:** swap `StaticTokenSource` for an `OAuth2TokenSource`/header relay
  that returns `Bearer <token>`. Gateways consume only the header string.
- **Transport:** `cmd` picks the go-sdk transport (stdio today; Streamable HTTP
  later). `NewSessionTransport` wraps the stdio transport; a remote mode wires a
  different transport + HTTP middleware that populates `ctx` (via
  `port.WithToken`) instead of reading config.
- Neither change touches `domain` or `application`.

---

## 6. Logging (stdio-friendly)

All via logrus; core sees only `port.AppLogger`/`port.CtxLogger`.
`adapter/logging` provides:

- `NewLogger(level, format)` → `*logrus.Logger` (FullTimestamp text).
- `NewAppLogger(l)` → `port.AppLogger` (and `RequestAwareLogger`).
- `NewSlogLogger(l)` → `*slog.Logger` (bridges go-sdk internal logs).
- `WithSession(ctx, l)` → entry tagged with `session_id`/`request_id`.
- **File sink:** when `LOG_LEVEL` is set, `cmd` writes to
  `/tmp/mcp-proxmox.log` (`0600`). One `session_id` per MCP session is shared
  between the transport connection and the request context
  (`adapter/mcp/session_transport.go` + `Deps.SessionID`).
- **Secrets never logged:** tokens only as header name at trace, never value;
  `ca_cert_path` content never logged; trace logs method + URL path only.

---

## 7. Tool registry (read-only + system for this milestone)

Registration helpers in `adapter/mcp`:

- `roTool[In, Out](s, name, desc, log, fn)` — wraps `mcpSDK.AddTool` so every
  read-only handler is uniform: `fn(ctx, in) (Out, error)`; on error it logs
  `"<name>: %v"` via `toolLogger` and returns the error (MCP `IsError`).
- `emptyIn` — input struct for no-parameter tools.
- Per-domain input structs with `json` + `jsonschema` tags.

**System (always):** `ping`, `status`.
**PVE (when enabled):** `pve_node_list`, `pve_node_status`, `pve_cluster_status`,
`pve_cluster_resources`, `pve_nextid`, `pve_vm_list`, `pve_vm_get`,
`pve_vm_status`, `pve_lxc_list`, `pve_lxc_get`, `pve_lxc_status`,
`pve_storage_list`, `pve_storage_get`, `pve_network_list`, `pve_task_list`,
`pve_task_status`, `pve_task_log`, `pve_version`.
**PBS (when enabled):** `pbs_datastore_list`, `pbs_datastore_status`,
`pbs_backup_list`, `pbs_backup_get`, `pbs_backup_notes_get`, `pbs_verify_status`,
`pbs_prune_status`, `pbs_version`.

The full 46-tool registry (including gated mutations) remains in SPEC.md §6.

---

## 8. What the developer owns vs. QA owns

### Developer
- `adapter/pve/gateway.go` `do()`: token header injection, URL building,
  TLS RootCAs from `CACertPath`, `httptest`-friendly `HTTPClient`, HTTP→error
  mapping, read-only retry policy (SPEC.md §7). Replace the `ErrNotImplemented`
  stubs.
- Same for `adapter/pbs/gateway.go`.
- Add the gated mutation methods when the gate is implemented.
- Harden model JSON tags against the real API fixtures.

### QA
- `domain/model` JSON serialization tests for every entity.
- `domain/port` tests: `WithToken`/`TokenFromContext`, `CtxLogger` fallback,
  error helpers.
- `application/*` tests through `PVEGateway`/`PBSGateway` mocks.
- `adapter/pve` & `adapter/pbs` `httptest` mocks of the real JSON APIs.
- `adapter/token`, `adapter/config`, `adapter/logging`, `adapter/mcp` tests
  (each tool success+error; assert `IsError`; assert tools absent when a
  backend is disabled; mutation tools absent when the gate is off).

Both are unblocked by the interfaces and layouts above; neither needs the other
to start.
