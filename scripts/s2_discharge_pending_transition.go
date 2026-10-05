//go:build ignore

// s2_discharge_pending_transition manually commits the missing
// materialization_failed event that clears a wedged pending transition on a
// guest whose deployed runtime predates the platform_update.go refusal-
// discharge fix (docs/problems/s2-refused-apply-wedges-pending-transition
// -2026-10-05.md).
//
// It replicates recordPlatformUpdateFailed over the corpusd event-CAS HTTP
// surface: mint a capability under the platform signing key, read the head,
// pin the result payload, pin the canonical event, CompareAndSwap.
//
// Run on Node B from a repo worktree as root:
//
//	go run scripts/s2_discharge_pending_transition.go -computer <id> [-note "..."]
//
// After discharge the guest's embedded projection lags the platform head —
// cold-boot the VM once so replay rebuilds it (ErrNeedsProjectionRepair on
// any append before then is expected).
package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/platform"
)

var (
	corpusdURL   = flag.String("corpusd", "http://127.0.0.1:8086", "corpusd base URL")
	computerID   = flag.String("computer", "", "computer id (required)")
	signingKey   = flag.String("key", "/var/lib/go-choir/platform-artifacts/signing-key", "platform signing key path")
	note         = flag.String("note", "manual discharge: pre-mutation refusal left pending transition wedged (s2-refused-apply-wedges-pending-transition-2026-10-05)", "cause recorded in the result payload")
	internalHead = flag.String("caller", "true", "X-Internal-Caller header value")
)

