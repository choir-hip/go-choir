package agentcore

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

type emptyReplayAuthority struct{}

func (emptyReplayAuthority) Head(context.Context, string) (*computerevent.Head, error) {
	return nil, nil
}

func (emptyReplayAuthority) CompareAndSwap(context.Context, computerevent.CASRequest) (computerevent.Receipt, error) {
	return computerevent.Receipt{}, errors.New("append is not part of the probe")
}

func (emptyReplayAuthority) Events(context.Context, string, uint64) ([]computerevent.DurableEvent, error) {
	return []computerevent.DurableEvent{}, nil
}

func (emptyReplayAuthority) PinEvent(context.Context, string, []byte, string) (computerevent.PinResult, error) {
	return computerevent.PinResult{}, errors.New("pin is not part of the probe")
}

type replayEventCAS struct {
	key        computerevent.SigningKey
	projection computerevent.ProjectionStore
	events     []computerevent.DurableEvent
	onEvents   func()
}

func (c *replayEventCAS) Head(ctx context.Context, computerID string) (*computerevent.Head, error) {
	return c.projection.Head(ctx, computerID)
}

func (c *replayEventCAS) CompareAndSwap(_ context.Context, request computerevent.CASRequest) (computerevent.Receipt, error) {
	receipt, err := computerevent.NewSignedReceipt(
		"EventHeadReceipt",
		"corpusd",
		map[string]any{"event_digest": request.EventDigest},
		[]computerevent.SigningKey{c.key},
		time.Now().UTC(),
	)
	if err != nil {
		return computerevent.Receipt{}, err
	}
	c.events = append(c.events, computerevent.DurableEvent{Request: request, Receipt: receipt})
	return receipt, nil
}

func (c *replayEventCAS) Events(_ context.Context, _ string, afterSequence uint64) ([]computerevent.DurableEvent, error) {
	if c.onEvents != nil {
		c.onEvents()
		c.onEvents = nil
	}
	records := make([]computerevent.DurableEvent, 0, len(c.events))
	for _, record := range c.events {
		if record.Request.Event.Sequence > afterSequence {
			records = append(records, record)
		}
	}
	return records, nil
}
func (emptyReplayAuthority) VerifyEventHeadReceipt(context.Context, computerevent.Receipt, computerevent.CASRequest) error {
	return nil
}

func TestCompareReplayRunMemoryClassifiesProjectionDrift(t *testing.T) {
	live := []choirstore.RunMemoryEntryFingerprint{
		{EntryID: "entry-a", RunID: "run-a", OwnerID: "owner-a", AgentID: "agent-a", Seq: 4, CreatedAt: "2026-08-18T00:00:00Z", RowDigest: "row-a-live", FieldDigests: map[string]string{"summary": "summary-a-live"}},
		{EntryID: "entry-b", RunID: "run-b", OwnerID: "owner-b", AgentID: "agent-b", Seq: 7, CreatedAt: "2026-08-19T00:00:00Z", RowDigest: "row-b-live", FieldDigests: map[string]string{"summary": "summary-b-live"}},
	}
	replay := []choirstore.RunMemoryEntryFingerprint{
		{EntryID: "entry-a", RunID: "run-a", OwnerID: "owner-a", AgentID: "agent-a", Seq: 4, CreatedAt: "2026-08-18T00:00:00Z", RowDigest: "row-a-replay", FieldDigests: map[string]string{"summary": "summary-a-replay"}},
		{EntryID: "entry-c", RunID: "run-c", OwnerID: "owner-c", AgentID: "agent-c", Seq: 9, CreatedAt: "2026-08-20T00:00:00Z", RowDigest: "row-c-replay", FieldDigests: map[string]string{"summary": "summary-c-replay"}},
	}
	got := compareReplayRunMemory(live, replay)
	if got.LiveCount != 2 || got.ReplayCount != 2 || got.LiveOnlyCount != 1 || got.ReplayOnlyCount != 1 || got.DifferentCount != 1 {
		t.Fatalf("comparison counts = %#v", got)
	}
	if len(got.Samples) != 3 {
		t.Fatalf("comparison samples = %#v", got.Samples)
	}
	if got.Samples[0].EntryID != "entry-a" || got.Samples[0].Kind != "different" || len(got.Samples[0].DifferentFields) != 1 || got.Samples[0].DifferentFields[0] != "summary" {
		t.Fatalf("different sample = %#v", got.Samples[0])
	}
	if got.Samples[1].EntryID != "entry-b" || got.Samples[1].Kind != "live_only" || got.Samples[1].RunID != "run-b" || got.Samples[1].Seq != 7 || got.Samples[2].EntryID != "entry-c" || got.Samples[2].Kind != "replay_only" {
		t.Fatalf("presence samples = %#v", got.Samples)
	}
	if got.LiveOnlyByRun["run-b"] != 1 || got.LiveOnlyByOwner["owner-b"] != 1 || got.LiveOnlyByAgent["agent-b"] != 1 || got.LiveOnlySeqMin != 7 || got.LiveOnlySeqMax != 7 || got.LiveOnlyCreatedMin != "2026-08-19T00:00:00Z" || got.LiveOnlyCreatedMax != "2026-08-19T00:00:00Z" {
		t.Fatalf("live-only lineage summary = %#v", got)
	}
}

