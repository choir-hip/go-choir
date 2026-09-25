package computerversion

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/base/model"
)

func TestProjectionMaterializerRejectsUnsupportedManifest(t *testing.T) {
	version := baseSliceComputerVersion()
	observations, err := BaseEventJournalObservationSet("base-event", version, []model.Event{baseCreateEvent(1, "a")})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	_, err = (ProjectionMaterializer{ID: "unsupported", Observations: observations}).Materialize(context.Background(), version, CapabilityManifest{
		Materializer: "unsupported",
		Substrate:    "projection",
		Unsupported: []UnsupportedCapability{{
			Kind:   ObservationFileManifest,
			Reason: "projection cannot expose file manifest",
		}},
	})
	if err == nil {
		t.Fatal("expected unsupported manifest to be rejected")
	}
}
