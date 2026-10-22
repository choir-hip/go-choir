package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// FP4 round 3, P2
// (docs/problems/fp4-r3-partial-advice-ends-in-system-cancel-2026-10-22.md).
// R3's refusal advised "partial or failed"; a partial report is not a
// terminal fate, so the completion guard retried and the system cancelled B
// with a reason naming the guard, not the open candidate. Ways the fix can
// fail: the refusal still offers "partial"; it advises failed but not with
// the refusal as the reason; the guard's cancel reason still omits the
// candidate when the model does not follow the advice; the guard names a
// candidate on a run whose freeze was never refused.

func TestFreezeRefusalAdvisesAFailedReport(t *testing.T) {
	ctx := context.Background()
	_, operations, computerID := freezeCandidateTestStore(t)
	authorized := func(context.Context) error { return nil }
	if _, err := openFreezeCandidate(ctx, operations, authorized, computerID, "trajectory-a", "engineering:assignment-a", "first app"); err != nil {
		t.Fatal(err)
	}
	_, err := openFreezeCandidate(ctx, operations, authorized, computerID, "trajectory-b", "engineering:assignment-b", "second app")
	if err == nil {
		t.Fatal("a second candidate opened while the first is open")
	}
	if strings.Contains(err.Error(), "partial") {
		t.Fatalf("the refusal still advises a partial report, which is not a terminal fate: %v", err)
	}
	for _, want := range []string{"failed", "this refusal as the reason"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the refusal does not say %q: %v", want, err)
		}
	}
}

func guardTestMessages(t *testing.T, toolResults ...string) []json.RawMessage {
	t.Helper()
	var out []json.RawMessage
	for _, content := range toolResults {
		msg, err := json.Marshal(map[string]any{
			"role":    "user",
			"content": []map[string]any{{"type": "tool_result", "tool_use_id": "call-1", "content": content}},
		})
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, msg)
	}
	return out
}

func TestCompletionGuardReasonNamesTheOpenCandidate(t *testing.T) {
	refusal := `{"error":"self-development candidate: assignment assignment-b cannot freeze until the open candidate settles (another self-development operation is open on this computer: selfdev-0593136f is awaiting_approval); report the work as failed"}`
	reason := engineeringTerminalFateGuardReason(guardTestMessages(t, `{"ok":true}`, refusal, `{"reported":"partial"}`))
	for _, want := range []string{"engineering_overlay_terminal_fate_required", "selfdev-0593136f", "awaiting_approval"} {
		if !strings.Contains(reason, want) {
			t.Fatalf("the guard's reason does not say %q: %q", want, reason)
		}
	}
	if plain := engineeringTerminalFateGuardReason(guardTestMessages(t, `{"ok":true}`)); plain != "engineering_overlay_terminal_fate_required" {
		t.Fatalf("a run whose freeze was never refused got reason %q", plain)
	}
}

// Round 3b (bunch 51, DeepSeek 5): the reason names the candidate and the
// state the refusal gave, and claims nothing about what that state waits
// for (an executing candidate is not waiting for management's Verify).
func TestCompletionGuardReasonIsStateNeutral(t *testing.T) {
	refusal := `cannot freeze until the open candidate settles (another self-development operation is open on this computer: selfdev-77aa is executing)`
	reason := engineeringTerminalFateGuardReason(guardTestMessages(t, refusal))
	for _, want := range []string{"selfdev-77aa", "executing"} {
		if !strings.Contains(reason, want) {
			t.Fatalf("the guard's reason does not say %q: %q", want, reason)
		}
	}
	for _, wrong := range []string{"Verify", "owner's decision"} {
		if strings.Contains(reason, wrong) {
			t.Fatalf("the guard's reason claims %q for an executing candidate: %q", wrong, reason)
		}
	}
}

