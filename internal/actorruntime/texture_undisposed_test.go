package actorruntime

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/actor"
	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/textureowner"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// undisposedTriggerFixture is a Texture desk whose turn ends without
// disposing its trigger: an exact research report that the provider never
// answers with a Texture turn (problem 17, where no desk cell could run).
type undisposedTriggerFixture struct {
	ctx       context.Context
	store     *store.Store
	provider  *countingLifecycleProvider
	handler   *actorHandler
	ownerID   string
	computer  string
	agentID   string
	trajectID string
	workID    string
	encoded   string
	research  types.RunRecord
	producer  string
	// from is the envelope source; revisionID is set for a revision trigger.
	from       string
	revisionID string
}

func newUndisposedTriggerFixture(t *testing.T, suffix string) *undisposedTriggerFixture {
	return newUndisposedTriggerFixtureKind(t, suffix, agentcore.TextureActorOccurrenceProducerReport)
}

// newUndisposedTriggerFixtureKind builds the fixture for a producer report
// trigger or, with TextureActorOccurrenceDocumentRevision, an owner revision.
func newUndisposedTriggerFixtureKind(t *testing.T, suffix, kind string) *undisposedTriggerFixture {
	t.Helper()
	prevWait := textureUndisposedMinWait
	textureUndisposedMinWait = 0
	t.Cleanup(func() { textureUndisposedMinWait = prevWait })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "undisposed.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ownerID, computerID := "owner-undisposed-"+suffix, "autoputer-test"
	docID := "doc-adapter-admission-" + suffix
	textureAgentID := "texture:" + docID
	prov := &countingLifecycleProvider{targetAgentID: textureAgentID}
	cfg := provideriface.Config{ComputerID: computerID, StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts"), ProviderTimeout: time.Second}
	// Bare runtime, never started: the handler is driven synchronously, as
	// the dispatcher would deliver each actor update.
	rt := agentcore.New(cfg, s, events.NewEventBus(), prov)
	t.Cleanup(rt.Stop)
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })

	researchRun := seedAdapterLifecycleResearchControl(t, s, rt, ownerID, computerID, suffix, false)
	packet := types.SourcePacket{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "evidence_update", Summary: "report the desk never disposes"}
	content := "report the desk never disposes"
	digest, _ := store.ComputeLifecycleUpdatePayloadDigest(packet, content)
	queue := types.QueueLifecycleUpdateRequest{OwnerID: ownerID, ComputerID: computerID, CommandID: "queue-undisposed-" + suffix, TrajectoryID: researchRun.TrajectoryID, TargetAgentID: textureAgentID, ProducerAgentID: researchRun.AgentID, ProducerUpdateID: "undisposed-" + suffix, UpdateID: "undisposed-" + suffix, ChannelID: docID, Role: agentprofile.Research, SourceRunID: researchRun.RunID, WorkItemID: "researcher-work-adapter-admission-" + suffix, WorkDisposition: types.WorkItemOpen, Packet: packet, Content: content, PayloadDigest: digest}
	queue.CommandDigest, _ = store.ComputeQueueLifecycleUpdateDigest(queue)
	result, err := s.QueueLifecycleUpdate(ctx, queue)
	if err != nil || result.Update == nil {
		t.Fatalf("queue report: result=%+v err=%v", result, err)
	}
	occurrence, err := agentcore.TextureProducerReportOccurrence(*result.Update)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := agentcore.EncodeTextureActorOccurrence(occurrence)
	if err != nil {
		t.Fatal(err)
	}
	from := researchRun.AgentID
	revisionID := ""
	if kind == agentcore.TextureActorOccurrenceDocumentRevision {
		snapshot, err := s.GetLifecycleSnapshot(ctx, ownerID, computerID, researchRun.TrajectoryID)
		if err != nil {
			t.Fatal(err)
		}
		commit := types.CommitLifecycleArtifactHeadRequest{
			OwnerID: ownerID, ComputerID: computerID, CommandID: "commit-owner-undisposed-" + suffix, TrajectoryID: researchRun.TrajectoryID,
			ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion, ExpectedHeadRevisionID: snapshot.HeadRevision.RevisionID,
			Revision: types.Revision{RevisionID: "revision-owner-undisposed-" + suffix, DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: researchRun.TrajectoryID,
				AuthorKind: types.AuthorUser, AuthorLabel: ownerID, Content: snapshot.HeadRevision.Content + " owner edit",
				Metadata: json.RawMessage(`{"input_origin":"user_prompt","owner_prompt":"round 6"}`)},
		}
		commit.CommandDigest, err = store.ComputeCommitLifecycleArtifactHeadWithSourceGraphDigest(commit, store.TextureSourceGraphWriteSet{})
		if err != nil {
			t.Fatal(err)
		}
		committed, err := s.CommitLifecycleArtifactHeadWithSourceGraph(ctx, commit, store.TextureSourceGraphWriteSet{})
		if err != nil || committed.Revision == nil {
			t.Fatalf("commit owner revision: %+v %v", committed, err)
		}
		// The revise path's coagent_result carries the revision occurrence
		// with the client's request id; the outbox owner_revision carries
		// the revision id as its request id (the same trigger).
		revOccurrence, err := agentcore.TextureDocumentRevisionOccurrence(*committed.Revision, "client-request-"+suffix, committed.Events[0].ReducerSeq)
		if err != nil {
			t.Fatal(err)
		}
		if encoded, err = agentcore.EncodeTextureActorOccurrence(revOccurrence); err != nil {
			t.Fatal(err)
		}
		from, revisionID = "owner:"+ownerID, committed.Revision.RevisionID
	}
	handler := newActorHandler(rt, textureowner.NewHandler(rt))
	// The observed turn (problem 17): the model is called, no desk cell can
	// commit, and the run parks with the trigger still pending.
	handler.textureExecute = func(ctx context.Context, rec *types.RunRecord) error {
		if err := prov.Execute(ctx, rec, nil); err != nil {
			return err
		}
		now := time.Now().UTC()
		rec.State = types.RunPassivated
		rec.UpdatedAt = now
		if rec.Metadata == nil {
			rec.Metadata = map[string]any{}
		}
		rec.Metadata["passivated_reason"] = "durable_transition_success"
		rec.Metadata["actor_sleep_state"] = "idle"
		rec.Metadata["actor_sleep_at"] = now.Format(time.RFC3339Nano)
		return s.UpdateRun(ctx, *rec)
	}
	return &undisposedTriggerFixture{
		ctx: ctx, store: s, provider: prov, handler: handler,
		ownerID: ownerID, computer: computerID, agentID: textureAgentID, trajectID: researchRun.TrajectoryID,
		workID: "texture-work-" + suffix, encoded: encoded, research: researchRun, producer: queue.ProducerUpdateID,
		from: from, revisionID: revisionID,
	}
}

