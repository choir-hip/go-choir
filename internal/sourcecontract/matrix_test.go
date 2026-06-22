package sourcecontract

import (
	"encoding/json"
	"os"
	"testing"
)

type sourceContractMatrix struct {
	EvidenceStates       []normalizerCase `json:"evidence_states"`
	ReaderArtifactStates []normalizerCase `json:"reader_artifact_states"`
	SourceKinds          []normalizerCase `json:"source_kinds"`
	SelectorKinds        []normalizerCase `json:"selector_kinds"`
	OpenSurfaces         []normalizerCase `json:"open_surfaces"`
}

type normalizerCase struct {
	Raw  string `json:"raw"`
	Want string `json:"want"`
}

func loadSourceContractMatrix(t *testing.T) sourceContractMatrix {
	t.Helper()
	raw, err := os.ReadFile("testdata/source_contract_matrix.json")
	if err != nil {
		t.Fatalf("read source contract matrix: %v", err)
	}
	var matrix sourceContractMatrix
	if err := json.Unmarshal(raw, &matrix); err != nil {
		t.Fatalf("decode source contract matrix: %v", err)
	}
	return matrix
}

func TestSharedSourceContractMatrix(t *testing.T) {
	matrix := loadSourceContractMatrix(t)
	for _, tc := range matrix.EvidenceStates {
		t.Run("evidence/"+tc.Raw, func(t *testing.T) {
			if got := NormalizeEvidenceState(tc.Raw); got != tc.Want {
				t.Fatalf("NormalizeEvidenceState(%q) = %q, want %q", tc.Raw, got, tc.Want)
			}
		})
	}
	for _, tc := range matrix.ReaderArtifactStates {
		t.Run("reader_artifact/"+tc.Raw, func(t *testing.T) {
			if got := NormalizeReaderArtifactState(tc.Raw); got != tc.Want {
				t.Fatalf("NormalizeReaderArtifactState(%q) = %q, want %q", tc.Raw, got, tc.Want)
			}
		})
	}
	for _, tc := range matrix.SourceKinds {
		t.Run("source/"+tc.Raw, func(t *testing.T) {
			if got := NormalizeSourceKind(tc.Raw); got != tc.Want {
				t.Fatalf("NormalizeSourceKind(%q) = %q, want %q", tc.Raw, got, tc.Want)
			}
		})
	}
	for _, tc := range matrix.SelectorKinds {
		t.Run("selector/"+tc.Raw, func(t *testing.T) {
			if got := NormalizeSelectorKind(tc.Raw); got != tc.Want {
				t.Fatalf("NormalizeSelectorKind(%q) = %q, want %q", tc.Raw, got, tc.Want)
			}
		})
	}
	for _, tc := range matrix.OpenSurfaces {
		t.Run("open_surface/"+tc.Raw, func(t *testing.T) {
			if got := NormalizeOpenSurface(tc.Raw); got != tc.Want {
				t.Fatalf("NormalizeOpenSurface(%q) = %q, want %q", tc.Raw, got, tc.Want)
			}
		})
	}
}