func TestReplayCompletenessUsesDisposableProjectionWithoutMutatingLiveStore(t *testing.T) {
	computerID := "computer-replay-probe"
	storePath := filepath.Join(t.TempDir(), "runtime.db")
	liveStore, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer liveStore.Close()

	authority := emptyReplayAuthority{}
	appender, err := computerevent.NewComputerEventAppender(computerID, authority, liveStore, authority, authority)
	if err != nil {
		t.Fatal(err)
	}
	rt := &Runtime{
		cfg:           provideriface.Config{ComputerID: computerID, StorePath: storePath},
		store:         liveStore,
		eventAppender: appender,
	}

	before, err := liveStore.Head(context.Background(), computerID)
	if err != nil {
		t.Fatal(err)
	}
	report, err := rt.ReplayCompleteness(context.Background(), computerID)
	if err != nil {
		t.Fatal(err)
	}
	after, err := liveStore.Head(context.Background(), computerID)
	if err != nil {
		t.Fatal(err)
	}
	if before != nil || after != nil || report.LiveHead != nil || report.ReplayHead != nil {
		t.Fatalf("empty probe changed event heads: before=%#v after=%#v report=%#v", before, after, report)
	}
	if report.SchemaVersion != replayCompletenessSchemaVersion {
		t.Fatalf("replay report schema=%d, want %d", report.SchemaVersion, replayCompletenessSchemaVersion)
	}
	if report.Eligibility.Eligible {
		t.Fatalf("nil-head probe was marked eligible: %#v", report.Eligibility)
	}
	if !report.Result.Equivalent() {
		t.Fatalf("identical empty projections should compare equivalent: %#v", report.Result)
	}
	if report.ProbeDigest == "" {
		t.Fatal("probe did not emit a digest")
	}

	request := httptest.NewRequest(http.MethodGet, "/api/computers/"+computerID+"/self-development/replay-completeness", nil)
	request.Header.Set("X-Authenticated-User", "owner-replay-probe")
	request.Header.Set("X-Authenticated-Computer", computerID)
	response := httptest.NewRecorder()
	NewAPIHandler(rt).HandleComputersRouter(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("product replay probe status=%d body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"probe_digest"`) {
		t.Fatalf("product replay probe omitted digest: %s", response.Body.String())
	}

	if _, err := liveStore.DB().ExecContext(context.Background(), "CREATE TABLE replay_probe_drift (id VARCHAR(64) PRIMARY KEY, value VARCHAR(64) NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	if _, err := liveStore.DB().ExecContext(context.Background(), "INSERT INTO replay_probe_drift (id, value) VALUES ('drift', 'live-only')"); err != nil {
		t.Fatal(err)
	}
	drift, err := rt.ReplayCompleteness(context.Background(), computerID)
	if err != nil {
		t.Fatal(err)
	}
	if drift.Result.Equivalent() {
		t.Fatal("live-only table was not reported as replay drift")
	}
	foundDrift := false
	for _, difference := range drift.Result.Differences {
		if strings.Contains(difference.Key, "replay_probe_drift") {
			foundDrift = true
			break
		}
	}
	if !foundDrift {
		t.Fatalf("replay drift omitted live-only table: %#v", drift.Result.Differences)
	}
}

type replayGuestDeadlineResponseWriter struct {
	*httptest.ResponseRecorder
	deadline time.Time
}

func (w *replayGuestDeadlineResponseWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadline = deadline
	return nil
}

func TestReplayCompletenessExtendsGuestWriteDeadline(t *testing.T) {
	handler := NewAPIHandler(&Runtime{})
	request := httptest.NewRequest(http.MethodGet, "/api/computers/computer-replay/self-development/replay-completeness", nil)
	request.Header.Set("X-Authenticated-User", "owner-replay")
	request.Header.Set("X-Authenticated-Computer", "computer-replay")
	response := &replayGuestDeadlineResponseWriter{ResponseRecorder: httptest.NewRecorder()}
	started := time.Now()

	handler.HandleComputersRouter(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("replay completeness status=%d body=%s, want unavailable runtime", response.Code, response.Body.String())
	}
	if response.deadline.Before(started.Add(replayCompletenessGuestTimeout)) {
		t.Fatalf("guest route deadline=%s, want at least %s after start", response.deadline, replayCompletenessGuestTimeout)
	}
	if response.deadline.After(started.Add(replayCompletenessGuestTimeout + 2*replayCompletenessGuestWriteGrace)) {
		t.Fatalf("guest route deadline=%s, unexpectedly beyond route budget plus grace", response.deadline)
	}
}

func TestReplayCompletenessRejectsLiveObservationDriftDuringReplay(t *testing.T) {
	computerID := "computer-replay-live-drift"
	storePath := filepath.Join(t.TempDir(), "runtime.db")
	liveStore, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer liveStore.Close()

	cas := &replayEventCAS{
		projection: liveStore,
		onEvents: func() {
			if _, err := liveStore.DB().ExecContext(context.Background(), "CREATE TABLE replay_probe_midflight (id VARCHAR(64) PRIMARY KEY)"); err != nil {
				t.Fatalf("create midflight table: %v", err)
			}
		},
	}
	authority := emptyReplayAuthority{}
	appender, err := computerevent.NewComputerEventAppender(computerID, authority, liveStore, cas, authority)
	if err != nil {
		t.Fatal(err)
	}
	rt := &Runtime{
		cfg:           provideriface.Config{ComputerID: computerID, StorePath: storePath},
		store:         liveStore,
		eventAppender: appender,
	}
	_, err = rt.ReplayCompleteness(context.Background(), computerID)
	if err == nil || !strings.Contains(err.Error(), "live Dolt state changed during probe") {
		t.Fatalf("live midflight mutation error=%v, want stable-observation refusal", err)
	}
}

func TestReplayCompletenessReconstructsNonNilEventChain(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-replay-non-nil"
	storePath := filepath.Join(t.TempDir(), "runtime.db")
	liveStore, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer liveStore.Close()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signingKey := computerevent.SigningKey{
		SignerRef:  computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "replay-test"},
		PrivateKey: privateKey,
	}
	cas := &replayEventCAS{key: signingKey, projection: liveStore}
	appender, err := computerevent.NewComputerEventAppender(
		computerID,
		rollbackTestPinner{signingKey},
		liveStore,
		cas,
		rollbackTestReceiptVerifier{},
	)
	if err != nil {
		t.Fatal(err)
	}
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	genesis := computerevent.Event{
		SchemaVersion:                computerevent.SchemaVersionV1,
		EventID:                      eventID,
		ComputerID:                   computerID,
		EventKind:                    computerevent.EventGenesisImported,
		OccurredAt:                   time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey:               "genesis",
		ActorProfile:                 "super",
		AuthorityRef:                 "owner",
		PrivacyClass:                 "owner",
		PayloadCommitment:            strings.Repeat("a", 64),
		ProposedEffectRef:            strings.Repeat("b", 64),
		ResultingEffectiveCommitment: strings.Repeat("a", 64),
		ReducerVersion:               computerevent.ReducerVersionV1,
	}
	if _, err := appender.AppendNew(ctx, genesis, computerevent.TransitionInput{
		TargetStateCommitment: strings.Repeat("a", 64),
	}, nil); err != nil {
		t.Fatal(err)
	}

	rt := &Runtime{
		cfg:           provideriface.Config{ComputerID: computerID, StorePath: storePath},
		store:         liveStore,
		eventAppender: appender,
	}
	report, err := rt.ReplayCompleteness(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	if report.LiveHead == nil || report.ReplayHead == nil {
		t.Fatalf("non-empty event chain produced nil heads: %#v", report)
	}
	if !report.Result.Equivalent() {
		t.Fatalf("non-empty event chain replay drifted: %#v", report.Result)
	}
	if !report.Eligibility.Eligible {
		t.Fatalf("non-empty event chain was not eligible: %#v", report.Eligibility)
	}
}
