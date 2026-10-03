package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ownerofglory/ondota/pkg/health"
)

func TestRoutersExposeOnlyTheirSurface(t *testing.T) {
	h := health.NewRegistry()
	public, device := NewPublicRouter(h), NewDeviceRouter(h)

	tests := []struct {
		name    string
		handler http.Handler
		path    string
		want    int
	}{
		{"public livez", public, "/livez", http.StatusOK},
		{"public version", public, "/api/v1/version", http.StatusOK},
		{"public has no device api", public, "/device/v1/version", http.StatusNotFound},
		{"device livez", device, "/livez", http.StatusOK},
		{"device version", device, "/device/v1/version", http.StatusOK},
		{"device has no user api", device, "/api/v1/version", http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tt.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if rec.Code != tt.want {
				t.Fatalf("GET %s = %d, want %d", tt.path, rec.Code, tt.want)
			}
		})
	}
}
