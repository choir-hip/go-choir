package agentcore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The judge's per-shot review
// (docs/problems/judge-checks-only-named-defects-2026-11-04.md). The
// failure modes are listed in selfdev_shot_review.go (FM1-FM8); each test
// names the one it holds.

func shotFixture(n int) []capsule.ScreenFile {
	var out []capsule.ScreenFile
	for i := 0; i < n; i++ {
		out = append(out, capsule.ScreenFile{Name: fmt.Sprintf("%02d-agenda.png", i+1), SHA256: strings.Repeat(fmt.Sprintf("%x", i%16), 64), Width: 1280, Height: 800, Bytes: 100})
	}
	return out
}

// scriptedJudge answers each batch by the images it was sent; answer maps
// a shot name to its JSON object body (without "image").
type scriptedJudge struct {
	mu      sync.Mutex
	calls   int
	refs    [][]string
	prompts []string
	answer  func(name string) string
	err     error
}

func (j *scriptedJudge) call(_ context.Context, prompt string, refs []string) (modelpolicy.VerifyOutcome, error) {
	j.mu.Lock()
	j.calls++
	j.refs = append(j.refs, refs)
	j.prompts = append(j.prompts, prompt)
	j.mu.Unlock()
	if j.err != nil {
		return modelpolicy.VerifyOutcome{}, j.err
	}
	var parts []string
	for i := range refs {
		name := ""
		for _, line := range strings.Split(prompt, "\n") {
			if rest, ok := strings.CutPrefix(line, fmt.Sprintf("Image %d: ", i+1)); ok {
				name = rest
			}
		}
		parts = append(parts, fmt.Sprintf(`{"image": %d, %s}`, i+1, j.answer(name)))
	}
	return modelpolicy.VerifyOutcome{Model: "vision-x", Text: "```json\n[" + strings.Join(parts, ",") + "]\n```", InputTokens: 1500 * len(refs), OutputTokens: 60 * len(refs)}, nil
}

const allYes = `"q1":"yes","q2":"yes","q3":"yes","q4":"n/a","q6":"yes"`

func always(v bool) func(string) bool { return func(string) bool { return v } }

func TestShotReviewJudgesEveryShotInBoundedBatches(t *testing.T) {
	judge := &scriptedJudge{answer: func(string) string { return allYes }}
	review, err := reviewShots(context.Background(), judge.call, shotFixture(capsule.MaxScreens), nil, always(true))
	if err != nil {
		t.Fatal(err)
	}
	// FM7: 12 shots are three calls of four.
	if judge.calls != 3 || review.Calls != 3 {
		t.Fatalf("calls = %d (review %d), want 3", judge.calls, review.Calls)
	}
	seen := map[string]bool{}
	for _, refs := range judge.refs {
		if len(refs) > shotReviewBatch {
			t.Fatalf("a batch carried %d images", len(refs))
		}
		for _, r := range refs {
			seen[r] = true
		}
	}
	if len(review.Shots) != capsule.MaxScreens || review.rejected() {
		t.Fatalf("review = %+v", review)
	}
	if review.InputTokens != 1500*12 || review.OutputTokens != 60*12 || len(review.Models) != 1 {
		t.Fatalf("cost = %d in, %d out, models %v", review.InputTokens, review.OutputTokens, review.Models)
	}
	for _, p := range judge.prompts {
		for _, want := range []string{"opaque", "ends in an ellipsis", "without an ellipsis", "OUTSIDE the focused surface", "a modal over a scrim passes", "INSIDE the focused surface's bounds", "dimmed behind the backdrop is out of focus", "overprint", "mid-word", "styled as a control", "secondary"} {
			if !strings.Contains(p, want) {
				t.Fatalf("prompt misses %q", want)
			}
		}
	}
}

