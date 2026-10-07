// ledger is a tiny payments-shaped HTTP service whose only job is to be
// observable, SLO-measurable and breakable on demand.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sloguard/internal/fault"
	"sloguard/internal/metrics"
)

var version = "dev" // set via -ldflags "-X main.version=..."

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// probe backs `ledger -healthcheck`: the distroless image has no shell or curl,
// so the binary checks itself (used by docker compose healthcheck).
func probe(addr string) int {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 1
	}
	c := http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe /healthz on localhost and exit 0/1")
	flag.Parse()

	cfg := config{
		listenAddr:  getenv("LISTEN_ADDR", ":8080"),
		authToken:   os.Getenv("AUTH_TOKEN"),
		enableFault: os.Getenv("ENABLE_FAULT_INJECTION") == "true",
		payTimeout:  2 * time.Second,
	}
	if *healthcheck {
		os.Exit(probe(cfg.listenAddr))
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if cfg.enableFault && cfg.authToken == "" {
		log.Warn("fault injection enabled WITHOUT an auth token - local development only")
	}

	srv := &http.Server{
		Addr:              cfg.listenAddr,
		Handler:           newMux(cfg, metrics.New(version), &fault.State{}, log),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	log.Info("ledger listening", "addr", cfg.listenAddr, "version", version, "fault_injection", cfg.enableFault)

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Error("server failed", "err", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		log.Info("shutting down, draining in-flight requests")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error("graceful shutdown failed", "err", err)
		}
	}
}
