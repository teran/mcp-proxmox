# syntax=docker/dockerfile:1

# Runtime image for mcp-proxmox — consumes goreleaser artifacts.
# The binary is NOT built here. Build it first with goreleaser, e.g.:
#   GOOS=linux GOARCH=amd64 goreleaser build --snapshot --clean \
#     --single-target --id mcp-proxmox --output dist/mcp-proxmox
# then build the image from this Dockerfile (there is no `make image` target —
# the Makefile exposes build/test/lint/mutation/secrets/e2e; see AGENTS.md).
#
# Produces a minimal `FROM scratch` runtime image carrying only the single
# static binary plus the CA certificate bundle (required for TLS to the
# Proxmox VE / PBS APIs). No shell, no package manager, no distroless base.

# ---------- Certificates stage ----------
# CA certificate bundle from a lightweight image. Required because the gateway
# clients dial the Proxmox endpoints over HTTPS.
FROM alpine:3 AS certs

# ---------- Runtime stage ----------
FROM scratch

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# The single static binary built by goreleaser — nothing else is needed.
COPY dist/mcp-proxmox /usr/local/bin/mcp-proxmox

# Run as an unprivileged user. scratch has no /etc/passwd, so use the numeric
# UID/GID directly (65532 is the conventional non-root container uid).
USER 65532:65532

# stdio mode needs no port; credentials and endpoint are passed via environment
# variables (PVE_ENDPOINT/PVE_TOKEN, PBS_ENDPOINT/PBS_TOKEN, LOG_LEVEL).
ENTRYPOINT ["/usr/local/bin/mcp-proxmox"]
