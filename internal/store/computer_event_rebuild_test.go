package store

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func TestRebuildComputerEventProjectionAtomicReplacement(t *testing.T) {
	ctx := context.Background()
	productStore := openTestStore(t)
	record := rebuildGenesisRecord(t, "computer-rebuild")

	if err := productStore.RebuildComputerEventProjection(ctx, []computerevent.DurableEvent{record}, &record.Request.Next); err != nil {
		t.Fatal(err)
	}
	head, err := productStore.Head(ctx, record.Request.Event.ComputerID)
	if err != nil || !rebuildSameHead(head, &record.Request.Next) {
		t.Fatalf("rebuilt head = %#v, %v", head, err)
	}
	var status, receiptDigest string
	if err := productStore.db.QueryRow(`SELECT status, event_head_receipt_digest FROM computer_event_index WHERE event_digest=?`, record.Request.EventDigest).Scan(&status, &receiptDigest); err != nil || status != "finalized" || receiptDigest == "" {
		t.Fatalf("rebuilt index = %q, %q, %v", status, receiptDigest, err)
	}

	// A non-projection object for the same computer must not be swept up by a
	// repair of the event projection.
	if _, err := productStore.db.Exec(`INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, FALSE)`, "choir.unrelated:rebuild", "choir.unrelated", "owner", record.Request.Event.ComputerID, "1", storeTestDigest('x'), `{}`, `{}`, time.Now().UTC(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := productStore.RebuildComputerEventProjection(ctx, []computerevent.DurableEvent{record}, &record.Request.Next); err != nil {
		t.Fatal(err)
	}
	var unrelated int
	if err := productStore.db.QueryRow(`SELECT COUNT(*) FROM og_objects WHERE canonical_id=?`, "choir.unrelated:rebuild").Scan(&unrelated); err != nil || unrelated != 1 {
		t.Fatalf("unrelated object count = %d, %v", unrelated, err)
	}
}

func TestRebuildComputerEventProjectionAcceptsExplicitEmptyHead(t *testing.T) {
	productStore := openTestStore(t)
	zero := &computerevent.Head{ComputerID: "computer-empty-rebuild"}
	now := time.Now().UTC()
	const frozenPlan = `{"encrypted_input_plan":"retained"}`
	if _, err := productStore.db.Exec(`INSERT INTO computer_supervision_commands (computer_id, command_id, command_digest, status, event_head_receipt_json, created_at, updated_at) VALUES (?, ?, ?, 'input_frozen', ?, ?, ?)`, zero.ComputerID, "command-pending", storeTestDigest('a'), frozenPlan, now, now); err != nil {
		t.Fatal(err)
	}
	if err := productStore.RebuildComputerEventProjection(context.Background(), nil, zero); err != nil {
		t.Fatalf("empty rebuild = %v", err)
	}
	head, err := productStore.Head(context.Background(), zero.ComputerID)
	if err != nil || head != nil {
		t.Fatalf("empty rebuild head = %#v, %v", head, err)
	}
	var status, rawPlan string
	if err := productStore.db.QueryRow(`SELECT status, event_head_receipt_json FROM computer_supervision_commands WHERE computer_id=? AND command_id=?`, zero.ComputerID, "command-pending").Scan(&status, &rawPlan); err != nil || status != "input_frozen" || rawPlan != frozenPlan {
		t.Fatalf("pending rebuild plan = status=%q plan=%q err=%v", status, rawPlan, err)
	}
}
func TestRebuildComputerEventProjectionRewindsPreparedSupervisionForExactRetry(t *testing.T) {
	ctx := context.Background()
	productStore := openTestStore(t)
	genesis := rebuildGenesisRecord(t, "computer-supervision")
	transaction := storeSupervisionOpenTransaction(t, genesis.Request.Next.CanonicalEventHead)
	envelope := []byte("encrypted-private-supervision-input")
	artifactDigest := computerevent.DigestBytes(envelope)
	artifactRef, err := computerevent.ArtifactRefFromDigest(artifactDigest)
	if err != nil {
		t.Fatal(err)
	}
	plaintextDigest := storeTestDigest('b')
	transaction.ReferencedArtifacts = []computerevent.ReferencedArtifact{{
		Ref: artifactRef.String(), ArtifactDigest: artifactDigest,
		PlaintextDigest: plaintextDigest, LogicalPlaintextDigest: plaintextDigest,
		MediaType: "application/json", BindingID: "command-open:input",
	}}
	var startBody map[string]any
	if err := json.Unmarshal(transaction.Mutations[0].Body, &startBody); err != nil {
		t.Fatal(err)
	}
	startBody["subject_refs"].(map[string]any)["input_artifact_ref"] = artifactRef.String()
	transaction.Mutations[0].Body, err = json.Marshal(startBody)
	if err != nil {
		t.Fatal(err)
	}
	transaction.CommandDigest, err = transaction.ComputeCommandDigest()
	if err != nil {
		t.Fatal(err)
	}
	inputPlan := computerevent.FrozenSupervisionPlan{
		Transaction: transaction,
		PrivateInputs: []computerevent.FrozenPrivateSupervisionInput{{
			BindingID: "command-open:input", MediaType: "application/json", Envelope: envelope,
			ArtifactDigest: artifactDigest, PlaintextDigest: plaintextDigest,
		}},
	}
	if _, _, finalized, err := productStore.ReserveFrozenSupervisionInputs(ctx, transaction.ComputerID, transaction.CommandID, transaction.CommandDigest, inputPlan); err != nil || finalized {
		t.Fatalf("reserve frozen inputs = finalized=%v err=%v", finalized, err)
	}

	finalTransaction := transaction
	finalTransaction.ReferencedArtifacts = append([]computerevent.ReferencedArtifact(nil), transaction.ReferencedArtifacts...)
	finalTransaction.ReferencedArtifacts[0].PinReceipt = computerevent.Receipt{ReceiptKind: "PinReceipt"}
	request := storeSupervisionRequest(t, finalTransaction, &genesis.Request.Next)
	frozenPlan := inputPlan
	frozenPlan.Transaction = *request.SupervisionTransaction
	frozenPlan.EventID = request.Event.EventID
	frozenPlan.OccurredAt = request.Event.OccurredAt
	frozenPlan.Envelope = []byte("encrypted-supervision-transaction")
	frozenPlan.ArtifactDigest = computerevent.DigestBytes(frozenPlan.Envelope)
	frozenPlan.ArtifactRef = "artifact:sha256:" + frozenPlan.ArtifactDigest
	frozenPlan.PinIntentCommitment = request.PinIntentCommitment
	if err := productStore.FreezeSupervisionPlan(ctx, transaction.ComputerID, transaction.CommandID, transaction.CommandDigest, frozenPlan); err != nil {
		t.Fatal(err)
	}
	if err := productStore.RecordSupervisionPin(ctx, transaction.ComputerID, transaction.CommandID, transaction.CommandDigest, computerevent.Receipt{ReceiptKind: "PinReceipt"}); err != nil {
		t.Fatal(err)
	}
	if err := productStore.Prepare(ctx, request); err != nil {
		t.Fatal(err)
	}

	if err := productStore.RebuildComputerEventProjection(ctx, []computerevent.DurableEvent{genesis}, &genesis.Request.Next); err != nil {
		t.Fatal(err)
	}
	recovered, found, err := productStore.FrozenSupervisionPlan(ctx, transaction.ComputerID, transaction.CommandID)
	if err != nil || !found || recovered.EventID != request.Event.EventID || recovered.PinReceipt == nil {
		t.Fatalf("recovered prepared plan = found=%v event=%q pin=%#v err=%v", found, recovered.EventID, recovered.PinReceipt, err)
	}
	var status string
	var eventDigest any
	if err := productStore.db.QueryRow(`SELECT status, event_digest FROM computer_supervision_commands WHERE computer_id=? AND command_id=?`, transaction.ComputerID, transaction.CommandID).Scan(&status, &eventDigest); err != nil || status != "pinned" || eventDigest != nil {
		t.Fatalf("rewound command = status=%q event_digest=%v err=%v", status, eventDigest, err)
	}
	if err := productStore.Prepare(ctx, request); err != nil {
		t.Fatalf("exact prepared retry = %v", err)
	}
	if err := productStore.db.QueryRow(`SELECT status FROM computer_supervision_commands WHERE computer_id=? AND command_id=?`, transaction.ComputerID, transaction.CommandID).Scan(&status); err != nil || status != "prepared" {
		t.Fatalf("retried command status = %q err=%v", status, err)
	}
}

func TestRebuildComputerEventProjectionReplaysSupervisionSnapshot(t *testing.T) {
	ctx := context.Background()
	productStore := openTestStore(t)
	genesis := rebuildGenesisRecord(t, "computer-supervision")
	transaction := storeSupervisionOpenTransaction(t, genesis.Request.Next.CanonicalEventHead)
	request := storeSupervisionRequest(t, transaction, &genesis.Request.Next)
	record := computerevent.DurableEvent{Request: request, Receipt: rebuildReceipt(t, request)}
	if err := productStore.RebuildComputerEventProjection(ctx, []computerevent.DurableEvent{genesis, record}, &request.Next); err != nil {
		t.Fatal(err)
	}
	snapshot, err := productStore.GetSupervisionProjectionSnapshot(ctx, transaction.OwnerID, transaction.ComputerID, transaction.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.LifecycleVersion != 1 || snapshot.CanonicalEventHead != request.EventDigest || snapshot.ArtifactHeadRevisionID != "revision-1" {
		t.Fatalf("rebuilt supervision snapshot = %#v", snapshot)
	}
	var status, eventDigest string
	if err := productStore.db.QueryRow(`SELECT status, event_digest FROM computer_supervision_commands WHERE computer_id=? AND command_id=?`, transaction.ComputerID, transaction.CommandID).Scan(&status, &eventDigest); err != nil || status != "finalized" || eventDigest != request.EventDigest {
		t.Fatalf("rebuilt command reservation = %q, %q, %v", status, eventDigest, err)
	}
}

func TestSupervisionProjectionSemanticDigestIsStableAcrossReplay(t *testing.T) {
	ctx := context.Background()
	productStore := openTestStore(t)
	genesis := rebuildGenesisRecord(t, "computer-supervision")
	transaction := storeSupervisionOpenTransaction(t, genesis.Request.Next.CanonicalEventHead)
	request := storeSupervisionRequest(t, transaction, &genesis.Request.Next)
	record := computerevent.DurableEvent{Request: request, Receipt: rebuildReceipt(t, request)}
	records := []computerevent.DurableEvent{genesis, record}
	if err := productStore.RebuildComputerEventProjection(ctx, records, &request.Next); err != nil {
		t.Fatal(err)
	}
	first, err := productStore.SupervisionProjectionSemanticDigest(ctx, transaction.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	if err := productStore.RebuildComputerEventProjection(ctx, records, &request.Next); err != nil {
		t.Fatal(err)
	}
	second, err := productStore.SupervisionProjectionSemanticDigest(ctx, transaction.ComputerID)
	if err != nil || first != second || !computerevent.IsSHA256(first) {
		t.Fatalf("semantic digest after replay = %q, %q, %v", first, second, err)
	}
}

func TestRebuildComputerEventProjectionRefusesBadTapeWithoutReplacement(t *testing.T) {
	ctx := context.Background()
	productStore := openTestStore(t)
	record := rebuildGenesisRecord(t, "computer-rebuild-refusal")
	if err := productStore.RebuildComputerEventProjection(ctx, []computerevent.DurableEvent{record}, &record.Request.Next); err != nil {
		t.Fatal(err)
	}

	badHead := record
	badHead.Request.Next.Sequence++
	corruptTransaction := record
	corruptTransaction.Request.SupervisionTransaction = &computerevent.SupervisionTransaction{}
	if err := productStore.RebuildComputerEventProjection(ctx, []computerevent.DurableEvent{corruptTransaction}, &record.Request.Next); err == nil {
		t.Fatal("corrupt supervision transaction was accepted")
	}
	if err := productStore.RebuildComputerEventProjection(ctx, []computerevent.DurableEvent{badHead}, &badHead.Request.Next); err == nil {
		t.Fatal("corrupt next head was accepted")
	}
	wrongExpected := record.Request.Next
	wrongExpected.CanonicalEventHead = storeTestDigest('z')
	if err := productStore.RebuildComputerEventProjection(ctx, []computerevent.DurableEvent{record}, &wrongExpected); err == nil {
		t.Fatal("wrong expected head was accepted")
	}
	head, err := productStore.Head(ctx, record.Request.Event.ComputerID)
	if err != nil || !rebuildSameHead(head, &record.Request.Next) {
		t.Fatalf("old head was replaced after refusal: %#v, %v", head, err)
	}
}

func TestRebuildComputerEventProjectionRollsBackBeforeCommit(t *testing.T) {
	ctx := context.Background()
	productStore := openTestStore(t)
	old := rebuildGenesisRecord(t, "computer-rebuild-rollback")
	if err := productStore.RebuildComputerEventProjection(ctx, []computerevent.DurableEvent{old}, &old.Request.Next); err != nil {
		t.Fatal(err)
	}

	productStore.rebuildComputerEventProjectionBeforeCommit = func() error { return errors.New("forced rollback") }
	t.Cleanup(func() { productStore.rebuildComputerEventProjectionBeforeCommit = nil })
	if err := productStore.RebuildComputerEventProjection(ctx, []computerevent.DurableEvent{old}, &old.Request.Next); err == nil {
		t.Fatal("injected pre-commit failure was accepted")
	}
	head, err := productStore.Head(ctx, old.Request.Event.ComputerID)
	if err != nil || !rebuildSameHead(head, &old.Request.Next) {
		t.Fatalf("old projection did not survive rollback: %#v, %v", head, err)
	}
	var count int
	if err := productStore.db.QueryRow(`SELECT COUNT(*) FROM computer_event_index WHERE computer_id=?`, old.Request.Event.ComputerID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("event rows after rollback = %d, %v", count, err)
	}
}

func TestRebuildComputerEventProjectionClearsExplicitZeroHead(t *testing.T) {
	ctx := context.Background()
	productStore := openTestStore(t)
	record := rebuildGenesisRecord(t, "computer-rebuild-zero")
	if err := productStore.RebuildComputerEventProjection(ctx, []computerevent.DurableEvent{record}, &record.Request.Next); err != nil {
		t.Fatal(err)
	}
	zero := &computerevent.Head{ComputerID: record.Request.Event.ComputerID}
	if err := productStore.RebuildComputerEventProjection(ctx, nil, zero); err != nil {
		t.Fatal(err)
	}
	head, err := productStore.Head(ctx, record.Request.Event.ComputerID)
	if err != nil || head != nil {
		t.Fatalf("zero rebuild head = %#v, %v", head, err)
	}
	var indexes, commands int
	if err := productStore.db.QueryRow(`SELECT COUNT(*) FROM computer_event_index WHERE computer_id=?`, record.Request.Event.ComputerID).Scan(&indexes); err != nil {
		t.Fatal(err)
	}
	if err := productStore.db.QueryRow(`SELECT COUNT(*) FROM computer_supervision_commands WHERE computer_id=?`, record.Request.Event.ComputerID).Scan(&commands); err != nil {
		t.Fatal(err)
	}
	if indexes != 0 || commands != 0 {
		t.Fatalf("zero rebuild rows = index:%d command:%d", indexes, commands)
	}
}

func rebuildGenesisRecord(t *testing.T, computerID string) computerevent.DurableEvent {
	t.Helper()
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	event := computerevent.Event{SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: computerID, Sequence: 1, PreviousHead: computerevent.ZeroHead, EventKind: computerevent.EventGenesisImported, OccurredAt: time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano), IdempotencyKey: "genesis-rebuild", ActorProfile: "trusted-core", AuthorityRef: "authority:test", PayloadCommitment: storeTestDigest('a'), PrivacyClass: "private", ReducerVersion: computerevent.ReducerVersionV1, ExpectedDesiredEventHead: computerevent.ZeroHead, ExpectedEffectiveEventHead: computerevent.ZeroHead, ExpectedDesiredStateCommitment: computerevent.ZeroHead, ExpectedEffectiveStateCommitment: computerevent.ZeroHead, ResultingEffectiveCommitment: storeTestDigest('b')}
	input := computerevent.TransitionInput{TargetStateCommitment: storeTestDigest('b')}
	pinIntent, err := computerevent.ComputePinIntentCommitment(event, input)
	if err != nil {
		t.Fatal(err)
	}
	event.RequestCommitment, err = computerevent.ComputeRequestCommitment(event, input, pinIntent, nil)
	if err != nil {
		t.Fatal(err)
	}
	next, err := computerevent.Reduce(nil, event, input)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := event.Digest()
	if err != nil {
		t.Fatal(err)
	}
	request := computerevent.CASRequest{Event: event, EventDigest: digest, EventArtifactDigest: digest, EventPinReceiptDigest: storeTestDigest('c'), PinIntentCommitment: pinIntent, Input: input, Next: next}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := computerevent.NewSignedReceipt("EventHeadReceipt", "corpusd", map[string]any{"computer_id": computerID, "event_digest": digest, "sequence": uint64(1)}, []computerevent.SigningKey{{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "test"}, PrivateKey: privateKey}}, time.Date(2026, 8, 4, 12, 0, 1, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return computerevent.DurableEvent{Request: request, Receipt: receipt}
}

func rebuildReceipt(t *testing.T, request computerevent.CASRequest) computerevent.Receipt {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := computerevent.NewSignedReceipt("EventHeadReceipt", "corpusd", map[string]any{
		"computer_id":  request.Event.ComputerID,
		"event_digest": request.EventDigest,
		"sequence":     request.Event.Sequence,
	}, []computerevent.SigningKey{{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "test"}, PrivateKey: privateKey}}, time.Date(2026, 8, 4, 12, 0, 1, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}