func (f *undisposedTriggerFixture) deliver(updateID, kind string) error {
	_, err := f.handler.HandleUpdate(f.ctx, f.agentID, actor.Update{
		UpdateID: updateID, ToAgentID: scopedActorMailboxID(f.ownerID, f.computer, f.agentID),
		FromAgentID: f.from, Kind: kind, Content: f.encoded,
		TrajectoryID: f.trajectID, CreatedAt: time.Now().UTC(),
	}, nil)
	return err
}

// deliverOwnerRevisionOutbox delivers the revision as the store's
// owner_revision outbox wake (its own content shape and update id).
func (f *undisposedTriggerFixture) deliverOwnerRevisionOutbox(updateID string) error {
	content, _ := json.Marshal(map[string]any{"revision_id": f.revisionID, "request_id": f.revisionID})
	_, err := f.handler.HandleUpdate(f.ctx, f.agentID, actor.Update{
		UpdateID: updateID, ToAgentID: scopedActorMailboxID(f.ownerID, f.computer, f.agentID),
		FromAgentID: f.from, Kind: "owner_revision", Content: string(content),
		TrajectoryID: f.trajectID, CreatedAt: time.Now().UTC(),
	}, nil)
	return err
}

func (f *undisposedTriggerFixture) report(t *testing.T) types.CoagentSourcePacket {
	t.Helper()
	update, err := f.store.GetLifecycleUpdate(f.ctx, f.ownerID, f.computer, f.trajectID, f.agentID, f.research.AgentID, f.producer)
	if err != nil {
		t.Fatal(err)
	}
	return update
}

func (f *undisposedTriggerFixture) ledgerHas(t *testing.T, kind types.LifecycleEventKind, substr string) bool {
	t.Helper()
	snapshot, err := f.store.GetLifecycleSnapshot(f.ctx, f.ownerID, f.computer, f.trajectID)
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range snapshot.Events {
		if ev.Kind == kind && strings.Contains(ev.Reason, substr) {
			return true
		}
	}
	return false
}

func (f *undisposedTriggerFixture) workStatus(t *testing.T) types.WorkItemRecord {
	t.Helper()
	work, err := f.store.GetLifecycleWorkItem(f.ctx, f.ownerID, f.computer, f.workID)
	if err != nil {
		t.Fatal(err)
	}
	return work
}

