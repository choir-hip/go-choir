package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/disclosure"
	"github.com/yusefmosiah/go-choir/internal/lease"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// MA3.6 in the runtime. Ways the disclosure path could fail (written first):
//
//  1. The paper stops publishing: a run labelled only world_untrusted is asked
//     at publication.
//  2. A package desk carrying owner-private data publishes or messages
//     anywhere with no ask (fetch and search are allowed, owner 2026-11-05).
//  3. A built-in desk is blocked (the observe mode is the rollout), or spends a
//     System 1 call per publish.
//  4. System 1 decides alone: a middling distribution passes.
//  5. The distribution is not recorded, or is recorded without the run's
//     labels.
//  6. The owner tap is not a tape fact, or an approval does not let the retry
//     pass, or lets it pass twice, or a denial is forgotten.
//  7. A refused sink blocks waiting for the owner, or the tap resumes work.
//  8. A desk reaches the owner's tap through ProductAPI.

type disclosureFixture struct {
	*system1Fixture
	slug string
	desk string
}

func newDisclosureFixture(t *testing.T, slug string, uid uint32) *disclosureFixture {
	t.Helper()
	f := &disclosureFixture{system1Fixture: newSystem1Fixture(t), slug: slug, desk: "app:" + slug}
	registerLeaseApp(t, slug, uid, true)
	disclosure.ResetCounts()
	t.Cleanup(disclosure.ResetCounts)
	return f
}

