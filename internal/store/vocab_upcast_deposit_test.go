package store

// Regression coverage for versioned projection-deposit upcasting (mission-2
// residual: cutover-aware replay; owner-authorized recovery policy
// 2026-10-07).
//
// The tests pin the policy boundary: the original immutable event, receipt,
// payload digest and reducer commitments are verified first and never
// rewritten; only the verified projection-deposit view is upcast, with the
// frozen vocabulary map, deterministically, in canonical event order. Fresh
// reconstruction converges on ONE canonical identity per object, later
// events (V1-spelled and V2-spelled) update that identity in sequence, deletes
// recorded under a legacy ID resolve to the same row, genesis replay and
// base-plus-tail replay agree byte for byte, and retained V1 stores keep
// their one-time end-of-replay migration.

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/vocabmigrate"
)

const vocabUpcastComputerID = "computer-vocab-upcast"
const vocabUpcastOwnerID = "owner"

func vocabUpcastStorePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(t.TempDir(), name+".db")
}

func vocabUpcastOpenStore(t *testing.T, name string) (*Store, string) {
	t.Helper()
	path := vocabUpcastStorePath(t, name)
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open %s store: %v", name, err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, path
}

// vocabUpcastAgentID derives the canonical ID a choir.agent row carries for
// the given agent_id spelling, exactly as the writer and the migration do.
func vocabUpcastAgentID(t *testing.T, agentID string) string {
	t.Helper()
	id, err := objectgraph.BuildCanonicalID("choir.agent", vocabUpcastOwnerID,
		objectgraph.StableSuffixFromKey(vocabUpcastComputerID+"\x00"+agentID))
	if err != nil {
		t.Fatalf("agent canonical id: %v", err)
	}
	return id
}

func vocabUpcastAgentObject(t *testing.T, agentID, profile, state string, tombstone bool, created, updated time.Time) objectgraph.Object {
	t.Helper()
	body, err := json.Marshal(map[string]any{"agent_id": agentID, "profile": profile, "state": state})
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := objectgraph.NormalizeMetadata(map[string]any{"agent_id": agentID, "profile": profile})
	if err != nil {
		t.Fatal(err)
	}
	return objectgraph.Object{
		CanonicalID: vocabUpcastAgentID(t, agentID),
		ObjectKind:  "choir.agent",
		OwnerID:     vocabUpcastOwnerID,
		ComputerID:  vocabUpcastComputerID,
		ContentHash: objectgraph.ContentHash("choir.agent", body, metadata),
		Body:        body,
		Metadata:    metadata,
		CreatedAt:   created,
		UpdatedAt:   updated,
		Tombstone:   tombstone,
	}
}

func vocabUpcastEventObject(t *testing.T, agentRef, actorProfile string, created time.Time) objectgraph.Object {
	t.Helper()
	body, err := json.Marshal(map[string]any{"event_id": "vocab-upcast-event-1", "actor_profile": actorProfile, "agent_ref": agentRef})
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := objectgraph.NormalizeMetadata(map[string]any{"actor_profile": actorProfile})
	if err != nil {
		t.Fatal(err)
	}
	hash := objectgraph.ContentHash("choir.event", body, metadata)
	id, err := objectgraph.BuildCanonicalID("choir.event", vocabUpcastOwnerID, objectgraph.StableSuffixFromContent(hash))
	if err != nil {
		t.Fatal(err)
	}
	return objectgraph.Object{
		CanonicalID: id, ObjectKind: "choir.event", OwnerID: vocabUpcastOwnerID,
		ComputerID: vocabUpcastComputerID, ContentHash: hash, Body: body, Metadata: metadata,
		CreatedAt: created, UpdatedAt: created,
	}
}

func vocabUpcastObjectOp(t *testing.T, obj objectgraph.Object) computerevent.ProjectionOp {
	t.Helper()
	body, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	return computerevent.ProjectionOp{Kind: computerevent.ProjectionOpObject, CanonicalID: obj.CanonicalID, Body: body}
}

func vocabUpcastEdgeOp(t *testing.T, edge objectgraph.Edge) computerevent.ProjectionOp {
	t.Helper()
	body, err := json.Marshal(edge)
	if err != nil {
		t.Fatal(err)
	}
	return computerevent.ProjectionOp{Kind: computerevent.ProjectionOpObjectEdge, CanonicalID: edge.EdgeID, Body: body}
}

type vocabUpcastTapeEvent struct {
	Event   computerevent.Event
	Input   computerevent.TransitionInput
	Next    computerevent.Head
	Receipt computerevent.Receipt
	// Ops is nil for the batchless genesis head of the tape.
	Ops []computerevent.ProjectionOp
}

func vocabUpcastTestEvent(t *testing.T, sequence uint64, previousHead string, kind computerevent.EventKind, current *computerevent.Head) (computerevent.Event, computerevent.TransitionInput, computerevent.Head) {
	t.Helper()
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatalf("event id: %v", err)
	}
	event := computerevent.Event{
		SchemaVersion:     computerevent.SchemaVersionV1,
		EventID:           eventID,
		ComputerID:        vocabUpcastComputerID,
		Sequence:          sequence,
		PreviousHead:      previousHead,
		EventKind:         kind,
		OccurredAt:        time.Date(2026, 10, 7, 12, 0, int(sequence%60), 0, time.UTC).Format(time.RFC3339Nano),
		IdempotencyKey:    fmt.Sprintf("vocab-upcast:%d", sequence),
		RequestCommitment: storeTestDigest('d'),
		ActorProfile:      "trusted-core",
		AuthorityRef:      "authority:vocab-upcast-test",
		PayloadCommitment: storeTestDigest('a'),
		PrivacyClass:      "owner",
		ReducerVersion:    computerevent.ReducerVersionV1,
	}
	input := computerevent.TransitionInput{}
	if current == nil {
		event.ExpectedDesiredEventHead = computerevent.ZeroHead
		event.ExpectedEffectiveEventHead = computerevent.ZeroHead
		event.ExpectedDesiredStateCommitment = computerevent.ZeroHead
		event.ExpectedEffectiveStateCommitment = computerevent.ZeroHead
		event.ResultingEffectiveCommitment = storeTestDigest('b')
		input.TargetStateCommitment = storeTestDigest('b')
	} else {
		event.ExpectedDesiredEventHead = current.DesiredEventHead
		event.ExpectedEffectiveEventHead = current.EffectiveEventHead
		event.ExpectedPendingTransitionRef = current.PendingTransitionRef
		event.ExpectedDesiredStateCommitment = current.DesiredStateCommitment
		event.ExpectedEffectiveStateCommitment = current.EffectiveStateCommitment
	}
	next, err := computerevent.Reduce(current, event, input)
	if err != nil {
		t.Fatalf("reduce sequence %d: %v", sequence, err)
	}
	return event, input, next
}

