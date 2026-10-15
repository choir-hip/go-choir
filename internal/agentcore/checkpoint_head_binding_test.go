package agentcore

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/projectionbase"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// Checkpoint head race (docs/problems/selfdev-go-drill-jobs-0101-2026-10-15.md,
// item 1). A checkpoint's accepted head and its VM-local witness must describe
// the same projection state. In the drill a desk appended while the
// checkpoint replayed, the publish named the later head, and the restore
// refused "reconstructed content root does not match checkpoint". Failure
// modes pinned here:
//   - C1 an append between capture and publish leaves the published head
//     ahead of the witness, so the restore refuses;
//   - C2 a live-only row written after the capture is attested at the bound
//     head without a comparison there;
//   - C5 a head that keeps moving outside this guest's appends loops forever
//     instead of failing loudly and bounded;
//   - C6 an idle checkpoint changes head, idempotency key or witness;
//   - C7 the base pin names a head other than the published checkpoint's.

// checkpointPlatformFake is the corpusd checkpoint authority as the guest
// sees it: it accepts a checkpoint only at the computer's current canonical
// head (CheckpointAuthority.Publish's FOR UPDATE equality) and signs the
// response with the platform-control key the guest credential verifies.
type checkpointPlatformFake struct {
	mu        sync.Mutex
	head      func() string
	key       ed25519.PrivateKey
	refuseAll bool
	requests  []selfdevprotocol.CheckpointRequest
	published []selfdevprotocol.CheckpointResponse
}

func (f *checkpointPlatformFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var request selfdevprotocol.CheckpointRequest
	raw, ok := readBody(r.Body)
	if !ok || selfdevprotocol.DecodeStrict(raw, &request) != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	f.requests = append(f.requests, request)
	if f.refuseAll || request.AcceptedEventHead != f.head() {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "checkpoint authority: current accepted/effective head does not match request"})
		return
	}
	checkpoint, _, err := selfdevprotocol.CheckpointFromRequest(request)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	commitment, _ := selfdevprotocol.Digest(request)
	key := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "checkpoint-head-test"}, PrivateKey: f.key}
	receipt, err := selfdevprotocol.NewAuthorityReceipt(selfdevprotocol.ReceiptKindCheckpoint, request.ComputerID, commitment, checkpoint.Digest, "corpusd", key, time.Now().UTC())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	response := selfdevprotocol.CheckpointResponse{Checkpoint: checkpoint, Receipt: receipt}
	f.published = append(f.published, response)
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}

type checkpointHeadFixture struct {
	rt       *Runtime
	cas      *replayEventCAS
	live     *choirstore.Store
	platform *checkpointPlatformFake
	base     *restoreBaseFake
	genesis  string
}

// newCheckpointHeadFixture is a computer with a genesis chain, a seeded W=1
// base, a served release, and owner-recovery platform control wired to the
// fake authority.
func newCheckpointHeadFixture(t *testing.T, computerID string) *checkpointHeadFixture {
	t.Helper()
	storePath := filepath.Join(t.TempDir(), "runtime.db")
	live, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	rt, cas, genesis := rematerializeTapeRuntime(t, computerID, storePath, live)
	seedRestoreBase(t, context.Background(), rt, cas, computerID, 1)
	updaterRoot := filepath.Join(t.TempDir(), "updater")
	digest, _ := pinFrontendRelease(t, updaterRoot, computerID, "<html>checkpoint</html>")
	pointCurrent(t, updaterRoot, digest)
	rt.selfdevUpdaterRoot = updaterRoot

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	platform := &checkpointPlatformFake{key: privateKey}
	platform.head = func() string {
		head, err := rt.store.Head(context.Background(), computerID)
		if err != nil || head == nil {
			return ""
		}
		return head.CanonicalEventHead
	}
	mux := http.NewServeMux()
	mux.Handle("/internal/computers/checkpoints", platform)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	WithOwnerRecoveryControl(selfdev.GuestCredentialsWithCapability(server.URL, computerID, "capability", time.Now().UTC().Add(time.Hour), publicKey))(rt)
	fx := &checkpointHeadFixture{rt: rt, cas: cas, live: live, platform: platform, base: rt.restoreBaseSource.(*restoreBaseFake), genesis: genesis}
	t.Cleanup(func() {
		if fx.rt.store != nil {
			_ = fx.rt.store.Close()
		}
	})
	return fx
}

