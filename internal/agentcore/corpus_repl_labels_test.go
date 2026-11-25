package agentcore

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The corpus verbs taint a run as the reads they are: the text of captures as
// ReadCapture (the owner-private capture floor, which publish-auto admission
// keys on), the metadata rows as the world's untrusted text.
func TestCorpusVerbsLabelTheRunByWhatTheyReturn(t *testing.T) {
	texts, ok := egressReadLabels(yaegikernel.ActionCorpusTexts, nil, nil)
	if !ok || !texts.Has(types.OwnerPrivateLabel("capture")) || !texts.Has(types.LabelWorldUntrusted) {
		t.Fatalf("corpus texts labels %v", texts)
	}
	for _, a := range []yaegikernel.BrokerAction{yaegikernel.ActionCorpusCaptures, yaegikernel.ActionCorpusNear, yaegikernel.ActionCorpusEdges} {
		got, ok := egressReadLabels(a, nil, nil)
		if !ok || got.Has(types.OwnerPrivateLabel("capture")) || !got.Has(types.LabelWorldUntrusted) {
			t.Fatalf("%s labels %v", a, got)
		}
	}
}
