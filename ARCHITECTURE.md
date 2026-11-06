# mcp-proxmox — Application-Level Architecture Blueprint

**Language:** English.
**Authoritative source:** [`SPEC.md`](./SPEC.md). This document is the
application-level blueprint that turns the spec into a concrete, stable contract
so a developer and a QA engineer can implement and test in parallel without
re-deriving the design. Where the locked grill decisions refine `SPEC.md`, the
grill decisions win (and `SPEC.md` has been updated to match).

> **Scope.** This blueprint fixes the full tool surface: the **read-only (query)
> surface + system tools (`ping`/`status`)** are always registered, while the
> **mutation surface** (start/stop/create/delete/restore/prune/gc/verify/… ) is
> implemented on the gateway interfaces and registered only when the
> `ENABLE_MUTATIONS` gate is on (SPEC.md §2.5; default off). A developer
> implements the tools and gateway bodies; QA writes the hermetic tests. Both
> work against the interfaces and layouts below.

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
│   ├── pve_service.go           # PVE use cases (read + mutation)
│   ├── pbs_service.go           # PBS use cases (read + mutation)
│   └── system_service.go        # ping, status
├── adapter/
│   ├── config/                  # envconfig loader + Config struct
│   ├── pve/                     # Proxmox VE HTTP client → port.PVEGateway
│   ├── pbs/                     # Proxmox Backup Server HTTP client → port.PBSGateway
│   ├── token/                   # StaticTokenSource → port.TokenSource
│   ├── logging/                 # logrus setup, AppLogger, slog→logrus, WithSession
│   └── mcp/                     # go-sdk server assembly + tool registry
├── go.mod / go.sum              # module github.com/teran/mcp-proxmox, go 1.27.3
├── .golangci.yml, .go-arch-lint.yml, gremlins.toml,
├── Dockerfile, .goreleaser.yml, .dockerignore, .gitignore,
├── .github/workflows/ci.yml, .github/workflows/release.yml
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

- `domain/*`, `application/*` never import go-sdk, low-level `net/http`, logrus,
  envconfig, or any Proxmox client.
- Outbound HTTP is done exclusively via `resty.dev/v3` confined to `adapter/pve` +
  `adapter/pbs` (never a bare `http.Client{}` / `http.NewRequestWithContext`).
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

### 2.3 `port.PVEGateway` (read-only + mutation surface)
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

    // Mutations (gated by EnableMutations, SPEC.md §2.5). These are never
    // retried. Lifecycle ops run as PVE tasks, so they return a Task whose UPID
    // can be polled with GetTaskStatus / GetTaskLog.
    CreateVM(ctx, node, req model.CreateVMRequest) (int, error)
    CreateLXC(ctx, node, req model.CreateLXCRequest) (int, error)
    DownloadURL(ctx, node, storage, req model.StorageDownloadURLRequest) (string, error)
    ResizeVM(ctx, node, vmid, req model.ResizeVMRequest) error
    MigrateVM(ctx, node, vmid, req model.MigrateVMRequest) error
    AddHAResource(ctx, req model.HAResourceRequest) error
    RemoveHAResource(ctx, sid) error
    StartVMBackup(ctx, node, req model.VMBackupRequest) (*model.Task, error)
    CreateBackupJob(ctx, req model.CreateBackupJobRequest) (string, error)
    UpdateBackupJob(ctx, id, req model.CreateBackupJobRequest) (string, error)
    DeleteBackupJob(ctx, id) error
    StartVM(ctx, node, vmid) (*model.Task, error)
    StopVM(ctx, node, vmid, skiplock) (*model.Task, error)
    ShutdownVM(ctx, node, vmid, forceStop, timeout) (*model.Task, error)
    RebootVM(ctx, node, vmid, timeout) (*model.Task, error)
    ResetVM(ctx, node, vmid) (*model.Task, error)
    SuspendVM(ctx, node, vmid, todisk) (*model.Task, error)
    ResumeVM(ctx, node, vmid) (*model.Task, error)
    DeleteVM(ctx, node, vmid, purge, destroyUnreferencedDisks) (*model.Task, error)
    SetVMConfig(ctx, node, vmid, req model.SetVMConfigRequest) error
    SetLXCConfig(ctx, node, vmid, req model.SetLXCConfigRequest) error
    CloneVM(ctx, node, vmid, req model.CloneVMRequest) (*model.Task, error)
    CreateVMSnapshot(ctx, node, vmid, req model.SnapshotCreateRequest) (*model.Task, error)
    DeleteVMSnapshot(ctx, node, vmid, snapname) (*model.Task, error)
    RollbackVMSnapshot(ctx, node, vmid, snapname) (*model.Task, error)
    RestoreVM(ctx, node, req model.PVERestoreRequest) (*model.Task, error)
    StartLXC(ctx, node, vmid) (*model.Task, error)
    StopLXC(ctx, node, vmid, skiplock, forceStop) (*model.Task, error)
    ShutdownLXC(ctx, node, vmid, forceStop, timeout) (*model.Task, error)
    RebootLXC(ctx, node, vmid, timeout) (*model.Task, error)
    DeleteLXC(ctx, node, vmid, purge, destroyUnreferencedDisks, force) (*model.Task, error)
    CloneLXC(ctx, node, vmid, req model.CloneLXCRequest) (*model.Task, error)
    CreateLXCSnapshot(ctx, node, vmid, req model.SnapshotCreateRequest) (*model.Task, error)
    DeleteLXCSnapshot(ctx, node, vmid, snapname) (*model.Task, error)
    RollbackLXCSnapshot(ctx, node, vmid, snapname) (*model.Task, error)
}
```

### 2.4 `port.PBSGateway` (read-only + mutation surface)
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

    // Mutations (gated by EnableMutations, SPEC.md §2.5). These are never
    // retried. StartVerify/StartGC/StartPrune/StartSync return the task UPID
    // taken from the response `data` field; RestoreFile returns no UPID.
    StartVerify(ctx, store) (string, error)
    StartGC(ctx, store) (string, error)
    StartPrune(ctx, store) (string, error)
    StartSync(ctx, store, req model.PBSSyncRequest) (string, error)
    RestoreFile(ctx, store, backupType, backupID, backupTime, req model.PBSFileRestoreRequest) error
    RestoreVMBackup(ctx, store, backupType, backupID, backupTime, req model.PBSVMRestoreRequest) (string, error)
    StopTask(ctx, upid) error
    ForgetBackup(ctx, store, backupType, backupID, backupTime) error
}
```

