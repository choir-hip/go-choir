// Command pkgmirror is the Node B package mirror for engineering capsules
// (station FP10, docs/definitions/choir-launch-fp10-net02-mirrors-2026-10-21.md).
// It listens on loopback; guests reach it through their tap's DNAT on port
// 8089, and capsules reach it only through the egress route
// mirror.choir.internal. Every ecosystem is off unless -ecosystems names it.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/pkgmirror"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8089", "address to listen on")
	cacheDir := flag.String("cache-dir", "/var/lib/go-choir-pkgmirror", "cache directory")
	maxBytes := flag.Int64("max-bytes", pkgmirror.DefaultMaxBytes, "artifact cache size cap in bytes; oldest entries are evicted past it (negative: no cap)")
	publicBase := flag.String("public-base", "http://mirror.choir.internal", "base URL capsules use for this mirror (rewritten links name it)")
	ecosystems := flag.String("ecosystems", "", "comma-separated ecosystems to serve (npm, pypi, crates); empty serves none")
	flag.Parse()
	log.SetFlags(0)

	var enabled []string
	for _, e := range strings.Split(*ecosystems, ",") {
		if e = strings.TrimSpace(e); e != "" {
			enabled = append(enabled, e)
		}
	}
	srv, err := pkgmirror.New(pkgmirror.Config{
		CacheDir:   *cacheDir,
		MaxBytes:   *maxBytes,
		PublicBase: *publicBase,
		Ecosystems: enabled,
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

	errc := make(chan error, 1)
	go func() { errc <- httpServer.ListenAndServe() }()
	log.Printf("pkgmirror: listening on %s, ecosystems %v, cache %s (%d bytes)", *listen, srv.Ecosystems(), *cacheDir, srv.CacheBytes())
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