// run seeds a run of the desk and taints it with labels.
func (f *disclosureFixture) run(t *testing.T, runID, desk, prompt string, labels ...string) toolregistry.ExecutionContext {
	t.Helper()
	rec := types.RunRecord{RunID: runID, AgentID: "agent-" + runID, OwnerID: f.owner, ComputerID: f.computer, ChannelID: "ch-" + runID,
		AgentProfile: desk, AgentRole: desk, State: types.RunRunning, Prompt: prompt}
	if err := f.s.CreateRun(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	f.rt.taintRun(runID, types.NewLabelSet(labels...))
	if _, err := f.rt.flushRunLabels(context.Background(), f.owner, f.computer, runID); err != nil {
		t.Fatal(err)
	}
	return toolregistry.ExecutionContext{RunID: runID, AgentID: rec.AgentID, OwnerID: f.owner, ComputerID: f.computer, RunRecord: &rec}
}

func (f *disclosureFixture) gate(exec toolregistry.ExecutionContext, desk string, sink disclosure.Sink, target, summary string) error {
	return f.rt.gateDisclosure(context.Background(), disclosureCall{exec: exec, deskRole: desk, sink: sink, target: target, summary: summary})
}

// escalating makes the judge answer yes at 0.5; clearing makes it answer yes at 0.9.
func (f *disclosureFixture) escalating() {
	f.judge.first = 0.5
	f.judge.prefer = map[string]string{disclosure.QuestionID: "yes"}
}
func (f *disclosureFixture) clearing() {
	f.judge.first = 0.9
	f.judge.prefer = map[string]string{disclosure.QuestionID: "yes"}
}

func (f *disclosureFixture) events(t *testing.T) []types.EventRecord {
	t.Helper()
	evs, err := f.s.ListAcknowledgedEventsByChannel(context.Background(), f.owner, disclosureChannel(f.computer), 1000)
	if err != nil {
		t.Fatal(err)
	}
	var out []types.EventRecord
	for _, e := range evs {
		if e.Kind == types.EventDisclosure {
			out = append(out, e)
		}
	}
	return out
}

func (f *disclosureFixture) pendingID(t *testing.T) string {
	t.Helper()
	for _, e := range f.events(t) {
		if e.Phase == disclosurePhasePending {
			return e.EventID
		}
	}
	t.Fatal("no pending disclosure on the tape")
	return ""
}

// 1: a run labelled only world_untrusted publishes, on a package desk and a
// built-in one, with no System 1 call.
func TestAWorldOnlyLabelledRunPublishes(t *testing.T) {
	f := newDisclosureFixture(t, "worldpub", 62821)
	for _, desk := range []string{f.desk, agentprofile.Texture} {
		exec := f.run(t, "run-world-"+strings.ReplaceAll(desk, ":", "-"), desk, "publish the day's stories", types.LabelWorldUntrusted, types.LabelPlatform, types.PublisherLabel("example-wire"))
		if err := f.gate(exec, desk, disclosure.SinkPublication, "doc-1", "publish revision r1 of doc-1"); err != nil {
			t.Fatalf("%s: a world-only run was refused: %v", desk, err)
		}
	}
	if f.judge.callCount() != 0 || len(f.events(t)) != 0 {
		t.Fatalf("a world-only publish reached System 1 (%d) or the tape (%d)", f.judge.callCount(), len(f.events(t)))
	}
	if disclosure.Counts()["publication|-|allow|allowed"] != 2 {
		t.Fatalf("counts = %v", disclosure.Counts())
	}
}

// 3: a built-in desk with an ask is counted and not blocked, and spends no
// System 1 call.
func TestABuiltInDeskIsCountedAndNotBlocked(t *testing.T) {
	f := newDisclosureFixture(t, "unusedbuiltin", 62822)
	exec := f.run(t, "run-builtin", agentprofile.Texture, "write the story", types.OwnerPrivateLabel("document"), types.LabelWorldUntrusted)
	for _, sink := range []disclosure.Sink{disclosure.SinkPublication, disclosure.SinkEgress, disclosure.SinkDeskMessage, disclosure.SinkLibraryRelease} {
		if err := f.gate(exec, agentprofile.Texture, sink, "somewhere", "a thing"); err != nil {
			t.Fatalf("a built-in desk was blocked at %s: %v", sink, err)
		}
	}
	if f.judge.callCount() != 0 || len(f.events(t)) != 0 {
		t.Fatalf("a built-in desk's ask spent System 1 (%d calls) or wrote the tape (%d)", f.judge.callCount(), len(f.events(t)))
	}
	counts := disclosure.Counts()
	if counts["publication|owner_private|ask|observed"] != 1 || counts["egress|owner_private|ask|observed"] != 0 || counts["library_release|owner_private|ask|observed"] != 1 {
		t.Fatalf("counts = %v", counts)
	}
	// Enforce mode, the MA3.7 flip, would park it.
	defer disclosure.SetMode(disclosure.ModeEnforce)()
	f.escalating()
	if err := f.gate(exec, agentprofile.Texture, disclosure.SinkPublication, "doc-1", "publish"); err == nil {
		t.Fatal("enforce mode let a built-in desk's ask pass")
	}
}

// 2, 4, 5, 6, 7: a package desk with owner-private labels is refused at
// publication; the ask reaches System 1; its distribution is a precommit
// record carrying the run's labels; the rule escalates a middling answer; the
// owner's tap is a tape fact; an approval lets exactly one retry pass.
func TestAPackageDeskWithOwnerPrivateLabelsIsRefusedThenApprovedByTheOwner(t *testing.T) {
	f := newDisclosureFixture(t, "pubdesk", 62823)
	f.escalating()
	exec := f.run(t, "run-pub", f.desk, "summarise my notes for me", types.OwnerPrivateLabel("document"), types.LabelWorldUntrusted)
	const target, summary = "story:doc-9", "publish revision r9 of doc-9"

	err := f.gate(exec, f.desk, disclosure.SinkPublication, target, summary)
	if err == nil || !strings.Contains(err.Error(), "waits on the owner") {
		t.Fatalf("first attempt = %v, want the fate to say it waits on the owner", err)
	}
	if f.judge.callCount() != 1 {
		t.Fatalf("System 1 calls = %d, want 1", f.judge.callCount())
	}
	// The judge was asked a closed yes/no/unsure question over host evidence.
	state := f.judge.lastState()
	if !strings.Contains(state, "summarise my notes for me") || !strings.Contains(state, "doc-9") {
		t.Fatalf("the judge's evidence = %s", state)
	}

	// The distribution is a precommit-kind record with the run's labels.
	recID := exec.RunID + ":disclosure:"
	var rec types.CommitmentRecord
	found := false
	recs, err := f.s.ListCommitmentRecords(context.Background(), f.owner, f.computer, "", 1000)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range recs {
		if strings.HasPrefix(r.RecordID, recID) {
			rec, found = r, true
		}
	}
	if !found || rec.Kind != types.CommitmentKindPrecommit || rec.Precommit == nil {
		t.Fatalf("no disclosure precommit record among %d", len(recs))
	}
	if got := rec.Precommit.Distribution; got["yes"] != 0.5 || len(got) != 3 {
		t.Fatalf("recorded distribution = %v", got)
	}
	if want := types.NewLabelSet(types.OwnerPrivateLabel("document"), types.LabelWorldUntrusted); strings.Join(rec.Provenance.Labels, ",") != strings.Join(want, ",") {
		t.Fatalf("record labels = %v, want %v", rec.Provenance.Labels, want)
	}

	// The pending disclosure is a fact on the tape.
	id := f.pendingID(t)
	var pending disclosurePayload
	for _, e := range f.events(t) {
		if e.EventID == id {
			_ = json.Unmarshal(e.Payload, &pending)
		}
	}
	if pending.Sink != "publication" || pending.Rule != "escalate" || pending.Distribution["yes"] != 0.5 || pending.RecordID == "" || len(pending.Labels) != 2 {
		t.Fatalf("pending fact = %+v", pending)
	}

	// A second attempt does not spend another call: it waits.
	if err := f.gate(exec, f.desk, disclosure.SinkPublication, target, summary); err == nil || !strings.Contains(err.Error(), "waits on the owner") {
		t.Fatalf("second attempt = %v", err)
	}
	if f.judge.callCount() != 1 {
		t.Fatalf("System 1 calls = %d after a retry, want 1", f.judge.callCount())
	}

	// The owner's routes: list, then approve. Another owner sees nothing.
	h := &APIHandler{rt: f.rt}
	call := func(method, path, user string, handler http.HandlerFunc) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		if user != "" {
			req.Header.Set("X-Authenticated-User", user)
		}
		w := httptest.NewRecorder()
		handler(w, req)
		return w
	}
	if got := call(http.MethodGet, "/api/disclosures", "", h.HandleDisclosures); got.Code != http.StatusUnauthorized {
		t.Fatalf("an unauthenticated list = %d", got.Code)
	}
	list := call(http.MethodGet, "/api/disclosures", f.owner, h.HandleDisclosures)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), id) || !strings.Contains(list.Body.String(), "yes 0.50") {
		t.Fatalf("list = %d %s", list.Code, list.Body.String())
	}
	if got := call(http.MethodPost, "/api/disclosures/"+id+"/approve", "owner-intruder", h.HandleDisclosureRouter); got.Code != http.StatusNotFound {
		t.Fatalf("another owner's approve = %d", got.Code)
	}
	if got := call(http.MethodGet, "/api/disclosures/"+id+"/approve", f.owner, h.HandleDisclosureRouter); got.Code != http.StatusMethodNotAllowed {
		t.Fatalf("a GET approve = %d", got.Code)
	}
	if got := call(http.MethodPost, "/api/disclosures/"+id+"/approve", f.owner, h.HandleDisclosureRouter); got.Code != http.StatusOK {
		t.Fatalf("approve = %d %s", got.Code, got.Body.String())
	}
	if got := call(http.MethodPost, "/api/disclosures/"+id+"/deny", f.owner, h.HandleDisclosureRouter); got.Code != http.StatusNotFound {
		t.Fatalf("a second answer = %d, want it refused", got.Code)
	}
	// The approval is a tape fact, by the owner, naming the pending event.
	var approved *types.EventRecord
	for _, e := range f.events(t) {
		if e.Phase == disclosurePhaseApproved {
			e := e
			approved = &e
		}
	}
	var ap disclosurePayload
	if approved == nil || json.Unmarshal(approved.Payload, &ap) != nil || ap.DisclosureID != id || ap.Actor != "owner" {
		t.Fatalf("approval fact = %+v", approved)
	}

	// The retry passes once.
	if err := f.gate(exec, f.desk, disclosure.SinkPublication, target, summary); err != nil {
		t.Fatalf("the retry after the owner's approval: %v", err)
	}
	if err := f.gate(exec, f.desk, disclosure.SinkPublication, target, summary); err == nil {
		t.Fatal("an approval let a second attempt pass")
	}
	if got := call(http.MethodGet, "/api/disclosures", f.owner, h.HandleDisclosures); !strings.Contains(got.Body.String(), `"disclosures":[`) {
		t.Fatalf("list after = %s", got.Body.String())
	}
}

