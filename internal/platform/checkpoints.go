package platform

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

type CheckpointAuthority struct {
	cas     *ComputerEventCAS
	service *Service
	now     func() time.Time
}

func NewCheckpointAuthority(cas *ComputerEventCAS, service *Service) (*CheckpointAuthority, error) {
	if cas == nil || cas.store == nil || cas.store.db == nil || service == nil || service.store != cas.store {
		return nil, fmt.Errorf("checkpoint authority: shared event authority and artifact service are required")
	}
	return &CheckpointAuthority{cas: cas, service: service, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (a *CheckpointAuthority) Publish(ctx context.Context, request selfdevprotocol.CheckpointRequest) (selfdevprotocol.CheckpointResponse, error) {
	checkpoint, artifact, err := selfdevprotocol.CheckpointFromRequest(request)
	if err != nil {
		return selfdevprotocol.CheckpointResponse{}, err
	}
	requestCommitment, err := selfdevprotocol.Digest(request)
	if err != nil {
		return selfdevprotocol.CheckpointResponse{}, err
	}
	lock := &a.cas.locks[computerEventLockIndex(request.ComputerID)]
	lock.Lock()
	defer lock.Unlock()
	if replay, ok, err := a.lookup(ctx, request.ComputerID, request.IdempotencyKey); err != nil {
		return selfdevprotocol.CheckpointResponse{}, err
	} else if ok {
		if replay.Receipt.RequestCommitment != requestCommitment {
			return selfdevprotocol.CheckpointResponse{}, ErrComputerEventCASConflict
		}
		return replay, nil
	}
	var head computerevent.Head
	err = a.cas.store.db.QueryRowContext(ctx, `SELECT computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, COALESCE(pending_transition_ref, ''), desired_state_commitment, effective_state_commitment, reducer_version, credential_revocation_epoch FROM computer_event_heads WHERE computer_id=? FOR UPDATE`, request.ComputerID).Scan(&head.ComputerID, &head.Sequence, &head.CanonicalEventHead, &head.DesiredEventHead, &head.EffectiveEventHead, &head.PendingTransitionRef, &head.DesiredStateCommitment, &head.EffectiveStateCommitment, &head.ReducerVersion, &head.CredentialRevocationEpoch)
	if err != nil || head.PendingTransitionRef != "" || head.CanonicalEventHead != request.AcceptedEventHead || head.EffectiveEventHead != request.EffectiveEventHead || head.EffectiveStateCommitment != request.EffectiveStateCommitment || head.ReducerVersion != request.ReducerVersion {
		return selfdevprotocol.CheckpointResponse{}, fmt.Errorf("checkpoint authority: current accepted/effective head does not match request")
	}
	var storedReceipt string
	err = a.cas.store.db.QueryRowContext(ctx, `SELECT event_head_receipt_json FROM computer_event_append_receipts WHERE computer_id=? AND event_head_receipt_id=?`, request.ComputerID, request.EventHeadReceiptID).Scan(&storedReceipt)
	if err != nil {
		return selfdevprotocol.CheckpointResponse{}, fmt.Errorf("checkpoint authority: accepted event receipt is unavailable: %w", err)
	}
	var eventReceipt computerevent.Receipt
	if json.Unmarshal([]byte(storedReceipt), &eventReceipt) != nil || eventReceipt.ReceiptID != request.EventHeadReceiptID {
		return selfdevprotocol.CheckpointResponse{}, fmt.Errorf("checkpoint authority: accepted event receipt binding failed")
	}
	if err := a.verifyVerifierEvidence(ctx, request); err != nil {
		return selfdevprotocol.CheckpointResponse{}, err
	}
	verifierKey, keyErr := base64.RawStdEncoding.DecodeString(request.VerifierCertificate.PublicKey)
	verifierKeyID := request.VerifierCertificate.Certificate.RequiredSigners[0].KeyID
	if keyErr != nil || len(verifierKey) != ed25519.PublicKeySize {
		return selfdevprotocol.CheckpointResponse{}, fmt.Errorf("checkpoint authority: verifier key is invalid")
	}
	var storedVerifierKeyID string
	var storedVerifierKey []byte
	keyLookupErr := a.cas.store.db.QueryRowContext(ctx, `SELECT key_id,public_key FROM control_key_history WHERE signer_domain='verifier-control' AND computer_id=? AND status='active'`, request.ComputerID).Scan(&storedVerifierKeyID, &storedVerifierKey)
	if request.VerifierTrustBootstrap {
		var eventKind string
		if eventErr := a.cas.store.db.QueryRowContext(ctx, `SELECT event_kind FROM computer_event_append_receipts WHERE computer_id=? AND event_digest=?`, request.ComputerID, request.AcceptedEventHead).Scan(&eventKind); eventErr != nil || eventKind != string(computerevent.EventGenesisImported) {
			return selfdevprotocol.CheckpointResponse{}, fmt.Errorf("checkpoint authority: verifier trust bootstrap requires GenesisImported")
		}
		if keyLookupErr != nil && !errors.Is(keyLookupErr, sql.ErrNoRows) {
			return selfdevprotocol.CheckpointResponse{}, keyLookupErr
		}
	} else if keyLookupErr != nil || storedVerifierKeyID != verifierKeyID || !ed25519.PublicKey(storedVerifierKey).Equal(ed25519.PublicKey(verifierKey)) {
		return selfdevprotocol.CheckpointResponse{}, fmt.Errorf("checkpoint authority: verifier key is not the pinned computer key")
	}
	if keyLookupErr == nil && (storedVerifierKeyID != verifierKeyID || !ed25519.PublicKey(storedVerifierKey).Equal(ed25519.PublicKey(verifierKey))) {
		return selfdevprotocol.CheckpointResponse{}, fmt.Errorf("checkpoint authority: verifier trust substitution refused")
	}
	receipt, err := selfdevprotocol.NewAuthorityReceipt(selfdevprotocol.ReceiptKindCheckpoint, request.ComputerID, requestCommitment, checkpoint.Digest, a.cas.issuer, a.cas.signingKey, a.now())
	if err != nil {
		return selfdevprotocol.CheckpointResponse{}, err
	}
	response := selfdevprotocol.CheckpointResponse{Checkpoint: checkpoint, Receipt: receipt}
	checkpointJSON, _ := json.Marshal(checkpoint)
	receiptJSON, _ := json.Marshal(receipt)
	receiptDigest, _ := selfdevprotocol.Digest(receipt)
	var verifierKeyReceipt computerevent.Receipt
	var verifierKeyReceiptJSON []byte
	var verifierKeyReceiptDigest string
	if request.VerifierTrustBootstrap && errors.Is(keyLookupErr, sql.ErrNoRows) {
		verifierKeyReceipt, err = computerevent.NewSignedReceipt("VerifierKeyPinned", "corpusd", map[string]any{
			"computer_id": request.ComputerID, "key_id": verifierKeyID,
			"public_key": request.VerifierCertificate.PublicKey, "genesis_event_head": request.AcceptedEventHead,
			"checkpoint_digest": checkpoint.Digest,
		}, []computerevent.SigningKey{a.cas.signingKey}, receipt.IssuedAt)
		if err != nil {
			return selfdevprotocol.CheckpointResponse{}, err
		}
		verifierKeyReceiptJSON, err = verifierKeyReceipt.CanonicalBytes()
		if err != nil {
			return selfdevprotocol.CheckpointResponse{}, err
		}
		verifierKeyReceiptDigest = computerevent.DigestBytes(verifierKeyReceiptJSON)
	}
	storageRef := filepath.Join("computer-checkpoints", checkpoint.Digest[:2], checkpoint.Digest+".json")
	if err := a.service.writeBlob(storageRef, artifact); err != nil {
		return selfdevprotocol.CheckpointResponse{}, err
	}
	tx, err := a.cas.store.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return selfdevprotocol.CheckpointResponse{}, err
	}
	defer tx.Rollback()
	if request.VerifierTrustBootstrap && errors.Is(keyLookupErr, sql.ErrNoRows) {
		if _, err = tx.ExecContext(ctx, `INSERT INTO control_key_history (signer_domain,computer_id,key_id,public_key,status,activation_sequence,activation_time,first_invalid_sequence,first_invalid_time,replacement_key_id,authorizing_receipt_json,authorizing_receipt_digest,inserted_at) VALUES ('verifier-control',?,?,?,'active',?,?,NULL,NULL,NULL,?,?,?)`,
			request.ComputerID, verifierKeyID, verifierKey, head.Sequence, receipt.IssuedAt, string(verifierKeyReceiptJSON), verifierKeyReceiptDigest, receipt.IssuedAt); err != nil {
			return selfdevprotocol.CheckpointResponse{}, err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO computer_checkpoints (computer_id,idempotency_key,request_commitment,checkpoint_digest,checkpoint_artifact_ref,checkpoint_json,receipt_json,receipt_digest,created_at) VALUES (?,?,?,?,?,?,?,?,?)`, request.ComputerID, request.IdempotencyKey, requestCommitment, checkpoint.Digest, "artifact://sha256/"+checkpoint.Digest, checkpointJSON, receiptJSON, receiptDigest, receipt.IssuedAt)
	if err != nil {
		return selfdevprotocol.CheckpointResponse{}, err
	}
	if err := tx.Commit(); err != nil {
		return selfdevprotocol.CheckpointResponse{}, err
	}
	if err := a.cas.store.commitDolt(ctx, "publish computer checkpoint "+checkpoint.Digest); err != nil {
		return selfdevprotocol.CheckpointResponse{}, err
	}
	return response, nil
}

func (a *CheckpointAuthority) verifyVerifierEvidence(ctx context.Context, request selfdevprotocol.CheckpointRequest) error {
	certificate := request.VerifierCertificate.Request
	if certificate.Decision == "genesis_baseline" {
		var eventKind string
		err := a.cas.store.db.QueryRowContext(ctx, `SELECT event_kind FROM computer_event_append_receipts WHERE computer_id=? AND event_digest=?`, request.ComputerID, certificate.VerificationEventDigest).Scan(&eventKind)
		if err != nil || eventKind != string(computerevent.EventGenesisImported) || certificate.VerificationEventDigest != certificate.DecisionEventHead {
			return fmt.Errorf("checkpoint authority: genesis verifier evidence refused")
		}
		rawEvent, err := os.ReadFile(filepath.Join(a.service.artifactsRoot, "sha256", "computer-event", certificate.VerificationEventDigest))
		var event computerevent.Event
		if err != nil || computerevent.DigestBytes(rawEvent) != certificate.VerificationEventDigest || json.Unmarshal(rawEvent, &event) != nil {
			return fmt.Errorf("checkpoint authority: genesis event artifact refused")
		}
		publicKey, _ := base64.RawStdEncoding.DecodeString(request.VerifierCertificate.PublicKey)
		keyID := request.VerifierCertificate.Certificate.RequiredSigners[0].KeyID
		verifierRef := "verifier-key:" + keyID + ":sha256:" + computerevent.DigestBytes(publicKey)
		releaseRef := "release:sha256:" + request.ReleaseDigest
		hasVerifier, hasUpdater, hasRelease := false, false, false
		for _, ref := range event.VerifierRefs {
			hasVerifier = hasVerifier || ref == verifierRef
			hasUpdater = hasUpdater || strings.HasPrefix(ref, "updater-key:")
			hasRelease = hasRelease || ref == releaseRef
		}
		if !hasVerifier || !hasUpdater || !hasRelease {
			return fmt.Errorf("checkpoint authority: genesis signing key or release binding refused")
		}
		return nil
	}
	var eventKind string
	err := a.cas.store.db.QueryRowContext(ctx, `SELECT event_kind FROM computer_event_append_receipts WHERE computer_id=? AND event_digest=?`, request.ComputerID, certificate.VerificationEventDigest).Scan(&eventKind)
	if err != nil || eventKind != string(computerevent.EventVerificationRecorded) {
		return fmt.Errorf("checkpoint authority: verifier event is unavailable")
	}
	eventPath := filepath.Join(a.service.artifactsRoot, "sha256", "computer-event", certificate.VerificationEventDigest)
	rawEvent, err := os.ReadFile(eventPath)
	if err != nil || computerevent.DigestBytes(rawEvent) != certificate.VerificationEventDigest {
		return fmt.Errorf("checkpoint authority: verifier event artifact refused")
	}
	var event computerevent.Event
	if json.Unmarshal(rawEvent, &event) != nil || event.EventKind != computerevent.EventVerificationRecorded ||
		event.ActorProfile != "co-super" || event.AuthorityRef != "guest-core:self-development-verifier" || len(event.OutputArtifactRefs) != 1 {
		return fmt.Errorf("checkpoint authority: verifier event authority mismatch")
	}
	payloadRef, err := computerevent.ParseArtifactRef(event.OutputArtifactRefs[0])
	if err != nil {
		return fmt.Errorf("checkpoint authority: verifier payload reference refused")
	}
	payloadDigest := payloadRef.Digest().String()
	rawPayload, err := os.ReadFile(filepath.Join(a.service.artifactsRoot, "sha256", "computer-event-payload", payloadDigest))
	if err != nil || computerevent.DigestBytes(rawPayload) != payloadDigest {
		return fmt.Errorf("checkpoint authority: verifier payload unavailable")
	}
	var payload struct {
		SchemaVersion int      `json:"schema_version"`
		OperationID   string   `json:"operation_id"`
		BundleDigest  string   `json:"bundle_digest"`
		Decision      string   `json:"decision"`
		VerifierRefs  []string `json:"verifier_refs"`
		VerifierRunID string   `json:"verifier_run_id"`
	}
	if json.Unmarshal(rawPayload, &payload) != nil || payload.SchemaVersion != 1 || payload.Decision != "pass" ||
		payload.OperationID != certificate.OperationID || payload.BundleDigest != certificate.BundleDigest {
		return fmt.Errorf("checkpoint authority: verifier decision mismatch")
	}
	expectedRefs, _ := computerevent.CanonicalJSON(certificate.VerifierEvidenceRefs)
	actualRefs, _ := computerevent.CanonicalJSON(payload.VerifierRefs)
	if !bytes.Equal(expectedRefs, actualRefs) || payload.VerifierRunID == "" {
		return fmt.Errorf("checkpoint authority: verifier evidence references mismatch")
	}
	return nil
}

func (a *CheckpointAuthority) lookup(ctx context.Context, computerID, idempotencyKey string) (selfdevprotocol.CheckpointResponse, bool, error) {
	var checkpointJSON, receiptJSON string
	err := a.cas.store.db.QueryRowContext(ctx, `SELECT checkpoint_json,receipt_json FROM computer_checkpoints WHERE computer_id=? AND idempotency_key=?`, computerID, idempotencyKey).Scan(&checkpointJSON, &receiptJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return selfdevprotocol.CheckpointResponse{}, false, nil
	}
	if err != nil {
		return selfdevprotocol.CheckpointResponse{}, false, err
	}
	var response selfdevprotocol.CheckpointResponse
	if json.Unmarshal([]byte(checkpointJSON), &response.Checkpoint) != nil || json.Unmarshal([]byte(receiptJSON), &response.Receipt) != nil {
		return selfdevprotocol.CheckpointResponse{}, false, fmt.Errorf("checkpoint authority: stored response is invalid")
	}
	return response, true, nil
}

func (a *CheckpointAuthority) PublishRouteProjection(ctx context.Context, request selfdevprotocol.RouteProjectionRequest) (selfdevprotocol.RouteProjectionResponse, error) {
	requestCommitment, err := selfdevprotocol.Digest(request)
	if err != nil {
		return selfdevprotocol.RouteProjectionResponse{}, err
	}
	lock := &a.cas.locks[computerEventLockIndex(request.ComputerID)]
	lock.Lock()
	defer lock.Unlock()
	if replay, ok, err := a.lookupRouteProjection(ctx, request.ComputerID, request.IdempotencyKey); err != nil {
		return selfdevprotocol.RouteProjectionResponse{}, err
	} else if ok {
		if replay.Receipt.RequestCommitment != requestCommitment {
			return selfdevprotocol.RouteProjectionResponse{}, ErrComputerEventCASConflict
		}
		return replay, nil
	}
	publicKey := a.cas.signingKey.PrivateKey.Public().(ed25519.PublicKey)
	if request.Checkpoint.Receipt.Verify(publicKey) != nil {
		return selfdevprotocol.RouteProjectionResponse{}, fmt.Errorf("route projection authority: checkpoint signature refused")
	}
	checkpointReceiptDigest, err := selfdevprotocol.Digest(request.Checkpoint.Receipt)
	if err != nil {
		return selfdevprotocol.RouteProjectionResponse{}, err
	}
	var persistedCheckpointDigest, persistedReceiptDigest string
	err = a.cas.store.db.QueryRowContext(ctx, `SELECT checkpoint_digest,receipt_digest FROM computer_checkpoints WHERE computer_id=? AND checkpoint_digest=?`, request.ComputerID, request.Checkpoint.Checkpoint.Digest).Scan(&persistedCheckpointDigest, &persistedReceiptDigest)
	if err != nil || persistedCheckpointDigest != request.Checkpoint.Checkpoint.Digest || persistedReceiptDigest != checkpointReceiptDigest {
		return selfdevprotocol.RouteProjectionResponse{}, fmt.Errorf("route projection authority: checkpoint is not durably published")
	}
	head, err := readComputerEventHead(ctx, a.cas.store.db, request.ComputerID, false)
	if err != nil || head.CanonicalEventHead != request.CanonicalEventHead || head.EffectiveEventHead != request.Checkpoint.Checkpoint.Request.EffectiveEventHead || head.PendingTransitionRef != "" {
		return selfdevprotocol.RouteProjectionResponse{}, fmt.Errorf("route projection authority: checkpoint is not the current effective head")
	}
	var routeEventDigest string
	if err := a.cas.store.db.QueryRowContext(ctx, `SELECT event_digest FROM computer_event_append_receipts WHERE computer_id=? AND event_head_receipt_id=?`, request.ComputerID, request.EventHeadReceiptID).Scan(&routeEventDigest); err != nil || routeEventDigest != request.CanonicalEventHead {
		return selfdevprotocol.RouteProjectionResponse{}, fmt.Errorf("route projection authority: current event head receipt is unavailable")
	}
	certificate, artifact, err := selfdevprotocol.RouteProjectionFromRequest(request, a.now())
	if err != nil {
		return selfdevprotocol.RouteProjectionResponse{}, err
	}
	certificateDigest := computerevent.DigestBytes(artifact)
	receipt, err := selfdevprotocol.NewAuthorityReceipt(selfdevprotocol.ReceiptKindRouteProjection, request.ComputerID, requestCommitment, certificateDigest, a.cas.issuer, a.cas.signingKey, a.now())
	if err != nil {
		return selfdevprotocol.RouteProjectionResponse{}, err
	}
	certificateJSON, _ := json.Marshal(certificate)
	receiptJSON, _ := json.Marshal(receipt)
	receiptDigest, _ := selfdevprotocol.Digest(receipt)
	expiresAt, _ := time.Parse(time.RFC3339Nano, request.ExpiresAt)
	_, err = a.cas.store.db.ExecContext(ctx, `INSERT INTO computer_route_projection_certificates (computer_id,idempotency_key,request_commitment,certificate_digest,certificate_json,receipt_json,receipt_digest,expires_at,created_at) VALUES (?,?,?,?,?,?,?,?,?)`, request.ComputerID, request.IdempotencyKey, requestCommitment, certificateDigest, certificateJSON, receiptJSON, receiptDigest, expiresAt, receipt.IssuedAt)
	if err != nil {
		return selfdevprotocol.RouteProjectionResponse{}, err
	}
	if err := a.cas.store.commitDolt(ctx, "publish route projection certificate "+certificateDigest); err != nil {
		return selfdevprotocol.RouteProjectionResponse{}, err
	}
	return selfdevprotocol.RouteProjectionResponse{Certificate: certificate, Receipt: receipt}, nil
}

func (a *CheckpointAuthority) lookupRouteProjection(ctx context.Context, computerID, idempotencyKey string) (selfdevprotocol.RouteProjectionResponse, bool, error) {
	var certificateJSON, receiptJSON string
	err := a.cas.store.db.QueryRowContext(ctx, `SELECT certificate_json,receipt_json FROM computer_route_projection_certificates WHERE computer_id=? AND idempotency_key=?`, computerID, idempotencyKey).Scan(&certificateJSON, &receiptJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return selfdevprotocol.RouteProjectionResponse{}, false, nil
	}
	if err != nil {
		return selfdevprotocol.RouteProjectionResponse{}, false, err
	}
	var response selfdevprotocol.RouteProjectionResponse
	if json.Unmarshal([]byte(certificateJSON), &response.Certificate) != nil || json.Unmarshal([]byte(receiptJSON), &response.Receipt) != nil {
		return selfdevprotocol.RouteProjectionResponse{}, false, fmt.Errorf("route projection authority: stored response is invalid")
	}
	return response, true, nil
}

func (h *Handler) HandleRouteProjectionCertificate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if h == nil || h.checkpointAuthority == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "route projection authority unavailable"})
		return
	}
	var request selfdevprotocol.RouteProjectionRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid route projection request"})
		return
	}
	if !h.authorizeComputerEvent(w, r, request.ComputerID, "event:append") {
		return
	}
	response, err := h.checkpointAuthority.PublishRouteProjection(r.Context(), request)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ErrComputerEventCASConflict) {
			status = http.StatusConflict
		}
		writeJSON(w, status, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *Handler) HandlePlatformControlPublicKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if r.Header.Get("X-Internal-Caller") != "true" || h == nil || h.eventCAS == nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal authorization required"})
		return
	}
	publicKey := h.eventCAS.signingKey.PrivateKey.Public().(ed25519.PublicKey)
	writeJSON(w, http.StatusOK, map[string]string{
		"signer_domain": h.eventCAS.signingKey.SignerDomain,
		"key_id":        h.eventCAS.signingKey.KeyID,
		"public_key":    base64.RawStdEncoding.EncodeToString(publicKey),
	})
}

func (h *Handler) HandleComputerCheckpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if h == nil || h.checkpointAuthority == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "checkpoint authority unavailable"})
		return
	}
	var request selfdevprotocol.CheckpointRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid checkpoint request"})
		return
	}
	if !h.authorizeComputerEvent(w, r, request.ComputerID, "event:append") {
		return
	}
	response, err := h.checkpointAuthority.Publish(r.Context(), request)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ErrComputerEventCASConflict) {
			status = http.StatusConflict
		}
		writeJSON(w, status, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, response)
}
