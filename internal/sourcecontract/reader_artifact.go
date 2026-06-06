package sourcecontract

import "strings"

const (
	ReaderArtifactStateReady              = "reader_snapshot_ready"
	ReaderArtifactStateNotPublicationSafe = "not_publication_safe"
	ReaderArtifactStateBoundedExcerptOnly = "bounded_excerpt_only"
	ReaderArtifactStateImportFailed       = "import_failed"
)

func NormalizeReaderArtifactState(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	switch normalized {
	case ReaderArtifactStateReady, "ready", "snapshot_ready":
		return ReaderArtifactStateReady
	case ReaderArtifactStateNotPublicationSafe, "publication_blocked", "not_safe_for_publication":
		return ReaderArtifactStateNotPublicationSafe
	case ReaderArtifactStateBoundedExcerptOnly, "excerpt_only", "bounded_excerpt":
		return ReaderArtifactStateBoundedExcerptOnly
	case ReaderArtifactStateImportFailed, "failed", "fetch_failed", "source_import_failed":
		return ReaderArtifactStateImportFailed
	default:
		return ""
	}
}
