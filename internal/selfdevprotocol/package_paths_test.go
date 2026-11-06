package selfdevprotocol

import (
	"strings"
	"testing"
)

// Failure modes: a bundle whose effects name a package path with or without
// the leading slash is admitted; a path outside /workspace/platform that
// happens to contain frontend/src/packages is refused; an undecodable bundle
// is admitted.
func TestRefuseBundlePackagePaths(t *testing.T) {
	for name, tc := range map[string]struct {
		bundle string
		refuse bool
	}{
		"platform":        {`{"ordered_file_effects":[{"path":"/workspace/platform/internal/x.go"}]}`, false},
		"package":         {`{"ordered_file_effects":[{"path":"/workspace/platform/frontend/src/packages/image/App.svelte"}]}`, true},
		"package noslash": {`{"ordered_file_effects":[{"path":"workspace/platform/frontend/src/packages/image/a.js"}]}`, true},
		"outside":         {`{"ordered_file_effects":[{"path":"/tmp/frontend/src/packages/image/a.js"}]}`, false},
		"garbage":         {`not json`, true},
	} {
		got := RefuseBundlePackagePaths([]byte(tc.bundle))
		if (got != "") != tc.refuse || (tc.refuse && !strings.HasPrefix(got, "package_path_refused")) {
			t.Errorf("%s: %q", name, got)
		}
	}
}
