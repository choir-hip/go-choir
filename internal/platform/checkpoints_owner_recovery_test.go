package platform

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

func ownerRecoveryPlatformFixture(t *testing.T) (*CheckpointAuthority, selfdevprotocol.CheckpointRequest, string, string) {
	t.Helper()
	store, root := openTestPlatformStore(t)
	service := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	artifacts, err := NewEventArtifactService(service, platformTestKeyResolver{key: service.signingKey.Public})
	if err != nil {
		t.Fatal(err)
	}
	cas, err := NewComputerEventCAS(store, "corpusd", service.computerEventSigningKey(), artifacts)
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewCheckpointAuthority(cas, service)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	canonicalHead := platformTestDigest('1')
	effectiveHead := platformTestDigest('2')
	stateCommitment := platformTestDigest('3')
	if _, err := store.db.ExecContext(t.Context(), `INSERT INTO computer_event_heads (
		computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head,
		desired_state_commitment, effective_state_commitment, pending_transition_ref,
		reducer_version, credential_revocation_epoch, created_at, updated_at
	) VALUES (?, 12, ?, ?, ?, ?, ?, NULL, ?, 0, ?, ?)`,
		"computer-owner-recovery", canonicalHead, effectiveHead, effectiveHead,
		stateCommitment, stateCommitment, computerevent.ReducerVersionV1, now, now,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(t.Context(), `INSERT INTO computer_event_append_receipts (
		computer_id, idempotency_key, request_commitment, sequence, previous_head, event_kind, event_digest,
		event_artifact_ref, event_pin_receipt_digest, pin_receipt_digests_json, event_head_receipt_id,
		event_head_receipt_json, event_head_receipt_digest, desired_event_head, effective_event_head,
		desired_state_commitment, effective_state_commitment, pending_transition_ref, created_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		"computer-owner-recovery", "head-12", platformTestDigest('c'), 12, platformTestDigest('p'), "key_revoked", canonicalHead,
		"artifact://sha256/"+canonicalHead, platformTestDigest('d'), "[]", "receipt-head-12",
		`{"receipt_version":1,"receipt_kind":"EventHead","receipt_id":"receipt-head-12","issuer":"corpusd","issued_at":"2026-08-14T00:00:00Z","required_signers":[],"canonical_payload_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","signature_set":[]}`, platformTestDigest('e'), effectiveHead, effectiveHead, stateCommitment, stateCommitment, nil, now,
	); err != nil {
		t.Fatal(err)
	}
	request := selfdevprotocol.CheckpointRequest{
		ComputerID: "computer-owner-recovery", IdempotencyKey: "owner-recovery-" + canonicalHead,
		ComputerVersion: computerversion.ComputerVersion{
			CodeRef:            computerversion.CodeRef("code:sha256:" + platformTestDigest('d')),
			ArtifactProgramRef: computerversion.ArtifactProgramRef("artifact-program:sha256:" + platformTestDigest('e')),
		},
		AcceptedEventHead: canonicalHead, EffectiveEventHead: effectiveHead, EffectiveStateCommitment: stateCommitment,
		EventHeadReceiptID: "receipt-head-12", ReleaseDigest: platformTestDigest('f'), ReconstructionDigest: platformTestDigest('4'),
		ReducerVersion: computerevent.ReducerVersionV1, OwnerRecovery: true,
		VMLocalContentWitness: selfdevprotocol.VMLocalContentWitness{
			Database: "texture", ContentRoot: platformTestDigest('7'), DoltHead: platformTestDigest('8'), DerivabilityDigest: platformTestDigest('9'),
			Schema: map[string]string{"agents": platformTestDigest('a')},
			Tables: map[string]string{"agents": platformTestDigest('b')},
		},
		FrontendIdentity: selfdevprotocol.FrontendIdentity{Digest: platformTestDigest('c'), Derivation: selfdevprotocol.FrontendDerivationRelease, ReleaseDigest: platformTestDigest('f')},
	}
	return authority, request, root, canonicalHead
}

func TestOwnerRecoveryCheckpointPublishesWithoutVerifierEvidence(t *testing.T) {
	authority, request, _, _ := ownerRecoveryPlatformFixture(t)
	response, err := authority.Publish(t.Context(), request)
	if err != nil {
		t.Fatalf("owner-recovery publish refused: %v", err)
	}
	if response.Receipt.Kind != selfdevprotocol.ReceiptKindCheckpoint || response.Receipt.ArtifactDigest != response.Checkpoint.Digest {
		t.Fatalf("owner-recovery receipt = %#v", response.Receipt)
	}
	// idempotent replay
	replay, err := authority.Publish(t.Context(), request)
	if err != nil || replay.Checkpoint.Digest != response.Checkpoint.Digest {
		t.Fatalf("owner-recovery replay: %v", err)
	}
}

func TestOwnerRecoveryCheckpointRefusesVerifierEvidence(t *testing.T) {
	authority, request, _, _ := ownerRecoveryPlatformFixture(t)
	request.MaterializationReceiptDigest = platformTestDigest('m')
	if _, err := authority.Publish(t.Context(), request); err == nil {
		t.Fatal("owner-recovery publish accepted verifier evidence")
	}
}

func TestOwnerRecoveryCheckpointRequiresHeadReceiptBinding(t *testing.T) {
	authority, request, _, _ := ownerRecoveryPlatformFixture(t)
	if _, err := authority.cas.store.db.ExecContext(t.Context(), `UPDATE computer_event_append_receipts SET event_digest=? WHERE computer_id=? AND event_head_receipt_id=?`, platformTestDigest('9'), request.ComputerID, request.EventHeadReceiptID); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.Publish(t.Context(), request); err == nil {
		t.Fatal("owner-recovery publish accepted a head the receipt does not bind")
	}
}

func TestOwnerRecoveryCheckpointRefusesStaleHead(t *testing.T) {
	authority, request, _, _ := ownerRecoveryPlatformFixture(t)
	request.AcceptedEventHead = strings.Repeat("e", 64)
	if _, err := authority.Publish(t.Context(), request); err == nil {
		t.Fatal("owner-recovery publish accepted a stale head")
	}
}
