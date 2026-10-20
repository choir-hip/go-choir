package proxy

import (
	"net/http"
	"testing"
)

// The release witness is a read of a joined projection (S6-0). Failure modes
// pinned: the classifier does not know the suffix and answers 404 before the
// guest sees it; the read is classed as a write scope; a write method reaches
// the guest as a read.
func TestSelfDevelopmentGuestClassifiesTheReleaseWitnessAsARead(t *testing.T) {
	const path = "/api/computers/computer-a/self-development/operations/op-1/release-witness"
	computerID, action, ok := selfDevelopmentGuestRoute(path)
	if !ok || computerID != "computer-a" || action != "read" {
		t.Fatalf("classifier = %q %q %v, want computer-a read", computerID, action, ok)
	}
	if !selfDevelopmentGuestMethodAllowed(http.MethodGet, action) {
		t.Fatal("GET refused for a read")
	}
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		if selfDevelopmentGuestMethodAllowed(method, action) {
			t.Fatalf("%s allowed for the witness read", method)
		}
	}
	// A suffix that merely resembles it is not a guest route.
	if _, _, ok := selfDevelopmentGuestRoute("/api/computers/computer-a/self-development/operations/op-1/release-witnesses"); ok {
		t.Fatal("a near-miss suffix was classified")
	}
}
