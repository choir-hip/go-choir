package agentcore

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/receiptsigner"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// The verifier-key hold
// (docs/problems/verifier-key-regenerated-locks-out-checkpoints-2026-11-02.md).
//
// The host pins this computer's verifier-control key and refuses every
// verifier-class checkpoint signed by another. A lost data disk regenerates
// the key, so an approved apply would spend every guest attempt on a refusal
// that can only repeat. The guest asks the host, read only, before an attempt
// is spent; a key the host refuses there holds the operation with a typed
// reason, no attempt and no retry wake, until the owner re-pins
// (choir computer repin-verifier). When the preflight could not answer (the
// host unreachable) or passed and the pin changed after it, the checkpoint
// refusal comes after the attempt was spent: that one attempt is spent, and
// the same hold stops every later one. /health carries the answer.

// ErrVerifierKeyNotPinned holds a self-development apply or rollback whose
// verifier key is not the host's pin. The owner clears it with a re-pin.
var ErrVerifierKeyNotPinned = errors.New("self-development held: the verifier key is not the pinned computer key; the owner re-pins it with choir computer repin-verifier")

// verifierKeyPinTTL is how long a pin answer holds before the next apply
// asks again; /health refreshes in the background after it.
const verifierKeyPinTTL = time.Minute

type verifierKeyCheck struct {
	KeyID      string
	PublicKey  string
	Provenance receiptsigner.KeyProvenance
	Pin        selfdevprotocol.VerifierKeyPin
	CheckedAt  time.Time
	Err        string
}

// verifierKeyPinCache is the last answer, shared by the hold and /health.
type verifierKeyPinCache struct {
	mu         sync.Mutex
	last       *verifierKeyCheck
	refreshing bool
	// generation moves on every invalidation; an answer whose question
	// started before it is never cached (a pre-re-pin mismatch written after
	// the re-pin would hold the one reconciler wake).
	generation uint64
}

func (rt *Runtime) presentVerifierKey(ctx context.Context) (string, string, receiptsigner.KeyProvenance, error) {
	if rt.verifierKeyPresenter != nil {
		return rt.verifierKeyPresenter(ctx)
	}
	if rt.selfdevVerifier == nil {
		return "", "", receiptsigner.KeyProvenance{}, errVerifierKeyUnwired
	}
	ref, public, provenance, err := rt.selfdevVerifier.PublicKeyProvenance(ctx)
	if err != nil {
		return "", "", receiptsigner.KeyProvenance{}, err
	}
	return ref.KeyID, encodeVerifierPublicKey(public), provenance, nil
}

func (rt *Runtime) preflightVerifierKey(ctx context.Context, keyID, publicKey string) (selfdevprotocol.VerifierKeyPin, error) {
	if rt.verifierKeyPreflighter != nil {
		return rt.verifierKeyPreflighter(ctx, keyID, publicKey)
	}
	control := rt.selfdevControl
	if control == nil {
		control = rt.ownerRecoveryControl
	}
	if control == nil {
		return selfdevprotocol.VerifierKeyPin{}, errVerifierKeyUnwired
	}
	return control.VerifierKeyPreflight(ctx, keyID, publicKey)
}

var errVerifierKeyUnwired = errors.New("verifier key check is not wired")

// checkVerifierKeyPin answers the pin state of the presented key, from the
// cache when the last answer is younger than maxAge.
func (rt *Runtime) checkVerifierKeyPin(ctx context.Context, maxAge time.Duration) (verifierKeyCheck, error) {
	cache := &rt.verifierKeyPin
	cache.mu.Lock()
	if last := cache.last; last != nil && maxAge > 0 && last.Err == "" && time.Since(last.CheckedAt) < maxAge {
		cached := *last
		cache.mu.Unlock()
		return cached, nil
	}
	generation := cache.generation
	cache.mu.Unlock()
	check := verifierKeyCheck{CheckedAt: time.Now().UTC()}
	keyID, public, provenance, err := rt.presentVerifierKey(ctx)
	if errors.Is(err, errVerifierKeyUnwired) {
		return check, err
	}
	if err == nil {
		check.KeyID, check.PublicKey, check.Provenance = keyID, public, provenance
		check.Pin, err = rt.preflightVerifierKey(ctx, keyID, public)
		if errors.Is(err, errVerifierKeyUnwired) {
			return check, err
		}
	}
	if err != nil {
		check.Err = err.Error()
	}
	cache.mu.Lock()
	previous := cache.last
	stale := cache.generation != generation
	if !stale {
		cache.last = &check
	}
	cache.mu.Unlock()
	if stale && maxAge > 0 {
		// Invalidated while asking (a re-pin landed): ask again.
		return rt.checkVerifierKeyPin(ctx, maxAge)
	}
	if check.Err == "" && check.Pin.Refuses() && (previous == nil || previous.Pin.State != check.Pin.State || previous.KeyID != check.KeyID) {
		log.Printf("selfdev: verifier key %s is %s (host pin %s; prior key on this disk %q): verifier checkpoints will be refused until the owner re-pins",
			check.KeyID, check.Pin.State, check.Pin.PinnedKeyID, check.Provenance.PriorKeyID)
	}
	return check, err
}