func main() {
	flag.Parse()
	if *computerID == "" {
		fmt.Fprintln(os.Stderr, "-computer required")
		os.Exit(2)
	}
	key, err := platform.LoadOrCreateSigningKey(*signingKey)
	if err != nil {
		fatal("signing key: %v", err)
	}
	token, err := platform.MintComputerCapability(platform.ComputerCapability{
		Version: 1, ComputerID: *computerID,
		Scopes:   []string{"event:read", "event:pin", "event:append"},
		ExpiresAt: time.Now().UTC().Add(10 * time.Minute).Truncate(time.Microsecond).Format(time.RFC3339Nano),
		Nonce:    fmt.Sprintf("manual-discharge-%d", time.Now().UnixNano()),
	}, ed25519.PrivateKey(key.Private))
	if err != nil {
		fatal("capability: %v", err)
	}
	_ = token // capability verified via signature only; send as Bearer

	// 1. Head must carry a pending transition bound to a platform-update
	//    accept event — otherwise there is nothing to discharge.
	var head computerevent.Head
	get("/internal/computers/events/head?computer_id="+*computerID, token, &head)
	if head.PendingTransitionRef == "" {
		fatal("head has no pending transition — nothing to discharge")
	}

	// 2. Recover the accepted event (the pending ref) to bind DecisionRef and
	//    read the offer's update_id for the canonical idempotency key.
	var page []computerevent.DurableEvent
	get("/internal/computers/events/replay?computer_id="+*computerID+"&limit=2000", token, &page)
	var accepted *computerevent.DurableEvent
	for i := range page {
		digest, _ := page[i].Request.Event.Digest()
		if digest == head.PendingTransitionRef || page[i].Request.EventDigest == head.PendingTransitionRef {
			accepted = &page[i]
		}
	}
	if accepted == nil {
		fatal("pending transition %s not found in replay", head.PendingTransitionRef)
	}
	if accepted.Request.Event.EventKind != computerevent.EventEffectAccepted {
		fatal("pending transition is %s, not effect_accepted — refusing discharge", accepted.Request.Event.EventKind)
	}
	offerDigest := accepted.Request.Event.PayloadCommitment
	if offerDigest == "" {
		offerDigest = accepted.Request.Event.ProposedEffectRef
	}
	// The offer pins as the accepted event's payload; read update_id from it.
	var offer map[string]any
	var payload struct {
		Artifact string `json:"artifact"`
	}
	_ = payload
	var payloadBytes []byte
	getBytes("/internal/computers/events/payload?computer_id="+*computerID+"&artifact_digest="+offerDigest, token, &payloadBytes)
	if err := json.Unmarshal(payloadBytes, &offer); err != nil {
		fatal("offer payload decode: %v", err)
	}
	updateID, _ := offer["update_id"].(string)
	if updateID == "" {
		fatal("offer payload lacks update_id")
	}

	// 3. Mirror recordPlatformUpdateFailed's event.
	eventID, err := computerevent.NewEventID()
	if err != nil {
		fatal("event id: %v", err)
	}
	resultPayload, _ := json.Marshal(map[string]any{
		"outcome":    "failed",
		"discharge":  "manual",
		"cause":      *note,
		"update_id":  updateID,
		"release":    offerDigest,
		"discharged": time.Now().UTC().Format(time.RFC3339Nano),
	})
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: *computerID,
		Sequence:      head.Sequence + 1, PreviousHead: head.CanonicalEventHead,
		EventKind:      computerevent.EventMaterializationFailed,
		OccurredAt:     time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: "platform-update-failed-" + updateID,
		ActorProfile:   "management", AuthorityRef: "guest-core:choir-updater",
		PrivacyClass:                    "owner",
		ProposedEffectRef:               offerDigest,
		DecisionRef:                     head.PendingTransitionRef,
		ReducerVersion:                  computerevent.ReducerVersionV1,
		ExpectedDesiredEventHead:        head.DesiredEventHead,
		ExpectedEffectiveEventHead:      head.EffectiveEventHead,
		ExpectedPendingTransitionRef:    head.PendingTransitionRef,
		ExpectedDesiredStateCommitment:  head.DesiredStateCommitment,
		ExpectedEffectiveStateCommitment: head.EffectiveStateCommitment,
	}
	input := computerevent.TransitionInput{RestoredPriorEffective: true}

	payloadDigest := computerevent.DigestBytes(resultPayload)
	ref, err := computerevent.ArtifactRefFromDigest(payloadDigest)
	if err != nil {
		fatal("artifact ref: %v", err)
	}
	event.PayloadCommitment = payloadDigest
	event.OutputArtifactRefs = []string{ref.String()}
	event.RequestCommitment = computerevent.ZeroHead
	pinIntent, err := computerevent.ComputePinIntentCommitment(event, input)
	if err != nil {
		fatal("pin intent: %v", err)
	}
	payloadPin := pin("computer-event-payload", *computerID, resultPayload,
		"application/vnd.choir.platform-update-result+json", "owner", pinIntent, token)
	if payloadPin.ArtifactDigest != payloadDigest {
		fatal("payload pin digest mismatch")
	}
	payloadReceiptBytes, err := payloadPin.Receipt.CanonicalBytes()
	if err != nil {
		fatal("payload receipt bytes: %v", err)
	}
	payloadReceiptDigest := computerevent.DigestBytes(payloadReceiptBytes)
	event.RequestCommitment, err = computerevent.ComputeRequestCommitment(event, input, pinIntent, []string{payloadReceiptDigest})
	if err != nil {
		fatal("request commitment: %v", err)
	}

	body, err := event.CanonicalBytes()
	if err != nil {
		fatal("canonical event: %v", err)
	}
	digest, err := event.Digest()
	if err != nil {
		fatal("event digest: %v", err)
	}
	eventPin := pinEvent(*computerID, body, event.RequestCommitment, token)
	if eventPin.ArtifactDigest != digest {
		fatal("event pin digest mismatch")
	}
	eventReceiptBytes, err := eventPin.Receipt.CanonicalBytes()
	if err != nil {
		fatal("event receipt bytes: %v", err)
	}
	next, err := computerevent.Reduce(&head, event, input)
	if err != nil {
		fatal("reduce: %v", err)
	}
	request := computerevent.CASRequest{
		Event:                    event,
		EventDigest:              digest,
		EventArtifactDigest:      eventPin.ArtifactDigest,
		EventPinReceiptDigest:    computerevent.DigestBytes(eventReceiptBytes),
		PayloadPinReceiptDigests: []string{payloadReceiptDigest},
		PinIntentCommitment:      pinIntent,
		Input:                    input,
		Next:                     next,
	}
	var receipt json.RawMessage
	post("/internal/computers/events/append", request, token, &receipt)

	var after computerevent.Head
	get("/internal/computers/events/head?computer_id="+*computerID, token, &after)
	out, _ := json.MarshalIndent(map[string]any{
		"discharged_update":      updateID,
		"failed_event_digest":    digest,
		"pending_before":         head.PendingTransitionRef,
		"pending_after":          after.PendingTransitionRef,
		"canonical_head_before":  head.CanonicalEventHead,
		"canonical_head_after":   after.CanonicalEventHead,
		"desired_head_after":     after.DesiredEventHead,
	}, "", "  ")
	fmt.Println(string(out))
	if after.PendingTransitionRef != "" {
		fatal("pending transition still set after discharge")
	}
}