func vocabUpcastReceipt(t *testing.T, event computerevent.Event) computerevent.Receipt {
	t.Helper()
	digest, err := event.Digest()
	if err != nil {
		t.Fatalf("event digest: %v", err)
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("receipt key: %v", err)
	}
	receipt, err := computerevent.NewSignedReceipt("EventHeadReceipt", "corpusd",
		map[string]any{"computer_id": event.ComputerID, "event_digest": digest, "sequence": event.Sequence},
		[]computerevent.SigningKey{{
			SignerRef:  computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "vocab-upcast-test"},
			PrivateKey: privateKey,
		}}, time.Now().UTC())
	if err != nil {
		t.Fatalf("receipt: %v", err)
	}
	return receipt
}

// vocabUpcastApplyTapeEvent replays one tape event through the store's
// per-event Prepare/Finalize path. Batches are copied first: the store's
// deposit upcast mutates the batch it is handed, and the tape must stay the
// original recorded bytes for every replay of it.
func vocabUpcastApplyTapeEvent(t *testing.T, s *Store, tapeEvent vocabUpcastTapeEvent) {
	t.Helper()
	ctx := context.Background()
	digest, err := tapeEvent.Event.Digest()
	if err != nil {
		t.Fatalf("event digest: %v", err)
	}
	request := computerevent.CASRequest{
		Event: tapeEvent.Event, EventDigest: digest, EventArtifactDigest: digest,
		EventPinReceiptDigest: storeTestDigest('c'), Input: tapeEvent.Input, Next: tapeEvent.Next,
	}
	if err := s.Prepare(ctx, request); err != nil {
		t.Fatalf("prepare sequence %d: %v", tapeEvent.Event.Sequence, err)
	}
	if tapeEvent.Ops == nil {
		// Mirror the product replay path (ComputerEventAppender.finalizeProjection
		// with replayProjection): a batchless event still finalizes through
		// FinalizeReplayBatch, which is where a fresh reconstruction is detected.
		if err := s.FinalizeReplayBatch(ctx, tapeEvent.Event.ComputerID, digest, tapeEvent.Receipt, nil); err != nil {
			t.Fatalf("finalize sequence %d: %v", tapeEvent.Event.Sequence, err)
		}
		return
	}
	batch := computerevent.ProjectionBatch{
		Version:          computerevent.ProjectionBatchV2,
		ProjectorVersion: computerevent.ProjectorVersionV2,
		ComputerID:       tapeEvent.Event.ComputerID,
		EventID:          tapeEvent.Event.EventID,
		EventDigest:      digest,
		Ops:              vocabUpcastCopyOps(tapeEvent.Ops),
	}
	if err := s.FinalizeReplayBatch(ctx, tapeEvent.Event.ComputerID, digest, tapeEvent.Receipt, &batch); err != nil {
		t.Fatalf("finalize replay batch sequence %d: %v", tapeEvent.Event.Sequence, err)
	}
}

func vocabUpcastCopyOps(ops []computerevent.ProjectionOp) []computerevent.ProjectionOp {
	copied := make([]computerevent.ProjectionOp, 0, len(ops))
	for _, op := range ops {
		clone := op
		if len(op.Body) > 0 {
			clone.Body = append(json.RawMessage(nil), op.Body...)
		}
		copied = append(copied, clone)
	}
	return copied
}

// vocabUpcastReplayTape replays tape[start:end] onto the store, asserting the
// store's head sits exactly at tape[start-1] first.
func vocabUpcastReplayTape(t *testing.T, s *Store, tape []vocabUpcastTapeEvent, start, end int) {
	t.Helper()
	ctx := context.Background()
	head, err := s.Head(ctx, vocabUpcastComputerID)
	if err != nil {
		t.Fatalf("read head: %v", err)
	}
	if start == 0 {
		if head != nil {
			t.Fatalf("store is not fresh: head=%+v", head)
		}
	} else {
		want := tape[start-1].Next
		if head == nil || *head != want {
			t.Fatalf("store head %+v does not match tape position %d (%+v)", head, start-1, want)
		}
	}
	for _, tapeEvent := range tape[start:end] {
		vocabUpcastApplyTapeEvent(t, s, tapeEvent)
	}
}

