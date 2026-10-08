package autoputer

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/server"
)

// bootRefusalObservationWindow bounds how long a refused guest keeps its
// /health listener up for the host probe. The host records the typed refusal
// within seconds and kills the VM; the window is a backstop so a refused guest
// can never linger as a zombie.
const bootRefusalObservationWindow = 5 * time.Minute

// bootRefusalForError preserves a typed planner refusal and classifies any
// other materialization failure as recovery_unavailable: still a typed
// refusal for prompt host propagation, but not a durable deterministic one.
func bootRefusalForError(computerID string, err error) *ProjectionBaseRefusal {
	var refusal *ProjectionBaseRefusal
	if errors.As(err, &refusal) {
		return refusal
	}
	return &ProjectionBaseRefusal{
		ComputerID: computerID,
		Kind:       BootRefusalKindRecoveryUnavailable,
		Reason:     err.Error(),
	}
}

// writeBootRefusalHealth serves the typed refusal body the host parses. It is
// the only surface a refused guest exposes: no product traffic, no runtime.
func writeBootRefusalHealth(w http.ResponseWriter, refusal *ProjectionBaseRefusal) {
	if refusal == nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":             "refused",
		"kind":               refusal.Kind,
		"reason":             refusal.Reason,
		"computer_id":        refusal.ComputerID,
		"local_sequence":     refusal.LocalSequence,
		"watermark_sequence": refusal.WatermarkSequence,
		"target_sequence":    refusal.TargetSequence,
		"empty_store":        refusal.EmptyStore,
		"chain_exists":       refusal.ChainExists,
	})
}

// serveBootRefusalAndExit publishes the refusal on /health, waits for the
// bounded observation window, then exits. The caller must not have started the
// runtime or appender: an unrecovered store never serves product traffic.
func serveBootRefusalAndExit(s *server.Server, refusal *ProjectionBaseRefusal) {
	s.SetHealthHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeBootRefusalHealth(w, refusal)
	})
	go s.Start()
	time.Sleep(bootRefusalObservationWindow)
	log.Printf("autoputer: boot refusal observation window elapsed; exiting refused guest")
	os.Exit(1)
}

// startupFailer ends a boot that can no longer reach readiness. Every fatal
// startup error takes this path instead of log.Fatalf, so the host records a
// typed reason within one health probe rather than waiting out its readiness
// deadline while systemd crash-loops the runtime (O9/O20;
// docs/problems/fresh-realization-missing-privacy-key-blind-boot-2026-10-08.md).
type startupFailer struct {
	server     *server.Server
	computerID string
	window     time.Duration
	exit       func(int)

	mu   sync.Mutex
	gate *replayHealthGate
}

func newStartupFailer(s *server.Server, computerID string) *startupFailer {
	window := bootRefusalObservationWindow
	if strings.TrimSpace(os.Getenv("RUNTIME_RECOVERY_REPLAY_ONLY")) == "1" {
		// The B14 host drive waits on process exit, not /health: fail fast.
		window = 0
	}
	return &startupFailer{server: s, computerID: computerID, window: window, exit: os.Exit}
}

// setGate records that the server is running with the replay gate as its
// /health handler; later refusals are published through the gate.
func (f *startupFailer) setGate(gate *replayHealthGate) {
	f.mu.Lock()
	f.gate = gate
	f.mu.Unlock()
}

// fatalf is the log.Fatalf replacement for startup: it logs, then refuses
// with kind startup_failed and the formatted message as the reason.
func (f *startupFailer) fatalf(format string, args ...any) {
	reason := fmt.Sprintf(format, args...)
	log.Print(reason)
	f.refuse(&ProjectionBaseRefusal{ComputerID: f.computerID, Kind: BootRefusalKindStartupFailed, Reason: reason})
}

// refuse publishes the typed refusal on /health for the observation window,
// then exits. Before the server runs it starts a refusal-only server.
func (f *startupFailer) refuse(refusal *ProjectionBaseRefusal) {
	f.mu.Lock()
	gate := f.gate
	f.mu.Unlock()
	if gate == nil {
		if f.window == 0 {
			f.exit(1)
			return
		}
		serveBootRefusalAndExit(f.server, refusal)
		return
	}
	readinessServed := !gate.isPending()
	gate.refuse(refusal)
	if readinessServed {
		// The host stopped its boot wait once readiness was served; nobody
		// reads the refusal, so restart now as before.
		log.Printf("autoputer: startup refused (%s) after readiness; exiting", refusal.Kind)
		f.exit(1)
		return
	}
	log.Printf("autoputer: startup refused (%s); serving typed refusal on /health for %s", refusal.Kind, f.window)
	time.Sleep(f.window)
	f.exit(1)
}

// privacyKeyBootRefusal classifies a privacy key load failure. A missing key
// over an existing chain is the typed privacy_key_unavailable refusal: the
// key may be created only before genesis, so this realization can never
// succeed without a key source.
func privacyKeyBootRefusal(computerID string, headSequence uint64, err error) *ProjectionBaseRefusal {
	reason := fmt.Sprintf("autoputer: configure guest-owned private artifact cipher: %v", err)
	if headSequence > 0 && errors.Is(err, os.ErrNotExist) {
		return &ProjectionBaseRefusal{
			ComputerID:     computerID,
			Kind:           BootRefusalKindPrivacyKeyUnavailable,
			Reason:         reason,
			TargetSequence: headSequence,
			ChainExists:    true,
		}
	}
	return &ProjectionBaseRefusal{ComputerID: computerID, Kind: BootRefusalKindStartupFailed, Reason: reason}
}