// Problem 17: one trigger carried by two actor updates (the outbox wake and
// the revise path's coagent_result) alternated, each re-running the model.
// The trigger gets one model turn; the other update and every re-delivery
// only re-check it.
func TestTextureUndisposedTriggerTwoUpdatesSpendOneModelTurn(t *testing.T) {
	f := newUndisposedTriggerFixture(t, "two-updates")
	first := f.deliver("actor-undisposed-a", "coagent_result")
	if !errors.Is(first, actor.ErrDeferUnprocessed) || !strings.Contains(first.Error(), "without disposing exact trigger") {
		t.Fatalf("first delivery = %v, want a deferral after the undisposed turn", first)
	}
	turns := f.provider.calls.Load()
	if turns == 0 {
		t.Fatalf("first delivery ran no model turn")
	}
	for i := 0; i < textureUndisposedRecheckLimit-1; i++ {
		id := "actor-undisposed-a"
		if i%2 == 1 {
			id = "actor-undisposed-b"
		}
		err := f.deliver(id, "coagent_result")
		if !errors.Is(err, actor.ErrDeferUnprocessed) || !strings.Contains(err.Error(), "no new model call") {
			t.Fatalf("re-delivery %d (%s) = %v, want a re-check deferral", i+1, id, err)
		}
		if got := f.provider.calls.Load(); got != turns {
			t.Fatalf("re-delivery %d (%s) called the model: calls %d, want %d", i+1, id, got, turns)
		}
	}
	if work := f.workStatus(t); work.Status != types.WorkItemOpen {
		t.Fatalf("work closed before the bound: %+v", work)
	}
}

// The bound for a producer report: the report is settled with the reason
// "trigger not disposed" (an update_late event on the trajectory's ledger), so
// it is not left pending for a later turn to consume unread, and Texture's
// work stays open for the owner. The sibling update is consumed with no model
// call.
func TestTextureUndisposedReportClosedAtBoundWithVisibleFate(t *testing.T) {
	f := newUndisposedTriggerFixture(t, "bound")
	if err := f.deliver("actor-bound-a", "coagent_result"); !errors.Is(err, actor.ErrDeferUnprocessed) {
		t.Fatalf("first delivery = %v", err)
	}
	turns := f.provider.calls.Load()
	var last error
	for i := 0; i < textureUndisposedRecheckLimit; i++ {
		last = f.deliver("actor-bound-a", "coagent_result")
	}
	if !errors.Is(last, actor.ErrDurableInvalid) || !strings.Contains(last.Error(), "trigger not disposed") {
		t.Fatalf("delivery at the bound = %v, want a durable-invalid trigger-not-disposed fate", last)
	}
	if report := f.report(t); report.Disposition == types.UpdatePending || !strings.Contains(report.DispositionReason, "trigger not disposed") {
		t.Fatalf("report fate = %+v, want settled with trigger not disposed", report)
	}
	if !f.ledgerHas(t, types.LifecycleUpdateLate, "trigger not disposed") {
		t.Fatalf("no trigger-not-disposed event on the trajectory ledger")
	}
	if work := f.workStatus(t); work.Status != types.WorkItemOpen {
		t.Fatalf("Texture work closed for a report's fate: %+v", work)
	}
	if err := f.deliver("actor-bound-b", "coagent_result"); err != nil {
		t.Fatalf("sibling update after the fate = %v, want consumed", err)
	}
	if got := f.provider.calls.Load(); got != turns {
		t.Fatalf("model calls after the first turn: %d, want %d", got, turns)
	}
}

