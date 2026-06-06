package sourcecontract

import "strings"

const (
	OpenSurfaceSource  = "source"
	OpenSurfaceWebLens = "web_lens"
	OpenSurfaceVText   = "vtext"
	OpenSurfaceVideo   = "video"
	OpenSurfaceImage   = "image"
)

func NormalizeOpenSurface(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	switch normalized {
	case "":
		return ""
	case OpenSurfaceSource, "source_viewer", "source_reader", "reader", "content":
		return OpenSurfaceSource
	case OpenSurfaceWebLens, "weblens", "browser", "web", "live", "original", "live_original":
		return OpenSurfaceWebLens
	case OpenSurfaceVText, "published_vtext", "publication_version", "published_vtext_span":
		return OpenSurfaceVText
	case OpenSurfaceVideo, "youtube", "youtube_video":
		return OpenSurfaceVideo
	case OpenSurfaceImage:
		return OpenSurfaceImage
	default:
		return normalized
	}
}

func IsSourceReaderOpenSurface(value string) bool {
	return NormalizeOpenSurface(value) == OpenSurfaceSource
}

func IsLiveOpenSurface(value string) bool {
	return NormalizeOpenSurface(value) == OpenSurfaceWebLens
}