func fatal(format string, args ...any) { fmt.Fprintf(os.Stderr, "discharge: "+format+"\n", args...); os.Exit(1) }

func do(req *http.Request, token string) *http.Response {
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Internal-Caller", *internalHead)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fatal("http %s: %v", req.URL, err)
	}
	return resp
}

func get(path, token string, out any) {
	req, _ := http.NewRequest(http.MethodGet, *corpusdURL+path, nil)
	resp := do(req, token)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		fatal("GET %s: %d %s", path, resp.StatusCode, b)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		fatal("GET %s decode: %v", path, err)
	}
}

func getBytes(path, token string, out *[]byte) {
	req, _ := http.NewRequest(http.MethodGet, *corpusdURL+path, nil)
	resp := do(req, token)
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fatal("GET %s: %d %s", path, resp.StatusCode, b)
	}
	// The payload endpoint returns {artifact_base64} or raw; try json first.
	var wrapped struct {
		PayloadBase64 string `json:"payload_base64"`
		Payload       string `json:"payload"`
	}
	if json.Unmarshal(b, &wrapped) == nil && (wrapped.PayloadBase64 != "" || wrapped.Payload != "") {
		raw := wrapped.PayloadBase64
		if raw == "" {
			raw = wrapped.Payload
		}
		dec, err := base64.StdEncoding.DecodeString(raw)
		if err == nil {
			*out = dec
			return
		}
	}
	*out = b
}

type pinResult struct {
	ArtifactDigest string               `json:"artifact_digest"`
	Receipt        computerevent.Receipt `json:"receipt"`
}

func pin(namespace, computerID string, payload []byte, mediaType, privacyClass, pinIntent, token string) pinResult {
	req := map[string]any{
		"computer_id": computerID, "payload_base64": base64.RawStdEncoding.EncodeToString(payload),
		"media_type": mediaType, "privacy_class": privacyClass,
		"pin_namespace": namespace, "pin_intent_commitment": pinIntent,
	}
	var out pinResult
	post("/internal/computers/events/pin", req, token, &out)
	return out
}

func pinEvent(computerID string, canonicalEvent []byte, requestCommitment, token string) pinResult {
	req := map[string]any{
		"computer_id": computerID, "payload_base64": base64.RawStdEncoding.EncodeToString(canonicalEvent),
		"media_type": "application/vnd.choir.computer-event+json", "privacy_class": "private",
		"pin_namespace": "computer-event", "request_commitment": requestCommitment,
	}
	var out pinResult
	post("/internal/computers/events/pin", req, token, &out)
	return out
}

func post(path string, body any, token string, out any) {
	buf := bytes.Buffer{}
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		fatal("encode %s: %v", path, err)
	}
	req, _ := http.NewRequest(http.MethodPost, *corpusdURL+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	resp := do(req, token)
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		fatal("POST %s: %d %s", path, resp.StatusCode, b)
	}
	if out != nil {
		if err := json.Unmarshal(b, out); err != nil {
			fatal("POST %s decode: %v (%s)", path, err, b)
		}
	}
}
