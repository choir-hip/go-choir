package sourcecontract

const (
	OpenSurfaceSource       = "source"
	OpenSurfaceWebLens      = "web_lens"
	OpenSurfaceTexture      = "texture"
	OpenSurfaceVideo        = "video"
	OpenSurfaceImage        = "image"
	OpenSurfaceAudio        = "audio"
	OpenSurfacePDF          = "pdf"
	OpenSurfaceTranscript   = "transcript"
	OpenSurfaceFile         = "file"
	OpenSurfaceSourceWindow = "source_window"
)

func NormalizeOpenSurface(value string) string {
	normalized := normalizeToken(value)
	if normalized == "" {
		return ""
	}
	if canonical := canonicalFromSchema(embeddedSourceContractSchema.OpenSurfaces, value); canonical != "" {
		return canonical
	}
	return normalized
}

func IsSourceReaderOpenSurface(value string) bool {
	return NormalizeOpenSurface(value) == OpenSurfaceSource
}

func IsLiveOpenSurface(value string) bool {
	return NormalizeOpenSurface(value) == OpenSurfaceWebLens
}
