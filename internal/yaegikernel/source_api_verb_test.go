package yaegikernel

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"testing"
)

// choir.SourceAPI (docs/problems/keyed-source-apis-2026-10-20.md). Ways it
// can fail, written before the code:
//
//   - research's surface or handle lacks it, so the cell is told there is no
//     choir.SourceAPI;
//   - the call reaches the host as another action, or with the provider,
//     endpoint or params changed;
//   - a desk without world-read authority (Texture, management,
//     engineering) gains it.
func TestResearchSourceAPIReachesHostEgress(t *testing.T) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	issuer, err := NewHandleIssuer(secret)
	if err != nil {
		t.Fatal(err)
	}
	var gotAction BrokerAction
	var got SourceAPIPayload
	broker, err := NewBroker(BrokerConfig{ComputerID: "computer-choir", CurrentEpoch: 1, AllowedRoot: t.TempDir(),
		Egress: func(ctx context.Context, action BrokerAction, payload json.RawMessage) (json.RawMessage, error) {
			gotAction = action
			if err := json.Unmarshal(payload, &got); err != nil {
				t.Fatal(err)
			}
			return json.RawMessage(`{"provider":"fred","endpoint":"series","upstream_status":200,"body":{"ok":true}}`), nil
		}}, issuer)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := NewChoirScope(broker, issuer, "computer-choir", "activation-research", 1, "research", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := scope.ChoirExports()["choir/choir"]["SourceAPI"]; !ok {
		t.Fatal("research must export choir.SourceAPI")
	}
	out, err := scope.SourceAPI("fred", "series_observations", map[string]any{"series_id": "UNRATE", "limit": 3})
	if err != nil {
		t.Fatalf("SourceAPI: %v", err)
	}
	if gotAction != ActionSourceAPI || got.Provider != "fred" || got.Endpoint != "series_observations" ||
		got.Params["series_id"] != "UNRATE" || got.Params["limit"] != float64(3) {
		t.Fatalf("host saw %s %+v", gotAction, got)
	}
	if len(out) == 0 {
		t.Fatal("empty answer")
	}

	for _, desk := range []string{"texture", "management", "engineering"} {
		other, err := NewChoirScope(broker, issuer, "computer-choir", "activation-"+desk, 1, desk, "")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := other.ChoirExports()["choir/choir"]["SourceAPI"]; ok {
			t.Errorf("%s must not export choir.SourceAPI", desk)
		}
	}
}