// The 2026-11-04 deepseek arm: shots the management Verify never looked at
// carried the defects. Every one must fail the candidate by name.
func TestShotReviewFailsOnADefectInAnyShotAndNamesIt(t *testing.T) {
	judge := &scriptedJudge{answer: func(name string) string {
		switch name {
		case "04-agenda.png":
			return `"q1":"yes","q2":"yes","q3":"no","q4":"n/a","q6":"yes","why":"time printed over the title: 2:45 PStandup"`
		case "10-agenda.png":
			return `"q1":"yes","q2":"yes","q3":"yes","q4":"n/a","q6":"unclear","why":"Import button is bare text"`
		}
		return allYes
	}}
	review, err := reviewShots(context.Background(), judge.call, shotFixture(11), nil, always(true))
	if err != nil {
		t.Fatal(err)
	}
	if !review.rejected() {
		t.Fatal("a shot with overprint passed")
	}
	reason := review.reason("the month view now shows times")
	for _, want := range []string{"shot review rejected 2 of 11 shots", "04-agenda.png Q3", "2:45 PStandup", "10-agenda.png Q6", "Management's Verify said pass: the month view"} {
		if !strings.Contains(reason, want) {
			t.Fatalf("reason %q misses %q", reason, want)
		}
	}
	// FM6: "unclear" is a no.
	if review.Shots[9].Q6 != "no" {
		t.Fatalf("q6 = %q", review.Shots[9].Q6)
	}
}

func TestShotReviewQuestionFourNeverRejectsAndNAIsOnlyForFour(t *testing.T) {
	judge := &scriptedJudge{answer: func(name string) string {
		if name == "01-agenda.png" {
			return `"q1":"yes","q2":"yes","q3":"yes","q4":"no","q6":"yes"`
		}
		return `"q1":"n/a","q2":"yes","q3":"yes","q4":"n/a","q6":"yes"`
	}}
	review, err := reviewShots(context.Background(), judge.call, shotFixture(2), nil, always(true))
	if err != nil {
		t.Fatal(err)
	}
	if got := review.Shots[0].rejects(); len(got) != 0 {
		t.Fatalf("q4 no rejected: %v", got)
	}
	if got := review.Shots[1].rejects(); len(got) != 1 || got[0] != "Q1" {
		t.Fatalf("n/a on q1 = %v, want Q1 rejected", got)
	}
}

// FM5: size is the bundle's, blank and unresolvable shots fail without a call.
func TestShotReviewSizeBlankAndUnresolvableNeedNoModel(t *testing.T) {
	screens := shotFixture(3)
	screens[0].Width, screens[0].Height = 1280, 2400 // a full-page capture
	screens[1].Blank = true
	unresolvable := screens[2].SHA256
	judge := &scriptedJudge{answer: func(string) string { return allYes }}
	review, err := reviewShots(context.Background(), judge.call, screens, nil, func(sha string) bool { return sha != unresolvable })
	if err != nil {
		t.Fatal(err)
	}
	if judge.calls != 1 || len(judge.refs[0]) != 1 {
		t.Fatalf("calls = %d refs %v, want one call for the one judgeable shot", judge.calls, judge.refs)
	}
	if got := review.Shots[0].rejects(); len(got) != 1 || got[0] != "Q5" {
		t.Fatalf("full-page shot rejects = %v", got)
	}
	if review.Shots[1].Source != "blank" || review.Shots[2].Source != "unresolvable" || len(review.Shots[1].rejects()) == 0 || len(review.Shots[2].rejects()) == 0 {
		t.Fatalf("blank/unresolvable = %+v", review.Shots[1:])
	}
}

func TestShotSizes(t *testing.T) {
	for _, c := range []struct {
		w, h int
		ok   bool
	}{{1100, 750, true}, {1280, 800, true}, {2560, 1600, true}, {390, 844, true}, {1170, 2532, true}, {1024, 1536, false}, {1280, 2400, false}, {1100, 751, false}, {195, 422, false}, {0, 0, false}} {
		if got := shotSizeOK(c.w, c.h); got != c.ok {
			t.Fatalf("%dx%d = %v, want %v", c.w, c.h, got, c.ok)
		}
	}
}

