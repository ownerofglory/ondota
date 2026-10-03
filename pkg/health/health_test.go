package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func serve(t *testing.T, r *Registry, path string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	r.Mount(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestLiveAlwaysOK(t *testing.T) {
	r := NewRegistry()
	r.Register("db", func(context.Context) error { return errors.New("down") })
	if rec := serve(t, r, "/livez"); rec.Code != http.StatusOK {
		t.Fatalf("/livez = %d, want 200", rec.Code)
	}
}

func TestReadyReportsFailedChecks(t *testing.T) {
	r := NewRegistry()
	r.Register("ok", func(context.Context) error { return nil })
	r.Register("db", func(context.Context) error { return errors.New("down") })
	rec := serve(t, r, "/readyz")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("/readyz = %d, want 503", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"failed":["db"]`) {
		t.Fatalf("body %q does not name the failed check", rec.Body.String())
	}
}

func TestStartupWaitsForMarkStarted(t *testing.T) {
	r := NewRegistry()
	if rec := serve(t, r, "/startupz"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("/startupz before start = %d, want 503", rec.Code)
	}
	r.MarkStarted()
	if rec := serve(t, r, "/startupz"); rec.Code != http.StatusOK {
		t.Fatalf("/startupz after start = %d, want 200", rec.Code)
	}
}
