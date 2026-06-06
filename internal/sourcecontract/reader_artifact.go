package sourcecontract

const (
	ReaderArtifactStateReady              = "reader_snapshot_ready"
	ReaderArtifactStateNotPublicationSafe = "not_publication_safe"
	ReaderArtifactStateBoundedExcerptOnly = "bounded_excerpt_only"
	ReaderArtifactStateImportFailed       = "import_failed"
)

func NormalizeReaderArtifactState(value string) string {
	return canonicalFromSchema(embeddedSourceContractSchema.ReaderArtifactStates, value)
}