// FM1, FM2: a failed call, a non-JSON answer, or a missed image is an
// error, never a pass.
func TestShotReviewNeverPassesAnUnreviewedShot(t *testing.T) {
	failing := &scriptedJudge{err: errors.New("gateway 502")}
	if _, err := reviewShots(context.Background(), failing.call, shotFixture(5), nil, always(true)); err == nil {
		t.Fatal("a failed call passed")
	}
	prose := func(context.Context, string, []string) (modelpolicy.VerifyOutcome, error) {
		return modelpolicy.VerifyOutcome{Text: "All shots look good."}, nil
	}
	if _, err := reviewShots(context.Background(), prose, shotFixture(2), nil, always(true)); err == nil {
		t.Fatal("a prose answer passed")
	}
	short := func(context.Context, string, []string) (modelpolicy.VerifyOutcome, error) {
		return modelpolicy.VerifyOutcome{Text: `[{"image":1,` + allYes + `}]`}, nil
	}
	if _, err := reviewShots(context.Background(), short, shotFixture(2), nil, always(true)); err == nil || !strings.Contains(err.Error(), "misses image 2") {
		t.Fatalf("an answer missing a shot = %v", err)
	}
}

// The wiring in commitVerifyIntent: a pass on a frozen candidate whose
// review cannot run stays frozen with the error stamped (FM1), and a
// replayed pass the review already failed answers from the operation
// without a model call (FM3). A fail verdict runs no review (FM8).
func TestVerifyPassRunsTheShotReviewAndReplaysItsFail(t *testing.T) {
	ctx := context.Background()
	rt, s, assignment := assignmentEndFixture(t, "owner-shot-review")
	rt.selfdevUpdaterRoot = t.TempDir()
	operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateFrozen, "")
	if _, err := rt.commitAssignedEngineeringReport(ctx, c1eGet(t, s, assignment), terminalAssignmentReport(assignment, types.EngineeringResultCompleted)); err != nil {
		t.Fatal(err)
	}
	judge := &scriptedJudge{answer: func(string) string { return allYes }}
	rt.shotReviewCall = judge.call
	digest := strings.Repeat("f", 64)
	intent := yaegikernel.StagedIntent{LocalID: "v1", TargetRef: assignment.AssignmentID, Decision: "pass", BundleDigest: digest, VerifierRefs: []string{"ref"}}

	_, err := verifyReduction(rt, assignment).commitVerifyIntent(ctx, intent)
	if err == nil || !strings.Contains(err.Error(), "per-shot review did not run") {
		t.Fatalf("pass with an unreadable bundle = %v, want the review's refusal", err)
	}
	op := operationState(t, rt, assignment.Binding.ComputerID, operationID)
	if op.State != selfdev.StateFrozen || !strings.Contains(op.LastIntentError, "per-shot review did not run") {
		t.Fatalf("operation = %s %q, want frozen with the error stamped", op.State, op.LastIntentError)
	}

	failedError := selfDevelopmentJudgmentRejected + ": " + shotReviewMarker + " rejected 1 of 3 shots: 04-agenda.png Q3"
	if _, err := s.DB().Exec(`UPDATE self_development_operations SET state = ?, terminal_error = ? WHERE operation_id = ?`, selfdev.StateFailed, failedError, operationID); err != nil {
		t.Fatal(err)
	}
	out, err := verifyReduction(rt, assignment).commitVerifyIntent(ctx, intent)
	if err != nil || out["state"] != selfdev.StateFailed || out["decision"] != "fail" {
		t.Fatalf("replayed pass after the review failed it = %v, %v", out, err)
	}
	if judge.calls != 0 {
		t.Fatalf("the judge was called %d times", judge.calls)
	}
}

