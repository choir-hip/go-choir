package platform

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func TestEventArtifactServiceEventsPageBoundsAndEmptyResult(t *testing.T) {
	platformStore, root := openTestPlatformStore(t)
	service := NewService(platformStore, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	artifacts, err := NewEventArtifactService(service, platformTestKeyResolver{key: service.signingKey.Public})
	if err != nil {
		t.Fatal(err)
	}

	records, err := artifacts.EventsPage(context.Background(), "computer-replay-page", 0, 1)
	if err != nil {
		t.Fatalf("empty replay page: %v", err)
	}
	if records == nil {
		t.Fatal("empty replay page returned nil records")
	}
	if len(records) != 0 {
		t.Fatalf("empty replay page records = %d, want 0", len(records))
	}
	for _, pageSize := range []int{0, computerevent.EventReplayMaxPageSize + 1} {
		if _, err := artifacts.EventsPage(context.Background(), "computer-replay-page", 0, pageSize); err == nil {
			t.Fatalf("invalid replay page size %d was accepted", pageSize)
		}
	}
}

func TestHandleComputerEventReplayValidatesPageLimit(t *testing.T) {
	platformStore, root := openTestPlatformStore(t)
	service := NewService(platformStore, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	artifacts, err := NewEventArtifactService(service, platformTestKeyResolver{key: service.signingKey.Public})
	if err != nil {
		t.Fatal(err)
	}
	cas, err := NewComputerEventCAS(platformStore, "corpusd", service.computerEventSigningKey(), artifacts)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	token, err := MintComputerCapability(ComputerCapability{
		Version: 1, ComputerID: "computer-replay-handler", Scopes: []string{"event:read"},
		ExpiresAt: now.Add(4 * time.Minute).Format(time.RFC3339Nano), Nonce: "replay-handler-test",
	}, service.signingKey.Private)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(service)
	if err := handler.ConfigureComputerEvents(cas, artifacts, SignedCapabilityVerifier{
		Store: platformStore, PublicKey: service.signingKey.Public, Now: func() time.Time { return now },
	}); err != nil {
		t.Fatal(err)
	}
	request := func(limit string) *httptest.ResponseRecorder {
		query := url.Values{"computer_id": []string{"computer-replay-handler"}, "after_sequence": []string{"0"}}
		if limit != "" {
			query.Set("limit", limit)
		}
		req := httptest.NewRequest(http.MethodGet, "/internal/computers/events/replay?"+query.Encode(), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		handler.HandleComputerEventReplay(response, req)
		return response
	}
	if response := request("1"); response.Code != http.StatusOK {
		t.Fatalf("valid replay page status = %d, body = %s", response.Code, response.Body.String())
	}
	if response := request(strconv.Itoa(computerevent.EventReplayMaxPageSize + 1)); response.Code != http.StatusBadRequest {
		t.Fatalf("oversized replay page status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

// insertReplayTestChain writes receipts (and their event artifacts) for the
// given sequences, in the given order, and a head at headSequence.
func insertReplayTestChain(t *testing.T, platformStore *Store, service *Service, computerID string, sequences []uint64, headSequence uint64) {
	t.Helper()
	now := time.Date(2026, 10, 9, 2, 0, 0, 0, time.UTC)
	_, receiptKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := computerevent.NewSignedReceipt("event_head", "corpusd", map[string]any{}, []computerevent.SigningKey{{
		SignerRef: computerevent.SignerRef{SignerDomain: "platform", KeyID: "replay-test"}, PrivateKey: receiptKey,
	}}, now)
	if err != nil {
		t.Fatal(err)
	}
	receiptJSON, err := receipt.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	receiptDigest := computerevent.DigestBytes(receiptJSON)
	for _, sequence := range sequences {
		eventID, err := computerevent.NewEventID()
		if err != nil {
			t.Fatal(err)
		}
		event := computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: computerID,
			Sequence: sequence, PreviousHead: platformTestDigest('b'), EventKind: computerevent.EventVerificationRecorded,
			OccurredAt: now.Format(time.RFC3339Nano), IdempotencyKey: fmt.Sprintf("replay-%d", sequence), RequestCommitment: platformTestDigest('c'),
			TrajectoryID: "trajectory-replay", CapsuleID: "capsule-replay", ActorProfile: "engineering",
			AuthorityRef: "guest-core:self-development-verifier", OutputArtifactRefs: []string{"artifact:sha256:" + platformTestDigest('a')},
			PayloadCommitment: platformTestDigest('a'), PrivacyClass: "public", ReducerVersion: computerevent.ReducerVersionV1,
			ExpectedDesiredEventHead: platformTestDigest('b'), ExpectedEffectiveEventHead: platformTestDigest('b'),
			ExpectedDesiredStateCommitment: platformTestDigest('f'), ExpectedEffectiveStateCommitment: platformTestDigest('f'),
		}
		rawEvent, err := event.CanonicalBytes()
		if err != nil {
			t.Fatal(err)
		}
		eventDigest := computerevent.DigestBytes(rawEvent)
		if err := service.writeBlob(filepath.Join("sha256", "computer-event", eventDigest), rawEvent); err != nil {
			t.Fatal(err)
		}
		if _, err := platformStore.db.Exec(`INSERT INTO computer_event_append_receipts (computer_id,idempotency_key,request_commitment,sequence,previous_head,event_kind,event_digest,event_artifact_ref,event_pin_receipt_digest,pin_receipt_digests_json,event_head_receipt_id,event_head_receipt_json,event_head_receipt_digest,desired_event_head,effective_event_head,desired_state_commitment,effective_state_commitment,pending_transition_ref,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			computerID, event.IdempotencyKey, platformTestDigest('c'), sequence, event.PreviousHead, string(event.EventKind), eventDigest, eventDigest, platformTestDigest('d'), "[]", fmt.Sprintf("receipt-%d", sequence), string(receiptJSON), receiptDigest, eventDigest, event.PreviousHead, platformTestDigest('f'), platformTestDigest('f'), nil, now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := platformStore.db.Exec(`INSERT INTO computer_event_heads (computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, desired_state_commitment, effective_state_commitment, pending_transition_ref, reducer_version, credential_revocation_epoch, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		computerID, headSequence, platformTestDigest('1'), platformTestDigest('1'), platformTestDigest('1'), platformTestDigest('f'), platformTestDigest('f'), nil, computerevent.ReducerVersionV1, 0, now, now); err != nil {
		t.Fatal(err)
	}
}

func replayPageSequences(records []computerevent.DurableEvent) []uint64 {
	sequences := []uint64{}
	for _, record := range records {
		sequences = append(sequences, record.Request.Event.Sequence)
	}
	return sequences
}

// The replay page is served by sequence point lookups bounded by the head
// (docs/problems/replay-page-query-scans-whole-chain-2026-10-09.md). Failure
// modes: wrong or reordered rows, a short final page, a nonempty page at or
// past the head, and a gap below the head silently read as end of chain.
func TestEventsPageWalksGaplessChainToHead(t *testing.T) {
	platformStore, root := openTestPlatformStore(t)
	service := NewService(platformStore, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	artifacts, err := NewEventArtifactService(service, platformTestKeyResolver{key: service.signingKey.Public})
	if err != nil {
		t.Fatal(err)
	}
	insertReplayTestChain(t, platformStore, service, "computer-replay-chain", []uint64{5, 3, 1, 4, 2}, 5)
	for _, tc := range []struct {
		after    uint64
		pageSize int
		want     []uint64
	}{
		{0, 2, []uint64{1, 2}},
		{2, 2, []uint64{3, 4}},
		{4, 2, []uint64{5}},
		{0, 1024, []uint64{1, 2, 3, 4, 5}},
		{5, 2, []uint64{}},
		{9, 2, []uint64{}},
	} {
		records, err := artifacts.EventsPage(context.Background(), "computer-replay-chain", tc.after, tc.pageSize)
		if err != nil {
			t.Fatalf("page after=%d size=%d: %v", tc.after, tc.pageSize, err)
		}
		if got := replayPageSequences(records); !slices.Equal(got, tc.want) {
			t.Fatalf("page after=%d size=%d = %v, want %v", tc.after, tc.pageSize, got, tc.want)
		}
	}

	insertReplayTestChain(t, platformStore, service, "computer-replay-gap", []uint64{1, 2, 4}, 4)
	if records, err := artifacts.EventsPage(context.Background(), "computer-replay-gap", 0, 1024); err == nil || !strings.Contains(err.Error(), "gap") {
		t.Fatalf("gapped chain page = %v, %v; want a gap error", replayPageSequences(records), err)
	}
	if records, err := artifacts.EventsPage(context.Background(), "computer-replay-gap", 2, 1024); err == nil || !strings.Contains(err.Error(), "gap") {
		t.Fatalf("page starting at a gap = %v, %v; want a gap error", replayPageSequences(records), err)
	}
}
