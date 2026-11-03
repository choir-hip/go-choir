package agentcore

import (
	"context"
	"errors"
	"github.com/yusefmosiah/go-choir/internal/visualcapture"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Freeze's visual evidence on the product path
// (docs/design/visual-evidence-slice-2026-10-15.md V17, V22, V23): the
// captures reach the computer's asset store, the result tells the cell what
// resolved, a failed store write does not fail the Freeze, and the visual_captured
// rule settles from the runtime's observation.

type failingAssetStore struct{ err error }

func (f failingAssetStore) Put([]byte) (string, string, error) { return "", "", f.err }

// stagedVisualRoot lays out <root>/evidence/visual/<digest>.png as Freeze
// stages it, and returns the record that cites it.
func stagedVisualRoot(t *testing.T, shots ...[]byte) (string, *capsule.VisualEvidence) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "evidence", "visual")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	visual := &capsule.VisualEvidence{
		ManifestSHA256: strings.Repeat("8", 64), FrontendSHA256: strings.Repeat("7", 64),
		CaptureReceipt: "capsule-exec:sha256:" + strings.Repeat("9", 64),
	}
	routes := []string{"/", "/choir-drill/x.html", "/c", "/d"}
	for i, shot := range shots {
		digest := sha256HexForTest(shot)
		if err := os.WriteFile(filepath.Join(dir, digest+".png"), shot, 0o444); err != nil {
			t.Fatal(err)
		}
		visual.Captures = append(visual.Captures, capsule.VisualEvidenceCapture{
			Artifact: "artifact:sha256:" + digest, Route: routes[i], MediaType: "image/png", Bytes: int64(len(shot)),
			Width: 40, Height: 30, Blank: i == 1,
		})
	}
	return root, visual
}

func TestRecordFreezeVisualPutsCapturesInTheAssetStoreAndTellsTheCell(t *testing.T) {
	runtime, _ := testRuntime(t)
	shotA, shotB := assetPNG(t), append(assetPNG(t)[:len(assetPNG(t)):len(assetPNG(t))], 0)
	root, visual := stagedVisualRoot(t, shotA, shotB)
	out := map[string]any{}

	recordFreezeVisual(runtime.imageStore(), root, visual, out)

	rows, _ := out["visual_captures"].([]map[string]any)
	if len(rows) != 2 {
		t.Fatalf("visual_captures = %v", out["visual_captures"])
	}
	for i, row := range rows {
		if row["artifact"] != visual.Captures[i].Artifact || row["route"] != visual.Captures[i].Route || row["resolvable"] != true {
			t.Fatalf("row %d = %v", i, row)
		}
		if !runtime.imageStore().Has(strings.TrimPrefix(visual.Captures[i].Artifact, "artifact:sha256:")) {
			t.Fatalf("capture %d is not in the asset store", i)
		}
	}
	if out["visual_capture_receipt"] != visual.CaptureReceipt {
		t.Fatalf("receipt = %v", out["visual_capture_receipt"])
	}
	if _, bad := out["visual_store_errors"]; bad {
		t.Fatalf("store errors on a clean write: %v", out["visual_store_errors"])
	}
	// The internal observation rides under a key the cell never sees.
	var obs types.EngineeringFreezeObservation
	applyFreezeVisual(&obs, out)
	if !obs.VisualKnown || obs.VisualReceipt != visual.CaptureReceipt || len(obs.VisualCaptures) != 2 ||
		obs.VisualCaptures[0].Route != "/" || obs.VisualCaptures[0].Blank || !obs.VisualCaptures[1].Blank {
		t.Fatalf("observation = %+v", obs)
	}
	if _, left := out[freezeVisualKey]; left {
		t.Fatal("the internal observation key survived into the cell's result")
	}
}