// 6: a denial is a tape fact and holds; a different disclosure is a different
// question.
func TestTheOwnerDenialHoldsForThatDisclosureOnly(t *testing.T) {
	f := newDisclosureFixture(t, "denydesk", 62824)
	f.escalating()
	exec := f.run(t, "run-deny", f.desk, "tidy my notes", types.OwnerPrivateLabel("document"))
	if err := f.gate(exec, f.desk, disclosure.SinkPublication, "doc-up", "post notes"); err == nil {
		t.Fatal("first attempt passed")
	}
	id := f.pendingID(t)
	if phase, err := f.rt.answerDisclosure(context.Background(), f.owner, id, false); err != nil || phase != disclosurePhaseDenied {
		t.Fatalf("deny = %q, %v", phase, err)
	}
	if err := f.gate(exec, f.desk, disclosure.SinkPublication, "doc-up", "post notes"); err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("after the denial = %v", err)
	}
	if err := f.gate(exec, f.desk, disclosure.SinkPublication, "doc-other", "post other notes"); err == nil || !strings.Contains(err.Error(), "waits on the owner") {
		t.Fatalf("a different disclosure = %v, want a fresh ask", err)
	}
}

// 4: the rule allows only a distribution that clears it; it records the
// distribution either way; no owner tap is raised.
func TestARuleClearedDistributionAllowsAndIsStillRecorded(t *testing.T) {
	f := newDisclosureFixture(t, "clearing", 62825)
	f.clearing()
	exec := f.run(t, "run-clear", f.desk, "email nobody; publish my reading list", types.OwnerPrivateLabel("document"))
	if err := f.gate(exec, f.desk, disclosure.SinkPublication, "doc-3", "publish revision r3"); err != nil {
		t.Fatalf("a cleared distribution was refused: %v", err)
	}
	phases := []string{}
	for _, e := range f.events(t) {
		phases = append(phases, e.Phase)
	}
	if len(phases) != 1 || phases[0] != disclosurePhaseAllowed {
		t.Fatalf("tape phases = %v, want [allowed]", phases)
	}
	recs, _ := f.s.ListCommitmentRecords(context.Background(), f.owner, f.computer, "", 1000)
	n := 0
	for _, r := range recs {
		if strings.HasPrefix(r.RecordID, exec.RunID+":disclosure:") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("disclosure precommit records = %d", n)
	}
	// A distribution that says no is not allowed even when yes is high-ish.
	f.judge.first, f.judge.prefer = 0.9, map[string]string{disclosure.QuestionID: "no"}
	exec2 := f.run(t, "run-no", f.desk, "keep my notes private", types.OwnerPrivateLabel("document"))
	if err := f.gate(exec2, f.desk, disclosure.SinkPublication, "doc-3", "publish revision r3"); err == nil {
		t.Fatal("a no distribution was allowed")
	}
}

