package agentcore

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/receiptsigner"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// The verifier-key hold
// (docs/problems/verifier-key-regenerated-locks-out-checkpoints-2026-11-02.md).
// d@e's shape: the lost-disk drills regenerated the verifier key; the host
// still pins the old one; an approved apply spent all three guest attempts
// on a checkpoint refusal that could only repeat.
//
// Ways this could fail, written first:
//   - the apply spends an attempt (or writes a planned marker) before a
//     mismatch is found, so the bound burns again;
//   - a mismatch is retried (a retry wake or a loop), not held;
//   - a host that cannot be reached holds every apply (a new stuck state);
//     it must proceed as today and let the checkpoint answer;
//   - a computer with no verifier or control wiring is held;
//   - a held mismatch stays held after the owner re-pins (stale cache);
//   - a checkpoint refused for the key after the apply is retried, not
//     held; any other refusal is held as a key refusal;
//   - /health hides the mismatch, or degrades a match.

type fakeVerifierKey struct {
	keyID      string
	public     string
	provenance receiptsigner.KeyProvenance
}

func newFakeVerifierKey(t *testing.T, prior string) fakeVerifierKey {
	t.Helper()
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := selfdevprotocol.VerifierKeyIDFor(public)
	return fakeVerifierKey{keyID: id, public: base64.RawStdEncoding.EncodeToString(public),
		provenance: receiptsigner.KeyProvenance{KeyID: id, PriorKeyID: prior, GeneratedAt: time.Date(2026, 10, 30, 21, 43, 20, 0, time.UTC)}}
}

// pinAnswer is the fake host's answer (atomic.Value keeps one type).
type pinAnswer struct {
	pin selfdevprotocol.VerifierKeyPin
	err error
}

func wireVerifierKeySeams(rt *Runtime, key fakeVerifierKey, pin *atomic.Value, calls *atomic.Int32) {
	rt.verifierKeyPresenter = func(context.Context) (string, string, receiptsigner.KeyProvenance, error) {
		return key.keyID, key.public, key.provenance, nil
	}
	rt.verifierKeyPreflighter = func(_ context.Context, keyID, _ string) (selfdevprotocol.VerifierKeyPin, error) {
		calls.Add(1)
		answer, _ := pin.Load().(pinAnswer)
		if answer.err != nil {
			return selfdevprotocol.VerifierKeyPin{}, answer.err
		}
		if answer.pin.State == "" {
			return selfdevprotocol.VerifierKeyPin{}, fmt.Errorf("no pin")
		}
		answer.pin.PresentedKeyID = keyID
		return answer.pin, nil
	}
}

func TestVerifierKeyMismatchHoldsTheApplyWithoutSpendingAnAttempt(t *testing.T) {
	store := filepath.Join(t.TempDir(), "state")
	rt := &Runtime{}
	rt.cfg.StorePath = store
	rt.cfg.ComputerID = "computer-59c902c5d46caecc0a180a8557164e72"
	key := newFakeVerifierKey(t, "verifier-control-22033e8312994d59")
	var pin atomic.Value
	var calls atomic.Int32
	pin.Store(pinAnswer{pin: selfdevprotocol.VerifierKeyPin{ComputerID: rt.cfg.ComputerID, State: selfdevprotocol.VerifierKeyMismatch, PinnedKeyID: "verifier-control-22033e8312994d59"}})
	wireVerifierKeySeams(rt, key, &pin, &calls)

	for attempt := 1; attempt <= guestApplyAttemptLimit+2; attempt++ {
		_, _, err := rt.applyOrReadJournal(context.Background(), updater.ApplyRequest{}, PlannedRestartSelfDevelopmentApply, "selfdev-534017c7", false)
		if !errors.Is(err, ErrVerifierKeyNotPinned) {
			t.Fatalf("attempt %d: err = %v, want ErrVerifierKeyNotPinned", attempt, err)
		}
		if !selfDevelopmentHeldWithoutRetry(err) {
			t.Fatalf("attempt %d: a key mismatch is retried: %v", attempt, err)
		}
	}
	if spent, err := peekGuestAttempts(store, PlannedRestartSelfDevelopmentApply, "selfdev-534017c7"); err != nil || spent != 0 {
		t.Fatalf("a held mismatch spent %d attempts (%v)", spent, err)
	}
	if _, err := os.Stat(PlannedRestartMarkerPath(store)); !os.IsNotExist(err) {
		t.Fatalf("a held mismatch wrote a planned marker: %v", err)
	}
	// The pin answer is cached: holding does not ask the host on every pass.
	if calls.Load() > 1 {
		t.Fatalf("the preflight asked the host %d times in one window", calls.Load())
	}

	// The owner re-pins: the guest is told, the cache drops, the hold lifts.
	pin.Store(pinAnswer{pin: selfdevprotocol.VerifierKeyPin{ComputerID: rt.cfg.ComputerID, State: selfdevprotocol.VerifierKeyMatch, PinnedKeyID: key.keyID}})
	rt.invalidateVerifierKeyPin()
	if err := rt.verifierKeyHold(context.Background()); err != nil {
		t.Fatalf("after the re-pin the apply is still held: %v", err)
	}
}

