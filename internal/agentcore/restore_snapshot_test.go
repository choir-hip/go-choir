package agentcore

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/recoveryplan"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/snapshot"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// restoreTapeFake is the recovery source of the tests: the chain the test
// runtime appended and, when seeded, one snapshot the tape names. The snapshot
// is presented by rewriting the chain's event at snapshotSeq into a
// snapshot_recorded event, so the scan sees it where the tape resolves the
// target; replay itself reads the chain the appender holds, unchanged.
type restoreTapeFake struct {
	events func() []computerevent.DurableEvent

	mu              sync.Mutex
	snapshotSeq     uint64
	snapshotEventID string
	snapshotDigest  string
	payloads        map[string][]byte
	chunks          map[string][]byte
	record          snapshot.Record
	foreignEnvelope []byte

	tapeErr       error
	foreign       bool
	corruptChunks bool
	chunkReads    int
}

func newRestoreTapeFake(events func() []computerevent.DurableEvent) *restoreTapeFake {
	return &restoreTapeFake{events: events, payloads: map[string][]byte{}, chunks: map[string][]byte{}}
}

func (f *restoreTapeFake) EventsPage(_ context.Context, _ string, after uint64, size int) ([]computerevent.DurableEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.tapeErr != nil {
		return nil, f.tapeErr
	}
	page := []computerevent.DurableEvent{}
	for _, record := range f.events() {
		if record.Request.Event.Sequence <= after {
			continue
		}
		if f.snapshotDigest != "" && record.Request.Event.Sequence == f.snapshotSeq {
			record.Request.Event.EventKind = computerevent.EventSnapshotRecorded
			record.Request.Event.EventID = f.snapshotEventID
			record.Request.Event.PayloadCommitment = f.snapshotDigest
		}
		page = append(page, record)
		if size > 0 && len(page) >= size {
			break
		}
	}
	return page, nil
}

func (f *restoreTapeFake) FetchPayload(_ context.Context, _, digest string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.foreign && f.foreignEnvelope != nil {
		return f.foreignEnvelope, nil
	}
	data, ok := f.payloads[digest]
	if !ok {
		return nil, errors.New("restore tape fake: payload not found")
	}
	return data, nil
}

func (f *restoreTapeFake) PutChunk(_ context.Context, digest string, sealed []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.chunks[digest] = append([]byte(nil), sealed...)
	return nil
}

func (f *restoreTapeFake) GetChunk(_ context.Context, digest string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.chunkReads++
	data, ok := f.chunks[digest]
	if !ok {
		return nil, errors.New("restore tape fake: chunk not found")
	}
	data = append([]byte(nil), data...)
	if f.corruptChunks {
		data[len(data)-1] ^= 0xff
	}
	return data, nil
}

func (f *restoreTapeFake) chunkReadCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.chunkReads
}

var _ restoreTape = (*restoreTapeFake)(nil)

// snapshotCapture is the appender TakeSnapshot writes through in tests: it
// seals nothing itself, it records the event and the sealed envelope.
type snapshotCapture struct {
	event    computerevent.Event
	envelope []byte
	digest   string
}

func (s *snapshotCapture) HoldAppends(fn func() error) error { return fn() }

func (s *snapshotCapture) AppendNewPrivatePayload(ctx context.Context, event computerevent.Event, _ computerevent.TransitionInput, plaintext []byte, mediaType string, cipher *computerevent.PrivateArtifactCipher) (computerevent.Receipt, string, error) {
	envelope, _, err := cipher.Encrypt(ctx, event.ComputerID, event.EventID, mediaType, "private", plaintext)
	if err != nil {
		return computerevent.Receipt{}, "", err
	}
	s.event, s.envelope, s.digest = event, envelope, computerevent.DigestBytes(envelope)
	return computerevent.Receipt{}, s.digest, nil
}