// appendDeskEvent appends one event the way a working desk does: through the
// live appender, outside any checkpoint code.
func (fx *checkpointHeadFixture) appendDeskEvent(t *testing.T, key string) {
	t.Helper()
	eventID, _ := computerevent.NewEventID()
	commitment := strings.Repeat("f", 64)
	update := computerevent.Event{SchemaVersion: 1, EventID: eventID, ComputerID: fx.rt.cfg.ComputerID, EventKind: computerevent.EventResearchUpdate,
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), IdempotencyKey: key, ActorProfile: "research", AuthorityRef: "typed-update",
		PayloadCommitment: strings.Repeat("0", 64), PrivacyClass: "owner", ResultingEffectiveCommitment: commitment, ReducerVersion: 1}
	if _, err := fx.rt.eventAppender.AppendNew(context.Background(), update, computerevent.TransitionInput{TargetStateCommitment: commitment}, nil); err != nil {
		t.Fatalf("desk append: %v", err)
	}
	fx.base.events = fx.cas.events
}

func (fx *checkpointHeadFixture) liveHead(t *testing.T) string {
	t.Helper()
	head, err := fx.rt.store.Head(context.Background(), fx.rt.cfg.ComputerID)
	if err != nil || head == nil {
		t.Fatalf("live head: %v %+v", err, head)
	}
	return head.CanonicalEventHead
}

// restore runs the product restore against a published checkpoint. The base
// source sees the chain as it is now (events appended after setup included).
func (fx *checkpointHeadFixture) restore(t *testing.T, checkpoint selfdevprotocol.Checkpoint) (RematerializeReport, error) {
	t.Helper()
	fx.base.events = fx.cas.events
	return fx.rt.RematerializeFromTape(context.Background(), fx.rt.cfg.ComputerID, checkpoint)
}

// C1, C7: the drill's race. A desk appends after the head and live state are
// captured, while the checkpoint replays. The published checkpoint must be
// restorable: its head is the one its witness was taken at.
func TestCheckpointBindBusyComputerPublishesRestorableHead(t *testing.T) {
	computerID := "computer-checkpoint-busy"
	fx := newCheckpointHeadFixture(t, computerID)
	appended := false
	fx.rt.replayProbeAfterCapture = func() {
		fx.appendDeskEvent(t, "desk-turn-during-checkpoint")
		appended = true
	}

	report, err := fx.rt.BindCheckpointRestoreSet(context.Background(), computerID)
	if err != nil {
		t.Fatalf("busy checkpoint refused: %v", err)
	}
	if !appended {
		t.Fatal("no desk event was appended during the checkpoint")
	}
	if report.PublishedCheckpoint == nil {
		t.Fatal("no checkpoint was published")
	}
	published := report.PublishedCheckpoint.Checkpoint
	headNow := fx.liveHead(t)
	if headNow == fx.genesis {
		t.Fatal("the live head did not move past the capture")
	}
	if published.Request.AcceptedEventHead != headNow {
		t.Fatalf("published head = %s, want the head at publish %s", published.Request.AcceptedEventHead, headNow)
	}
	if published.Request.VMLocalContentWitness.ContentRoot != report.VMLocalContentWitness.ContentRoot {
		t.Fatal("the bind report's witness is not the published witness")
	}
	if report.BoundEventHead != headNow || report.CaughtUpEvents != 1 {
		t.Fatalf("bind report head/caught-up = %s/%d, want %s/1", report.BoundEventHead, report.CaughtUpEvents, headNow)
	}
	pinRef := projectionbase.CheckpointPinReference(published.Request.AcceptedEventHead)
	pinned := false
	for _, pin := range fx.base.pins {
		if strings.HasSuffix(pin, " "+pinRef) {
			pinned = true
		}
	}
	if !pinned {
		t.Errorf("base pins %v do not name the published head %s", fx.base.pins, pinRef)
	}

	result, err := fx.restore(t, published)
	if err != nil {
		t.Fatalf("restore to the busy checkpoint refused: %v", err)
	}
	if !result.WitnessMatched {
		t.Fatalf("restore did not match the witness: %#v", result)
	}
}

// C6: an idle checkpoint binds the head it captured, under the same
// idempotency key and witness as before, and restores.
func TestCheckpointBindIdleComputerBindsCapturedHead(t *testing.T) {
	computerID := "computer-checkpoint-idle"
	fx := newCheckpointHeadFixture(t, computerID)
	fx.appendDeskEvent(t, "desk-turn-before-checkpoint")
	before := fx.liveHead(t)
	probe, err := fx.rt.ReplayCompleteness(context.Background(), computerID)
	if err != nil {
		t.Fatal(err)
	}
	probeWitness, err := selfdevprotocol.WitnessFromObservationSets(probe.Live, probe.Replay, probe.Result)
	if err != nil {
		t.Fatal(err)
	}

	report, err := fx.rt.BindCheckpointRestoreSet(context.Background(), computerID)
	if err != nil {
		t.Fatalf("idle checkpoint refused: %v", err)
	}
	if report.PublishedCheckpoint == nil {
		t.Fatal("no checkpoint was published")
	}
	request := report.PublishedCheckpoint.Checkpoint.Request
	if request.AcceptedEventHead != before || request.IdempotencyKey != "owner-recovery-"+before {
		t.Fatalf("idle checkpoint head/key = %s/%s, want %s", request.AcceptedEventHead, request.IdempotencyKey, before)
	}
	if request.VMLocalContentWitness.ContentRoot != probeWitness.ContentRoot || request.VMLocalContentWitness.DerivabilityDigest != probeWitness.DerivabilityDigest {
		t.Fatal("idle checkpoint witness differs from the probe's witness at the same head")
	}
	if len(fx.platform.requests) != 1 {
		t.Fatalf("idle checkpoint made %d publish requests, want 1", len(fx.platform.requests))
	}
	result, err := fx.restore(t, report.PublishedCheckpoint.Checkpoint)
	if err != nil || !result.WitnessMatched {
		t.Fatalf("restore to the idle checkpoint: %v %#v", err, result)
	}
}