func TestVerifierKeyHoldNeverHoldsOnAnUnansweredHostOrAnUnwiredComputer(t *testing.T) {
	rt := &Runtime{}
	if err := rt.verifierKeyHold(context.Background()); err != nil {
		t.Fatalf("an unwired computer is held: %v", err)
	}
	key := newFakeVerifierKey(t, "")
	var pin atomic.Value
	var calls atomic.Int32
	pin.Store(pinAnswer{err: errors.New("dial corpusd: connection refused")})
	wireVerifierKeySeams(rt, key, &pin, &calls)
	if err := rt.verifierKeyHold(context.Background()); err != nil {
		t.Fatalf("an unreachable host holds the apply: %v", err)
	}
	// First use: the checkpoint pins the presented key.
	pin.Store(pinAnswer{pin: selfdevprotocol.VerifierKeyPin{ComputerID: "c", State: selfdevprotocol.VerifierKeyUnpinned}})
	rt.invalidateVerifierKeyPin()
	if err := rt.verifierKeyHold(context.Background()); err != nil {
		t.Fatalf("an unpinned computer is held: %v", err)
	}
}

func TestCheckpointKeyRefusalAfterTheApplyIsTheSameHold(t *testing.T) {
	rt := &Runtime{}
	refused := &selfdev.CheckpointRefusedError{Status: http.StatusBadRequest, Detail: `{"error":"` + selfdevprotocol.CheckpointRefusalVerifierKeyNotPinned + `"}`}
	err := rt.classifyCheckpointPublishError(fmt.Errorf("bind: %w", refused))
	if !errors.Is(err, ErrVerifierKeyNotPinned) || !selfDevelopmentHeldWithoutRetry(err) {
		t.Fatalf("checkpoint key refusal = %v, want a held ErrVerifierKeyNotPinned", err)
	}
	other := &selfdev.CheckpointRefusedError{Status: http.StatusBadRequest, Detail: `{"error":"checkpoint authority: non-verifier head receipt does not bind the accepted head"}`}
	if err := rt.classifyCheckpointPublishError(other); errors.Is(err, ErrVerifierKeyNotPinned) || selfDevelopmentHeldWithoutRetry(err) {
		t.Fatalf("another refusal was held as a key refusal: %v", err)
	}
	conflict := &selfdev.CheckpointRefusedError{Status: http.StatusConflict, Detail: selfdevprotocol.CheckpointRefusalVerifierKeyNotPinned}
	if err := rt.classifyCheckpointPublishError(conflict); !errors.Is(err, ErrVerifierKeyNotPinned) {
		t.Fatalf("a 4xx key refusal was not held: %v", err)
	}
	if err := rt.classifyCheckpointPublishError(nil); err != nil {
		t.Fatalf("nil = %v", err)
	}
}

