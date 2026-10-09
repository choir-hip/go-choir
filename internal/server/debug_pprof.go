// Go runtime profiles (net/http/pprof) for host-sourced callers on every
// guest and host service. Latency claims about a running computer
// should come from a profile, not inference (2026-10-09: a 0.15 s Texture list
// went to 1-2.7 s after idle and only a mutex/block profile can say whether
// reads were queuing behind the shared store engine mutex).

package server

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"runtime"
	rpprof "runtime/pprof"
	"strings"
	"sync"
)

// DebugPprofPrefix is the route prefix. The public edge refuses /internal/*
// (Caddy), and the proxy never forwards /internal/ to guests.
const DebugPprofPrefix = "/internal/debug/pprof/"

// EnableContentionProfiles turns on sampled mutex and block profiling: one in
// ten mutex contention events, and blocking events of 1 ms or longer. Both are
// cheap at these rates and are what diagnose lock-convoy latency.
func EnableContentionProfiles() {
	contentionOnce.Do(func() {
		runtime.SetMutexProfileFraction(10)
		runtime.SetBlockProfileRate(1_000_000)
	})
}

var contentionOnce sync.Once

func serveDebugPprof(w http.ResponseWriter, r *http.Request) {
	if !HostSourcedCaller(r) {
		http.Error(w, "profiles are served to host-sourced callers only", http.StatusForbidden)
		return
	}
	name := strings.Trim(strings.TrimPrefix(r.URL.Path, DebugPprofPrefix), "/")
	switch name {
	case "":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, "profiles: profile?seconds=N (cpu), trace?seconds=N, cmdline, symbol")
		for _, p := range rpprof.Profiles() {
			fmt.Fprintf(w, "%s (%d)\n", p.Name(), p.Count())
		}
	case "profile":
		pprof.Profile(w, r)
	case "trace":
		pprof.Trace(w, r)
	case "cmdline":
		pprof.Cmdline(w, r)
	case "symbol":
		pprof.Symbol(w, r)
	default:
		if rpprof.Lookup(name) == nil {
			http.NotFound(w, r)
			return
		}
		pprof.Handler(name).ServeHTTP(w, r)
	}
}
