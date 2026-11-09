package autoputer

// The early health gate of the store header/body split
// (docs/design/store-header-body-split-2026-11-01.md section 6). The split
// migrates the store before it opens, which on the paper is 15-45 minutes,
// and the host fails a guest that has not answered /health within
// BootReadyTimeout (20 s) or whose replay progress has not advanced for
// ReplayStallTimeout (120 s); the in-guest updater probe reads the same body.
// So before migrating, the guest binds its listen address with this gate. It
// answers 503 {"status":"replaying","sequence":0,"committed_sequence":0,
// "progress":N} with N the rows copied plus one tick per splitGateTickEvery,
// so a long single statement (an index build) still advances. The main server
// takes the gate's socket over at Start (server.SetListenerSource), so the
// address is never unbound between the two. A refused split is served by the
// main server's refusal path as the typed refusal "store_header_body_split".

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// BootRefusalKindStoreHeaderBodySplit is the typed refusal of a split that
// refused (too little disk, attempts exhausted) or failed.
const BootRefusalKindStoreHeaderBodySplit = store.HeaderBodySplitRefusalKind

// splitGateTickEvery paces the liveness tick (a var for tests).
var splitGateTickEvery = 5 * time.Second

// splitGate holds the guest listen address while the split runs.
type splitGate struct {
	tcp  *net.TCPListener
	srv  *http.Server
	rows atomic.Uint64
	tick atomic.Uint64

	handedOff atomic.Bool
	served    chan struct{}
	stopTick  chan struct{}
	stopOnce  sync.Once
	// tickMu orders a tick against the stop: no tick lands after stopTicking
	// returns (a select with both channels ready picks either).
	tickMu  sync.Mutex
	stopped bool
}

// startSplitGate binds addr and serves the gate on it.
func startSplitGate(addr string) (*splitGate, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	tcp, ok := ln.(*net.TCPListener)
	if !ok {
		_ = ln.Close()
		return nil, fmt.Errorf("split gate: %s is not a TCP listener", addr)
	}
	g := &splitGate{tcp: tcp, served: make(chan struct{}), stopTick: make(chan struct{})}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", g.serveHealth)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeSplitGateJSON(w, map[string]any{"status": "migrating", "phase": "store_header_body_split"})
	})
	g.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	// One response per connection: nothing the gate served outlives it.
	g.srv.SetKeepAlivesEnabled(false)
	go func() {
		defer close(g.served)
		_ = g.srv.Serve(&splitGateListener{g: g})
	}()
	go g.ticker()
	log.Printf("autoputer: store split gate listening on %s", tcp.Addr())
	return g, nil
}

func (g *splitGate) ticker() {
	t := time.NewTicker(splitGateTickEvery)
	defer t.Stop()
	for {
		select {
		case <-g.stopTick:
			return
		case <-t.C:
			g.tickMu.Lock()
			if !g.stopped {
				g.tick.Add(1)
			}
			g.tickMu.Unlock()
		}
	}
}

// setRows records the rows the migration copied so far (monotone).
func (g *splitGate) setRows(n uint64) {
	for {
		cur := g.rows.Load()
		if n <= cur || g.rows.CompareAndSwap(cur, n) {
			return
		}
	}
}

// progress is the liveness counter the host and the updater read.
func (g *splitGate) progress() uint64 { return g.rows.Load() + g.tick.Load() }

// stopTicking ends the liveness tick once the migration is over: the steps
// after it are bounded by the host's stall window as before.
func (g *splitGate) stopTicking() {
	g.stopOnce.Do(func() {
		g.tickMu.Lock()
		g.stopped = true
		g.tickMu.Unlock()
		close(g.stopTick)
	})
}

func (g *splitGate) serveHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeSplitGateJSON(w, map[string]any{
		"status":             "replaying",
		"sequence":           0,
		"committed_sequence": 0,
		"progress":           g.progress(),
		"phase":              "store_header_body_split",
	})
}

func writeSplitGateJSON(w http.ResponseWriter, body map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(body)
}

// handoff stops the gate without closing its socket and returns the socket
// for the main server: connections the kernel queued meanwhile are accepted
// by the main server. It is the server's listener source, called once.
func (g *splitGate) handoff() (net.Listener, error) {
	g.stopTicking()
	if g.handedOff.Swap(true) {
		return nil, errors.New("split gate: already handed off")
	}
	// Wake the gate's Accept; the wrapper then ends the gate's Serve.
	_ = g.tcp.SetDeadline(time.Now())
	<-g.served
	if err := g.tcp.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = g.srv.Shutdown(ctx) // in-flight gate responses finish; the socket stays open
	log.Printf("autoputer: store split gate handed %s to the server", g.tcp.Addr())
	return g.tcp, nil
}

// close releases the socket (a gate that is not handed off).
func (g *splitGate) close() {
	g.stopTicking()
	if !g.handedOff.Swap(true) {
		_ = g.tcp.SetDeadline(time.Now())
		<-g.served
		_ = g.tcp.Close()
	}
}

// splitGateListener is the gate's view of the socket: Close does not close it,
// and Accept ends once the gate is handed off.
type splitGateListener struct{ g *splitGate }

func (l *splitGateListener) Accept() (net.Conn, error) {
	for {
		if l.g.handedOff.Load() {
			return nil, net.ErrClosed
		}
		conn, err := l.g.tcp.Accept()
		if err == nil {
			return conn, nil
		}
		if l.g.handedOff.Load() {
			return nil, net.ErrClosed
		}
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			continue
		}
		return nil, err
	}
}

func (l *splitGateListener) Close() error   { return nil }
func (l *splitGateListener) Addr() net.Addr { return l.g.tcp.Addr() }

// runStoreSplitUnderGate migrates the store to the header/body split layout
// when it needs it, holding the guest listen address with the gate while it
// runs. It reports the refusal to serve (nil on success or when there was
// nothing to do); either way the server takes the gate's socket at Start.
func runStoreSplitUnderGate(s *server.Server, computerID, storePath string) *ProjectionBaseRefusal {
	needed, err := store.HeaderBodySplitNeeded(storePath)
	if err != nil {
		log.Printf("autoputer: store split: detect: %v (the store open runs the migration)", err)
		return nil
	}
	if !needed {
		return nil
	}
	gate, err := startSplitGate(s.ListenAddress())
	if err != nil {
		// No gate: the migration still runs inside the store open.
		log.Printf("autoputer: store split gate: %v", err)
		return nil
	}
	s.SetListenerSource(gate.handoff)
	err = store.MigrateHeaderBodySplit(storePath, gate.setRows)
	gate.stopTicking()
	if err == nil {
		return nil
	}
	return &ProjectionBaseRefusal{ComputerID: computerID, Kind: BootRefusalKindStoreHeaderBodySplit, Reason: err.Error()}
}

// splitFinalizeHandler is POST /internal/store/split-finalize (host peer
// only): the ring tooling removes the split's rollback copy once the
// differential is green.
func splitFinalizeHandler(storePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !server.HostPeerCaller(r) {
			http.Error(w, "host peer required", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		freed, err := store.FinalizeHeaderBodySplit(storePath)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"removed": freed > 0, "freed_bytes": freed})
	}
}