func TestHealthReportsTheVerifierKeyPin(t *testing.T) {
	productStore, err := choirstore.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer productStore.Close()
	rt := &Runtime{cfg: provideriface.Config{ComputerID: "computer-59c902c5d46caecc0a180a8557164e72"}, store: productStore, provider: healthStubProvider{}}
	rt.health = types.HealthReady
	handler := &APIHandler{rt: rt}
	key := newFakeVerifierKey(t, "verifier-control-22033e8312994d59")
	var pin atomic.Value
	var calls atomic.Int32
	pin.Store(pinAnswer{pin: selfdevprotocol.VerifierKeyPin{ComputerID: rt.cfg.ComputerID, State: selfdevprotocol.VerifierKeyMismatch, PinnedKeyID: "verifier-control-22033e8312994d59"}})
	wireVerifierKeySeams(rt, key, &pin, &calls)

	read := func() (string, map[string]any) {
		t.Helper()
		rec := httptest.NewRecorder()
		handler.HandleHealth(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("/health = %d %s", rec.Code, rec.Body.String())
		}
		var body struct {
			Status      string         `json:"status"`
			VerifierKey map[string]any `json:"verifier_key"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body.Status, body.VerifierKey
	}
	// /health never asks the host itself: it reads what the last check saw.
	if _, err := rt.checkVerifierKeyPin(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	status, verifier := read()
	if status != "degraded" || verifier["pin"] != selfdevprotocol.VerifierKeyMismatch || verifier["key_id"] != key.keyID ||
		verifier["pinned_key_id"] != "verifier-control-22033e8312994d59" || verifier["prior_key_id"] != "verifier-control-22033e8312994d59" {
		t.Fatalf("/health on d@e's shape: status=%q verifier_key=%v", status, verifier)
	}
	pin.Store(pinAnswer{pin: selfdevprotocol.VerifierKeyPin{ComputerID: rt.cfg.ComputerID, State: selfdevprotocol.VerifierKeyMatch, PinnedKeyID: key.keyID}})
	rt.invalidateVerifierKeyPin()
	if _, err := rt.checkVerifierKeyPin(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	if status, verifier = read(); status == "degraded" || verifier["pin"] != selfdevprotocol.VerifierKeyMatch {
		t.Fatalf("/health after the re-pin: status=%q verifier_key=%v", status, verifier)
	}
}

func TestRepinnedRecordVerifiesTheHostsReceiptAndNamesThisKey(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	platformKey := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "platform-control-test"}, PrivateKey: private}
	resolver := selfdev.PlatformKeyResolver{ComputerID: "computer-de", KeyID: "platform-control-test", PublicKey: public}
	key := newFakeVerifierKey(t, "verifier-control-old")
	mint := func(fields map[string]any, signer computerevent.SigningKey) computerevent.Receipt {
		t.Helper()
		receipt, err := computerevent.NewSignedReceipt(selfdevprotocol.VerifierKeyPinnedReceiptKind, "corpusd", fields, []computerevent.SigningKey{signer}, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		return receipt
	}
	good := map[string]any{"computer_id": "computer-de", "key_id": key.keyID, "public_key": key.public, "repair": selfdevprotocol.VerifierKeyRepinRepair,
		"reason": "lost disk", "replaces_key_id": "verifier-control-old"}
	if err := verifyRepinReceipt(mint(good, platformKey), resolver, "computer-de", key.keyID); err != nil {
		t.Fatalf("good pin receipt refused: %v", err)
	}
	other := map[string]any{}
	for k, v := range good {
		other[k] = v
	}
	other["key_id"] = "verifier-control-someone-else"
	if err := verifyRepinReceipt(mint(other, platformKey), resolver, "computer-de", key.keyID); err == nil {
		t.Fatal("a pin receipt for another key was recorded")
	}
	_, forger, _ := ed25519.GenerateKey(rand.Reader)
	if err := verifyRepinReceipt(mint(good, computerevent.SigningKey{SignerRef: platformKey.SignerRef, PrivateKey: forger}), resolver, "computer-de", key.keyID); err == nil {
		t.Fatal("a pin receipt the host did not sign was recorded")
	}
	if err := verifyRepinReceipt(mint(good, platformKey), resolver, "computer-other", key.keyID); err == nil {
		t.Fatal("a pin receipt for another computer was recorded")
	}
}

// Review of 9a465a7f2, DeepSeek #3: a check in flight when the re-pin
// invalidates the cache must not write its pre-re-pin mismatch afterwards.
func TestVerifierKeyAnswerStartedBeforeAnInvalidationIsNotCached(t *testing.T) {
	rt := &Runtime{}
	key := newFakeVerifierKey(t, "")
	asked, release := make(chan struct{}), make(chan struct{})
	rt.verifierKeyPresenter = func(context.Context) (string, string, receiptsigner.KeyProvenance, error) {
		return key.keyID, key.public, key.provenance, nil
	}
	rt.verifierKeyPreflighter = func(context.Context, string, string) (selfdevprotocol.VerifierKeyPin, error) {
		close(asked)
		<-release
		return selfdevprotocol.VerifierKeyPin{ComputerID: "c", State: selfdevprotocol.VerifierKeyMismatch, PinnedKeyID: "old"}, nil
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = rt.checkVerifierKeyPin(context.Background(), 0)
	}()
	<-asked
	rt.invalidateVerifierKeyPin()
	close(release)
	<-done
	rt.verifierKeyPin.mu.Lock()
	last := rt.verifierKeyPin.last
	rt.verifierKeyPin.mu.Unlock()
	if last != nil {
		t.Fatalf("a pre-invalidation answer was cached: %+v", last.Pin)
	}
}
