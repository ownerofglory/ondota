// Package http wires the server's two inbound HTTP surfaces (ADR-0003):
// the public router (user API, enrollment, webhooks) and the device router
// (mTLS-only device API). Feature handlers are mounted here as they are added.
package http

import (
	"encoding/json"
	"net/http"

	"github.com/ownerofglory/ondota/internal/version"
	"github.com/ownerofglory/ondota/pkg/health"
)

// NewPublicRouter returns the handler for the public listener.
func NewPublicRouter(h *health.Registry) http.Handler {
	mux := http.NewServeMux()
	h.Mount(mux)
	mux.HandleFunc("GET /api/v1/version", versionHandler)
	return mux
}

// NewDeviceRouter returns the handler for the device listener. Only the Traefik
// router that enforces client certificates may reach this listener.
func NewDeviceRouter(h *health.Registry) http.Handler {
	mux := http.NewServeMux()
	h.Mount(mux)
	mux.HandleFunc("GET /device/v1/version", versionHandler)
	return mux
}

// versionHandler reports the running build version.
func versionHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{"version": version.Version})
}
