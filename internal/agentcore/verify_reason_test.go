package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// The verify's reason is data on the operation and the tape
// (docs/evidence/selfdev-trace-reviews-2026-10-25.md Round 8, R8-2): until
// now it survived only in management's prose report, so a trace review was
// needed to learn why a round failed. Ways it can fail, written first:
//   - a fail verdict with a reason leaves the operation's error as the fixed
//     string, so the dashboard and the trace export cannot say why;
//   - the reason is on the operation but not in the verification event;
//   - the fixed prefix callers match on changes;
//   - a verdict with no reason changes: a different error, or an empty
//     "reason" key on the event (old events must decode unchanged);
//   - an unbounded reason reaches the operation row;
//   - the reason enters the event's idempotency key, so a replay with the
//     same refs but different prose appends a second verification.

const failedJudgmentPrefix = "independent judgment rejected the frozen bundle"

// verificationPayloadOf is the payload of the one verification_recorded event
// on the fixture's tape.
func verificationPayloadOf(t *testing.T, f *p4Fixture) map[string]any {
	t.Helper()
	var found map[string]any
	for _, record := range f.cas.snapshot() {
		event := record.Request.Event
		if event.EventKind != computerevent.EventVerificationRecorded {
			continue
		}
		if found != nil {
			t.Fatal("more than one verification_recorded event")
		}
		ref, err := computerevent.ParseArtifactRef(event.OutputArtifactRefs[0])
		if err != nil {
			t.Fatal(err)
		}
		raw, ok := f.pinner.fetch(ref.Digest().String())
		if !ok {
			t.Fatalf("verification payload %s not pinned", ref.Digest())
		}
		found = map[string]any{}
		if err := json.Unmarshal(raw, &found); err != nil {
			t.Fatal(err)
		}
	}
	if found == nil {
		t.Fatal("no verification_recorded event")
	}
	return found
}

func failVerdict(t *testing.T, f *p4Fixture, key string, extra map[string]any) selfdev.Operation {
	t.Helper()
	frozen := f.freezeWithPatch(t, key, key, []byte("diff --git a/frontend/y b/frontend/y\n"))
	toolCtx := &CapsuleToolCtx{AgentRunID: "run:management-" + key, ComputerID: f.computerID, OwnerID: "owner",
		EventAppender: f.appender, OperationStore: f.operations, UpdaterRoot: f.updaterRoot, EventProjection: f.store}
	out, err := recordSelfDevelopmentVerdict(context.Background(), toolCtx, frozen.TrajectoryID, frozen.OperationID, frozen.BundleDigest, "fail",
		[]string{"capsule-go-eval:sha256:" + strings.Repeat("a", 64)}, extra)
	if err != nil || out["state"] != selfdev.StateFailed {
		t.Fatalf("fail verdict: %v, %v", out, err)
	}
	operation, err := f.operations.Get(context.Background(), f.computerID, frozen.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	return operation
}

func TestFailVerdictWithReasonIsOnTheEventAndTheOperationError(t *testing.T) {
	f := newBundleDurabilityFixture(t)
	const reason = "the bundle never opens the signed-in surface, so the claim is unproven"
	operation := failVerdict(t, f, "reason-fail", map[string]any{"verifier_desk": "management", "reason": reason})
	if want := failedJudgmentPrefix + ": " + reason; operation.TerminalError != want {
		t.Fatalf("operation error = %q, want %q", operation.TerminalError, want)
	}
	if got := verificationPayloadOf(t, f)["reason"]; got != reason {
		t.Fatalf("verification event reason = %v, want %q", got, reason)
	}
}

func TestFailVerdictWithoutReasonIsUnchanged(t *testing.T) {
	f := newBundleDurabilityFixture(t)
	operation := failVerdict(t, f, "reason-none", map[string]any{"verifier_desk": "management"})
	if operation.TerminalError != failedJudgmentPrefix {
		t.Fatalf("operation error = %q, want exactly %q", operation.TerminalError, failedJudgmentPrefix)
	}
	if _, present := verificationPayloadOf(t, f)["reason"]; present {
		t.Fatal("a verdict without a reason wrote a reason key to the event")
	}
}

func TestFailVerdictReasonIsBoundedOnTheOperation(t *testing.T) {
	f := newBundleDurabilityFixture(t)
	operation := failVerdict(t, f, "reason-long", map[string]any{"reason": strings.Repeat("x", 50000)})
	if !strings.HasPrefix(operation.TerminalError, failedJudgmentPrefix+": ") {
		t.Fatalf("operation error lost its prefix: %.80q", operation.TerminalError)
	}
	if n := len([]rune(operation.TerminalError)); n > len(failedJudgmentPrefix)+2+2000 {
		t.Fatalf("operation error holds %d characters, want the reason bounded at 2000", n)
	}
	if got, _ := verificationPayloadOf(t, f)["reason"].(string); len([]rune(got)) > 2000 {
		t.Fatalf("event reason holds %d characters", len([]rune(got)))
	}
}

// A reason with line breaks must not let the owner-visible operation error
// pose as extra status lines (review 286, LOW): the error collapses every run
// of whitespace to one space, while the event keeps the reason as normalized.
func TestFailVerdictReasonWhitespaceIsCollapsedOnTheOperationErrorOnly(t *testing.T) {
	f := newBundleDurabilityFixture(t)
	const reason = "first line\n\n\tsecond   line\nstate: approved"
	operation := failVerdict(t, f, "reason-lines", map[string]any{"reason": reason})
	if want := failedJudgmentPrefix + ": first line second line state: approved"; operation.TerminalError != want {
		t.Fatalf("operation error = %q, want %q", operation.TerminalError, want)
	}
	if got := verificationPayloadOf(t, f)["reason"]; got != reason {
		t.Fatalf("event reason = %q, want it kept as normalized: %q", got, reason)
	}
}
