package autoputer

// The early health gate of the store header/body split (store_split_gate.go).
// Ways it can fail, written before the tests:
//
//   - G1: the gate answers a body the host or the updater does not parse as
//     replaying, so the 20 s boot window kills the migrating guest;
//   - G2: progress does not advance during a long single statement, so the
//     120 s stall window kills it;
//   - G3: the address is unbound between the gate and the main server (a
//     probe then fails past the boot deadline), or the main server cannot
//     bind because the gate still holds it;
//   - G4: a refused split is not served as the typed refusal, so the host
//     waits out its window instead of failing fast with a witness.

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// splitGateProbe reads /health the way vmmanager.probeGuestHealthDetailed and
// the updater's health probe do.
type splitGateProbe struct {
	Status            string `json:"status"`
	Kind              string `json:"kind"`
	Reason            string `json:"reason"`
	Sequence          uint64 `json:"sequence"`
	CommittedSequence uint64 `json:"committed_sequence"`
	Progress          uint64 `json:"progress"`
}

func probeSplitGate(t *testing.T, addr string) (int, splitGateProbe) {
	t.Helper()
	resp, err := (&http.Client{Timeout: 2 * time.Second}).Get("http://" + addr + "/health")
	if err != nil {
		t.Fatalf("probe %s: %v", addr, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var p splitGateProbe
	if resp.StatusCode == http.StatusServiceUnavailable {
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Fatalf("body %q: %v", raw, err)
		}
	}
	return resp.StatusCode, p
}

func TestSplitGateAnswersReplayingWithAdvancingProgress(t *testing.T) {
	prev := splitGateTickEvery
	splitGateTickEvery = 20 * time.Millisecond
	t.Cleanup(func() { splitGateTickEvery = prev })
	g, err := startSplitGate("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := g.tcp.Addr().String()
	status, p := probeSplitGate(t, addr)
	if status != http.StatusServiceUnavailable || p.Status != "replaying" || p.Sequence != 0 || p.CommittedSequence != 0 {
		t.Fatalf("gate answered %d %+v, want 503 replaying at sequence 0", status, p)
	}
	g.setRows(1000)
	g.setRows(10) // monotone
	_, p2 := probeSplitGate(t, addr)
	if p2.Progress < 1000 {
		t.Fatalf("progress %d after 1000 rows", p2.Progress)
	}
	// No rows copied (an index build): the tick still advances it.
	time.Sleep(120 * time.Millisecond)
	_, p3 := probeSplitGate(t, addr)
	if p3.Progress <= p2.Progress {
		t.Fatalf("progress did not advance during a long statement: %d then %d", p2.Progress, p3.Progress)
	}
	// The main server takes the socket over: the same address answers, with
	// no unbound moment.
	ln, err := g.handoff()
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, `{"status":"ok"}`) })}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()
	if status, _ := probeSplitGate(t, addr); status != http.StatusOK {
		t.Fatalf("after the handoff the address answered %d", status)
	}
	// The tick stopped with the handoff.
	if a, b := g.progress(), func() uint64 { time.Sleep(60 * time.Millisecond); return g.progress() }(); a != b {
		t.Fatalf("progress still ticking after the handoff: %d then %d", a, b)
	}
}

// presplitStoreForGate writes an old-layout state file under dir.
func presplitStoreForGate(t *testing.T, dir string) string {
	t.Helper()
	storePath := filepath.Join(dir, "runtime.db")
	statePath := store.StateDBPath(storePath)
	schema, err := os.ReadFile(filepath.Join("..", "store", "testdata", "presplit", "schema.sql"))
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", dbx.SQLiteDSN(statePath, dbx.SQLiteOptions{}, false))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO og_objects (canonical_id, object_kind, owner_id, content_hash, body, metadata, created_at, updated_at) VALUES ('o1', 'choir.run', 'owner', 'h', 'b', '{}', '2026-11-01', '2026-11-01')`); err != nil {
		t.Fatal(err)
	}
	return storePath
}

// A split that refuses (here: its attempts are exhausted) is served by the
// main server, on the gate's socket, as the typed refusal.
func TestSplitGateServesTypedRefusal(t *testing.T) {
	storePath := presplitStoreForGate(t, t.TempDir())
	if err := os.WriteFile(store.StateDBPath(storePath)+".split-attempts", []byte("3"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SERVER_HOST", "127.0.0.1")
	s := server.NewServer("autoputer-split-test", "0")
	refusal := runStoreSplitUnderGate(s, "computer-split", storePath)
	if refusal == nil || refusal.Kind != BootRefusalKindStoreHeaderBodySplit || !strings.Contains(refusal.Reason, "header-body-split refused") {
		t.Fatalf("refusal = %+v", refusal)
	}
	s.SetHealthHandler(func(w http.ResponseWriter, r *http.Request) { writeBootRefusalHealth(w, refusal) })
	go s.Start()
	defer s.Shutdown()
	deadline := time.Now().Add(5 * time.Second)
	for s.Addr() == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	status, p := probeSplitGate(t, s.Addr())
	if status != http.StatusServiceUnavailable || p.Status != "refused" || p.Kind != "store_header_body_split" || p.Reason == "" {
		t.Fatalf("refused split answered %d %+v", status, p)
	}
}

// A store already split (or new) binds no gate: the boot is unchanged.
func TestSplitGateNotBoundWithoutWork(t *testing.T) {
	s := server.NewServer("autoputer-split-test", "0")
	if refusal := runStoreSplitUnderGate(s, "c", filepath.Join(t.TempDir(), "runtime.db")); refusal != nil {
		t.Fatalf("refusal on an empty store: %+v", refusal)
	}
}
