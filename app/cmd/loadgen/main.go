// loadgen sends steady open-loop POST /pay traffic and prints a summary every
// 10s. Open-loop (fixed arrival rate, not "wait for the previous response")
// matters: a closed-loop generator slows down when the service slows down and
// therefore hides exactly the latency problems we want to see.
package main

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

func envInt(k string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil && v > 0 {
		return v
	}
	return def
}

func main() {
	target := strings.TrimRight(func() string {
		if v := os.Getenv("TARGET"); v != "" {
			return v
		}
		return "http://localhost:8080"
	}(), "/")
	rps := envInt("RPS", 20)
	maxInflight := envInt("MAX_INFLIGHT", 256)
	var duration time.Duration
	if v := os.Getenv("DURATION"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			fmt.Fprintln(os.Stderr, "bad DURATION:", err)
			os.Exit(2)
		}
		duration = d
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, duration)
		defer cancel()
	}

	var sent, ok, srvErr, otherCode, netErr, dropped atomic.Int64
	client := &http.Client{Timeout: 3 * time.Second}
	sem := make(chan struct{}, maxInflight)

	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				fmt.Printf("loadgen target=%s rps=%d sent=%d ok=%d 5xx=%d other=%d neterr=%d dropped=%d\n",
					target, rps, sent.Load(), ok.Load(), srvErr.Load(), otherCode.Load(), netErr.Load(), dropped.Load())
			}
		}
	}()

	fmt.Printf("loadgen starting: target=%s rps=%d\n", target, rps)
	tick := time.NewTicker(time.Second / time.Duration(rps))
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("loadgen done: sent=%d ok=%d 5xx=%d other=%d neterr=%d dropped=%d\n",
				sent.Load(), ok.Load(), srvErr.Load(), otherCode.Load(), netErr.Load(), dropped.Load())
			return
		case <-tick.C:
			select {
			case sem <- struct{}{}:
			default:
				dropped.Add(1) // generator saturated; count it, never block the arrival clock
				continue
			}
			sent.Add(1)
			go func() {
				defer func() { <-sem }()
				body := fmt.Sprintf(`{"amount_cents":%d,"currency":"USD"}`, 100+rand.IntN(50000))
				resp, err := client.Post(target+"/pay", "application/json", strings.NewReader(body))
				if err != nil {
					netErr.Add(1)
					return
				}
				defer resp.Body.Close()
				_, _ = io.Copy(io.Discard, resp.Body)
				switch {
				case resp.StatusCode < 300:
					ok.Add(1)
				case resp.StatusCode >= 500:
					srvErr.Add(1)
				default:
					otherCode.Add(1)
				}
			}()
		}
	}
}
