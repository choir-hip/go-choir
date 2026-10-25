package imageblob

// Pin on cite (docs/design/texture-evidence-embeds-te-2026-10-25.md section
// 4, visual evidence V35). There is no sweep yet; the pin is what the first
// one must honor. Ways it could fail, written before the code:
//   - a pin is lost or overwritten by a second holder of the same digest;
//   - pinning twice for one holder records two holds;
//   - a malformed digest or an empty holder becomes a path or a pin;
//   - a pin needs the bytes to be present (a capture whose store write
//     failed is still cited, and its bytes may arrive later);
//   - a store without a directory pretends to pin.

import (
	"strings"
	"testing"
)

func TestPinRecordsEachHolderOnce(t *testing.T) {
	s := New(t.TempDir())
	digest := strings.Repeat("a", 64)
	if pins, err := s.Pins(digest); err != nil || len(pins) != 0 {
		t.Fatalf("unpinned digest: pins %v, %v", pins, err)
	}
	for _, holder := range []string{"texture-revision:doc-1:rev-1", "texture-revision:doc-1:rev-2", "texture-revision:doc-1:rev-1"} {
		if err := s.Pin(digest, holder); err != nil {
			t.Fatalf("pin %s: %v", holder, err)
		}
	}
	pins, err := s.Pins(digest)
	if err != nil || len(pins) != 2 || pins[0] != "texture-revision:doc-1:rev-1" || pins[1] != "texture-revision:doc-1:rev-2" {
		t.Fatalf("pins = %v, %v; want the two holders, sorted, once each", pins, err)
	}
	if s.Has(digest) {
		t.Fatal("pinning stored bytes")
	}
	for _, bad := range []struct{ digest, holder string }{
		{"../" + strings.Repeat("a", 61), "h"}, {strings.ToUpper(digest), "h"}, {digest, " "}, {digest, "a\nb"},
	} {
		if err := s.Pin(bad.digest, bad.holder); err == nil {
			t.Fatalf("pin(%q, %q) accepted", bad.digest, bad.holder)
		}
	}
	if err := New("").Pin(digest, "h"); err == nil {
		t.Fatal("a store with no directory pinned")
	}
}
