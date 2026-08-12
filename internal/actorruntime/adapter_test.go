package actorruntime

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/actor"
	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/textureowner"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// adapterTestEnv holds the common test infrastructure.
type adapterTestEnv struct {
	t       *testing.T
	adapter *Adapter
	store   *store.Store
	ctx     context.Context
	cancel  context.CancelFunc
}

type startupBlockingProvider struct {
	started chan struct{}
	release chan struct{}
}

func (p *startupBlockingProvider) Execute(ctx context.Context, task *types.RunRecord, _ provideriface.EventEmitFunc) error {
	select {
	case p.started <- struct{}{}:
	default:
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.release:
		task.Result = "startup recovery completed"
		return nil
	}
}

func (p *startupBlockingProvider) ProviderName() string { return "startup-blocking" }

type targetStartupBlockingProvider struct {
	targetAgentID string
	started       chan struct{}
	release       chan struct{}
}

func (p *targetStartupBlockingProvider) Execute(ctx context.Context, task *types.RunRecord, _ provideriface.EventEmitFunc) error {
	if task.AgentID == p.targetAgentID {
		select {
		case p.started <- struct{}{}:
		default:
		}
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.release:
		task.Result = "startup recovery completed"
		return nil
	}
}

func (p *targetStartupBlockingProvider) ProviderName() string { return "target-startup-blocking" }

type countingLifecycleProvider struct {
	targetAgentID string
	calls         atomic.Int32
}

func (p *countingLifecycleProvider) Execute(_ context.Context, task *types.RunRecord, _ provideriface.EventEmitFunc) error {
	if p.targetAgentID == "" || task.AgentID == p.targetAgentID {
		p.calls.Add(1)
	}
	task.Result = "lifecycle control executed"
	return nil
}

func (p *countingLifecycleProvider) ProviderName() string { return "counting-lifecycle" }

func seedDurableTextureUpdate(t *testing.T, s *store.Store, ctx context.Context, computerID, ownerID, docID, updateID, content string) types.QueueLifecycleUpdateRequest {
	t.Helper()
	agentID := "texture:" + docID
	now := time.Now().UTC()
	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "start:" + docID, TrajectoryID: "trajectory:" + docID,
		Kind:            types.TrajectoryKindDocument,
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		SubjectRefs:     map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		InitialWork:     types.WorkItemRecord{WorkItemID: "work:" + docID, Objective: "incorporate durable update", AssignedAgentID: agentID},
		InitialDocument: types.Document{DocID: docID, Title: "Durable Texture target"},
		InitialRevision: types.Revision{
			RevisionID: "revision:" + docID, AuthorKind: types.AuthorUser, AuthorLabel: "user", Content: "Initial durable content",
		},
		Agent: types.AgentRecord{
			AgentID: agentID, OwnerID: ownerID, ComputerID: computerID,
			Profile: "texture", Role: "texture", ChannelID: docID, CreatedAt: now, UpdatedAt: now,
		},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start durable lifecycle: %v", err)
	}
	producerAgentID := "researcher:" + docID
	producerWorkID := "producer-work:" + docID
	producerRunID := "producer-run:" + docID
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID: producerAgentID, OwnerID: ownerID, ComputerID: computerID,
		Profile: "researcher", Role: "researcher", ChannelID: docID, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed lifecycle producer: %v", err)
	}
	open := types.OpenLifecycleWorkRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "open-producer:" + docID,
		TrajectoryID: start.TrajectoryID,
		WorkItem:     types.WorkItemRecord{WorkItemID: producerWorkID, Objective: "produce durable update", AssignedAgentID: producerAgentID, AuthorityProfile: "researcher"},
	}
	open.CommandDigest, _ = store.ComputeOpenLifecycleWorkDigest(open)
	if _, err := s.OpenLifecycleWork(ctx, open); err != nil {
		t.Fatalf("open lifecycle producer work: %v", err)
	}
	producerRun := types.RunRecord{
		RunID: producerRunID, AgentID: producerAgentID, ChannelID: docID, TrajectoryID: start.TrajectoryID,
		AgentProfile: "researcher", AgentRole: "researcher", OwnerID: ownerID, ComputerID: computerID,
		State: types.RunRunning, CreatedAt: now, UpdatedAt: now, Metadata: map[string]any{"lifecycle_work_item_id": producerWorkID},
	}
	projectProducer := types.ReplaceLifecycleActivationRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "project-producer:" + docID,
		TrajectoryID: start.TrajectoryID, AgentID: producerAgentID, Run: producerRun,
	}
	projectProducer.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(projectProducer)
	if _, err := s.ReplaceLifecycleActivation(ctx, projectProducer); err != nil {
		t.Fatalf("project lifecycle producer: %v", err)
	}
	packet := types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "evidence_update", Summary: content}
	payloadDigest, _ := store.ComputeLifecycleUpdatePayloadDigest(packet, content)
	queue := types.QueueLifecycleUpdateRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "queue:" + updateID, TrajectoryID: start.TrajectoryID,
		TargetAgentID: agentID, ProducerAgentID: producerAgentID, ProducerUpdateID: updateID, UpdateID: updateID,
		ChannelID: docID, Role: "researcher", SourceRunID: producerRunID,
		WorkItemID: producerWorkID, WorkDisposition: types.WorkItemOpen,
		Packet: packet, Content: content, PayloadDigest: payloadDigest,
	}
	queue.CommandDigest, _ = store.ComputeQueueLifecycleUpdateDigest(queue)
	if _, err := s.QueueLifecycleUpdate(ctx, queue); err != nil {
		t.Fatalf("queue durable lifecycle update: %v", err)
	}
	return queue
}

type actorLifecycleControlFixture struct {
	ownerID, computerID, trajectoryID, docID string
	agentID, workID                          string
	control                                  types.CoagentSourcePacket
}

func seedActorLifecycleControl(t *testing.T, s *store.Store, suffix string) actorLifecycleControlFixture {
	t.Helper()
	ctx := context.Background()
	ownerID, computerID := "owner-actor-"+suffix, "autoputer-test"
	docID, trajectoryID := "doc-actor-"+suffix, "trajectory-actor-"+suffix
	textureAgentID := agentprofile.Texture + ":" + docID
	now := time.Now().UTC()
	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "start-actor-" + suffix, TrajectoryID: trajectoryID, Kind: types.TrajectoryKindDocument,
		SubjectRefs:     map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:     types.WorkItemRecord{WorkItemID: "texture-work-actor-" + suffix, Objective: "author direction", AssignedAgentID: textureAgentID, AuthorityProfile: agentprofile.Texture},
		InitialDocument: types.Document{DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID, Title: "Actor parked wake", CreatedAt: now, UpdatedAt: now},
		InitialRevision: types.Revision{RevisionID: "revision-actor-" + suffix, DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID, AuthorKind: types.AuthorUser, AuthorLabel: ownerID, Content: "initial", CreatedAt: now},
		Agent:           types.AgentRecord{AgentID: textureAgentID, OwnerID: ownerID, ComputerID: computerID, Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: docID, CreatedAt: now, UpdatedAt: now},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start actor lifecycle: %v", err)
	}
	caller := types.RunRecord{
		RunID: "texture-run-actor-" + suffix, OwnerID: ownerID, ComputerID: computerID, AgentID: textureAgentID,
		AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture, ChannelID: docID, TrajectoryID: trajectoryID,
		State: types.RunRunning, Metadata: map[string]any{"lifecycle_work_item_id": start.InitialWork.WorkItemID, "work_item_ids": []string{start.InitialWork.WorkItemID}}, CreatedAt: now, UpdatedAt: now,
	}
	project := types.ReplaceLifecycleActivationRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "project-texture-actor-" + suffix,
		TrajectoryID: trajectoryID, AgentID: textureAgentID, Run: caller,
	}
	project.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(project)
	if _, err := s.ReplaceLifecycleActivation(ctx, project); err != nil {
		t.Fatalf("project actor Texture run: %v", err)
	}
	agentID, workID := "researcher:actor-"+suffix, "research-work-actor-"+suffix
	packet := types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "question", Summary: "research exact gap", Questions: []string{"What evidence resolves it?"}}
	content := "research exact gap"
	payloadDigest, _ := store.ComputeLifecycleUpdatePayloadDigest(packet, content)
	snapshot, _ := s.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	textureAgent, _ := s.GetAgentByScope(ctx, ownerID, computerID, textureAgentID)
	turn := types.ApplyTextureTurnRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "turn-actor-" + suffix, DocumentID: docID, TrajectoryID: trajectoryID,
		CallerAgentID: textureAgentID, CallerRunID: caller.RunID, ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion, ExpectedCallerLifecycleVersion: textureAgent.LifecycleVersion,
		ExpectedHeadRevisionID: snapshot.HeadRevision.RevisionID, CallerWorkItemID: start.InitialWork.WorkItemID, CallerWorkDisposition: types.WorkItemOpen,
		Outcome: types.TextureTurnWait, Reason: "wait",
		Controls: []types.TextureTurnControl{{
			ControlID: "control-actor-" + suffix, TargetAgentID: agentID, TargetWorkItemID: workID, Packet: packet, Content: content, PayloadDigest: payloadDigest,
			OpenAgent: &types.AgentRecord{AgentID: agentID, Profile: agentprofile.Researcher, Role: agentprofile.Researcher, ChannelID: docID},
			OpenWork:  &types.WorkItemRecord{WorkItemID: workID, Objective: "research exact gap", AuthorityProfile: agentprofile.Researcher, AssignedAgentID: agentID},
		}},
	}
	turn.CommandDigest, _ = store.ComputeApplyTextureTurnDigest(turn)
	result, err := s.ApplyTextureTurn(ctx, turn)
	if err != nil || len(result.Controls) != 1 {
		t.Fatalf("apply actor Texture turn: result=%+v err=%v", result, err)
	}
	return actorLifecycleControlFixture{ownerID: ownerID, computerID: computerID, trajectoryID: trajectoryID, docID: docID, agentID: agentID, workID: workID, control: result.Controls[0]}
}

func commitActorLaterControl(t *testing.T, s *store.Store, fixture actorLifecycleControlFixture, suffix, controlID string) types.CoagentSourcePacket {
	t.Helper()
	ctx := context.Background()
	snapshot, err := s.GetLifecycleSnapshot(ctx, fixture.ownerID, fixture.computerID, fixture.trajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	textureAgentID := agentprofile.Texture + ":" + fixture.docID
	textureAgent, err := s.GetAgentByScope(ctx, fixture.ownerID, fixture.computerID, textureAgentID)
	if err != nil {
		t.Fatal(err)
	}
	packet := types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "question", Summary: "later parked question", Questions: []string{"Was this recovered?"}}
	content := "later parked control " + controlID
	payloadDigest, _ := store.ComputeLifecycleUpdatePayloadDigest(packet, content)
	turn := types.ApplyTextureTurnRequest{
		OwnerID: fixture.ownerID, ComputerID: fixture.computerID, CommandID: "turn-" + controlID, DocumentID: fixture.docID, TrajectoryID: fixture.trajectoryID,
		CallerAgentID: textureAgentID, CallerRunID: "texture-run-actor-" + suffix, ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion, ExpectedCallerLifecycleVersion: textureAgent.LifecycleVersion,
		ExpectedHeadRevisionID: snapshot.HeadRevision.RevisionID, CallerWorkItemID: "texture-work-actor-" + suffix, CallerWorkDisposition: types.WorkItemOpen,
		Outcome: types.TextureTurnWait, Reason: "wait after later control",
		Controls: []types.TextureTurnControl{{ControlID: controlID, TargetAgentID: fixture.agentID, TargetWorkItemID: fixture.workID, Packet: packet, Content: content, PayloadDigest: payloadDigest}},
	}
	turn.CommandDigest, _ = store.ComputeApplyTextureTurnDigest(turn)
	result, err := s.ApplyTextureTurn(ctx, turn)
	if err != nil || len(result.Controls) != 1 {
		t.Fatalf("apply later actor control: result=%+v err=%v", result, err)
	}
	return result.Controls[0]
}