// ensureTestCipher gives the runtime a private-artifact cipher when the
// fixture did not.
func ensureTestCipher(t *testing.T, rt *Runtime, computerID string) {
	t.Helper()
	if rt.privateArtifactCipher != nil {
		return
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	cipher, err := computerevent.NewPrivateArtifactCipher(computerID, key)
	if err != nil {
		t.Fatal(err)
	}
	rt.privateArtifactCipher = cipher
}

// memChainSource replays the test chain verbatim: the retained durable events
// carry their original receipts, so a rebuilt store stays witness-equivalent
// with the live store.
type memChainSource struct {
	events     []computerevent.DurableEvent
	head       *computerevent.Head
	computerID string
}

func (s *memChainSource) EventsPage(ctx context.Context, computerID string, afterSequence uint64, pageSize int) ([]computerevent.DurableEvent, error) {
	var page []computerevent.DurableEvent
	for _, record := range s.events {
		if record.Request.Event.Sequence <= afterSequence {
			continue
		}
		page = append(page, record)
		if pageSize > 0 && len(page) >= pageSize {
			break
		}
	}
	if page == nil {
		return []computerevent.DurableEvent{}, nil
	}
	return page, nil
}

func (s *memChainSource) Events(ctx context.Context, computerID string, afterSequence uint64) ([]computerevent.DurableEvent, error) {
	return s.EventsPage(ctx, computerID, afterSequence, 0)
}

func (s *memChainSource) Head(ctx context.Context, computerID string) (*computerevent.Head, error) {
	return s.head, nil
}

func (s *memChainSource) CompareAndSwap(ctx context.Context, request computerevent.CASRequest) (computerevent.Receipt, error) {
	return computerevent.Receipt{}, errors.New("mem chain source: offline rebuilder cannot CAS events")
}

func (s *memChainSource) PinEvent(ctx context.Context, computerID string, canonicalEvent []byte, requestCommitment string) (computerevent.PinResult, error) {
	return computerevent.PinResult{ArtifactDigest: computerevent.DigestBytes(canonicalEvent)}, nil
}

func (s *memChainSource) PinEventPayload(ctx context.Context, computerID, eventID string, payload []byte, mediaType, privacyClass, requestCommitment string) (computerevent.PinResult, error) {
	return computerevent.PinResult{ArtifactDigest: computerevent.DigestBytes(payload)}, nil
}

func (s *memChainSource) FetchPayload(ctx context.Context, computerID, artifactDigest string) ([]byte, error) {
	return nil, errors.New("mem chain source: fixture chains carry no payloads")
}

func (s *memChainSource) VerifyEventHeadReceipt(ctx context.Context, receipt computerevent.Receipt, request computerevent.CASRequest) error {
	return nil
}

// seedRestoreBase takes a real snapshot of the store replayed to baseSeq over
// the test chain retained by cas, and installs a tape that names it as the
// runtime's restore source. The genesis never carries payload references, so a
// W=1 snapshot always builds; later events replay as the tail through the
// production loop. (The name is historical: this was the projection base.)
func seedRestoreBase(t *testing.T, ctx context.Context, rt *Runtime, cas *replayEventCAS, computerID string, baseSeq int) snapshot.Record {
	t.Helper()
	if len(cas.events) < baseSeq {
		t.Fatalf("chain has %d events, want at least %d for the snapshot", len(cas.events), baseSeq)
	}
	var target string
	for _, record := range cas.events {
		raw, err := record.Request.Event.CanonicalBytes()
		if err != nil {
			t.Fatal(err)
		}
		if computerevent.DigestBytes(raw) != record.Request.EventDigest {
			t.Fatalf("fixture envelope digest mismatch at seq %d", record.Request.Event.Sequence)
		}
		if record.Request.Event.Sequence == uint64(baseSeq) {
			target = record.Request.EventDigest
		}
	}
	if target == "" {
		t.Fatalf("no event at sequence %d", baseSeq)
	}
	ensureTestCipher(t, rt, computerID)

	// The store at the snapshot point: the chain replayed to baseSeq into a
	// scratch store in the live store's deposit mode, as restore does.
	scratch, err := choirstore.OpenFresh(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = scratch.Close() }()
	if _, err := matchLiveDepositMode(rt.store, scratch); err != nil {
		t.Fatal(err)
	}
	if err := rt.eventAppender.ReconstructThroughTarget(ctx, scratch, target); err != nil {
		t.Fatalf("build test snapshot at %d: %v", baseSeq, err)
	}
	fake := newRestoreTapeFake(func() []computerevent.DurableEvent { return cas.events })
	capture := &snapshotCapture{}
	ref, err := snapshot.TakeSnapshot(ctx, snapshot.Config{
		ComputerID: computerID,
		DB:         scratch.DB(),
		TempDir:    t.TempDir(),
		Head: func(ctx context.Context, db *sql.DB) (*computerevent.Head, error) {
			return choirstore.ReadProjectionHead(ctx, db, computerID)
		},
		Witness: func(ctx context.Context, db *sql.DB) (selfdevprotocol.VMLocalContentWitness, error) {
			return LiveStoreWitness(ctx, db)
		},
		Cipher:   rt.privateArtifactCipher,
		Chunks:   fake,
		Appender: capture,
	})
	if err != nil {
		t.Fatalf("take test snapshot at %d: %v", baseSeq, err)
	}
	if ref.Record.StoreHeadSequence != uint64(baseSeq) {
		t.Fatalf("snapshot captured sequence %d, want %d", ref.Record.StoreHeadSequence, baseSeq)
	}
	fake.snapshotSeq, fake.snapshotEventID, fake.snapshotDigest = uint64(baseSeq), ref.EventID, ref.ArtifactDigest
	fake.payloads[ref.ArtifactDigest] = capture.envelope
	fake.record = ref.Record
	// The same record sealed for another computer: a foreign snapshot.
	foreignKey := make([]byte, 32)
	if _, err := rand.Read(foreignKey); err != nil {
		t.Fatal(err)
	}
	foreignCipher, err := computerevent.NewPrivateArtifactCipher("computer-foreign-base", foreignKey)
	if err != nil {
		t.Fatal(err)
	}
	foreign, _, err := foreignCipher.Encrypt(ctx, "computer-foreign-base", ref.EventID, snapshot.RecordMediaType, "private", mustJSON(t, ref.Record))
	if err != nil {
		t.Fatal(err)
	}
	fake.foreignEnvelope = foreign
	rt.restoreTape = fake
	return ref.Record
}

// TestReplayCompletenessSnapshotlessChain proves a computer whose tape holds
// no snapshot still produces a witness: the probe replays the full tape into a
// disposable store and compares live vs replayed state. Fresh machines must be
// able to checkpoint; long chains still refuse through MaxRecoveryTailEvents.
func TestReplayCompletenessSnapshotlessChain(t *testing.T) {
	computerID := "computer-replay-snapshotless"
	storePath := filepath.Join(t.TempDir(), "runtime.db")
	live, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = live.Close() }()
	rt, cas, _ := rematerializeTapeRuntime(t, computerID, storePath, live)
	rt.restoreTape = newRestoreTapeFake(func() []computerevent.DurableEvent { return cas.events })

	report, err := rt.ReplayCompleteness(context.Background(), computerID)
	if err != nil {
		t.Fatalf("snapshotless replay completeness refused: %v", err)
	}
	if report.BaseSequence != 0 || report.BaseBlobSHA256 != "" {
		t.Fatalf("snapshotless probe reported a snapshot: seq=%d root=%s", report.BaseSequence, report.BaseBlobSHA256)
	}
	if !report.Result.Equivalent() {
		t.Fatalf("snapshotless full replay was not equivalent: %#v", report.Result)
	}
}