> **Mutations are on the interfaces**, gated by `EnableMutations` (SPEC.md §2.5).
> They were added to these interfaces additively, so the read-only (RO) contract
> below stays stable; the mutation methods are registered as tools only when the
> `ENABLE_MUTATIONS` gate is on.

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
| `LogLevel` | `LOG_LEVEL` | trace\|debug\|info\|warn\|error; **when set → logging enabled** to `LOG_FILENAME`; unset → disabled |
| `LogFileName` | `LOG_FILENAME` | log file path (default `/tmp/mcp-proxmox.log`, `0600`) |
| `LogFormat` | `LOG_FORMAT` | `text` (default) \| `json` |

```go
func Load() (Config, error)                       // envconfig.Process("", &c)
func (c Config) PVEEnabled() bool                 // endpoint != "" && token != ""
func (c Config) PBSEnabled() bool
```

The composition root builds a `StaticTokenSource` per enabled backend and passes
it as the gateway's `port.TokenSource`.

---

## 5. Mutation gate + remote/OAuth2 seams

### 5.1 Mutation gate (`ENABLE_MUTATIONS`, clean seam)
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
- `cmd/mcp-proxmox/main.go` passes `cfg.EnableMutations` as
  `deps.EnableMutations` (SPEC.md §2.5).
- The gate is driven by the **`ENABLE_MUTATIONS`** env var (default `false`),
  read by envconfig into `Config.EnableMutations` (`adapter/config/config.go`)
  and threaded from the composition root into `adapter/mcp`.
- The mutation surface — VM/LXC lifecycle (start/stop/shutdown/reboot/reset/
  suspend/resume/delete/clone), snapshots, backup jobs, HA, and PBS verify/gc/
  prune/sync/restore/forget — is registered only when the gate is on.

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
- **Disabled by default (L2):** when `LOG_LEVEL` is unset the logger discards all
  output (`io.Discard`). When set, `cmd` writes to `LOG_FILENAME` (default
  `/tmp/mcp-proxmox.log`, `0600`) in `LOG_FORMAT` (text|json), and the **first**
  log line is the B5 banner: `Starting {appName}/{appVersion} (commit: {appCommit};
  built at {appTimestamp}) ...`. One `session_id` per MCP session is shared
  between the transport connection and the request context
  (`adapter/mcp/session_transport.go` + `Deps.SessionID`).
- **Per-request (L8):** an **`info` access log** for every incoming call (no
  debug/trace gate) carries tool, source=`STDIO`, duration_ms, outcome, and
  redacted args; byte sizes (in/out bytes) are confined to a **trace** line.
  Gateways forward `request_id` as `X-Request-ID` upstream.
- **Secrets never logged:** tokens only as header name at trace, never value;
  `ca_cert_path` content never logged; trace logs method + URL path only.

---

## 7. Tool registry (read-only always + mutations gated)

Registration helpers in `adapter/mcp`:

- `roTool[In, Out](s, name, desc, log, fn)` — wraps `mcpSDK.AddTool` so every
  read-only handler is uniform: `fn(ctx, in) (Out, error)`; on error it logs
  `"<name>: %v"` via `toolLogger` and returns the error (MCP `IsError`).
- `mutTool[In, Out](...)` — mirrors `roTool` for mutation handlers, registered
  only when `enableMutations` is true (SPEC.md §2.5).
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

The full tool registry (read-only tools always registered, mutations gated by
`ENABLE_MUTATIONS`) remains in SPEC.md §6. `RegisteredToolSurface(app,
enableMutations)` in `adapter/mcp/surface.go` mirrors the `tools/list` surface
exposed by the server.

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
