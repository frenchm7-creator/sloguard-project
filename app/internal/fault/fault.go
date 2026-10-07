// Package fault holds runtime-adjustable failure injection (error rate and
// added latency). It is what lets us *prove* the SLO alerts fire.
package fault

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

const MaxLatency = 10 * time.Second

type State struct {
	rateBits  atomic.Uint64
	latencyNs atomic.Int64
}

func (s *State) Set(rate float64, latency time.Duration) error {
	if math.IsNaN(rate) || rate < 0 || rate > 1 {
		return fmt.Errorf("error_rate must be within [0,1], got %v", rate)
	}
	if latency < 0 || latency > MaxLatency {
		return fmt.Errorf("latency_ms must be within [0,%d]", MaxLatency.Milliseconds())
	}
	s.rateBits.Store(math.Float64bits(rate))
	s.latencyNs.Store(int64(latency))
	return nil
}

func (s *State) Get() (float64, time.Duration) {
	return math.Float64frombits(s.rateBits.Load()), time.Duration(s.latencyNs.Load())
}

// Handler: GET shows state. POST updates it; omitted params keep their value,
// reset=true zeroes everything first.
func (s *State) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
		case http.MethodPost:
			rate, lat := s.Get()
			q := r.URL.Query()
			if q.Get("reset") == "true" {
				rate, lat = 0, 0
			}
			if v := q.Get("error_rate"); v != "" {
				f, err := strconv.ParseFloat(v, 64)
				if err != nil {
					http.Error(w, "invalid error_rate", http.StatusBadRequest)
					return
				}
				rate = f
			}
			if v := q.Get("latency_ms"); v != "" {
				ms, err := strconv.Atoi(v)
				if err != nil || ms < 0 || ms > int(MaxLatency.Milliseconds()) {
					http.Error(w, "invalid latency_ms", http.StatusBadRequest)
					return
				}
				lat = time.Duration(ms) * time.Millisecond
			}
			if err := s.Set(rate, lat); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		rate, lat := s.Get()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"error_rate": rate, "latency_ms": lat.Milliseconds()})
	})
}