// A failed System 1 call escalates; it never allows.
func TestAFailedJudgeEscalatesToTheOwner(t *testing.T) {
	f := newDisclosureFixture(t, "failjudge", 62826)
	f.judge.err = context.DeadlineExceeded
	exec := f.run(t, "run-fail", f.desk, "x", types.OwnerPrivateLabel("document"))
	err := f.gate(exec, f.desk, disclosure.SinkPublication, "doc-4", "publish")
	if err == nil || !strings.Contains(err.Error(), "waits on the owner") {
		t.Fatalf("err = %v", err)
	}
}

// The table's decision for each sink, for a package desk carrying
// owner-private labels, and admission for a message.
func TestALabelledRunAtEachSinkGetsTheTablesDecision(t *testing.T) {
	f := newDisclosureFixture(t, "eachsink", 62827)
	f.escalating()
	exec := f.run(t, "run-each", f.desk, "x", types.OwnerPrivateLabel("document"))
	for _, sink := range []disclosure.Sink{disclosure.SinkPublication, disclosure.SinkLibraryRelease, disclosure.SinkMailSend, disclosure.SinkDeskMessage} {
		if err := f.gate(exec, f.desk, sink, "elsewhere", "send "+string(sink)); err == nil || !strings.Contains(err.Error(), "waits on the owner") {
			t.Errorf("%s: %v, want an ask that waits on the owner", sink, err)
		}
	}
	// Egress (search, fetch) is allowed outright (owner, 2026-11-05).
	if err := f.gate(exec, f.desk, disclosure.SinkEgress, "elsewhere", "search"); err != nil {
		t.Errorf("egress: %v, want allowed", err)
	}
	// A message to the custodian of the label's kind is allowed outright.
	if err := f.gate(exec, f.desk, disclosure.SinkDeskMessage, agentprofile.Texture, "cast to texture"); err != nil {
		t.Fatalf("a message to the document's custodian: %v", err)
	}
}

// Mail: only the mail custodian replying in the owner's thread is admitted.
func TestMailSendIsAdmittedOnlyForTheCustodianRepliesInTheOwnersThread(t *testing.T) {
	f := newDisclosureFixture(t, "mailer", 62828)
	labels := types.NewLabelSet(types.OwnerPrivateLabel("mail"))
	reply := disclosureCall{deskRole: "mail", sink: disclosure.SinkMailSend, mailReply: true}
	if v := disclosure.Decide(labels, disclosure.SinkMailSend, f.rt.disclosureFacts(reply)); v.Decision != disclosure.Allow {
		t.Fatalf("the mail custodian replying in the thread = %s", v.Decision)
	}
	if v := disclosure.Decide(labels, disclosure.SinkMailSend, f.rt.disclosureFacts(disclosureCall{deskRole: "mail", sink: disclosure.SinkMailSend})); v.Decision != disclosure.Ask {
		t.Errorf("mail in a new thread = %s, want ask", v.Decision)
	}
}

// 2: the egress call a package desk's cell makes passes the gate before it
// reaches the host; an owner-private label allows egress (owner, 2026-11-05),
// so the fetch reaches the host (unbound here) rather than an ask.
func TestAPackageDeskEgressFetchPassesTheGate(t *testing.T) {
	f := newDisclosureFixture(t, "fetcher", 62829)
	f.escalating()
	exec := f.run(t, "run-fetch", f.desk, "x", types.OwnerPrivateLabel("document"))
	payload, _ := json.Marshal(yaegikernel.ReadPayload{URL: "https://example.com/?q=my+private+notes"})
	_, err := f.rt.deskEgress(context.Background(), f.desk, exec, yaegikernel.ActionRead, payload)
	if err != nil && (strings.Contains(err.Error(), "waits on the owner") || strings.Contains(err.Error(), "disclosure")) {
		t.Fatalf("an owner-private fetch was gated: %v", err)
	}
	// A world-only run fetches (it reaches the host, which is unbound here).
	world := f.run(t, "run-fetch-world", f.desk, "x", types.LabelWorldUntrusted)
	_, err = f.rt.deskEgress(context.Background(), f.desk, world, yaegikernel.ActionRead, payload)
	if err != nil && strings.Contains(err.Error(), "disclosure") {
		t.Fatalf("a world-only fetch was gated: %v", err)
	}
}

// 8: no desk reaches the owner's tap.
func TestProductAPIRefusesTheOwnersDisclosureRoutes(t *testing.T) {
	for _, p := range []string{"/api/disclosures", "/api/disclosures/abc/approve"} {
		if err := validateProductAPIRoute(http.MethodPost, p); err == nil {
			t.Errorf("ProductAPI allows %s", p)
		}
	}
}

