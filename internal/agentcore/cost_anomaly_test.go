package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/llmcost"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// An anomaly produces an owner notice (MA7.7): a desk whose spend in the last
// window is far above its own history gets one system notice in the
// management inbox, a repeat scan commits the same record, and a steady desk
// gets none. Nothing caps spend.
func TestCostAnomalyScanCommitsOneNoticeToManagement(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer := "owner-cost", rt.TextureComputerID()
	models := llmcost.KnownModels()
	if len(models) == 0 {
		t.Skip("no priced model")
	}
	model := models[0].Model
	now := time.Now().UTC().Truncate(time.Minute)
	emit := func(desk string, at time.Time, in, out int) {
		payload, _ := json.Marshal(map[string]any{"phase": "tool_loop", "model": model, "llm_model": model, "input_tokens": in, "output_tokens": out})
		ev := types.EventRecord{EventID: fmt.Sprintf("ev-%s-%d", desk, at.UnixNano()), RunID: "run-" + desk, AgentID: desk + ":doc", OwnerID: owner,
			Kind: types.EventRunProgress, Phase: "tool_loop", Payload: payload, Timestamp: at}
		if err := s.AppendEvent(context.Background(), &ev); err != nil {
			t.Fatalf("append event: %v", err)
		}
	}
	for h := 1; h <= 40; h++ {
		at := now.Add(-time.Duration(h)*time.Hour - 20*time.Minute)
		emit("research", at, 20000, 2000)
		emit("texture", at, 20000, 2000)
	}
	emit("texture", now.Add(-5*time.Minute), 20000, 2000)
	// research spikes far above its history in the last window
	for i := 0; i < 40; i++ {
		emit("research", now.Add(-time.Duration(i)*time.Minute-time.Minute), 4_000_000, 400_000)
	}
	got, err := rt.scanCostAnomalies(context.Background(), owner, computer, now)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(got) != 1 || got[0].Desk != "research" {
		t.Fatalf("anomalies: %+v", got)
	}
	again, err := rt.scanCostAnomalies(context.Background(), owner, computer, now)
	if err != nil || len(again) != 1 {
		t.Fatalf("repeat scan: %v %+v", err, again)
	}
	recs, err := s.ListCommitmentRecords(context.Background(), owner, computer, agentprofile.Management+":"+owner, 50)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, r := range recs {
		if r.RecordID == got[0].NoticeRecordID() {
			n++
			if r.Directive == nil || !r.Directive.Subtype.IsNotice() || r.EmitterKind != types.CommitmentEmitterSystem {
				t.Fatalf("notice shape: %+v", r)
			}
		}
	}
	if n != 1 {
		t.Fatalf("want one notice in management's inbox, got %d of %d records", n, len(recs))
	}
}
