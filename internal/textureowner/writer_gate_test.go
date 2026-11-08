package textureowner

import (
	"fmt"
	"os"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// MA3.2: every Texture write of this package declares its writer, the owner
// on the owner's routes and the custodian on the host's. The suite arms the
// undeclared-write oracle, so a desk or service kind written from this
// package's product code with no writer fails it.
func TestMain(m *testing.M) {
	// F6: product behaviour is enforce. This package's fixtures seed and read
	// through the store with no declared writer or reader, so the package runs
	// its fixtures in observe, as it did before the flip; the tests of the gates
	// themselves set enforce explicitly. TestMain has no t, so it cannot use
	// gatefixture.Observe.
	_ = objectgraph.SetModeForTest(objectgraph.WriterGateObserve)
	stop := objectgraph.TrackUndeclaredWrites()
	stopReads := objectgraph.TrackUndeclaredReads()
	code := m.Run()
	stop()
	stopReads()
	if undeclared := objectgraph.UndeclaredWrites(); len(undeclared) != 0 && code == 0 {
		fmt.Fprintf(os.Stderr, "FAIL: the writer gate saw writes that declare no writer: %v\n", undeclared)
		code = 1
	}
	// MA3.3: every read of the product declares its reader.
	if undeclared := objectgraph.UndeclaredReads(); len(undeclared) != 0 && code == 0 {
		fmt.Fprintf(os.Stderr, "FAIL: the reader gate saw reads that declare no reader: %v\n", undeclared)
		code = 1
	}
	os.Exit(code)
}

// requireNoUndeclaredWrites fails the test if the product code it drives wrote
// a desk or service kind with no declared writer. (The writer-gate counters are
// not asserted: the fixtures seed documents straight into the store.)
func requireNoUndeclaredWrites(t *testing.T) {
	t.Helper()
	before := objectgraph.UndeclaredWrites()
	t.Cleanup(func() {
		for key, n := range objectgraph.UndeclaredWrites() {
			if n > before[key] {
				t.Errorf("undeclared write: %s", key)
			}
		}
	})
}
