package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func render(r *Registry) string {
	var b strings.Builder
	r.Render(&b)
	return b.String()
}

func TestHistogramIsCumulativeAndInclusive(t *testing.T) {
	r := NewWithBuckets("t", []float64{0.1, 0.3})
	r.observe("POST", "/pay", 200, 0.05) // <= 0.1
	r.observe("POST", "/pay", 200, 0.3)  // exactly on 0.3 -> le is inclusive
	r.observe("POST", "/pay", 200, 9)    // overflow, only in +Inf
	out := render(r)
	for _, want := range []string{
		`http_request_duration_seconds_bucket{route="/pay",le="0.1"} 1`,
		`http_request_duration_seconds_bucket{route="/pay",le="0.3"} 2`,
		`http_request_duration_seconds_bucket{route="/pay",le="+Inf"} 3`,
		`http_request_duration_seconds_count{route="/pay"} 3`,
		`http_requests_total{method="POST",route="/pay",code="200"} 3`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing line %q in:\n%s", want, out)
		}
	}
}

func TestInstrumentRecordsStatusAndBoundsMethodCardinality(t *testing.T) {
	r := New("t")
	h := r.Instrument("/pay", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	for _, m := range []string{"POST", "WEIRDVERB1", "WEIRDVERB2"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(m, "/pay?x=1", nil))
	}
	out := render(r)
	if !strings.Contains(out, `http_requests_total{method="OTHER",route="/pay",code="503"} 2`) {
		t.Errorf("unknown methods must collapse into OTHER:\n%s", out)
	}
	if strings.Contains(out, "x=1") {
		t.Error("raw URL must never leak into labels")
	}
}

func TestPanicIsCountedAs500(t *testing.T) {
	r := New("t")
	h := r.Instrument("/pay", http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	func() {
		defer func() { _ = recover() }()
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/pay", nil))
	}()
	if !strings.Contains(render(r), `code="500"} 1`) {
		t.Error("panic should be recorded as a 500")
	}
}
