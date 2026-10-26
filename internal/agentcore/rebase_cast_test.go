package agentcore

// U4 slice 2 (docs/design/u4-stack-rebase-build-2026-10-26.md, slice 3 and
// failure modes 6, 9, 10, D1, D2, D3): management's Cast bound to the notice
// is an ordinary implementation assignment whose capsule starts from B'.
// Ways it can fail, written before the code:
//
//   1. the capsule starts from HEAD (the projection commit on the old base)
//      and the round "rebases" onto the old base -> ...PinsTheCapsuleToTheBootedBase
//   2. the gate widens: a wrong base, a wrong operation, a switch off, an
//      applied stack that is not on a moved base, or missing objects admits
//      a rebase -> ...RefusesWhatIsNotTheMovedStack
//   3. a spec without a notice is admitted at stack_base_moved, or the
//      refusal text changes -> ...WithoutANoticeKeepsTodaysRefusal
//   4. the Cast opens an assignment but the pair is not claimed, or the claim
//      names another assignment -> ...OpensWithTheClaim
//   5. rebase_of is read from anything but the tape: an ordinary assignment,
//      or an assignment whose pair another assignment claimed, derives a
//      lineage -> ...LineageIsDerivedFromTheTape
//   6. a stray rebase field reaches an off switch's ordinary path
//      -> ...WithoutANoticeKeepsTodaysRefusal

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/sourceproj"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// fakeSourcePreflight stands in for the linux-only capsule executor's
// preflight and records what it was asked for.
type fakeSourcePreflight struct {
	head    string
	refs    []string
	commits []string
}

func (f *fakeSourcePreflight) preflight(commit string) capsule.SourcePreflight {
	digest := strings.Repeat("d", 64)
	return capsule.SourcePreflight{SubjectDigest: digest, ArtifactRef: "capsule-source-git:" + commit + ":sha256:" + digest}
}

func (f *fakeSourcePreflight) PreflightSourceSnapshot(_ context.Context, ref string) (capsule.SourcePreflight, error) {
	f.refs = append(f.refs, ref)
	return f.preflight(f.head), nil
}

func (f *fakeSourcePreflight) PreflightSourceCommit(_ context.Context, commit string) (capsule.SourcePreflight, error) {
	f.commits = append(f.commits, commit)
	return f.preflight(commit), nil
}

type rebaseCastFixture struct {
	*rebaseFixture
	preflight *fakeSourcePreflight
	castID    string
}

// newRebaseCastFixture is the rebase fixture with attempt 1 minted, the root's
// Cast naming it (a round open), and a fake source preflight.
func newRebaseCastFixture(t *testing.T) *rebaseCastFixture {
	t.Helper()
	f := &rebaseCastFixture{rebaseFixture: newRebaseFixture(t), castID: "cast-root-1"}
	f.preflight = &fakeSourcePreflight{head: gitHead(t, f.repo)}
	f.rt.sourcePreflight = f.preflight
	f.rt.capsuleExecutor = capsule.NewExecutor(t.TempDir(), t.TempDir(), t.TempDir(), 0)
	if out := f.fire(t); out.Minted != f.noticeID(1) {
		t.Fatalf("fixture: attempt 1 not minted: %+v", out)
	}
	f.rootCast(t, f.castID, f.noticeID(1))
	return f
}

func (f *rebaseCastFixture) spec(mutate func(map[string]string)) string {
	fields := map[string]string{"parent": f.castID, "rebase_notice": f.noticeID(1), "rebase_of": f.opID, "base": f.moved}
	if mutate != nil {
		mutate(fields)
	}
	out := "{"
	first := true
	for k, v := range fields {
		if v == "" {
			continue
		}
		if !first {
			out += ","
		}
		first = false
		out += `"` + k + `":"` + v + `"`
	}
	return out + "}"
}

func (f *rebaseCastFixture) plan(t *testing.T, spec, objective string) (engineeringCastPlan, error) {
	t.Helper()
	scope := ReductionScope{
		FromAgentID: f.management, DeskAgentID: f.management, FromRole: agentprofile.Management, ChannelID: f.management,
		RunID: "run-management", OwnerID: f.owner, ComputerID: f.computer, CellID: "cell-" + strings.ReplaceAll(t.Name(), "/", "-"),
	}
	in := yaegikernel.StagedIntent{Kind: yaegikernel.IntentCast, LocalID: "e1", ToDesk: "engineering", Objective: objective, Statement: spec}
	return f.rt.planEngineeringCast(context.Background(), scope, &types.RunRecord{RunID: "run-management"}, in)
}

