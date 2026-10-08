package autoputer

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
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
