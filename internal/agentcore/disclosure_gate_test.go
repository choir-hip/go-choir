package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/disclosure"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// MA3.6 in the runtime. Ways the disclosure path could fail (written first):
//
//  1. The paper stops publishing: a run labelled only world_untrusted is asked
//     at publication.
//  2. A package desk carrying owner-private data publishes, fetches or
//     messages anywhere with no ask.
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
	if counts["publication|owner_private|ask|observed"] != 1 || counts["egress|owner_private|ask|observed"] != 1 || counts["library_release|owner_private|ask|observed"] != 1 {
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
	if err := f.gate(exec, f.desk, disclosure.SinkEgress, "https://example.com/upload", "post notes"); err == nil {
		t.Fatal("first attempt passed")
	}
	id := f.pendingID(t)
	if phase, err := f.rt.answerDisclosure(context.Background(), f.owner, id, false); err != nil || phase != disclosurePhaseDenied {
		t.Fatalf("deny = %q, %v", phase, err)
	}
	if err := f.gate(exec, f.desk, disclosure.SinkEgress, "https://example.com/upload", "post notes"); err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("after the denial = %v", err)
	}
	if err := f.gate(exec, f.desk, disclosure.SinkEgress, "https://example.com/other", "post other notes"); err == nil || !strings.Contains(err.Error(), "waits on the owner") {
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
	for _, sink := range []disclosure.Sink{disclosure.SinkPublication, disclosure.SinkEgress, disclosure.SinkLibraryRelease, disclosure.SinkMailSend, disclosure.SinkDeskMessage} {
		if err := f.gate(exec, f.desk, sink, "elsewhere", "send "+string(sink)); err == nil || !strings.Contains(err.Error(), "waits on the owner") {
			t.Errorf("%s: %v, want an ask that waits on the owner", sink, err)
		}
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

// 2: the egress call a package desk's cell makes is gated before it reaches
// the host.
func TestAPackageDeskEgressFetchIsGatedBeforeTheHost(t *testing.T) {
	f := newDisclosureFixture(t, "fetcher", 62829)
	f.escalating()
	exec := f.run(t, "run-fetch", f.desk, "x", types.OwnerPrivateLabel("document"))
	payload, _ := json.Marshal(yaegikernel.ReadPayload{URL: "https://example.com/?q=my+private+notes"})
	_, err := f.rt.deskEgress(context.Background(), f.desk, exec, yaegikernel.ActionRead, payload)
	if err == nil || !strings.Contains(err.Error(), "waits on the owner") {
		t.Fatalf("egress = %v", err)
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
