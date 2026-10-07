// Command emulator is a hermetic, in-repo stand-in for the Proxmox VE (PVE)
// and Proxmox Backup Server (PBS) REST APIs, used by the e2e suite
// (e2e/e2e_test.go). It serves the real Proxmox JSON payload shapes under the
// `/api2/json` path prefix and enforces the real `Authorization` header
// (`PVEAPIToken=...` / `PBSAPIToken=...`), so an e2e round-trip exercises the
// full auth path end-to-end against the real `cmd/mcp-proxmox` server.
//
// It is intentionally stdlib-only (no external dependencies) so the package
// stays clean for depguard/golangci-lint even though it lives outside
// adapter/**.
//
// Run for PVE (default):   emulator -addr :8080
// Run PVE + PBS:           emulator -addr :8080 -pbs-addr :8081
package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	// pveAuthPrefix is the Authorization scheme the real PVE API token source
	// emits (adapter/token). PBS uses the PBSAPIToken scheme instead.
	pveAuthPrefix = "PVEAPIToken="
	pbsAuthPrefix = "PBSAPIToken="
)

func main() {
	addr := flag.String("addr", ":8080", "PVE emulator listen address")
	pbsAddr := flag.String("pbs-addr", "", "PBS emulator listen address (empty disables the PBS emulator)")
	flag.Parse()

	// Start the PBS emulator (if requested) in the background; the PVE server
	// blocks in main so the process stays alive as long as a listener runs.
	if *pbsAddr != "" {
		go serve(*pbsAddr, pbsHandler())
	}
	serve(*addr, pveHandler())
}

// serve runs h on addr. It uses an http.Server with a read-header timeout
// (gosec G114: bare http.ListenAndServe has no timeout support). A listener
// error is fatal — an emulator that cannot bind its port must fail the e2e.
func serve(addr string, h http.Handler) {
	srv := &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("emulator listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("emulator %s: %v", addr, err)
	}
}

// pveHandler returns the PVE mux. Paths map one-to-one to the endpoints the
// gateway (`adapter/pve`) calls, each wrapped in requireToken so an e2e asserts
// the Authorization header flows end-to-end.
func pveHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api2/json/version", requireToken(pveAuthPrefix, func(w http.ResponseWriter, _ *http.Request) {
		writeData(w, map[string]any{"version": "8.2.4", "release": "8"})
	}))

	mux.HandleFunc("/api2/json/nodes", requireToken(pveAuthPrefix, func(w http.ResponseWriter, _ *http.Request) {
		writeData(w, []map[string]any{{
			"node": "pve", "status": "online", "type": "node", "id": "node/pve",
		}})
	}))

	mux.HandleFunc("/api2/json/nodes/{node}/qemu", requireToken(pveAuthPrefix, func(w http.ResponseWriter, _ *http.Request) {
		writeData(w, []map[string]any{{
			"vmid": 100, "name": "vm100", "status": "running", "type": "qemu",
		}})
	}))

	return mux
}

// pbsHandler returns the PBS mux for the endpoints the PBS gateway
// (`adapter/pbs`) uses in a read-only round-trip.
func pbsHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api2/json/version", requireToken(pbsAuthPrefix, func(w http.ResponseWriter, _ *http.Request) {
		writeData(w, map[string]any{"version": "3.2.7", "release": "3"})
	}))

	mux.HandleFunc("/api2/json/admin/datastore", requireToken(pbsAuthPrefix, func(w http.ResponseWriter, _ *http.Request) {
		writeData(w, []string{"local", "backup"})
	}))

	return mux
}

// requireToken enforces the real Proxmox Authorization scheme. The header must
// carry the given prefix followed by a non-empty token value, mirroring how the
// real API treats a missing/invalid API token (401). This lets the e2e prove
// that the server's token source and gateway forward the credential.
func requireToken(prefix string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, prefix) || strings.TrimSpace(strings.TrimPrefix(auth, prefix)) == "" {
			writeError(w, http.StatusUnauthorized, "missing or invalid API token")
			return
		}
		next(w, r)
	}
}

// writeData writes the real Proxmox JSON envelope {"data": ...} that the
// gateways decode into their domain models.
func writeData(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

// writeError writes an error envelope with the given HTTP status.
func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"data": nil, "errors": msg})
}

// writeJSON encodes v as a JSON response with the given status.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
