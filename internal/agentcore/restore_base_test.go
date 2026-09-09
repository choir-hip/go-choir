package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
		ComputerID: computerID, TargetHead: target, ArtifactsRoot: artifactsRoot,
		ScratchDir: t.TempDir(), KeyMaterial: keyMaterial, BatchSize: 100,
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
	t.Run("missing watermark", func(t *testing.T) {
		rt, fake, checkpoint, storePath := rematerializeSeededWithFrontend(t, "computer-refuse-missing")
		fake.emptyWatermark = true
		if _, err := rt.RematerializeFromTape(ctx, "computer-refuse-missing", checkpoint); !errors.Is(err, projectionbase.ErrBaseRefused) {
			t.Fatalf("missing base did not refuse: %v", err)
		}
		if rt.store == nil {
			t.Fatal("refusal closed the original realization")
		}
		if _, err := os.Stat(storePath); err != nil {
			t.Fatalf("original marker moved on refusal: %v", err)
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
