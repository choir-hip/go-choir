package store

// A decision-model (Jev) call writes a model.served line whose payload.role is
// "system1", a provenance role. After the vocabulary cutover the write guard
// read it as a desk role and refused every line
// (docs/problems/paper-judge-model-record-and-story-rarity-2026-11-09.md).
//
// Failure modes this guards: the line refused by the real guard (the defect);
// the boot fence refusing the stored line on the next boot; the exemption
// widened so a model.served-shaped payload hides a bad desk role elsewhere in
// it; the exemption applied to payloads that are not model.served lines, so
// "system1" passes as a desk role; the exemption widened to the event's other
// role leaves (payload.agent_role); the test running with the guard off
// (vocabCutover unset), as the agentcore tests do.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestVocabWriteGuardPassesTheJudgesModelServedLine(t *testing.T) {
	s := openTestStore(t)
	s.vocabCutover.Store(true)
	ctx := objectgraph.WithWriter(context.Background(), objectgraph.WriterResearch)

	var tally types.ModelServedTally
	tally.Note("fireworks", "jev-judge", "", 10, 1, 0)
	served := tally.Record("system1")
	served.Role = types.ModelServedRoleSystem1
	good, _ := json.Marshal(served)
	write := func(id string, payload []byte) error {
		ev := types.EventRecord{EventID: id, Timestamp: time.Now().UTC(), RunID: "run-" + id, OwnerID: "owner-ms",
			AgentID: "research:doc", Kind: types.EventModelServed, Payload: payload}
		return s.AppendEvent(ctx, &ev)
	}
	if err := write("ev-good", good); err != nil {
		t.Fatalf("the judge's model.served line refused by the real write guard: %v", err)
	}
	// The boot fence reads the same leaves from the stored row.
	if err := s.VerifyServingVocabularyV2(context.Background(), nil); err != nil {
		t.Fatalf("boot fence refused the stored line: %v", err)
	}

	// A research child's search.served line has the same shape
	// (tools_desk.go writes its role beside the schema).
	search, _ := json.Marshal(map[string]any{"schema": types.SearchServedSchema, "role": "research_web"})
	if err := write("ev-search", search); err != nil {
		t.Fatalf("a research child's search.served line refused by the real write guard: %v", err)
	}

	// A bad desk role in a served-shaped payload is still refused.
	var other map[string]any
	_ = json.Unmarshal(good, &other)
	other["agent_role"] = "boss"
	bad, _ := json.Marshal(other)
	if err := write("ev-bad-agent-role", bad); err == nil {
		t.Error("payload.agent_role=boss accepted inside a model.served line")
	}

	// system1 as the role of a payload that is not a model.served line is
	// still refused.
	delete(other, "agent_role")
	other["schema"] = "choir.something_else.v1"
	notServed, _ := json.Marshal(other)
	if err := write("ev-not-served", notServed); err == nil {
		t.Error("payload.role=system1 accepted on a payload that is not a model.served line")
	}
}
