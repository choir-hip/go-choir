package sources

import "testing"

func TestSourceAllowsReaderImport(t *testing.T) {
	for _, tc := range []struct {
		policy string
		want   bool
	}{
		{"bounded_text", true},
		{"bounded_release_text", true},
		{"bounded_abstract", true},
		{"excerpt_only", false},
		{"bounded_metadata", false},
		{"", false},
	} {
		if got := SourceAllowsReaderImport(tc.policy); got != tc.want {
			t.Fatalf("SourceAllowsReaderImport(%q) = %v, want %v", tc.policy, got, tc.want)
		}
	}
}