// rematerializeSeededWithFrontend builds a runtime with a genesis chain, a
// seeded W=1 snapshot, pinned frontend releases, and a valid checkpoint, ready
// for rematerialize refusal-matrix tests.
func rematerializeSeededWithFrontend(t *testing.T, computerID string) (*Runtime, *restoreTapeFake, selfdevprotocol.Checkpoint, string) {
	t.Helper()
	storePath := filepath.Join(t.TempDir(), "runtime.db")
	live, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = live.Close() })
	rt, cas, acceptedHead := rematerializeTapeRuntime(t, computerID, storePath, live)
	seedRestoreBase(t, context.Background(), rt, cas, computerID, 1)
	updaterRoot := filepath.Join(t.TempDir(), "updater")
	priorDigest, _ := pinFrontendRelease(t, updaterRoot, computerID, "<html>live</html>")
	targetDigest, targetIdentity := pinFrontendRelease(t, updaterRoot, computerID, "<html>checkpoint</html>")
	pointCurrent(t, updaterRoot, priorDigest)
	rt.selfdevUpdaterRoot = updaterRoot
	ctx := context.Background()
	report, err := rt.ReplayCompleteness(ctx, computerID)
	if err != nil {
		t.Fatalf("seeded probe refused: %v", err)
	}
	if report.BaseSequence != 1 {
		t.Fatalf("probe snapshot = %d, want 1", report.BaseSequence)
	}
	witness, err := selfdevprotocol.WitnessFromObservationSets(report.Live, report.Replay, report.Result)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := rematerializeTestCheckpoint(t, computerID, witness, targetDigest, targetIdentity, acceptedHead)
	fake, ok := rt.restoreTape.(*restoreTapeFake)
	if !ok {
		t.Fatal("seeded source is not a test fake")
	}
	return rt, fake, checkpoint, storePath
}

