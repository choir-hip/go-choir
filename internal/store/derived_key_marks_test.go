package store

// The derived-key marks on the real paths: Open, FinalizeBatch,
// FinalizeReplayBatch, PrepareCompactCopy and RebaseOntoCompactCopy
// (docs/design/derived-key-marks-2026-11-01.md, "Ways it can fail"; the case
// number of each test is the note's).

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

type marksHarness struct {
	t          *testing.T
	path       string
	s          *Store
	computerID string
	head       *computerevent.Head
	n          int
}

func newMarksHarness(t *testing.T) *marksHarness {
	t.Helper()
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	h := &marksHarness{t: t, path: path, s: s, computerID: "computer-derived-marks"}
	t.Cleanup(func() {
		if h.s != nil {
			_ = h.s.Close()
		}
	})
	prepareGenesis(t, s, h.computerID, "genesis-derived-marks")
	h.loadHead()
	return h
}

func (h *marksHarness) loadHead() {
	h.t.Helper()
	head, err := h.s.Head(context.Background(), h.computerID)
	if err != nil {
		h.t.Fatal(err)
	}
	h.head = head
}

// reopen closes the store and opens it again: a boot.
func (h *marksHarness) reopen() {
	h.t.Helper()
	if err := h.s.Close(); err != nil {
		h.t.Fatal(err)
	}
	h.s = nil
	s, err := Open(h.path)
	if err != nil {
		h.t.Fatal(err)
	}
	h.s = s
	h.loadHead()
}

func (h *marksHarness) closeStore() {
	h.t.Helper()
	if err := h.s.Close(); err != nil {
		h.t.Fatal(err)
	}
	h.s = nil
}

// op is a projected object of a keyed kind.
func (h *marksHarness) op(kind objectgraph.ObjectKind, key string, meta map[string]any, body string, at time.Time) computerevent.ProjectionOp {
	h.t.Helper()
	id, err := objectgraph.BuildCanonicalID(kind, "owner-1", key)
	if err != nil {
		h.t.Fatal(err)
	}
	m, _ := json.Marshal(meta)
	raw, err := json.Marshal(objectgraph.Object{CanonicalID: id, ObjectKind: kind, OwnerID: "owner-1", ComputerID: h.computerID, VersionID: "v",
		ContentHash: objectgraph.ContentHash(kind, []byte(body), m), Body: []byte(body), Metadata: m, CreatedAt: at, UpdatedAt: at})
	if err != nil {
		h.t.Fatal(err)
	}
	return computerevent.ProjectionOp{Kind: computerevent.ProjectionOpObject, CanonicalID: id, Body: raw}
}

// fourKinds is one object of every keyed family, values tagged by tag.
func (h *marksHarness) fourKinds(tag string, at time.Time) []computerevent.ProjectionOp {
	return []computerevent.ProjectionOp{
		h.op("choir.run", "run-"+tag, map[string]any{"agent_id": "agent-" + tag}, `{}`, at),
		h.op("choir.source_ref", "src-"+tag, map[string]any{"doc_id": "doc-" + tag}, `{}`, at),
		h.op(objectgraph.ActorWakeOutboxKind, "wake-"+tag, map[string]any{"projected": "false"}, `{}`, at),
		h.op("choir.commitment_record", "rec-"+tag, map[string]any{}, `{"addressee":"addr-`+tag+`"}`, at),
	}
}

func (h *marksHarness) batch(ops []computerevent.ProjectionOp) (string, computerevent.Receipt, *computerevent.ProjectionBatch) {
	h.t.Helper()
	h.n++
	eventID, err := computerevent.NewEventID()
	if err != nil {
		h.t.Fatal(err)
	}
	event, digest := prepareProjectionEvent(h.t, h.s, *h.head, eventID, fmt.Sprintf("marks-%d-%d", h.n, time.Now().UnixNano()))
	b := &computerevent.ProjectionBatch{Version: computerevent.ProjectionBatchV1, ProjectorVersion: computerevent.ProjectorVersionV1,
		ComputerID: h.computerID, EventID: event.EventID, EventDigest: digest, Ops: ops}
	return digest, signedReceipt(h.t, h.computerID, digest, event.Sequence), b
}

