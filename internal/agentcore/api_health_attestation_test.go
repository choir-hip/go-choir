package agentcore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/updater"
)

func TestHealthServesImmutableStartupReplayAttestation(t *testing.T) {
	rt, handler := testAPISetup(t)
	compatibility := updater.CurrentSupervisionCompatibility()
	expectedMedia := compatibility.PrivatePayloadMedia[0]
	attestation := StartupSupervisionReplayAttestation{
		Marker:                          "replayed-release",
		EventSchemaVersion:              1,
		ReducerVersion:                  1,
		ReleaseDigest:                   strings.Repeat("a", 64),
		Supervision:                     compatibility,
		SupervisionWritesDisabled:       true,
		PrivateTapeReplaySemanticDigest: strings.Repeat("b", 64),
		ProjectionSemanticDigest:        strings.Repeat("b", 64),
	}
	WithStartupSupervisionReplayAttestation(attestation)(rt)
	attestation.Supervision.PrivatePayloadMedia[0] = "mutated-after-boot"

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()
	handler.HandleHealth(response, request)
	var health runtimeHealthResponse
	if err := json.NewDecoder(response.Body).Decode(&health); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || health.Supervision == nil ||
		health.ReleaseDigest != strings.Repeat("a", 64) ||
		!health.SupervisionWritesDisabled ||
		health.PrivateTapeReplaySemanticDigest != health.ProjectionSemanticDigest ||
		health.Supervision.PrivatePayloadMedia[0] != expectedMedia {
		t.Fatalf("health replay attestation = %#v", health)
	}
}
