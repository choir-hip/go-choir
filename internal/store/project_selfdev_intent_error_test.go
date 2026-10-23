package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// An operation event written before last_intent_error existed carries no such
// key and still projects, with the column empty; a later state-preserving
// event carries the stamp and the clear, and the column follows the tape.
func TestProjectSelfDevelopmentOperationIntentErrorFollowsTheTape(t *testing.T) {
	ctx := context.Background()
	productStore := openProjectStore(t)
	computerID := "computer-intent-error-projection"
	prepareGenesis(t, productStore, computerID, "genesis-intent-error-projection")
	now := time.Date(2026, 10, 23, 7, 0, 0, 0, time.UTC)
	commitment := storeTestDigest('a')
	operation := computerevent.SelfDevelopmentOperationProjection{
		OperationID: "operation-intent-error", IdempotencyKey: "idem-intent-error", RequestCommitment: commitment,
		ComputerID: computerID, TrajectoryID: "trajectory-intent-error", BaseHead: commitment,
		PromptArtifactRef: "artifact:sha256:" + strings.Repeat("b", 64), VerifierRefsJSON: "[]",
		DesiredHead: commitment, EffectiveHead: commitment, State: selfdev.StateExecuting,
		CreatedAt: now.Format(time.RFC3339Nano), UpdatedAt: now.Format(time.RFC3339Nano),
	}
	// The pre-field spelling: marshal, then prove the key is absent.
	oldBody := mustJSON(t, operation)
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(oldBody, &keys); err != nil {
		t.Fatal(err)
	}
	if _, present := keys["last_intent_error"]; present {
		t.Fatal("an empty stamp is spelled in the event body; events written before the field would not match")
	}

	apply := func(name string, op computerevent.SelfDevelopmentOperationProjection) {
		t.Helper()
		head, err := productStore.Head(ctx, computerID)
		if err != nil || head == nil {
			t.Fatalf("%s: head: %v %#v", name, err, head)
		}
		batch := computerevent.ProjectionBatch{
			Version: computerevent.ProjectionBatchV2, ProjectorVersion: computerevent.ProjectorVersionV2, ComputerID: computerID,
			Ops: []computerevent.ProjectionOp{{Kind: computerevent.ProjectionOpSelfDevelopmentOperation, CanonicalID: op.OperationID, Body: mustJSON(t, op)}},
		}
		event, digest := prepareProjectionEvent(t, productStore, *head, mustEventID(t), name)
		batch.EventID, batch.EventDigest = event.EventID, digest
		if err := productStore.FinalizeBatch(ctx, computerID, digest, signedReceipt(t, computerID, digest, event.Sequence), &batch); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	stamp := func() (string, string) {
		t.Helper()
		var lastIntentError, state string
		if err := productStore.db.QueryRowContext(ctx, `SELECT last_intent_error, state FROM self_development_operations WHERE operation_id=?`, operation.OperationID).Scan(&lastIntentError, &state); err != nil {
			t.Fatal(err)
		}
		return lastIntentError, state
	}

	apply("intent-error-create", operation)
	if got, state := stamp(); got != "" || state != selfdev.StateExecuting {
		t.Fatalf("an event without the field projected last_intent_error=%q state=%q", got, state)
	}

	operation.ExpectedState = selfdev.StateExecuting
	operation.LastIntentError = "freeze: refused"
	operation.UpdatedAt = now.Add(time.Minute).Format(time.RFC3339Nano)
	apply("intent-error-stamp", operation)
	if got, state := stamp(); got != "freeze: refused" || state != selfdev.StateExecuting {
		t.Fatalf("a state-preserving stamp projected last_intent_error=%q state=%q", got, state)
	}

	operation.LastIntentError = ""
	operation.UpdatedAt = now.Add(2 * time.Minute).Format(time.RFC3339Nano)
	apply("intent-error-clear", operation)
	if got, _ := stamp(); got != "" {
		t.Fatalf("a clearing event left last_intent_error=%q", got)
	}
}