// finalize is the live projector.
func (h *marksHarness) finalize(ops ...computerevent.ProjectionOp) error {
	h.t.Helper()
	digest, receipt, b := h.batch(ops)
	err := h.s.FinalizeBatch(context.Background(), h.computerID, digest, receipt, b)
	h.loadHead()
	return err
}

// replay is the boot's tape replay.
func (h *marksHarness) replay(ops ...computerevent.ProjectionOp) {
	h.t.Helper()
	digest, receipt, b := h.batch(ops)
	if err := h.s.FinalizeReplayBatch(context.Background(), h.computerID, digest, receipt, b); err != nil {
		h.t.Fatal(err)
	}
	h.loadHead()
}

var allFamilies = []string{objectgraph.KeyActorWakeOutboxProjected, objectgraph.KeyRunAgentID, objectgraph.KeyCommitmentAddressee, objectgraph.KeySourceRefDocID}

func (h *marksHarness) validated() map[string]bool {
	out := map[string]bool{}
	for _, name := range allFamilies {
		out[name] = h.s.derivedKeys().Validated(name)
	}
	return out
}

func (h *marksHarness) wantValidated(when string, want map[string]bool) {
	h.t.Helper()
	got := h.validated()
	for _, name := range allFamilies {
		if got[name] != want[name] {
			h.t.Fatalf("%s: family %s validated=%t, want %t (all: %v)", when, name, got[name], want[name], got)
		}
	}
}

func every(v bool) map[string]bool {
	out := map[string]bool{}
	for _, name := range allFamilies {
		out[name] = v
	}
	return out
}

func except(v bool, names ...string) map[string]bool {
	out := every(v)
	for _, name := range names {
		out[name] = !v
	}
	return out
}

func (h *marksHarness) marks() map[string]string {
	h.t.Helper()
	rows, err := h.s.db.Query(`SELECT key_name, content_hash FROM og_key_index WHERE canonical_id = ''`)
	if err != nil {
		h.t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var name, head string
		if err := rows.Scan(&name, &head); err != nil {
			h.t.Fatal(err)
		}
		out[name] = head
	}
	return out
}

var passLine = regexp.MustCompile(`derived keys (\S+): validated (\d+) rows in \S+ \((\d+) re-derived, (\d+) orphans dropped`)

// pass runs the real validator over every family and returns, per family, the
// rows it re-derived plus the orphans it dropped: zero means the table was
// already exactly what a fresh derivation gives.
func (h *marksHarness) pass() map[string]int {
	h.t.Helper()
	var mu sync.Mutex
	fixed := map[string]int{}
	err := h.s.ValidateDerivedKeys(context.Background(), objectgraph.DerivedKeyValidation{Logf: func(format string, args ...any) {
		line := fmt.Sprintf(format, args...)
		if m := passLine.FindStringSubmatch(line); m != nil {
			r, _ := strconv.Atoi(m[3])
			o, _ := strconv.Atoi(m[4])
			mu.Lock()
			fixed[m[1]] = r + o
			mu.Unlock()
		}
	}})
	if err != nil {
		h.t.Fatal(err)
	}
	for _, name := range allFamilies {
		if _, ok := fixed[name]; !ok {
			h.t.Fatalf("the pass did not publish %s (%v)", name, fixed)
		}
	}
	return fixed
}

func (h *marksHarness) wantCurrent(when string) {
	h.t.Helper()
	for name, n := range h.pass() {
		if n != 0 {
			h.t.Fatalf("%s: the side table was stale for %s: a fresh pass re-derived or dropped %d rows", when, name, n)
		}
	}
}

func (h *marksHarness) agentRuns(agent string) []string {
	h.t.Helper()
	objs, err := h.s.ogListAllByMetadata(context.Background(), "choir.run", "agent_id", agent)
	if err != nil {
		h.t.Fatal(err)
	}
	ids := make([]string, 0, len(objs))
	for _, o := range objs {
		ids = append(ids, o.CanonicalID)
	}
	return ids
}

