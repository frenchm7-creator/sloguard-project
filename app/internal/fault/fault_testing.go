package fault

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSetValidation(t *testing.T) {
	var s State
	for _, rate := range []float64{-0.1, 1.1} {
		if s.Set(rate, 0) == nil {
			t.Errorf("rate %v should be rejected", rate)
		}
	}
	if s.Set(0.5, time.Minute) == nil {
		t.Error("latency over MaxLatency should be rejected")
	}
	if err := s.Set(0.25, 300*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if r, l := s.Get(); r != 0.25 || l != 300*time.Millisecond {
		t.Errorf("got %v %v", r, l)
	}
}

func TestHandlerPartialUpdateAndReset(t *testing.T) {
	var s State
	h := s.Handler()
	do := func(url string) int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, url, nil))
		return rec.Code
	}
	if c := do("/admin/fault?error_rate=0.1"); c != 200 {
		t.Fatalf("status %d", c)
	}
	if c := do("/admin/fault?latency_ms=400"); c != 200 {
		t.Fatalf("status %d", c)
	}
	if r, l := s.Get(); r != 0.1 || l != 400*time.Millisecond {
		t.Errorf("partial update lost state: %v %v", r, l)
	}
	if c := do("/admin/fault?error_rate=abc"); c != 400 {
		t.Errorf("want 400 got %d", c)
	}
	do("/admin/fault?reset=true")
	if r, l := s.Get(); r != 0 || l != 0 {
		t.Errorf("reset failed: %v %v", r, l)
	}
}