// vocabUpcastBuildTape returns the frozen cross-version tape used by the
// tests: a batchless genesis, then one projection-batch event per stage — a
// pre-cutover V1 agent lifecycle, a V1 event object embedding the agent's V1
// identity, a V1 edge, the cutover V2 continuation, a delete recorded under
// the legacy V1 ID, and a V2 recreate. The tape is built once and replayed
// verbatim so genesis and base-plus-tail runs consume identical bytes.
func vocabUpcastBuildTape(t *testing.T) ([]vocabUpcastTapeEvent, map[string]string) {
	t.Helper()
	base := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	agentV1 := vocabUpcastAgentID(t, "super:root")
	agentV2 := vocabUpcastAgentID(t, "management:root")
	create := vocabUpcastAgentObject(t, "super:root", "super", "a", false, base, base)
	updateV1 := vocabUpcastAgentObject(t, "super:root", "super", "b", false, base, base.Add(time.Minute))
	updateV2 := vocabUpcastAgentObject(t, "management:root", "management", "c", false, base, base.Add(2*time.Minute))
	tombstoneV2 := vocabUpcastAgentObject(t, "management:root", "management", "c", true, base, base.Add(3*time.Minute))
	// Delete recorded under the legacy V1 canonical ID (an operator or a late
	// pre-cutover writer addressing the row by its old identity) with already
	// V2 content: only alias resolution can land this on the one live row.
	deleteByAlias := vocabUpcastAgentObject(t, "management:root", "management", "c", true, base, base.Add(4*time.Minute))
	deleteByAlias.CanonicalID = agentV1
	recreate := vocabUpcastAgentObject(t, "management:root", "management", "d", false, base, base.Add(5*time.Minute))

	eventV1Object := vocabUpcastEventObject(t, agentV1, "co-super", base.Add(30*time.Second))
	edgeV1, err := objectgraph.BuildEdgeID(agentV1, eventV1Object.CanonicalID, "produced", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	edge := objectgraph.Edge{
		EdgeID: edgeV1, FromID: agentV1, ToID: eventV1Object.CanonicalID,
		Kind: "produced", Metadata: json.RawMessage(`{}`), CreatedAt: base.Add(45 * time.Second),
	}
	batches := [][]computerevent.ProjectionOp{
		{vocabUpcastObjectOp(t, create)},
		{vocabUpcastObjectOp(t, eventV1Object)},
		{vocabUpcastEdgeOp(t, edge)},
		{vocabUpcastObjectOp(t, updateV1)},
		{vocabUpcastObjectOp(t, updateV2)},
		{vocabUpcastObjectOp(t, tombstoneV2)},
		{vocabUpcastObjectOp(t, deleteByAlias)},
		{vocabUpcastObjectOp(t, recreate)},
	}
	expectations := map[string]string{
		"agent_v1": agentV1,
		"agent_v2": agentV2,
		"event_v1": eventV1Object.CanonicalID,
		"edge_v1":  edgeV1,
	}
	return vocabUpcastTapeFromBatches(t, batches), expectations
}

// vocabUpcastTapeFromBatches chains one batchless genesis plus one
// projection-batch event per supplied batch into a replayable tape.
func vocabUpcastTapeFromBatches(t *testing.T, batches [][]computerevent.ProjectionOp) []vocabUpcastTapeEvent {
	t.Helper()
	tape := make([]vocabUpcastTapeEvent, 0, len(batches)+1)
	var current *computerevent.Head
	for index := range len(batches) + 1 {
		sequence := uint64(index + 1)
		kind := computerevent.EventProjectionBatchRecorded
		previousHead := computerevent.ZeroHead
		if current == nil {
			kind = computerevent.EventGenesisImported
		} else {
			previousHead = current.CanonicalEventHead
		}
		event, input, next := vocabUpcastTestEvent(t, sequence, previousHead, kind, current)
		tapeEvent := vocabUpcastTapeEvent{Event: event, Input: input, Next: next, Receipt: vocabUpcastReceipt(t, event)}
		if index > 0 {
			tapeEvent.Ops = batches[index-1]
		}
		tape = append(tape, tapeEvent)
		current = &next
	}
	return tape
}

func vocabUpcastOGSnapshot(t *testing.T, s *Store) map[string][]string {
	t.Helper()
	ctx := context.Background()
	out := map[string][]string{}
	for _, spec := range []struct{ table, order string }{
		{"og_objects", "canonical_id"},
		{"og_edges", "edge_id"},
	} {
		rows, err := s.db.QueryContext(ctx, "SELECT * FROM "+spec.table+" ORDER BY "+spec.order)
		if err != nil {
			t.Fatalf("snapshot %s: %v", spec.table, err)
		}
		columns, err := rows.Columns()
		if err != nil {
			rows.Close()
			t.Fatal(err)
		}
		for rows.Next() {
			values := make([]any, len(columns))
			pointers := make([]any, len(values))
			for i := range values {
				pointers[i] = &values[i]
			}
			if err := rows.Scan(pointers...); err != nil {
				rows.Close()
				t.Fatalf("snapshot %s scan: %v", spec.table, err)
			}
			var line strings.Builder
			for i, column := range columns {
				fmt.Fprintf(&line, "%s=%v|", column, values[i])
			}
			out[spec.table] = append(out[spec.table], line.String())
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatalf("snapshot %s: %v", spec.table, err)
		}
		sort.Strings(out[spec.table])
	}
	return out
}

// vocabUpcastTapeRowSnapshot captures the raw tape rows (event bytes, receipt
// bytes, receipt digest, status) so cross-path runs can be compared without
// re-deriving them.
func vocabUpcastTapeRowSnapshot(t *testing.T, s *Store) []string {
	t.Helper()
	ctx := context.Background()
	rows, err := s.db.QueryContext(ctx,
		`SELECT sequence, event_digest, event_json, event_head_receipt_json, event_head_receipt_digest, status FROM computer_event_index WHERE computer_id=? ORDER BY sequence`,
		vocabUpcastComputerID)
	if err != nil {
		t.Fatalf("tape row snapshot: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var sequence uint64
		var digest, eventJSON, receiptJSON, receiptDigest, status string
		if err := rows.Scan(&sequence, &digest, &eventJSON, &receiptJSON, &receiptDigest, &status); err != nil {
			t.Fatalf("tape row snapshot scan: %v", err)
		}
		out = append(out, fmt.Sprintf("%d|%s|%s|%s|%s|%s", sequence, digest, eventJSON, receiptJSON, receiptDigest, status))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("tape row snapshot: %v", err)
	}
	return out
}

func vocabUpcastAssertSnapshotsEqual(t *testing.T, name string, want, got []string) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("%s rows %d, want %d", name, len(got), len(want))
	}
	for i := range want {
		if want[i] != got[i] {
			t.Fatalf("%s row %d differs:\n got: %s\nwant: %s", name, i, got[i], want[i])
		}
	}
}