// trustedHarness is a store whose four families were validated by a pass and
// then written by the live projector: the state every running computer is in.
func trustedHarness(t *testing.T) *marksHarness {
	t.Helper()
	h := newMarksHarness(t)
	at := time.Date(2026, 11, 1, 1, 0, 0, 0, time.UTC)
	if err := h.finalize(h.fourKinds("a", at)...); err != nil {
		t.Fatal(err)
	}
	h.pass()
	if err := h.finalize(h.fourKinds("b", at.Add(time.Minute))...); err != nil {
		t.Fatal(err)
	}
	h.wantValidated("after a pass and a live finalize", every(true))
	if got := len(h.marks()); got != len(allFamilies) {
		t.Fatalf("premise: %d marks after a live finalize, want %d (%v)", got, len(allFamilies), h.marks())
	}
	return h
}

// Case 1: a store that validated and then ran is reopened; every family is
// validated by its mark, with no pass.
func TestReopenTrustsMarksStampedByTheProjector(t *testing.T) {
	h := trustedHarness(t)
	h.reopen()
	h.wantValidated("straight after Open", every(true))
	if got := h.agentRuns("agent-b"); len(got) != 1 {
		t.Fatalf("the trusted run family answers %v for agent-b", got)
	}
	h.wantCurrent("after the reopen")
}

// Case 2: a binary that does not stamp (an older binary's replay) writes
// og_objects and moves the head; the next open trusts nothing and the pass
// repairs the row.
func TestHeadMovedWithoutStampDistrustsTheMarks(t *testing.T) {
	h := trustedHarness(t)
	runID, _ := objectgraph.BuildCanonicalID("choir.run", "owner-1", "run-a")
	statePath := h.s.statePath
	h.closeStore()
	raw, err := sql.Open("sqlite", dbx.SQLiteDSN(statePath, dbx.SQLiteOptions{}, false))
	if err != nil {
		t.Fatal(err)
	}
	meta := `{"agent_id":"agent-moved"}`
	if _, err := raw.Exec(`UPDATE og_objects SET metadata = ?, content_hash = ? WHERE canonical_id = ?`, meta, objectgraph.ContentHash("choir.run", []byte(`{}`), json.RawMessage(meta)), runID); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`UPDATE computer_event_projection_heads SET sequence = sequence + 1, canonical_event_head = 'older-binary-head'`); err != nil {
		t.Fatal(err)
	}
	_ = raw.Close()
	s, err := Open(h.path)
	if err != nil {
		t.Fatal(err)
	}
	h.s = s
	h.wantValidated("after a head move with no stamp", every(false))
	if got := len(h.marks()); got != 0 {
		t.Fatalf("Open kept %d distrusted marks", got)
	}
	if got := h.agentRuns("agent-moved"); len(got) != 1 {
		t.Fatalf("the JSON path answers %v for the moved row", got)
	}
	if fixed := h.pass(); fixed[objectgraph.KeyRunAgentID] != 1 {
		t.Fatalf("the pass re-derived %v, want the one moved run row", fixed)
	}
	if got := h.agentRuns("agent-moved"); len(got) != 1 {
		t.Fatalf("after the pass the run family answers %v", got)
	}
}

// Case 3: a head-neutral maintained writer meets a malformed body; the mark
// goes in the writer's own transaction.
func TestMalformedDirectWriteDeletesTheMark(t *testing.T) {
	h := trustedHarness(t)
	id, _ := objectgraph.BuildCanonicalID("choir.commitment_record", "owner-1", "rec-malformed")
	at := time.Date(2026, 11, 1, 2, 0, 0, 0, time.UTC)
	body := []byte(`not json`)
	if err := h.s.ogStore.PutObject(context.Background(), objectgraph.Object{CanonicalID: id, ObjectKind: "choir.commitment_record", OwnerID: "owner-1",
		ComputerID: h.computerID, VersionID: "v", ContentHash: objectgraph.ContentHash("choir.commitment_record", body, json.RawMessage(`{}`)),
		Body: body, Metadata: json.RawMessage(`{}`), CreatedAt: at, UpdatedAt: at}); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.marks()[objectgraph.KeyCommitmentAddressee]; ok {
		t.Fatal("the malformed write left the addressee mark")
	}
	h.reopen()
	h.wantValidated("after the malformed write", except(true, objectgraph.KeyCommitmentAddressee))
}