func (f *rebaseCastFixture) rebaseObjective() string {
	return "Rebase the applied change " + f.opID + " onto the platform base (rebase notice " + f.noticeID(1) + ")"
}

func TestRebaseCastPinsTheCapsuleToTheBootedBase(t *testing.T) {
	f := newRebaseCastFixture(t)
	plan, err := f.plan(t, f.spec(nil), f.rebaseObjective())
	if err != nil {
		t.Fatalf("a bound rebase Cast must plan: %v", err)
	}
	// Failure mode 9: the capsule starts from B', not from HEAD (the projection
	// commit on the old base).
	if len(f.preflight.commits) != 1 || f.preflight.commits[0] != f.moved || len(f.preflight.refs) != 0 {
		t.Fatalf("preflight asked for commits %v refs %v; want exactly the booted base %s", f.preflight.commits, f.preflight.refs, f.moved)
	}
	if got := capsule.SourceArtifactCommit(plan.Binding.SourceArtifactRef); got != f.moved || got == gitHead(t, f.repo) {
		t.Fatalf("binding source %s pins %q, want the booted base %s", plan.Binding.SourceArtifactRef, got, f.moved)
	}
	if !plan.Rebase {
		t.Fatal("the plan must say it is a rebase, so the open carries the admission clock")
	}
	b := plan.Binding
	if b.Kind != types.EngineeringAssignmentImplementation || b.CastAuthority != types.EngineeringCastAuthorityRecords || b.Attempt != 1 {
		t.Fatalf("a rebase round is an ordinary implementation assignment: %+v", b)
	}
	if b.ParentConversationID != types.InboxConversationID(f.management, f.root) {
		t.Fatalf("the parent is the root Texture RLM's round: %s", b.ParentConversationID)
	}
}

func TestRebaseCastOpensWithTheClaim(t *testing.T) {
	f := newRebaseCastFixture(t)
	plan, err := f.plan(t, f.spec(nil), f.rebaseObjective())
	if err != nil {
		t.Fatal(err)
	}
	records := types.CommitInboxRecordsRequest{OwnerID: f.owner, ComputerID: f.computer, CommandID: "cell-records:" + plan.AssignmentID, Records: []types.CommitmentRecord{plan.Cast}}
	records.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(records)
	if _, err := f.rt.openDelegatedCastAssignment(context.Background(), plan, records); err != nil {
		t.Fatalf("open: %v", err)
	}
	claim, err := f.s.GetRebaseNoticeClaim(context.Background(), f.owner, f.computer, types.StackRebasePairKey(f.opID, f.moved))
	if err != nil || claim.AssignmentID != plan.AssignmentID || claim.NoticeID != f.noticeID(1) {
		t.Fatalf("claim %+v, %v", claim, err)
	}
	// The pair is spent: the timer mints nothing more, and a second rebase Cast
	// (a retry is a new Cast) is refused by the gate.
	f.now = f.now.Add(3 * time.Hour)
	if out := f.fire(t); out.Minted != "" {
		t.Fatalf("a claimed pair minted %+v", out)
	}
	if _, err := f.plan(t, f.spec(nil), f.rebaseObjective()); err == nil || !strings.Contains(err.Error(), "rebase_pair_claimed") {
		t.Fatalf("second Cast for a claimed pair: %v", err)
	}
	// The assignment's lineage derives from the tape.
	assignment, err := f.s.GetEngineeringAssignment(context.Background(), f.owner, f.computer, plan.AssignmentID, 1)
	if err != nil {
		t.Fatal(err)
	}
	lineage, ok := f.rt.rebaseLineage(context.Background(), assignment)
	if !ok || lineage.OperationID != f.opID || lineage.BootedBase != f.moved || lineage.NoticeID != f.noticeID(1) || lineage.Attempt != 1 {
		t.Fatalf("lineage %+v ok=%v", lineage, ok)
	}
}

