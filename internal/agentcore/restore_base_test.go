package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/projectionbase"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

type restoreBaseFake struct {
	descriptor     projectionbase.Descriptor
	blob           []byte
	events         []computerevent.DurableEvent
	watermarkErr   error
	emptyWatermark bool
	foreignBase    bool
	corruptBlob    bool
	dropTailEvent  bool
}

func (f *restoreBaseFake) Watermark(ctx context.Context, computerID string) (uint64, string, error) {
	if f.watermarkErr != nil {
		return 0, "", f.watermarkErr
	}
	if f.emptyWatermark {
		return 0, "", nil
	}
	return f.descriptor.Sequence, f.descriptor.BlobSHA256, nil
}

func (f *restoreBaseFake) Descriptor(ctx context.Context, computerID, baseRef string) (projectionbase.Descriptor, error) {
	d := f.descriptor
	if f.foreignBase {
		d.ComputerID = "computer-foreign-base"
	}
	return d, nil
}

func (f *restoreBaseFake) DownloadBlob(ctx context.Context, computerID, baseRef string, dst io.Writer) error {
	blob := f.blob
	if f.corruptBlob {
		blob = append(append([]byte{}, blob...), 0x00)
	}
	_, err := dst.Write(blob)
	return err
}

func (f *restoreBaseFake) TailPage(ctx context.Context, computerID string, afterSequence uint64, pageSize int) ([]computerevent.DurableEvent, error) {
	var page []computerevent.DurableEvent
	for _, record := range f.events {
		if record.Request.Event.Sequence <= afterSequence {
			continue
		}
		if f.dropTailEvent && record.Request.Event.Sequence == afterSequence+1 {
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

// memChainSource replays the test chain verbatim: the retained durable events
// carry their original receipts, so a rebuilt base stays witness-equivalent
// with the live store. A disk-layout source would synthesize stub receipts
// and break equivalence.
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

// seedRestoreBase rebuilds a real base at baseSeq through the offline rebuilder
// over the test chain retained by cas, and installs it as the runtime's restore
// source. The genesis never carries payload references, so a W=1 base always
// rebuilds; later events replay as the tail through the production loop.
func seedRestoreBase(t *testing.T, ctx context.Context, rt *Runtime, cas *replayEventCAS, computerID string, baseSeq int) projectionbase.Descriptor {
	t.Helper()
	if len(cas.events) < baseSeq {
		t.Fatalf("chain has %d events, want at least %d for base", len(cas.events), baseSeq)
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
	head, err := cas.Head(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	artifactsRoot := t.TempDir()
	keyMaterial := make([]byte, 32)
	for i := range keyMaterial {
		keyMaterial[i] = byte(i + 11)
	}
	rebuilder, err := projectionbase.NewRebuilder(projectionbase.Config{
		ComputerID: computerID, TargetHead: target, TargetSequence: uint64(baseSeq),
		ArtifactsRoot: artifactsRoot,
		ScratchDir:    t.TempDir(), KeyMaterial: keyMaterial, BatchSize: 100,
		MemoryLimitRSS: 512 * 1024 * 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	mem := &memChainSource{events: cas.events, head: head, computerID: computerID}
	result, err := rebuilder.Run(ctx, mem)
	if err != nil {
		t.Fatalf("rebuild test base at %d: %v", baseSeq, err)
	}
	blob, err := os.ReadFile(result.BlobPath)
	if err != nil {
		t.Fatal(err)
	}
	rt.restoreBaseSource = &restoreBaseFake{descriptor: result.Descriptor, blob: blob, events: cas.events}
	return result.Descriptor
}

var _ projectionbase.BaseSource = (*restoreBaseFake)(nil)

// TestReplayCompletenessBaselessChain proves a computer with no advertised
// base still produces a witness: the probe replays the full tape into a
// disposable store and compares live vs replayed state. Fresh machines must
// be able to checkpoint; long chains still refuse through
// MaxRecoveryTailEvents in openProbeReplayStore.
func TestReplayCompletenessBaselessChain(t *testing.T) {
	computerID := "computer-replay-baseless"
	storePath := filepath.Join(t.TempDir(), "runtime.db")
	live, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = live.Close() }()
	rt, cas, _ := rematerializeTapeRuntime(t, computerID, storePath, live)
	rt.restoreBaseSource = restoreBaseFakeFor(projectionbase.Descriptor{}, nil, cas.events)
	rt.restoreBaseSource.(*restoreBaseFake).emptyWatermark = true

	report, err := rt.ReplayCompleteness(context.Background(), computerID)
	if err != nil {
		t.Fatalf("baseless replay completeness refused: %v", err)
	}
	if report.BaseSequence != 0 || report.BaseBlobSHA256 != "" {
		t.Fatalf("baseless probe reported a base: seq=%d blob=%s", report.BaseSequence, report.BaseBlobSHA256)
	}
	if !report.Result.Equivalent() {
		t.Fatalf("baseless full replay was not equivalent: %#v", report.Result)
	}
}

func restoreBaseFakeFor(descriptor projectionbase.Descriptor, blob []byte, events []computerevent.DurableEvent) *restoreBaseFake {
	return &restoreBaseFake{descriptor: descriptor, blob: blob, events: events}
}

// rematerializeSeededWithFrontend builds a runtime with a genesis chain, a
// seeded W=1 base, pinned frontend releases, and a valid checkpoint, ready
// for rematerialize refusal-matrix tests.
func rematerializeSeededWithFrontend(t *testing.T, computerID string) (*Runtime, *restoreBaseFake, selfdevprotocol.Checkpoint, string) {
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
		t.Fatalf("probe base = %d, want 1", report.BaseSequence)
	}
	witness, err := selfdevprotocol.WitnessFromObservationSets(report.Live, report.Replay, report.Result)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := rematerializeTestCheckpoint(t, computerID, witness, targetDigest, targetIdentity, acceptedHead)
	fake, ok := rt.restoreBaseSource.(*restoreBaseFake)
	if !ok {
		t.Fatal("seeded source is not a test fake")
	}
	return rt, fake, checkpoint, storePath
}

func TestRematerializeRefusesFailureClasses(t *testing.T) {
	ctx := context.Background()
	t.Run("absent watermark replays the bounded tape", func(t *testing.T) {
		// No advertised base is the fresh-computer state, not corruption: the
		// tape is the truth and a bounded genesis replay is allowed. Stale or
		// corrupt bases — the failures a watermark exists to catch — still
		// refuse in the subtests below.
		rt, fake, checkpoint, _ := rematerializeSeededWithFrontend(t, "computer-refuse-missing")
		fake.emptyWatermark = true
		result, err := rt.RematerializeFromTape(ctx, "computer-refuse-missing", checkpoint)
		if err != nil {
			t.Fatalf("baseless rematerialize refused: %v", err)
		}
		if result.BaseSequence != 0 || result.BaseBlobSHA256 != "" {
			t.Fatalf("baseless replay reported a base: seq=%d blob=%s", result.BaseSequence, result.BaseBlobSHA256)
		}
	})
	t.Run("watermark outage is not a refusal", func(t *testing.T) {
		rt, fake, checkpoint, _ := rematerializeSeededWithFrontend(t, "computer-refuse-outage")
		fake.watermarkErr = errors.New("platform outage")
		_, err := rt.RematerializeFromTape(ctx, "computer-refuse-outage", checkpoint)
		if err == nil || errors.Is(err, projectionbase.ErrBaseRefused) {
			t.Fatalf("outage misclassified as refusal: %v", err)
		}
	})
	t.Run("foreign descriptor", func(t *testing.T) {
		rt, fake, checkpoint, _ := rematerializeSeededWithFrontend(t, "computer-refuse-foreign")
		fake.foreignBase = true
		if _, err := rt.RematerializeFromTape(ctx, "computer-refuse-foreign", checkpoint); !errors.Is(err, projectionbase.ErrBaseRefused) {
			t.Fatalf("foreign base did not refuse: %v", err)
		}
		if rt.store == nil {
			t.Fatal("refusal closed the original realization")
		}
	})
	t.Run("corrupt blob", func(t *testing.T) {
		rt, fake, checkpoint, _ := rematerializeSeededWithFrontend(t, "computer-refuse-corrupt")
		fake.corruptBlob = true
		if _, err := rt.RematerializeFromTape(ctx, "computer-refuse-corrupt", checkpoint); !errors.Is(err, projectionbase.ErrBaseRefused) {
			t.Fatalf("corrupt blob did not refuse: %v", err)
		}
		if rt.store == nil {
			t.Fatal("refusal closed the original realization")
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
		if _, err := rt.RematerializeFromTape(ctx, "computer-refuse-target", checkpoint); !errors.Is(err, projectionbase.ErrBaseRefused) {
			t.Fatalf("non-descendant target did not refuse: %v", err)
		}
		if rt.store == nil {
			t.Fatal("refusal closed the original realization")
		}
	})
	t.Run("refusal maps to conflict", func(t *testing.T) {
		rt, fake, checkpoint, _ := rematerializeSeededWithFrontend(t, "computer-refuse-status")
		fake.foreignBase = true
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
	if _, err := prefix.tailReceipt(1, 3); !errors.Is(err, projectionbase.ErrBaseRefused) {
		t.Fatalf("prefix replay admitted: %v", err)
	}
	gap := &restoreReplayObserver{}
	gap.PageFetched(1, 2)
	gap.RecordApplied(2)
	gap.RecordApplied(4)
	if _, err := gap.tailReceipt(1, 4); !errors.Is(err, projectionbase.ErrBaseRefused) {
		t.Fatalf("gapped tail admitted: %v", err)
	}
}

// Rerun 12 (docs/problems/m11-rerun-12-restore-refused-and-texture-reports-no-change-2026-10-10.md):
// checkpointd advanced the advertised base past the pinned head, and restore
// refused "target is not a descendant of watermark". Failure modes pinned:
//   - a target older than the newest base is refused although the tape
//     proves it is on the chain (rollback impossible by construction);
//   - a target that is not on the chain falls back to genesis instead of
//     refusing;
//   - an old target beyond the replay bound is replayed anyway;
//   - a target the base should cover (at or past its sequence) skips a base
//     that refused, so a stale or corrupt base no longer refuses;
//   - a target past the base stops using the base.
func TestPlanRestoreTargetBeforeNewestBase(t *testing.T) {
	ctx := context.Background()
	const computerID = "computer-restore-plan"
	head := func(seq uint64) string { return fmt.Sprintf("%064x", seq) }
	chain := func(n uint64) []computerevent.DurableEvent {
		events := make([]computerevent.DurableEvent, 0, n)
		for seq := uint64(1); seq <= n; seq++ {
			var record computerevent.DurableEvent
			record.Request.Event.Sequence = seq
			record.Request.Next.CanonicalEventHead = head(seq)
			events = append(events, record)
		}
		return events
	}
	source := func(watermark uint64, baseHead string, events []computerevent.DurableEvent) *restoreBaseFake {
		return &restoreBaseFake{
			descriptor: projectionbase.Descriptor{ComputerID: computerID, Sequence: watermark, CanonicalHead: baseHead, BlobSHA256: strings.Repeat("e", 64)},
			events:     events,
		}
	}

	t.Run("older than the newest base replays from genesis", func(t *testing.T) {
		plan, err := planRestoreTarget(ctx, source(40, head(40), chain(60)), computerID, head(12))
		if err != nil || plan.fromBase || plan.sequence != 12 {
			t.Fatalf("plan = %+v, %v; want genesis replay to 12", plan, err)
		}
	})
	t.Run("past the base uses the base", func(t *testing.T) {
		plan, err := planRestoreTarget(ctx, source(40, head(40), chain(60)), computerID, head(55))
		if err != nil || !plan.fromBase || plan.sequence != 55 {
			t.Fatalf("plan = %+v, %v; want base then tail to 55", plan, err)
		}
	})
	t.Run("not on the chain refuses", func(t *testing.T) {
		_, err := planRestoreTarget(ctx, source(40, head(40), chain(60)), computerID, strings.Repeat("9", 64))
		if !errors.Is(err, projectionbase.ErrBaseRefused) {
			t.Fatalf("foreign target: %v, want refusal", err)
		}
	})
	t.Run("a base that should cover the target still refuses", func(t *testing.T) {
		// The tail after the base is missing the target's event, so the
		// base path refuses; a genesis read past the base must not bypass it.
		src := source(40, head(40), chain(60))
		src.dropTailEvent = true
		_, err := planRestoreTarget(ctx, src, computerID, head(41))
		if !errors.Is(err, projectionbase.ErrBaseRefused) {
			t.Fatalf("corrupt base covering the target: %v, want refusal", err)
		}
	})
	t.Run("old target beyond the replay bound refuses", func(t *testing.T) {
		bound := projectionbase.MaxRecoveryTailEvents
		_, err := planRestoreTarget(ctx, source(bound+20, head(bound+20), chain(bound+30)), computerID, head(bound+5))
		if !errors.Is(err, projectionbase.ErrBaseRefused) || !strings.Contains(err.Error(), "replay bound") {
			t.Fatalf("old target past the bound: %v, want bound refusal", err)
		}
	})
	t.Run("absent base replays from genesis", func(t *testing.T) {
		src := source(0, "", chain(30))
		src.emptyWatermark = true
		plan, err := planRestoreTarget(ctx, src, computerID, head(30))
		if err != nil || plan.fromBase || plan.sequence != 30 {
			t.Fatalf("plan = %+v, %v; want genesis replay to 30", plan, err)
		}
	})
	t.Run("watermark outage is not a refusal", func(t *testing.T) {
		src := source(40, head(40), chain(60))
		src.watermarkErr = errors.New("platform outage")
		if _, err := planRestoreTarget(ctx, src, computerID, head(12)); err == nil || errors.Is(err, projectionbase.ErrBaseRefused) {
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
	rt.restoreBaseSource = restoreBaseFakeFor(projectionbase.Descriptor{}, nil, cas.events)
	rt.restoreBaseSource.(*restoreBaseFake).emptyWatermark = true
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
