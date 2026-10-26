package yaegikernel

import (
	"strings"
	"testing"
)

// Verify carries the verdict's reason as data
// (docs/evidence/selfdev-trace-reviews-2026-10-25.md Round 8, R8-2). Ways it
// can fail, written first:
//   - the optional reason is dropped between choir.Verify in the cell and the
//     staged intent, so the reducer never sees it;
//   - the reason is a required argument, so every existing four-argument call
//     in a desk cell breaks;
//   - an unbounded reason rides the event and the operation row;
//   - a second reason argument is accepted and silently ignored.

func managementVerifyCell(t *testing.T, source string) (SessionResult, []StagedIntent) {
	t.Helper()
	broker, issuer, _, _ := testChoirFixture(t)
	scope, err := NewChoirScope(broker, issuer, "computer-choir", "activation-management", 1, "management", "")
	if err != nil {
		t.Fatal(err)
	}
	sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), scope.ChoirExports())
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	hooks := scope.BindCell()
	out, err := serveCell(sess, SessionFrame{ID: "cell", Source: source}, nil, &hooks)
	if err != nil {
		t.Fatalf("cell: %v", err)
	}
	return out, hooks.End()
}

func TestCellVerifyCarriesReasonAndStaysOptional(t *testing.T) {
	out, staged := managementVerifyCell(t, `fmt.Println(choir.Verify("a-1", "fail", []string{"r1"}, "sha256:abc", "the signed-in page was never opened"))`)
	if out.Error != "" || len(staged) != 1 || staged[0].Kind != IntentVerify {
		t.Fatalf("with a reason: error %q, staged %+v", out.Error, staged)
	}
	if staged[0].Reason != "the signed-in page was never opened" {
		t.Fatalf("staged reason = %q", staged[0].Reason)
	}

	out, staged = managementVerifyCell(t, `fmt.Println(choir.Verify("a-1", "pass", []string{"r1"}, "sha256:abc"))`)
	if out.Error != "" || len(staged) != 1 || staged[0].Kind != IntentVerify || staged[0].Decision != "pass" || staged[0].Reason != "" {
		t.Fatalf("without a reason: error %q, staged %+v", out.Error, staged)
	}
}

func TestVerifyReasonIsBoundedPlainText(t *testing.T) {
	var tray Tray
	long := strings.Repeat("é", MaxVerifyReasonChars+500)
	if err := tray.Verify("a-1", "fail", []string{"r"}, "sha256:abc", "  "+long+"\x00\x07  "); err != nil {
		t.Fatal(err)
	}
	reason := tray.Drain()[0].Reason
	if n := len([]rune(reason)); n > MaxVerifyReasonChars {
		t.Fatalf("reason holds %d characters, bound %d", n, MaxVerifyReasonChars)
	}
	if !strings.HasSuffix(reason, VerifyReasonTruncationMarker) {
		t.Fatalf("a truncated reason must end with %q, got tail %q", VerifyReasonTruncationMarker, reason[len(reason)-20:])
	}
	if strings.ContainsAny(reason, "\x00\x07") {
		t.Fatalf("control characters survived: %q", reason)
	}

	var short Tray
	if err := short.Verify("a-1", "fail", []string{"r"}, "sha256:abc", "  two\nlines  "); err != nil {
		t.Fatal(err)
	}
	if got := short.Drain()[0].Reason; got != "two\nlines" {
		t.Fatalf("short reason = %q, want it trimmed and otherwise unchanged", got)
	}

	var twice Tray
	if err := twice.Verify("a-1", "fail", []string{"r"}, "sha256:abc", "one", "two"); err == nil {
		t.Fatal("two reasons staged")
	}
}
