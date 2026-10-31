package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// unknownOutcomeCAS fails the next head CAS once with an error that is not a
// refusal: the append's outcome is unknown, so the sealer drops and does not
// retry.
type unknownOutcomeCAS struct {
	computerevent.HeadCAS
	armed atomic.Bool
}

func (c *unknownOutcomeCAS) CompareAndSwap(ctx context.Context, req computerevent.CASRequest) (computerevent.Receipt, error) {
	if c.armed.CompareAndSwap(true, false) {
		return computerevent.Receipt{}, errors.New("cas: connection reset, outcome unknown")
	}
	return c.HeadCAS.CompareAndSwap(ctx, req)
}

// armingProvider answers once with a final message and arms the CAS failure
// on its way out: the assistant message the loop appends next is queued in the
// window, and the first write after the provider call is its seal.
type armingProvider struct {
	*provider.StubProvider
	cas *unknownOutcomeCAS
}

func (p *armingProvider) CallWithTools(context.Context, provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.cas.armed.Store(true)
	return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "final answer",
		Usage: provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1}, Model: "test-model"}, nil
}

// damageAfterLastAppend drives a real run through executeWithToolLoop whose
// last run-memory append is queued and whose seal then fails with an unknown
// outcome, before the run ends. park makes the run end by passivation (the
// coagent park waiter) instead of completion.
func damageAfterLastAppend(t *testing.T, park bool) (*Runtime, types.RunRecord) {
	t.Helper()
	t.Setenv("CHOIR_OWNER_ID", "owner-telemetry")
	cas := &unknownOutcomeCAS{}
	rt := telemetryRuntimeWithCAS(t, fmt.Sprintf("computer-damage-%v", park), func(h computerevent.HeadCAS) computerevent.HeadCAS {
		cas.HeadCAS = h
		return cas
	})
	rt.bus = events.NewEventBus()
	rt.provider = &armingProvider{StubProvider: provider.NewStubProvider(0), cas: cas}
	if err := rt.store.ConfigureTelemetryBatching(store.TelemetryBatchConfig{Enabled: true, Window: time.Hour}); err != nil {
		t.Fatal(err)
	}
	rt.store.SetTapeGroupCommit(true)
	ctx := context.Background()
	now := time.Now().UTC()
	meta := map[string]any{runMetadataAgentProfile: agentprofile.Research, runMetadataAgentRole: agentprofile.Research,
		runMetadataAgentID: "research:damaged", runMetadataTrajectoryID: "traj-damaged"}
	if park {
		meta["actor_park_on_idle"] = true
	}
	rec := types.RunRecord{RunID: "run-damaged", AgentID: "research:damaged", ChannelID: "chan-damaged", TrajectoryID: "traj-damaged",
		AgentProfile: agentprofile.Research, AgentRole: agentprofile.Research, OwnerID: "owner-telemetry",
		ComputerID: rt.cfg.ComputerID, State: types.RunRunning, Prompt: "work", CreatedAt: now, UpdatedAt: now, Metadata: meta}
	if err := rt.store.CreateRun(ctx, rec); err != nil {
		t.Fatal(err)
	}
	rt.executeWithToolLoop(ctx, &rec, nil, func(types.EventKind, string, json.RawMessage) {})
	return rt, rec
}

func assertDamagedRunFailedAndNotSeeding(t *testing.T, rt *Runtime, rec types.RunRecord) {
	t.Helper()
	ctx := context.Background()
	got, err := rt.store.GetRun(ctx, rec.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != types.RunFailed || !strings.Contains(got.Error, store.ErrRunMemoryLost.Error()) {
		t.Fatalf("damaged run = %s / %q, want failed with %q", got.State, got.Error, store.ErrRunMemoryLost)
	}
	evs, err := rt.store.ListEvents(ctx, rec.RunID, 200)
	if err != nil {
		t.Fatal(err)
	}
	failed := false
	for _, e := range evs {
		if e.Kind == types.EventRunFailed {
			failed = true
		}
		if e.Kind == types.EventRunCompleted || e.Kind == types.EventRunPassivated {
			t.Fatalf("damaged run recorded %s on the tape", e.Kind)
		}
	}
	if !failed {
		t.Fatalf("no run_failed event for the damaged run (%d events)", len(evs))
	}
	sourceRunID, entries, err := rt.store.LatestActorRunMemoryEntries(ctx, rec.OwnerID, rec.ComputerID, rec.AgentID, "run-next")
	if (err != nil && !errors.Is(err, store.ErrNotFound)) || sourceRunID != "" || len(entries) != 0 {
		t.Fatalf("next run seeded from the damaged run: source %q entries %d (%v)", sourceRunID, len(entries), err)
	}
}

// Repair note 2, end to end through the real completion path.
func TestDamageAfterLastAppendFailsCompletion(t *testing.T) {
	rt, rec := damageAfterLastAppend(t, false)
	assertDamagedRunFailedAndNotSeeding(t, rt, rec)
}

// The same order, ending by passivation.
func TestDamageAfterLastAppendFailsPassivation(t *testing.T) {
	rt, rec := damageAfterLastAppend(t, true)
	assertDamagedRunFailedAndNotSeeding(t, rt, rec)
}
