package selfdev

// The guest's read of its checkpoint listing (slice 3 of
// docs/archive/design/apply-checkpoint-without-replay-2026-10-26.md), written before
// the code. Ways it could fail:
//
//  1. the client reads another computer's listing, or sends no capability;
//  2. a non-200 answer reads as an empty listing (the cadence would then bind
//     every tick: an outage is not "no checkpoint");
//  3. a fetched checkpoint whose receipt does not verify against the
//     platform-control key, or that is another digest's, is accepted: an
//     owner would restore to a forged artifact.

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

func TestListCheckpointsReadsThisComputersListing(t *testing.T) {
	var seen string
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.Path + "?" + r.URL.RawQuery
		auth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(selfdevprotocol.CheckpointList{Checkpoints: []selfdevprotocol.CheckpointSummary{
			{CheckpointDigest: strings.Repeat("a", 64), AcceptedSequence: 7, Verdict: selfdevprotocol.CheckpointVerdict{Status: "verified"}},
		}})
	}))
	defer server.Close()
	creds := GuestCredentialsWithCapability(server.URL, "computer-list", "cap-token", time.Now().Add(time.Hour))

	list, err := creds.ListCheckpoints(context.Background(), "computer-list", 5)
	if err != nil || len(list.Checkpoints) != 1 || list.Checkpoints[0].AcceptedSequence != 7 {
		t.Fatalf("list = %+v %v", list, err)
	}
	if !strings.HasPrefix(seen, "/internal/computers/checkpoints/list?") || !strings.Contains(seen, "computer_id=computer-list") || !strings.Contains(seen, "limit=5") {
		t.Fatalf("request = %q", seen)
	}
	if auth != "Bearer cap-token" {
		t.Fatalf("authorization = %q", auth)
	}
	if _, err := creds.ListCheckpoints(context.Background(), "computer-other", 5); err == nil {
		t.Fatal("the client listed another computer's checkpoints")
	}
}

func TestListCheckpointsTreatsAnOutageAsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	creds := GuestCredentialsWithCapability(server.URL, "computer-list", "cap-token", time.Now().Add(time.Hour))
	if list, err := creds.ListCheckpoints(context.Background(), "computer-list", 5); err == nil {
		t.Fatalf("a 502 read as a listing: %+v", list)
	}
}

func signedCheckpointResponse(t *testing.T, key ed25519.PrivateKey, computerID string) selfdevprotocol.CheckpointResponse {
	t.Helper()
	digest := func(c byte) string { return strings.Repeat(string(c), 64) }
	request := selfdevprotocol.CheckpointRequest{
		ComputerID: computerID, IdempotencyKey: "owner-recovery-test",
		ComputerVersion: computerversion.ComputerVersion{
			CodeRef:            computerversion.CodeRef("code:sha256:" + digest('d')),
			ArtifactProgramRef: computerversion.ArtifactProgramRef("artifact-program:sha256:" + digest('e')),
		},
		AcceptedEventHead: digest('1'), EffectiveEventHead: digest('2'), EffectiveStateCommitment: digest('2'),
		EventHeadReceiptID: "receipt-test", ReleaseDigest: digest('f'), ReconstructionDigest: digest('3'),
		ReducerVersion: 1, OwnerRecovery: true,
		VMLocalContentWitness: mustWitnessV3(t, "texture", digest('a'), map[string]string{"agents": digest('b')}),
		FrontendIdentity: selfdevprotocol.FrontendIdentity{Digest: digest('c'), Derivation: selfdevprotocol.FrontendDerivationRelease, ReleaseDigest: digest('f')},
	}
	checkpoint, _, err := selfdevprotocol.CheckpointFromRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	commitment, _ := selfdevprotocol.Digest(request)
	signer := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "list-test"}, PrivateKey: key}
	receipt, err := selfdevprotocol.NewAuthorityReceipt(selfdevprotocol.ReceiptKindCheckpoint, computerID, commitment, checkpoint.Digest, "corpusd", signer, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	return selfdevprotocol.CheckpointResponse{Checkpoint: checkpoint, Receipt: receipt}
}

func TestFetchCheckpointByDigestVerifiesTheReceipt(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	_, otherKey, _ := ed25519.GenerateKey(rand.Reader)
	good := signedCheckpointResponse(t, privateKey, "computer-fetch")
	forged := signedCheckpointResponse(t, otherKey, "computer-fetch")
	serve := func(response selfdevprotocol.CheckpointResponse, status int) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if status != http.StatusOK {
				w.WriteHeader(status)
				return
			}
			_ = json.NewEncoder(w).Encode(response)
		}))
	}

	server := serve(good, http.StatusOK)
	defer server.Close()
	creds := GuestCredentialsWithCapability(server.URL, "computer-fetch", "cap", time.Now().Add(time.Hour), publicKey)
	got, err := creds.FetchCheckpointByDigest(context.Background(), "computer-fetch", good.Checkpoint.Digest)
	if err != nil || got == nil || got.Checkpoint.Digest != good.Checkpoint.Digest {
		t.Fatalf("fetch = %+v %v", got, err)
	}
	if _, err := creds.FetchCheckpointByDigest(context.Background(), "computer-fetch", strings.Repeat("0", 64)); err == nil {
		t.Fatal("a checkpoint with another digest than asked for was accepted")
	}

	forgedServer := serve(forged, http.StatusOK)
	defer forgedServer.Close()
	forgedCreds := GuestCredentialsWithCapability(forgedServer.URL, "computer-fetch", "cap", time.Now().Add(time.Hour), publicKey)
	if got, err := forgedCreds.FetchCheckpointByDigest(context.Background(), "computer-fetch", forged.Checkpoint.Digest); err == nil {
		t.Fatalf("a checkpoint signed by another key was accepted: %+v", got)
	}

	missingServer := serve(good, http.StatusNotFound)
	defer missingServer.Close()
	missingCreds := GuestCredentialsWithCapability(missingServer.URL, "computer-fetch", "cap", time.Now().Add(time.Hour), publicKey)
	if got, err := missingCreds.FetchCheckpointByDigest(context.Background(), "computer-fetch", good.Checkpoint.Digest); err != nil || got != nil {
		t.Fatalf("404 = %+v %v, want nil and no error", got, err)
	}
}

// mustWitnessV3 is a valid v3 witness for tests that only need a checkpoint to
// pass validation.
func mustWitnessV3(t *testing.T, database, schemaHash string, tables map[string]string) selfdevprotocol.VMLocalContentWitness {
	t.Helper()
	w, err := selfdevprotocol.NewWitnessV3(database, schemaHash, tables)
	if err != nil {
		t.Fatal(err)
	}
	return w
}
