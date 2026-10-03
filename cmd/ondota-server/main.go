// Command ondota-server runs the ondOTA backend: the public listener (user API,
// enrollment, webhooks) and the mTLS-only device listener (ADR-0003).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/ownerofglory/ondota/config"
	inhttp "github.com/ownerofglory/ondota/internal/adapter/inbound/http"
	"github.com/ownerofglory/ondota/internal/adapter/outbound/postgres"
	"github.com/ownerofglory/ondota/internal/version"
	"github.com/ownerofglory/ondota/pkg/health"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)})))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	probes := health.NewRegistry()
	if cfg.DatabaseURL != "" {
		pool, err := postgres.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer pool.Close()
		probes.Register("postgres", postgres.Check(pool))
	} else {
		slog.Warn("DATABASE_URL not set; running without a database (development only)")
	}

	servers := []*http.Server{
		newServer(cfg.PublicAddr, inhttp.NewPublicRouter(probes)),
		newServer(cfg.DeviceAddr, inhttp.NewDeviceRouter(probes)),
	}
	probes.MarkStarted()
	slog.Info("ondota-server starting", "version", version.Version, "public", cfg.PublicAddr, "device", cfg.DeviceAddr)

	g, gctx := errgroup.WithContext(ctx)
	for _, srv := range servers {
		g.Go(func() error {
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		})
	}
	g.Go(func() error {
		<-gctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var errs []error
		for _, srv := range servers {
			errs = append(errs, srv.Shutdown(shutdownCtx))
		}
		return errors.Join(errs...)
	})
	return g.Wait()
}

// newServer returns an HTTP server with conservative timeouts.
func newServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

// parseLevel maps a validated LOG_LEVEL to a slog level.
func parseLevel(s string) slog.Level {
	var l slog.Level
	if err := l.UnmarshalText([]byte(strings.ToUpper(s))); err != nil {
		return slog.LevelInfo
	}
	return l
}
