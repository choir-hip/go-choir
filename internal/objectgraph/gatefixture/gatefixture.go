// Package gatefixture is the test fixture for the three custody gates (the
// object-graph writer and reader gates and the disclosure gate). Product
// behaviour is enforce, with no setting (docs/design/enforce-flip-2026-11-08.md
// section 7.1). A test that needs the old observe behaviour calls Observe(t);
// the gates return to enforce when the test ends.
//
// Only _test.go files may import this package. scripts/check-gatefixture-imports
// refuses any other file, and the seams it wraps panic outside a test binary.
// Tests inside internal/objectgraph and internal/disclosure cannot import it
// (an import cycle) and call their package's SetModeForTest directly.
package gatefixture

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/disclosure"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Observe sets the object-graph writer and reader gates and the disclosure
// gate to observe for the test, and restores them on cleanup.
func Observe(t testing.TB) {
	t.Helper()
	t.Cleanup(objectgraph.SetModeForTest(objectgraph.WriterGateObserve))
	t.Cleanup(disclosure.SetModeForTest(disclosure.ModeObserve))
}