// invalidateVerifierKeyPin drops the cached answer (after a re-pin).
func (rt *Runtime) invalidateVerifierKeyPin() {
	rt.verifierKeyPin.mu.Lock()
	rt.verifierKeyPin.last = nil
	rt.verifierKeyPin.generation++
	rt.verifierKeyPin.mu.Unlock()
}

// verifierKeyHold is nil unless the host answered that it would refuse the
// presented key. An unwired computer or a host that cannot be asked proceeds
// as before: the checkpoint itself answers, and holding on an outage would be
// a new stuck state.
func (rt *Runtime) verifierKeyHold(ctx context.Context) error {
	if rt == nil {
		return nil
	}
	check, err := rt.checkVerifierKeyPin(ctx, verifierKeyPinTTL)
	if err != nil {
		if !errors.Is(err, errVerifierKeyUnwired) {
			log.Printf("selfdev: verifier key preflight unanswered (the apply proceeds; the checkpoint decides): %v", err)
		}
		return nil
	}
	switch check.Pin.State {
	case selfdevprotocol.VerifierKeyMismatch:
		return fmt.Errorf("%w (presented %s, host pins %s)", ErrVerifierKeyNotPinned, check.KeyID, check.Pin.PinnedKeyID)
	case selfdevprotocol.VerifierKeyUnpinnedNoGenesis:
		return fmt.Errorf("%w (host state %s: the computer has no genesis on the host; bootstrap its chain, a re-pin does not apply)", ErrVerifierKeyNotPinned, check.Pin.State)
	case selfdevprotocol.VerifierKeyInvalid:
		return fmt.Errorf("%w (host state %s: the signer presents a key id that does not name its key; a re-pin does not apply)", ErrVerifierKeyNotPinned, check.Pin.State)
	}
	return nil
}

// classifyCheckpointPublishError maps a checkpoint refusal of the verifier
// key to the hold; anything else is returned unchanged.
func (rt *Runtime) classifyCheckpointPublishError(err error) error {
	var refused *selfdev.CheckpointRefusedError
	if err == nil || !errors.As(err, &refused) || !refused.RefusesVerifierKey() {
		return err
	}
	if rt != nil {
		rt.invalidateVerifierKeyPin()
	}
	return fmt.Errorf("%w: %v", ErrVerifierKeyNotPinned, err)
}

// selfDevelopmentHeldWithoutRetry reports an operation error the reconciler
// holds without a retry wake: a retry can only repeat it until an owner act.
func selfDevelopmentHeldWithoutRetry(err error) bool {
	return errors.Is(err, ErrGuestApplyAttemptsSpent) || errors.Is(err, ErrVerifierKeyNotPinned)
}

// VerifierKeyHealth is /health's verifier_key: what the last check saw.
type VerifierKeyHealth struct {
	KeyID       string `json:"key_id,omitempty"`
	Pin         string `json:"pin"`
	PinnedKeyID string `json:"pinned_key_id,omitempty"`
	GeneratedAt string `json:"generated_at,omitempty"`
	PriorKeyID  string `json:"prior_key_id,omitempty"`
	CheckedAt   string `json:"checked_at,omitempty"`
	Error       string `json:"error,omitempty"`
}

// verifierKeyHealth reads the cached answer and, when it is missing or
// stale, refreshes it in the background: /health never waits on the host.
func (rt *Runtime) verifierKeyHealth() *VerifierKeyHealth {
	if rt == nil || (rt.verifierKeyPresenter == nil && rt.selfdevVerifier == nil) ||
		(rt.verifierKeyPreflighter == nil && rt.selfdevControl == nil && rt.ownerRecoveryControl == nil) {
		return nil
	}
	cache := &rt.verifierKeyPin
	cache.mu.Lock()
	last := cache.last
	stale := last == nil || time.Since(last.CheckedAt) >= verifierKeyPinTTL
	if stale && !cache.refreshing {
		cache.refreshing = true
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			_, _ = rt.checkVerifierKeyPin(ctx, 0)
			cache.mu.Lock()
			cache.refreshing = false
			cache.mu.Unlock()
		}()
	}
	cache.mu.Unlock()
	if last == nil {
		return &VerifierKeyHealth{Pin: "unknown"}
	}
	health := &VerifierKeyHealth{KeyID: last.KeyID, Pin: last.Pin.State, PinnedKeyID: last.Pin.PinnedKeyID,
		PriorKeyID: last.Provenance.PriorKeyID, CheckedAt: last.CheckedAt.Format(time.RFC3339), Error: last.Err}
	if !last.Provenance.GeneratedAt.IsZero() {
		health.GeneratedAt = last.Provenance.GeneratedAt.UTC().Format(time.RFC3339Nano)
	}
	if last.Err != "" {
		health.Pin = "unknown"
	}
	return health
}