// FP4 round 4, P3: a refusal before a later successful freeze no longer
// shapes the reason or the instruction. A cell whose freeze committed
// returns a top-level freeze_result with the operation id.
func TestCompletionGuardReasonIgnoresARefusalBeforeASuccessfulFreeze(t *testing.T) {
	refusal := `cannot freeze until the open candidate settles (another self-development operation is open on this computer: selfdev-77aa is awaiting_approval)`
	frozen := `{"output":"ok","freeze_result":{"operation_id":"selfdev-88bb","state":"frozen","bundle_digest":"sha256:x"}}`
	if reason := engineeringTerminalFateGuardReason(guardTestMessages(t, refusal, frozen)); reason != engineeringTerminalFateRequired {
		t.Fatalf("a refusal before a successful freeze still names a candidate: %q", reason)
	}
	if reason := engineeringTerminalFateGuardReason(guardTestMessages(t, frozen, refusal)); !strings.Contains(reason, "selfdev-77aa") {
		t.Fatalf("a refusal after a successful freeze is not named: %q", reason)
	}
	echoed := `{"output":"I saw \"freeze_result\" earlier","freeze_result":null}`
	if reason := engineeringTerminalFateGuardReason(guardTestMessages(t, refusal, echoed)); !strings.Contains(reason, "selfdev-77aa") {
		t.Fatalf("output text mentioning freeze_result reset the scan: %q", reason)
	}
}

// Review round 4b (Sonnet): a successful freeze cell resets the refusal
// even when its own output echoes the refusal text, and a freeze that got
// as far as a rejected bundle passed the open-candidate check too.
func TestCompletionGuardResetTakesPrecedenceInTheFreezeCell(t *testing.T) {
	refusal := `cannot freeze until the open candidate settles (another self-development operation is open on this computer: selfdev-77aa is awaiting_approval)`
	echo := `{"output":"earlier: cannot freeze until the open candidate settles (another self-development operation is open on this computer: selfdev-77aa is awaiting_approval)","freeze_result":{"operation_id":"selfdev-88bb","state":"frozen"}}`
	if reason := engineeringTerminalFateGuardReason(guardTestMessages(t, refusal, echo)); reason != engineeringTerminalFateRequired {
		t.Fatalf("a freeze cell that echoes the old refusal kept it: %q", reason)
	}
	rejected := `{"output":"","freeze_result":{"handle":"h","rejected":true,"reject_reason":"bundle too large"}}`
	if reason := engineeringTerminalFateGuardReason(guardTestMessages(t, refusal, rejected)); reason != engineeringTerminalFateRequired {
		t.Fatalf("a rejected freeze after the refusal kept the refusal: %q", reason)
	}
}

// Review round 4b (DeepSeek): a cell that freezes and then fails a later
// intent returns an error, not a result, so it carried no freeze_result and
// the stale refusal survived. The error now carries the committed freeze.
func TestCompletionGuardResetsOnAFreezeCommittedBeforeAFailedIntent(t *testing.T) {
	refusal := `cannot freeze until the open candidate settles (another self-development operation is open on this computer: selfdev-77aa is awaiting_approval)`
	failed := withCommittedFreeze(errors.New("reduce: persist tray-1: assignment command evidence unavailable after durable freeze: 5 cited refs resolved to 4 receipts"),
		map[string]any{"operation_id": "selfdev-88bb", "state": "frozen"})
	if failed == nil || !strings.Contains(failed.Error(), "selfdev-88bb") {
		t.Fatalf("the cell error does not name the committed freeze: %v", failed)
	}
	if reason := engineeringTerminalFateGuardReason(guardTestMessages(t, refusal, "tool_error: "+failed.Error())); reason != engineeringTerminalFateRequired {
		t.Fatalf("a freeze committed before a failed intent did not reset the refusal: %q", reason)
	}
	if withCommittedFreeze(nil, map[string]any{"operation_id": "x"}) != nil || withCommittedFreeze(errors.New("e"), nil).Error() != "e" {
		t.Fatal("withCommittedFreeze must leave nil errors and freeze-less cells alone")
	}
}
