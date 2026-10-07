// Package metrics is a small, dependency-free implementation of the Prometheus
// text exposition format (v0.0.4): one labelled counter family, one labelled
// histogram family and a few gauges. It exists so the service ships with an
// empty dependency tree (see "Decision 3" in the README).
//
// Hot path (observe) is lock-free for existing series: atomic counters and a
// CAS loop for the float sum. A scrape racing with observations can see a sum
// that lags the buckets by a few samples; bucket counts and _count are always
// mutually consistent because both are derived from the same loads.
package metrics

import (
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultBuckets are in seconds. 0.3 is deliberately present: the latency SLO
// threshold in slo/slos.yaml must be one of these (tools/gen_rules.py enforces it).
var DefaultBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.2, 0.3, 0.5, 1, 2.5, 5}

type counterKey struct{ method, route, code string }

type histogram struct {
	bounds  []float64
	counts  []atomic.Uint64 // len(bounds)+1; the last slot is the +Inf overflow
	sumBits atomic.Uint64
}

func newHistogram(bounds []float64) *histogram {
	return &histogram{bounds: bounds, counts: make([]atomic.Uint64, len(bounds)+1)}
}

func (h *histogram) observe(v float64) {
	// First bucket whose upper bound is >= v (Prometheus "le" is inclusive).
	h.counts[sort.SearchFloat64s(h.bounds, v)].Add(1)
	for {
		old := h.sumBits.Load()
		if h.sumBits.CompareAndSwap(old, math.Float64bits(math.Float64frombits(old)+v)) {
			return
		}
	}
}

// Registry holds every series the service exposes.
type Registry struct {
	version  string
	started  time.Time
	bounds   []float64
	inflight atomic.Int64

	mu       sync.RWMutex
	counters map[counterKey]*atomic.Uint64
	hists    map[string]*histogram
}

func New(version string) *Registry { return NewWithBuckets(version, DefaultBuckets) }

func NewWithBuckets(version string, buckets []float64) *Registry {
	b := append([]float64(nil), buckets...)
	sort.Float64s(b)
	return &Registry{
		version:  version,
		started:  time.Now(),
		bounds:   b,
		counters: make(map[counterKey]*atomic.Uint64),
		hists:    make(map[string]*histogram),
	}
}

func (r *Registry) counter(k counterKey) *atomic.Uint64 {
	r.mu.RLock()
	c, ok := r.counters[k]
	r.mu.RUnlock()
	if ok {
		return c
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok = r.counters[k]; !ok {
		c = new(atomic.Uint64)
		r.counters[k] = c
	}
	return c
}

func (r *Registry) hist(route string) *histogram {
	r.mu.RLock()
	h, ok := r.hists[route]
	r.mu.RUnlock()
	if ok {
		return h
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok = r.hists[route]; !ok {
		h = newHistogram(r.bounds)
		r.hists[route] = h
	}
	return h
}

// normalizeMethod bounds label cardinality: the method is attacker-controlled.
func normalizeMethod(m string) string {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodOptions:
		return m
	}
	return "OTHER"
}

func (r *Registry) observe(method, route string, status int, seconds float64) {
	r.counter(counterKey{normalizeMethod(method), route, strconv.Itoa(status)}).Add(1)
	r.hist(route).observe(seconds)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wrote {
		s.status, s.wrote = code, true
	}
	s.ResponseWriter.WriteHeader(code)
}

// Instrument records request count and latency under a FIXED route label
// (never the raw URL path, which would let clients blow up cardinality).
// A panicking handler is counted as a 500 and the panic is re-raised.
func (r *Registry) Instrument(route string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		r.inflight.Add(1)
		defer r.inflight.Add(-1)
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if p := recover(); p != nil {
				r.observe(req.Method, route, http.StatusInternalServerError, time.Since(start).Seconds())
				panic(p)
			}
		}()
		next.ServeHTTP(rec, req)
		r.observe(req.Method, route, rec.status, time.Since(start).Seconds())
	})
}

// Handler serves the exposition format.
func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		var b strings.Builder
		r.Render(&b)
		_, _ = io.WriteString(w, b.String())
	})
}

func escape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(s)
}

func ff(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

// Render writes all series, sorted, so output is deterministic and diff-able.
func (r *Registry) Render(b *strings.Builder) {
	type cv struct {
		k counterKey
		c *atomic.Uint64
	}
	type hv struct {
		route string
		h     *histogram
	}
	r.mu.RLock()
	cs := make([]cv, 0, len(r.counters))
	for k, c := range r.counters {
		cs = append(cs, cv{k, c})
	}
	hs := make([]hv, 0, len(r.hists))
	for route, h := range r.hists {
		hs = append(hs, hv{route, h})
	}
	r.mu.RUnlock()

	sort.Slice(cs, func(i, j int) bool {
		a, c := cs[i].k, cs[j].k
		if a.route != c.route {
			return a.route < c.route
		}
		if a.method != c.method {
			return a.method < c.method
		}
		return a.code < c.code
	})
	sort.Slice(hs, func(i, j int) bool { return hs[i].route < hs[j].route })

	b.WriteString("# HELP http_requests_total Total HTTP requests by method, route and status code.\n")
	b.WriteString("# TYPE http_requests_total counter\n")
	for _, e := range cs {
		b.WriteString(`http_requests_total{method="` + escape(e.k.method) + `",route="` + escape(e.k.route) +
			`",code="` + e.k.code + `"} ` + strconv.FormatUint(e.c.Load(), 10) + "\n")
	}

	b.WriteString("# HELP http_request_duration_seconds HTTP request latency in seconds by route.\n")
	b.WriteString("# TYPE http_request_duration_seconds histogram\n")
	for _, e := range hs {
		route := escape(e.route)
		var cum uint64
		for i, bound := range e.h.bounds {
			cum += e.h.counts[i].Load()
			b.WriteString(`http_request_duration_seconds_bucket{route="` + route + `",le="` + ff(bound) + `"} ` +
				strconv.FormatUint(cum, 10) + "\n")
		}
		cum += e.h.counts[len(e.h.bounds)].Load()
		b.WriteString(`http_request_duration_seconds_bucket{route="` + route + `",le="+Inf"} ` + strconv.FormatUint(cum, 10) + "\n")
		b.WriteString(`http_request_duration_seconds_sum{route="` + route + `"} ` + ff(math.Float64frombits(e.h.sumBits.Load())) + "\n")
		b.WriteString(`http_request_duration_seconds_count{route="` + route + `"} ` + strconv.FormatUint(cum, 10) + "\n")
	}

	b.WriteString("# HELP http_requests_in_flight Requests currently being served.\n")
	b.WriteString("# TYPE http_requests_in_flight gauge\n")
	b.WriteString("http_requests_in_flight " + strconv.FormatInt(r.inflight.Load(), 10) + "\n")
	b.WriteString("# HELP process_start_time_seconds Start time of the process since unix epoch.\n")
	b.WriteString("# TYPE process_start_time_seconds gauge\n")
	b.WriteString("process_start_time_seconds " + strconv.FormatInt(r.started.Unix(), 10) + "\n")
	b.WriteString("# HELP ledger_build_info Build metadata; value is always 1.\n")
	b.WriteString("# TYPE ledger_build_info gauge\n")
	b.WriteString(`ledger_build_info{version="` + escape(r.version) + `"} 1` + "\n")
}
