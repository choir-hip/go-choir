package actorruntime

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Enforce flip S1, section 6.7: the legacy Texture owner-wake fallback resolves
// the document and the revision on the dispatcher's context, which declares no
// reader. Both reads are the custodian's.
func TestLegacyOwnerWakeFallbackReadsAsTheCustodian(t *testing.T) {
	env := newAdapterTestEnv(t)
	const ownerID, docID = "user-legacy-reader", "doc-legacy-reader"
	seedDurableTextureUpdate(t, env.store, env.ctx, "autoputer-test", ownerID, docID, "update-legacy-reader", "Durable Texture wake")
	doc, err := env.store.GetLifecycleDocument(objectgraph.WithCustodianReader(env.ctx, "choir.texture_document"), ownerID, "autoputer-test", docID)
	if err != nil {
		t.Fatal(err)
	}
	objectgraph.ResetReaderGateCounts()
	// The pre-repair owner wake carried only the revision id.
	_, _, _, _ = env.adapter.canonicalTextureDispatch(env.ctx, ownerID, "autoputer-test", "texture:"+docID, "coagent_result", doc.CurrentRevisionID, "", "")
	for key, n := range objectgraph.ReaderGateCounts() {
		if strings.Contains(key, "texture_") {
			t.Errorf("the fallback read was counted: %s x%d", key, n)
		}
	}
}
