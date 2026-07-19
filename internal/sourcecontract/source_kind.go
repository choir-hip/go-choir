package sourcecontract

import "sort"

const (
	SourceKindWebSource            = "web_source"
	SourceKindContentItem          = "content_item"
	SourceKindSourceServiceItem    = "source_service_item"
	SourceKindPublicationVersion   = "publication_version"
	SourceKindPublicationSpan      = "publication_span"
	SourceKindTextureSpan          = "texture_span"
	SourceKindReaderArtifact       = "reader_artifact"
	SourceKindSourceViewerArtifact = "source_viewer_artifact"
	SourceKindImage                = "image"
	SourceKindVideo                = "video"
	SourceKindYouTubeVideo         = "youtube_video"
	SourceKindAudio                = "audio"
	SourceKindPDF                  = "pdf"
	SourceKindTranscript           = "transcript"
	SourceKindFileArtifact         = "file_artifact"
	SourceKindCommandOutput        = "command_output"
	SourceKindShellSession         = "shell_session"
	SourceKindDiffHunk             = "diff_hunk"
	SourceKindPatch                = "patch"
	SourceKindTestRun              = "test_run"
	SourceKindCapsuleBundle        = "capsule_bundle"
	SourceKindScreenshot           = "screenshot"
	SourceKindVideoArtifact        = "video_artifact"
	SourceKindBenchmarkLog         = "benchmark_log"
)

func NormalizeSourceKind(value string) string {
	if canonical := canonicalFromSchema(embeddedSourceContractSchema.SourceKinds, value); canonical != "" {
		return canonical
	}
	return normalizeToken(value)
}

func IsSourceKind(value string) bool {
	normalized := NormalizeSourceKind(value)
	if normalized == "" {
		return false
	}
	_, ok := embeddedSourceContractSchema.SourceKinds[normalized]
	return ok
}

func SourceKindValues() []string {
	values := make([]string, 0, len(embeddedSourceContractSchema.SourceKinds))
	for value := range embeddedSourceContractSchema.SourceKinds {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}
