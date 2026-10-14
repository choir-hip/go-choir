package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Structured dispositions (L2a R2-5) and corrections (A3). Ways they can
// fail, written before the code:
//   - choir.Feed or choir.Decline writes a different record from the Note
//     a hand-written feed or decline writes (another inbox, kind or body),
//     so Texture or the source reads something new;
//   - the capture list is lost, so the "every capture has a disposition"
//     score and the override rate cannot be read from the ledger;
//   - a decline goes to the wrong source, or one capture's source is
//     guessed when the record does not name it;
//   - a disposition that agrees with the label is recorded as an override,
//     or one that disagrees is not;
//   - an override edits the label version management saw instead of
//     linking to it;
//   - with intake.labels off, Feed and Decline work (new behaviour before
//     the fixture passed), or with it on, overrides never reach Jev.

func managementCell(t *testing.T, rt *Runtime, owner, computer, runID string) func(string) string {
	t.Helper()
	ctx := context.Background()
	management := agentprofile.Management + ":" + owner
	rec := types.RunRecord{RunID: runID, AgentID: management, OwnerID: owner, ComputerID: computer, ChannelID: management,
		AgentProfile: agentprofile.Management, AgentRole: agentprofile.Management, State: types.RunRunning,
		Metadata: map[string]any{"request_source": inboxRecordRequestSource}}
	if err := rt.store.CreateRun(ctx, rec); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	execCtx := toolregistry.ExecutionContext{RunID: rec.RunID, AgentID: rec.AgentID, OwnerID: owner, ChannelID: management, ComputerID: computer,
		Profile: agentprofile.Management, Role: agentprofile.Management, RunRecord: &rec}
	tool := newDeskGoEvalTool(rt, newDeskSessionWorkers(), agentprofile.Management)
	calls := 0
	return func(src string) string {
		t.Helper()
		calls++
		execCtx.ToolCallID = fmt.Sprintf("call-%s-%d", runID, calls)
		payload, _ := json.Marshal(map[string]any{"source": src, "timeout_ms": 15000})
		out, err := tool.Func(toolregistry.WithExecutionContext(ctx, execCtx), payload)
		if err != nil {
			out += err.Error()
		}
		return out
	}
}

func TestFeedAndDeclineWriteTheNoteRecordsWithCapturesAndRecordOverrides(t *testing.T) {
	deskTestWorkerBin(t)
	ctx := context.Background()
	rt, s := testRuntime(t)
	owner, computer := "owner-dispose", rt.cfg.ComputerID
	pope := openTestStory(t, s, owner, computer, "Pope has lung surgery")
	trial := openTestStory(t, s, owner, computer, "Nord Stream trial opens in Hamburg")
	seedArrival(t, s, owner, computer, "rss:dispose", []types.CaptureArrival{
		{CaptureID: "cap-d1", Title: "Vatican confirms diagnosis", URL: "https://d.example/1"},
		{CaptureID: "cap-d2", Title: "Ukrainian goes on trial", URL: "https://d.example/2"},
		{CaptureID: "cap-d3", Title: "Newsletter signup", URL: "https://d.example/3"},
		{CaptureID: "cap-d4", Title: "Hamburg court adjourns", URL: "https://d.example/4"},
	})
	for _, l := range []store.CaptureLabel{
		{CaptureID: "cap-d1", Title: "Vatican confirms diagnosis", Event: store.CaptureLabelEventStory, EventDocID: pope, EventHeadline: "Pope has lung surgery", EventP: 0.95},
		{CaptureID: "cap-d2", Title: "Ukrainian goes on trial", Event: store.CaptureLabelEventStory, EventDocID: pope, EventHeadline: "Pope has lung surgery", EventP: 0.55},
		{CaptureID: "cap-d3", Title: "Newsletter signup", Event: store.CaptureLabelEventNew, EventP: 0.9},
		{CaptureID: "cap-d4", Title: "Hamburg court adjourns", Event: store.CaptureLabelEventStory, EventDocID: trial, EventHeadline: "Nord Stream trial opens in Hamburg", EventP: 0.8},
	} {
		l.OwnerID, l.ComputerID, l.Model = owner, computer, "jev"
		if _, err := s.AppendCaptureLabel(ctx, l); err != nil {
			t.Fatal(err)
		}
	}

	// Off: Feed and Decline are refused and commit nothing.
	cell := managementCell(t, rt, owner, computer, "run-dispose-off")
	if out := cell(`choir.Feed("` + pope + `", []string{"cap-d1"}, "adds the diagnosis")`); !strings.Contains(out, "intake.labels") {
		t.Fatalf("Feed with intake.labels off: %s", out)
	}
	if got, _ := s.ListInboxRecords(ctx, owner, computer, "texture:"+pope, 0, 0); len(got) != 1 {
		t.Fatalf("a refused Feed committed: %+v", got)
	}

	if err := s.PutComputerSetting(ctx, store.ComputerSetting{OwnerID: owner, ComputerID: computer, Key: store.SettingIntakeLabels, Value: true}); err != nil {
		t.Fatal(err)
	}
	cell = managementCell(t, rt, owner, computer, "run-dispose-on")
	out := cell(`rs := choir.Records()
fmt.Println(rs[0].Labels["cap-d2"].Says)
choir.Feed("texture:` + pope + `", []string{"cap-d1"}, "adds the Vatican's confirmation of the diagnosis")
choir.Feed("` + trial + `", []string{"cap-d2"}, "this is the Nord Stream trial, not the pope's surgery")
choir.Decline([]string{"cap-d3", "cap-d4"}, "no news event: a signup page and a procedural note")`)
	if !strings.Contains(out, "Jev: same event as texture:"+pope) || strings.Count(out, "rlm:note:") < 3 {
		t.Fatalf("dispositions did not commit: %s", out)
	}

	// The feed is the Note a hand-written feed is, plus the captures.
	popeInbox, _ := s.ListInboxRecords(ctx, owner, computer, "texture:"+pope, 0, 0)
	feed := popeInbox[len(popeInbox)-1]
	if feed.Directive == nil || feed.Directive.Subtype != types.CommitmentDirectiveNote || feed.SenderInbox != agentprofile.Management+":"+owner ||
		!strings.Contains(feed.Directive.Body, "Vatican's confirmation") || !strings.Contains(feed.Directive.Body, "cap-d1") {
		t.Fatalf("feed record = %+v", feed)
	}
	if d := feed.Disposition; d == nil || d.Kind != types.DispositionFeed || strings.Join(d.Captures, ",") != "cap-d1" || d.Reason == "" {
		t.Fatalf("feed disposition = %+v", feed.Disposition)
	}
	// The decline is the Note to the source, plus the captures.
	declines, _ := s.ListInboxRecords(ctx, owner, computer, types.CaptureArrivalSenderInbox("rss:dispose"), 0, 0)
	if len(declines) != 1 || declines[0].Directive == nil || !strings.Contains(declines[0].Directive.Body, "cap-d3, cap-d4") ||
		declines[0].Disposition == nil || declines[0].Disposition.Kind != types.DispositionDecline || len(declines[0].Disposition.Captures) != 2 {
		t.Fatalf("decline records = %+v", declines)
	}

	// Overrides: cap-d2 (label said the pope story, fed into the trial)
	// and cap-d4 (label said the trial, declined). cap-d1 agreed; cap-d3
	// was new and declined.
	for id, want := range map[string]int{"cap-d1": 0, "cap-d2": 1, "cap-d3": 0, "cap-d4": 1} {
		got, err := s.CaptureLabelOverrides(ctx, owner, computer, id)
		if err != nil || len(got) != want {
			t.Fatalf("overrides of %s = %+v, %v; want %d", id, got, err, want)
		}
	}
	o, _ := s.CaptureLabelOverrides(ctx, owner, computer, "cap-d2")
	if o[0].LabelVersion != 1 || o[0].Disposition != types.DispositionFeed || o[0].DocID != trial || o[0].Headline != "Nord Stream trial opens in Hamburg" ||
		!strings.Contains(o[0].Reason, "not the pope") || !strings.Contains(o[0].JevSaid, pope) {
		t.Fatalf("override of cap-d2 = %+v", o[0])
	}
	// The label version management saw is unchanged.
	labels, _ := s.CaptureLabels(ctx, owner, computer, "cap-d2")
	if len(labels) != 1 || labels[0].EventDocID != pope {
		t.Fatalf("an override edited the label: %+v", labels)
	}
	if out := cell(`choir.Decline([]string{"cap-unknown"}, "no")`); !strings.Contains(out, "not named by a captures record") {
		t.Fatalf("Decline of a capture no record names: %s", out)
	}
}

