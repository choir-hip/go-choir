package capsule

import (
	"strings"
	"testing"
)

// A desk that retypes a receipt ref drops or swaps characters
// (drills-0565-request-unseen-and-receipt-typos-2026-10-20): the refusal names
// the ref it could not find and, when a receipt this executor issued shares a
// long prefix, names that one.
func TestUnknownReceiptRefusalNamesTheRefAndTheNearestIssued(t *testing.T) {
	real := "capsule-go-eval:sha256:0c1cb7ecb4b77cac69606e660abad57bbbbe7014a14a84a3ca9dcd761e53105b"
	typo := "capsule-go-eval:sha256:0c1cb7ecb4b77cac69606e660abad57bbbbe7014a84a3ca9dcd761e53105b"
	other := "capsule-go-eval:sha256:ffff0000b4b77cac69606e660abad57bbbbe7014a14a84a3ca9dcd761e53105b"
	err := unknownReceiptError(typo, []string{other, real})
	msg := err.Error()
	if !strings.Contains(msg, typo) || !strings.Contains(msg, "did you mean "+real) {
		t.Fatalf("refusal %q does not name the ref and the nearest issued one", msg)
	}
	if !strings.Contains(msg, "unavailable") {
		t.Fatalf("refusal %q lost the word callers match on", msg)
	}
	// No issued ref shares a long prefix: no guess.
	if msg := unknownReceiptError("capsule-go-eval:sha256:abcd", []string{other}).Error(); strings.Contains(msg, "did you mean") {
		t.Fatalf("guessed with no near match: %q", msg)
	}
	// A different receipt kind is never suggested.
	if msg := unknownReceiptError(typo, []string{"capsule-exec:sha256:0c1cb7ecb4b77cac69606e660abad57bbbbe7014a14a84a3ca9dcd761e53105b"}).Error(); strings.Contains(msg, "did you mean") {
		t.Fatalf("suggested another kind: %q", msg)
	}
}
