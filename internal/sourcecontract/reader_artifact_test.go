package sourcecontract

import "testing"

func TestNormalizeReaderArtifactState(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want string
	}{
		{raw: ReaderArtifactStateReady, want: ReaderArtifactStateReady},
		{raw: "ready", want: ReaderArtifactStateReady},
		{raw: "snapshot-ready", want: ReaderArtifactStateReady},
		{raw: ReaderArtifactStateNotPublicationSafe, want: ReaderArtifactStateNotPublicationSafe},
		{raw: "publication blocked", want: ReaderArtifactStateNotPublicationSafe},
		{raw: "not_safe_for_publication", want: ReaderArtifactStateNotPublicationSafe},
		{raw: ReaderArtifactStateBoundedExcerptOnly, want: ReaderArtifactStateBoundedExcerptOnly},
		{raw: "excerpt_only", want: ReaderArtifactStateBoundedExcerptOnly},
		{raw: "bounded excerpt", want: ReaderArtifactStateBoundedExcerptOnly},
		{raw: ReaderArtifactStateImportFailed, want: ReaderArtifactStateImportFailed},
		{raw: "fetch-failed", want: ReaderArtifactStateImportFailed},
		{raw: "source_import_failed", want: ReaderArtifactStateImportFailed},
		{raw: "confirms", want: ""},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			if got := NormalizeReaderArtifactState(tc.raw); got != tc.want {
				t.Fatalf("NormalizeReaderArtifactState(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}