// The production pair (problem 17): one owner revision as a coagent_result and
// as the outbox owner_revision. One model turn; at the bound the revision's
// Texture work is refused "trigger not disposed" (work_refused on the ledger);
// the other update is consumed with no model call.
func TestTextureUndisposedRevisionPairClosedAtBoundWithVisibleFate(t *testing.T) {
	f := newUndisposedTriggerFixtureKind(t, "revision-pair", agentcore.TextureActorOccurrenceDocumentRevision)
	if err := f.deliver("actor-revision-coagent", "coagent_result"); !errors.Is(err, actor.ErrDeferUnprocessed) || !strings.Contains(err.Error(), "without disposing exact trigger") {
		t.Fatalf("first delivery = %v, want a deferral after the undisposed turn", err)
	}
	turns := f.provider.calls.Load()
	if turns == 0 {
		t.Fatalf("first delivery ran no model turn")
	}
	var last error
	for i := 0; i < textureUndisposedRecheckLimit; i++ {
		if i%2 == 0 {
			last = f.deliverOwnerRevisionOutbox("actor-revision-outbox")
		} else {
			last = f.deliver("actor-revision-coagent", "coagent_result")
		}
		if got := f.provider.calls.Load(); got != turns {
			t.Fatalf("re-delivery %d called the model: %d, want %d", i+1, got, turns)
		}
		if i < textureUndisposedRecheckLimit-1 && (!errors.Is(last, actor.ErrDeferUnprocessed) || !strings.Contains(last.Error(), "no new model call")) {
			t.Fatalf("re-delivery %d = %v, want a re-check deferral of the same trigger", i+1, last)
		}
	}
	if !errors.Is(last, actor.ErrDurableInvalid) || !strings.Contains(last.Error(), "trigger not disposed") {
		t.Fatalf("delivery at the bound = %v, want the trigger-not-disposed fate", last)
	}
	if work := f.workStatus(t); work.Status != types.WorkItemRefused || !strings.HasPrefix(work.Reason, textureowner.TriggerNotDisposedReason) {
		t.Fatalf("work fate = %+v, want refused with trigger not disposed", work)
	}
	if !f.ledgerHas(t, types.LifecycleWorkRefused, "trigger not disposed") {
		t.Fatalf("no work_refused trigger-not-disposed event on the trajectory ledger")
	}
	if err := f.deliverOwnerRevisionOutbox("actor-revision-outbox"); err != nil {
		t.Fatalf("sibling after the fate = %v, want consumed", err)
	}
	if got := f.provider.calls.Load(); got != turns {
		t.Fatalf("model calls after the first turn: %d, want %d", got, turns)
	}
}

// The wait half of the bound: six quick re-checks (two updates sharing the
// count) do not close the trigger before the minimum wait has passed.
func TestTextureUndisposedTriggerWaitsBeforeTheFate(t *testing.T) {
	f := newUndisposedTriggerFixture(t, "wait")
	textureUndisposedMinWait = time.Hour
	if err := f.deliver("actor-wait-a", "coagent_result"); !errors.Is(err, actor.ErrDeferUnprocessed) {
		t.Fatalf("first delivery = %v", err)
	}
	for i := 0; i < 2*textureUndisposedRecheckLimit; i++ {
		if err := f.deliver("actor-wait-a", "coagent_result"); !errors.Is(err, actor.ErrDeferUnprocessed) {
			t.Fatalf("re-check %d before the wait = %v, want a deferral", i+1, err)
		}
	}
	if report := f.report(t); report.Disposition != types.UpdatePending {
		t.Fatalf("report closed before the wait: %+v", report)
	}
}

// A trigger whose work another path closed after its undisposed turn is
// consumed on its next re-check, not given a second fate.
func TestTextureUndisposedTriggerWorkClosedElsewhereIsConsumed(t *testing.T) {
	f := newUndisposedTriggerFixture(t, "disposed-elsewhere")
	if err := f.deliver("actor-elsewhere-a", "coagent_result"); !errors.Is(err, actor.ErrDeferUnprocessed) {
		t.Fatalf("first delivery = %v", err)
	}
	turns := f.provider.calls.Load()
	// Another path closes the trigger's work first (an owner act, or a
	// sibling delivery's fate).
	refuse := types.RefuseLifecycleWorkRequest{OwnerID: f.ownerID, ComputerID: f.computer, CommandID: "refuse-elsewhere", TrajectoryID: f.trajectID, WorkItemID: f.workID, ActingAgentID: f.agentID, RefusalRef: "elsewhere", Reason: "closed elsewhere"}
	refuse.CommandDigest, _ = store.ComputeRefuseLifecycleWorkDigest(refuse)
	if _, err := f.store.RefuseLifecycleWork(f.ctx, refuse); err != nil {
		t.Fatalf("refuse: %v", err)
	}
	if err := f.deliver("actor-elsewhere-a", "coagent_result"); err != nil {
		t.Fatalf("re-check after the work closed = %v, want consumed", err)
	}
	if got := f.provider.calls.Load(); got != turns {
		t.Fatalf("re-check called the model: %d, want %d", got, turns)
	}
	if work := f.workStatus(t); work.Reason != "closed elsewhere" {
		t.Fatalf("work fate overwritten: %+v", work)
	}
}

