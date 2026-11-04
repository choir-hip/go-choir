package agentcore

// Problem 16 (docs/problems/selfdev-arm-rounds-fail-at-freeze-and-capsules-lack-real-content-2026-11-03.md,
// clustering assessment 2 in docs/problems/selfdev-round-blockers-cluster-2026-11-03.md):
// pool's rebase notice reached management as
// rebase-notice:selfdev-rebase:<op>:<base>:1, admission refused that and the
// right id, and the timer did not see the answer. The notice now travels as
// the notice record's own id. Ways it can fail, written before the code:
//   1. a miscopied naming reaches the tape and admission compares strings
//      -> ...TextureCastCarriesTheRecordsOwnID
//   2. resolution widens: a naming of no notice, or of a notice sent to
//      another Texture, is accepted -> ...TextureCastRefusesANamingOfNoNotice
//   3. management must copy the id again, and a miscopy or an omission
//      refuses the rebase -> ...ManagementCastTakesTheNoticeFromItsParent
//   4. an ordinary request (its parent names no notice) becomes a rebase
//      -> TestRebaseCastWithoutANoticeKeepsTodaysRefusal (parent without a notice)
//   5. a tape that already holds a miscopied parent (pool's) never admits
//      -> ...AdmissionReadsAMiscopiedParent
//   6. the timer does not see a miscopied answer, or a refused rebase waits
//      out the 30-minute window before the next attempt
//      -> ...TimerSeesAMiscopiedAnswer, ...RefusedRebaseReMintsPromptly

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

