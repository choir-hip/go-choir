package actorruntime

import (
	"os"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// F6: product behaviour is enforce. This package's fixtures start lifecycles
// and seed documents straight through the store with no declared writer, so
// the package runs its fixtures in observe, as it did before the flip. The
// product callers of StartLifecycle declare their writer and are exercised
// under enforce in internal/agentcore. TestMain has no t, so it cannot use
// gatefixture.Observe.
func TestMain(m *testing.M) {
	_ = objectgraph.SetModeForTest(objectgraph.WriterGateObserve)
	os.Exit(m.Run())
}
