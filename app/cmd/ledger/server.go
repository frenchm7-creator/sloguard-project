package main

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	mrand "math/rand/v2"

	"sloguard/internal/auth"
	"sloguard/internal/fault"
	"sloguard/internal/metrics"
)

type config struct {
	listenAddr  string
	authToken   string
	enableFault bool
	payTimeout  time.Duration
}

// newMux wires routes. Design notes:
//   - /pay is the ONLY instrumented business route; the SLO is defined on it.
//   - /healthz is deliberately NOT instrumented: probe traffic would flatter
//     the availability ratio and hide real outages.
//   - /metrics and /admin/fault sit behind the same bearer token. The admin
//     route does not exist at all unless ENABLE_FAULT_INJECTION=true.
func newMux(cfg config, reg *metrics.Registry, faults *fault.State, log *slog.Logger) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/pay", reg.Instrument("/pay", payHandler(faults, cfg.payTimeout, log)))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.Handle("/metrics", auth.Bearer(cfg.authToken, reg.Handler()))
	if cfg.enableFault {
		mux.Handle("/admin/fault", auth.Bearer(cfg.authToken, faults.Handler()))
	}
	return mux
}

// baseLatency simulates a healthy downstream: ~5ms floor plus an exponential
// tail (mean 8ms), capped at 200ms. p99 lands near 45ms, well inside the
// 300ms latency SLO, so only injected faults can burn budget.
func baseLatency() time.Duration {
	d := 5*time.Millisecond + time.Duration(mrand.ExpFloat64()*float64(8*time.Millisecond))
	return min(d, 200*time.Millisecond)
}

func payHandler(f *fault.State, timeout time.Duration, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed) // 4xx: caller's fault, not budget burn
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		rate, extra := f.Get()
		t := time.NewTimer(baseLatency() + extra)
		defer t.Stop()
		select {
		case <-t.C:
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				log.Warn("pay timed out", "timeout", timeout.String())
				http.Error(w, "upstream timeout", http.StatusGatewayTimeout)
			} else {
				// Client hung up. 499 (nginx convention) keeps client aborts out of the 5xx error ratio.
				w.WriteHeader(499)
			}
			return
		}

		if mrand.Float64() < rate {
			log.Warn("injected failure", "route", "/pay", "error_rate", rate)
			http.Error(w, "payment processor unavailable", http.StatusServiceUnavailable)
			return
		}
		var id [8]byte
		_, _ = crand.Read(id[:])
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id": hex.EncodeToString(id[:]), "status": "authorized"})
	})
}
