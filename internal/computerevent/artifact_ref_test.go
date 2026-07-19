package computerevent

import (
	"strings"
	"testing"
)

func TestArtifactRefCanonicalParsingAndLegacyProjection(t *testing.T) {
	digest := strings.Repeat("a", 64)
	ref, err := ArtifactRefFromDigest(digest)
	if err != nil || ref.String() != "artifact:sha256:"+digest || ref.Digest().String() != digest {
		t.Fatalf("artifact ref = %q digest=%q err=%v", ref.String(), ref.Digest().String(), err)
	}
	parsed, err := ParseArtifactRef(ref.String())
	if err != nil || parsed != ref {
		t.Fatalf("parsed artifact ref = %#v err=%v", parsed, err)
	}
	legacy, err := NormalizeArtifactRef(digest)
	if err != nil || legacy != ref {
		t.Fatalf("legacy artifact projection = %#v err=%v", legacy, err)
	}
	for _, invalid := range []string{"A" + digest[1:], "sha256:" + digest, "artifact:sha256:" + digest[:63]} {
		if _, err := NormalizeArtifactRef(invalid); err == nil {
			t.Fatalf("invalid artifact reference %q accepted", invalid)
		}
	}
}
