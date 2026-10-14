// Command goproxy is the Node B Go module proxy for engineering capsules
// (docs/reports/capsule-go-module-proxy-design-2026-10-14.md). It listens
// on loopback; guests reach it through their tap's DNAT on port 8088, and
// capsules reach it only through the egress route goproxy.choir.internal.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/goproxy"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8088", "address to listen on")
	cacheDir := flag.String("cache-dir", "/var/lib/go-choir/goproxy", "cache directory")
	maxBytes := flag.Int64("max-bytes", goproxy.DefaultMaxBytes, "cache size cap in bytes; oldest entries are evicted past it (0: no cap)")
	warmGoSum := flag.String("warm-go-sum", "", "go.sum to pre-warm and verify on start (the deployed commit's)")
	upstream := flag.String("upstream", goproxy.DefaultUpstream, "GOPROXY upstream")
	sumdbUpstream := flag.String("sumdb-upstream", goproxy.DefaultSumDBUpstream, "checksum database upstream")
	flag.Parse()
	log.SetFlags(0)

	var sums map[string]string
	if *warmGoSum != "" {
		data, err := os.ReadFile(*warmGoSum)
		if err == nil {
			sums, err = goproxy.ParseGoSum(data)
		}
		if err != nil {
			// The proxy still serves; only the warm and the go.sum checks
			// are lost.
			log.Printf("goproxy: warm go.sum %s unusable: %v", *warmGoSum, err)
			sums = nil
		}
	}
	srv, err := goproxy.New(goproxy.Config{
		CacheDir:      *cacheDir,
		MaxBytes:      *maxBytes,
		Upstream:      *upstream,
		SumDBUpstream: *sumdbUpstream,
		KnownSums:     sums,
	})
	if err != nil {
		log.Fatal(err)
	}
	httpServer := &http.Server{
		Addr:              *listen,
		Handler:           srv,
		ReadHeaderTimeout: 30 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()

	if len(sums) > 0 {
		go func() {
			start := time.Now()
			rep := srv.Warm(ctx, sums, 4)
			line, _ := json.Marshal(struct {
				Event string `json:"event"`
				goproxy.WarmReport
				DurationMS int64 `json:"duration_ms"`
			}{"warm_done", rep, time.Since(start).Milliseconds()})
			log.Printf("goproxy: %s", line)
		}()
	}

	errc := make(chan error, 1)
	go func() { errc <- httpServer.ListenAndServe() }()
	log.Printf("goproxy: listening on %s, cache %s (%d bytes)", *listen, *cacheDir, srv.CacheBytes())
	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	case <-ctx.Done():
		// Drain in-flight downloads so a deploy restart cuts as few
		// capsule builds as it can.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}
}