func TestRematerializeRefusesFailureClasses(t *testing.T) {
	ctx := context.Background()
	t.Run("an absent snapshot replays the bounded tape", func(t *testing.T) {
		// No snapshot on the tape is the fresh-computer state, not
		// corruption: the tape is the truth and a bounded genesis replay is
		// allowed. Corrupt or foreign snapshots — the failures a snapshot's
		// seal exists to catch — still refuse in the subtests below.
		rt, fake, checkpoint, _ := rematerializeSeededWithFrontend(t, "computer-refuse-missing")
		fake.snapshotDigest = ""
		result, err := rt.RematerializeFromTape(ctx, "computer-refuse-missing", checkpoint)
		if err != nil {
			t.Fatalf("snapshotless rematerialize refused: %v", err)
		}
		if result.BaseSequence != 0 || result.BaseBlobSHA256 != "" {
			t.Fatalf("snapshotless replay reported a snapshot: seq=%d root=%s", result.BaseSequence, result.BaseBlobSHA256)
		}
	})
	t.Run("the snapshot starts the restore", func(t *testing.T) {
		rt, fake, checkpoint, _ := rematerializeSeededWithFrontend(t, "computer-restore-snapshot")
		result, err := rt.RematerializeFromTape(ctx, "computer-restore-snapshot", checkpoint)
		if err != nil {
			t.Fatalf("snapshot rematerialize refused: %v", err)
		}
		if result.BaseSequence != fake.record.StoreHeadSequence || result.BaseBlobSHA256 != fake.record.Manifest.Root || !result.WitnessMatched {
			t.Fatalf("restore used snapshot seq=%d root=%s witness=%v; want %d %s", result.BaseSequence, result.BaseBlobSHA256, result.WitnessMatched, fake.record.StoreHeadSequence, fake.record.Manifest.Root)
		}
	})
	t.Run("a tape outage is not a refusal", func(t *testing.T) {
		rt, fake, checkpoint, _ := rematerializeSeededWithFrontend(t, "computer-refuse-outage")
		fake.tapeErr = errors.New("platform outage")
		_, err := rt.RematerializeFromTape(ctx, "computer-refuse-outage", checkpoint)
		if err == nil || errors.Is(err, recoveryplan.ErrBaseRefused) {
			t.Fatalf("outage misclassified as refusal: %v", err)
		}
	})
	t.Run("foreign snapshot", func(t *testing.T) {
		rt, fake, checkpoint, _ := rematerializeSeededWithFrontend(t, "computer-refuse-foreign")
		fake.foreign = true
		if _, err := rt.RematerializeFromTape(ctx, "computer-refuse-foreign", checkpoint); !errors.Is(err, recoveryplan.ErrBaseRefused) {
			t.Fatalf("foreign snapshot did not refuse: %v", err)
		}
		if rt.store == nil {
			t.Fatal("refusal closed the original realization")
		}
	})
	t.Run("corrupt chunk", func(t *testing.T) {
		rt, fake, checkpoint, _ := rematerializeSeededWithFrontend(t, "computer-refuse-corrupt")
		fake.corruptChunks = true
		if _, err := rt.RematerializeFromTape(ctx, "computer-refuse-corrupt", checkpoint); !errors.Is(err, recoveryplan.ErrBaseRefused) {
			t.Fatalf("corrupt chunk did not refuse: %v", err)
		}
		if rt.store == nil {
			t.Fatal("refusal closed the original realization")
		}
	})
	t.Run("a witness of another version", func(t *testing.T) {
		rt, _, checkpoint, _ := rematerializeSeededWithFrontend(t, "computer-refuse-version")
		checkpoint.Request.VMLocalContentWitness.WitnessVersion = 2
		_, err := rt.RematerializeFromTape(ctx, "computer-refuse-version", checkpoint)
		if err == nil || !strings.Contains(err.Error(), "not judgeable") {
			t.Fatalf("a v2 witness was not refused with its version: %v", err)
		}
	})
	t.Run("non-descendant target", func(t *testing.T) {
		rt, _, checkpoint, _ := rematerializeSeededWithFrontend(t, "computer-refuse-target")
		checkpoint.Request.AcceptedEventHead = strings.Repeat("9", 64)
		checkpoint.Request.EffectiveEventHead = strings.Repeat("9", 64)
		rebuilt, _, err := selfdevprotocol.CheckpointFromRequest(checkpoint.Request)
		if err != nil {
			t.Fatal(err)
		}
		checkpoint.Digest = rebuilt.Digest
		if _, err := rt.RematerializeFromTape(ctx, "computer-refuse-target", checkpoint); !errors.Is(err, recoveryplan.ErrBaseRefused) {
			t.Fatalf("non-descendant target did not refuse: %v", err)
		}
		if rt.store == nil {
			t.Fatal("refusal closed the original realization")
		}
	})
	t.Run("refusal maps to conflict", func(t *testing.T) {
		rt, fake, checkpoint, _ := rematerializeSeededWithFrontend(t, "computer-refuse-status")
		fake.foreign = true
		body, _ := json.Marshal(rematerializeAPIRequest{Checkpoint: checkpoint})
		request := httptest.NewRequest(http.MethodPost, "/api/computers/computer-refuse-status/lifecycle/rematerialize-from-tape", bytes.NewReader(body))
		request.Header.Set("X-Authenticated-User", "owner-refuse")
		request.Header.Set("X-Authenticated-Computer", "computer-refuse-status")
		response := httptest.NewRecorder()
		NewAPIHandler(rt).HandleComputersRouter(response, request)
		if response.Code != http.StatusConflict {
			t.Fatalf("refusal status=%d body=%s", response.Code, response.Body.String())
		}
	})
}

