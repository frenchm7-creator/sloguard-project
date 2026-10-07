package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sloguard/internal/fault"
	"sloguard/internal/metrics"
)

func newTestServer(t *testing.T, cfg config) (*httptest.Server, *fault.State) {
	t.Helper()
	f := &fault.State{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if cfg.payTimeout == 0 {
		cfg.payTimeout = 2 * time.Second
	}
	s := httptest.NewServer(newMux(cfg, metrics.New("test"), f, log))
	t.Cleanup(s.Close)
	return s, f
}

func post(t *testing.T, url string) int {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

func TestPayHealthyFailingAndWrongMethod(t *testing.T) {
	s, f := newTestServer(t, config{})
	if c := post(t, s.URL+"/pay"); c != 200 {
		t.Fatalf("healthy: %d", c)
	}
	_ = f.Set(1, 0)
	if c := post(t, s.URL+"/pay"); c != 503 {
		t.Fatalf("injected failure: %d", c)
	}
	resp, _ := http.Get(s.URL + "/pay")
	resp.Body.Close()
	if resp.StatusCode != 405 {
		t.Fatalf("GET /pay: %d", resp.StatusCode)
	}
}

func TestPayTimeoutIs504(t *testing.T) {
	s, f := newTestServer(t, config{payTimeout: 50 * time.Millisecond})
	_ = f.Set(0, 500*time.Millisecond)
	if c := post(t, s.URL+"/pay"); c != 504 {
		t.Fatalf("want 504 got %d", c)
	}
}

func TestHealthzIsNotInstrumented(t *testing.T) {
	s, _ := newTestServer(t, config{})
	for i := 0; i < 3; i++ {
		resp, _ := http.Get(s.URL + "/healthz")
		resp.Body.Close()
	}
	resp, _ := http.Get(s.URL + "/metrics")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if strings.Contains(string(body), `/healthz`) {
		t.Error("probe traffic must not appear in SLI metrics")
	}
}

func TestAdminRouteOnlyExistsWhenEnabledAndIsGuarded(t *testing.T) {
	off, _ := newTestServer(t, config{})
	resp, _ := http.Get(off.URL + "/admin/fault")
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("disabled: want 404 got %d", resp.StatusCode)
	}
	on, _ := newTestServer(t, config{enableFault: true, authToken: "tok"})
	resp, _ = http.Get(on.URL + "/admin/fault")
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("no token: want 401 got %d", resp.StatusCode)
	}
	req, _ := http.NewRequest("GET", on.URL+"/metrics", nil)
	req.Header.Set("Authorization", "Bearer tok")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("metrics with token: got %d", resp.StatusCode)
	}
}
