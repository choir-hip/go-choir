package sourcecontract

import "testing"

func TestNormalizeSourceKind(t *testing.T) {
	for _, tc := range []struct {
		name  string
		raw   string
		want  string
		valid bool
	}{
		{name: "web page alias", raw: "web-page", want: SourceKindWebSource, valid: true},
		{name: "content item space", raw: "content item", want: SourceKindContentItem, valid: true},
		{name: "source item alias", raw: "source-item", want: SourceKindSourceServiceItem, valid: true},
		{name: "command output alias", raw: "cmd output", want: SourceKindCommandOutput, valid: true},
		{name: "test run hyphen", raw: "test-run", want: SourceKindTestRun, valid: true},
		{name: "unknown normalized", raw: "custom source", want: "custom_source", valid: false},
		{name: "empty", raw: "", want: "", valid: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeSourceKind(tc.raw); got != tc.want {
				t.Fatalf("NormalizeSourceKind(%q) = %q, want %q", tc.raw, got, tc.want)
			}
			if got := IsSourceKind(tc.raw); got != tc.valid {
				t.Fatalf("IsSourceKind(%q) = %v, want %v", tc.raw, got, tc.valid)
			}
		})
	}
}

func TestSourceKindValues(t *testing.T) {
	values := SourceKindValues()
	if len(values) != len(embeddedSourceContractSchema.SourceKinds) {
		t.Fatalf("SourceKindValues length = %d, want %d", len(values), len(embeddedSourceContractSchema.SourceKinds))
	}
	if !containsSourceContractValue(values, SourceKindContentItem) || !containsSourceContractValue(values, SourceKindTestRun) {
		t.Fatalf("SourceKindValues missing expected source kinds: %#v", values)
	}
}

func containsSourceContractValue(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
