package agentcore

// Gap F (docs/problems/selfdev-reject-refused-after-accept-once-consumed-2026-10-24.md):
// a reject must stay available for every candidate that has not been approved,
// whatever mode transition produced the computer's current mode receipt.

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

func TestRejectStaysAvailableUnderAConsumptionReceiptOfAnotherOperation(t *testing.T) {
	const computerID = "computer-reject-receipt"
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "platform-test"}, PrivateKey: private}
	handler := &APIHandler{rt: &Runtime{selfdevControl: selfdev.GuestCredentialsWithPlatformKey("http://corpusd.invalid", computerID, "capability", time.Now().UTC().Add(time.Hour), "platform-test", public)}}

	modeReceipt := func(oldMode, newMode, consumedOperation, idempotencyKey, computer string) *computerevent.Receipt {
		t.Helper()
		receipt, err := computerevent.NewSignedReceipt("ModeReceipt", "corpusd", map[string]any{
			"computer_id": computer, "old_mode": oldMode, "new_mode": newMode,
			"consumed_operation_id": consumedOperation, "idempotency_key": idempotencyKey,
		}, []computerevent.SigningKey{key}, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		return &receipt
	}
	decide := func(receipt *computerevent.Receipt, operationID, decision string) error {
		return handler.verifyDecisionModeReceipt(computerID, operationID, "propose_only", receipt, selfDevelopmentDecisionRequest{Decision: decision, ModeReceipt: receipt})
	}
	consumed := func(prefix, operationID string) string { return prefix + operationID + ":" + strings.Repeat("d", 64) }

	cases := []struct {
		name      string
		receipt   *computerevent.Receipt
		operation string
		decision  string
		wantErr   bool
	}{
		{"1 reject Y after accept_once consumed X", modeReceipt("accept_once", "propose_only", "op-x", consumed(platform.AcceptOnceConsumptionKeyPrefix, "op-x"), computerID), "op-y", "reject", false},
		{"2 reject X under X's own accept_once consumption", modeReceipt("accept_once", "propose_only", "op-x", consumed(platform.AcceptOnceConsumptionKeyPrefix, "op-x"), computerID), "op-x", "reject", true},
		{"2b reject X under X's own qualified_consensus consumption", modeReceipt("qualified_consensus", "propose_only", "op-x", consumed(platform.QualifiedConsensusConsumptionKeyPrefix, "op-x"), computerID), "op-x", "reject", true},
		{"3 reject X under an owner disarm of X's arm", modeReceipt("accept_once", "propose_only", "op-x", "owner-disarm-1", computerID), "op-x", "reject", false},
		{"4 reject Y after qualified_consensus consumed X", modeReceipt("qualified_consensus", "propose_only", "op-x", consumed(platform.QualifiedConsensusConsumptionKeyPrefix, "op-x"), computerID), "op-y", "reject", false},
		{"5a receipt for another computer", modeReceipt("accept_once", "propose_only", "op-x", consumed(platform.AcceptOnceConsumptionKeyPrefix, "op-x"), "computer-other"), "op-y", "reject", true},
		{"5b unsigned receipt", &computerevent.Receipt{ReceiptKind: "ModeReceipt", ReceiptID: "unsigned"}, "op-y", "reject", true},
		{"6 an unknown decision", modeReceipt("propose_only", "propose_only", "", "k", computerID), "op-y", "defer", true},
		{"plain propose_only reject", modeReceipt("propose_only", "propose_only", "", "k", computerID), "op-y", "reject", false},
	}
	for _, tc := range cases {
		err := decide(tc.receipt, tc.operation, tc.decision)
		if tc.wantErr && err == nil {
			t.Errorf("%s: admitted, want refused", tc.name)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("%s: refused: %v", tc.name, err)
		}
	}
}
