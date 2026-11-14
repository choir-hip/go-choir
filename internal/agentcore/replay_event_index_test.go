package agentcore

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

func TestCompareReplayEventIndexNamesRowsAndColumns(t *testing.T) {
	row := func(digest string, seq int64, kind, status, rowDigest string, fields map[string]string) choirstore.EventIndexFingerprint {
		return choirstore.EventIndexFingerprint{EventDigest: digest, Sequence: seq, EventKind: kind, Status: status, RowDigest: rowDigest, FieldDigests: fields}
	}
	live := []choirstore.EventIndexFingerprint{
		row("a", 1, "k", "finalized", "r1", map[string]string{"x": "1", "y": "1"}),
		row("b", 2, "k", "finalized", "r2", map[string]string{"x": "1", "y": "1"}),
		row("c", 3, "j", "prepared", "r3", map[string]string{"x": "1"}),
	}
	replay := []choirstore.EventIndexFingerprint{
		row("a", 1, "k", "finalized", "r1", map[string]string{"x": "1", "y": "1"}),
		row("b", 2, "k", "finalized", "r2x", map[string]string{"x": "1", "y": "2"}),
		row("d", 9, "m", "finalized", "r4", map[string]string{"x": "1"}),
	}
	got := compareReplayEventIndex(live, replay, 3, 0, 40000)
	if got == nil {
		t.Fatal("differing indexes produced no comparison")
	}
	if got.LiveOnlyCount != 1 || got.ReplayOnlyCount != 1 || got.DifferentCount != 1 || got.LiveCount != 3 || got.ReplayCount != 3 {
		t.Fatalf("counts = %+v", got)
	}
	if got.LiveOnlyByKind["j"] != 1 || got.LiveOnlyByStatus["prepared"] != 1 || got.ReplayOnlyByKind["m"] != 1 || got.DifferentByField["y"] != 1 {
		t.Fatalf("breakdowns = %+v", got)
	}
	if got.LiveOnlySeqMin != 3 || got.LiveOnlySeqMax != 3 || got.ReplayOnlySeqMin != 9 || got.ReplayOnlySeqMax != 9 {
		t.Fatalf("sequence ranges = %+v", got)
	}
	if got.LiveDisposableRows != 3 || got.TrimBoundary != 40000 {
		t.Fatalf("trim context = %+v", got)
	}
	var sawDifferent bool
	for _, s := range got.Samples {
		if s.Kind == "different" && s.EventDigest == "b" && len(s.DifferentFields) == 1 && s.DifferentFields[0] == "y" {
			sawDifferent = true
		}
	}
	if !sawDifferent || len(got.Samples) != 3 {
		t.Fatalf("samples = %+v", got.Samples)
	}
	if compareReplayEventIndex(live, live, 0, 0, 0) != nil {
		t.Fatal("equal indexes must produce no comparison")
	}
	many := make([]choirstore.EventIndexFingerprint, 100)
	for i := range many {
		many[i] = row(string(rune('A'+i%26))+strings.Repeat("x", i), int64(i), "k", "finalized", "r", nil)
	}
	if c := compareReplayEventIndex(many, nil, 0, 0, 0); len(c.Samples) != replayEventIndexSampleLimit || c.LiveOnlyCount != 100 {
		t.Fatalf("sample bound: %d samples, %d live-only", len(c.Samples), c.LiveOnlyCount)
	}
}