func newAdapterTestEnv(t *testing.T) *adapterTestEnv {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	promptRoot := filepath.Join(dir, "prompts")

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	cfg := provideriface.Config{
		ComputerID:          "autoputer-test",
		StorePath:           dbPath,
		PromptRoot:          promptRoot,
		ProviderTimeout:     time.Second,
		SupervisionInterval: time.Hour,
	}

	adapter := New(cfg, s, events.NewEventBus(), provider.NewStubProvider(0), nil)
	t.Cleanup(func() {
		adapter.Stop()
		adapter.cleanupLog()
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("start adapter: %v", err)
	}

	return &adapterTestEnv{t: t, adapter: adapter, store: s, ctx: ctx, cancel: cancel}
}

func TestInitialDispatchUpdateIdentityIsStableAndScoped(t *testing.T) {
	first := actorDispatchUpdateID("owner-a", "computer-a", "agent-a", "initial_dispatch", "run-a", "", "")
	replay := actorDispatchUpdateID("owner-a", "computer-a", "agent-a", "initial_dispatch", "run-a", "", "")
	if first != replay {
		t.Fatalf("initial dispatch replay IDs differ: %q != %q", first, replay)
	}
	for name, changed := range map[string]string{
		"owner":    actorDispatchUpdateID("owner-b", "computer-a", "agent-a", "initial_dispatch", "run-a", "", ""),
		"computer": actorDispatchUpdateID("owner-a", "computer-b", "agent-a", "initial_dispatch", "run-a", "", ""),
		"agent":    actorDispatchUpdateID("owner-a", "computer-a", "agent-b", "initial_dispatch", "run-a", "", ""),
		"run":      actorDispatchUpdateID("owner-a", "computer-a", "agent-a", "initial_dispatch", "run-b", "", ""),
	} {
		if changed == first {
			t.Fatalf("%s scope change reused initial dispatch ID %q", name, first)
		}
	}
}

func TestLifecycleCoagentResultUpdateIdentityUsesExactCanonicalOccurrence(t *testing.T) {
	content := "control-a\x1fproducer-update-a"
	first := actorDispatchUpdateID("owner-a", "computer-a", "researcher-a", "coagent_result", content, "trajectory-a", "texture-a")
	replay := actorDispatchUpdateID("owner-a", "computer-a", "researcher-a", "coagent_result", content, "trajectory-a", "texture-a")
	if first != replay {
		t.Fatalf("exact occurrence replay IDs differ: %q != %q", first, replay)
	}
	for name, changed := range map[string]string{
		"owner":           actorDispatchUpdateID("owner-b", "computer-a", "researcher-a", "coagent_result", content, "trajectory-a", "texture-a"),
		"computer":        actorDispatchUpdateID("owner-a", "computer-b", "researcher-a", "coagent_result", content, "trajectory-a", "texture-a"),
		"target":          actorDispatchUpdateID("owner-a", "computer-a", "researcher-b", "coagent_result", content, "trajectory-a", "texture-a"),
		"kind":            actorDispatchUpdateID("owner-a", "computer-a", "researcher-a", "initial_dispatch", content, "trajectory-a", "texture-a"),
		"trajectory":      actorDispatchUpdateID("owner-a", "computer-a", "researcher-a", "coagent_result", content, "trajectory-b", "texture-a"),
		"producer":        actorDispatchUpdateID("owner-a", "computer-a", "researcher-a", "coagent_result", content, "trajectory-a", "texture-b"),
		"producer_update": actorDispatchUpdateID("owner-a", "computer-a", "researcher-a", "coagent_result", "control-a\x1fproducer-update-b", "trajectory-a", "texture-a"),
	} {
		if changed == first {
			t.Fatalf("%s change reused actor update ID %q", name, first)
		}
	}
	// Non-lifecycle/generic coagent wakes remain independently retryable.
	if one, two := actorDispatchUpdateID("owner-a", "computer-a", "agent-a", "coagent_result", "legacy", "", ""), actorDispatchUpdateID("owner-a", "computer-a", "agent-a", "coagent_result", "legacy", "", ""); one == two {
		t.Fatalf("generic coagent_result unexpectedly deduplicated: %q", one)
	}
}

func TestLifecycleCoagentResultUpdateIdentityResistsDelimiterCollision(t *testing.T) {
	leftFields := []string{"choir:coagent-result", "owner-a", "computer-a", "researcher-a", "trajectory-a", "texture-a\x00control-a", "producer-update-a"}
	rightFields := []string{"choir:coagent-result", "owner-a", "computer-a", "researcher-a", "trajectory-a", "texture-a", "control-a\x00producer-update-a"}
	if left, right := strings.Join(leftFields, "\x00"), strings.Join(rightFields, "\x00"); left != right {
		t.Fatalf("adversarial fixture does not collide under delimiter concatenation: %q != %q", left, right)
	}

	left := actorDispatchUpdateID("owner-a", "computer-a", "researcher-a", "coagent_result", "producer-update-a", "trajectory-a", "texture-a\x00control-a")
	right := actorDispatchUpdateID("owner-a", "computer-a", "researcher-a", "coagent_result", "control-a\x00producer-update-a", "trajectory-a", "texture-a")
	if left == right {
		t.Fatalf("distinct lifecycle occurrences reused actor update ID %q", left)
	}
}

func TestAcknowledgeDurablyTerminalLifecycleControlActivation(t *testing.T) {
	terminal := agentcore.ErrDurablyTerminalLifecycleControlActivation
	for name, err := range map[string]error{
		"sentinel": terminal,
		"wrapped":  fmt.Errorf("reconcile lifecycle control: %w", terminal),
	} {
		if got := acknowledgeDurablyTerminalLifecycleControlActivation(err); got != nil {
			t.Errorf("%s error was not acknowledged: %v", name, got)
		}
	}

	if got := acknowledgeDurablyTerminalLifecycleControlActivation(nil); got != nil {
		t.Errorf("nil error = %v, want nil", got)
	}
	for name, retryable := range map[string]error{
		"ordinary":  errors.New("temporary reconcile failure"),
		"same text": errors.New(terminal.Error()),
	} {
		if got := acknowledgeDurablyTerminalLifecycleControlActivation(retryable); got != retryable {
			t.Errorf("%s error = %v, want original retryable error %v", name, got, retryable)
		}
	}
}
func TestAdapterRestartResumesRunningLifecycleActivationFromDurableBacklog(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "restart-running.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	ownerID, docID := "owner-running-restart", "doc-running-restart"
	queue := seedDurableTextureUpdate(t, s, ctx, "autoputer-test", ownerID, docID, "update-running-restart", "durable update")
	now := time.Now().UTC()
	run := types.RunRecord{
		RunID: "run-running-restart", AgentID: queue.TargetAgentID, OwnerID: ownerID,
		ComputerID: "autoputer-test", ChannelID: docID, TrajectoryID: queue.TrajectoryID,
		State: types.RunRunning, Prompt: "resume after process crash", AgentProfile: "texture", AgentRole: "texture",
		CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{
			"type": "texture_agent_revision", "current_revision_id": "revision:" + docID,
			"agent_profile": "texture", "agent_role": "texture", "doc_id": docID,
			"trajectory_id": queue.TrajectoryID, "lifecycle_work_item_id": "work:" + docID,
		},
	}
	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatalf("project running lifecycle activation: %v", err)
	}
	if err := s.CreateAgentMutation(ctx, store.AgentMutation{
		DocID: docID, RunID: run.RunID, OwnerID: ownerID, ComputerID: "autoputer-test",
		State: "pending", RevisionID: "revision:" + docID, CreatedAt: now.Add(-time.Hour),
	}); err != nil {
		t.Fatalf("create running Texture mutation: %v", err)
	}

	logDB, err := sql.Open("sqlite", actorLogPath(dbPath)+"?_busy_timeout=60000")
	if err != nil {
		t.Fatalf("open actor log: %v", err)
	}
	actorLog, err := actor.NewSQLiteLog(logDB)
	if err != nil {
		_ = logDB.Close()
		t.Fatalf("initialize actor log: %v", err)
	}
	mailboxID := scopedActorMailboxID(ownerID, "autoputer-test", run.AgentID)
	dispatch := actor.Update{
		UpdateID:  actorDispatchUpdateID(ownerID, "autoputer-test", run.AgentID, "initial_dispatch", run.RunID, "", ""),
		ToAgentID: mailboxID, Kind: "initial_dispatch", Content: run.RunID,
		TrajectoryID: run.TrajectoryID, CreatedAt: now,
	}
	if appended, err := actorLog.Append(ctx, dispatch); err != nil || !appended {
		_ = logDB.Close()
		t.Fatalf("seed unprocessed initial dispatch: appended=%v err=%v", appended, err)
	}
	if err := logDB.Close(); err != nil {
		t.Fatalf("close seeded actor log: %v", err)
	}

	cfg := provideriface.Config{
		ComputerID: "autoputer-test", StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts"),
		ProviderTimeout: time.Second, SupervisionInterval: time.Hour,
	}
	adapter := New(cfg, s, events.NewEventBus(), provider.NewStubProvider(0), nil)
	if err := adapter.BindTextureOwner(textureowner.NewHandler(adapter.Runtime)); err != nil {
		t.Fatalf("bind Texture owner: %v", err)
	}
	t.Cleanup(func() {
		adapter.Stop()
		adapter.cleanupLog()
	})
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("restart adapter: %v", err)
	}
	var backlog []actor.Update
	seededDispatchPending := true
	backlogDeadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(backlogDeadline) {
		backlog, err = adapter.log.Unprocessed(ctx, mailboxID)
		seededDispatchPending = false
		for _, pending := range backlog {
			if pending.UpdateID == dispatch.UpdateID {
				seededDispatchPending = true
				break
			}
		}
		if err == nil && !seededDispatchPending {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil || seededDispatchPending {
		t.Fatalf("durable initial dispatch %q remained pending in %+v, %v", dispatch.UpdateID, backlog, err)
	}
	stored, err := s.GetLifecycleRun(ctx, ownerID, "autoputer-test", run.RunID)
	mutation, mutationErr := s.GetAgentMutationByRun(ctx, ownerID, "autoputer-test", run.RunID)
	reactivated, _ := stored.Metadata["actor_reactivated_from_passivated"].(bool)
	if err != nil || stored.State != types.RunCompleted || !reactivated || mutationErr != nil || mutation == nil ||
		mutation.State != "completed" || !mutation.CreatedAt.After(now.Add(-time.Minute)) {
		t.Fatalf("running lifecycle activation was not owner-reactivated: run=%+v mutation=%+v run_err=%v mutation_err=%v", stored, mutation, err, mutationErr)
	}
}

func TestAdapterStartRecoversLifecycleActorSnapshotsBeforeRuntimeSweep(t *testing.T) {
	for _, tc := range []struct {
		name              string
		state             types.RunState
		durableOccurrence bool
	}{
		{name: "passivated missing send", state: types.RunPassivated},
		{name: "passivated durable unprocessed occurrence", state: types.RunPassivated, durableOccurrence: true},
		{name: "blocked missing send", state: types.RunBlocked},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			dir := t.TempDir()
			dbPath := filepath.Join(dir, "restart-snapshot.db")
			s, err := store.Open(dbPath)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = s.Close() })
			cfg := provideriface.Config{
				ComputerID: "autoputer-test", StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts"),
				ProviderTimeout: time.Second, SupervisionInterval: time.Hour,
			}

			seed := New(cfg, s, events.NewEventBus(), provider.NewStubProvider(0), nil)
			seed.Runtime.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })
			suffix := strings.ReplaceAll(tc.name, " ", "-")
			fixture := seedActorLifecycleControl(t, s, suffix)
			initial, err := seed.Runtime.ReconcileCoagentWake(ctx, fixture.ownerID, fixture.agentID)
			if err != nil || initial == nil {
				t.Fatalf("seed initial lifecycle run=%+v err=%v", initial, err)
			}
			initial.State = tc.state
			initial.UpdatedAt = time.Now().UTC()
			if err := s.UpdateRun(ctx, *initial); err != nil {
				t.Fatalf("seed lifecycle run state %s: %v", tc.state, err)
			}
			mailboxID := scopedActorMailboxID(fixture.ownerID, fixture.computerID, fixture.agentID)
			initialDispatch := actor.Update{
				UpdateID:  actorDispatchUpdateID(fixture.ownerID, fixture.computerID, fixture.agentID, "initial_dispatch", initial.RunID, fixture.trajectoryID, ""),
				ToAgentID: mailboxID, Kind: "initial_dispatch", Content: initial.RunID, TrajectoryID: fixture.trajectoryID, CreatedAt: time.Now().UTC(),
			}
			if appended, err := seed.log.Append(ctx, initialDispatch); err != nil || !appended {
				t.Fatalf("seed processed initial dispatch appended=%v err=%v", appended, err)
			}
			if err := seed.log.MarkProcessed(ctx, mailboxID, initialDispatch.UpdateID); err != nil {
				t.Fatalf("mark seed initial dispatch processed: %v", err)
			}
			memory, _ := json.Marshal(resumeState{RunID: initial.RunID, Phase: "parked"})
			if err := seed.log.SaveSnapshot(ctx, mailboxID, memory); err != nil {
				t.Fatalf("seed actor memory: %v", err)
			}
			later := commitActorLaterControl(t, s, fixture, suffix, "control-restart-snapshot-b")
			occurrenceContent := agentcore.LifecycleControlActorOccurrenceContent(later)
			occurrenceID := actorDispatchUpdateID(fixture.ownerID, fixture.computerID, fixture.agentID, "coagent_result", occurrenceContent, fixture.trajectoryID, later.AgentID)
			if tc.durableOccurrence {
				appended, appendErr := seed.log.Append(ctx, actor.Update{
					UpdateID: occurrenceID, ToAgentID: mailboxID, FromAgentID: later.AgentID, Kind: "coagent_result",
					Content: occurrenceContent, TrajectoryID: fixture.trajectoryID, CreatedAt: time.Now().UTC(),
				})
				if appendErr != nil || !appended {
					t.Fatalf("seed durable B occurrence appended=%v err=%v", appended, appendErr)
				}
			}
			before, err := seed.log.Unprocessed(ctx, mailboxID)
			if err != nil {
				t.Fatal(err)
			}
			foundBefore := false
			for _, update := range before {
				foundBefore = foundBefore || update.UpdateID == occurrenceID
			}
			if foundBefore != tc.durableOccurrence {
				t.Fatalf("pre-restart durable B occurrence=%v, want %v: %+v", foundBefore, tc.durableOccurrence, before)
			}
			seed.Stop()

			blocking := &targetStartupBlockingProvider{targetAgentID: fixture.agentID, started: make(chan struct{}, 1), release: make(chan struct{})}
			t.Cleanup(func() { close(blocking.release) })
			restarted := New(cfg, s, events.NewEventBus(), blocking, nil)
			if err := restarted.BindTextureOwner(textureowner.NewHandler(restarted.Runtime)); err != nil {
				t.Fatalf("bind restart Texture owner: %v", err)
			}
			t.Cleanup(func() {
				restarted.Stop()
				restarted.cleanupLog()
			})
			if err := restarted.Start(ctx); err != nil {
				t.Fatalf("restart adapter: %v", err)
			}
			bound, boundErr := s.GetLifecycleUpdate(ctx, fixture.ownerID, fixture.computerID, fixture.trajectoryID, fixture.agentID, later.AgentID, later.ProducerUpdateID)
			stored, storedErr := s.GetLifecycleRun(ctx, fixture.ownerID, fixture.computerID, initial.RunID)
			var occurrenceCount int
			countErr := restarted.logDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM actor_updates WHERE update_id = ?`, occurrenceID).Scan(&occurrenceCount)
			if boundErr != nil || storedErr != nil || countErr != nil || bound.DeliveredToRunID != initial.RunID || bound.DeliveredAt == nil || occurrenceCount != 1 ||
				metadataString(stored.Metadata, "request_source") != "lifecycle_texture_control" {
				t.Fatalf("restart recovery state=%+v B=%+v occurrence_count=%d stored_err=%v bound_err=%v count_err=%v", stored, bound, occurrenceCount, storedErr, boundErr, countErr)
			}
			runs, err := s.ListLifecycleRunsByTrajectory(ctx, fixture.ownerID, fixture.computerID, fixture.trajectoryID, 0)
			if err != nil {
				t.Fatal(err)
			}
			lifecycleRuns := 0
			for _, run := range runs {
				if run.AgentID == fixture.agentID && metadataString(run.Metadata, "request_source") == "lifecycle_texture_control" {
					lifecycleRuns++
				}
			}
			if lifecycleRuns != 1 {
				t.Fatalf("restart created %d lifecycle-control runs: %+v", lifecycleRuns, runs)
			}
		})
	}
}

func TestAdapterStartSerializesTextureOwnerRecoveryBeforeActorDelivery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "startup.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	blocking := &startupBlockingProvider{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	cfg := provideriface.Config{
		ComputerID:          "autoputer-startup",
		StorePath:           dbPath,
		PromptRoot:          filepath.Join(dir, "prompts"),
		ProviderTimeout:     10 * time.Second,
		SupervisionInterval: time.Hour,
	}
	adapter := New(cfg, s, events.NewEventBus(), blocking, nil)
	t.Cleanup(func() {
		adapter.Stop()
		adapter.cleanupLog()
		_ = s.Close()
	})
	t.Cleanup(func() { close(blocking.release) })

	const (
		ownerID = "user-startup-recovery"
		docID   = "doc-startup-recovery"
		agentID = "texture:" + docID
	)
	seedDurableTextureUpdate(t, s, ctx, "autoputer-startup", ownerID, docID, "update-startup-recovery", "Durable startup finding")
	if runs, err := s.ListLifecycleRunsByOwner(ctx, ownerID, "autoputer-startup", 20); err != nil {
		t.Fatalf("list fixture runs before startup: %v", err)
	} else {
		for _, run := range runs {
			if run.AgentID == agentID {
				t.Fatalf("fixture unexpectedly has a Texture activation before startup: %+v", runs)
			}
		}
	}

	owner := textureowner.NewHandler(adapter.Runtime)
	if err := adapter.BindTextureOwner(owner); err != nil {
		t.Fatalf("bind Texture owner: %v", err)
	}
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("start adapter: %v", err)
	}
	select {
	case <-blocking.started:
	case <-time.After(5 * time.Second):
		t.Fatal("Texture activation did not start after owner recovery")
	}

	agent, err := s.GetAgentByScope(ctx, ownerID, "autoputer-startup", agentID)
	if err != nil {
		t.Fatalf("load recovered Texture identity: %v", err)
	}
	if agent.OwnerID != ownerID || agent.ChannelID != docID {
		t.Fatalf("recovered Texture identity = %+v", agent)
	}
	runs, err := s.ListLifecycleRunsByOwner(ctx, ownerID, "autoputer-startup", 20)
	if err != nil {
		t.Fatalf("list startup recovery runs: %v", err)
	}
	textureRuns := 0
	var recovered *types.RunRecord
	for i := range runs {
		run := &runs[i]
		if run.AgentID == agentID && run.ChannelID == docID {
			textureRuns++
			recovered = run
		}
	}
	if textureRuns != 1 {
		t.Fatalf("startup recovery created %d Texture runs, want exactly one: %+v", textureRuns, runs)
	}
	if recovered == nil || recovered.Metadata["type"] != "texture_agent_revision" ||
		recovered.Metadata["doc_id"] != docID {
		t.Fatalf("startup recovery bypassed canonical Texture revision authority: %+v", recovered)
	}
}

// waitForRunState polls the store until the run reaches the target state or
// times out.
func waitForRunState(t *testing.T, s *store.Store, ctx context.Context, runID string, target types.RunState, timeout time.Duration) types.RunRecord {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		rec, err := s.GetRun(ctx, runID)
		if err != nil {
			t.Fatalf("GetRun %s: %v", runID, err)
		}
		if rec.State == target {
			return rec
		}
		if rec.State.Terminal() && target != rec.State {
			t.Fatalf("run %s reached terminal state %s, want %s", runID, rec.State, target)
		}
		time.Sleep(20 * time.Millisecond)
	}
	rec, _ := s.GetRun(ctx, runID)
	t.Fatalf("run %s did not reach %s within %s (state=%s)", runID, target, timeout, rec.State)
	return types.RunRecord{}
}

// TestAdapterStartRunExecutesViaActorHandler verifies that a run started via
// the Adapter executes through the actor handler (not startRunAsync) and
// completes. This is the Phase 1 existential test: the actor handler IS the
// execution boundary.
func TestAdapterStartRunExecutesViaActorHandler(t *testing.T) {
	env := newAdapterTestEnv(t)

	rec, err := env.adapter.Runtime.StartRun(env.ctx, "Test prompt for actor handler", "test-owner")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if rec.RunID == "" {
		t.Fatal("StartRun returned empty run ID")
	}

	final := waitForRunState(t, env.store, env.ctx, rec.RunID, types.RunCompleted, 5*time.Second)
	if final.Result == "" {
		t.Fatal("run completed but result is empty")
	}
}

// TestAdapterDispatchActorActive verifies that the Adapter wires the
// dispatch function on the runtime core.
func TestAdapterDispatchActorActive(t *testing.T) {
	env := newAdapterTestEnv(t)

	if !env.adapter.Runtime.DispatchActorActive() {
		t.Fatal("DispatchActorActive() = false, want true (adapter should wire dispatch)")
	}
	if env.adapter.ActorRuntime() == nil {
		t.Fatal("ActorRuntime() = nil, want non-nil")
	}
}

func TestAdapterRuntimeCoreIsNamedAndNotEmbedded(t *testing.T) {
	adapterType := reflect.TypeOf(Adapter{})
	runtimeType := reflect.TypeOf((*agentcore.Runtime)(nil))
	runtimeFields := 0
	for i := range adapterType.NumField() {
		field := adapterType.Field(i)
		if field.Type != runtimeType {
			continue
		}
		runtimeFields++
		if field.Name != "Runtime" {
			t.Errorf("runtime core field name = %q, want %q", field.Name, "Runtime")
		}
		if field.Anonymous {
			t.Error("runtime core field is anonymous, want named field")
		}
	}
	if runtimeFields != 1 {
		t.Errorf("runtime core field count = %d, want 1", runtimeFields)
	}
}

// TestHandlerColdStartCoagentResult tests the cold-start path: a coagent_result
// arrives with nil memory (no parked run). The handler should call
// ReconcileCoagentWake to create a new run.
func TestHandlerColdStartCoagentResult(t *testing.T) {
	env := newAdapterTestEnv(t)

	// Create an agent record so ownerForAgent can look it up.
	agentID := "agent-test-cold-start"
	ownerID := "user-cold-start"
	err := env.store.UpsertAgent(env.ctx, types.AgentRecord{
		AgentID:    agentID,
		OwnerID:    ownerID,
		ComputerID: "autoputer-test",
		Profile:    "test-profile",
	})
	if err != nil {
		t.Fatalf("UpsertAgent: %v", err)
	}

	// Send a coagent_result with nil memory — simulates cold start.
	handler := newActorHandler(env.adapter.Runtime, nil)
	u := actorUpdate("user-cold-start", "coagent_result", agentID, "coagent-result-content")
	memory, err := handler.HandleUpdate(env.ctx, agentID, u, nil)
	if err != nil {
		t.Fatalf("HandleUpdate cold start: %v", err)
	}

	// Cold start returns nil memory (the new run will be started by
	// the initial_dispatch message from ReconcileCoagentWake).
	if memory != nil {
		t.Errorf("cold start memory = %v, want nil (new run started via initial_dispatch)", memory)
	}
}

func TestHandlerParkedLifecycleControlReconcilesBeforeRetryAcknowledgement(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "parked-lifecycle.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	provider := &countingLifecycleProvider{}
	cfg := provideriface.Config{
		ComputerID: "autoputer-test", StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts"),
		ProviderTimeout: time.Second, SupervisionInterval: time.Hour,
	}
	adapter := New(cfg, s, events.NewEventBus(), provider, nil)
	t.Cleanup(func() {
		adapter.Stop()
		adapter.cleanupLog()
		_ = s.Close()
	})
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("start adapter: %v", err)
	}
	// Keep reconciliation dispatch observable but out of the live actor loop;
	// this test invokes the handler synchronously at the actor boundary.
	adapter.Runtime.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })

	const suffix = "parked-handler-retry"
	fixture := seedActorLifecycleControl(t, s, suffix)
	initial, err := adapter.Runtime.ReconcileCoagentWake(ctx, fixture.ownerID, fixture.agentID)
	if err != nil || initial == nil {
		t.Fatalf("initial lifecycle reconcile: run=%+v err=%v", initial, err)
	}
	parked := *initial
	parked.State = types.RunPassivated
	parked.UpdatedAt = time.Now().UTC()
	if err := s.UpdateRun(ctx, parked); err != nil {
		t.Fatalf("passivate lifecycle run: %v", err)
	}
	later := commitActorLaterControl(t, s, fixture, suffix, "control-parked-handler-b")
	memory, err := json.Marshal(resumeState{RunID: initial.RunID, Phase: "parked"})
	if err != nil {
		t.Fatal(err)
	}
	handler := newActorHandler(adapter.Runtime, nil)
	update := actor.Update{
		UpdateID: "actor-parked-handler-b", ToAgentID: scopedActorMailboxID(fixture.ownerID, fixture.computerID, fixture.agentID),
		FromAgentID: later.AgentID, Kind: "coagent_result", Content: later.UpdateID, TrajectoryID: fixture.trajectoryID, CreatedAt: time.Now().UTC(),
	}

	wrongTrajectory := update
	wrongTrajectory.TrajectoryID = "trajectory-cross-scope"
	if _, err := handler.HandleUpdate(ctx, fixture.agentID, wrongTrajectory, memory); err == nil || !strings.Contains(err.Error(), "trajectory mismatch") {
		t.Fatalf("cross-trajectory parked wake error = %v", err)
	}
	unchanged, _ := s.GetLifecycleRun(ctx, fixture.ownerID, fixture.computerID, initial.RunID)
	if unchanged.State != types.RunPassivated || provider.calls.Load() != 0 {
		t.Fatalf("cross-trajectory wake mutated/executed run=%+v provider_calls=%d", unchanged, provider.calls.Load())
	}

	// Fail only the worker-update write in the atomic append batch. The exact
	// parked run reactivation succeeds first, while control B remains pending
	// and the actor update must remain unacknowledged/retryable.
	if _, err := s.DB().ExecContext(ctx, `
		CREATE TRIGGER fail_parked_actor_control_append
		BEFORE INSERT ON og_objects FOR EACH ROW
		BEGIN
			IF NEW.object_kind = 'choir.worker_update' THEN
				SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'injected parked actor append failure';
			END IF;
		END`); err != nil {
		t.Fatalf("install append failure trigger: %v", err)
	}
	if _, err := handler.HandleUpdate(ctx, fixture.agentID, update, memory); err == nil || errors.Is(err, agentcore.ErrDurablyTerminalLifecycleControlActivation) {
		t.Fatalf("first parked handler error = %v", err)
	}
	afterFailure, err := s.GetLifecycleRun(ctx, fixture.ownerID, fixture.computerID, initial.RunID)
	pending, pendingErr := s.GetLifecycleUpdate(ctx, fixture.ownerID, fixture.computerID, fixture.trajectoryID, fixture.agentID, later.AgentID, later.ProducerUpdateID)
	if err != nil || pendingErr != nil || afterFailure.State != types.RunPending || metadataString(afterFailure.Metadata, "request_source") != "lifecycle_texture_control" ||
		pending.DeliveredAt != nil || pending.DeliveredToRunID != "" || provider.calls.Load() != 0 {
		t.Fatalf("transient parked wake run=%+v pending=%+v run_err=%v pending_err=%v provider_calls=%d", afterFailure, pending, err, pendingErr, provider.calls.Load())
	}
	if _, err := s.DB().ExecContext(ctx, `DROP TRIGGER fail_parked_actor_control_append`); err != nil {
		t.Fatalf("drop append failure trigger: %v", err)
	}

	gotMemory, err := handler.HandleUpdate(ctx, fixture.agentID, update, memory)
	if err != nil {
		t.Fatalf("retry parked handler: %v", err)
	}
	if gotMemory != nil {
		t.Fatalf("completed retry memory = %s, want nil", gotMemory)
	}
	bound, boundErr := s.GetLifecycleUpdate(ctx, fixture.ownerID, fixture.computerID, fixture.trajectoryID, fixture.agentID, later.AgentID, later.ProducerUpdateID)
	final, finalErr := s.GetLifecycleRun(ctx, fixture.ownerID, fixture.computerID, initial.RunID)
	if boundErr != nil || finalErr != nil || bound.DeliveredToRunID != initial.RunID || bound.DeliveredAt == nil ||
		metadataString(final.Metadata, "request_source") != "lifecycle_texture_control" || provider.calls.Load() != 1 {
		t.Fatalf("retried parked wake final=%+v bound=%+v final_err=%v bound_err=%v provider_calls=%d", final, bound, finalErr, boundErr, provider.calls.Load())
	}
	runs, err := s.ListLifecycleRunsByTrajectory(ctx, fixture.ownerID, fixture.computerID, fixture.trajectoryID, 0)
	if err != nil {
		t.Fatal(err)
	}
	lifecycleControlRuns := 0
	for _, run := range runs {
		if run.AgentID == fixture.agentID && metadataString(run.Metadata, "request_source") == "lifecycle_texture_control" {
			lifecycleControlRuns++
		}
	}
	if lifecycleControlRuns != 1 {
		t.Fatalf("parked retry created %d lifecycle-control Researcher runs: %+v", lifecycleControlRuns, runs)
	}
}

func TestTextureWakeAcceptsExactResearcherReportWithImplicitTargetWorkBinding(t *testing.T) {
	env := newAdapterTestEnv(t)
	env.adapter.Runtime.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })
	const suffix = "exact-report-empty-target-work"
	const ownerID, computerID = "owner-adapter-exact-report", "autoputer-test"
	researcherRun := seedAdapterLifecycleResearcherControl(t, env.store, env.adapter.Runtime, ownerID, computerID, suffix, false)
	docID := "doc-adapter-admission-" + suffix
	textureAgentID := "texture:" + docID
	workID := "researcher-work-adapter-admission-" + suffix
	packet := types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "evidence_update", Summary: "exact report with implicit target work"}
	content := "exact report with implicit target work"
	digest, _ := store.ComputeLifecycleUpdatePayloadDigest(packet, content)
	queue := types.QueueLifecycleUpdateRequest{OwnerID: ownerID, ComputerID: computerID, CommandID: "queue-exact-report-empty-target-work", TrajectoryID: researcherRun.TrajectoryID, TargetAgentID: textureAgentID, ProducerAgentID: researcherRun.AgentID, ProducerUpdateID: "exact-report-empty-target-work", UpdateID: "exact-report-empty-target-work", ChannelID: docID, Role: agentprofile.Researcher, SourceRunID: researcherRun.RunID, WorkItemID: workID, WorkDisposition: types.WorkItemOpen, Packet: packet, Content: content, PayloadDigest: digest}
	queue.CommandDigest, _ = store.ComputeQueueLifecycleUpdateDigest(queue)
	result, err := env.store.QueueLifecycleUpdate(env.ctx, queue)
	if err != nil || result.Update == nil || result.Update.TargetWorkItemID != "" {
		t.Fatalf("queue exact implicit-binding report result=%+v err=%v", result, err)
	}
	occurrence, err := agentcore.TextureProducerReportOccurrence(*result.Update)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := agentcore.EncodeTextureActorOccurrence(occurrence)
	if err != nil {
		t.Fatal(err)
	}
	handler := newActorHandler(env.adapter.Runtime, textureowner.NewHandler(env.adapter.Runtime))
	_, err = handler.HandleUpdate(env.ctx, textureAgentID, actor.Update{UpdateID: "actor-exact-report-empty-target-work", ToAgentID: scopedActorMailboxID(ownerID, computerID, textureAgentID), FromAgentID: researcherRun.AgentID, Kind: "coagent_result", Content: encoded, TrajectoryID: researcherRun.TrajectoryID, CreatedAt: time.Now().UTC()}, nil)
	if err == nil || !strings.Contains(err.Error(), "without disposing exact trigger") {
		t.Fatalf("exact implicit-binding report outcome=%v, want executed no-write refusal rather than zero acknowledgement", err)
	}
	stored, loadErr := env.store.GetLifecycleUpdate(env.ctx, ownerID, computerID, researcherRun.TrajectoryID, textureAgentID, researcherRun.AgentID, queue.ProducerUpdateID)
	if loadErr != nil || stored.Disposition != types.UpdatePending {
		t.Fatalf("no-write report fate=%+v err=%v", stored, loadErr)
	}
}

func TestTextureColdWakeRejectsUnboundLegacyProducerReport(t *testing.T) {
	env := newAdapterTestEnv(t)
	env.adapter.Runtime.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })

	const ownerID, docID = "user-texture-wake", "doc-texture-wake"
	agentID := "texture:" + docID
	update := seedDurableTextureUpdate(t, env.store, env.ctx, "autoputer-test", ownerID, docID, "update-texture-wake", "Durable Texture wake")
	handler := newActorHandler(env.adapter.Runtime, textureowner.NewHandler(env.adapter.Runtime))
	memory, err := handler.HandleUpdate(env.ctx, agentID, actorUpdate(ownerID, "coagent_result", agentID, update.Content), nil)
	if err != nil || memory != nil {
		t.Fatalf("unbound legacy producer report outcome memory=%v err=%v, want typed zero-provider acknowledgement", memory, err)
	}
	pending, err := env.store.GetLifecycleUpdate(env.ctx, ownerID, "autoputer-test", update.TrajectoryID, agentID, update.ProducerAgentID, update.ProducerUpdateID)
	if err != nil || pending.Disposition != types.UpdatePending {
		t.Fatalf("malformed report canonical fate changed: update=%+v err=%v", pending, err)
	}
	runs, err := env.store.ListLifecycleRunsByOwner(env.ctx, ownerID, "autoputer-test", 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, rec := range runs {
		if rec.AgentID == agentID && rec.RunID != "" {
			t.Fatalf("unbound legacy report projected Texture run: %+v", rec)
		}
	}
}

func TestTextureWakeDoesNotLetUnboundLegacyReportReactivatePassivatedRun(t *testing.T) {
	env := newAdapterTestEnv(t)
	env.adapter.Runtime.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })
	const ownerID, docID = "user-texture-stale-memory", "doc-texture-stale-memory"
	agentID := "texture:" + docID
	update := seedDurableTextureUpdate(t, env.store, env.ctx, "autoputer-test", ownerID, docID, "update-texture-stale-memory", "Durable Texture wake")
	now := time.Now().UTC()
	canonical := types.RunRecord{RunID: "run-texture-canonical-passivated", AgentID: agentID, OwnerID: ownerID, ComputerID: "autoputer-test", ChannelID: docID, TrajectoryID: update.TrajectoryID, State: types.RunPassivated, Prompt: "canonical Texture revision memory", AgentProfile: "texture", AgentRole: "texture", CreatedAt: now, UpdatedAt: now, Metadata: map[string]any{"type": "texture_agent_revision", "doc_id": docID, "current_revision_id": "revision:" + docID, "agent_profile": "texture", "agent_role": "texture", "trajectory_id": update.TrajectoryID, "lifecycle_work_item_id": "work:" + docID}}
	if err := env.store.CreateRun(env.ctx, canonical); err != nil {
		t.Fatal(err)
	}
	if err := env.store.CreateAgentMutation(env.ctx, store.AgentMutation{DocID: docID, RunID: canonical.RunID, OwnerID: ownerID, ComputerID: "autoputer-test", State: "pending", RevisionID: "revision:" + docID, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	handler := newActorHandler(env.adapter.Runtime, textureowner.NewHandler(env.adapter.Runtime))
	memory, err := handler.HandleUpdate(env.ctx, agentID, actorUpdate(ownerID, "coagent_result", agentID, update.Content), []byte(`{"run_id":"unrelated-parked-run","phase":"parked"}`))
	if err != nil || memory != nil {
		t.Fatalf("malformed wake outcome memory=%v err=%v", memory, err)
	}
	after, err := env.store.GetLifecycleRun(env.ctx, ownerID, "autoputer-test", canonical.RunID)
	if err != nil || after.State != types.RunPassivated {
		t.Fatalf("malformed wake reactivated run=%+v err=%v", after, err)
	}
}

func TestTextureColdWakeFailsClosedWithoutOwner(t *testing.T) {
	env := newAdapterTestEnv(t)
	_, err := newActorHandler(env.adapter.Runtime, nil).reconcileCoagentWake(
		env.ctx, actor.Update{ToAgentID: scopedActorMailboxID("owner-texture", "autoputer-test", "texture:doc-texture-wake")},
	)
	if err == nil || err.Error() != "Texture owner is not bound" {
		t.Fatalf("Texture wake error = %v, want explicit unbound-owner failure", err)
	}
	memory, marshalErr := json.Marshal(resumeState{RunID: "run-stale-generic-texture", Phase: "parked"})
	if marshalErr != nil {
		t.Fatalf("encode stale Texture memory: %v", marshalErr)
	}
	_, err = newActorHandler(env.adapter.Runtime, nil).HandleUpdate(
		env.ctx,
		"texture:doc-texture-wake",
		actorUpdate("owner-texture", "coagent_result", "texture:doc-texture-wake", ""),
		memory,
	)
	if err == nil || !strings.Contains(err.Error(), "Texture owner is not bound") {
		t.Fatalf("parked Texture wake error = %v, want explicit unbound-owner failure", err)
	}
	invalid := createLifecycleActorRun(t, env, "unbound-dispatch", types.RunPending)
	initial := actorUpdate(invalid.OwnerID, "initial_dispatch", invalid.AgentID, invalid.RunID)
	if _, err = newActorHandler(env.adapter.Runtime, nil).HandleUpdate(env.ctx, initial.ToAgentID, initial, nil); err == nil ||
		!strings.Contains(err.Error(), "Texture owner is not bound") {
		t.Fatalf("unbound Texture initial dispatch error = %v, want fail-closed owner requirement", err)
	}
	seedDurableTextureUpdate(t, env.store, env.ctx, "autoputer-test", "owner-start-unbound", "doc-start-unbound", "update-start-unbound", "durable update")
	if err := env.adapter.Start(env.ctx); err == nil || !strings.Contains(err.Error(), "Texture owner is not bound for durable subject") {
		t.Fatalf("unbound Texture startup error = %v, want fail-closed owner requirement", err)
	}
}

// TestHandlerCancelPassivatedRun tests that a cancel message for a passivated
// run projects canonical RunCancelled state.
func TestHandlerCancelPassivatedRun(t *testing.T) {
	env := newAdapterTestEnv(t)

	// Create a run and manually set it to RunPassivated.
	rec := types.RunRecord{
		RunID:      "run-cancel-test",
		OwnerID:    "user-cancel",
		AgentID:    "agent-cancel-test",
		ComputerID: "autoputer-test",
		Prompt:     "test cancel",
		State:      types.RunPassivated,
	}
	if err := env.store.CreateRun(env.ctx, rec); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	// Encode memory with the run ID.
	mem, _ := json.Marshal(resumeState{RunID: rec.RunID, Phase: "parked"})

	// Send cancel.
	handler := newActorHandler(env.adapter.Runtime, nil)
	u := actorUpdate(rec.OwnerID, "cancel", "agent-cancel-test", "")
	_, err := handler.HandleUpdate(env.ctx, "agent-cancel-test", u, mem)
	if err != nil {
		t.Fatalf("HandleUpdate cancel: %v", err)
	}

	// Verify the run was cancelled without impersonating execution failure.
	updated, err := env.store.GetRun(env.ctx, rec.RunID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if updated.State != types.RunCancelled {
		t.Errorf("run state = %s, want RunCancelled", updated.State)
	}
	if updated.Error == "" {
		t.Error("run error is empty, want cancel message")
	}
}

// TestHandlerCancelMissingRun tests that cancelling a non-existent run is a
// no-op (no error).
func TestHandlerCancelMissingRun(t *testing.T) {
	env := newAdapterTestEnv(t)

	mem, _ := json.Marshal(resumeState{RunID: "nonexistent-run", Phase: "parked"})
	handler := newActorHandler(env.adapter.Runtime, nil)
	u := actorUpdate("owner-missing", "cancel", "agent-missing", "")
	_, err := handler.HandleUpdate(env.ctx, "agent-missing", u, mem)
	if err != nil {
		t.Errorf("HandleUpdate cancel missing run: error = %v, want nil (no-op)", err)
	}
}

// TestHandlerCoagentResultForCompletedRun tests that a coagent_result for a
// terminal (completed) run triggers ReconcileCoagentWake to create a new run.
func TestHandlerCoagentResultForCompletedRun(t *testing.T) {
	env := newAdapterTestEnv(t)

	agentID := "agent-completed-test"
	ownerID := "user-completed"
	err := env.store.UpsertAgent(env.ctx, types.AgentRecord{
		AgentID:    agentID,
		OwnerID:    ownerID,
		ComputerID: "autoputer-test",
		Profile:    "test-profile",
	})
	if err != nil {
		t.Fatalf("UpsertAgent: %v", err)
	}

	// Create a completed run.
	rec := types.RunRecord{
		RunID:      "run-completed-test",
		OwnerID:    ownerID,
		AgentID:    agentID,
		ComputerID: "autoputer-test",
		Prompt:     "test completed",
		State:      types.RunCompleted,
		Result:     "done",
	}
	now := time.Now().UTC()
	rec.FinishedAt = &now
	if err := env.store.CreateRun(env.ctx, rec); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	// Send coagent_result with memory pointing to the completed run.
	mem, _ := json.Marshal(resumeState{RunID: rec.RunID, Phase: "parked"})
	handler := newActorHandler(env.adapter.Runtime, nil)
	u := actorUpdate(ownerID, "coagent_result", agentID, "new-result")
	_, err = handler.HandleUpdate(env.ctx, agentID, u, mem)
	if err != nil {
		t.Fatalf("HandleUpdate coagent_result for completed run: %v", err)
	}

	// The handler should have called ReconcileCoagentWake, which creates
	// a new run and sends an initial_dispatch. The new run should eventually
	// complete (stub provider returns immediately).
	// We can't easily wait for the new run here, but the absence of an
	// error means ReconcileCoagentWake succeeded.
}

// TestHandlerCoagentResultForBlockedRun tests the bug fix: a coagent_result
// for a blocked run should reactivate it, NOT silently drop the message and
// clear memory. Before the fix, this would orphan the blocked run.
func TestHandlerCoagentResultForBlockedRun(t *testing.T) {
	env := newAdapterTestEnv(t)

	agentID := "agent-blocked-test"
	ownerID := "user-blocked"
	err := env.store.UpsertAgent(env.ctx, types.AgentRecord{
		AgentID:    agentID,
		OwnerID:    ownerID,
		ComputerID: "autoputer-test",
		Profile:    "test-profile",
	})
	if err != nil {
		t.Fatalf("UpsertAgent: %v", err)
	}

	// Create a blocked run.
	rec := types.RunRecord{
		RunID:      "run-blocked-test",
		OwnerID:    ownerID,
		ComputerID: "autoputer-test",
		AgentID:    agentID,
		Prompt:     "test blocked",
		State:      types.RunBlocked,
		Error:      "provider rate limit",
	}
	if err := env.store.CreateRun(env.ctx, rec); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	// Send coagent_result with memory pointing to the blocked run.
	mem, _ := json.Marshal(resumeState{RunID: rec.RunID, Phase: "parked"})
	handler := newActorHandler(env.adapter.Runtime, nil)
	u := actorUpdate(ownerID, "coagent_result", agentID, "unblocking-result")
	resultMem, err := handler.HandleUpdate(env.ctx, agentID, u, mem)
	if err != nil {
		t.Fatalf("HandleUpdate coagent_result for blocked run: %v", err)
	}

	// The handler should have reactivated the run (set to RunPending,
	// called ExecuteActivationSync). The stub provider should complete it.
	// Memory should NOT be nil (the run was reactivated, not dropped).
	_ = resultMem // memory may be nil if the run completed immediately

	// Verify the run was reactivated (no longer RunBlocked).
	updated, err := env.store.GetRun(env.ctx, rec.RunID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if updated.State == types.RunBlocked {
		t.Error("run is still RunBlocked — the bug: coagent_result was silently dropped instead of reactivating")
	}
	// The run should have been reactivated and completed (stub provider).
	if updated.State != types.RunCompleted {
		// Give it a moment to complete.
		updated = waitForRunState(t, env.store, env.ctx, rec.RunID, types.RunCompleted, 3*time.Second)
	}
}

// TestHandlerUnknownUpdateKind tests that an unknown update kind is handled
// gracefully (memory unchanged, no error).
func TestHandlerUnknownUpdateKind(t *testing.T) {
	env := newAdapterTestEnv(t)

	handler := newActorHandler(env.adapter.Runtime, nil)
	u := actorUpdate("owner-unknown", "unknown_kind", "agent-test", "content")
	existingMem := []byte(`{"run_id":"run-x","phase":"parked"}`)
	resultMem, err := handler.HandleUpdate(env.ctx, "agent-test", u, existingMem)
	if err != nil {
		t.Errorf("HandleUpdate unknown kind: error = %v, want nil", err)
	}
	// Memory should be unchanged.
	if string(resultMem) != string(existingMem) {
		t.Errorf("memory changed: got %q, want %q (unchanged for unknown kind)", resultMem, existingMem)
	}
}

func createLifecycleActorRun(t *testing.T, env *adapterTestEnv, suffix string, state types.RunState) types.RunRecord {
	t.Helper()
	ownerID := "owner-lifecycle-" + suffix
	docID := "doc-lifecycle-" + suffix
	queue := seedDurableTextureUpdate(t, env.store, env.ctx, "autoputer-test", ownerID, docID, "update-"+suffix, "durable update")
	now := time.Now().UTC()
	rec := types.RunRecord{
		RunID: "run-lifecycle-" + suffix, AgentID: queue.TargetAgentID, OwnerID: ownerID,
		ComputerID: "autoputer-test", ChannelID: docID, TrajectoryID: queue.TrajectoryID,
		State: state, Prompt: "execute lifecycle activation", AgentProfile: "texture", AgentRole: "texture",
		CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{
			"agent_profile": "texture", "agent_role": "texture", "doc_id": docID,
			"trajectory_id": queue.TrajectoryID, "lifecycle_work_item_id": "work:" + docID,
		},
	}
	if err := env.store.CreateRun(env.ctx, rec); err != nil {
		t.Fatalf("create lifecycle actor run: %v", err)
	}
	return rec
}

func TestProductionActorHandlerCancelsLifecycleRun(t *testing.T) {
	env := newAdapterTestEnv(t)
	handler := newActorHandler(env.adapter.Runtime, nil)

	cancelled := createLifecycleActorRun(t, env, "cancel", types.RunPassivated)
	memory, _ := json.Marshal(resumeState{RunID: cancelled.RunID, Phase: "parked"})
	update := actorUpdate(cancelled.OwnerID, "cancel", cancelled.AgentID, "")
	if _, err := handler.HandleUpdate(env.ctx, update.ToAgentID, update, memory); err != nil {
		t.Fatalf("cancel lifecycle activation: %v", err)
	}
	stored, err := env.store.GetLifecycleRun(env.ctx, cancelled.OwnerID, cancelled.ComputerID, cancelled.RunID)
	if err != nil || stored.State != types.RunCancelled {
		t.Fatalf("cancelled lifecycle run = %+v, %v", stored, err)
	}
}

func TestScopedActorMailboxDoesNotCrossOwner(t *testing.T) {
	env := newAdapterTestEnv(t)
	const agentID = "shared-agent-id"
	for _, ownerID := range []string{"owner-scope-a", "owner-scope-b"} {
		if err := env.store.UpsertAgent(env.ctx, types.AgentRecord{
			AgentID: agentID, OwnerID: ownerID, ComputerID: "autoputer-test",
			Profile: "researcher", Role: "researcher", ChannelID: "channel-" + ownerID,
		}); err != nil {
			t.Fatalf("upsert scoped agent %s: %v", ownerID, err)
		}
	}
	handler := newActorHandler(env.adapter.Runtime, nil)
	update := actorUpdate("owner-scope-a", "coagent_result", agentID, "scoped wake")
	now := time.Now().UTC()
	packet := types.CoagentSourcePacket{
		UpdateID: "scoped-update-a", OwnerID: "owner-scope-a", ComputerID: "autoputer-test",
		AgentID: "producer-a", TargetAgentID: agentID, ChannelID: "channel-owner-scope-a", Role: "researcher",
		Packet:  types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "evidence_update", Summary: "scoped update"},
		Content: "scoped wake", CreatedAt: now,
	}
	message := types.ChannelMessage{
		ChannelID: packet.ChannelID, FromAgentID: packet.AgentID, ToAgentID: agentID,
		Role: packet.Role, Content: packet.Content, Timestamp: now,
	}
	if _, _, err := env.store.DispatchWorkerUpdate(env.ctx, packet, &message); err != nil {
		t.Fatalf("dispatch scoped update: %v", err)
	}
	if _, err := handler.HandleUpdate(env.ctx, update.ToAgentID, update, nil); err != nil {
		t.Fatalf("handle scoped wake: %v", err)
	}
	ownerARuns, err := env.store.ListRunsByOwner(env.ctx, "owner-scope-a", 20)
	if err != nil || len(ownerARuns) == 0 {
		t.Fatalf("owner A wake missing: runs=%+v err=%v", ownerARuns, err)
	}
	ownerBRuns, err := env.store.ListRunsByOwner(env.ctx, "owner-scope-b", 20)
	if err != nil {
		t.Fatalf("list owner B runs: %v", err)
	}
	if len(ownerBRuns) != 0 {
		t.Fatalf("owner A wake crossed into owner B: %+v", ownerBRuns)
	}
}

func TestInitialDispatchCannotLoadAnotherOwnersRun(t *testing.T) {
	env := newAdapterTestEnv(t)
	now := time.Now().UTC()
	rec := types.RunRecord{
		RunID: "run-owner-b", OwnerID: "owner-scope-b", ComputerID: "autoputer-test",
		AgentID: "agent-owner-b", State: types.RunPending, CreatedAt: now, UpdatedAt: now,
	}
	if err := env.store.CreateRun(env.ctx, rec); err != nil {
		t.Fatalf("create owner B run: %v", err)
	}
	handler := newActorHandler(env.adapter.Runtime, nil)
	update := actorUpdate("owner-scope-a", "initial_dispatch", rec.AgentID, rec.RunID)
	if _, err := handler.HandleUpdate(env.ctx, update.ToAgentID, update, nil); err == nil {
		t.Fatal("cross-owner initial dispatch succeeded")
	}
	stored, err := env.store.GetRunByOwner(env.ctx, rec.OwnerID, rec.RunID)
	if err != nil || stored.State != types.RunPending {
		t.Fatalf("cross-owner dispatch changed run: %+v, %v", stored, err)
	}
}

func TestAdapterStartMigratesUniqueLegacyUnscopedMailbox(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "legacy-mailbox.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	cfg := provideriface.Config{ComputerID: "autoputer-legacy", StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts")}
	adapter := New(cfg, s, events.NewEventBus(), provider.NewStubProvider(0), nil)
	t.Cleanup(func() {
		adapter.Stop()
		adapter.cleanupLog()
		_ = s.Close()
	})
	now := time.Now().UTC()
	agent := types.AgentRecord{
		AgentID: "legacy-unscoped-agent", OwnerID: "legacy-owner", ComputerID: cfg.ComputerID,
		Profile: "processor", Role: "processor", ChannelID: "legacy-channel", CreatedAt: now, UpdatedAt: now,
	}
	if err := s.UpsertAgent(ctx, agent); err != nil {
		t.Fatalf("upsert legacy agent: %v", err)
	}
	if appended, err := adapter.log.Append(ctx, actor.Update{
		UpdateID: "legacy-update", ToAgentID: agent.AgentID, Kind: "retained_unknown_kind", CreatedAt: now,
	}); err != nil || !appended {
		t.Fatalf("append legacy mailbox: appended=%v err=%v", appended, err)
	}
	if err := adapter.log.SaveSnapshot(ctx, agent.AgentID, []byte(`{"phase":"parked"}`)); err != nil {
		t.Fatalf("save legacy snapshot: %v", err)
	}
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("start with unique legacy mailbox: %v", err)
	}
	scopedID := scopedActorMailboxID(agent.OwnerID, agent.ComputerID, agent.AgentID)
	if legacy, err := adapter.log.Unprocessed(ctx, agent.AgentID); err != nil || len(legacy) != 0 {
		t.Fatalf("legacy backlog after startup: %+v, %v", legacy, err)
	}
	memory, err := adapter.log.LoadSnapshot(ctx, scopedID)
	if err != nil || string(memory) != `{"phase":"parked"}` {
		t.Fatalf("scoped snapshot after startup: %q, %v", memory, err)
	}
}

func TestAdapterStartMigratesLegacyMailboxFromRunWitness(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "legacy-run-witness.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	cfg := provideriface.Config{ComputerID: "autoputer-legacy", StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts")}
	adapter := New(cfg, s, events.NewEventBus(), provider.NewStubProvider(0), nil)
	t.Cleanup(func() {
		adapter.Stop()
		adapter.cleanupLog()
		_ = s.Close()
	})
	now := time.Now().UTC()
	const (
		agentID = "legacy-agent-without-record"
		ownerID = "legacy-owner"
	)
	if err := s.CreateRun(ctx, types.RunRecord{
		RunID: "legacy-run-witness", AgentID: agentID, OwnerID: ownerID, ComputerID: cfg.ComputerID,
		State: types.RunCompleted, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create legacy run witness: %v", err)
	}
	if err := adapter.log.SaveSnapshot(ctx, agentID, []byte(`{"phase":"parked"}`)); err != nil {
		t.Fatalf("save legacy snapshot: %v", err)
	}
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("start with run-witness legacy mailbox: %v", err)
	}
	scopedID := scopedActorMailboxID(ownerID, cfg.ComputerID, agentID)
	identities, err := adapter.log.MailboxIdentities(ctx)
	if err != nil || len(identities) != 1 || identities[0] != scopedID {
		t.Fatalf("mailbox identities after startup: %q, %v; want [%q]", identities, err, scopedID)
	}
	memory, err := adapter.log.LoadSnapshot(ctx, scopedID)
	if err != nil || string(memory) != `{"phase":"parked"}` {
		t.Fatalf("scoped snapshot after startup: %q, %v", memory, err)
	}
}

func TestAdapterStartMigratesLegacyMailboxWithoutPendingBacklog(t *testing.T) {
	for _, tc := range []struct {
		name string
		seed func(*testing.T, *actor.SQLiteLog, context.Context, string, time.Time)
	}{
		{
			name: "snapshot-only",
			seed: func(t *testing.T, log *actor.SQLiteLog, ctx context.Context, mailboxID string, _ time.Time) {
				t.Helper()
				if err := log.SaveSnapshot(ctx, mailboxID, []byte(`{"phase":"parked"}`)); err != nil {
					t.Fatalf("save legacy snapshot: %v", err)
				}
			},
		},
		{
			name: "processed-only",
			seed: func(t *testing.T, log *actor.SQLiteLog, ctx context.Context, mailboxID string, now time.Time) {
				t.Helper()
				if appended, err := log.Append(ctx, actor.Update{UpdateID: "processed-update", ToAgentID: mailboxID, CreatedAt: now}); err != nil || !appended {
					t.Fatalf("append processed update: appended=%v err=%v", appended, err)
				}
				if err := log.MarkProcessed(ctx, mailboxID, "processed-update"); err != nil {
					t.Fatalf("mark update processed: %v", err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			dir := t.TempDir()
			dbPath := filepath.Join(dir, "legacy-mailbox.db")
			s, err := store.Open(dbPath)
			if err != nil {
				t.Fatalf("open store: %v", err)
			}
			cfg := provideriface.Config{ComputerID: "autoputer-legacy", StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts")}
			adapter := New(cfg, s, events.NewEventBus(), provider.NewStubProvider(0), nil)
			t.Cleanup(func() {
				adapter.Stop()
				adapter.cleanupLog()
				_ = s.Close()
			})
			now := time.Now().UTC()
			agent := types.AgentRecord{
				AgentID: "legacy-unscoped-agent", OwnerID: "legacy-owner", ComputerID: cfg.ComputerID,
				Profile: "processor", Role: "processor", ChannelID: "legacy-channel", CreatedAt: now, UpdatedAt: now,
			}
			if err := s.UpsertAgent(ctx, agent); err != nil {
				t.Fatalf("upsert legacy agent: %v", err)
			}
			tc.seed(t, adapter.log, ctx, agent.AgentID, now)
			if err := adapter.Start(ctx); err != nil {
				t.Fatalf("start with %s legacy mailbox: %v", tc.name, err)
			}
			identities, err := adapter.log.MailboxIdentities(ctx)
			scopedID := scopedActorMailboxID(agent.OwnerID, agent.ComputerID, agent.AgentID)
			if err != nil || len(identities) != 1 || identities[0] != scopedID {
				t.Fatalf("mailbox identities after startup: %q, %v; want [%q]", identities, err, scopedID)
			}
		})
	}
}

func TestAdapterStartRefusesAmbiguousLegacyUnscopedMailbox(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "ambiguous-legacy-mailbox.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	cfg := provideriface.Config{ComputerID: "autoputer-legacy", StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts")}
	adapter := New(cfg, s, events.NewEventBus(), provider.NewStubProvider(0), nil)
	t.Cleanup(func() {
		adapter.Stop()
		adapter.cleanupLog()
		_ = s.Close()
	})
	now := time.Now().UTC()
	for _, ownerID := range []string{"owner-a", "owner-b"} {
		if err := s.UpsertAgent(ctx, types.AgentRecord{
			AgentID: "legacy-unscoped-agent", OwnerID: ownerID, ComputerID: cfg.ComputerID,
			Profile: "processor", Role: "processor", ChannelID: ownerID, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("upsert legacy agent for %s: %v", ownerID, err)
		}
	}
	if appended, err := adapter.log.Append(ctx, actor.Update{
		UpdateID: "legacy-update", ToAgentID: "legacy-unscoped-agent", Kind: "coagent_result", CreatedAt: now,
	}); err != nil || !appended {
		t.Fatalf("append legacy mailbox: appended=%v err=%v", appended, err)
	}
	if err := adapter.Start(ctx); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("startup error = %v, want ambiguous legacy mailbox refusal", err)
	}
	if legacy, err := adapter.log.Unprocessed(ctx, "legacy-unscoped-agent"); err != nil || len(legacy) != 1 {
		t.Fatalf("legacy backlog changed after refusal: %+v, %v", legacy, err)
	}
}

func TestAdapterStartRefusesConflictingAgentAndRunWitnesses(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "conflicting-witness-mailbox.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	cfg := provideriface.Config{ComputerID: "autoputer-legacy", StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts")}
	adapter := New(cfg, s, events.NewEventBus(), provider.NewStubProvider(0), nil)
	t.Cleanup(func() {
		adapter.Stop()
		adapter.cleanupLog()
		_ = s.Close()
	})
	now := time.Now().UTC()
	const agentID = "legacy-conflicting-agent"
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID: agentID, OwnerID: "owner-a", ComputerID: cfg.ComputerID,
		Profile: "processor", Role: "processor", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert agent witness: %v", err)
	}
	if err := s.CreateRun(ctx, types.RunRecord{
		RunID: "conflicting-run", AgentID: agentID, OwnerID: "owner-b", ComputerID: cfg.ComputerID,
		State: types.RunCompleted, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create conflicting run witness: %v", err)
	}
	if appended, err := adapter.log.Append(ctx, actor.Update{
		UpdateID: "retained-update", ToAgentID: agentID, CreatedAt: now,
	}); err != nil || !appended {
		t.Fatalf("append legacy update: appended=%v err=%v", appended, err)
	}
	if err := adapter.log.SaveSnapshot(ctx, agentID, []byte(`{"phase":"parked"}`)); err != nil {
		t.Fatalf("save legacy snapshot: %v", err)
	}
	if err := adapter.Start(ctx); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("startup error = %v, want conflicting witness refusal", err)
	}
	if backlog, err := adapter.log.Unprocessed(ctx, agentID); err != nil || len(backlog) != 1 {
		t.Fatalf("legacy backlog changed after refusal: %+v, %v", backlog, err)
	}
	memory, err := adapter.log.LoadSnapshot(ctx, agentID)
	if err != nil || string(memory) != `{"phase":"parked"}` {
		t.Fatalf("legacy snapshot changed after refusal: %q, %v", memory, err)
	}
	for _, ownerID := range []string{"owner-a", "owner-b"} {
		scopedID := scopedActorMailboxID(ownerID, cfg.ComputerID, agentID)
		if scopedMemory, err := adapter.log.LoadSnapshot(ctx, scopedID); err != nil || scopedMemory != nil {
			t.Fatalf("scoped snapshot created for %s after refusal: %q, %v", ownerID, scopedMemory, err)
		}
	}
}

func TestAdapterLegacyMailboxMigrationConvergesMixedBatchAndRepeats(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "mixed-mailbox.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	cfg := provideriface.Config{ComputerID: "autoputer-legacy", StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts")}
	adapter := New(cfg, s, events.NewEventBus(), provider.NewStubProvider(0), nil)
	t.Cleanup(func() {
		adapter.Stop()
		adapter.cleanupLog()
		_ = s.Close()
	})
	now := time.Now().UTC()
	first := types.AgentRecord{
		AgentID: "legacy-a", OwnerID: "owner-a", ComputerID: cfg.ComputerID,
		Profile: "processor", Role: "processor", ChannelID: "channel-a", CreatedAt: now, UpdatedAt: now,
	}
	second := types.AgentRecord{
		AgentID: "legacy-b", OwnerID: "owner-b", ComputerID: cfg.ComputerID,
		Profile: "processor", Role: "processor", ChannelID: "channel-b", CreatedAt: now, UpdatedAt: now,
	}
	for _, agent := range []types.AgentRecord{first, second} {
		if err := s.UpsertAgent(ctx, agent); err != nil {
			t.Fatalf("upsert legacy agent %s: %v", agent.AgentID, err)
		}
		if appended, err := adapter.log.Append(ctx, actor.Update{
			UpdateID: "update-" + agent.AgentID, ToAgentID: agent.AgentID, CreatedAt: now,
		}); err != nil || !appended {
			t.Fatalf("append legacy mailbox %s: appended=%v err=%v", agent.AgentID, appended, err)
		}
	}
	secondScopedID := scopedActorMailboxID(second.OwnerID, second.ComputerID, second.AgentID)
	if appended, err := adapter.log.Append(ctx, actor.Update{
		UpdateID: "scoped-destination-update", ToAgentID: secondScopedID, CreatedAt: now,
	}); err != nil || !appended {
		t.Fatalf("append mixed destination update: appended=%v err=%v", appended, err)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		if err := adapter.migrateLegacyActorMailboxes(ctx); err != nil {
			t.Fatalf("migration attempt %d: %v", attempt, err)
		}
	}
	firstScopedID := scopedActorMailboxID(first.OwnerID, first.ComputerID, first.AgentID)
	firstLegacy, firstErr := adapter.log.Unprocessed(ctx, first.AgentID)
	firstScoped, firstScopedErr := adapter.log.Unprocessed(ctx, firstScopedID)
	secondLegacy, secondErr := adapter.log.Unprocessed(ctx, second.AgentID)
	secondScoped, secondScopedErr := adapter.log.Unprocessed(ctx, secondScopedID)
	if firstErr != nil || firstScopedErr != nil || secondErr != nil || secondScopedErr != nil ||
		len(firstLegacy) != 0 || len(firstScoped) != 1 || len(secondLegacy) != 0 || len(secondScoped) != 2 {
		t.Fatalf("mailboxes after convergence: first legacy=%+v (%v), first scoped=%+v (%v), second legacy=%+v (%v), second scoped=%+v (%v)",
			firstLegacy, firstErr, firstScoped, firstScopedErr, secondLegacy, secondErr, secondScoped, secondScopedErr)
	}
}

// actorUpdate creates an actor.Update for testing.
func actorUpdate(ownerID, kind, toAgentID, content string) actor.Update {
	return actor.Update{
		UpdateID:  "test-update-id",
		ToAgentID: scopedActorMailboxID(ownerID, "autoputer-test", toAgentID),
		Kind:      kind,
		Content:   content,
		CreatedAt: time.Now().UTC(),
	}
}

func TestTextureCanonicalOccurrenceIdentitiesIncludeExactStoreScope(t *testing.T) {
	report := types.CoagentSourcePacket{
		UpdateID: "update-a", ProducerUpdateID: "producer-update-a", OwnerID: "owner-a", ComputerID: "computer-a",
		AgentID: "researcher-a", TargetAgentID: "texture:doc-a", ChannelID: "doc-a", TrajectoryID: "trajectory-a",
		Direction: types.LifecyclePacketDirectionProducerReport, ProducerWorkItemID: "producer-work-a", TargetWorkItemID: "texture-work-a",
		LifecycleVersion: 7, ReducerSeq: 11, MessageSeq: 13, Disposition: types.UpdatePending,
	}
	base, err := agentcore.TextureProducerReportOccurrence(report)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := agentcore.EncodeTextureActorOccurrence(base)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := agentcore.DecodeTextureActorOccurrence(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != base {
		t.Fatalf("producer occurrence round trip = %+v, want %+v", decoded, base)
	}

	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(encoded, "texture-occurrence:"))
	if err != nil {
		t.Fatal(err)
	}
	fields, at := make([]string, 23), 0
	for i := range fields {
		n, used := binary.Uvarint(raw[at:])
		if used <= 0 {
			t.Fatalf("decode test field %d", i)
		}
		at += used
		fields[i] = string(raw[at : at+int(n)])
		at += int(n)
	}
	malformedOccurrence := func(value string) string {
		changed := append([]string(nil), fields...)
		changed[16] = value
		var body []byte
		for _, field := range changed {
			body = binary.AppendUvarint(body, uint64(len(field)))
			body = append(body, field...)
		}
		return "texture-occurrence:" + base64.RawURLEncoding.EncodeToString(body)
	}
	for name, malformed := range map[string]string{"leading zero": malformedOccurrence("07"), "explicit plus": malformedOccurrence("+7"), "overflow": malformedOccurrence("9223372036854775808")} {
		if _, err := agentcore.DecodeTextureActorOccurrence(malformed); err == nil {
			t.Fatalf("%s noncanonical occurrence decoded: %q", name, malformed)
		}
	}

	variations := []types.CoagentSourcePacket{report, report, report, report, report, report, report, report, report, report, report, report}
	variations[0].OwnerID = "owner-b"
	variations[1].ComputerID = "computer-b"
	variations[2].TrajectoryID = "trajectory-b"
	variations[3].TargetAgentID = "texture:doc-b"
	variations[4].AgentID = "researcher-b"
	variations[5].UpdateID = "update-b"
	variations[6].ProducerUpdateID = "producer-update-b"
	variations[7].ProducerWorkItemID = "producer-work-b"
	variations[8].TargetWorkItemID = "texture-work-b"
	variations[9].LifecycleVersion++
	variations[10].ReducerSeq++
	variations[11].MessageSeq++
	for i, changed := range variations {
		o, err := agentcore.TextureProducerReportOccurrence(changed)
		if err != nil {
			t.Fatalf("variation %d: %v", i, err)
		}
		got, err := agentcore.EncodeTextureActorOccurrence(o)
		if err != nil {
			t.Fatal(err)
		}
		if got == encoded {
			t.Fatalf("producer variation %d collided with canonical occurrence", i)
		}
	}

	instruction := types.LifecycleOwnerInstruction{
		Schema: types.LifecycleOwnerInstructionSchemaV1, InstructionID: "instruction-a", RequestID: "request-a",
		OwnerID: "owner-a", ComputerID: "computer-a", DocumentID: "doc-a", TrajectoryID: "trajectory-a",
		TargetAgentID: "texture:doc-a", TargetWorkItemID: "texture-work-a", HeadRevisionID: "revision-a",
		Kind: types.LifecycleOwnerCorrect, Status: types.LifecycleOwnerInstructionPending, LifecycleVersion: 3, ReducerSeq: 17,
	}
	ownerOccurrence, err := agentcore.TextureOwnerInstructionOccurrence(instruction)
	if err != nil {
		t.Fatal(err)
	}
	ownerEncoded, err := agentcore.EncodeTextureActorOccurrence(ownerOccurrence)
	if err != nil {
		t.Fatal(err)
	}
	if ownerEncoded == encoded {
		t.Fatal("producer and owner occurrence domains collided")
	}
	ownerDecoded, err := agentcore.DecodeTextureActorOccurrence(ownerEncoded)
	if err != nil || ownerDecoded != ownerOccurrence {
		t.Fatalf("owner occurrence round trip=%+v err=%v", ownerDecoded, err)
	}

	recovery := agentcore.TextureRecoveryOccurrence(base, "run-a", "tail-a", "revision-a", "sleeping:13:revision-a")
	recoveryEncoded, err := agentcore.EncodeTextureActorOccurrence(recovery)
	if err != nil {
		t.Fatal(err)
	}
	if recoveryEncoded == encoded {
		t.Fatal("recovery occurrence collided with canonical live occurrence")
	}
	if again, _ := agentcore.EncodeTextureActorOccurrence(recovery); again != recoveryEncoded {
		t.Fatal("recovery occurrence is not deterministic")
	}
}

func TestAdapterSQLitePersistsExactTextureReportAndOwnerInstructionOccurrencesBeforeBoot(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "canonical-texture-occurrences.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	const ownerID, computerID, docID = "owner-occurrence", "computer-occurrence", "doc-occurrence"
	queued := seedDurableTextureUpdate(t, s, ctx, computerID, ownerID, docID, "report-occurrence", "exact report")
	stored, err := s.GetLifecycleUpdate(ctx, ownerID, computerID, queued.TrajectoryID, queued.TargetAgentID, queued.ProducerAgentID, queued.ProducerUpdateID)
	if err != nil {
		t.Fatal(err)
	}

	cfg := provideriface.Config{ComputerID: computerID, StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts"), ProviderTimeout: time.Second, SupervisionInterval: time.Hour}
	adapter := New(cfg, s, events.NewEventBus(), provider.NewStubProvider(0), nil)
	t.Cleanup(func() { adapter.Stop(); adapter.cleanupLog() })
	owner := textureowner.NewHandler(adapter.Runtime)
	if err := adapter.BindTextureOwner(owner); err != nil {
		t.Fatal(err)
	}
	// The legacy producer API supplies its old digest; Adapter must upgrade the
	// durable SQLite row to the complete canonical occurrence.
	adapter.Runtime.WakeUpdatedCoagent(ctx, stored)

	snapshot, err := s.GetLifecycleSnapshot(ctx, ownerID, computerID, queued.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	instructionReq := types.QueueLifecycleOwnerInstructionRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "queue-owner-occurrence", RequestID: "request-occurrence",
		InstructionID: "instruction-occurrence", DocumentID: docID, TrajectoryID: queued.TrajectoryID,
		TargetAgentID: "texture:" + docID, TargetWorkItemID: "work:" + docID,
		ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion, ExpectedHeadRevisionID: snapshot.Document.CurrentRevisionID,
		Kind: types.LifecycleOwnerCorrect, Content: "exact correction",
	}
	instructionReq.CommandDigest, _ = store.ComputeQueueLifecycleOwnerInstructionDigest(instructionReq)
	instructionResult, err := s.QueueLifecycleOwnerInstruction(ctx, instructionReq)
	if err != nil {
		t.Fatal(err)
	}
	instructionOccurrence, err := agentcore.TextureOwnerInstructionOccurrence(*instructionResult.OwnerInstruction)
	if err != nil {
		t.Fatal(err)
	}
	instructionContent, err := agentcore.EncodeTextureActorOccurrence(instructionOccurrence)
	if err != nil {
		t.Fatal(err)
	}
	if err := adapter.Runtime.DispatchActor(ctx, ownerID, computerID, "texture:"+docID, "coagent_result", instructionContent, queued.TrajectoryID, "owner:"+ownerID); err != nil {
		t.Fatal(err)
	}

	backlog, err := adapter.log.Unprocessed(ctx, scopedActorMailboxID(ownerID, computerID, "texture:"+docID))
	if err != nil {
		t.Fatal(err)
	}
	if len(backlog) != 2 {
		t.Fatalf("SQLite canonical Texture backlog=%+v", backlog)
	}
	seen := map[string]bool{}
	for _, update := range backlog {
		o, err := agentcore.DecodeTextureActorOccurrence(update.Content)
		if err != nil {
			t.Fatalf("decode durable occurrence %q: %v", update.Content, err)
		}
		if update.TrajectoryID != queued.TrajectoryID {
			t.Fatalf("durable occurrence trajectory=%q", update.TrajectoryID)
		}
		if o.Kind == agentcore.TextureActorOccurrenceProducerReport && update.FromAgentID != stored.AgentID {
			t.Fatalf("producer source=%q", update.FromAgentID)
		}
		if o.Kind == agentcore.TextureActorOccurrenceOwnerInstruction && update.FromAgentID != "owner:"+ownerID {
			t.Fatalf("owner source=%q", update.FromAgentID)
		}
		seen[o.Kind] = true
	}
	if !seen[agentcore.TextureActorOccurrenceProducerReport] || !seen[agentcore.TextureActorOccurrenceOwnerInstruction] {
		t.Fatalf("canonical kinds=%v", seen)
	}
}

func TestAdapterSQLiteBootRecoveryUsesJoinedOccurrenceNotDuplicateInitialDispatch(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "texture-recovery-occurrence.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	const ownerID, computerID, docID = "owner-recovery", "computer-recovery", "doc-recovery"
	queued := seedDurableTextureUpdate(t, s, ctx, computerID, ownerID, docID, "report-recovery", "recover exact report")
	now := time.Now().UTC()
	run := types.RunRecord{
		RunID: "texture-run-recovery", AgentID: "texture:" + docID, ChannelID: docID, TrajectoryID: queued.TrajectoryID,
		AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture, OwnerID: ownerID, ComputerID: computerID,
		State: types.RunPassivated, CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{"type": "texture_agent_revision", "doc_id": docID, "current_revision_id": "revision:" + docID, "lifecycle_work_item_id": "work:" + docID, "trajectory_id": queued.TrajectoryID},
	}
	project := types.ReplaceLifecycleActivationRequest{OwnerID: ownerID, ComputerID: computerID, CommandID: "project-texture-recovery", TrajectoryID: queued.TrajectoryID, AgentID: run.AgentID, Run: run}
	project.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(project)
	if _, err := s.ReplaceLifecycleActivation(ctx, project); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateAgentMutation(ctx, store.AgentMutation{DocID: docID, RunID: run.RunID, OwnerID: ownerID, ComputerID: computerID, State: "sleeping", RevisionID: "revision:" + docID, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{RunID: run.RunID, OwnerID: ownerID, AgentID: run.AgentID, Kind: types.RunMemoryEntryMessage, Role: types.RunMemoryRoleRuntimeInjection, Message: json.RawMessage(`{"role":"user","content":"prior authenticated turn"}`), CreatedAt: now}); err != nil {
		t.Fatal(err)
	}

	cfg := provideriface.Config{ComputerID: computerID, StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts"), ProviderTimeout: time.Second, SupervisionInterval: time.Hour}
	adapter := New(cfg, s, events.NewEventBus(), provider.NewStubProvider(0), nil)
	t.Cleanup(func() { adapter.Stop(); adapter.cleanupLog() })
	owner := textureowner.NewHandler(adapter.Runtime)
	if err := adapter.BindTextureOwner(owner); err != nil {
		t.Fatal(err)
	}
	stored, err := s.GetLifecycleUpdate(ctx, ownerID, computerID, queued.TrajectoryID, queued.TargetAgentID, queued.ProducerAgentID, queued.ProducerUpdateID)
	if err != nil {
		t.Fatal(err)
	}
	legacy := actor.Update{UpdateID: actorDispatchUpdateID(ownerID, computerID, run.AgentID, "coagent_result", agentcore.LifecycleControlActorOccurrenceContent(stored), queued.TrajectoryID, stored.AgentID), ToAgentID: scopedActorMailboxID(ownerID, computerID, run.AgentID), FromAgentID: stored.AgentID, Kind: "coagent_result", Content: agentcore.LifecycleControlActorOccurrenceContent(stored), TrajectoryID: queued.TrajectoryID, CreatedAt: now}
	if appended, err := adapter.log.Append(ctx, legacy); err != nil || !appended {
		t.Fatalf("append legacy=%v err=%v", appended, err)
	}
	if err := adapter.log.MarkProcessed(ctx, legacy.ToAgentID, legacy.UpdateID); err != nil {
		t.Fatal(err)
	}

	if err := owner.Start(ctx); err != nil {
		t.Fatal(err)
	}
	backlog, err := adapter.log.Unprocessed(ctx, legacy.ToAgentID)
	if err != nil {
		t.Fatal(err)
	}
	var normal, recovery int
	for _, update := range backlog {
		if update.Kind == "initial_dispatch" {
			t.Fatalf("boot recovery duplicated initial dispatch: %+v", backlog)
		}
		o, err := agentcore.DecodeTextureActorOccurrence(update.Content)
		if err != nil {
			t.Fatal(err)
		}
		if o.RecoveryRunID == "" {
			normal++
		} else {
			recovery++
			if o.RecoveryRunID != run.RunID || o.RecoveryHeadID != "revision:"+docID || o.RecoveryTailID == "" || !strings.HasPrefix(o.RecoveryMutation, "sleeping:") {
				t.Fatalf("recovery join=%+v", o)
			}
		}
	}
	if normal != 1 || recovery != 0 {
		t.Fatalf("fresh canonical base must execute before recovery normal=%d recovery=%d backlog=%+v", normal, recovery, backlog)
	}
	projected, err := s.GetLifecycleRun(ctx, ownerID, computerID, run.RunID)
	if err != nil || projected.State != types.RunPending {
		t.Fatalf("recovered exact run=%+v err=%v", projected, err)
	}

	// Crash after the canonical base occurrence is processed while its Store
	// trigger remains pending. A later boot proves that exact processed base
	// before appending one joined recovery identity.
	baseUpdate := backlog[0]
	if err := adapter.log.MarkProcessed(ctx, baseUpdate.ToAgentID, baseUpdate.UpdateID); err != nil {
		t.Fatal(err)
	}
	projected.State = types.RunPassivated
	projected.UpdatedAt = time.Now().UTC()
	if err := s.UpdateRun(ctx, projected); err != nil {
		t.Fatal(err)
	}
	if err := s.SleepAgentMutation(ctx, ownerID, computerID, projected.RunID); err != nil {
		t.Fatal(err)
	}
	if err := owner.Start(ctx); err != nil {
		t.Fatal(err)
	}
	backlog, err = adapter.log.Unprocessed(ctx, legacy.ToAgentID)
	if err != nil {
		t.Fatal(err)
	}
	normal, recovery = 0, 0
	for _, update := range backlog {
		o, decodeErr := agentcore.DecodeTextureActorOccurrence(update.Content)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if o.RecoveryRunID == "" {
			normal++
		} else {
			recovery++
			if o.RecoveryRunID != run.RunID || o.RecoveryHeadID != "revision:"+docID || o.RecoveryTailID == "" || !strings.HasPrefix(o.RecoveryMutation, "sleeping:") {
				t.Fatalf("recovery join=%+v", o)
			}
		}
	}
	if normal != 0 || recovery != 1 {
		t.Fatalf("processed canonical base recovery normal=%d recovery=%d backlog=%+v", normal, recovery, backlog)
	}
}

type admissionRecoveryCountingProvider struct {
	calls atomic.Int32
	stub  *provider.StubProvider
}

func (p *admissionRecoveryCountingProvider) Execute(ctx context.Context, rec *types.RunRecord, emit provideriface.EventEmitFunc) error {
	p.calls.Add(1)
	return p.stub.Execute(ctx, rec, emit)
}
func (p *admissionRecoveryCountingProvider) ProviderName() string {
	return "admission-recovery-counting"
}

func seedAdapterLifecycleResearcherControl(t *testing.T, s *store.Store, rt *agentcore.Runtime, ownerID, computerID, suffix string, failBind bool) types.RunRecord {
	t.Helper()
	ctx := context.Background()
	docID, trajectoryID := "doc-adapter-admission-"+suffix, "trajectory-adapter-admission-"+suffix
	textureAgentID, researcherAgentID := "texture:"+docID, "researcher:"+suffix
	now := time.Now().UTC()
	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "start-adapter-admission-" + suffix,
		TrajectoryID: trajectoryID, Kind: types.TrajectoryKindDocument,
		SubjectRefs:     map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:     types.WorkItemRecord{WorkItemID: "texture-work-" + suffix, Objective: "author exact control", AssignedAgentID: textureAgentID, AuthorityProfile: agentprofile.Texture},
		InitialDocument: types.Document{DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID, Title: "Adapter admission", CreatedAt: now, UpdatedAt: now},
		InitialRevision: types.Revision{RevisionID: "revision-adapter-admission-" + suffix, DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID, AuthorKind: types.AuthorUser, AuthorLabel: ownerID, Content: "initial", CreatedAt: now},
		Agent:           types.AgentRecord{AgentID: textureAgentID, OwnerID: ownerID, ComputerID: computerID, Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: docID, CreatedAt: now, UpdatedAt: now},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start: %v", err)
	}
	caller := types.RunRecord{RunID: "texture-run-adapter-admission-" + suffix, OwnerID: ownerID, ComputerID: computerID, AgentID: textureAgentID, AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture, ChannelID: docID, TrajectoryID: trajectoryID, State: types.RunRunning, Metadata: map[string]any{"lifecycle_work_item_id": start.InitialWork.WorkItemID, "work_item_ids": []string{start.InitialWork.WorkItemID}}, CreatedAt: now, UpdatedAt: now}
	project := types.ReplaceLifecycleActivationRequest{OwnerID: ownerID, ComputerID: computerID, CommandID: "project-adapter-admission-" + suffix, TrajectoryID: trajectoryID, AgentID: textureAgentID, Run: caller}
	project.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(project)
	if _, err := s.ReplaceLifecycleActivation(ctx, project); err != nil {
		t.Fatalf("project texture: %v", err)
	}
	if err := s.UpsertAgent(ctx, types.AgentRecord{AgentID: researcherAgentID, OwnerID: ownerID, ComputerID: computerID, Profile: agentprofile.Researcher, Role: agentprofile.Researcher, ChannelID: docID, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("upsert researcher: %v", err)
	}
	workID := "researcher-work-adapter-admission-" + suffix
	open := types.OpenLifecycleWorkRequest{OwnerID: ownerID, ComputerID: computerID, CommandID: "open-adapter-admission-" + suffix, TrajectoryID: trajectoryID, WorkItem: types.WorkItemRecord{WorkItemID: workID, Objective: "research exact control", AuthorityProfile: agentprofile.Researcher, AssignedAgentID: researcherAgentID, CreatedByRunID: caller.RunID, Details: map[string]any{"requested_by_profile": agentprofile.Texture, "requested_by_agent_id": textureAgentID, "requested_by_run_id": caller.RunID}}}
	open.CommandDigest, _ = store.ComputeOpenLifecycleWorkDigest(open)
	if _, err := s.OpenLifecycleWork(ctx, open); err != nil {
		t.Fatalf("open work: %v", err)
	}
	targetRun := types.RunRecord{RunID: "researcher-bootstrap-adapter-admission-" + suffix, OwnerID: ownerID, ComputerID: computerID, AgentID: researcherAgentID, AgentProfile: agentprofile.Researcher, AgentRole: agentprofile.Researcher, ChannelID: docID, TrajectoryID: trajectoryID, State: types.RunRunning, Metadata: map[string]any{"lifecycle_work_item_id": workID, "work_item_ids": []string{workID}}, CreatedAt: now, UpdatedAt: now}
	projectTarget := types.ReplaceLifecycleActivationRequest{OwnerID: ownerID, ComputerID: computerID, CommandID: "project-researcher-adapter-admission-" + suffix, TrajectoryID: trajectoryID, AgentID: researcherAgentID, Run: targetRun}
	projectTarget.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(projectTarget)
	if _, err := s.ReplaceLifecycleActivation(ctx, projectTarget); err != nil {
		t.Fatalf("project researcher: %v", err)
	}
	finished := now.Add(time.Millisecond)
	targetRun.State, targetRun.FinishedAt, targetRun.UpdatedAt = types.RunCompleted, &finished, finished
	clearTarget := types.ReplaceLifecycleActivationRequest{OwnerID: ownerID, ComputerID: computerID, CommandID: "clear-researcher-adapter-admission-" + suffix, TrajectoryID: trajectoryID, AgentID: researcherAgentID, Run: targetRun}
	clearTarget.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(clearTarget)
	if _, err := s.ReplaceLifecycleActivation(ctx, clearTarget); err != nil {
		t.Fatalf("clear researcher bootstrap: %v", err)
	}
	packet := types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "evidence_update", Summary: "exact adapter control"}
	content := "exact adapter control content"
	digest, _ := store.ComputeLifecycleUpdatePayloadDigest(packet, content)
	snapshot, _ := s.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	textureAgent, _ := s.GetAgentByScope(ctx, ownerID, computerID, textureAgentID)
	turn := types.ApplyTextureTurnRequest{OwnerID: ownerID, ComputerID: computerID, CommandID: "turn-adapter-admission-" + suffix, DocumentID: docID, TrajectoryID: trajectoryID, CallerAgentID: textureAgentID, CallerRunID: caller.RunID, ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion, ExpectedCallerLifecycleVersion: textureAgent.LifecycleVersion, ExpectedHeadRevisionID: snapshot.HeadRevision.RevisionID, CallerWorkItemID: start.InitialWork.WorkItemID, CallerWorkDisposition: types.WorkItemOpen, Outcome: types.TextureTurnWait, Reason: "wait for research", Controls: []types.TextureTurnControl{{ControlID: "control-adapter-admission-" + suffix, TargetAgentID: researcherAgentID, TargetWorkItemID: workID, Packet: packet, Content: content, PayloadDigest: digest}}}
	turn.CommandDigest, _ = store.ComputeApplyTextureTurnDigest(turn)
	if _, err := s.ApplyTextureTurn(ctx, turn); err != nil {
		t.Fatalf("apply turn: %v", err)
	}
	if failBind {
		if _, err := s.DB().ExecContext(ctx, `
			CREATE TRIGGER fail_adapter_prebind_control_delivery
			BEFORE INSERT ON og_objects FOR EACH ROW
			BEGIN
				IF NEW.object_kind = 'choir.worker_update' THEN
					SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'injected pre-bind control delivery failure';
				END IF;
			END`); err != nil {
			t.Fatalf("install pre-bind failure: %v", err)
		}
	}
	rec, err := rt.ReconcileCoagentWake(ctx, ownerID, researcherAgentID)
	if failBind {
		if err == nil {
			t.Fatal("pre-bind injection unexpectedly succeeded")
		}
		if _, dropErr := s.DB().ExecContext(ctx, `DROP TRIGGER fail_adapter_prebind_control_delivery`); dropErr != nil {
			t.Fatalf("drop pre-bind failure: %v", dropErr)
		}
		runs, listErr := s.ListLifecycleRunsByOwner(ctx, ownerID, computerID, 100)
		if listErr != nil {
			t.Fatal(listErr)
		}
		for _, candidate := range runs {
			if candidate.AgentID == researcherAgentID && candidate.RunID != targetRun.RunID && candidate.State == types.RunPending {
				return candidate
			}
		}
		t.Fatalf("pre-bind failure left no exact pending Researcher run: %+v", runs)
	}
	if err != nil {
		t.Fatalf("reconcile control: %v", err)
	}
	if rec == nil {
		t.Fatal("exact control created no Researcher run")
	}
	return *rec
}

func TestAdapterSQLitePreBindResearcherRecoveryBindsAndExecutesWithoutSnapshot(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "researcher-prebind-recovery.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	counting := &admissionRecoveryCountingProvider{stub: provider.NewStubProvider(0)}
	const ownerID, computerID = "owner-adapter-prebind-recovery", "computer-adapter-prebind-recovery"
	cfg := provideriface.Config{ComputerID: computerID, StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts"), ProviderTimeout: time.Second, SupervisionInterval: time.Hour}
	adapter := New(cfg, s, events.NewEventBus(), counting, nil)
	t.Cleanup(func() { adapter.Stop(); adapter.cleanupLog() })
	rec := seedAdapterLifecycleResearcherControl(t, s, adapter.Runtime, ownerID, computerID, "prebind-missing-snapshot", true)
	mailboxID := scopedActorMailboxID(ownerID, computerID, rec.AgentID)
	if memory, err := adapter.log.LoadSnapshot(ctx, mailboxID); err != nil || len(memory) != 0 {
		t.Fatalf("pre-bind setup unexpectedly has snapshot memory=%q err=%v", memory, err)
	}
	if delivered, err := s.ListLifecycleControlsDeliveredToRun(ctx, ownerID, computerID, rec.TrajectoryID, rec.AgentID, rec.RunID, 10); err != nil || len(delivered) != 0 {
		t.Fatalf("pre-bind setup already delivered controls=%+v err=%v", delivered, err)
	}
	adapter.Runtime.Start(ctx)
	if err := adapter.actorRT.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		stored, loadErr := s.GetLifecycleRun(ctx, ownerID, computerID, rec.RunID)
		if loadErr == nil && stored.State == types.RunCompleted && counting.calls.Load() == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	stored, loadErr := s.GetLifecycleRun(ctx, ownerID, computerID, rec.RunID)
	if loadErr != nil || stored.State != types.RunCompleted || counting.calls.Load() != 1 {
		t.Fatalf("pre-bind recovery state=%s calls=%d err=%v metadata=%+v run=%s", stored.State, counting.calls.Load(), loadErr, stored.Metadata, stored.RunID)
	}
	delivered, deliveryErr := s.ListLifecycleControlsDeliveredToRun(ctx, ownerID, computerID, rec.TrajectoryID, rec.AgentID, rec.RunID, 10)
	if deliveryErr != nil || len(delivered) != 1 || delivered[0].DeliveredToRunID != rec.RunID {
		t.Fatalf("pre-bind recovery delivery=%+v err=%v", delivered, deliveryErr)
	}
	if memory, err := adapter.log.LoadSnapshot(ctx, mailboxID); err != nil || len(memory) != 0 {
		t.Fatalf("pre-bind recovery relied on snapshot memory=%q err=%v", memory, err)
	}
}

func TestAdapterSQLiteResearcherAdmissionRecoveryExecutesWithoutSnapshot(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "researcher-admission-recovery.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	counting := &admissionRecoveryCountingProvider{stub: provider.NewStubProvider(0)}
	const ownerID, computerID = "owner-adapter-admission-recovery", "computer-adapter-admission-recovery"
	cfg := provideriface.Config{ComputerID: computerID, StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts"), ProviderTimeout: time.Second, SupervisionInterval: time.Hour}
	adapter := New(cfg, s, events.NewEventBus(), counting, nil)
	t.Cleanup(func() { adapter.Stop(); adapter.cleanupLog() })
	rec := seedAdapterLifecycleResearcherControl(t, s, adapter.Runtime, ownerID, computerID, "missing-snapshot", false)

	initialID := actorDispatchUpdateID(ownerID, computerID, rec.AgentID, "initial_dispatch", rec.RunID, rec.TrajectoryID, "")
	mailboxID := scopedActorMailboxID(ownerID, computerID, rec.AgentID)
	if err := adapter.log.MarkProcessed(ctx, mailboxID, initialID); err != nil {
		t.Fatal(err)
	}
	rec.State = types.RunPassivated
	metadata := make(map[string]any, len(rec.Metadata))
	for key, value := range rec.Metadata {
		metadata[key] = value
	}
	rec.Metadata = metadata
	rec.Metadata["passivated_reason"] = "lifecycle_researcher_provider_admission_retry"
	rec.UpdatedAt = time.Now().UTC()
	if err := s.UpdateRun(ctx, rec); err != nil {
		t.Fatal(err)
	}
	// Intentionally do not save an actor snapshot. Runtime boot must enqueue a
	// distinct recovery occurrence, and the real handler must resolve its run.
	adapter.Runtime.Start(ctx)
	if err := adapter.actorRT.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		stored, loadErr := s.GetLifecycleRun(ctx, ownerID, computerID, rec.RunID)
		if loadErr == nil && stored.State == types.RunCompleted && counting.calls.Load() == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	stored, loadErr := s.GetLifecycleRun(ctx, ownerID, computerID, rec.RunID)
	if loadErr != nil || stored.State != types.RunCompleted || counting.calls.Load() != 1 {
		t.Fatalf("recovery state=%s calls=%d err=%v metadata=%+v run=%s", stored.State, counting.calls.Load(), loadErr, stored.Metadata, stored.RunID)
	}
	if memory, err := adapter.log.LoadSnapshot(ctx, mailboxID); err != nil || len(memory) != 0 {
		t.Fatalf("test unexpectedly relied on actor snapshot memory=%q err=%v", memory, err)
	}
	if got := counting.calls.Load(); got != 1 {
		t.Fatalf("provider calls=%d", got)
	}
}

func TestAdapterSQLiteInjectionAppendRecoveryExecutesWithoutSnapshot(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "researcher-injection-append-recovery.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	counting := &admissionRecoveryCountingProvider{stub: provider.NewStubProvider(0)}
	const ownerID, computerID = "owner-adapter-injection-recovery", "computer-adapter-injection-recovery"
	cfg := provideriface.Config{ComputerID: computerID, StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts"), ProviderTimeout: time.Second, SupervisionInterval: time.Hour}
	adapter := New(cfg, s, events.NewEventBus(), counting, nil)
	t.Cleanup(func() { adapter.Stop(); adapter.cleanupLog() })
	rec := seedAdapterLifecycleResearcherControl(t, s, adapter.Runtime, ownerID, computerID, "injection-missing-snapshot", false)

	initialID := actorDispatchUpdateID(ownerID, computerID, rec.AgentID, "initial_dispatch", rec.RunID, rec.TrajectoryID, "")
	mailboxID := scopedActorMailboxID(ownerID, computerID, rec.AgentID)
	if err := adapter.log.MarkProcessed(ctx, mailboxID, initialID); err != nil {
		t.Fatal(err)
	}
	rec.State = types.RunPassivated
	metadata := make(map[string]any, len(rec.Metadata))
	for key, value := range rec.Metadata {
		metadata[key] = value
	}
	rec.Metadata = metadata
	rec.Metadata["passivated_reason"] = "runtime_injection_append_failed"
	rec.UpdatedAt = time.Now().UTC()
	if err := s.UpdateRun(ctx, rec); err != nil {
		t.Fatal(err)
	}
	// A durable malformed recovery row ahead of the valid boot occurrence must
	// be acknowledged/quarantined rather than poison the FIFO forever.
	malformed := actor.Update{UpdateID: "malformed-recovery-before-valid", ToAgentID: mailboxID, FromAgentID: "texture:" + rec.ChannelID, Kind: "coagent_result", Content: agentcore.LifecycleResearcherAdmissionRecoveryPrefix + "malformed", TrajectoryID: rec.TrajectoryID, CreatedAt: time.Now().UTC().Add(-time.Second)}
	if appended, err := adapter.log.Append(ctx, malformed); err != nil || !appended {
		t.Fatalf("append malformed recovery=%v err=%v", appended, err)
	}
	// Intentionally do not save an actor snapshot. Runtime boot must enqueue a
	// distinct recovery occurrence, and the real handler must resolve its run.
	adapter.Runtime.Start(ctx)
	for range 3 {
		if err := adapter.actorRT.Sweep(ctx); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		stored, loadErr := s.GetLifecycleRun(ctx, ownerID, computerID, rec.RunID)
		if loadErr == nil && stored.State == types.RunCompleted && counting.calls.Load() == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	stored, loadErr := s.GetLifecycleRun(ctx, ownerID, computerID, rec.RunID)
	if loadErr != nil || stored.State != types.RunCompleted || counting.calls.Load() != 1 {
		t.Fatalf("injection recovery state=%s calls=%d err=%v metadata=%+v run=%s", stored.State, counting.calls.Load(), loadErr, stored.Metadata, stored.RunID)
	}
	if memory, err := adapter.log.LoadSnapshot(ctx, mailboxID); err != nil || len(memory) != 0 {
		t.Fatalf("test unexpectedly relied on actor snapshot memory=%q err=%v", memory, err)
	}
	if got := counting.calls.Load(); got != 1 {
		t.Fatalf("provider calls=%d", got)
	}
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		backlog, _ := adapter.log.Unprocessed(ctx, mailboxID)
		if len(backlog) == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if backlog, err := adapter.log.Unprocessed(ctx, mailboxID); err != nil || len(backlog) != 0 {
		t.Fatalf("malformed recovery poisoned FIFO backlog=%+v err=%v", backlog, err)
	}
}

func TestAdapterSQLiteStartAcknowledgesCancelledTextureOwnerOccurrenceWithoutMutation(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cancelled-texture-owner-occurrence.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	const ownerID, computerID, docID = "owner-terminal-texture-boot", "computer-terminal-texture-boot", "doc-terminal-texture-boot"
	const trajectoryID, workID = "trajectory-terminal-texture-boot", "work-terminal-texture-boot"
	textureAgentID := agentprofile.Texture + ":" + docID
	now := time.Now().UTC()
	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "start-terminal-texture-boot", TrajectoryID: trajectoryID,
		Kind:            types.TrajectoryKindDocument,
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		SubjectRefs:     map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		InitialWork:     types.WorkItemRecord{WorkItemID: workID, Objective: "apply owner direction", AssignedAgentID: textureAgentID, AuthorityProfile: agentprofile.Texture},
		InitialDocument: types.Document{DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID, Title: "Terminal Texture boot", CreatedAt: now, UpdatedAt: now},
		InitialRevision: types.Revision{RevisionID: "revision-terminal-texture-boot", DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID, AuthorKind: types.AuthorUser, AuthorLabel: ownerID, Content: "initial", CreatedAt: now},
		Agent:           types.AgentRecord{AgentID: textureAgentID, OwnerID: ownerID, ComputerID: computerID, Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: docID, CreatedAt: now, UpdatedAt: now},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	started, err := s.StartLifecycle(ctx, start)
	if err != nil {
		t.Fatalf("start lifecycle fixture: %v", err)
	}

	producerAgentID := agentprofile.Researcher + ":terminal-texture-boot"
	producerWorkID := "producer-work-terminal-texture-boot"
	producerRunID := "producer-run-terminal-texture-boot"
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID: producerAgentID, OwnerID: ownerID, ComputerID: computerID,
		Profile: agentprofile.Researcher, Role: agentprofile.Researcher, ChannelID: docID, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed producer subject: %v", err)
	}
	openProducer := types.OpenLifecycleWorkRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "open-producer-terminal-texture-boot", TrajectoryID: trajectoryID,
		WorkItem: types.WorkItemRecord{WorkItemID: producerWorkID, Objective: "produce cancellation fixture", AssignedAgentID: producerAgentID, AuthorityProfile: agentprofile.Researcher},
	}
	openProducer.CommandDigest, _ = store.ComputeOpenLifecycleWorkDigest(openProducer)
	if _, err := s.OpenLifecycleWork(ctx, openProducer); err != nil {
		t.Fatalf("open producer work: %v", err)
	}
	producerRun := types.RunRecord{
		RunID: producerRunID, OwnerID: ownerID, ComputerID: computerID, AgentID: producerAgentID,
		AgentProfile: agentprofile.Researcher, AgentRole: agentprofile.Researcher, ChannelID: docID, TrajectoryID: trajectoryID,
		State: types.RunRunning, Metadata: map[string]any{"lifecycle_work_item_id": producerWorkID, "work_item_ids": []string{producerWorkID}}, CreatedAt: now, UpdatedAt: now,
	}
	projectProducer := types.ReplaceLifecycleActivationRequest{OwnerID: ownerID, ComputerID: computerID, CommandID: "project-producer-terminal-texture-boot", TrajectoryID: trajectoryID, AgentID: producerAgentID, Run: producerRun}
	projectProducer.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(projectProducer)
	if _, err := s.ReplaceLifecycleActivation(ctx, projectProducer); err != nil {
		t.Fatalf("project producer run: %v", err)
	}
	packet := types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "evidence_update", Summary: "valid report cancelled before Texture boot"}
	producerContent := "valid producer report retained as cancelled history"
	payloadDigest, _ := store.ComputeLifecycleUpdatePayloadDigest(packet, producerContent)
	producerQueue := types.QueueLifecycleUpdateRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "queue-producer-terminal-texture-boot", TrajectoryID: trajectoryID,
		TargetAgentID: textureAgentID, ProducerAgentID: producerAgentID, ProducerUpdateID: "producer-update-terminal-texture-boot", UpdateID: "producer-update-terminal-texture-boot",
		ChannelID: docID, Role: agentprofile.Researcher, SourceRunID: producerRunID, WorkItemID: producerWorkID, WorkDisposition: types.WorkItemOpen,
		Packet: packet, Content: producerContent, PayloadDigest: payloadDigest,
	}
	producerQueue.CommandDigest, _ = store.ComputeQueueLifecycleUpdateDigest(producerQueue)
	if _, err := s.QueueLifecycleUpdate(ctx, producerQueue); err != nil {
		t.Fatalf("queue valid producer update: %v", err)
	}
	finished := now.Add(time.Millisecond)
	producerRun.State, producerRun.FinishedAt, producerRun.UpdatedAt = types.RunCompleted, &finished, finished
	clearProducer := types.ReplaceLifecycleActivationRequest{OwnerID: ownerID, ComputerID: computerID, CommandID: "clear-producer-terminal-texture-boot", TrajectoryID: trajectoryID, AgentID: producerAgentID, Run: producerRun}
	clearProducer.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(clearProducer)
	if _, err := s.ReplaceLifecycleActivation(ctx, clearProducer); err != nil {
		t.Fatalf("complete producer run: %v", err)
	}
	beforeInstruction, err := s.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	instructionReq := types.QueueLifecycleOwnerInstructionRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "queue-terminal-texture-instruction", RequestID: "request-terminal-texture-instruction",
		InstructionID: "instruction-terminal-texture-boot", DocumentID: docID, TrajectoryID: trajectoryID,
		TargetAgentID: textureAgentID, TargetWorkItemID: workID,
		ExpectedLifecycleVersion: beforeInstruction.Trajectory.LifecycleVersion, ExpectedHeadRevisionID: started.Revision.RevisionID,
		Kind: types.LifecycleOwnerCorrect, Content: "retain this pending correction after cancellation",
	}
	instructionReq.CommandDigest, _ = store.ComputeQueueLifecycleOwnerInstructionDigest(instructionReq)
	queued, err := s.QueueLifecycleOwnerInstruction(ctx, instructionReq)
	if err != nil || queued.OwnerInstruction == nil {
		t.Fatalf("queue owner instruction: result=%+v err=%v", queued, err)
	}
	occurrence, err := agentcore.TextureOwnerInstructionOccurrence(*queued.OwnerInstruction)
	if err != nil {
		t.Fatal(err)
	}
	content, err := agentcore.EncodeTextureActorOccurrence(occurrence)
	if err != nil {
		t.Fatal(err)
	}

	providerCalls := &countingLifecycleProvider{targetAgentID: textureAgentID}
	cfg := provideriface.Config{ComputerID: computerID, StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts"), ProviderTimeout: time.Second, SupervisionInterval: time.Hour}
	adapter := New(cfg, s, events.NewEventBus(), providerCalls, nil)
	t.Cleanup(adapter.Stop)
	if err := adapter.BindTextureOwner(textureowner.NewHandler(adapter.Runtime)); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Runtime.DispatchActor(ctx, ownerID, computerID, textureAgentID, "coagent_result", content, trajectoryID, "owner:"+ownerID); err != nil {
		t.Fatalf("persist pre-crash owner occurrence: %v", err)
	}
	mailboxID := scopedActorMailboxID(ownerID, computerID, textureAgentID)
	updateID := actorDispatchUpdateID(ownerID, computerID, textureAgentID, "coagent_result", content, trajectoryID, "owner:"+ownerID)
	if exists, processed, statusErr := adapter.log.UpdateStatus(ctx, mailboxID, updateID); statusErr != nil || !exists || processed {
		t.Fatalf("pre-cancel actor occurrence exists=%v processed=%v err=%v", exists, processed, statusErr)
	}

	// Retain two equally plausible passivated Texture histories. A terminal boot
	// must classify the trajectory before trying to derive one recovery authority.
	candidateRunIDs := []string{"texture-candidate-a-terminal-boot", "texture-candidate-b-terminal-boot"}
	for index, runID := range candidateRunIDs {
		candidate := types.RunRecord{
			RunID: runID, OwnerID: ownerID, ComputerID: computerID, AgentID: textureAgentID,
			AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture, ChannelID: docID, TrajectoryID: trajectoryID,
			State:     types.RunPassivated,
			Metadata:  map[string]any{"type": "texture_agent_revision", "doc_id": docID, "current_revision_id": started.Revision.RevisionID, "lifecycle_work_item_id": workID, "work_item_ids": []string{workID}},
			CreatedAt: now.Add(time.Duration(index+1) * time.Second), UpdatedAt: now.Add(time.Duration(index+1) * time.Second),
		}
		if err := s.CreateRun(ctx, candidate); err != nil {
			t.Fatalf("seed retained Texture candidate %s: %v", runID, err)
		}
		if err := s.CreateAgentMutation(ctx, store.AgentMutation{
			DocID: docID, RunID: runID, OwnerID: ownerID, ComputerID: computerID,
			State: "sleeping", RevisionID: started.Revision.RevisionID, CreatedAt: candidate.CreatedAt,
		}); err != nil {
			t.Fatalf("seed retained Texture mutation %s: %v", runID, err)
		}
	}

	preCancel, err := s.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	cancel := types.CancelLifecycleRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "cancel-terminal-texture-boot", TrajectoryID: trajectoryID,
		ExpectedLifecycleVersion: preCancel.Trajectory.LifecycleVersion, ExpectedHeadRevisionID: preCancel.HeadRevision.RevisionID,
		Reason: "terminal before actor boot sweep",
	}
	cancel.CommandDigest, _ = store.ComputeCancelLifecycleDigest(cancel)
	if _, err := s.CancelLifecycleTrajectory(ctx, cancel); err != nil {
		t.Fatalf("cancel lifecycle fixture: %v", err)
	}
	terminal, err := s.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if err != nil || terminal.Trajectory.Status != types.TrajectoryCancelled || len(terminal.WorkItems) != 2 {
		t.Fatalf("terminal fixture snapshot=%+v err=%v", terminal, err)
	}
	for _, work := range terminal.WorkItems {
		if work.Status != types.WorkItemCancelled {
			t.Fatalf("terminal fixture retained nonterminal work: %+v", work)
		}
	}
	terminalUpdate, err := s.GetLifecycleUpdate(ctx, ownerID, computerID, trajectoryID, textureAgentID, producerAgentID, producerQueue.ProducerUpdateID)
	if err != nil || terminalUpdate.Disposition != types.UpdateCancelled || terminalUpdate.DispositionRef != "trajectory:"+trajectoryID || terminalUpdate.LifecycleVersion <= 1 {
		t.Fatalf("terminal producer update=%+v err=%v", terminalUpdate, err)
	}
	baselineRuns, err := s.ListLifecycleRunsByChannel(ctx, ownerID, computerID, docID, 0)
	if err != nil {
		t.Fatal(err)
	}
	baselineRevisions, err := s.ListRevisionsByScope(ctx, docID, ownerID, computerID, 100)
	if err != nil {
		t.Fatal(err)
	}
	var baselineMutations int
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM texture_agent_mutations WHERE owner_id = ? AND computer_id = ? AND doc_id = ?`, ownerID, computerID, docID).Scan(&baselineMutations); err != nil {
		t.Fatal(err)
	}
	if len(baselineRuns) != 3 || baselineMutations != len(candidateRunIDs) {
		t.Fatalf("terminal candidate fixture runs=%+v mutations=%d", baselineRuns, baselineMutations)
	}
	for _, runID := range candidateRunIDs {
		candidate, runErr := s.GetLifecycleRun(ctx, ownerID, computerID, runID)
		mutation, mutationErr := s.GetAgentMutationByRun(ctx, ownerID, computerID, runID)
		if runErr != nil || mutationErr != nil || candidate.State != types.RunPassivated || mutation == nil || mutation.State != "sleeping" {
			t.Fatalf("terminal candidate %s run=%+v mutation=%+v run_err=%v mutation_err=%v", runID, candidate, mutation, runErr, mutationErr)
		}
	}

	assertTerminalState := func(stage string, current *Adapter) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			exists, processed, statusErr := current.log.UpdateStatus(ctx, mailboxID, updateID)
			if statusErr == nil && exists && processed {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s actor occurrence exists=%v processed=%v err=%v", stage, exists, processed, statusErr)
			}
			time.Sleep(10 * time.Millisecond)
		}
		var rows int
		if err := current.logDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM actor_updates WHERE update_id = ? AND to_agent_id = ? AND content = ? AND processed_at IS NOT NULL`, updateID, mailboxID, content).Scan(&rows); err != nil || rows != 1 {
			t.Fatalf("%s durable processed actor rows=%d err=%v", stage, rows, err)
		}
		instruction, instructionErr := s.GetLifecycleOwnerInstruction(ctx, ownerID, computerID, trajectoryID, instructionReq.InstructionID)
		pending, pendingErr := s.ListPendingLifecycleOwnerInstructions(ctx, ownerID, computerID, trajectoryID, textureAgentID, 10)
		if instructionErr != nil || pendingErr != nil || instruction.Status != types.LifecycleOwnerInstructionPending || len(pending) != 1 || pending[0].InstructionID != instructionReq.InstructionID {
			t.Fatalf("%s retained instruction=%+v pending=%+v instruction_err=%v pending_err=%v", stage, instruction, pending, instructionErr, pendingErr)
		}
		cancelledUpdate, updateErr := s.GetLifecycleUpdate(ctx, ownerID, computerID, trajectoryID, textureAgentID, producerAgentID, producerQueue.ProducerUpdateID)
		if updateErr != nil || cancelledUpdate.Disposition != terminalUpdate.Disposition || cancelledUpdate.DispositionRef != terminalUpdate.DispositionRef ||
			cancelledUpdate.LifecycleVersion != terminalUpdate.LifecycleVersion || cancelledUpdate.ReducerSeq != terminalUpdate.ReducerSeq {
			t.Fatalf("%s changed exact cancelled producer update: got=%+v want=%+v err=%v", stage, cancelledUpdate, terminalUpdate, updateErr)
		}
		for _, runID := range candidateRunIDs {
			candidate, runErr := s.GetLifecycleRun(ctx, ownerID, computerID, runID)
			mutation, mutationErr := s.GetAgentMutationByRun(ctx, ownerID, computerID, runID)
			if runErr != nil || mutationErr != nil || candidate.State != types.RunPassivated || mutation == nil || mutation.State != "sleeping" || mutation.RevisionID != started.Revision.RevisionID {
				t.Fatalf("%s derived terminal candidate %s: run=%+v mutation=%+v run_err=%v mutation_err=%v", stage, runID, candidate, mutation, runErr, mutationErr)
			}
		}
		snapshot, snapshotErr := s.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
		runs, runsErr := s.ListLifecycleRunsByChannel(ctx, ownerID, computerID, docID, 0)
		revisions, revisionsErr := s.ListRevisionsByScope(ctx, docID, ownerID, computerID, 100)
		var mutations int
		mutationsErr := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM texture_agent_mutations WHERE owner_id = ? AND computer_id = ? AND doc_id = ?`, ownerID, computerID, docID).Scan(&mutations)
		if snapshotErr != nil || runsErr != nil || revisionsErr != nil || mutationsErr != nil ||
			len(runs) != len(baselineRuns) || len(revisions) != len(baselineRevisions) || mutations != baselineMutations ||
			len(snapshot.WorkItems) != len(terminal.WorkItems) || len(snapshot.Updates) != len(terminal.Updates) ||
			snapshot.Trajectory.LifecycleVersion != terminal.Trajectory.LifecycleVersion || snapshot.Watermark != terminal.Watermark {
			t.Fatalf("%s mutated terminal Texture state: runs=%d/%d revisions=%d/%d mutations=%d/%d work=%d/%d controls=%d/%d version=%d/%d watermark=%d/%d errs=[%v %v %v %v]", stage, len(runs), len(baselineRuns), len(revisions), len(baselineRevisions), mutations, baselineMutations, len(snapshot.WorkItems), len(terminal.WorkItems), len(snapshot.Updates), len(terminal.Updates), snapshot.Trajectory.LifecycleVersion, terminal.Trajectory.LifecycleVersion, snapshot.Watermark, terminal.Watermark, snapshotErr, runsErr, revisionsErr, mutationsErr)
		}
		if providerCalls.calls.Load() != 0 {
			t.Fatalf("%s provider calls=%d", stage, providerCalls.calls.Load())
		}
	}

	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("start over terminal Texture occurrence: %v", err)
	}
	assertTerminalState("first start", adapter)
	adapter.Stop()

	restarted := New(cfg, s, events.NewEventBus(), providerCalls, nil)
	t.Cleanup(func() { restarted.Stop(); restarted.cleanupLog() })
	if err := restarted.BindTextureOwner(textureowner.NewHandler(restarted.Runtime)); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Start(ctx); err != nil {
		t.Fatalf("idempotent restart over processed terminal occurrence: %v", err)
	}
	assertTerminalState("second start", restarted)
}