// vocabUpcastReadLedger parses the deposit-upcast sidecar; nil when absent.
func vocabUpcastReadLedger(t *testing.T, s *Store) *depositUpcastLedger {
	t.Helper()
	ledger, err := loadDepositUpcastLedger(filepath.Join(s.TexturePath(), depositUpcastLedgerFileName))
	if err != nil {
		t.Fatalf("load deposit upcast ledger: %v", err)
	}
	return ledger
}

// vocabUpcastAssertRawTapeUntouched proves the raw tape rows are byte-identical
// to the recorded tape: event bytes, receipt bytes and receipt digests.
func vocabUpcastAssertRawTapeUntouched(t *testing.T, s *Store, tape []vocabUpcastTapeEvent) {
	t.Helper()
	ctx := context.Background()
	for _, tapeEvent := range tape {
		eventJSON, err := tapeEvent.Event.CanonicalBytes()
		if err != nil {
			t.Fatal(err)
		}
		receiptJSON, err := tapeEvent.Receipt.CanonicalBytes()
		if err != nil {
			t.Fatal(err)
		}
		digest, err := tapeEvent.Event.Digest()
		if err != nil {
			t.Fatal(err)
		}
		var storedEvent, storedReceipt, storedReceiptDigest, status string
		if err := s.db.QueryRowContext(ctx,
			`SELECT event_json, event_head_receipt_json, event_head_receipt_digest, status FROM computer_event_index WHERE computer_id=? AND event_digest=?`,
			vocabUpcastComputerID, digest).Scan(&storedEvent, &storedReceipt, &storedReceiptDigest, &status); err != nil {
			t.Fatalf("read raw tape row: %v", err)
		}
		if status != "finalized" {
			t.Fatalf("event row status = %q, want finalized", status)
		}
		if storedEvent != string(eventJSON) {
			t.Fatalf("raw event bytes were rewritten:\n got: %s\nwant: %s", storedEvent, eventJSON)
		}
		if storedReceipt != string(receiptJSON) {
			t.Fatalf("original receipt bytes were rewritten:\n got: %s\nwant: %s", storedReceipt, receiptJSON)
		}
		if want := computerevent.DigestBytes(receiptJSON); storedReceiptDigest != want {
			t.Fatalf("receipt digest = %s, want %s", storedReceiptDigest, want)
		}
	}
}

func vocabUpcastCopyTree(t *testing.T, src, dst string) {
	t.Helper()
	if err := filepath.WalkDir(src, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	}); err != nil {
		t.Fatalf("copy tree %s -> %s: %v", src, dst, err)
	}
}

