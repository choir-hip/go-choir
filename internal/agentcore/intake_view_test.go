package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Management sees labels and the whole library (L2a A2 / R2-6). Ways it
// can fail, written before the code:
//   - the label is joined in inboxRecordView, which has no store, or
//     written into the record, so the arrival's digest changes;
//   - the view does not name the label version, so a relabel or an
//     override silently rewrites what management saw;
//   - an unlabelled capture shows nothing, so management cannot tell "no
//     label" from "label not joined";
//   - with intake.labels off, labels or the new functions still show (new
//     prompt text before the fixture passed);
//   - choir.Stories stops at 50 (the old library cap) or repeats or skips
//     stories across pages;
//   - another desk can call choir.Stories.

func seedArrival(t *testing.T, s *store.Store, owner, computer, sourceID string, items []types.CaptureArrival) {
	t.Helper()
	req, err := captureArrivalRequest(owner, computer, sourceID, items)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CommitInboxRecords(context.Background(), req); err != nil {
		t.Fatalf("seed arrival: %v", err)
	}
}

func TestRecordViewJoinsLabelsNamingTheVersion(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	owner, computer := "owner-view", rt.cfg.ComputerID
	management := agentprofile.Management + ":" + owner
	pope := openTestStory(t, s, owner, computer, "Pope has lung surgery")
	items := []types.CaptureArrival{{CaptureID: "cap-view-1", Title: "Papst operiert", URL: "https://view.example/1"}, {CaptureID: "cap-view-2", Title: "Kyiv strike", URL: "https://view.example/2"}, {CaptureID: "cap-view-3", Title: "Удар по Киеву", URL: "https://view.example/3"}}
	seedArrival(t, s, owner, computer, "rss:view", items)
	before, _ := s.ListInboxRecords(ctx, owner, computer, management, 0, 0)
	for _, l := range []store.CaptureLabel{
		{OwnerID: owner, ComputerID: computer, CaptureID: "cap-view-1", Event: store.CaptureLabelEventStory, EventDocID: pope, EventHeadline: "Pope has lung surgery", EventP: 0.97, Model: "jev"},
		{OwnerID: owner, ComputerID: computer, CaptureID: "cap-view-3", Event: store.CaptureLabelEventSibling, EventCaptureID: "cap-view-2", EventP: 0.9, Model: "jev"},
	} {
		if _, err := s.AppendCaptureLabel(ctx, l); err != nil {
			t.Fatal(err)
		}
	}

	// Off: no labels in the view.
	got, err := rt.loadCellRecords(ctx, owner, computer, management, "")
	if err != nil || len(got.view) != 1 {
		t.Fatalf("records = %+v, %v", got.view, err)
	}
	if got.view[0].Labels != nil {
		t.Fatalf("labels shown with intake.labels off: %+v", got.view[0].Labels)
	}

	if err := s.PutComputerSetting(ctx, store.ComputerSetting{OwnerID: owner, ComputerID: computer, Key: store.SettingIntakeLabels, Value: true}); err != nil {
		t.Fatal(err)
	}
	got, err = rt.loadCellRecords(ctx, owner, computer, management, "")
	if err != nil || len(got.view) != 1 {
		t.Fatalf("records = %+v, %v", got.view, err)
	}
	labels := got.view[0].Labels
	if l := labels["cap-view-1"]; l.Version != 1 || l.Story != "texture:"+pope || !strings.Contains(l.Says, "same event as texture:"+pope) || !strings.Contains(l.Says, "Pope has lung surgery") || !strings.Contains(l.Says, "0.97") {
		t.Fatalf("label view of cap-view-1 = %+v", l)
	}
	if l := labels["cap-view-2"]; l.Says != "unlabelled" || l.Version != 0 {
		t.Fatalf("label view of an unlabelled capture = %+v", l)
	}
	if l := labels["cap-view-3"]; l.SameEventAs != "cap-view-2" || !strings.Contains(l.Says, "cap-view-2") {
		t.Fatalf("label view of a sibling = %+v", l)
	}
	if printed := got.view[0].String(); !strings.Contains(printed, `"labels"`) || !strings.Contains(printed, `"version":1`) {
		t.Fatalf("a printed record hides its labels: %s", printed)
	}
	// A relabel shows the new version; the record itself never changed.
	if _, err := s.AppendCaptureLabel(ctx, store.CaptureLabel{OwnerID: owner, ComputerID: computer, CaptureID: "cap-view-1", Event: store.CaptureLabelEventNew, EventP: 0.6, Model: "jev"}); err != nil {
		t.Fatal(err)
	}
	got, _ = rt.loadCellRecords(ctx, owner, computer, management, "")
	if l := got.view[0].Labels["cap-view-1"]; l.Version != 2 || !strings.HasPrefix(l.Says, "Jev: new") {
		t.Fatalf("after a relabel the view shows %+v; want version 2", l)
	}
	after, _ := s.ListInboxRecords(ctx, owner, computer, management, 0, 0)
	b1, _ := json.Marshal(before)
	b2, _ := json.Marshal(after)
	if string(b1) != string(b2) {
		t.Fatal("joining labels changed the stored arrival record")
	}
}

