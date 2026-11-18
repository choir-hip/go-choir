package lease

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// The notebook grants (docs/reference/AUTORADIO-NOTEBOOK.md): Texture is the
// custodian, the Autoradio desk the only holder of both, and the two are
// separate grants.
func TestNotebookGrantsAreTheAutoradioDesksAlone(t *testing.T) {
	for _, g := range []string{GrantNotebookPatch, GrantNotebookRead} {
		if !GrantHeldBy(g, WriterAutoradio) {
			t.Errorf("%s: the Autoradio desk does not hold it", g)
		}
		for _, w := range []objectgraph.Writer{objectgraph.WriterTexture, objectgraph.WriterResearch, WriterResearchTexture, WriterMail, objectgraph.WriterOwner} {
			if GrantHeldBy(g, w) {
				t.Errorf("%s: %s holds it", g, w)
			}
		}
		if c, ok := grantCustodian(g); !ok || c != objectgraph.WriterTexture {
			t.Errorf("%s: custodian %v", g, c)
		}
	}
	if GrantNotebookPatch == GrantNotebookRead {
		t.Error("one grant for two powers")
	}
}
