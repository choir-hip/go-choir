package agentcore

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Enforce flip S1, section 6.7: the activation-end document read forces the
// document custodian as its reader, whatever desk the context carries.
func TestActivationEndDocumentReadIsTheCustodiansUnderAnotherDesksContext(t *testing.T) {
	rt, _, rec := idleTriggerTextureRun(t)
	rt.noteTextureCell(rec.RunID, nil)
	objectgraph.ResetReaderGateCounts()
	ctx := withDeskReader(context.Background(), agentprofile.Management)
	if err := rt.answerIdleTextureTrigger(ctx, &rec); err != nil {
		t.Fatal(err)
	}
	requireNoReaderCrossings(t, "the activation-end read under the management context")
}
