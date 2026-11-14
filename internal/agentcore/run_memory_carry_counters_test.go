package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func carryDelta(before RunMemoryCarryStats) RunMemoryCarryStats {
	a := runMemoryCarry.snapshot()
	return RunMemoryCarryStats{
		Carries: a.Carries - before.Carries, CarriedMessages: a.CarriedMessages - before.CarriedMessages,
		CarriedMessageBytes:  a.CarriedMessageBytes - before.CarriedMessageBytes,
		CarriedCheckpoints:   a.CarriedCheckpoints - before.CarriedCheckpoints,
		CarriedEmptyMessages: a.CarriedEmptyMessages - before.CarriedEmptyMessages,
		NewMessages:          a.NewMessages - before.NewMessages, NewMessageBytes: a.NewMessageBytes - before.NewMessageBytes,
	}
}

func seedCarrySource(t *testing.T, withCheckpoint, withCellFate bool) (*choirstore.Store, types.RunRecord, int64) {
	t.Helper()
	_, s := testRuntime(t)
	ctx := context.Background()
	now := time.Now().UTC()
	src := holdRun(t, s, "carry-src", holdAgent, now, nil)
	base := types.RunMemoryEntry{RunID: src.RunID, OwnerID: src.OwnerID, AgentID: src.AgentID}
	var batch []types.RunMemoryEntry
	if withCheckpoint {
		e := base
		e.Kind, e.Summary, e.TokensBefore = types.RunMemoryEntryCompaction, "checkpoint summary", 7
		batch = append(batch, e)
	}
	for _, txt := range []string{"one", "two two", "three three three"} {
		e := base
		e.Kind, e.Role, e.Message = types.RunMemoryEntryMessage, "user", holdMessage(t, holdText("user", txt))
		batch = append(batch, e)
	}
	if withCellFate {
		e := base
		e.Kind = types.RunMemoryEntryCellFate
		batch = append(batch, e)
	}
	if _, err := s.AppendRunMemoryEntries(ctx, batch); err != nil {
		t.Fatal(err)
	}
	dst := holdRun(t, s, "carry-dst", holdAgent, now.Add(time.Minute), map[string]any{
		runMetadataCarryActorConversation: true, runMetadataActorRewarmSource: src.RunID,
	})
	var want int64
	for _, txt := range []string{"one", "two two", "three three three"} {
		want += int64(len(holdMessage(t, holdText("user", txt))))
	}
	return s, dst, want
}

func TestCarryCountsCheckpointMessagesAndBytes(t *testing.T) {
	st, dst, wantBytes := seedCarrySource(t, true, false)
	before := runMemoryCarry.snapshot()
	m := newRunMemoryManager(st, &dst, provideriface.Config{}, nil)
	if err := m.carryActorConversation(context.Background(), dst.OwnerID, dst.ComputerID, dst.AgentID, dst.RunID); err != nil {
		t.Fatal(err)
	}
	d := carryDelta(before)
	want := RunMemoryCarryStats{Carries: 1, CarriedMessages: 3, CarriedMessageBytes: wantBytes, CarriedCheckpoints: 1}
	if d != want {
		t.Errorf("delta = %+v, want %+v", d, want)
	}
}

func TestCarryWithoutCheckpointCountsAnEmptyMessage(t *testing.T) {
	st, dst, wantBytes := seedCarrySource(t, false, true)
	before := runMemoryCarry.snapshot()
	m := newRunMemoryManager(st, &dst, provideriface.Config{}, nil)
	if err := m.carryActorConversation(context.Background(), dst.OwnerID, dst.ComputerID, dst.AgentID, dst.RunID); err != nil {
		t.Fatal(err)
	}
	d := carryDelta(before)
	want := RunMemoryCarryStats{Carries: 1, CarriedMessages: 4, CarriedMessageBytes: wantBytes, CarriedEmptyMessages: 1}
	if d != want {
		t.Errorf("delta = %+v, want %+v", d, want)
	}
}

func TestOrdinaryAppendsCountAsNew(t *testing.T) {
	_, s := testRuntime(t)
	ctx := context.Background()
	rec := holdRun(t, s, "carry-new", holdAgent, time.Now().UTC(), nil)
	m := newRunMemoryManager(s, &rec, provideriface.Config{}, nil)
	a, b, c := holdMessage(t, holdText("user", "a")), holdMessage(t, holdText("user", "bb")), holdMessage(t, holdText("assistant", "ccc"))
	before := runMemoryCarry.snapshot()
	if err := m.appendMessages(ctx, []json.RawMessage{a, b}); err != nil { // batch path
		t.Fatal(err)
	}
	if err := m.appendMessages(ctx, []json.RawMessage{c}); err != nil { // single path
		t.Fatal(err)
	}
	d := carryDelta(before)
	want := RunMemoryCarryStats{NewMessages: 3, NewMessageBytes: int64(len(a) + len(b) + len(c))}
	if d != want {
		t.Errorf("delta = %+v, want %+v", d, want)
	}
}

func TestObservabilityServesRunMemoryCarry(t *testing.T) {
	rt := telemetryRuntime(t, "computer-carry-obs")
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability", nil)
	req.Header.Set("X-Authenticated-User", "owner-telemetry")
	rec := httptest.NewRecorder()
	(&APIHandler{rt: rt}).HandleGuestObservability(rec, req)
	var obs struct {
		Carry map[string]json.RawMessage `json:"run_memory_carry"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &obs); err != nil || obs.Carry == nil {
		t.Fatalf("observability = %d %s", rec.Code, rec.Body.String())
	}
	for _, key := range []string{"carries", "carried_messages", "carried_message_bytes", "carried_checkpoints", "carried_empty_messages", "new_messages", "new_message_bytes"} {
		if _, ok := obs.Carry[key]; !ok {
			t.Errorf("run_memory_carry lacks %q", key)
		}
	}
}