// V22: the bundle is the record; a failed store write is reported, not fatal.
func TestRecordFreezeVisualReportsAFailedStoreWriteWithoutFailingTheFreeze(t *testing.T) {
	root, visual := stagedVisualRoot(t, assetPNG(t))
	out := map[string]any{}
	recordFreezeVisual(failingAssetStore{err: errors.New("disk full")}, root, visual, out)
	rows, _ := out["visual_captures"].([]map[string]any)
	if len(rows) != 1 || rows[0]["resolvable"] != false {
		t.Fatalf("rows = %v", out["visual_captures"])
	}
	errs, _ := out["visual_store_errors"].([]string)
	if len(errs) != 1 || !strings.Contains(errs[0], "disk full") {
		t.Fatalf("store errors = %v", out["visual_store_errors"])
	}
	// The rule still sees the captures: the bundle cites them.
	var obs types.EngineeringFreezeObservation
	applyFreezeVisual(&obs, out)
	if len(obs.VisualCaptures) != 1 {
		t.Fatalf("observation = %+v", obs)
	}

	// A staged file that is not the cited bytes is reported, never stored.
	runtime, _ := testRuntime(t)
	root, visual = stagedVisualRoot(t, assetPNG(t))
	digest := strings.TrimPrefix(visual.Captures[0].Artifact, "artifact:sha256:")
	if err := os.Chmod(filepath.Join(root, "evidence", "visual", digest+".png"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "evidence", "visual", digest+".png"), []byte("not the cited bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	out = map[string]any{}
	recordFreezeVisual(runtime.imageStore(), root, visual, out)
	if rows, _ := out["visual_captures"].([]map[string]any); len(rows) != 1 || rows[0]["resolvable"] != false {
		t.Fatalf("a swapped staged file reported resolvable: %v", out["visual_captures"])
	}
	if runtime.imageStore().Has(digest) {
		t.Fatal("swapped bytes were stored under the cited digest")
	}
}

func TestRecordFreezeVisualWithNoCapturesStillObservesNone(t *testing.T) {
	out := map[string]any{}
	recordFreezeVisual(failingAssetStore{}, t.TempDir(), nil, out)
	if _, present := out["visual_captures"]; present {
		t.Fatalf("a freeze without captures reports them: %v", out)
	}
	var obs types.EngineeringFreezeObservation
	applyFreezeVisual(&obs, out)
	if !obs.VisualKnown || len(obs.VisualCaptures) != 0 || obs.VisualReceipt != "" {
		t.Fatalf("observation = %+v, want known and empty (visual_captured settles no)", obs)
	}
	// A replayed freeze carries no observation key: nothing is known.
	var replay types.EngineeringFreezeObservation
	applyFreezeVisual(&replay, map[string]any{"bundle_digest": "x"})
	if replay.VisualKnown {
		t.Fatal("a replayed freeze claims to have observed the evidence")
	}
}

// R1, R2, V23: a visual_captured precommit is recorded with its routes
// normalized and the assignment stamped, and settles from the runtime's
// observation, not the desk's word.
func TestVisualCapturedPrecommitRecordedAndSettledFromTheFreezeObservation(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-visual-precommit", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, run := c1eOpenBound(t, s, seed, 0)
	run.Metadata[runMetadataModel] = "test-model-1"
	r := &rlmCallReduction{active: true, mb: rt, st: rt.store, ledger: rt.store, rec: &run, scope: ReductionScope{
		FromAgentID: run.AgentID, DeskAgentID: run.AgentID, FromRole: agentprofile.Engineering, ChannelID: run.AgentID,
		RunID: run.RunID, OwnerID: seed.OwnerID, ComputerID: seed.ComputerID, CellID: "cell-eng-1",
	}}
	in := rulePrecommitIntent(t, "shot", 0.8, map[string]any{"kind": "visual_captured", "routes": []string{" /choir-drill/x.html "}})
	if _, err := r.commitEngineeringPrecommit(ctx, in); err != nil {
		t.Fatal(err)
	}
	bad := rulePrecommitIntent(t, "bad", 0.8, map[string]any{"kind": "visual_captured", "routes": []string{"https://evil.example/"}})
	if err := validateEngineeringPrecommit(agentprofile.Engineering, bad, nil); err == nil {
		t.Fatal("a route off this origin passed the reducer's pre-commit check")
	}
	listed, err := s.ListRulePrecommitRecords(ctx, seed.OwnerID, seed.ComputerID, assignment.AssignmentID)
	if err != nil || len(listed) != 1 {
		t.Fatalf("listed %d precommits (%v)", len(listed), err)
	}
	rule := listed[0].Precommit.Rule
	if rule.Kind != types.PrecommitRuleVisualCaptured || strings.Join(rule.Routes, ",") != "/choir-drill/x.html" || rule.AssignmentID != assignment.AssignmentID {
		t.Fatalf("recorded rule = %+v", rule)
	}

	obs := types.EngineeringFreezeObservation{
		BundleDigest: "b1", OperationID: "op1", VisualKnown: true, VisualReceipt: "capsule-exec:sha256:" + strings.Repeat("9", 64),
		VisualCaptures: []types.VisualCaptureObservation{{Route: "/choir-drill/x.html", Artifact: "artifact:sha256:" + strings.Repeat("a", 64)}},
	}
	rt.settleRulePrecommits(ctx, seed.OwnerID, seed.ComputerID, assignment.AssignmentID, "freeze", time.Now().UTC(),
		func(rule types.PrecommitRule) (types.RuleSettlement, bool) {
			return types.SettleRuleOnFreeze(rule, obs)
		})

	all, err := s.ListCommitmentRecords(ctx, seed.OwnerID, seed.ComputerID, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	var verdict *types.CommitmentRecord
	for i := range all {
		if all[i].Resolve != nil && all[i].Provenance.AgentID == types.RuleScorerAgentID {
			verdict = &all[i]
		}
	}
	if verdict == nil || verdict.Resolve.Verdict != types.PrecommitAnswerYes || verdict.Scores[0].ScorerModelID != "rule:visual_captured" {
		t.Fatalf("settlement = %+v", verdict)
	}
	if !strings.Contains(strings.Join(verdict.Resolve.EvidenceRefs, " "), "artifact:sha256:"+strings.Repeat("a", 64)) {
		t.Fatalf("evidence refs = %v", verdict.Resolve.EvidenceRefs)
	}
}

// Problem 7: two routes that froze one screenshot are named in the Freeze
// result the engineer reads.
func TestRecordFreezeVisualWarnsWhenRoutesShareOneScreenshot(t *testing.T) {
	runtime, _ := testRuntime(t)
	root, visual := stagedVisualRoot(t, assetPNG(t))
	dup := visual.Captures[0]
	dup.Route = "/read"
	visual.Captures = append(visual.Captures, dup)
	out := map[string]any{}
	recordFreezeVisual(runtime.imageStore(), root, visual, out)
	warnings, _ := out["visual_warnings"].([]string)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "/ and /read") {
		t.Fatalf("visual_warnings = %v", out["visual_warnings"])
	}
	root, visual = stagedVisualRoot(t, assetPNG(t))
	out = map[string]any{}
	recordFreezeVisual(runtime.imageStore(), root, visual, out)
	if _, present := out["visual_warnings"]; present {
		t.Fatalf("one route warns: %v", out["visual_warnings"])
	}
}

// Fix 3 (trace review round 4): the failed asset answers Freeze carried into
// the record are named in the Freeze result's visual_warnings.
func TestRecordFreezeVisualWarnsOfFailedAssets(t *testing.T) {
	runtime, _ := testRuntime(t)
	root, visual := stagedVisualRoot(t, assetPNG(t))
	visual.AssetFailures = []visualcapture.AssetFailure{{Path: "/pdf-text-BRoX2Cg_.js", Status: 404, Count: 2}}
	out := map[string]any{}
	recordFreezeVisual(runtime.imageStore(), root, visual, out)
	warnings, _ := out["visual_warnings"].([]string)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "404 /pdf-text-BRoX2Cg_.js (x2)") {
		t.Fatalf("visual_warnings = %v", out["visual_warnings"])
	}
}
