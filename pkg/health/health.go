// Package health provides dependency-aware HTTP probes for long-running services.
package health

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sort"
	"sync"
)

// Check verifies whether a required runtime component is usable.
type Check func(context.Context) error

// Registry holds the readiness checks registered by outbound adapters and workers.
type Registry struct {
	mu      sync.RWMutex
	checks  map[string]Check
	started bool
}

// NewRegistry creates an empty registry. Call MarkStarted once initialization finishes.
func NewRegistry() *Registry {
	return &Registry{checks: make(map[string]Check)}
}

// Register adds a named required component check.
func (r *Registry) Register(name string, check Check) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checks[name] = check
}

// MarkStarted records that application initialization completed successfully.
func (r *Registry) MarkStarted() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.started = true
}

// Live responds once the process can serve HTTP.
func (r *Registry) Live(w http.ResponseWriter, _ *http.Request) {
	writeStatus(w, http.StatusOK, "ok", nil)
}

// Ready verifies every registered dependency.
func (r *Registry) Ready(w http.ResponseWriter, req *http.Request) {
	r.mu.RLock()
	checks := make(map[string]Check, len(r.checks))
	for name, check := range r.checks {
		checks[name] = check
	}
	r.mu.RUnlock()

	var failed []string
	for name, check := range checks {
		if err := check(req.Context()); err != nil {
			slog.Warn("readiness check failed", "check", name, "error", err)
			failed = append(failed, name)
		}
	}
	if len(failed) > 0 {
		sort.Strings(failed)
		writeStatus(w, http.StatusServiceUnavailable, "unavailable", failed)
		return
	}
	writeStatus(w, http.StatusOK, "ok", nil)
}

// Startup reports unavailable until MarkStarted, then behaves like Ready.
func (r *Registry) Startup(w http.ResponseWriter, req *http.Request) {
	r.mu.RLock()
	started := r.started
	r.mu.RUnlock()
	if !started {
		writeStatus(w, http.StatusServiceUnavailable, "starting", nil)
		return
	}
	r.Ready(w, req)
}

// Mount registers /livez, /readyz and /startupz on mux.
func (r *Registry) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /livez", r.Live)
	mux.HandleFunc("GET /readyz", r.Ready)
	mux.HandleFunc("GET /startupz", r.Startup)
}

func writeStatus(w http.ResponseWriter, status int, value string, failed []string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"status": value, "failed": failed})
}
