package main

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebhookLogsEachAlert(t *testing.T) {
	var buf bytes.Buffer
	h := handler(slog.New(slog.NewJSONHandler(&buf, nil)))
	body := `{"status":"firing","alerts":[{"status":"firing","labels":{"alertname":"SLOBurnFast","severity":"page","slo":"ledger-availability"},"annotations":{"summary":"burning"},"startsAt":"2026-01-01T00:00:00Z"}]}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body)))
	if rec.Code != 200 || !strings.Contains(buf.String(), "SLOBurnFast") {
		t.Fatalf("code=%d log=%s", rec.Code, buf.String())
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader("not json")))
	if rec.Code != 400 {
		t.Errorf("want 400 got %d", rec.Code)
	}
}