// Case 4: a family withdrawn in memory (a read tripwire, a failed
// maintenance) loses its mark at the next head-moving transaction.
func TestWithdrawnFamilyLosesItsMarkAtTheNextFinalize(t *testing.T) {
	h := trustedHarness(t)
	h.s.derivedKeys().Invalidate(objectgraph.KeyRunAgentID, "test: a tripwire")
	if err := h.finalize(h.op("choir.source_ref", "src-after-withdrawal", map[string]any{"doc_id": "doc-w"}, `{}`, time.Date(2026, 11, 1, 2, 0, 0, 0, time.UTC))); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.marks()[objectgraph.KeyRunAgentID]; ok {
		t.Fatal("a finalize after the withdrawal kept the run mark")
	}
	h.reopen()
	h.wantValidated("after a withdrawal and a finalize", except(true, objectgraph.KeyRunAgentID))
}

// Case 5: a replayed batch keeps the table and the marks; the reopen trusts
// them and the table is what a fresh derivation gives.
func TestReplayMaintainsTheTableAndTheMarks(t *testing.T) {
	h := trustedHarness(t)
	at := time.Date(2026, 11, 1, 3, 0, 0, 0, time.UTC)
	ops := h.fourKinds("replayed", at)
	// An update of a row the table already holds, with a new key.
	ops = append(ops, h.op("choir.run", "run-a", map[string]any{"agent_id": "agent-a2"}, `{}`, at))
	h.replay(ops...)
	h.wantValidated("after a replayed batch", every(true))
	heads := h.marks()
	if len(heads) != len(allFamilies) {
		t.Fatalf("after the replay: marks %v", heads)
	}
	if got := h.agentRuns("agent-replayed"); len(got) != 1 {
		t.Fatalf("the replayed run reads %v", got)
	}
	if got := h.agentRuns("agent-a"); len(got) != 0 {
		t.Fatalf("the replayed update left the old key: %v", got)
	}
	h.reopen()
	h.wantValidated("after a replay and a reopen", every(true))
	h.wantCurrent("after a replay")
}

// Case 6: a compact copy staged while the store runs, more live writes after
// it, a planned boot that swaps the copy in, and the tail replayed. The
// 05:30 rebase on the paper left 531 rows to re-derive this way.
func TestCompactRebaseThenReplayKeepsTheMarksTrue(t *testing.T) {
	h := trustedHarness(t)
	ctx := context.Background()
	if done, err := h.s.prepareCompactCopy(ctx, true); err != nil || !done {
		t.Fatalf("prepareCompactCopy = %v, %v", done, err)
	}
	at := time.Date(2026, 11, 1, 4, 0, 0, 0, time.UTC)
	tail := [][]computerevent.ProjectionOp{
		h.fourKinds("tail1", at),
		append(h.fourKinds("tail2", at.Add(time.Minute)), h.op("choir.run", "run-a", map[string]any{"agent_id": "agent-a-tail"}, `{}`, at.Add(time.Minute))),
	}
	for _, ops := range tail {
		if err := h.finalize(ops...); err != nil {
			t.Fatal(err)
		}
	}
	h.closeStore()
	outcome, err := RebaseOntoCompactCopy(h.path, true)
	if err != nil || !strings.HasPrefix(outcome, "rebased") {
		t.Fatalf("planned boot: %q, %v", outcome, err)
	}
	s, err := Open(h.path)
	if err != nil {
		t.Fatal(err)
	}
	h.s = s
	h.loadHead()
	// The copy is one committed state: its marks name its own head.
	h.wantValidated("on the swapped-in copy", every(true))
	if got := h.agentRuns("agent-tail1"); len(got) != 0 {
		t.Fatalf("premise: the copy already holds the tail (%v)", got)
	}
	for _, ops := range tail {
		h.replay(ops...)
	}
	h.wantValidated("after the tail replay", every(true))
	if got := h.agentRuns("agent-a-tail"); len(got) != 1 {
		t.Fatalf("the replayed tail update reads %v", got)
	}
	h.reopen()
	h.wantValidated("after the rebase boot's reopen", every(true))
	h.wantCurrent("after the rebase and the tail replay")
}

