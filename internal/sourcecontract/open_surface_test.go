package sourcecontract

import "testing"

func TestOpenSurfacePredicates(t *testing.T) {
	if !IsSourceReaderOpenSurface("content") || !IsSourceReaderOpenSurface("source-viewer") {
		t.Fatalf("source reader aliases should normalize to %q", OpenSurfaceSource)
	}
	if !IsLiveOpenSurface("browser") || !IsLiveOpenSurface("live-original") {
		t.Fatalf("live aliases should normalize to %q", OpenSurfaceWebLens)
	}
	if IsLiveOpenSurface("source") || IsSourceReaderOpenSurface("web_lens") {
		t.Fatalf("source and live aliases should stay distinct")
	}
}