// TestVocabDepositUpcastMonoIdentityAcrossCutover is the primary regression:
// one fresh replay deposits a pre-cutover V1 agent, a V1 event and edge
// referencing it, the post-cutover V2 continuation, a delete recorded under
// the legacy V1 canonical ID, and a V2 recreate. The store must converge on
// ONE canonical identity, later events must update it in sequence, aliases
// must resolve, the raw tape rows must be byte-identical, and the
// end-of-replay migration must be verify-only (deposit upcast recorded, no
// rows left to migrate).
func TestVocabDepositUpcastMonoIdentityAcrossCutover(t *testing.T) {
	ctx := context.Background()
	s, _ := vocabUpcastOpenStore(t, "mono")
	tape, expect := vocabUpcastBuildTape(t)
	vocabUpcastReplayTape(t, s, tape, 0, len(tape))
	head, err := s.Head(ctx, vocabUpcastComputerID)
	if err != nil {
		t.Fatal(err)
	}
	if head == nil || head.Sequence != uint64(len(tape)) {
		t.Fatalf("head = %+v, want sequence %d", head, len(tape))
	}

	// One canonical agent identity, updated in sequence, ending recreated.
	var agentID, agentBody, agentMeta, agentHash string
	var tombstone int
	if err := s.db.QueryRowContext(ctx,
		`SELECT canonical_id, body, metadata, content_hash, tombstone FROM og_objects WHERE object_kind='choir.agent'`).
		Scan(&agentID, &agentBody, &agentMeta, &agentHash, &tombstone); err != nil {
		t.Fatalf("read agent row: %v", err)
	}
	if agentID != expect["agent_v2"] {
		t.Fatalf("agent canonical_id = %s, want %s (single V2 identity)", agentID, expect["agent_v2"])
	}
	var agentCount int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM og_objects WHERE object_kind='choir.agent'`).Scan(&agentCount); err != nil {
		t.Fatal(err)
	}
	if agentCount != 1 {
		t.Fatalf("agent rows = %d, want 1 (no V1/V2 dual identity)", agentCount)
	}
	if tombstone != 0 || !strings.Contains(agentBody, `"state":"d"`) {
		t.Fatalf("agent row not at the recreated state: tombstone=%d body=%s", tombstone, agentBody)
	}
	if !strings.Contains(agentBody, `"agent_id":"management:root"`) || !strings.Contains(agentBody, `"profile":"management"`) {
		t.Fatalf("agent body not upcast: %s", agentBody)
	}
	if !strings.Contains(agentMeta, `"agent_id":"management:root"`) {
		t.Fatalf("agent metadata not upcast: %s", agentMeta)
	}
	if storedHash := objectgraph.ContentHash("choir.agent", []byte(agentBody), json.RawMessage(agentMeta)); storedHash != agentHash {
		t.Fatalf("agent content_hash %s does not cover stored content (%s)", agentHash, storedHash)
	}

	// The V1 event object's embedded reference and actor profile converge.
	var eventID, eventBody, eventMeta, eventHash string
	if err := s.db.QueryRowContext(ctx,
		`SELECT canonical_id, body, metadata, content_hash FROM og_objects WHERE object_kind='choir.event'`).
		Scan(&eventID, &eventBody, &eventMeta, &eventHash); err != nil {
		t.Fatalf("read event row: %v", err)
	}
	if !strings.Contains(eventBody, expect["agent_v2"]) || strings.Contains(eventBody, expect["agent_v1"]) {
		t.Fatalf("event body ref not rewritten through the alias ledger: %s", eventBody)
	}
	if !strings.Contains(eventBody, `"actor_profile":"engineering"`) {
		t.Fatalf("event body profile not upcast: %s", eventBody)
	}
	if wantID, err := objectgraph.BuildCanonicalID("choir.event", vocabUpcastOwnerID, objectgraph.StableSuffixFromContent(eventHash)); err != nil {
		t.Fatal(err)
	} else if eventID != wantID {
		t.Fatalf("event canonical_id = %s, want content-derived %s", eventID, wantID)
	}
	if storedHash := objectgraph.ContentHash("choir.event", []byte(eventBody), json.RawMessage(eventMeta)); storedHash != eventHash {
		t.Fatalf("event content_hash %s does not cover stored content (%s)", eventHash, storedHash)
	}

	// The edge resolves both endpoints and re-derives its ID.
	var edgeID, edgeFrom, edgeTo string
	if err := s.db.QueryRowContext(ctx, `SELECT edge_id, from_id, to_id FROM og_edges`).Scan(&edgeID, &edgeFrom, &edgeTo); err != nil {
		t.Fatalf("read edge row: %v", err)
	}
	if edgeFrom != expect["agent_v2"] || edgeTo != eventID {
		t.Fatalf("edge endpoints = (%s, %s), want (%s, %s)", edgeFrom, edgeTo, expect["agent_v2"], eventID)
	}
	if wantEdge, err := objectgraph.BuildEdgeID(edgeFrom, edgeTo, "produced", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	} else if edgeID != wantEdge {
		t.Fatalf("edge_id = %s, want %s", edgeID, wantEdge)
	}

	// Alias ledger: the frozen upcast version, active vocabulary, and the
	// three identity renames are durable and consumer-visible.
	ledger := vocabUpcastReadLedger(t, s)
	if ledger == nil {
		t.Fatal("deposit upcast ledger missing after fresh replay")
	}
	if ledger.Version != vocabmigrate.DepositUpcastVersion || ledger.Active != vocabmigrate.VocabularyV2 {
		t.Fatalf("ledger header = v%d/%s, want v%d/%s", ledger.Version, ledger.Active, vocabmigrate.DepositUpcastVersion, vocabmigrate.VocabularyV2)
	}
	if got := ledger.Objects[expect["agent_v1"]]; got != expect["agent_v2"] {
		t.Fatalf("ledger agent alias = %q, want %s", got, expect["agent_v2"])
	}
	if got := ledger.Objects[expect["event_v1"]]; got != eventID {
		t.Fatalf("ledger event alias = %q, want %s", got, eventID)
	}
	if got := ledger.Edges[expect["edge_v1"]]; got != edgeID {
		t.Fatalf("ledger edge alias = %q, want %s", got, edgeID)
	}

	// Raw tape rows and receipts are untouched.
	vocabUpcastAssertRawTapeUntouched(t, s, tape)

	// The end-of-replay migration is verify-only: nothing left to migrate,
	// the fence passes, and the upcast version is visible in the report.
	before := vocabUpcastOGSnapshot(t, s)
	rep, err := s.MigrateAndFenceServingVocabulary(ctx, true, nil)
	if err != nil {
		t.Fatalf("migrate+fence: %v", err)
	}
	if len(rep.OGObjects) != 0 || len(rep.OGEdges) != 0 || len(rep.OGDropped) != 0 || len(rep.OGSuperseded) != 0 {
		t.Fatalf("verify-only migration planned writes: objects=%d edges=%d dropped=%v superseded=%d",
			len(rep.OGObjects), len(rep.OGEdges), rep.OGDropped, len(rep.OGSuperseded))
	}
	if rep.DepositUpcastVersion != vocabmigrate.DepositUpcastVersion {
		t.Fatalf("report deposit upcast version = %d, want %d", rep.DepositUpcastVersion, vocabmigrate.DepositUpcastVersion)
	}
	if err := s.VerifyServingVocabularyV2(ctx, nil); err != nil {
		t.Fatalf("serving fence: %v", err)
	}
	vocabUpcastAssertSnapshotsEqual(t, "og_objects", before["og_objects"], vocabUpcastOGSnapshot(t, s)["og_objects"])
	vocabUpcastAssertSnapshotsEqual(t, "og_edges", before["og_edges"], vocabUpcastOGSnapshot(t, s)["og_edges"])
}

// TestVocabDepositUpcastGenesisMatchesBaseTailReplay proves the policy's
// release condition: full genesis replay and base-plus-tail replay of the same
// tape produce byte-identical projections, tape rows, heads and upcast
// ledgers.
func TestVocabDepositUpcastGenesisMatchesBaseTailReplay(t *testing.T) {
	ctx := context.Background()
	tape, expect := vocabUpcastBuildTape(t)
	const baseEvents = 5 // genesis + 4 batches: V1 lifecycle + V1 event + V1 edge + V1 update

	base, basePath := vocabUpcastOpenStore(t, "base")
	vocabUpcastReplayTape(t, base, tape, 0, baseEvents)
	if _, err := base.MigrateAndFenceServingVocabulary(ctx, true, nil); err != nil {
		t.Fatalf("base migrate+fence: %v", err)
	}
	// The published base is already mono-vocabulary (the prerequisite for a
	// compatible base seed).
	var baseAgentID string
	if err := base.db.QueryRowContext(ctx, `SELECT canonical_id FROM og_objects WHERE object_kind='choir.agent'`).Scan(&baseAgentID); err != nil {
		t.Fatalf("read base agent: %v", err)
	}
	if baseAgentID != expect["agent_v2"] {
		t.Fatalf("base agent = %s, want %s", baseAgentID, expect["agent_v2"])
	}
	if err := base.Close(); err != nil {
		t.Fatalf("close base: %v", err)
	}

	// The base is published (marker + workspace, sidecar reports included) and
	// the tail is replayed onto the staged copy — the rebuild-with-base path.
	tailPath := vocabUpcastStorePath(t, "base-tail")
	marker, err := os.ReadFile(basePath)
	if err != nil {
		t.Fatalf("read base marker: %v", err)
	}
	if err := os.WriteFile(tailPath, marker, 0o644); err != nil {
		t.Fatalf("write tail marker: %v", err)
	}
	vocabUpcastCopyTree(t, deriveTextureWorkspacePath(basePath), deriveTextureWorkspacePath(tailPath))
	tail, err := Open(tailPath)
	if err != nil {
		t.Fatalf("open base+tail store: %v", err)
	}
	defer func() { _ = tail.Close() }()

	genesis, _ := vocabUpcastOpenStore(t, "genesis")
	vocabUpcastReplayTape(t, genesis, tape, 0, len(tape))

	vocabUpcastReplayTape(t, tail, tape, baseEvents, len(tape))
	if _, err := tail.MigrateAndFenceServingVocabulary(ctx, true, nil); err != nil {
		t.Fatalf("tail migrate+fence: %v", err)
	}
	if _, err := genesis.MigrateAndFenceServingVocabulary(ctx, true, nil); err != nil {
		t.Fatalf("genesis migrate+fence: %v", err)
	}

	genesisSnapshot := vocabUpcastOGSnapshot(t, genesis)
	vocabUpcastAssertSnapshotsEqual(t, "og_objects", genesisSnapshot["og_objects"], vocabUpcastOGSnapshot(t, tail)["og_objects"])
	vocabUpcastAssertSnapshotsEqual(t, "og_edges", genesisSnapshot["og_edges"], vocabUpcastOGSnapshot(t, tail)["og_edges"])
	vocabUpcastAssertSnapshotsEqual(t, "tape rows", vocabUpcastTapeRowSnapshot(t, genesis), vocabUpcastTapeRowSnapshot(t, tail))

	genesisHead, err := genesis.Head(ctx, vocabUpcastComputerID)
	if err != nil {
		t.Fatal(err)
	}
	tailHead, err := tail.Head(ctx, vocabUpcastComputerID)
	if err != nil {
		t.Fatal(err)
	}
	if genesisHead == nil || tailHead == nil || *genesisHead != *tailHead {
		t.Fatalf("heads differ:\n genesis: %+v\n    tail: %+v", genesisHead, tailHead)
	}

	// Consumer-visible compatibility state agrees too.
	genesisLedger := vocabUpcastReadLedger(t, genesis)
	tailLedger := vocabUpcastReadLedger(t, tail)
	if genesisLedger == nil || tailLedger == nil {
		t.Fatalf("ledger missing: genesis=%v tail=%v", genesisLedger != nil, tailLedger != nil)
	}
	if genesisLedger.Version != tailLedger.Version || genesisLedger.Active != tailLedger.Active ||
		len(genesisLedger.Objects) != len(tailLedger.Objects) || len(genesisLedger.Edges) != len(tailLedger.Edges) {
		t.Fatalf("ledger mismatch:\n genesis: v%d/%s objects=%d edges=%d\n    tail: v%d/%s objects=%d edges=%d",
			genesisLedger.Version, genesisLedger.Active, len(genesisLedger.Objects), len(genesisLedger.Edges),
			tailLedger.Version, tailLedger.Active, len(tailLedger.Objects), len(tailLedger.Edges))
	}
	for old, current := range genesisLedger.Objects {
		if tailLedger.Objects[old] != current {
			t.Fatalf("ledger object alias %s = %q, want %q", old, tailLedger.Objects[old], current)
		}
	}
}

// TestVocabDepositUpcastRetainedV1StoreDefersToEndMigration pins the
// compatibility boundary: a retained V1 store (projection head present, no
// ledger, not cut over) keeps byte-identical replay deposits, and its
// one-time end-of-replay migration performs the cutover.
func TestVocabDepositUpcastRetainedV1StoreDefersToEndMigration(t *testing.T) {
	ctx := context.Background()
	s, _ := vocabUpcastOpenStore(t, "retained")
	base := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	agentV1 := vocabUpcastAgentID(t, "super:root")
	agentV2 := vocabUpcastAgentID(t, "management:root")

	// Seed the retained V1 row and a projection head so the store cannot be
	// mistaken for a fresh reconstruction.
	retained := vocabUpcastAgentObject(t, "super:root", "super", "a", false, base, base)
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by) VALUES (?,?,?,?,?,?,?,?,?,?,0,'')`,
		retained.CanonicalID, string(retained.ObjectKind), retained.OwnerID, retained.ComputerID, retained.VersionID,
		retained.ContentHash, retained.Body, string(retained.Metadata), base, base); err != nil {
		t.Fatalf("seed retained agent: %v", err)
	}
	seededHead := computerevent.Head{
		ComputerID: vocabUpcastComputerID, Sequence: 1, CanonicalEventHead: storeTestDigest('e'),
		DesiredEventHead: storeTestDigest('e'), EffectiveEventHead: storeTestDigest('e'),
		DesiredStateCommitment: storeTestDigest('f'), EffectiveStateCommitment: storeTestDigest('f'),
		ReducerVersion: computerevent.ReducerVersionV1,
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO computer_event_projection_heads (computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, pending_transition_ref, desired_state_commitment, effective_state_commitment, reducer_version, credential_revocation_epoch, updated_at) VALUES (?,?,?,?,?,NULL,?,?,?,?,?)`,
		seededHead.ComputerID, seededHead.Sequence, seededHead.CanonicalEventHead, seededHead.DesiredEventHead,
		seededHead.EffectiveEventHead, seededHead.DesiredStateCommitment, seededHead.EffectiveStateCommitment,
		seededHead.ReducerVersion, seededHead.CredentialRevocationEpoch, time.Now().UTC()); err != nil {
		t.Fatalf("seed retained head: %v", err)
	}

	// A V1 update replays byte-identically onto the retained V1 row.
	update := vocabUpcastAgentObject(t, "super:root", "super", "b", false, base, base.Add(time.Minute))
	event, input, next := vocabUpcastTestEvent(t, 2, seededHead.CanonicalEventHead, computerevent.EventProjectionBatchRecorded, &seededHead)
	vocabUpcastApplyTapeEvent(t, s, vocabUpcastTapeEvent{
		Event: event, Input: input, Next: next, Receipt: vocabUpcastReceipt(t, event),
		Ops: []computerevent.ProjectionOp{vocabUpcastObjectOp(t, update)},
	})

	var gotID, gotBody string
	if err := s.db.QueryRowContext(ctx, `SELECT canonical_id, body FROM og_objects WHERE object_kind='choir.agent'`).Scan(&gotID, &gotBody); err != nil {
		t.Fatal(err)
	}
	if gotID != agentV1 || !strings.Contains(gotBody, `"agent_id":"super:root"`) || !strings.Contains(gotBody, `"state":"b"`) {
		t.Fatalf("retained V1 deposit was not byte-identical: id=%s body=%s", gotID, gotBody)
	}
	if ledger := vocabUpcastReadLedger(t, s); ledger != nil {
		t.Fatalf("retained V1 store activated deposit upcasting: %+v", ledger)
	}

	// The one-time end-of-replay migration performs the cutover.
	rep, err := s.MigrateAndFenceServingVocabulary(ctx, true, nil)
	if err != nil {
		t.Fatalf("retained migrate+fence: %v", err)
	}
	if len(rep.OGObjects) != 1 {
		t.Fatalf("retained migration OGObjects = %d, want 1", len(rep.OGObjects))
	}
	if rep.DepositUpcastVersion != 0 {
		t.Fatalf("retained migration claimed deposit upcast version %d, want 0", rep.DepositUpcastVersion)
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM og_objects WHERE object_kind='choir.agent'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("agent rows after retained cutover = %d, want 1", count)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT canonical_id, body FROM og_objects WHERE object_kind='choir.agent'`).Scan(&gotID, &gotBody); err != nil {
		t.Fatal(err)
	}
	if gotID != agentV2 || !strings.Contains(gotBody, `"agent_id":"management:root"`) {
		t.Fatalf("retained cutover landed at %s (%s), want %s", gotID, gotBody, agentV2)
	}
}