func TestRebaseCastRefusesWhatIsNotTheMovedStack(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(t *testing.T, f *rebaseCastFixture)
		spec   func(map[string]string)
		expect string
	}{
		{"wrong base", nil, func(m map[string]string) { m["base"] = f40("c") }, "rebase_base_mismatch"},
		{"wrong operation", nil, func(m map[string]string) { m["rebase_of"] = "op-zzz" }, "rebase_of_mismatch"},
		{"spec missing rebase_of", nil, func(m map[string]string) { m["rebase_of"] = "" }, "rebase_spec_incomplete"},
		{"spec missing base", nil, func(m map[string]string) { m["base"] = "" }, "rebase_spec_incomplete"},
		{"stack is projected, not moved", func(t *testing.T, f *rebaseCastFixture) { setBootedBase(t, f.rt, f.base) }, nil, "rebase_not_needed"},
		{"the booted base's objects are missing", func(t *testing.T, f *rebaseCastFixture) {
			setBootedBase(t, f.rt, f40("d"))
		}, func(m map[string]string) { m["base"] = f40("d") }, "rebase_base_missing"},
		{"source.rebase_offer off keeps today's refusal", func(t *testing.T, f *rebaseCastFixture) { f.setSwitch(t, store.SettingSourceRebaseOffer, false) }, nil, "stack_base_moved"},
		{"texture.root_desk off keeps today's refusal", func(t *testing.T, f *rebaseCastFixture) { f.setSwitch(t, store.SettingTextureRootDesk, false) }, nil, "stack_base_moved"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newRebaseCastFixture(t)
			if tc.setup != nil {
				tc.setup(t, f)
			}
			_, err := f.plan(t, f.spec(tc.spec), f.rebaseObjective())
			if err == nil || !strings.Contains(err.Error(), tc.expect) {
				t.Fatalf("want a refusal with %q, got %v", tc.expect, err)
			}
			if len(f.preflight.commits) != 0 || len(f.preflight.refs) != 0 {
				t.Fatalf("a refused Cast preflighted the capsule source: %v %v", f.preflight.commits, f.preflight.refs)
			}
		})
	}
}

func f40(c string) string { return strings.Repeat(c, 40) }

func TestRebaseCastWithoutANoticeKeepsTodaysRefusal(t *testing.T) {
	f := newRebaseCastFixture(t)
	// An ordinary request at stack_base_moved is refused exactly as before,
	// even when it carries rebase_of and base but no notice (D3).
	spec := f.spec(func(m map[string]string) { m["rebase_notice"] = "" })
	_, err := f.plan(t, spec, "add a feature")
	if err == nil || !strings.HasPrefix(strings.TrimPrefix(err.Error(), "engineering cast: "), "stack_base_moved") {
		t.Fatalf("an ordinary Cast at stack_base_moved: %v", err)
	}
	// Projection off: a rebase spec reaches nothing; an off switch with the
	// state not demanding a refusal still refuses the stray field.
	t.Setenv(sourceproj.EnvSwitch, "0")
	sourceproj.SetSetting(false)
	if _, err := f.plan(t, f.spec(nil), f.rebaseObjective()); err == nil || !strings.Contains(err.Error(), "rebase_offer_off") {
		t.Fatalf("projection off: %v", err)
	}
}

func TestRebaseLineageIsDerivedFromTheTape(t *testing.T) {
	f := newRebaseCastFixture(t)
	plan, err := f.plan(t, f.spec(nil), f.rebaseObjective())
	if err != nil {
		t.Fatal(err)
	}
	records := types.CommitInboxRecordsRequest{OwnerID: f.owner, ComputerID: f.computer, CommandID: "cell-records:" + plan.AssignmentID, Records: []types.CommitmentRecord{plan.Cast}}
	records.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(records)
	if _, err := f.rt.openDelegatedCastAssignment(context.Background(), plan, records); err != nil {
		t.Fatal(err)
	}
	assignment, _ := f.s.GetEngineeringAssignment(context.Background(), f.owner, f.computer, plan.AssignmentID, 1)

	// An assignment whose Cast names the notice but whose pair another
	// assignment claimed has no lineage.
	other := assignment
	other.AssignmentID = "assignment-other"
	if _, ok := f.rt.rebaseLineage(context.Background(), other); ok {
		t.Fatal("an assignment the claim does not name derived a lineage")
	}
	// An assignment whose opening Cast carries the notice but not rebase_of and
	// base (here the root's own Cast) has none: the spec must agree with the
	// notice.
	ordinary := assignment
	ordinary.Binding.ParentControlID = f.castID
	if _, ok := f.rt.rebaseLineage(context.Background(), ordinary); ok {
		t.Fatal("a Cast whose spec lacks rebase_of and base derived a lineage")
	}
	// A record that does not exist derives none, and nothing panics.
	missing := assignment
	missing.Binding.ParentControlID = "no-such-record"
	if _, ok := f.rt.rebaseLineage(context.Background(), missing); ok {
		t.Fatal("a missing Cast derived a lineage")
	}
}
