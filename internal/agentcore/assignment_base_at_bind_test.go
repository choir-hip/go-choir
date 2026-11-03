package agentcore

// Problem 28 (docs/problems/selfdev-arm-rounds-fail-at-freeze-and-capsules-lack-real-content-2026-11-03.md):
// deepseek's assignment opened at 19:27:40Z on base 2cd70f60, a planned
// refresh booted e4f1c685 at 19:27:57Z, and the assignment bound at 19:28:08Z
// on the open-time base. Its candidate froze on 2cd70f60 and the updater
// refused the apply (base_moved_since_freeze). The base was captured at the
// Cast and re-preflighted at bind. Ways the fix can fail, written first:
//   F1 a request opened on X and bound after a planned restart onto Y still
//      spawns on X (the defect), or the store keeps X while the capsule gets
//      Y (the run, the capsule and the binding disagree);
//   F2 the moved base is not recorded on the bind, so Freeze's base (read
//      from the capsule's source ref) is not Y;
//   F3 work already done is re-based, or an unbound capsule of unknown
//      source is adopted (review r1): a rebase round (pinned to its notice's booted base, not HEAD), a request
//      seeded from a candidate, or a verification;
//   F4 the store accepts a re-stamp it cannot check: a stale prior ref, or a
//      verification or candidate binding;
//   F5 a candidate frozen on X before the restart is no longer refused;
//   F6 a crash boot binds the request instead of closing it.

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	p28BaseX = "2cd70f6064ee962dd753acc24191e91ad2b735cb"
	p28BaseY = "e4f1c685a3ced870362e633b390e710aa54103ab"
)

// headPreflight is the guest source HEAD as the capsule executor's preflight
// sees it: each commit has its own digest.
type headPreflight struct{ head string }

func p28Preflight(commit string) capsule.SourcePreflight {
	digest := strings.TrimPrefix(objectgraph.SHA256([]byte("tree:"+commit)), "sha256:")
	return capsule.SourcePreflight{SubjectDigest: digest, ArtifactRef: "capsule-source-git:" + commit + ":sha256:" + digest}
}

func (h *headPreflight) PreflightSourceSnapshot(_ context.Context, ref string) (capsule.SourcePreflight, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return p28Preflight(h.head), nil
	}
	if commit := capsule.SourceArtifactCommit(ref); commit != "" && p28Preflight(commit).ArtifactRef == ref {
		return p28Preflight(commit), nil
	}
	return capsule.SourcePreflight{}, errors.New("capsule source artifact is unavailable or corrupt")
}

func (h *headPreflight) PreflightSourceCommit(_ context.Context, commit string) (capsule.SourcePreflight, error) {
	return p28Preflight(commit), nil
}

// spawningCapsule spawns and mints like the executor and records the spawn.
type spawningCapsule struct {
	absentAssignmentCapsule
	spawned   []capsule.SpawnSpec
	destroyed []string
	live      map[string]bool
}

func (c *spawningCapsule) Spawn(_ context.Context, spec capsule.SpawnSpec) (*capsule.Capsule, error) {
	c.spawned = append(c.spawned, spec)
	if c.live == nil {
		c.live = map[string]bool{}
	}
	c.live[spec.CapsuleID] = true
	return &capsule.Capsule{ID: spec.CapsuleID, State: capsule.StateActive}, nil
}

func (c *spawningCapsule) HasCapsule(id string) bool { return c.live[id] }

func (c *spawningCapsule) ForceDestroy(_ context.Context, id string) error {
	c.destroyed = append(c.destroyed, id)
	delete(c.live, id)
	return nil
}

func (c *spawningCapsule) InspectCapsuleRaw(id string) (*capsule.CapsuleDiagnostics, error) {
	if c.live[id] {
		return &capsule.CapsuleDiagnostics{}, nil
	}
	return nil, errors.New("capsule not found")
}

