package textureowner

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Pins the digest-conflict fix: the texture-delivery reconcile CommandID is
// content-derived, so a re-emitted reconcile whose item batch drifted across a
// boot gets a fresh command identity instead of colliding on the prior receipt.
// A same-slot reconcile re-presenting a different armed/exhausted set must not
// reuse the same CommandID — that collision is what produced the
// `lifecycle command digest conflict` startup-refusal crash loop.
// Receipt: docs/problems/m0-residual-texture-delivery-command-digest-crashloop-2026-09-30.md

func deliveryReq(targetRunID, breakerReason string, items ...types.ReconcileUpdateDeliveryItem) types.ReconcileUpdateDeliveryRequest {
	return types.ReconcileUpdateDeliveryRequest{
		TrajectoryID:  "trajectory-1",
		TargetAgentID: "texture:doc-1",
		TargetRunID:   targetRunID,
		MaxAttempts:   3,
		BreakerReason: breakerReason,
		Items:         items,
	}
}

func TestTextureDeliveryContentKeyDriftsWithItems(t *testing.T) {
	base := []types.ReconcileUpdateDeliveryItem{
		{UpdateID: "u-1", ProducerAgentID: "research:p", ProducerUpdateID: "pu-1", ExpectedLifecycleVersion: 1, ExpectedRunID: "run-1"},
		{UpdateID: "u-2", ProducerAgentID: "research:p", ProducerUpdateID: "pu-2", ExpectedLifecycleVersion: 1, ExpectedRunID: "run-1"},
	}
	drifted := []types.ReconcileUpdateDeliveryItem{
		{UpdateID: "u-1", ProducerAgentID: "research:p", ProducerUpdateID: "pu-1", ExpectedLifecycleVersion: 1, ExpectedRunID: "run-1"},
		{UpdateID: "u-2", ProducerAgentID: "research:p", ProducerUpdateID: "pu-2", ExpectedLifecycleVersion: 1, ExpectedRunID: "run-1"},
		{UpdateID: "u-3", ProducerAgentID: "research:p", ProducerUpdateID: "pu-3", ExpectedLifecycleVersion: 1, ExpectedRunID: "run-1"},
	}
	keyBase := textureDeliveryContentKey(deliveryReq("run-1", "", base...))
	keyDrifted := textureDeliveryContentKey(deliveryReq("run-1", "", drifted...))
	if keyBase == "" || keyDrifted == "" {
		t.Fatalf("content key empty: base=%q drifted=%q", keyBase, keyDrifted)
	}
	if keyBase == keyDrifted {
		t.Fatalf("drifted item set produced identical content key %q — reconcile would collide on the prior receipt", keyBase)
	}
}

func TestTextureDeliveryContentKeyStableAcrossItemOrder(t *testing.T) {
	a := types.ReconcileUpdateDeliveryItem{UpdateID: "u-1", ProducerAgentID: "research:p", ProducerUpdateID: "pu-1"}
	b := types.ReconcileUpdateDeliveryItem{UpdateID: "u-2", ProducerAgentID: "research:p", ProducerUpdateID: "pu-2"}
	keyAB := textureDeliveryContentKey(deliveryReq("run-1", "", a, b))
	keyBA := textureDeliveryContentKey(deliveryReq("run-1", "", b, a))
	if keyAB != keyBA {
		t.Fatalf("same logical batch in different order produced different keys (%q vs %q) — identical reconcile would mint a redundant command", keyAB, keyBA)
	}
}

func TestTextureDeliveryContentKeyDistinguishesSlotAndBreaker(t *testing.T) {
	items := []types.ReconcileUpdateDeliveryItem{
		{UpdateID: "u-1", ProducerAgentID: "research:p", ProducerUpdateID: "pu-1", ExpectedRunID: "run-1"},
	}
	// Same items, different bind target → different identity.
	if textureDeliveryContentKey(deliveryReq("run-1", "", items...)) == textureDeliveryContentKey(deliveryReq("run-2", "", items...)) {
		t.Fatal("different target run produced identical content key")
	}
	// Same items, breaker vs no-breaker → different identity.
	if textureDeliveryContentKey(deliveryReq("", "delivery attempts exceeded", items...)) == textureDeliveryContentKey(deliveryReq("", "", items...)) {
		t.Fatal("breaker state change produced identical content key")
	}
}