func TestRestoreTailReceiptBounds(t *testing.T) {
	full := &restoreReplayObserver{}
	for _, seq := range []uint64{2, 3} {
		full.RecordApplied(seq)
	}
	full.PageFetched(1, 2)
	if applied, err := full.tailReceipt(1, 3); err != nil || applied != 2 {
		t.Fatalf("valid tail receipt = %d %v", applied, err)
	}
	prefix := &restoreReplayObserver{}
	prefix.PageFetched(0, 3)
	prefix.RecordApplied(1)
	if _, err := prefix.tailReceipt(1, 3); !errors.Is(err, recoveryplan.ErrBaseRefused) {
		t.Fatalf("prefix replay admitted: %v", err)
	}
	gap := &restoreReplayObserver{}
	gap.PageFetched(1, 2)
	gap.RecordApplied(2)
	gap.RecordApplied(4)
	if _, err := gap.tailReceipt(1, 4); !errors.Is(err, recoveryplan.ErrBaseRefused) {
		t.Fatalf("gapped tail admitted: %v", err)
	}
}

// Rerun 12 (docs/problems/m11-rerun-12-restore-refused-and-texture-reports-no-change-2026-10-10.md):
// a rollback target older than the newest recovery start point must still
// restore, and a target that is not on the chain must refuse. Failure modes
// pinned:
//   - a target older than the newest snapshot is refused although the tape
//     proves it is on the chain (rollback impossible by construction);
//   - a snapshot above the target is offered as a start point;
//   - a target that is not on the chain falls back to genesis instead of
//     refusing;
//   - an old target beyond the replay bound is replayed with no snapshot;
//   - a tape outage is reported as a refusal.
func TestPlanRestoreTargetBeforeNewestSnapshot(t *testing.T) {
	ctx := context.Background()
	const computerID = "computer-restore-plan"
	head := func(seq uint64) string { return fmt.Sprintf("%064x", seq) }
	// chain is n events; the events at snapshotAt are snapshot_recorded.
	tape := func(n uint64, snapshotAt ...uint64) *restoreTapeFake {
		events := make([]computerevent.DurableEvent, 0, n)
		for seq := uint64(1); seq <= n; seq++ {
			var record computerevent.DurableEvent
			record.Request.Event.Sequence = seq
			record.Request.Next.CanonicalEventHead = head(seq)
			for _, at := range snapshotAt {
				if at == seq {
					record.Request.Event.EventKind = computerevent.EventSnapshotRecorded
					record.Request.Event.EventID = fmt.Sprintf("event-%d", seq)
					record.Request.Event.PayloadCommitment = strings.Repeat("a", 64)
				}
			}
			events = append(events, record)
		}
		return newRestoreTapeFake(func() []computerevent.DurableEvent { return events })
	}

	t.Run("a target past the snapshot starts from it", func(t *testing.T) {
		plan, err := planRestoreTarget(ctx, tape(60, 40), computerID, head(55))
		if err != nil || plan.sequence != 55 || len(plan.candidates) != 1 || plan.candidates[0].Sequence != 40 {
			t.Fatalf("plan = %+v, %v; want snapshot 40 then tail to 55", plan, err)
		}
	})
	t.Run("a snapshot above the target is not offered", func(t *testing.T) {
		plan, err := planRestoreTarget(ctx, tape(60, 40), computerID, head(12))
		if err != nil || plan.sequence != 12 || len(plan.candidates) != 0 {
			t.Fatalf("plan = %+v, %v; want genesis replay to 12", plan, err)
		}
	})
	t.Run("the newest snapshot below the target is last", func(t *testing.T) {
		plan, err := planRestoreTarget(ctx, tape(60, 5, 10, 20), computerID, head(12))
		if err != nil || len(plan.candidates) != 2 || plan.candidates[1].Sequence != 10 {
			t.Fatalf("plan = %+v, %v; want snapshots 5 and 10, newest last", plan, err)
		}
	})
	t.Run("not on the chain refuses", func(t *testing.T) {
		_, err := planRestoreTarget(ctx, tape(60, 40), computerID, strings.Repeat("9", 64))
		if !errors.Is(err, recoveryplan.ErrBaseRefused) {
			t.Fatalf("foreign target: %v, want refusal", err)
		}
	})
	t.Run("no snapshot and a target within the bound replays from genesis", func(t *testing.T) {
		plan, err := planRestoreTarget(ctx, tape(30), computerID, head(30))
		if err != nil || plan.sequence != 30 || len(plan.candidates) != 0 {
			t.Fatalf("plan = %+v, %v; want genesis replay to 30", plan, err)
		}
	})
	t.Run("a target beyond the replay bound with no snapshot refuses", func(t *testing.T) {
		bound := recoveryplan.MaxRecoveryTailEvents
		_, err := planRestoreTarget(ctx, tape(bound+30), computerID, head(bound+5))
		if !errors.Is(err, recoveryplan.ErrBaseRefused) || !strings.Contains(err.Error(), "replay bound") {
			t.Fatalf("old target past the bound: %v, want bound refusal", err)
		}
	})
	t.Run("a snapshot lifts the replay bound", func(t *testing.T) {
		bound := recoveryplan.MaxRecoveryTailEvents
		plan, err := planRestoreTarget(ctx, tape(bound+30, bound), computerID, head(bound+5))
		if err != nil || len(plan.candidates) != 1 {
			t.Fatalf("plan = %+v, %v; want the snapshot to start the restore", plan, err)
		}
	})
	t.Run("a tape outage is not a refusal", func(t *testing.T) {
		src := tape(60, 40)
		src.tapeErr = errors.New("platform outage")
		if _, err := planRestoreTarget(ctx, src, computerID, head(12)); err == nil || errors.Is(err, recoveryplan.ErrBaseRefused) {
			t.Fatalf("outage: %v, want a non-refusal error", err)
		}
	})
}