func (c *spawningCapsule) MintCapabilityHandle(runID string, role capsule.AgentRole, capsuleID, handle string, ttl time.Duration, slot string) (*capsule.Capability, error) {
	verbs := capsule.VerbSet{}
	for verb, allowed := range capsule.RoleVerbSets[role] {
		if allowed {
			verbs[verb] = true
		}
	}
	return &capsule.Capability{CapabilityID: "cap-" + capsuleID, Handle: handle, CapsuleID: capsuleID, AgentRunID: runID, AgentRole: role,
		Slot: slot, TargetCapsule: capsuleID, Verbs: verbs, ExpiresAt: time.Now().UTC().Add(ttl), KeyID: "key-test", Signature: []byte("sig")}, nil
}

type baseAtBindFixture struct {
	*admissionFixture
	head    *headPreflight
	capsule *spawningCapsule
	wakes   *[]admissionWake
}

// newBaseAtBindFixture opens one ordinary implementation request whose Cast
// captured HEAD = X, as planEngineeringCast does.
func newBaseAtBindFixture(t *testing.T, castBody string) *baseAtBindFixture {
	t.Helper()
	rt, s := testRuntime(t)
	computerID := rt.TextureComputerID()
	seed, err := store.SeedEngineeringAssignmentAuthorityWithCastBody(s, "owner-p28", computerID, "assignment", 1, "")
	if err != nil {
		t.Fatal(err)
	}
	f := &baseAtBindFixture{admissionFixture: &admissionFixture{t: t, rt: rt, s: s, seed: seed, computer: computerID},
		head: &headPreflight{head: p28BaseX}, capsule: &spawningCapsule{}}
	rt.assignmentRuntime = f.capsule
	rt.sourcePreflight = f.head
	rt.capsuleExecutor = capsule.NewExecutor(t.TempDir(), t.TempDir(), t.TempDir(), 0)
	f.wakes = captureAdmissionWakes(rt)

	assignmentID := seed.AssignmentIDs[0]
	req := store.SeedEngineeringOpenRequest(seed, 0, deterministicDocumentAssignmentCapability(assignmentID, 1))
	opened := p28Preflight(p28BaseX)
	req.Binding.SourceArtifactRef = opened.ArtifactRef
	req.Binding.SubjectDigest = "sha256:" + opened.SubjectDigest
	if castBody != "" {
		req.Records.Records[0].Directive.Body = castBody
		req.Records.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req.Records)
	}
	req.CommandDigest, _ = store.ComputeOpenEngineeringAssignmentDigest(req)
	if _, err := s.OpenEngineeringAssignment(context.Background(), req); err != nil {
		t.Fatalf("open: %v", err)
	}
	f.opens = append(f.opens, req)
	return f
}

// plannedRefresh is vmctl's marker-backed refresh onto Y: the boot projects
// the guest source HEAD onto Y and keeps the queued request.
func (f *baseAtBindFixture) plannedRefresh() {
	f.restart(true)
	f.head.head = p28BaseY
	f.rt.reconcileEngineeringAssignmentCapsulesAfterRestart(context.Background())
}

func (f *baseAtBindFixture) wake() error {
	content, _ := json.Marshal(delegatedAssignmentSpawnDeadline{AssignmentID: f.opens[0].AssignmentID, Attempt: 1})
	return f.rt.HandleDelegatedAssignmentSpawnDeadline(context.Background(), f.seed.OwnerID, f.computer, EngineeringAdmissionMailbox, string(content))
}

