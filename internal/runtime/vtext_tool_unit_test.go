package runtime

import "testing"

func TestCleanVTextToolContentRemovesWrapperTags(t *testing.T) {
	input := " <payload>\nStaging smoke after RSS title extraction works.\n</payload> "
	if got := cleanVTextToolContent(input); got != "Staging smoke after RSS title extraction works." {
		t.Fatalf("cleanVTextToolContent() = %q", got)
	}
}

func TestCleanVTextToolContentRemovesDanglingClosingMarker(t *testing.T) {
	for _, input := range []string{
		"VText wrapper cleanup works.</\n",
		"VText wrapper cleanup works.</妮>",
	} {
		if got := cleanVTextToolContent(input); got != "VText wrapper cleanup works." {
			t.Fatalf("cleanVTextToolContent(%q) = %q", input, got)
		}
	}
}

func TestCleanVTextToolContentPreservesOrdinaryText(t *testing.T) {
	input := "The paragraph mentions <payload> as literal text inside the body."
	if got := cleanVTextToolContent(input); got != input {
		t.Fatalf("cleanVTextToolContent() = %q, want original", got)
	}
}