// TestVocabDepositUpcastRefusesUnverifiableRekey pins the fail-loud boundary:
// a key-suffixed object whose identity-bearing fields migrate but whose kind
// has no verifiable identity formula must refuse the replay instead of being
// renamed by guesswork.
func TestVocabDepositUpcastRefusesUnverifiableRekey(t *testing.T) {
	ctx := context.Background()
	s, _ := vocabUpcastOpenStore(t, "refuse")
	base := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)

	genesisEvent, genesisInput, genesisNext := vocabUpcastTestEvent(t, 1, computerevent.ZeroHead, computerevent.EventGenesisImported, nil)
	vocabUpcastApplyTapeEvent(t, s, vocabUpcastTapeEvent{
		Event: genesisEvent, Input: genesisInput, Next: genesisNext, Receipt: vocabUpcastReceipt(t, genesisEvent),
	})

	// choir.event is a frozen kind without an identity formula; a key-suffixed
	// row carrying a migrated ID-shaped leaf cannot be safely re-derived.
	body, err := json.Marshal(map[string]any{"actor_profile": "co-super", "subject": "super:root"})
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := objectgraph.NormalizeMetadata(map[string]any{"actor_profile": "co-super"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := objectgraph.BuildCanonicalID("choir.event", vocabUpcastOwnerID, objectgraph.StableSuffixFromKey("event-subject-super:root"))
	if err != nil {
		t.Fatal(err)
	}
	obj := objectgraph.Object{
		CanonicalID: id, ObjectKind: "choir.event", OwnerID: vocabUpcastOwnerID,
		ComputerID: vocabUpcastComputerID, ContentHash: objectgraph.ContentHash("choir.event", body, metadata),
		Body: body, Metadata: metadata, CreatedAt: base, UpdatedAt: base,
	}
	event, input, next := vocabUpcastTestEvent(t, 2, genesisNext.CanonicalEventHead, computerevent.EventProjectionBatchRecorded, &genesisNext)
	digest, err := event.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Prepare(ctx, computerevent.CASRequest{Event: event, EventDigest: digest, EventArtifactDigest: digest, EventPinReceiptDigest: storeTestDigest('c'), Input: input, Next: next}); err != nil {
		t.Fatalf("prepare refusing batch: %v", err)
	}
	receipt := vocabUpcastReceipt(t, event)
	batch := computerevent.ProjectionBatch{
		Version: computerevent.ProjectionBatchV2, ProjectorVersion: computerevent.ProjectorVersionV2,
		ComputerID: vocabUpcastComputerID, EventID: event.EventID, EventDigest: digest,
		Ops: []computerevent.ProjectionOp{vocabUpcastObjectOp(t, obj)},
	}
	replayErr := s.FinalizeReplayBatch(ctx, vocabUpcastComputerID, digest, receipt, &batch)
	if replayErr == nil || !strings.Contains(replayErr.Error(), "no verified identity formula") {
		t.Fatalf("replay error = %v, want unverifiable-identity refusal", replayErr)
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM og_objects WHERE object_kind='choir.event'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("refused replay wrote %d choir.event rows", count)
	}
	head, err := s.Head(ctx, vocabUpcastComputerID)
	if err != nil {
		t.Fatal(err)
	}
	if head == nil || head.Sequence != 1 {
		t.Fatalf("head advanced past the refused batch: %+v", head)
	}
}

// TestVocabDepositUpcastConvergentSpellingsFollowEventOrder reproduces the
// 2026-10-07 rebuild defect class: two pre-cutover agent_id spellings that
// collapse onto one V2 identity ("co-super:x" and "cosuper:x" →
// "engineering:x"). The end-of-replay migration cannot order them and refuses
// divergent content; the deposit upcast knows the canonical event order, so
// the later deposit updates the one row — exactly what the live computer's
// post-cutover writer does — and the ending migration is verify-only.
func TestVocabDepositUpcastConvergentSpellingsFollowEventOrder(t *testing.T) {
	ctx := context.Background()
	s, _ := vocabUpcastOpenStore(t, "converge")
	base := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	v1ID := vocabUpcastAgentID(t, "co-super:impl")
	v1AltID := vocabUpcastAgentID(t, "cosuper:impl")
	v2ID := vocabUpcastAgentID(t, "engineering:impl")
	if v1ID == v1AltID || v2ID == v1ID || v2ID == v1AltID {
		t.Fatalf("test requires distinct recorded IDs (v1=%s alt=%s v2=%s)", v1ID, v1AltID, v2ID)
	}
	tape := vocabUpcastTapeFromBatches(t, [][]computerevent.ProjectionOp{
		{vocabUpcastObjectOp(t, vocabUpcastAgentObject(t, "co-super:impl", "co-super", "first", false, base, base))},
		{vocabUpcastObjectOp(t, vocabUpcastAgentObject(t, "cosuper:impl", "cosuper", "second", false, base, base.Add(time.Minute)))},
		{vocabUpcastObjectOp(t, vocabUpcastAgentObject(t, "engineering:impl", "engineering", "third", false, base, base.Add(2*time.Minute)))},
	})
	vocabUpcastReplayTape(t, s, tape, 0, len(tape))

	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM og_objects WHERE object_kind='choir.agent'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("agent rows = %d, want 1 (convergent spellings collapse to one identity)", count)
	}
	var agentID, agentBody, createdAt string
	if err := s.db.QueryRowContext(ctx,
		`SELECT canonical_id, body, created_at FROM og_objects WHERE object_kind='choir.agent'`).Scan(&agentID, &agentBody, &createdAt); err != nil {
		t.Fatal(err)
	}
	if agentID != v2ID {
		t.Fatalf("agent canonical_id = %s, want %s", agentID, v2ID)
	}
	if !strings.Contains(agentBody, `"state":"third"`) || !strings.Contains(agentBody, `"agent_id":"engineering:impl"`) {
		t.Fatalf("event order did not decide the survivor content: %s", agentBody)
	}
	if !strings.Contains(createdAt, "2026-10-07") || !strings.Contains(createdAt, "09:00:00") {
		t.Fatalf("created_at = %q, want the first deposit's instant (no delete/recreate)", createdAt)
	}

	ledger := vocabUpcastReadLedger(t, s)
	if ledger == nil {
		t.Fatal("deposit upcast ledger missing after fresh replay")
	}
	if ledger.Objects[v1ID] != v2ID || ledger.Objects[v1AltID] != v2ID {
		t.Fatalf("ledger convergence aliases = %q/%q, want both → %s", ledger.Objects[v1ID], ledger.Objects[v1AltID], v2ID)
	}

	rep, err := s.MigrateAndFenceServingVocabulary(ctx, true, nil)
	if err != nil {
		t.Fatalf("migrate+fence after convergent deposits: %v", err)
	}
	if len(rep.OGObjects) != 0 || len(rep.OGDropped) != 0 || len(rep.OGSuperseded) != 0 {
		t.Fatalf("convergent replay still needed migration writes: objects=%d dropped=%v superseded=%d",
			len(rep.OGObjects), rep.OGDropped, len(rep.OGSuperseded))
	}
	if err := s.VerifyServingVocabularyV2(ctx, nil); err != nil {
		t.Fatalf("serving fence: %v", err)
	}
}

// TestVocabDepositUpcastRefusesInvalidLedgerOnEveryCall pins the frozen-candidate
// panel finding (2026-10-08): a ledger that fails validation must refuse every
// replay attempt in the process, not only the first; a cached "loaded" flag
// let a same-process retry skip validation and append to the bad ledger.
func TestVocabDepositUpcastRefusesInvalidLedgerOnEveryCall(t *testing.T) {
	s, _ := vocabUpcastOpenStore(t, "badledger")
	path := s.depositUpcastLedgerPath()
	if path == "" {
		t.Fatal("store has no deposit upcast ledger path")
	}
	if err := os.WriteFile(path, []byte(`{"version":999,"active":"v2"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		if err := s.upcastProjectionDeposit(nil, true, true, true); err == nil || !strings.Contains(err.Error(), "unsupported version") {
			t.Fatalf("replay attempt %d: err=%v, want unsupported-version refusal", attempt, err)
		}
		if err := s.upcastProjectionDeposit(nil, false, false, false); err == nil {
			t.Fatalf("dry-run attempt %d accepted an invalid ledger", attempt)
		}
	}
}