// The probe's extractor reads the index the way the trim defines it: rows of
// the trimmed kind at or below the boundary are disposable, and JSON null in
// the pin list is the empty list. The stored witness extractor does neither.
func TestReplayProbeExtractorIgnoresTrimBacklogAndNullPinList(t *testing.T) {
	ctx := context.Background()
	open := func() *choirstore.Store {
		s, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.Close() })
		return s
	}
	insert := func(s *choirstore.Store, digest string, seq int, kind, pins string) {
		t.Helper()
		_, err := s.DB().Exec(`INSERT INTO computer_event_index (event_digest, computer_id, sequence, previous_head, event_kind, event_artifact_digest, event_pin_receipt_digest, payload_pin_receipt_digests_json, request_commitment, idempotency_key, status, next_desired_event_head, next_effective_event_head, next_desired_state_commitment, next_effective_state_commitment, next_reducer_version, next_credential_revocation_epoch, prepared_at)
			VALUES (?, 'c', ?, 'p', ?, 'a', 'b', ?, 'r', ?, 'finalized', 'd', 'e', 'c', 'f', 1, 0, ?)`, digest, seq, kind, pins, "key-"+digest, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
	}
	live, replay := open(), open()
	batch := string(computerevent.EventProjectionBatchRecorded)
	insert(live, "kept", 45000, "k", "[]")
	insert(replay, "kept", 45000, "k", "null")
	insert(live, "backlog", 100, batch, "[]") // a base copied before the catch-up
	const head = 50000
	digest := func(s *choirstore.Store, probe bool) string {
		e := replayStateExtractor(s.ReadDB())
		if probe {
			e = replayProbeStateExtractor(s.ReadDB(), head)
		}
		d, err := e.Digests(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return d.Tables["computer_event_index"]
	}
	if digest(live, false) == digest(replay, false) {
		t.Fatal("precondition: the witness extractor distinguishes these stores")
	}
	if digest(live, true) != digest(replay, true) {
		t.Fatal("the probe extractor still sees the trim backlog or null pin list")
	}
	insert(replay, "extra", 46000, "k", "[]")
	if digest(live, true) == digest(replay, true) {
		t.Fatal("the probe extractor hid a row above the boundary")
	}
}

type probeFixture struct {
	rt  *Runtime
	cas *replayEventCAS
}

// newProbeFixture appends a genesis and, when tail is set, a no-payload
// desk event after it, then installs the base snapshot at the genesis so the
// probe replays the second event as a tail (failure 3).
func newProbeFixture(t *testing.T, computerID string, tail bool) *probeFixture {
	t.Helper()
	ctx := context.Background()
	storePath := filepath.Join(t.TempDir(), "runtime.db")
	liveStore, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = liveStore.Close() })
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signingKey := computerevent.SigningKey{
		SignerRef:  computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "replay-test"},
		PrivateKey: privateKey,
	}
	cas := &replayEventCAS{key: signingKey, projection: liveStore}
	appender, err := computerevent.NewComputerEventAppender(computerID, rollbackTestPinner{signingKey}, liveStore, cas, rollbackTestReceiptVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	newEvent := func(kind computerevent.EventKind, key, commitment, actor string) computerevent.Event {
		eventID, err := computerevent.NewEventID()
		if err != nil {
			t.Fatal(err)
		}
		return computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: computerID, EventKind: kind,
			OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), IdempotencyKey: key, ActorProfile: actor,
			AuthorityRef: "owner", PrivacyClass: "owner", PayloadCommitment: strings.Repeat("a", 64),
			ProposedEffectRef: strings.Repeat("b", 64), ResultingEffectiveCommitment: commitment,
			ReducerVersion: computerevent.ReducerVersionV1,
		}
	}
	a := strings.Repeat("a", 64)
	if _, err := appender.AppendNew(ctx, newEvent(computerevent.EventGenesisImported, "genesis", a, "management"), computerevent.TransitionInput{TargetStateCommitment: a}, nil); err != nil {
		t.Fatal(err)
	}
	if tail {
		f := strings.Repeat("f", 64)
		update := newEvent(computerevent.EventResearchUpdate, "desk-update", f, "research")
		update.PayloadCommitment = strings.Repeat("0", 64)
		if _, err := appender.AppendNew(ctx, update, computerevent.TransitionInput{TargetStateCommitment: f}, nil); err != nil {
			t.Fatal(err)
		}
	}
	rt := &Runtime{
		cfg:           provideriface.Config{ComputerID: computerID, StorePath: storePath},
		store:         liveStore,
		eventAppender: appender,
	}
	// The production tape (platform/event_replay.go) rebuilds a request from
	// its receipt and leaves PayloadPinReceiptDigests nil when the event has
	// no payload pins; the live appender hands the projection an empty slice.
	for i := range cas.events {
		if len(cas.events[i].Request.PayloadPinReceiptDigests) == 0 {
			cas.events[i].Request.PayloadPinReceiptDigests = nil
		}
	}
	seedRestoreBase(t, ctx, rt, cas, computerID, 1)
	return &probeFixture{rt: rt, cas: cas}
}

// Failure 3: the live append stores "[]" for an event with no payload pins and
// replay stored null, so a tail event past the base made the probe answer
// not_equivalent on the event index alone.
func TestReplayCompletenessTailEventWithoutPayloadPinsIsEquivalent(t *testing.T) {
	fx := newProbeFixture(t, "computer-replay-tail-nopins", true)
	report, err := fx.rt.ReplayCompleteness(context.Background(), "computer-replay-tail-nopins")
	if err != nil {
		t.Fatal(err)
	}
	if report.TailEventsApplied != 1 {
		t.Fatalf("tail events applied = %d, want 1", report.TailEventsApplied)
	}
	if !report.Result.Equivalent() {
		t.Fatalf("replay of a no-payload tail event drifted: %#v", report.Result.Differences)
	}
	if report.EventIndex != nil {
		t.Fatalf("an equivalent report carries an event index comparison: %+v", report.EventIndex)
	}
}

// Failure 4 end to end: when the result is not equivalent the report names
// the event index row and column.
func TestReplayCompletenessReportNamesTheDifferingEventIndexColumn(t *testing.T) {
	fx := newProbeFixture(t, "computer-replay-index-diff", true)
	if _, err := fx.rt.store.DB().Exec(`UPDATE computer_event_index SET next_reducer_version = 99 WHERE sequence = 1`); err != nil {
		t.Fatal(err)
	}
	report, err := fx.rt.ReplayCompleteness(context.Background(), "computer-replay-index-diff")
	if err != nil {
		t.Fatal(err)
	}
	if report.Result.Equivalent() {
		t.Fatal("a changed live index row must not be equivalent")
	}
	if report.SchemaVersion < 5 {
		t.Fatalf("schema version = %d, want it bumped past 4", report.SchemaVersion)
	}
	got := report.EventIndex
	if got == nil || got.DifferentCount != 1 || got.LiveOnlyCount != 0 || got.ReplayOnlyCount != 0 {
		t.Fatalf("event index comparison = %+v", got)
	}
	if len(got.Samples) != 1 || got.Samples[0].Sequence != 1 || len(got.Samples[0].DifferentFields) != 1 || got.Samples[0].DifferentFields[0] != "next_reducer_version" {
		t.Fatalf("samples = %+v", got.Samples)
	}
}