// F1, F2: opened on X, planned restart onto Y, then bind: the capsule, the
// stored binding and the run are on Y, and Freeze's base (the capsule's
// source ref commit) is Y.
func TestAssignmentBaseCapturedAtBindAfterPlannedRestart(t *testing.T) {
	f := newBaseAtBindFixture(t, "")
	f.plannedRefresh()
	if got := f.get(0); got.Disposition != types.EngineeringAssignmentOpen {
		t.Fatalf("fixture: a planned boot closed the queued request: %s (%q)", got.Disposition, got.DispositionReason)
	}
	if err := f.wake(); err != nil {
		t.Fatalf("bind after the planned restart: %v", err)
	}
	bound := f.get(0)
	if bound.Disposition != types.EngineeringAssignmentBound {
		t.Fatalf("not bound: %s (%q)", bound.Disposition, bound.DispositionReason)
	}
	want := p28Preflight(p28BaseY)
	if bound.Binding.SourceArtifactRef != want.ArtifactRef || bound.Binding.SubjectDigest != "sha256:"+want.SubjectDigest {
		t.Fatalf("F1: the binding kept %s / %s, want the booted base's %s", bound.Binding.SourceArtifactRef, bound.Binding.SubjectDigest, want.ArtifactRef)
	}
	if len(f.capsule.spawned) != 1 {
		t.Fatalf("spawned %d capsules, want one", len(f.capsule.spawned))
	}
	spec := f.capsule.spawned[0]
	if spec.SourceArtifactRef != want.ArtifactRef || spec.ExpectedSubjectDigest != want.SubjectDigest {
		t.Fatalf("F1: the capsule spawned on %s, want %s", spec.SourceArtifactRef, want.ArtifactRef)
	}
	// Freeze (capsule emitSourcePatch) takes the release base from the
	// capsule's pinned source ref.
	if got := capsule.SourceArtifactCommit(spec.SourceArtifactRef); got != p28BaseY {
		t.Fatalf("F2: Freeze would record base %s, want %s", got, p28BaseY)
	}
	run, err := f.s.GetLifecycleRun(context.Background(), bound.Binding.OwnerID, bound.Binding.ComputerID, bound.BoundRunID)
	if err != nil {
		t.Fatal(err)
	}
	if metadataStringValue(run.Metadata, "source_artifact_ref") != want.ArtifactRef || metadataStringValue(run.Metadata, "subject_digest") != "sha256:"+want.SubjectDigest {
		t.Fatalf("F1: the bound run names %v / %v", run.Metadata["source_artifact_ref"], run.Metadata["subject_digest"])
	}
}

// A request bound with no base move keeps its committed binding (no re-stamp).
func TestAssignmentBaseUnchangedBindKeepsTheCastBase(t *testing.T) {
	f := newBaseAtBindFixture(t, "")
	if err := f.wake(); err != nil {
		t.Fatal(err)
	}
	bound := f.get(0)
	if bound.Disposition != types.EngineeringAssignmentBound || bound.Binding.SourceArtifactRef != p28Preflight(p28BaseX).ArtifactRef {
		t.Fatalf("bound %s on %s, want bound on X", bound.Disposition, bound.Binding.SourceArtifactRef)
	}
}

// F3: a rebase round stays pinned to the booted base its notice named, never
// HEAD; a capsule that already exists keeps the base it was spawned on.
func TestAssignmentBaseAtBindLeavesStartedOrPinnedWorkAlone(t *testing.T) {
	t.Run("rebase round", func(t *testing.T) {
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
		assignment, err := f.s.GetEngineeringAssignment(context.Background(), f.owner, f.computer, plan.AssignmentID, 1)
		if err != nil {
			t.Fatal(err)
		}
		f.rt.assignmentRuntime = &spawningCapsule{}
		if f.rt.takesSourceAtBind(context.Background(), assignment) {
			t.Fatal("F3: a rebase round would take HEAD at bind instead of its notice's booted base")
		}
		if _, _, ok := f.rt.sourceAtBind(context.Background(), assignment); ok {
			t.Fatal("F3: a rebase round read its source at bind")
		}
	})
	// Review r1 (DeepSeek, blocking): an attempt that spawned on Y and never
	// bound leaves a capsule; the retry must not bind it as X. An unbound
	// capsule ran no engineer, so it is replaced and the bind is on Y.
	t.Run("unbound capsule from an interrupted attempt", func(t *testing.T) {
		f := newBaseAtBindFixture(t, "")
		f.capsule.live = map[string]bool{f.opens[0].Binding.CapsuleID: true}
		f.head.head = p28BaseY
		if err := f.wake(); err != nil {
			t.Fatal(err)
		}
		want := p28Preflight(p28BaseY).ArtifactRef
		got := f.get(0)
		if got.Disposition != types.EngineeringAssignmentBound || got.Binding.SourceArtifactRef != want {
			t.Fatalf("bound %s on %s, want bound on Y", got.Disposition, got.Binding.SourceArtifactRef)
		}
		if len(f.capsule.destroyed) != 1 || len(f.capsule.spawned) != 1 || f.capsule.spawned[0].SourceArtifactRef != want {
			t.Fatalf("the interrupted attempt's capsule was adopted, not replaced: destroyed %v, spawned %d", f.capsule.destroyed, len(f.capsule.spawned))
		}
	})
}