// Case 7: a finalize that fails after its projection writes and its stamp
// (here the partition hook, the last step before commit) commits nothing:
// the reopen trusts the old marks against the old head, and the table matches.
func TestFailedFinalizeCommitsNeitherRowsNorMark(t *testing.T) {
	h := trustedHarness(t)
	before := h.marks()
	partitionFlushedHook = func(uint64) error { return errors.New("test: crash before commit") }
	at := time.Date(2026, 11, 1, 5, 0, 0, 0, time.UTC)
	err := h.finalize(h.fourKinds("lost", at)...)
	partitionFlushedHook = nil
	if err == nil {
		t.Fatal("the finalize did not fail")
	}
	if after := h.marks(); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Fatalf("a failed finalize changed the marks: %v -> %v", before, after)
	}
	h.reopen()
	h.wantValidated("after a failed finalize", every(true))
	if got := h.agentRuns("agent-lost"); len(got) != 0 {
		t.Fatalf("the failed finalize's run is visible: %v", got)
	}
	h.wantCurrent("after a failed finalize")
}

// Case 8: a mark written for another family definition or generation is
// never trusted, and Open deletes it.
func TestMarkOfAnotherDefinitionIsDeleted(t *testing.T) {
	h := trustedHarness(t)
	if _, err := h.s.db.Exec(`UPDATE og_key_index SET owner_id = 'og_key_index mark g0 0000' WHERE canonical_id = '' AND key_name = ?`, objectgraph.KeySourceRefDocID); err != nil {
		t.Fatal(err)
	}
	h.reopen()
	h.wantValidated("after a foreign mark", except(true, objectgraph.KeySourceRefDocID))
	if _, ok := h.marks()[objectgraph.KeySourceRefDocID]; ok {
		t.Fatal("Open kept the foreign mark")
	}
}

// Case 9: a store with no head, or more than one head row, is never trusted
// and never stamped.
func TestNoHeadNeverTrusts(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ValidateDerivedKeys(context.Background(), objectgraph.DerivedKeyValidation{}); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM og_key_index WHERE canonical_id = ''`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("a store with no head stamped %d marks (%v)", n, err)
	}
	_ = s.Close()

	h := trustedHarness(t)
	if _, err := h.s.db.Exec(`INSERT INTO computer_event_projection_heads (computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, desired_state_commitment, effective_state_commitment, reducer_version, credential_revocation_epoch, updated_at)
		VALUES ('another-computer', 1, 'h', 'd', 'e', 'c', 'f', 1, 0, ?)`, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := h.s.Close(); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(h.path)
	if err != nil {
		t.Fatal(err)
	}
	h.s = s2
	h.wantValidated("with two head rows", every(false))
}

// The replay used to withdraw the families and skip the table; it now
// maintains it, so a replay onto a store whose families are not validated
// leaves the table complete for the pass.
func TestReplayOntoAnUnvalidatedStoreFillsTheTable(t *testing.T) {
	h := newMarksHarness(t)
	at := time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC)
	h.replay(h.fourKinds("r", at)...)
	h.wantValidated("before any pass", every(false))
	if got := len(h.marks()); got != 0 {
		t.Fatalf("an unvalidated replay stamped %d marks", got)
	}
	var n int
	if err := h.s.db.QueryRow(`SELECT COUNT(*) FROM og_key_index WHERE canonical_id <> ''`).Scan(&n); err != nil || n != 4 {
		t.Fatalf("the replay left %d key rows (%v), want 4", n, err)
	}
	h.wantCurrent("after a replay onto an unvalidated store")
}