func (f *rebaseFixture) poolMiscopy(n int) string {
	return "rebase-notice:selfdev-rebase:" + f.opID + ":" + f.moved + ":" + itoa(n)
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

func (f *rebaseFixture) textureReduction(desk string) *rlmCallReduction {
	return &rlmCallReduction{active: true, mb: f.rt, st: f.s, ledger: f.s, scope: ReductionScope{OwnerID: f.owner, ComputerID: f.computer, DeskAgentID: desk}}
}

func textureRebaseCast(notice string) yaegikernel.StagedIntent {
	packet, _ := json.Marshal(map[string]any{"kind": "execution_request", "summary": "rebase", "rebase_notice": notice})
	return yaegikernel.StagedIntent{Kind: yaegikernel.IntentCast, Req: testRequest(), LocalID: "c1", ToDesk: "management", Objective: "Rebase", Statement: string(packet)}
}

func TestRebaseNoticeTextureCastCarriesTheRecordsOwnID(t *testing.T) {
	f := newRebaseFixture(t)
	if out := f.fire(t); out.Minted != f.noticeID(1) {
		t.Fatalf("fixture: %+v", out)
	}
	in := []yaegikernel.StagedIntent{textureRebaseCast(f.poolMiscopy(1))}
	out, err := f.textureReduction(f.root).resolveTextureRebaseNotices(context.Background(), in)
	if err != nil {
		t.Fatalf("pool's miscopy names notice 1: %v", err)
	}
	if got := types.CastRebaseNotice(out[0].Statement); got != f.noticeID(1) {
		t.Fatalf("the packet carries %q, want the record's id %q", got, f.noticeID(1))
	}
	if types.CastRebaseNotice(in[0].Statement) != f.poolMiscopy(1) {
		t.Fatal("the caller's intents were modified")
	}
	var packet map[string]any
	if json.Unmarshal([]byte(out[0].Statement), &packet) != nil || packet["summary"] != "rebase" || packet["kind"] != "execution_request" {
		t.Fatalf("the rest of the packet is kept: %s", out[0].Statement)
	}
	// The right id is left byte for byte.
	same, err := f.textureReduction(f.root).resolveTextureRebaseNotices(context.Background(), []yaegikernel.StagedIntent{textureRebaseCast(f.noticeID(1))})
	if err != nil || same[0].Statement != textureRebaseCast(f.noticeID(1)).Statement {
		t.Fatalf("a right id was rewritten: %v %s", err, same[0].Statement)
	}
}

func TestRebaseNoticeTextureCastRefusesANamingOfNoNotice(t *testing.T) {
	f := newRebaseFixture(t)
	f.fire(t)
	for name, naming := range map[string]string{
		"no attempt 2 on the tape": f.noticeID(2),
		"another operation":        types.StackRebaseNoticeID("op-zzz", f.moved, 1),
		"not a notice id":          "selfdev-rebase:" + f.opID,
	} {
		if _, err := f.textureReduction(f.root).resolveTextureRebaseNotices(context.Background(), []yaegikernel.StagedIntent{textureRebaseCast(naming)}); err == nil || !strings.Contains(err.Error(), "rebase_notice") {
			t.Fatalf("%s: want a refusal, got %v", name, err)
		}
	}
	// The notice was sent to the root: another Texture cannot answer it.
	if _, err := f.textureReduction(currentTextureAgentID("doc-other")).resolveTextureRebaseNotices(context.Background(), []yaegikernel.StagedIntent{textureRebaseCast(f.noticeID(1))}); err == nil {
		t.Fatal("a Texture answered a notice sent to another")
	}
	// A Cast without the field, or to another desk, is untouched.
	plain := yaegikernel.StagedIntent{Kind: yaegikernel.IntentCast, Req: testRequest(), LocalID: "c2", ToDesk: "management", Statement: `{"kind":"execution_request"}`}
	if out, err := f.textureReduction(f.root).resolveTextureRebaseNotices(context.Background(), []yaegikernel.StagedIntent{plain}); err != nil || out[0].Statement != plain.Statement {
		t.Fatalf("an ordinary Cast changed: %v", err)
	}
}

func TestRebaseNoticeManagementCastTakesTheNoticeFromItsParent(t *testing.T) {
	f := newRebaseCastFixture(t)
	// Management names only the parent: the notice, rebase_of and base come
	// from the notice record, and the objective names the id.
	plan, err := f.plan(t, `{"parent":"`+f.castID+`"}`, "Rebase the applied change onto the new base")
	if err != nil {
		t.Fatalf("a rebase Cast naming only its parent: %v", err)
	}
	if !plan.Rebase || types.CastRebaseNotice(plan.Cast.Directive.Body) != f.noticeID(1) || !strings.Contains(plan.Cast.Directive.Objective, f.noticeID(1)) {
		t.Fatalf("the Cast is not bound to the notice: rebase=%v body=%s objective=%q", plan.Rebase, plan.Cast.Directive.Body, plan.Cast.Directive.Objective)
	}
	var spec engineeringCastSpec
	if json.Unmarshal([]byte(plan.Cast.Directive.Body), &spec) != nil || spec.RebaseOf != f.opID || spec.Base != f.moved || spec.Parent != f.castID {
		t.Fatalf("spec %s", plan.Cast.Directive.Body)
	}
	records := types.CommitInboxRecordsRequest{OwnerID: f.owner, ComputerID: f.computer, CommandID: "cell-records:" + plan.AssignmentID, Records: []types.CommitmentRecord{plan.Cast}}
	records.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(records)
	if _, err := f.rt.openDelegatedCastAssignment(context.Background(), plan, records); err != nil {
		t.Fatalf("admission refused the bound Cast: %v", err)
	}
	if claim, err := f.s.GetRebaseNoticeClaim(context.Background(), f.owner, f.computer, types.StackRebasePairKey(f.opID, f.moved)); err != nil || claim.NoticeID != f.noticeID(1) {
		t.Fatalf("claim %+v %v", claim, err)
	}
}

func TestRebaseNoticeManagementCastResolvesAMiscopiedSpec(t *testing.T) {
	f := newRebaseCastFixture(t)
	plan, err := f.plan(t, f.spec(func(m map[string]string) { m["rebase_notice"] = f.poolMiscopy(1) }), "Rebase the applied change")
	if err != nil {
		t.Fatalf("a miscopied spec: %v", err)
	}
	if types.CastRebaseNotice(plan.Cast.Directive.Body) != f.noticeID(1) {
		t.Fatalf("body %s", plan.Cast.Directive.Body)
	}
	// A spec that disagrees with the record is still refused.
	if _, err := f.plan(t, f.spec(func(m map[string]string) { m["base"] = f40("c") }), f.rebaseObjective()); err == nil {
		t.Fatal("a spec naming another base was admitted")
	}
}

func TestRebaseNoticeAdmissionReadsAMiscopiedParent(t *testing.T) {
	f := &rebaseCastFixture{rebaseFixture: newRebaseFixture(t), castID: "cast-root-pool"}
	f.preflight = &fakeSourcePreflight{head: gitHead(t, f.repo)}
	f.rt.sourcePreflight = f.preflight
	f.rt.capsuleExecutor = capsule.NewExecutor(t.TempDir(), t.TempDir(), t.TempDir(), 0)
	f.fire(t)
	// Pool's tape: the Texture Cast was committed with the miscopy.
	f.rootCast(t, f.castID, f.poolMiscopy(1))
	plan, err := f.plan(t, `{"parent":"`+f.castID+`"}`, "Rebase")
	if err != nil {
		t.Fatal(err)
	}
	records := types.CommitInboxRecordsRequest{OwnerID: f.owner, ComputerID: f.computer, CommandID: "cell-records:" + plan.AssignmentID, Records: []types.CommitmentRecord{plan.Cast}}
	records.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(records)
	if _, err := f.rt.openDelegatedCastAssignment(context.Background(), plan, records); err != nil {
		t.Fatalf("admission refused pool's case: %v", err)
	}
}

func TestRebaseNoticeTimerSeesAMiscopiedAnswer(t *testing.T) {
	f := newRebaseFixture(t)
	f.fire(t)
	f.rootCast(t, "cast-root-1", f.poolMiscopy(1)) // answered, round open
	f.now = f.now.Add(2 * time.Hour)
	if out := f.fire(t); out.Minted != "" {
		t.Fatalf("an answered attempt with an open round was re-minted: %+v", out)
	}
}

func TestRebaseNoticeRefusedRebaseReMintsPromptly(t *testing.T) {
	f := newRebaseFixture(t)
	first := f.fire(t)
	if first.Rearm <= 0 || first.Rearm > f.rt.rebaseStayUp+time.Minute {
		t.Fatalf("after a mint the timer looks again within the stay-up interval, got %s", first.Rearm)
	}
	f.now = f.now.Add(time.Minute)
	f.rootCast(t, "cast-root-1", f.noticeID(1))
	f.now = f.now.Add(time.Minute)
	f.closeRound(t, "report-refused") // management's round ends: the rebase was refused
	f.now = f.now.Add(first.Rearm)
	if out := f.fire(t); out.Minted != f.noticeID(2) {
		t.Fatalf("a refused rebase waits for the next attempt: %+v", out)
	}
}

// Review r1 (DeepSeek F8): a spec naming another notice than its parent's is
// refused at plan time, before the capsule preflight.
func TestRebaseNoticeManagementCastNamingAnotherNoticeIsRefusedEarly(t *testing.T) {
	f := newRebaseCastFixture(t)
	_, err := f.plan(t, f.spec(func(m map[string]string) { m["rebase_notice"] = f.noticeID(2) }), f.rebaseObjective())
	if err == nil || !strings.Contains(err.Error(), "rebase_notice_mismatch") {
		t.Fatalf("want rebase_notice_mismatch, got %v", err)
	}
	if len(f.preflight.commits) != 0 || len(f.preflight.refs) != 0 {
		t.Fatal("the refused Cast preflighted")
	}
}
