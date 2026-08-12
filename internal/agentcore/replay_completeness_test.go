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
