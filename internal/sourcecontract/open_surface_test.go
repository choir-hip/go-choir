package sourcecontract

import "testing"

func TestNormalizeOpenSurface(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want string
	}{
		{raw: "", want: ""},
		{raw: "source", want: OpenSurfaceSource},
		{raw: "content", want: OpenSurfaceSource},
		{raw: "source-viewer", want: OpenSurfaceSource},
		{raw: "reader", want: OpenSurfaceSource},
		{raw: "web-lens", want: OpenSurfaceWebLens},
		{raw: "weblens", want: OpenSurfaceWebLens},
		{raw: "browser", want: OpenSurfaceWebLens},
		{raw: "live original", want: OpenSurfaceWebLens},
		{raw: "publication-version", want: OpenSurfaceTexture},
		{raw: "published-texture-span", want: OpenSurfaceTexture},
		{raw: "published-texture-span", want: OpenSurfaceTexture},
		{raw: "youtube_video", want: OpenSurfaceVideo},
		{raw: "image", want: OpenSurfaceImage},
		{raw: "custom-app", want: "custom_app"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			if got := NormalizeOpenSurface(tc.raw); got != tc.want {
				t.Fatalf("NormalizeOpenSurface(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

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