func TestLatestTwelveOverridesAreWorkedExamplesInJevsState(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	owner, computer := "owner-examples", rt.cfg.ComputerID
	if err := s.PutComputerSetting(ctx, store.ComputerSetting{OwnerID: owner, ComputerID: computer, Key: store.SettingIntakeLabels, Value: true}); err != nil {
		t.Fatal(err)
	}
	openTestStory(t, s, owner, computer, "Pope has lung surgery")
	base := time.Now().UTC().Add(-time.Hour)
	for i := 0; i < 13; i++ {
		if err := s.PutCaptureLabelOverride(ctx, store.CaptureLabelOverride{OwnerID: owner, ComputerID: computer, CaptureID: fmt.Sprintf("cap-o%02d", i), LabelVersion: 1,
			CaptureTitle: fmt.Sprintf("Capture %02d", i), JevSaid: "Jev: new (p 0.70)", Disposition: types.DispositionFeed, Headline: "Pope has lung surgery",
			RecordID: fmt.Sprintf("rec-%d", i), Reason: fmt.Sprintf("reason %02d", i), CreatedAt: base.Add(time.Duration(i) * time.Minute)}); err != nil {
			t.Fatal(err)
		}
	}
	judge := &fakeIntakeJudge{}
	rt.SetCandidateJudge(judge)
	if labelled, _ := rt.labelCaptures(ctx, owner, computer, "rss:examples", []sourcegraph.CapturedItem{{CaptureID: "cap-new", Title: "Papst operiert", URL: "https://e.example/1"}}); labelled != 1 {
		t.Fatal("capture not labelled")
	}
	corrections, _ := judge.calls[0].state["corrections"].([]any)
	if len(corrections) != 12 {
		t.Fatalf("Jev's state carries %d corrections; want the latest 12", len(corrections))
	}
	first, _ := corrections[0].(map[string]any)
	if first["capture"] != "Capture 12" || first["reason"] != "reason 12" || !strings.Contains(fmt.Sprint(first["editor_did"]), "Pope has lung surgery") || first["label_said"] != "Jev: new (p 0.70)" {
		t.Fatalf("newest correction = %+v", first)
	}
	for _, c := range corrections {
		if m, _ := c.(map[string]any); m["capture"] == "Capture 00" {
			t.Fatal("the oldest of 13 corrections is still shown")
		}
	}
}
