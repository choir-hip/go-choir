package agentcore

import (
	"bytes"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func TestTransitionPrivateArtifactPlaceholderUsesReservedBindingIdentity(t *testing.T) {
	mutations := []computerevent.SupervisionMutation{{Kind: "super_belief_recorded", Body: []byte(`{"belief_artifact_ref":"$private:belief-payload"}`)}}
	if !replaceTransitionPrivateArtifactPlaceholder(mutations, "belief-payload") {
		t.Fatal("placeholder was not replaced")
	}
	want := []byte(computerevent.SupervisionArtifactPlaceholder("belief-payload"))
	if !bytes.Contains(mutations[0].Body, want) || bytes.Contains(mutations[0].Body, []byte("$private:")) {
		t.Fatalf("bound mutation body = %s", mutations[0].Body)
	}
}
