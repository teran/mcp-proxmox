# Security Policy

This MCP server is developed and tested largely by AI agents with minimal human
review, so security is enforced automatically by CI (lint, coverage, mutation,
gosec, govulncheck, gitleaks). The security baseline is described in `SPEC.md`
(S01–S14) — the single source of truth for the security contracts this
repository must honour, including the domain-specific security audit in
`SPEC.md` §4.9.

## Reporting a Vulnerability

Please **do not** open a public issue for a security vulnerability.

Report vulnerabilities privately to the maintainers by opening a GitHub
[security advisory](https://github.com/teran/mcp-proxmox/security/advisories)
on this repository, or by emailing the maintainers directly (see the issue
tracker / repository metadata for contact details).

Please include:

- the affected version(s) and how you detected the issue;
- a minimal, reproducible description of the vulnerability;
- the potential impact and, if known, a suggested fix;
- the configuration context in which it occurs (endpoint/token/env vars).

## What happens next

1. The maintainers acknowledge the report within 5 business days.
2. The issue is triaged: severity, affected versions, and a fix plan.
3. A fix is developed **test-first** and lands through the normal CI gates.
4. Once a fix is available, the vulnerability is disclosed and (if appropriate)
   a `v*` release is published.

We ask that you **do not** disclose the vulnerability publicly until a fix has
been released or the maintainers explicitly agree otherwise (responsible
disclosure).

## Supported Versions

| Version | Supported          |
|---------|--------------------|
| 0.x     | ✅ |

Only the latest release line is supported. Older versions are not patched;
upgrade to a supported version.

## Security Baseline

This is a **Local (stdio)** MCP server. Its security contracts (see `SPEC.md`
§4.8 / §4.9 / §8):

- **API tokens** are configured once per backend via environment variables
  (`PVE_TOKEN` / `PBS_TOKEN`) and are **never** per-call tool parameters and
  **never** logged or exposed in tool output (S02/L05).
- **TLS verification is always on.** Outbound HTTPS uses system roots; custom
  CAs are supported only via a per-backend `ca_cert_path` that builds the
  client TLS `RootCAs` — there is **no** `insecure_skip_verify` switch (S01).
- **No user-supplied local filesystem access.** No tool argument names a local
  path the server accesses; the only file I/O is reading the configured
  `ca_cert_path` and (when `LOG_LEVEL` is set) writing to `LOG_FILENAME`
  (mode `0600`) (S04).
- **Read-only by default.** Read/query tools are always registered; **mutation
  tools are registered only when `ENABLE_MUTATIONS=true`** (default off, §2.5).
- **Input/output validation.** Every tool validates its `inputSchema`
  (S08) and its result against an `outputSchema`, with ANSI/control stripped
  and sensitive keys redacted (S09/S02).
- **Destructive tools** require human confirmation (HITL, S12).

See `SPEC.md` for the full domain-specific security audit (S14).
