package agentcore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskprotocol"
	"github.com/yusefmosiah/go-choir/internal/deskprotocol/contract"
	"github.com/yusefmosiah/go-choir/internal/release"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Reduce side of the desk protocol contract (docs/design/app-agent-desks-2026-11-04.md
// 5.5): each locked intent shape goes through the reducer's record derivation
// and yields a CommitmentRecord that decodes under the locked schema and
// routes to the inbox the locked address table says. The same run is the
// source of the golden corpus when DESKPROTOCOL_WRITE_FIXTURES=1; every
// fixture is built here from constructors, never from production data.

const fixtureTime = "2026-11-04T00:00:00Z"

type fixture struct {
	Type string          `json:"type"`
	Case string          `json:"case"`
	Body json.RawMessage `json:"body"`
}

func contractLock(t *testing.T) *deskprotocol.Surface {
	t.Helper()
	l, err := contract.LoadCurrentLock("../..")
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestDeskProtocolReduceSideAndGoldenCorpus(t *testing.T) {
	lk := contractLock(t)
	scope := ReductionScope{
		FromAgentID: "texture:doc-1", DeskAgentID: "texture:doc-1", FromRole: "texture", ChannelID: "doc-1",
		RunID: "run-contract", OwnerID: "owner-contract", ComputerID: "computer-contract", CellID: "cell-contract", ModelID: "test/model",
	}
	pre := `{"question":"Will the build pass?","distribution":{"yes":0.8,"no":0.2},"resolver":"management"}`
	pre2 := `{"question":"Does it settle?","distribution":{"yes":0.6,"no":0.4},"resolver":"rule","rule":{"kind":"tests_pass"}}`
	pSucc, wt := 0.8, 0.3
	reqF := &types.RequestFields{PSuccess: &pSucc, Weight: &wt, Settlement: types.SettlementRecipient}
	type act struct {
		name  string
		from  string // desk agent id
		to    string // desk named by the act
		build func(tr *yaegikernel.Tray) (string, error)
	}
	acts := []act{
		{"cast", "texture:doc-1", "research", func(tr *yaegikernel.Tray) (string, error) { return tr.Cast("research", "find sources", "", reqF) }},
		{"ask", "texture:doc-1", "management", func(tr *yaegikernel.Tray) (string, error) { return tr.Ask("management", "which sources?", reqF) }},
		{"feed", "management", "texture:doc-1", func(tr *yaegikernel.Tray) (string, error) {
			return tr.Dispose("texture:doc-1", yaegikernel.DispositionFeed, []string{"capture-1"}, "supports the story", reqF)
		}},
		{"decline", "management", "source:src-1", func(tr *yaegikernel.Tray) (string, error) {
			return tr.Dispose("source:src-1", yaegikernel.DispositionDecline, []string{"capture-2"}, "off topic", reqF)
		}},
		{"reply", "research:doc-1", "texture", func(tr *yaegikernel.Tray) (string, error) { return tr.Reply("texture", "rec-1", "the answer") }},
		{"report", "research:doc-1", "texture", func(tr *yaegikernel.Tray) (string, error) {
			return tr.Report("texture", "a claim", []string{"evidence://1"}, "texture")
		}},
		{"escalate", "management", "texture", func(tr *yaegikernel.Tray) (string, error) { return tr.Escalate("texture", "stuck", reqF) }},
		{"cancel", "management", "texture", func(tr *yaegikernel.Tray) (string, error) { return tr.Cancel("rec-1") }},
		{"precommit", "engineering:a-1", "management", func(tr *yaegikernel.Tray) (string, error) { return tr.Precommit(pre) }},
		{"precommit_rule", "engineering:a-1", "management", func(tr *yaegikernel.Tray) (string, error) { return tr.Precommit(pre2) }},
		{"resolve", "management", "texture", func(tr *yaegikernel.Tray) (string, error) {
			return tr.Resolve("rec-1", `{"verdict":"confirmed","evidence_refs":["e1"]}`)
		}},
		{"disagreement", "management", "texture", func(tr *yaegikernel.Tray) (string, error) {
			return tr.Disagreement(`{"commitment_id":"rec-1","scorer_verdict":"contradicted","resolver_verdict":"confirmed"}`)
		}},
	}

	corpus := map[string]fixture{}
	add := func(typ, name string, v any) {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		corpus[typ+"."+name] = fixture{Type: typ, Case: name, Body: raw}
	}
	routed := 0
	for _, a := range acts {
		tr := &yaegikernel.Tray{}
		if _, err := a.build(tr); err != nil {
			t.Fatalf("%s: stage: %v", a.name, err)
		}
		in := tr.Drain()[0]
		raw, _ := json.Marshal(in)
		if probs := contract.CheckShape(lk, "yaegikernel.StagedIntent", raw); len(probs) > 0 {
			t.Fatalf("%s intent outside the lock: %v", a.name, probs)
		}
		add("yaegikernel.StagedIntent", a.name, in)

		sc := scope
		sc.FromAgentID, sc.DeskAgentID = a.from, a.from
		rec := commitmentRecordForIntent(sc, in)
		if recordsIntent(in) {
			if route, err := ResolveAddress(a.from, sc.OwnerID, a.to); err == nil && route.OnRoute {
				rec, err = inboxRecordForIntent(sc, in, route.Inbox, false)
				if err != nil {
					t.Fatalf("%s: %v", a.name, err)
				}
				routed++
			}
		}
		rec.Provenance.CommittedAt = fixtureTime
		rec.Provenance.PromptVersion = "fixture" // the prompt hash moves with every prompt edit
		rec.Prediction.CommittedAt = ""
		if rec.Prediction.Hypothesis != "" || len(rec.Prediction.Questions) > 0 {
			rec.Prediction.CommittedAt = fixtureTime
		}
		if rec.Observation.ObservedAt != "" {
			rec.Observation.ObservedAt = fixtureTime
		}
		if rec.Provenance.ResolvedAt != "" {
			rec.Provenance.ResolvedAt = fixtureTime
		}
		rec.Scores = []types.CommitmentScore{}
		rb, _ := json.Marshal(rec)
		if probs := contract.CheckShape(lk, "types.CommitmentRecord", rb); len(probs) > 0 {
			t.Errorf("%s: record outside the locked schema:\n%s\n%s", a.name, strings.Join(probs, "\n"), rb)
		}
		add("types.CommitmentRecord", a.name, rec)
	}
	if routed < 4 {
		t.Fatalf("only %d acts routed through the address resolver", routed)
	}

	// Sub-call records (MA1.5): a settlement of a Cast by the judge, and a
	// self-score; built from constructors like the rest.
	add("types.CommitmentRecord", "subcall_settlement", types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "subcall-settle-1", Kind: types.CommitmentKindDirective,
		Discrepancy: types.DiscrepancyUnresolved, EmitterKind: types.CommitmentEmitterSystem, ComputerID: "computer-contract",
		Scores: []types.CommitmentScore{},
		SubCall: &types.CommitmentSubCall{
			Kind: types.SubCallSettlement, FanOutID: "fan-1", Role: "research_child", ParentRunID: "run-contract", ParentAgent: "texture:doc-1",
			CastID: "cell-contract:cast:tray-1", Scorer: types.SubCallJudgeScorerID, Method: "judge",
			Verdicts: []types.SubCallVerdict{{Q: 0, Answer: "yes", Probabilities: map[string]float64{"yes": 0.9, "no": 0.1}}},
			Answered: map[string]float64{"answered": 0.8, "partly": 0.15, "not_answered": 0.05},
		},
	})
	add("types.CommitmentRecord", "subcall_self_score", types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "subcall-self-1", Kind: types.CommitmentKindDirective,
		Discrepancy: types.DiscrepancyUnresolved, EmitterKind: types.CommitmentEmitterSystem, ComputerID: "computer-contract",
		Scores: []types.CommitmentScore{},
		SubCall: &types.CommitmentSubCall{
			Kind: types.SubCallSelfScore, FanOutID: "fan-1", CastID: "cell-contract:cast:tray-1",
			Self: []types.SubCallSelfAnswer{{Q: 0, Answer: "yes", P: 0.7}},
		},
	})

	// A platform notice: written by the system, never by a cell.
	add("types.CommitmentRecord", "notice", types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "notice-1", Kind: types.CommitmentKindDirective,
		Discrepancy: types.DiscrepancyUnresolved, EmitterKind: types.CommitmentEmitterSystem, ComputerID: "computer-contract",
		Scores:    []types.CommitmentScore{},
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNotice, Objective: "rebase notice"},
		Addressee: "management:owner-contract",
	})

	// Records of the other locked shapes, built from constructors.
	ts := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	add("types.WorkItemRecord", "open", types.WorkItemRecord{
		WorkItemID: "work-1", TrajectoryID: "traj-1", OwnerID: "owner-contract", ComputerID: "computer-contract",
		Objective: "integrate the reports", Status: types.WorkItemOpen, LifecycleVersion: 1, AssignedAgentID: "texture:doc-1", CreatedAt: ts, UpdatedAt: ts,
	})
	add("types.WorkItemRecord", "refused", types.WorkItemRecord{
		WorkItemID: "work-2", TrajectoryID: "traj-1", OwnerID: "owner-contract", ComputerID: "computer-contract",
		Objective: "integrate the reports", Status: types.WorkItemRefused, Reason: "verification failed", ResultRef: "evidence://x", LifecycleVersion: 2, CreatedAt: ts, UpdatedAt: ts,
	})
	add("types.TrajectoryRecord", "live", types.TrajectoryRecord{
		TrajectoryID: "traj-1", OwnerID: "owner-contract", ComputerID: "computer-contract", Kind: types.TrajectoryKindDocument,
		SubjectRefs: map[string]string{"doc_id": "doc-1"}, Status: types.TrajectoryLive,
		SettlementRule: types.SettlementRule{RequireNoOpenWorkItems: true, Version: "durable-work/v1"}, CreatedAt: ts, UpdatedAt: ts,
	})
	add("release.Precommitment", "settled", release.Precommitment{RecordID: "rec-1", Kind: "tests_pass", Question: "Do the tests pass?", PYesPermille: 800, Settled: "yes", ResolveRecordID: "rec-2"})
	add("yaegikernel.BrokerRequest", "read_file", yaegikernel.BrokerRequest{
		ProtocolVersion: yaegikernel.ProtocolVersion, RequestID: "req-1", HandleRef: "handle-1", Epoch: 1,
		Action: yaegikernel.ActionReadFile, Payload: json.RawMessage(`{"path":"notes.md"}`), TimeoutMs: 1000,
	})
	add("yaegikernel.BrokerResponse", "ok", yaegikernel.BrokerResponse{
		ProtocolVersion: yaegikernel.ProtocolVersion, RequestID: "req-1", Success: true, Result: json.RawMessage(`{"content":"x"}`), DurationMs: 3,
	})
	add("yaegikernel.InboxRecord", "cast", yaegikernel.InboxRecord{
		Seq: 1, RecordID: "rec-1", Kind: "directive", From: "management:owner-contract", Objective: "write", CreatedAt: ts,
	})
	add("yaegikernel.PendingUpdate", "packet", yaegikernel.PendingUpdate{UpdateID: "update-1", FromAgentID: "research:doc-1", Packet: types.SourcePacket{}})

	registry := map[string]func() any{
		"yaegikernel.StagedIntent":   func() any { return &yaegikernel.StagedIntent{} },
		"types.CommitmentRecord":     func() any { return &types.CommitmentRecord{} },
		"types.WorkItemRecord":       func() any { return &types.WorkItemRecord{} },
		"types.TrajectoryRecord":     func() any { return &types.TrajectoryRecord{} },
		"release.Precommitment":      func() any { return &release.Precommitment{} },
		"yaegikernel.BrokerRequest":  func() any { return &yaegikernel.BrokerRequest{} },
		"yaegikernel.BrokerResponse": func() any { return &yaegikernel.BrokerResponse{} },
		"yaegikernel.InboxRecord":    func() any { return &yaegikernel.InboxRecord{} },
		"yaegikernel.PendingUpdate":  func() any { return &yaegikernel.PendingUpdate{} },
	}
	for _, f := range corpus {
		if _, ok := registry[f.Type]; !ok {
			t.Fatalf("no decoder registered for fixture type %s", f.Type)
		}
		if probs := contract.CheckShape(lk, f.Type, f.Body); len(probs) > 0 {
			t.Errorf("fixture %s.%s outside the lock:\n%s", f.Type, f.Case, strings.Join(probs, "\n"))
		}
	}

	dir := filepath.Join("..", "deskprotocol", "testdata", "v"+deskprotocol.Version)
	if os.Getenv("DESKPROTOCOL_WRITE_FIXTURES") == "1" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		names := make([]string, 0, len(corpus))
		for k := range corpus {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			b, _ := json.MarshalIndent(corpus[k], "", "  ")
			if err := os.WriteFile(filepath.Join(dir, k+".json"), append(b, '\n'), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("wrote %d fixtures to %s", len(names), dir)
	}
}