// Review r1 (deepseek B2): the review reads the bound bundle and asks the
// asset store by digest, and the stored shot reaches the model by its ref.
func TestReviewCandidateShotsReadsTheBoundBundleAndTheStore(t *testing.T) {
	ctx := context.Background()
	rt, productStore := testRuntime(t)
	computerID := "computer-shot-review"
	rt.cfg.ComputerID = computerID
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	rt.selfdevOperations = operations
	rt.selfdevUpdaterRoot = t.TempDir()
	_, screens, data := screenFixture(t)
	if ref, _, err := rt.imageStore().Put(data); err != nil || ref != "artifact:sha256:"+screens[0].SHA256 {
		t.Fatalf("store put = %s, %v", ref, err)
	}
	missing := capsule.ScreenFile{Name: "gone.png", SHA256: strings.Repeat("e", 64), Bytes: 10, Width: 390, Height: 844}
	bundle := transaction.CapsuleEffectBundle{
		BundleVersion: 1, ComputerID: computerID, BaseEventHead: strings.Repeat("a", 64),
		TrajectoryRef: "trajectory-op", CapsuleIdentity: "capsule-op", VerifierReceipts: []string{}, Screens: append(screens, missing),
	}
	if bundle.ContentDigest, err = bundle.ComputeContentDigest(); err != nil {
		t.Fatal(err)
	}
	raw, err := computerevent.CanonicalJSON(bundle)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(rt.selfdevUpdaterRoot, "incoming", bundle.ContentDigest)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bundle.draft.json"), raw, 0o400); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := productStore.DB().ExecContext(ctx, `INSERT INTO self_development_operations (operation_id,computer_id,idempotency_key,request_commitment,trajectory_id,capsule_id,base_head,prompt_artifact_ref,bundle_digest,verifier_refs_json,desired_head,effective_head,state,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,'[]',?,?,?,?,?)`,
		"op", computerID, "key-op", strings.Repeat("d", 64), "trajectory-op", "capsule-op", strings.Repeat("a", 64),
		"artifact:sha256:"+strings.Repeat("b", 64), bundle.ContentDigest, strings.Repeat("c", 64), strings.Repeat("c", 64), selfdev.StateFrozen, now, now); err != nil {
		t.Fatal(err)
	}
	operation, err := operations.Get(ctx, computerID, "op")
	if err != nil {
		t.Fatal(err)
	}
	judge := &scriptedJudge{answer: func(string) string { return allYes }}
	rt.shotReviewCall = judge.call
	review, err := rt.reviewCandidateShots(ctx, "owner", operation, "")
	if err != nil || review == nil {
		t.Fatalf("review = %v, %v", review, err)
	}
	if judge.calls != 1 || len(judge.refs[0]) != 1 || judge.refs[0][0] != "artifact:sha256:"+screens[0].SHA256 {
		t.Fatalf("judge saw %v, want only the stored shot by ref", judge.refs)
	}
	if review.Shots[0].Source != "model" || review.Shots[1].Source != "unresolvable" || review.Shots[1].Q5 != "yes" {
		t.Fatalf("shots = %+v", review.Shots)
	}
	if got := review.Shots[0].rejects(); len(got) != 1 || got[0] != "Q5" { // the fixture is 40x30
		t.Fatalf("stored shot rejects = %v", got)
	}
}

// FM9: crouton Library r5 (op selfdev-8d4532481b4aab23820dad0aeb264035) froze
// 6 of the 12 shots the request named and passed; its "phone" shot 11 was
// 1280x800. The request text below is the Library request's shot line.
const libraryShotLine = "Screenshots: scripts/choir-shot.mjs alone, `--size 1100x750` (11 and 12 at 390x844), `--contrast <selector>` for contrast. Take 01 alone first and look at it. Twelve PNGs in /var/lib/artifact/screens/: 01 aero-chrome-heart the package list; 02 london-salmon a search with results; 03 futuristic-noir an empty search; 04 carbon-fiber-kintsugi package detail with its release history; 05 frutiger-aero release detail (publisher, time, track and seq, changed paths, precommitments) with the gallery's larger view open on one screenshot; 06 vector-heart the \"Install <package> <release>?\" confirm; 07 liquid-metal the install outcome; 08 aero-chrome-heart the installed list with releases marked \"Installed\"; 09 london-salmon a refused install shown with its reason; 10 futuristic-noir the error state (catalog unreachable) with Try again; 11 390 aero-chrome-heart the list on the phone; 12 390 futuristic-noir release detail on the phone, for a release with no screenshots, showing \"No screenshots yet\".\nThe owner wrote this at 04:10Z: 10 more later."