// C2: state written after the capture is compared at the head the checkpoint
// binds. A live-only table that replay cannot produce refuses the
// checkpoint; nothing is published.
func TestCheckpointBindRefusesLiveOnlyStateWrittenAfterCapture(t *testing.T) {
	computerID := "computer-checkpoint-live-only"
	fx := newCheckpointHeadFixture(t, computerID)
	fx.rt.replayProbeAfterCapture = func() {
		fx.appendDeskEvent(t, "desk-turn-during-checkpoint")
		ctx := context.Background()
		if _, err := fx.live.DB().ExecContext(ctx, "CREATE TABLE checkpoint_live_only (id VARCHAR(64) PRIMARY KEY)"); err != nil {
			t.Error(err)
		}
		if _, err := fx.live.DB().ExecContext(ctx, "INSERT INTO checkpoint_live_only (id) VALUES ('drift')"); err != nil {
			t.Error(err)
		}
	}
	_, err := fx.rt.BindCheckpointRestoreSet(context.Background(), computerID)
	if err == nil {
		t.Fatal("a checkpoint attested live-only state written after the capture")
	}
	if !strings.Contains(err.Error(), "at bound sequence") {
		t.Fatalf("refusal is not the bound head's comparison: %v", err)
	}
	if len(fx.platform.published) != 0 {
		t.Fatalf("%d checkpoints were published over live-only state", len(fx.platform.published))
	}
}

// C4, C8: the tail that lands while the probe replays is replayed before the
// hold (the held section replays nothing it did not have to), and the
// bind-only report (no platform credential) names the head its witness is.
func TestCheckpointBindCatchesUpBeforeTheHold(t *testing.T) {
	computerID := "computer-checkpoint-held-tail"
	fx := newCheckpointHeadFixture(t, computerID)
	fx.rt.ownerRecoveryControl = nil
	manifest, err := updater.VerifyCurrentRelease(fx.rt.selfdevUpdaterRoot)
	if err != nil {
		t.Fatal(err)
	}
	fx.rt.replayProbeAfterCapture = func() {
		fx.appendDeskEvent(t, "desk-turn-1")
		fx.appendDeskEvent(t, "desk-turn-2")
	}
	binding, published, err := fx.rt.bindCheckpointAtHead(context.Background(), computerID, manifest.ContentDigest, manifest.Files, nil)
	if err != nil {
		t.Fatalf("bind-only checkpoint refused: %v", err)
	}
	if published != nil {
		t.Fatal("bind-only checkpoint published")
	}
	if headDigest(binding.Head) != fx.liveHead(t) || binding.CaughtUpEvents != 2 || binding.HeldTailEvents != 0 {
		t.Fatalf("binding head=%s caught_up=%d held_tail=%d, want live head, 2, 0", headDigest(binding.Head), binding.CaughtUpEvents, binding.HeldTailEvents)
	}
	if binding.CapturedHead == headDigest(binding.Head) {
		t.Fatal("the binding did not move past the captured head")
	}
}

// C5: a platform head that never matches (a writer outside this guest's
// appender) fails the checkpoint loudly after a bounded number of attempts.
func TestCheckpointBindFailsLoudlyWhenPlatformHeadKeepsMoving(t *testing.T) {
	computerID := "computer-checkpoint-moving"
	fx := newCheckpointHeadFixture(t, computerID)
	fx.platform.refuseAll = true
	_, err := fx.rt.BindCheckpointRestoreSet(context.Background(), computerID)
	if err == nil {
		t.Fatal("a checkpoint the platform refused every time succeeded")
	}
	if got := len(fx.platform.requests); got != checkpointBindAttempts {
		t.Fatalf("publish attempts = %d, want %d", got, checkpointBindAttempts)
	}
	if !strings.Contains(err.Error(), "attempts") {
		t.Fatalf("refusal does not say it gave up: %v", err)
	}
}