// F4: the store re-stamps only an unbound implementation request whose
// committed source is the one the runtime read.
func TestAssignmentBaseAtBindStoreRefusesUncheckedRestamp(t *testing.T) {
	f := newBaseAtBindFixture(t, "")
	open := f.opens[0]
	moved := p28Preflight(p28BaseY)
	bind := func(prior string) types.BindEngineeringAssignmentRequest {
		req := store.SeedEngineeringBindRequest(open, "run:"+open.AssignmentID, deterministicDocumentAssignmentCapability(open.AssignmentID, 1))
		req.Source = &types.EngineeringBindSource{PriorSourceArtifactRef: prior, SourceArtifactRef: moved.ArtifactRef, SubjectDigest: "sha256:" + moved.SubjectDigest}
		req.Run.Metadata["source_artifact_ref"] = moved.ArtifactRef
		req.Run.Metadata["subject_digest"] = "sha256:" + moved.SubjectDigest
		req.CommandDigest, _ = store.ComputeBindEngineeringAssignmentDigest(req)
		return req
	}
	if _, err := f.s.BindEngineeringAssignment(context.Background(), bind(p28Preflight("0000000000000000000000000000000000000000").ArtifactRef)); !errors.Is(err, store.ErrEngineeringAssignmentInvalid) {
		t.Fatalf("F4: a re-stamp from a stale prior ref was accepted: %v", err)
	}
	bound, err := f.s.BindEngineeringAssignment(context.Background(), bind(open.Binding.SourceArtifactRef))
	if err != nil {
		t.Fatalf("a checked re-stamp was refused: %v", err)
	}
	if bound.Assignment.Binding.SourceArtifactRef != moved.ArtifactRef {
		t.Fatalf("the bind did not record the moved base: %s", bound.Assignment.Binding.SourceArtifactRef)
	}
}

// F5: a candidate frozen on X before the restart is still refused on Y.
func TestAssignmentBaseAtBindFrozenOldBaseCandidateStillRefused(t *testing.T) {
	fx := joinFixture(t, "p28", p28BaseY, p28BaseX)
	fx.requireRefusedForBase(t, "p28-old-base", p28BaseX, p28BaseY)
}

// F6: a crash boot closes the open request and never binds it.
func TestAssignmentBaseAtBindCrashBootClosesTheRequest(t *testing.T) {
	f := newBaseAtBindFixture(t, "")
	f.restart(false)
	f.head.head = p28BaseY
	*f.wakes = nil
	f.rt.reconcileEngineeringAssignmentCapsulesAfterRestart(context.Background())
	got := f.get(0)
	if !got.Disposition.Terminal() || !strings.Contains(got.DispositionReason, engineeringInterruptedByRestart) {
		t.Fatalf("F6: a crash boot left the request %s (%q)", got.Disposition, got.DispositionReason)
	}
	if err := f.wake(); err != nil {
		t.Fatal(err)
	}
	if len(f.capsule.spawned) != 0 || f.get(0).Binding.SourceArtifactRef != p28Preflight(p28BaseX).ArtifactRef {
		t.Fatalf("F6: a closed request was spawned or re-based after a crash boot")
	}
}
