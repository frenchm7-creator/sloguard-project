// alertsink is a stand-in for PagerDuty/Slack: it receives Alertmanager
// webhooks and prints one structured log line per alert, so `docker compose
// logs -f alertsink` shows exactly what a pager would have received.
package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type payload struct {
	Status string `json:"status"`
	Alerts []struct {
		Status      string            `json:"status"`
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
		StartsAt    time.Time         `json:"startsAt"`
	} `json:"alerts"`
}

func handler(log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var p payload
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&p); err != nil {
			http.Error(w, "bad payload", http.StatusBadRequest)
			return
		}
		for _, a := range p.Alerts {
			log.Info("ALERT",
				"status", a.Status,
				"alertname", a.Labels["alertname"],
				"severity", a.Labels["severity"],
				"slo", a.Labels["slo"],
				"summary", a.Annotations["summary"],
				"since", a.StartsAt.Format(time.RFC3339))
		}
		w.WriteHeader(http.StatusOK)
	})
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	mux := http.NewServeMux()
	mux.Handle("/webhook", handler(log))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	addr := ":9095"
	log.Info("alertsink listening", "addr", addr)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		log.Error("alertsink failed", "err", err)
		os.Exit(1)
	}
}