// Rerun 12: the post-apply checkpoint failed "projection repair required"
// because 37 events landed while the probe replayed to the end of the tape and
// compared with a live head it had read earlier. Failure modes pinned:
//   - an event appended during the probe fails it (a busy computer can never
//     checkpoint);
//   - the replay runs past the captured head, so it compares different heads;
//   - the report names a head other than the one its live state was read at.
func TestReplayCompletenessCapturesHeadAndStateTogether(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-replay-busy"
	storePath := filepath.Join(t.TempDir(), "runtime.db")
	live, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = live.Close() }()
	rt, cas, capturedHead := rematerializeTapeRuntime(t, computerID, storePath, live)
	rt.restoreTape = newRestoreTapeFake(func() []computerevent.DurableEvent { return cas.events })
	appended := false
	rt.replayProbeAfterCapture = func() {
		eventID, _ := computerevent.NewEventID()
		update := computerevent.Event{SchemaVersion: 1, EventID: eventID, ComputerID: computerID, EventKind: computerevent.EventResearchUpdate,
			OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), IdempotencyKey: "during-probe", ActorProfile: "research", AuthorityRef: "typed-update",
			PayloadCommitment: strings.Repeat("0", 64), PrivacyClass: "owner", ResultingEffectiveCommitment: strings.Repeat("f", 64), ReducerVersion: 1}
		if _, err := rt.eventAppender.AppendNew(ctx, update, computerevent.TransitionInput{TargetStateCommitment: strings.Repeat("f", 64)}, nil); err != nil {
			t.Errorf("append during probe: %v", err)
			return
		}
		appended = true
	}

	report, err := rt.ReplayCompleteness(ctx, computerID)
	if err != nil {
		t.Fatalf("probe on a busy computer refused: %v", err)
	}
	if !appended {
		t.Fatal("no event was appended during the probe")
	}
	if report.LiveHead == nil || report.LiveHead.CanonicalEventHead != capturedHead {
		t.Fatalf("report live head = %+v, want the captured head %s", report.LiveHead, capturedHead)
	}
	if report.ReplayHead == nil || report.ReplayHead.CanonicalEventHead != capturedHead {
		t.Fatalf("replay ran to %+v, want the captured head %s", report.ReplayHead, capturedHead)
	}
	if !report.Result.Equivalent() {
		t.Fatalf("state captured with its head is not equivalent: %#v", report.Result)
	}
	if now, _ := live.Head(ctx, computerID); now == nil || now.CanonicalEventHead == capturedHead {
		t.Fatalf("live head did not advance past the capture: %+v", now)
	}
}