// verifyRepinReceipt checks corpusd's pin receipt: signed by the platform
// key, an owner re-pin of this computer's presented key.
func verifyRepinReceipt(receipt computerevent.Receipt, resolver selfdev.PlatformKeyResolver, computerID, keyID string) error {
	if receipt.ReceiptKind != selfdevprotocol.VerifierKeyPinnedReceiptKind || receipt.Verify(resolver) != nil {
		return fmt.Errorf("verifier re-pin: the pin receipt is not the host's")
	}
	field := func(name string) string { value, _ := receipt.KindFields[name].(string); return value }
	if field("computer_id") != computerID || field("key_id") != keyID || field("repair") != selfdevprotocol.VerifierKeyRepinRepair || strings.TrimSpace(field("reason")) == "" {
		return fmt.Errorf("verifier re-pin: the pin receipt does not re-pin this computer's verifier key")
	}
	return nil
}

// verifierKeyRoute serves GET .../self-development/verifier-key (the key
// this computer's verifier presents and the host's pin, asked now) and
// POST .../self-development/verifier-key/repinned (the proxy hands over
// corpusd's receipt after the owner's re-pin; the guest records it on its
// tape and wakes the reconciler once). The record cannot change the pin:
// only corpusd's owner-act route does.
func (h *APIHandler) verifierKeyRoute(w http.ResponseWriter, r *http.Request, computerID string, repinned bool) {
	rt := h.rt
	if !repinned {
		if r.Method != http.MethodGet {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		check, err := rt.checkVerifierKeyPin(r.Context(), 0)
		if errors.Is(err, errVerifierKeyUnwired) {
			writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "verifier key check is not wired on this computer"})
			return
		}
		writeAPIJSON(w, http.StatusOK, map[string]any{
			"computer_id": computerID, "key_id": check.KeyID, "public_key": check.PublicKey,
			"generated_at": formatOptionalTime(check.Provenance.GeneratedAt), "prior_key_id": check.Provenance.PriorKeyID,
			"pin": check.Pin, "error": check.Err,
		})
		return
	}
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	var body struct {
		PinReceipt computerevent.Receipt `json:"pin_receipt"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid re-pin record"})
		return
	}
	control := rt.selfdevControl
	if control == nil {
		control = rt.ownerRecoveryControl
	}
	if control == nil || rt.eventAppender == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "re-pin record unavailable on this computer"})
		return
	}
	keyID, _, _, err := rt.presentVerifierKey(r.Context())
	if err != nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "verifier key unavailable"})
		return
	}
	if err := verifyRepinReceipt(body.PinReceipt, control.KeyResolver(), computerID, keyID); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	digest, err := rt.recordVerifierRepin(r.Context(), computerID, body.PinReceipt)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	rt.invalidateVerifierKeyPin()
	rt.TriggerSelfDevelopmentReconcile()
	writeAPIJSON(w, http.StatusOK, map[string]any{"computer_id": computerID, "key_id": keyID, "event_digest": digest})
}

// recordVerifierRepin appends key_rotated with the pin receipt as its
// payload, idempotent on the receipt.
func (rt *Runtime) recordVerifierRepin(ctx context.Context, computerID string, receipt computerevent.Receipt) (string, error) {
	payload, err := receipt.CanonicalBytes()
	if err != nil {
		return "", err
	}
	idempotency := "verifier-key-repinned:" + receipt.ReceiptID
	if existing, found, lookupErr := rt.store.EventByIdempotency(ctx, computerID, idempotency); lookupErr != nil {
		return "", lookupErr
	} else if found {
		raw, _ := existing.CanonicalBytes()
		return computerevent.DigestBytes(raw), nil
	}
	eventID, err := computerevent.NewEventID()
	if err != nil {
		return "", err
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: computerID,
		EventKind: computerevent.EventKeyRotated, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: idempotency, ActorProfile: agentprofile.Management,
		AuthorityRef: "platform-control:verifier-repin", PrivacyClass: "owner", ReducerVersion: computerevent.ReducerVersionV1,
	}
	appended, _, err := rt.eventAppender.AppendNewPayload(ctx, event, computerevent.TransitionInput{}, payload, "application/vnd.choir.verifier-key-pinned+json", "owner")
	if err != nil {
		return "", fmt.Errorf("verifier re-pin: record on the tape: %w", err)
	}
	return fmt.Sprint(appended.KindFields["event_digest"]), nil
}

func encodeVerifierPublicKey(public []byte) string {
	return base64.RawStdEncoding.EncodeToString(public)
}

func formatOptionalTime(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return at.UTC().Format(time.RFC3339Nano)
}