func TestStoriesPagesPastFiftyNewestFirst(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	owner, computer := "owner-stories", rt.cfg.ComputerID
	for i := 0; i < 57; i++ {
		openTestStory(t, s, owner, computer, fmt.Sprintf("Story number %02d", i))
		time.Sleep(time.Millisecond)
	}
	exec := toolregistry.ExecutionContext{OwnerID: owner, ComputerID: computer, Role: agentprofile.Management, Profile: agentprofile.Management}
	page := func(limit int, before string) (yaegikernel.StoriesPage, error) {
		payload, _ := json.Marshal(yaegikernel.StoriesPayload{Limit: limit, Before: before})
		raw, err := rt.storiesEgress(ctx, exec, payload)
		if err != nil {
			return yaegikernel.StoriesPage{}, err
		}
		var p yaegikernel.StoriesPage
		return p, json.Unmarshal(raw, &p)
	}
	if _, err := page(0, ""); err == nil || !strings.Contains(err.Error(), "intake.labels") {
		t.Fatalf("choir.Stories with intake.labels off = %v; want refused naming the setting", err)
	}
	if err := s.PutComputerSetting(ctx, store.ComputerSetting{OwnerID: owner, ComputerID: computer, Key: store.SettingIntakeLabels, Value: true}); err != nil {
		t.Fatal(err)
	}
	first, err := page(0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Stories) != 50 || first.Next == "" {
		t.Fatalf("first page = %d stories, next %q; want 50 and a next cursor", len(first.Stories), first.Next)
	}
	if first.Stories[0].Headline != "Story number 56" || first.Stories[0].Inbox != "texture:"+first.Stories[0].DocID || first.Stories[0].Version != 1 || first.Stories[0].UpdatedAt == "" {
		t.Fatalf("newest story = %+v", first.Stories[0])
	}
	second, err := page(50, first.Next)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Stories) != 7 || second.Next != "" {
		t.Fatalf("second page = %d stories, next %q; want the 7 older and no next", len(second.Stories), second.Next)
	}
	seen := map[string]bool{}
	for _, st := range append(first.Stories, second.Stories...) {
		if seen[st.DocID] {
			t.Fatalf("story %s listed twice across pages", st.DocID)
		}
		seen[st.DocID] = true
	}
	if second.Stories[6].Headline != "Story number 00" {
		t.Fatalf("oldest story = %+v", second.Stories[6])
	}
	exec.Role, exec.Profile = agentprofile.Research, agentprofile.Research
	if _, err := page(10, ""); err == nil {
		t.Fatal("research called choir.Stories; it is management's")
	}
}

func TestManagementSurfaceListsIntakeFunctionsOnlyWhenOn(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	owner, computer := "owner-surface", rt.cfg.ComputerID
	rec := &types.RunRecord{OwnerID: owner, ComputerID: computer, AgentProfile: agentprofile.Management, AgentID: agentprofile.Management + ":" + owner}
	off, err := rt.systemPromptForRun(rec)
	if err != nil {
		t.Fatal(err)
	}
	for _, verb := range yaegikernel.IntakeVerbs {
		if strings.Contains(off, "choir."+verb) {
			t.Fatalf("management's prompt names choir.%s with intake.labels off", verb)
		}
	}
	for _, text := range []string{"Intake labels", "r.Labels", "Jev names"} {
		if strings.Contains(off, text) {
			t.Fatalf("management's prompt carries intake text %q with intake.labels off", text)
		}
	}
	if err := s.PutComputerSetting(ctx, store.ComputerSetting{OwnerID: owner, ComputerID: computer, Key: store.SettingIntakeLabels, Value: true}); err != nil {
		t.Fatal(err)
	}
	on, err := rt.systemPromptForRun(rec)
	if err != nil {
		t.Fatal(err)
	}
	for _, verb := range yaegikernel.IntakeVerbs {
		if !strings.Contains(on, "choir."+verb+"(") {
			t.Fatalf("management's prompt omits choir.%s with intake.labels on", verb)
		}
	}
	for _, text := range []string{"r.Labels[id]", "Feed a capture into the story Jev names unless you read otherwise", "One story per event across languages", "state why in the reason"} {
		if !strings.Contains(on, text) {
			t.Fatalf("management's prompt lacks %q with intake.labels on", text)
		}
	}
	// The block renders as one bullet of the Living stories list.
	if !strings.Contains(on, "Management never writes a Texture.\n- Intake labels:") {
		t.Fatalf("the intake bullet is not rendered in the surface list:\n%s", on)
	}
}
