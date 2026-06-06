package sourcecontract

import (
	"os"
	"strings"
	"testing"
)

func TestSourceContractSchemaMatchesGoConstants(t *testing.T) {
	assertSchemaHas := func(name string, entries map[string]sourceContractState, values ...string) {
		t.Helper()
		for _, value := range values {
			if _, ok := entries[value]; !ok {
				t.Fatalf("%s schema missing Go constant %q", name, value)
			}
		}
	}

	assertSchemaHas("evidence_states", embeddedSourceContractSchema.EvidenceStates,
		EvidenceStateCandidate,
		EvidenceStateAvailable,
		EvidenceStateConfirms,
		EvidenceStateRefutes,
		EvidenceStateQualifies,
		EvidenceStateNoSourceNeeded,
		EvidenceStateStale,
		EvidenceStateBlockedByAccess,
		EvidenceStateUnavailable,
	)
	assertSchemaHas("reader_artifact_states", embeddedSourceContractSchema.ReaderArtifactStates,
		ReaderArtifactStateReady,
		ReaderArtifactStateNotPublicationSafe,
		ReaderArtifactStateBoundedExcerptOnly,
		ReaderArtifactStateImportFailed,
	)
	assertSchemaHas("selector_kinds", embeddedSourceContractSchema.SelectorKinds,
		SelectorKindWholeResource,
		SelectorKindTextQuote,
		SelectorKindTextPosition,
		SelectorKindParagraphHeading,
		SelectorKindByteRange,
		SelectorKindPageRange,
		SelectorKindTimestampRange,
		SelectorKindTranscriptSegment,
		SelectorKindTableRange,
		SelectorKindTableCell,
		SelectorKindDataVintage,
		SelectorKindSelectorSet,
	)
	assertSchemaHas("open_surfaces", embeddedSourceContractSchema.OpenSurfaces,
		OpenSurfaceSource,
		OpenSurfaceWebLens,
		OpenSurfaceVText,
		OpenSurfaceVideo,
		OpenSurfaceImage,
	)
}

func TestGeneratedFrontendSourceContractUsesCurrentSchema(t *testing.T) {
	raw, err := os.ReadFile("../../frontend/src/lib/source-contract.generated.ts")
	if err != nil {
		t.Fatal(err)
	}
	want := "source-contract-schema-sha256: " + SourceContractSchemaHash()
	if !strings.Contains(string(raw), want) {
		t.Fatalf("generated frontend source contract is stale: missing %q", want)
	}
}