// The observability route reports the gate.
func TestObservabilityReportsTheDisclosureGate(t *testing.T) {
	f := newDisclosureFixture(t, "observer", 62830)
	exec := f.run(t, "run-obs", agentprofile.Research, "x", types.OwnerPrivateLabel("capture"))
	_ = f.gate(exec, agentprofile.Research, disclosure.SinkPublication, "d", "s")
	h := &APIHandler{rt: f.rt}
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability", nil)
	req.Header.Set("X-Authenticated-User", f.owner)
	w := httptest.NewRecorder()
	h.HandleGuestObservability(w, req)
	var resp struct {
		Gate *struct {
			Mode   string            `json:"mode"`
			Counts map[string]uint64 `json:"counts"`
		} `json:"disclosure_gate"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.Gate == nil {
		t.Fatalf("observability = %d %s", w.Code, w.Body.String())
	}
	if resp.Gate.Mode != "observe" || resp.Gate.Counts["publication|owner_private|ask|observed"] != 1 {
		t.Fatalf("disclosure_gate = %+v", resp.Gate)
	}
}

// 2: the reducer gates a Cast or Report to a desk that is not the custodian of
// a label the run carries; the custodian is not gated; a message to the owner
// leaves no custody.
func TestTheReducerGatesAMessageToANonCustodianDesk(t *testing.T) {
	f := newDisclosureFixture(t, "messenger", 62831)
	f.escalating()
	exec := f.run(t, "run-msg", f.desk, "x", types.OwnerPrivateLabel("document"))
	r := &rlmCallReduction{mb: f.rt, rec: exec.RunRecord, scope: ReductionScope{RunID: exec.RunID, OwnerID: f.owner, ComputerID: f.computer, DeskAgentID: exec.AgentID}}
	cast := func(to string) yaegikernel.StagedIntent {
		return yaegikernel.StagedIntent{LocalID: "i1", Kind: yaegikernel.IntentCast, ToDesk: to, Statement: "please draft this"}
	}
	if err := r.gateMessageDisclosure(context.Background(), cast(agentprofile.Management)); err == nil || !strings.Contains(err.Error(), "waits on the owner") {
		t.Fatalf("a cast to a non-custodian desk = %v", err)
	}
	if err := r.gateMessageDisclosure(context.Background(), cast(agentprofile.Texture+":doc-1")); err != nil {
		t.Fatalf("a cast to the document's custodian = %v", err)
	}
	if err := r.gateMessageDisclosure(context.Background(), cast("owner")); err != nil {
		t.Fatalf("a message to the owner = %v", err)
	}
	if err := r.gateMessageDisclosure(context.Background(), yaegikernel.StagedIntent{LocalID: "i2", Kind: yaegikernel.IntentPrecommit}); err != nil {
		t.Fatalf("an unaddressed act = %v", err)
	}
}

// B1: a search query is an egress sink, gated at the dispatch (gateEgressCall
// is the step the async search dispatch runs before it starts a search). An
// owner-private label allows egress for every desk (owner, 2026-11-05: "No,
// no asking before each search"), so neither a built-in nor a package desk
// asks.
func TestALabelledSearchStartIsAllowedForEveryDesk(t *testing.T) {
	f := newDisclosureFixture(t, "searcher", 62832)
	f.escalating()
	payload, _ := json.Marshal(yaegikernel.SearchStartPayload{Query: "my private notes about the merger"})
	builtin := f.run(t, "run-search-b", agentprofile.Research, "x", types.OwnerPrivateLabel("document"))
	if err := f.rt.gateEgressCall(context.Background(), agentprofile.Research, builtin, yaegikernel.ActionSearchStart, payload); err != nil {
		t.Fatalf("a built-in desk's search was blocked: %v", err)
	}
	pkg := f.run(t, "run-search-p", f.desk, "x", types.OwnerPrivateLabel("document"))
	if err := f.rt.gateEgressCall(context.Background(), f.desk, pkg, yaegikernel.ActionSearchStart, payload); err != nil {
		t.Fatalf("a package desk's search was blocked: %v", err)
	}
	if n := disclosure.Counts()["egress|owner_private|ask|observed"]; n != 0 {
		t.Fatalf("a search asked: counts = %v", disclosure.Counts())
	}
	if f.judge.callCount() != 0 {
		t.Fatalf("a search spent System 1 (%d calls)", f.judge.callCount())
	}
}

// B2: the address keys on the full desk name; admission is checked against
// the resolved desk; Decline and a child's Complete are message sinks.
func TestMessageApprovalsDoNotSpanPackageDesks(t *testing.T) {
	f := newDisclosureFixture(t, "notes", 62833)
	registerLeaseApp(t, "weather", 62834, true)
	f.escalating()
	exec := f.run(t, "run-two", f.desk, "x", types.OwnerPrivateLabel("document"))
	r := &rlmCallReduction{mb: f.rt, rec: exec.RunRecord, scope: ReductionScope{RunID: exec.RunID, OwnerID: f.owner, ComputerID: f.computer, DeskAgentID: exec.AgentID, ReturnTo: "app:weather"}}
	in := func(id, to string) yaegikernel.StagedIntent {
		return yaegikernel.StagedIntent{LocalID: id, Kind: yaegikernel.IntentDecline, ToDesk: to, Body: "same text"}
	}
	if err := r.gateMessageDisclosure(context.Background(), in("a", "app:notes")); err == nil {
		t.Fatal("a decline to a non-custodian passed")
	}
	id := f.pendingID(t)
	if _, err := f.rt.answerDisclosure(context.Background(), f.owner, id, true); err != nil {
		t.Fatal(err)
	}
	// The approval is for app:notes; app:weather is a different disclosure.
	if err := r.gateMessageDisclosure(context.Background(), in("b", "app:weather")); err == nil || !strings.Contains(err.Error(), "waits on the owner") {
		t.Fatalf("an approval for app:notes passed a message to app:weather: %v", err)
	}
	if err := r.gateMessageDisclosure(context.Background(), in("c", "app:notes")); err != nil {
		t.Fatalf("the approved disclosure: %v", err)
	}
	a := disclosureDigest("o", "c", disclosureCall{deskRole: "d", sink: disclosure.SinkDeskMessage, target: "app:notes", summary: "s"}, nil)
	b := disclosureDigest("o", "c", disclosureCall{deskRole: "d", sink: disclosure.SinkDeskMessage, target: "app:weather", summary: "s"}, nil)
	if a == b {
		t.Fatal("digests of two app desks are equal")
	}
	if name, ok := resolveMessageDesk("app:weather"); !ok || name != "app:weather" {
		t.Fatalf("resolve = %q %v", name, ok)
	}
	if name, ok := resolveMessageDesk(agentprofile.Texture + ":doc-1"); !ok || name != agentprofile.Texture {
		t.Fatalf("resolve texture subject = %q %v", name, ok)
	}
	// A child's Complete returns to its requester: gated like a Reply.
	if err := r.gateMessage(context.Background(), yaegikernel.IntentComplete, r.scope.ReturnTo, "result"); err == nil {
		t.Fatal("a Complete to a non-custodian requester was not gated")
	}
}

// 11: the approve route matches exactly.
func TestDisclosureRouteRefusesTrailingSegments(t *testing.T) {
	f := newDisclosureFixture(t, "routes", 62835)
	h := &APIHandler{rt: f.rt}
	req := httptest.NewRequest(http.MethodPost, "/api/disclosures/abc/approve/extra", nil)
	req.Header.Set("X-Authenticated-User", f.owner)
	w := httptest.NewRecorder()
	h.HandleDisclosureRouter(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("trailing segment = %d", w.Code)
	}
}

// 6: a store error reading the run's labels fails an enforced desk closed.
func TestAnUnreadableLabelSetFailsAPackageDeskClosed(t *testing.T) {
	f := newDisclosureFixture(t, "closed", 62836)
	exec := f.run(t, "run-closed", f.desk, "x", types.LabelWorldUntrusted)
	_ = f.s.Close()
	err := f.gate(exec, f.desk, disclosure.SinkPublication, "d", "s")
	if err == nil || !strings.Contains(err.Error(), "could not be read") {
		t.Fatalf("package desk with unreadable labels = %v", err)
	}
	if err := f.gate(exec, agentprofile.Texture, disclosure.SinkPublication, "d", "s"); err != nil {
		t.Fatalf("a built-in desk was blocked: %v", err)
	}
	if disclosure.Counts()["publication|unreadable|labels|observed"] == 0 {
		t.Fatalf("counts = %v", disclosure.Counts())
	}
}

// texture.brief (the enforce-flip prerequisite): when the Texture custodian's
// run Casts research, the host mints a custodian lease bound to that run, so
// the brief carrying the run's documents is admitted. Ways it could fail
// (written before the code):
//
//	T1. the brief still asks (no lease, or the gate does not consult it);
//	T2. another run's message (another trajectory's documents) is admitted by
//	    the lease, or a label outside Texture's custody rides along unasked;
//	T3. a non-Texture desk's message to research mints a lease, or Texture's
//	    message to another desk does;
//	T4. the lease outlives its run or its expiry;
//	T5. the lease is not on the tape.
func TestATextureBriefToResearchIsAdmittedByTheCustodianLease(t *testing.T) {
	f := newDisclosureFixture(t, "briefs", 62851)
	f.escalating()
	defer disclosure.SetMode(disclosure.ModeEnforce)()
	ctx := context.Background()
	reduction := func(exec toolregistry.ExecutionContext) *rlmCallReduction {
		return &rlmCallReduction{mb: f.rt, rec: exec.RunRecord, scope: ReductionScope{RunID: exec.RunID, OwnerID: f.owner, ComputerID: f.computer, DeskAgentID: exec.AgentID}}
	}
	cast := func(id, to string) yaegikernel.StagedIntent {
		return yaegikernel.StagedIntent{LocalID: id, Kind: yaegikernel.IntentCast, ToDesk: to, Objective: "research the claims in this draft"}
	}
	asks := func(err error) bool { return err != nil && strings.Contains(err.Error(), "waits on the owner") }
	doc := types.OwnerPrivateLabel("document")

	// T1: Texture's brief to research is admitted, and the lease is on the tape.
	texA := f.run(t, "run-tex-a", agentprofile.Texture, "draft", doc, types.LabelWorldUntrusted, types.OwnerPrivateLabel("texture"))
	if err := reduction(texA).gateMessageDisclosure(ctx, cast("c1", agentprofile.Research)); err != nil {
		t.Fatalf("T1: Texture's brief to research = %v", err)
	}
	live := f.rt.leases.Live(f.owner)
	if len(live) != 1 || live[0].Holder != agentprofile.Research || live[0].HolderRunID != "run-tex-a" || live[0].Scope.Grants[0] != "texture.brief" {
		t.Fatalf("T5: live leases = %+v", live)
	}
	stored, err := f.rt.leases.All(ctx, f.owner, f.computer)
	if err != nil || len(stored) != 1 {
		t.Fatalf("T5: the lease is not on the tape: %v %+v", err, stored)
	}
	if f.judge.callCount() != 0 {
		t.Fatalf("T1: the admitted brief spent System 1 (%d calls)", f.judge.callCount())
	}

	// T2: another Texture run's message is not admitted by run A's lease (the
	// gate is consulted directly, so nothing is minted for run B).
	_ = f.run(t, "run-tex-b", agentprofile.Texture, "draft", doc)
	// (The owner flow now carries document and texture labels between Texture
	// and research without an ask, so admission is asked of the lease check
	// itself.)
	if f.rt.messageAdmitted(f.owner, "run-tex-b", agentprofile.Research, doc) {
		t.Fatal("T2: another run's message under run A's lease was admitted")
	}
	// A label outside Texture's custody still asks, lease or not.
	texC := f.run(t, "run-tex-c", agentprofile.Texture, "draft", doc, types.OwnerPrivateLabel("mail"))
	if err := reduction(texC).gateMessageDisclosure(ctx, cast("c2", agentprofile.Research)); !asks(err) {
		t.Fatalf("T2: a mail label rode the brief = %v", err)
	}

	// T3: management's message to research and Texture's to management mint
	// nothing and ask.
	before := len(f.rt.leases.Live(f.owner))
	mgmt := f.run(t, "run-mgmt", agentprofile.Management, "x", doc)
	if err := reduction(mgmt).gateMessageDisclosure(ctx, cast("c3", agentprofile.Research)); !asks(err) {
		t.Fatalf("T3: management's message to research = %v", err)
	}
	texD := f.run(t, "run-tex-d", agentprofile.Texture, "draft", doc)
	// Texture to management is on the owner's delegation chain: no ask, and
	// no lease minted for it.
	if err := reduction(texD).gateMessageDisclosure(ctx, cast("c4", agentprofile.Management)); err != nil {
		t.Fatalf("T3: Texture's message to management = %v", err)
	}
	if n := len(f.rt.leases.Live(f.owner)); n != before {
		t.Fatalf("T3: a lease was minted for another path (%d -> %d)", before, n)
	}

	// T4: the lease ends with its run.
	f.rt.endRunLeases(ctx, "run-tex-a", "run ended")
	if f.rt.messageAdmitted(f.owner, "run-tex-a", agentprofile.Research, doc) {
		t.Fatal("T4: the lease outlived its run")
	}
	// And at its expiry.
	texE := f.run(t, "run-tex-e", agentprofile.Texture, "draft", doc)
	if err := reduction(texE).gateMessageDisclosure(ctx, cast("c5", agentprofile.Research)); err != nil {
		t.Fatalf("T1: Texture's brief to research = %v", err)
	}
	later := time.Now().UTC().Add(2 * lease.MaxDuration)
	f.rt.leases.SetClock(func() time.Time { return later })
	if f.rt.messageAdmitted(f.owner, "run-tex-e", agentprofile.Research, doc) {
		t.Fatal("T4: the lease outlived its expiry")
	}
}

// A research run that read Texture's snippets carries owner_private:texture;
// its message back to Texture goes to that label's custodian and is admitted.
// The label's kind is the custodian's name, not an object kind, so it must
// name Texture's kinds (d@e 2026-11-06, trajectory 3ee6da5c: six
// desk_message|owner_private asks with a texture.brief lease live).
func TestAResearchAnswerCarryingTextureLabelsIsAdmittedByTheCustodian(t *testing.T) {
	f := newDisclosureFixture(t, "answers", 62852)
	f.escalating()
	defer disclosure.SetMode(disclosure.ModeEnforce)()
	res := f.run(t, "run-res-a", agentprofile.Research, "research", types.OwnerPrivateLabel("texture"), types.OwnerPrivateLabel("document"))
	if err := f.gate(res, agentprofile.Research, disclosure.SinkDeskMessage, agentprofile.Texture, "Report findings"); err != nil {
		t.Fatalf("research's answer to the custodian = %v", err)
	}
	if f.judge.callCount() != 0 {
		t.Fatalf("the admitted answer spent System 1 (%d calls)", f.judge.callCount())
	}
	// The same labels to a desk that is not the custodian still ask.
	if err := f.gate(res, agentprofile.Research, disclosure.SinkDeskMessage, agentprofile.Management, "Report findings"); err == nil || !strings.Contains(err.Error(), "waits on the owner") {
		t.Fatalf("texture labels to management = %v", err)
	}
}

// The gate records who asked: a built-in desk's observed ask names the sender,
// the recipient desk, the message kind and the labels, and changes no decision.
func TestTheGateTalliesTheSiteOfAnObservedAsk(t *testing.T) {
	f := newDisclosureFixture(t, "sites", 62840)
	// A system desk is decided in observe mode; a package desk is always
	// enforced, so the fixture's app desk would block.
	exec := f.run(t, "run-sites", agentprofile.Texture, "x", types.OwnerPrivateLabel("document"))
	r := &rlmCallReduction{mb: f.rt, rec: exec.RunRecord, scope: ReductionScope{RunID: exec.RunID, OwnerID: f.owner, ComputerID: f.computer, DeskAgentID: exec.AgentID}}
	in := yaegikernel.StagedIntent{LocalID: "s1", Kind: yaegikernel.IntentCast, ToDesk: agentprofile.Management + ":inst-secret-1", Statement: "private body text"}
	if err := r.gateMessageDisclosure(context.Background(), in); err != nil {
		t.Fatalf("an observed ask blocked: %v", err)
	}
	sites := disclosure.Sites()
	found := false
	for k, n := range sites {
		if strings.Contains(k, "private") && !strings.Contains(k, "owner_private:document") || strings.Contains(k, "secret") || strings.Contains(k, "body") {
			t.Fatalf("a site key carries content: %q", k)
		}
		if strings.HasPrefix(k, "desk_message|ask|") && strings.Contains(k, "->"+agentprofile.Management+"|cast|owner_private:document") && n == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("sites = %v", sites)
	}
	if disclosure.Counts()["desk_message|owner_private|ask|observed"] != 1 {
		t.Fatalf("counts = %v", disclosure.Counts())
	}
}

// Owner ruling 2026-11-06: an owner's capture used by Texture needs no ask and
// no notice; the provenance stays. The shapes d@e showed (research to texture
// report, texture to research cast, carrying owner_private:capture and
// owner_private:evidence) are allowed, in enforce mode too, with the labels
// still on the run; the same labels to any other desk or to a publication,
// mail or library sink behave as before.
func TestOwnerCapturesFlowBetweenTextureAndResearchWithoutAnAsk(t *testing.T) {
	f := newDisclosureFixture(t, "ownerflow", 62840)
	f.escalating()
	capture, evidence := types.OwnerPrivateLabel("capture"), types.OwnerPrivateLabel("evidence")
	res := f.run(t, "run-of-res", agentprofile.Research, "x", capture, evidence)
	tex := f.run(t, "run-of-tex", agentprofile.Texture, "x", capture, evidence)
	for _, enforce := range []bool{false, true} {
		if enforce {
			defer disclosure.SetMode(disclosure.ModeEnforce)()
		}
		if err := f.gate(res, agentprofile.Research, disclosure.SinkDeskMessage, agentprofile.Texture+":doc-1", "report"); err != nil {
			t.Fatalf("enforce=%v research->texture = %v", enforce, err)
		}
		if err := f.gate(tex, agentprofile.Texture, disclosure.SinkDeskMessage, agentprofile.Research, "cast"); err != nil {
			t.Fatalf("enforce=%v texture->research = %v", enforce, err)
		}
	}
	if sites := disclosure.Sites(); len(sites) != 0 {
		t.Fatalf("an owner flow was tallied as an ask: %v", sites)
	}
	if f.judge.callCount() != 0 || len(f.events(t)) != 0 {
		t.Fatalf("an owner flow spent System 1 (%d) or wrote an owner tap (%d)", f.judge.callCount(), len(f.events(t)))
	}
	// Provenance is unchanged: the runs still carry both labels.
	for _, id := range []string{"run-of-res", "run-of-tex"} {
		got, err := f.rt.store.RunLabels(context.Background(), f.owner, f.computer, id)
		if err != nil || strings.Join(got, ",") != strings.Join(types.NewLabelSet(capture, evidence), ",") {
			t.Fatalf("run %s labels = %v %v", id, got, err)
		}
	}
	// Anywhere else it still asks: another desk, publication, library release.
	if err := f.gate(res, agentprofile.Research, disclosure.SinkDeskMessage, agentprofile.Management, "report"); err == nil {
		t.Error("a capture to management did not ask")
	}
	for _, sink := range []disclosure.Sink{disclosure.SinkPublication, disclosure.SinkLibraryRelease} {
		if err := f.gate(tex, agentprofile.Texture, sink, "elsewhere", "send"); err == nil {
			t.Errorf("a capture at %s did not ask", sink)
		}
	}
	// The pair is by desk, not by label alone: another label still asks.
	doc := f.run(t, "run-of-doc", agentprofile.Research, "x", types.OwnerPrivateLabel("mail"))
	if err := f.gate(doc, agentprofile.Research, disclosure.SinkDeskMessage, agentprofile.Texture+":doc-1", "report"); err == nil {
		t.Error("a mail label to texture did not ask")
	}
}