// A trigger disposed elsewhere (here the report settled by another path)
// resolves terminal on its re-check: consumed, the record forgotten.
func TestTextureUndisposedTriggerDisposedElsewhereGoesTerminal(t *testing.T) {
	f := newUndisposedTriggerFixture(t, "terminal")
	if err := f.deliver("actor-terminal-a", "coagent_result"); !errors.Is(err, actor.ErrDeferUnprocessed) {
		t.Fatalf("first delivery = %v", err)
	}
	report := f.report(t)
	settle := types.SettleLifecycleProducerReportsRequest{OwnerID: f.ownerID, ComputerID: f.computer, CommandID: "settle-elsewhere", TrajectoryID: f.trajectID, ReportIDs: []string{report.UpdateID}, Reason: "settled elsewhere", IncludeDeliveredStale: true}
	settle.CommandDigest, _ = store.ComputeSettleLifecycleProducerReportsDigest(settle)
	if _, err := f.store.SettleLifecycleProducerReports(f.ctx, settle); err != nil {
		t.Fatalf("settle: %v", err)
	}
	if err := f.deliver("actor-terminal-a", "coagent_result"); err != nil {
		t.Fatalf("re-check after disposal = %v, want consumed", err)
	}
	if f.handler.undisposed.hadTurn(textureTriggerKeyFromEncoded(t, f.encoded)) {
		t.Fatalf("record kept after the trigger went terminal")
	}
}

func textureTriggerKeyFromEncoded(t *testing.T, encoded string) string {
	t.Helper()
	o, err := agentcore.DecodeTextureActorOccurrence(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return textureTriggerKey(o)
}

func TestTextureTriggerKeyIgnoresDeliveryIdentity(t *testing.T) {
	base := agentcore.TextureActorOccurrence{Kind: agentcore.TextureActorOccurrenceDocumentRevision, OwnerID: "o", ComputerID: "c", TrajectoryID: "t", TargetAgentID: "texture:d", DocumentID: "d", HeadRevisionID: "r1"}
	outbox, revise := base, base
	outbox.RequestID, outbox.ReducerSeq = "r1", 7
	revise.RequestID, revise.ReducerSeq = "client-request", 0
	if textureTriggerKey(outbox) != textureTriggerKey(revise) {
		t.Fatalf("one owner revision delivered twice has two trigger keys")
	}
	newer := base
	newer.HeadRevisionID = "r2"
	if textureTriggerKey(newer) == textureTriggerKey(base) {
		t.Fatalf("a newer revision shares the older trigger's key")
	}
}

// Failure mode 11: a run parked for a runtime failure, or one whose idle
// stamp is from an earlier activation, did not spend a turn on this trigger:
// the retry re-executes it as before.
func TestTextureTurnParkedIdleSinceOnlyCountsThisActivationsIdlePark(t *testing.T) {
	start := time.Now().UTC()
	idle := types.RunRecord{State: types.RunPassivated, Metadata: map[string]any{"actor_sleep_state": "idle", "actor_sleep_at": start.Add(time.Second).Format(time.RFC3339Nano)}}
	if !textureTurnParkedIdleSince(idle, start) {
		t.Fatalf("idle park in this activation not counted")
	}
	stale := types.RunRecord{State: types.RunPassivated, Metadata: map[string]any{"actor_sleep_state": "idle", "actor_sleep_at": start.Add(-time.Minute).Format(time.RFC3339Nano)}}
	if textureTurnParkedIdleSince(stale, start) {
		t.Fatalf("an earlier activation's idle park counted")
	}
	failure := types.RunRecord{State: types.RunPassivated, Metadata: map[string]any{"actor_sleep_state": "retryable_runtime_failure", "actor_sleep_at": start.Add(time.Second).Format(time.RFC3339Nano)}}
	if textureTurnParkedIdleSince(failure, start) {
		t.Fatalf("a runtime-failure park counted as a turn")
	}
}

func TestTextureUndisposedRuntimeFailureParkKeepsTheRetry(t *testing.T) {
	f := newUndisposedTriggerFixture(t, "runtime-failure")
	f.handler.textureExecute = func(ctx context.Context, rec *types.RunRecord) error {
		rec.State = types.RunPassivated
		rec.UpdatedAt = time.Now().UTC()
		if rec.Metadata == nil {
			rec.Metadata = map[string]any{}
		}
		rec.Metadata["passivated_reason"] = "runtime_injection_append_failed"
		rec.Metadata["actor_sleep_state"] = "retryable_runtime_failure"
		return f.store.UpdateRun(ctx, *rec)
	}
	if err := f.deliver("actor-runtime-failure", "coagent_result"); !errors.Is(err, actor.ErrDeferUnprocessed) {
		t.Fatalf("first delivery = %v", err)
	}
	if f.handler.undisposed.hadTurn(textureTriggerKeyFromEncoded(t, f.encoded)) {
		t.Fatalf("a runtime-failure park was recorded as a spent turn")
	}
}