func TestRequestedShotsParsesTheNumberedList(t *testing.T) {
	shots := requestedShots(libraryShotLine)
	if len(shots) != 12 {
		t.Fatalf("parsed %d shots: %+v", len(shots), shots)
	}
	if shots[0].Number != "01" || shots[0].Text != "aero-chrome-heart the package list" || shots[11].Number != "12" {
		t.Fatalf("ends = %+v, %+v", shots[0], shots[11])
	}
	for _, s := range shots {
		if want := s.Number == "11" || s.Number == "12"; s.Phone != want {
			t.Fatalf("shot %s phone = %v (%q)", s.Number, s.Phone, s.Text)
		}
	}
	for _, text := range []string{"", "Fix the 10 issues from 04:10Z", "01 only one; 02 two"} {
		if got := requestedShots(text); got != nil {
			t.Fatalf("%q parsed as a shot list: %+v", text, got)
		}
	}
}

func TestShotReviewFailsMissingRequestedShotsWithoutAModelCall(t *testing.T) {
	var screens []capsule.ScreenFile
	for _, n := range []string{"01-aero-chrome-heart.aero-chrome-heart.png", "02-london-salmon.london-salmon.png", "03-futuristic-noir.futuristic-noir.png", "04-carbon-fiber-kintsugi.carbon-fiber-kintsugi.png", "10-futuristic-noir.futuristic-noir.png", "11-aero-chrome-heart.aero-chrome-heart.png"} {
		screens = append(screens, capsule.ScreenFile{Name: n, SHA256: strings.Repeat("a", 64), Width: 1280, Height: 800})
	}
	judge := &scriptedJudge{answer: func(string) string { return allYes }}
	review, err := reviewShots(context.Background(), judge.call, screens, requestedShots(libraryShotLine), always(true))
	if err != nil {
		t.Fatal(err)
	}
	if judge.calls != 0 {
		t.Fatalf("the judge was called %d times for a bundle missing shots", judge.calls)
	}
	if !review.rejected() || strings.Join(review.Missing, ",") != "05,06,07,08,09,12" {
		t.Fatalf("missing = %v", review.Missing)
	}
	reason := review.reason("")
	for _, want := range []string{"missing shots the request names: 05, 06, 07, 08, 09, 12", "11-aero-chrome-heart.aero-chrome-heart.png Q5", "a phone shot at 1280x800"} {
		if !strings.Contains(reason, want) {
			t.Fatalf("reason %q misses %q", reason, want)
		}
	}
	// Only the phone shot is named among the present ones: the others were
	// not judged, so they carry no invented answers.
	if strings.Contains(reason, "01-aero") {
		t.Fatalf("an unreviewed shot was named: %q", reason)
	}
}

func TestShotReviewPhoneShotsMustBePhoneSizedAndCarryTheRequestDescription(t *testing.T) {
	var screens []capsule.ScreenFile
	for i := 1; i <= 12; i++ {
		s := capsule.ScreenFile{Name: fmt.Sprintf("%02d-shot.png", i), SHA256: strings.Repeat("b", 64), Width: 1100, Height: 750}
		if i >= 11 {
			s.Width, s.Height = 1170, 2532 // 390x844 at DPR 3
		}
		screens = append(screens, s)
	}
	// A shot whose own name says phone, at desktop size.
	screens[2] = capsule.ScreenFile{Name: "03-phone.png", SHA256: strings.Repeat("c", 64), Width: 1280, Height: 800}
	judge := &scriptedJudge{answer: func(string) string { return allYes }}
	review, err := reviewShots(context.Background(), judge.call, screens, requestedShots(libraryShotLine), always(true))
	if err != nil {
		t.Fatal(err)
	}
	if len(review.Missing) != 0 || judge.calls != 3 {
		t.Fatalf("missing %v, calls %d", review.Missing, judge.calls)
	}
	for _, a := range review.Shots {
		if got, want := len(a.rejects()) > 0, a.Name == "03-phone.png"; got != want {
			t.Fatalf("%s rejects %v (Q5 %s, %s)", a.Name, a.rejects(), a.Q5, a.Why)
		}
	}
	if !strings.Contains(strings.Join(judge.prompts, "\n"), "(the request describes it as: frutiger-aero release detail") {
		t.Fatal("the prompt does not carry the request's description of shot 05")
	}
}
