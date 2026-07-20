package agentcore

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/updater"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

const selfDevelopmentPromptMediaType = "text/markdown; charset=utf-8"

type selfDevelopmentStartRequest struct {
	IdempotencyKey string                 `json:"idempotency_key"`
	Prompt         string                 `json:"prompt"`
	ModeReceipt    *computerevent.Receipt `json:"mode_receipt,omitempty"`
}

type selfDevelopmentDecisionRequest struct {
	Decision                         string                 `json:"decision"`
	IdempotencyKey                   string                 `json:"idempotency_key"`
	BundleDigest                     string                 `json:"bundle_digest"`
	VerifierRef                      string                 `json:"verifier_ref"`
	Reason                           string                 `json:"reason,omitempty"`
	ExpectedDesiredEventHead         string                 `json:"expected_desired_event_head"`
	ExpectedEffectiveEventHead       string                 `json:"expected_effective_event_head"`
	ExpectedPendingTransitionRef     *string                `json:"expected_pending_transition_ref"`
	ExpectedDesiredStateCommitment   string                 `json:"expected_desired_state_commitment"`
	ExpectedEffectiveStateCommitment string                 `json:"expected_effective_state_commitment"`
	ModeReceipt                      *computerevent.Receipt `json:"mode_receipt,omitempty"`
}

type selfDevelopmentGenesisRequest struct {
	BaselineVersion    string `json:"baseline_version"`
	BaselineState      string `json:"baseline_state"`
	ExpectedAbsent     bool   `json:"expected_absent"`
	IdempotencyKey     string `json:"idempotency_key"`
	G0Receipt          string `json:"g0_receipt"`
	G1Receipt          string `json:"g1_receipt"`
	CandidateRef       string `json:"candidate_ref"`
	DeployedReleaseRef string `json:"deployed_release_ref"`
}

type selfDevelopmentRollbackRequest struct {
	ExpectedDesiredHead     string `json:"expected_desired_head"`
	CurrentAppliedHead      string `json:"current_applied_head"`
	ToAppliedHead           string `json:"to_applied_head"`
	PriorMaterialization    string `json:"prior_materialization"`
	PriorCheckpoint         string `json:"prior_checkpoint"`
	ExpectedRouteGeneration uint64 `json:"expected_route_generation"`
	IdempotencyKey          string `json:"idempotency_key"`
}

// HandleComputersRouter serves the public computer-scoped self-development API.
func (h *APIHandler) HandleComputersRouter(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	suffix := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/computers/"), "/")
	parts := strings.Split(suffix, "/")
	if len(parts) < 3 || strings.TrimSpace(parts[0]) == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "computer route not found"})
		return
	}
	h.handleSelfDevelopmentRoute(w, r, ownerID, strings.TrimSpace(parts[0]), parts)
}

func (h *APIHandler) handleSelfDevelopmentRoute(w http.ResponseWriter, r *http.Request, ownerID, computerID string, parts []string) {
	if strings.TrimSpace(r.Header.Get("X-Authenticated-Computer")) != computerID {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "authenticated computer binding required"})
		return
	}
	if h == nil || h.rt == nil || h.rt.selfdevOperations == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "self-development operation authority unavailable"})
		return
	}
	switch {
	case len(parts) == 3 && parts[1] == "self-development" && parts[2] == "kernel-capabilities":
		if r.Method != http.MethodGet {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		h.readKernelCapabilityReceipt(w, r, computerID)
	case len(parts) == 3 && parts[1] == "self-development" && parts[2] == "operations":
		if r.Method != http.MethodPost {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		h.startSelfDevelopmentOperation(w, r, ownerID, computerID)
	case len(parts) == 3 && parts[1] == "self-development" && parts[2] == "genesis":
		if r.Method != http.MethodPost {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		h.importSelfDevelopmentGenesis(w, r, ownerID, computerID)
	case len(parts) == 3 && parts[1] == "self-development" && parts[2] == "rollbacks":
		if r.Method != http.MethodPost {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		h.startSelfDevelopmentRollback(w, r, ownerID, computerID)
	case len(parts) == 5 && parts[1] == "self-development" && parts[2] == "operations" && parts[4] == "decision":
		if r.Method != http.MethodPost {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		h.decideSelfDevelopmentOperation(w, r, ownerID, computerID, strings.TrimSpace(parts[3]))
	case len(parts) == 5 && parts[1] == "self-development" && parts[2] == "operations" && parts[4] == "receipts":
		if r.Method != http.MethodGet {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		operation, err := h.rt.selfdevOperations.Get(r.Context(), computerID, strings.TrimSpace(parts[3]))
		if err != nil {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "self-development operation not found"})
			return
		}
		writeAPIJSON(w, http.StatusOK, operationReceiptProjection(operation))
	case len(parts) == 4 && parts[1] == "self-development" && parts[2] == "operations":
		if r.Method != http.MethodGet {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		operation, err := h.rt.selfdevOperations.Get(r.Context(), computerID, strings.TrimSpace(parts[3]))
		if err != nil {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "self-development operation not found"})
			return
		}
		writeAPIJSON(w, http.StatusOK, operation)
	case len(parts) == 3 && parts[1] == "events":
		if r.Method != http.MethodGet || !computerevent.IsSHA256(strings.TrimSpace(parts[2])) {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "computer event not found"})
			return
		}
		event, found, err := h.rt.store.EventByDigest(r.Context(), computerID, strings.TrimSpace(parts[2]))
		if err != nil || !found {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "computer event not found"})
			return
		}
		writeAPIJSON(w, http.StatusOK, event)
	default:
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "computer route not found"})
	}
}

func (h *APIHandler) startSelfDevelopmentOperation(w http.ResponseWriter, r *http.Request, ownerID, computerID string) {
	var request selfDevelopmentStartRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || strings.TrimSpace(request.IdempotencyKey) == "" || strings.TrimSpace(request.Prompt) == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "idempotency_key and prompt are required"})
		return
	}
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	prompt := []byte(request.Prompt)
	promptDigest := computerevent.DigestBytes(prompt)
	requestCommitment := computerevent.DigestBytes([]byte(computerID + "\x00" + request.IdempotencyKey + "\x00" + promptDigest))
	if existing, found, err := h.rt.selfdevOperations.GetByIdempotency(r.Context(), computerID, request.IdempotencyKey); err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to read self-development operation"})
		return
	} else if found {
		if existing.RequestCommitment != requestCommitment {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "idempotency key reused with different prompt"})
			return
		}
		writeAPIJSON(w, http.StatusOK, existing)
		return
	}
	identityDigest := computerevent.DigestBytes([]byte(computerID + "\x00" + request.IdempotencyKey))
	operationID := "selfdev-" + identityDigest[:32]
	trajectoryID := "trajectory-" + identityDigest[32:]
	eventIdempotency := "selfdev-start-" + identityDigest
	event, found, err := h.rt.store.EventByIdempotency(r.Context(), computerID, eventIdempotency)
	if err != nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: err.Error()})
		return
	}
	if found {
		promptArtifactRef, recoveryErr := recoveredStartPromptRef(event, computerID, trajectoryID, eventIdempotency, ownerID, requestCommitment)
		if recoveryErr != nil {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: recoveryErr.Error()})
			return
		}
		if recoveryErr = h.rt.selfdevOperations.BindStartIntent(r.Context(), computerID, request.IdempotencyKey, requestCommitment); recoveryErr != nil {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: recoveryErr.Error()})
			return
		}
		operation, recoveryErr := h.rt.selfdevOperations.Start(r.Context(), selfdev.StartRequest{
			ComputerID: computerID, IdempotencyKey: request.IdempotencyKey, PromptArtifactRef: promptArtifactRef,
			OperationID: operationID, TrajectoryID: trajectoryID, BaseHead: event.PreviousHead, RequestCommitment: requestCommitment,
		})
		if recoveryErr != nil {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: recoveryErr.Error()})
			return
		}
		operation, recoveryErr = h.ensureSelfDevelopmentRun(r, operation, ownerID, request.Prompt)
		if recoveryErr != nil {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: recoveryErr.Error()})
			return
		}
		writeAPIJSON(w, http.StatusOK, operation)
		return
	}
	if h.rt.selfdevControl == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "self-development mode authority unavailable"})
		return
	}
	currentMode, modeErr := h.rt.selfdevControl.SelfDevelopmentMode(r.Context())
	if modeErr != nil || h.verifyStartModeReceipt(computerID, currentMode.Mode, currentMode.Receipt, request.ModeReceipt) != nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "current signed mode does not authorize proposal"})
		return
	}
	if err := h.rt.selfdevOperations.BindStartIntent(r.Context(), computerID, request.IdempotencyKey, requestCommitment); err != nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: err.Error()})
		return
	}
	if h.rt.eventAppender == nil || h.rt.privateArtifactCipher == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "computer event authority unavailable"})
		return
	}

	if !found {
		eventID, eventErr := computerevent.NewEventID()
		if eventErr != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create trajectory event"})
			return
		}
		event = computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1,
			EventID:       eventID, ComputerID: computerID, EventKind: computerevent.EventTrajectoryStarted,
			OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), IdempotencyKey: eventIdempotency,
			RequestCommitment: computerevent.ZeroHead, TrajectoryID: trajectoryID,
			ActorProfile: "super", AuthorityRef: "public-self-development-api:" + ownerID,
			PrivacyClass: "private", ReducerVersion: computerevent.ReducerVersionV1,
			DecisionRef: requestCommitment,
		}
		if _, pinnedDigest, appendErr := h.rt.eventAppender.AppendNewPrivatePayload(r.Context(), event, computerevent.TransitionInput{}, prompt, selfDevelopmentPromptMediaType, h.rt.privateArtifactCipher); appendErr != nil {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: fmt.Sprintf("append self-development trajectory: %v", appendErr)})
			return
		} else if pinnedDigest != promptDigest {
			// The durable reference names the authenticated encrypted envelope. The
			// plaintext digest is never exposed as an artifact reference.
			promptDigest = pinnedDigest
		}
		event, found, err = h.rt.store.EventByIdempotency(r.Context(), computerID, eventIdempotency)
		if err != nil || !found {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "trajectory event projection unavailable"})
			return
		}
	}
	promptDigest, err = recoveredStartPromptRef(event, computerID, trajectoryID, eventIdempotency, ownerID, requestCommitment)
	if err != nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: err.Error()})
		return
	}

	operation, err := h.rt.selfdevOperations.Start(r.Context(), selfdev.StartRequest{
		ComputerID: computerID, IdempotencyKey: request.IdempotencyKey, PromptArtifactRef: promptDigest,
		OperationID: operationID, TrajectoryID: trajectoryID, BaseHead: event.PreviousHead, RequestCommitment: requestCommitment,
	})
	if err != nil {
		status := http.StatusConflict
		if errors.Is(err, selfdev.ErrInvalidTransition) {
			status = http.StatusBadRequest
		}
		writeAPIJSON(w, status, apiError{Error: err.Error()})
		return
	}
	operation, err = h.ensureSelfDevelopmentRun(r, operation, ownerID, request.Prompt)
	if err != nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusCreated, operation)
}

func recoveredStartPromptRef(event computerevent.Event, computerID, trajectoryID, eventIdempotency, ownerID, requestCommitment string) (string, error) {
	if event.SchemaVersion != computerevent.SchemaVersionV1 || event.ComputerID != computerID ||
		event.EventKind != computerevent.EventTrajectoryStarted || event.TrajectoryID != trajectoryID ||
		event.IdempotencyKey != eventIdempotency || event.DecisionRef != requestCommitment ||
		event.AuthorityRef != "public-self-development-api:"+ownerID || event.PrivacyClass != "private" ||
		len(event.OutputArtifactRefs) != 1 {
		return "", fmt.Errorf("durable trajectory event binding mismatch")
	}
	ref, err := computerevent.ParseArtifactRef(event.OutputArtifactRefs[0])
	if err != nil {
		return "", fmt.Errorf("durable trajectory event artifact binding mismatch")
	}
	return ref.String(), nil
}

func operationReceiptProjection(operation selfdev.Operation) map[string]any {
	return map[string]any{
		"operation_id": operation.OperationID, "request_commitment": operation.RequestCommitment,
		"computer_id": operation.ComputerID, "trajectory_id": operation.TrajectoryID, "capsule_id": operation.CapsuleID,
		"base_head": operation.BaseHead, "bundle_digest": operation.BundleDigest, "verifier_refs": operation.VerifierRefs,
		"decision_actor": operation.DecisionActor, "decision_event": operation.DecisionEvent, "decision_receipt": operation.DecisionReceipt,
		"desired_head": operation.DesiredHead, "effective_head": operation.EffectiveHead,
		"materialization_receipt": operation.MaterializationReceipt, "checkpoint_ref": operation.CheckpointRef,
		"route_certificate": operation.RouteCertificate, "route_generation": operation.RouteGeneration,
		"mode_receipt": operation.ModeReceipt, "lifecycle_receipt": operation.LifecycleReceipt,
		"terminal_state": operation.State, "error": operation.TerminalError,
	}
}

func (h *APIHandler) importSelfDevelopmentGenesis(w http.ResponseWriter, r *http.Request, ownerID, computerID string) {
	var request selfDevelopmentGenesisRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid genesis request"})
		return
	}
	request.BaselineVersion, request.BaselineState, request.IdempotencyKey = strings.TrimSpace(request.BaselineVersion), strings.TrimSpace(request.BaselineState), strings.TrimSpace(request.IdempotencyKey)
	request.G0Receipt, request.G1Receipt, request.CandidateRef, request.DeployedReleaseRef = strings.TrimSpace(request.G0Receipt), strings.TrimSpace(request.G1Receipt), strings.TrimSpace(request.CandidateRef), strings.TrimSpace(request.DeployedReleaseRef)
	if !request.ExpectedAbsent || !computerevent.IsSHA256(request.BaselineVersion) || !computerevent.IsSHA256(request.BaselineState) || request.IdempotencyKey == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "exact absent baseline version, state, and idempotency binding are required"})
		return
	}
	expectedG0 := strings.TrimSpace(os.Getenv("CHOIR_SELF_DEVELOPMENT_G0_RECEIPT"))
	expectedG1 := strings.TrimSpace(os.Getenv("CHOIR_SELF_DEVELOPMENT_G1_RECEIPT"))
	expectedCandidate := strings.TrimSpace(os.Getenv("CHOIR_SELF_DEVELOPMENT_G1_CANDIDATE_REF"))
	if r.Header.Get("X-Self-Development-Disposable") != "true" || r.Header.Get("X-Self-Development-Mode-Generation") != "0" {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "genesis requires disposable target and absent off-mode state"})
		return
	}
	genesisAuthorityRef, genesisAuthorityPayload, err := selfDevelopmentGenesisAuthorityRef(request, expectedG0, expectedG1, expectedCandidate, strings.TrimSpace(buildinfo.Commit))
	if err != nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "genesis requires exact frozen G0/G1 candidate and deployed release bindings"})
		return
	}
	eventIdempotency := "selfdev-genesis-" + computerevent.DigestBytes([]byte(computerID+"\x00"+request.IdempotencyKey))
	if existingEvent, found, lookupErr := h.rt.store.EventByIdempotency(r.Context(), computerID, eventIdempotency); lookupErr == nil && found {
		if existingEvent.ProposedEffectRef != request.BaselineVersion || existingEvent.ResultingEffectiveCommitment != request.BaselineState || !selfDevelopmentContainsString(existingEvent.InputArtifactRefs, genesisAuthorityRef) {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "genesis idempotency binding changed"})
			return
		}
		if baseline, baselineFound, baselineErr := h.rt.selfdevOperations.GetByIdempotency(r.Context(), computerID, "genesis-baseline-"+eventIdempotency); baselineErr == nil && baselineFound {
			head, _ := h.rt.store.Head(r.Context(), computerID)
			writeAPIJSON(w, http.StatusOK, map[string]any{"event": existingEvent, "head": head, "baseline": baseline})
			return
		}
	}
	if h.rt.selfdevRoute == nil || h.rt.selfdevUpdater == nil || h.rt.selfdevVerifier == nil || h.rt.selfdevControl == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "genesis reconstruction authority unavailable"})
		return
	}
	routeSlotID, routeSlotErr := routeledger.RouteSlotID(h.rt.selfdevRouteOwnerID, h.rt.selfdevRouteDesktopID)
	if routeSlotErr != nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "genesis route identity unavailable"})
		return
	}
	route, routeErr := h.rt.selfdevRoute.ResolveComputerVersionRoute(r.Context(), routeSlotID)
	if routeErr != nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "genesis route is unavailable"})
		return
	}
	versionDigest, digestErr := selfdevprotocol.Digest(route.Slot.Current)
	manifest, manifestErr := updater.ReadCurrentManifest(h.rt.selfdevUpdaterRoot)
	if manifestErr != nil {
		baselineRoot := filepath.Clean(strings.TrimSpace(os.Getenv("CHOIR_BASELINE_RELEASE_ROOT")))
		if strings.HasPrefix(baselineRoot, "/nix/store/") {
			manifest, manifestErr = updater.BuildBaselineManifest(baselineRoot, computerID, string(route.Slot.Current.CodeRef), string(route.Slot.Current.ArtifactProgramRef))
			if manifestErr == nil {
				importRequest := updater.BaselineImportRequest{
					ComputerID: computerID, RealizationID: h.rt.selfdevRealizationID,
					IdempotencyKey: "genesis-baseline-" + request.IdempotencyKey, SourceDir: baselineRoot, Manifest: manifest,
				}
				importRequest.RequestCommitment, manifestErr = updater.ComputeBaselineImportCommitment(importRequest)
				if manifestErr == nil {
					manifest, manifestErr = h.rt.selfdevUpdater.ImportBaseline(r.Context(), importRequest)
				}
			}
		}
	}
	if digestErr != nil || manifestErr != nil || versionDigest != request.BaselineVersion ||
		manifest.ContentDigest == "" || manifest.CodeRef != string(route.Slot.Current.CodeRef) || manifest.ArtifactProgramRef != string(route.Slot.Current.ArtifactProgramRef) {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "genesis baseline does not match the current immutable release and route"})
		return
	}
	updaterSigner, updaterPublicKey, updaterKeyErr := h.rt.selfdevUpdater.PublicKey(r.Context())
	verifierSigner, verifierPublicKey, verifierKeyErr := h.rt.selfdevVerifier.PublicKey(r.Context())
	if updaterKeyErr != nil || verifierKeyErr != nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "genesis guest signing keys unavailable"})
		return
	}
	genesisKeyRefs := []string{
		"updater-key:" + updaterSigner.KeyID + ":sha256:" + computerevent.DigestBytes(updaterPublicKey),
		"verifier-key:" + verifierSigner.KeyID + ":sha256:" + computerevent.DigestBytes(verifierPublicKey),
		"release:sha256:" + manifest.ContentDigest,
	}
	if existing, found, err := h.rt.store.EventByIdempotency(r.Context(), computerID, eventIdempotency); err != nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: err.Error()})
		return
	} else if found {
		if existing.ProposedEffectRef != request.BaselineVersion || existing.ResultingEffectiveCommitment != request.BaselineState || !selfDevelopmentContainsString(existing.InputArtifactRefs, genesisAuthorityRef) {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "genesis idempotency binding changed"})
			return
		}
		baseline, checkpoint, baselineErr := h.recordGenesisBaseline(r.Context(), request, existing, eventIdempotency, route, manifest)
		if baselineErr != nil {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: baselineErr.Error()})
			return
		}
		head, _ := h.rt.store.Head(r.Context(), computerID)
		writeAPIJSON(w, http.StatusOK, map[string]any{"event": existing, "head": head, "checkpoint": checkpoint, "baseline": baseline})
		return
	}
	if head, err := h.rt.store.Head(r.Context(), computerID); err != nil || head != nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "GenesisImported requires an absent computer event head"})
		return
	}
	if h.rt.eventAppender == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "computer event authority unavailable"})
		return
	}
	eventID, err := computerevent.NewEventID()
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create genesis event"})
		return
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: computerID,
		EventKind: computerevent.EventGenesisImported, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: eventIdempotency, RequestCommitment: computerevent.ZeroHead,
		ActorProfile: agentprofile.Super, AuthorityRef: "external-owner-genesis:" + ownerID, PrivacyClass: "owner",
		PayloadCommitment: request.BaselineState, ProposedEffectRef: request.BaselineVersion,
		VerifierRefs: genesisKeyRefs, ResultingEffectiveCommitment: request.BaselineState,
		ReducerVersion: computerevent.ReducerVersionV1,
	}
	receipt, artifactDigests, err := h.rt.eventAppender.AppendNewPayloadSet(r.Context(), event, computerevent.TransitionInput{TargetStateCommitment: request.BaselineState}, []computerevent.EventPayload{{
		Content: genesisAuthorityPayload, MediaType: "application/vnd.choir.genesis-authority+json",
		PrivacyClass: "owner", Direction: computerevent.EventPayloadInput,
	}}, nil)
	if err == nil && (len(artifactDigests) != 1 || genesisAuthorityRef != "artifact:sha256:"+artifactDigests[0]) {
		err = fmt.Errorf("genesis authority artifact binding mismatch")
	}
	if err != nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: err.Error()})
		return
	}
	event, found, eventErr := h.rt.store.EventByIdempotency(r.Context(), computerID, eventIdempotency)
	if eventErr != nil || !found {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "genesis event projection unavailable"})
		return
	}
	baseline, checkpoint, baselineErr := h.recordGenesisBaseline(r.Context(), request, event, eventIdempotency, route, manifest)
	if baselineErr != nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: baselineErr.Error()})
		return
	}
	head, err := h.rt.store.Head(r.Context(), computerID)
	if err != nil || head == nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "genesis head projection unavailable"})
		return
	}
	writeAPIJSON(w, http.StatusCreated, map[string]any{"receipt": receipt, "head": head, "checkpoint": checkpoint, "baseline": baseline})
}
func selfDevelopmentGenesisAuthorityRef(request selfDevelopmentGenesisRequest, expectedG0, expectedG1, expectedCandidate, deployedRelease string) (string, []byte, error) {
	if expectedG0 == "" || expectedG1 == "" || expectedCandidate == "" || deployedRelease == "" || deployedRelease == "local" ||
		request.G0Receipt != expectedG0 || request.G1Receipt != expectedG1 || request.CandidateRef != expectedCandidate ||
		request.DeployedReleaseRef != deployedRelease {
		return "", nil, fmt.Errorf("genesis authority identity mismatch")
	}
	canonical, err := computerevent.CanonicalJSON(map[string]string{
		"g0_receipt": request.G0Receipt, "g1_receipt": request.G1Receipt,
		"candidate_ref": request.CandidateRef, "deployed_release_ref": request.DeployedReleaseRef,
	})
	if err != nil {
		return "", nil, err
	}
	ref, err := computerevent.ArtifactRefFromDigest(computerevent.DigestBytes(canonical))
	if err != nil {
		return "", nil, err
	}
	return ref.String(), canonical, nil
}

func (h *APIHandler) recordGenesisBaseline(ctx context.Context, request selfDevelopmentGenesisRequest, event computerevent.Event, eventIdempotency string, route vmctl.RouteResolution, manifest updater.ReleaseManifest) (selfdev.Operation, selfdevprotocol.CheckpointResponse, error) {
	eventDigest, err := event.Digest()
	if err != nil {
		return selfdev.Operation{}, selfdevprotocol.CheckpointResponse{}, err
	}
	eventReceipt, found, err := h.rt.store.EventReceiptByIdempotency(ctx, event.ComputerID, eventIdempotency)
	if err != nil || !found {
		return selfdev.Operation{}, selfdevprotocol.CheckpointResponse{}, fmt.Errorf("genesis event receipt unavailable")
	}
	head, err := h.rt.store.Head(ctx, event.ComputerID)
	if err != nil || head == nil || head.EffectiveEventHead != eventDigest {
		return selfdev.Operation{}, selfdevprotocol.CheckpointResponse{}, fmt.Errorf("genesis effective head mismatch")
	}
	updaterRef, updaterKey, err := h.rt.selfdevUpdater.PublicKey(ctx)
	if err != nil {
		return selfdev.Operation{}, selfdevprotocol.CheckpointResponse{}, err
	}
	verifierRef, verifierKey, err := h.rt.selfdevVerifier.PublicKey(ctx)
	if err != nil || !selfDevelopmentContainsString(event.VerifierRefs, "updater-key:"+updaterRef.KeyID+":sha256:"+computerevent.DigestBytes(updaterKey)) ||
		!selfDevelopmentContainsString(event.VerifierRefs, "verifier-key:"+verifierRef.KeyID+":sha256:"+computerevent.DigestBytes(verifierKey)) ||
		!selfDevelopmentContainsString(event.VerifierRefs, "release:sha256:"+manifest.ContentDigest) {
		return selfdev.Operation{}, selfdevprotocol.CheckpointResponse{}, fmt.Errorf("genesis signing key and release bindings mismatch")
	}
	materializationDigest, err := selfdevprotocol.Digest(struct {
		ReleaseDigest string                  `json:"release_digest"`
		Updater       computerevent.SignerRef `json:"updater"`
		PublicKey     []byte                  `json:"public_key"`
	}{manifest.ContentDigest, updaterRef, updaterKey})
	if err != nil {
		return selfdev.Operation{}, selfdevprotocol.CheckpointResponse{}, err
	}
	verifierCertificate, err := h.rt.selfdevVerifier.SignVerifierCertificate(ctx, selfdevprotocol.VerifierCertificateRequest{
		Version: 1, ComputerID: event.ComputerID, OperationID: "genesis-baseline",
		BundleDigest: manifest.ContentDigest, VerificationEventDigest: eventDigest,
		VerifierEvidenceRefs: []string{eventDigest}, DecisionEventHead: eventDigest,
		CodeRef: string(route.Slot.Current.CodeRef), ArtifactProgramRef: string(route.Slot.Current.ArtifactProgramRef),
		ReleaseDigest: manifest.ContentDigest, Decision: "genesis_baseline",
	})
	if err != nil {
		return selfdev.Operation{}, selfdevprotocol.CheckpointResponse{}, err
	}
	if len(verifierCertificate.Certificate.RequiredSigners) != 1 ||
		verifierCertificate.PublicKey != base64.RawStdEncoding.EncodeToString(verifierKey) ||
		verifierCertificate.Certificate.RequiredSigners[0].KeyID != verifierRef.KeyID {
		return selfdev.Operation{}, selfdevprotocol.CheckpointResponse{}, fmt.Errorf("genesis verifier certificate key changed")
	}
	verifierJSON, err := computerevent.CanonicalJSON(verifierCertificate.Certificate)
	if err != nil {
		return selfdev.Operation{}, selfdevprotocol.CheckpointResponse{}, err
	}
	verifierDigest := computerevent.DigestBytes(verifierJSON)
	reconstructionDigest, err := selfdevprotocol.Digest(struct {
		Version       any    `json:"computer_version"`
		EffectiveHead string `json:"effective_event_head"`
		ReleaseDigest string `json:"release_digest"`
	}{route.Slot.Current, eventDigest, manifest.ContentDigest})
	if err != nil {
		return selfdev.Operation{}, selfdevprotocol.CheckpointResponse{}, err
	}
	checkpoint, err := h.rt.selfdevControl.PublishCheckpoint(ctx, selfdevprotocol.CheckpointRequest{
		ComputerID: event.ComputerID, IdempotencyKey: "selfdev-genesis-checkpoint-" + eventDigest,
		ComputerVersion: route.Slot.Current, AcceptedEventHead: eventDigest, EffectiveEventHead: eventDigest,
		EffectiveStateCommitment: request.BaselineState, EventHeadReceiptID: eventReceipt.ReceiptID,
		ReleaseDigest: manifest.ContentDigest, ReconstructionDigest: reconstructionDigest,
		MaterializationReceiptDigest: materializationDigest, VerifierCertificateDigest: verifierDigest,
		VerifierCertificate: verifierCertificate, VerifierTrustBootstrap: true, ReducerVersion: head.ReducerVersion,
	})
	if err != nil {
		return selfdev.Operation{}, selfdevprotocol.CheckpointResponse{}, err
	}
	checkpointRef := "checkpoint:sha256:" + checkpoint.Checkpoint.Digest
	checkpointEventIdempotency := "selfdev-genesis-checkpoint-published-" + eventDigest
	if _, found, lookupErr := h.rt.store.EventByIdempotency(ctx, event.ComputerID, checkpointEventIdempotency); lookupErr != nil {
		return selfdev.Operation{}, selfdevprotocol.CheckpointResponse{}, lookupErr
	} else if !found {
		eventID, eventErr := computerevent.NewEventID()
		if eventErr != nil {
			return selfdev.Operation{}, selfdevprotocol.CheckpointResponse{}, eventErr
		}
		checkpointEvent := computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: event.ComputerID,
			EventKind: computerevent.EventCheckpointPublished, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
			IdempotencyKey: checkpointEventIdempotency, RequestCommitment: computerevent.ZeroHead,
			ActorProfile: agentprofile.Super, AuthorityRef: "platform-control:checkpoint",
			PayloadCommitment: computerevent.ZeroHead, PrivacyClass: "owner",
			ProposedEffectRef: checkpoint.Checkpoint.Digest, DecisionRef: eventDigest, ReducerVersion: computerevent.ReducerVersionV1,
		}
		if _, eventErr = h.rt.eventAppender.AppendNew(ctx, checkpointEvent, computerevent.TransitionInput{}, nil); eventErr != nil {
			return selfdev.Operation{}, selfdevprotocol.CheckpointResponse{}, eventErr
		}
	}
	baseline, err := h.rt.selfdevOperations.RecordAppliedBaseline(ctx, selfdev.BaselineRequest{
		ComputerID: event.ComputerID, IdempotencyKey: "genesis-baseline-" + eventIdempotency,
		EventHead: eventDigest, StateCommitment: request.BaselineState, ReleaseDigest: manifest.ContentDigest,
		CodeRef: string(route.Slot.Current.CodeRef), ArtifactProgramRef: string(route.Slot.Current.ArtifactProgramRef),
		VerifierRefs:           []string{eventDigest},
		MaterializationReceipt: materializationDigest, CheckpointRef: checkpointRef,
		RouteReceipt: string(route.LatestReceipt.ID), RouteGeneration: route.Slot.Generation,
	})
	return baseline, checkpoint, err
}

func (h *APIHandler) decideSelfDevelopmentOperation(w http.ResponseWriter, r *http.Request, ownerID, computerID, operationID string) {
	var request selfDevelopmentDecisionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid self-development decision"})
		return
	}
	request.Decision, request.IdempotencyKey, request.BundleDigest, request.VerifierRef = strings.TrimSpace(request.Decision), strings.TrimSpace(request.IdempotencyKey), strings.TrimSpace(request.BundleDigest), strings.TrimSpace(request.VerifierRef)
	request.ExpectedDesiredEventHead = strings.TrimSpace(request.ExpectedDesiredEventHead)
	request.ExpectedEffectiveEventHead = strings.TrimSpace(request.ExpectedEffectiveEventHead)
	request.ExpectedDesiredStateCommitment = strings.TrimSpace(request.ExpectedDesiredStateCommitment)
	request.ExpectedEffectiveStateCommitment = strings.TrimSpace(request.ExpectedEffectiveStateCommitment)
	if request.ExpectedPendingTransitionRef == nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "expected_pending_transition_ref is required"})
		return
	}
	expectedPendingTransitionRef := strings.TrimSpace(*request.ExpectedPendingTransitionRef)
	if operationID == "" || request.IdempotencyKey == "" || !computerevent.IsSHA256(request.BundleDigest) || request.VerifierRef == "" ||
		!computerevent.IsSHA256(request.ExpectedDesiredEventHead) || !computerevent.IsSHA256(request.ExpectedEffectiveEventHead) ||
		!computerevent.IsSHA256(request.ExpectedDesiredStateCommitment) || !computerevent.IsSHA256(request.ExpectedEffectiveStateCommitment) ||
		(request.Decision != "approve" && request.Decision != "reject") || (request.Decision == "approve" && strings.TrimSpace(request.Reason) != "") || (request.Decision == "reject" && strings.TrimSpace(request.Reason) == "") {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "complete approve or reject decision binding is required"})
		return
	}
	operation, err := h.rt.selfdevOperations.Get(r.Context(), computerID, operationID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "self-development operation not found"})
		return
	}
	if operation.BundleDigest != request.BundleDigest || !selfDevelopmentContainsString(operation.VerifierRefs, request.VerifierRef) {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "decision binding does not match frozen operation"})
		return
	}
	decisionRef, err := selfDevelopmentDecisionRef(request)
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid self-development decision"})
		return
	}
	eventIdempotency := "selfdev-decision-" + computerevent.DigestBytes([]byte(computerID+"\x00"+operationID+"\x00"+request.IdempotencyKey))
	event, found, err := h.rt.store.EventByIdempotency(r.Context(), computerID, eventIdempotency)
	if err != nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: err.Error()})
		return
	}
	nextState := selfdev.StateRejected
	expectedKind := computerevent.EventEffectRejected
	if request.Decision == "approve" {
		nextState = selfdev.StateAccepted
		expectedKind = computerevent.EventEffectAccepted
	}
	modeReceiptDigest := ""
	if request.ModeReceipt != nil {
		modeBytes, modeErr := request.ModeReceipt.CanonicalBytes()
		if modeErr != nil {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "mode receipt encoding failed"})
			return
		}
		modeReceiptDigest = computerevent.DigestBytes(modeBytes)
	}
	if found {
		transition, transitionFound, transitionErr := h.rt.store.FinalizedDecisionForOperation(r.Context(), computerID, operation.OperationID, operation.TrajectoryID, operation.CapsuleID)
		if transitionErr != nil || !transitionFound || transition.Request.EventDigest == "" {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "durable decision transition projection unavailable"})
			return
		}
		decision, bindingErr := verifyFinalizedSelfDevelopmentDecision(operation, transition)
		if bindingErr != nil || decision.NextState != nextState ||
			!exactSelfDevelopmentDecisionRequestMatches(event, computerID, operationID, ownerID, expectedKind, decisionRef, request) {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "decision idempotency key reused with different binding"})
			return
		}
		if operation.State == selfdev.StateAwaitingApproval {
			operation, _, bindingErr = h.rt.recoverSelfDevelopmentDecision(r.Context(), operation)
			if bindingErr != nil {
				writeAPIJSON(w, http.StatusConflict, apiError{Error: bindingErr.Error()})
				return
			}
		}
		writeAPIJSON(w, http.StatusOK, operation)
		return
	}
	if h.rt.selfdevControl == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "self-development mode authority unavailable"})
		return
	}
	currentMode, modeErr := h.rt.selfdevControl.SelfDevelopmentMode(r.Context())
	if modeErr != nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "current signed mode does not authorize this decision"})
		return
	}
	if currentMode.Receipt == nil || currentMode.Receipt.ReceiptKind != "ModeReceipt" ||
		currentMode.Receipt.Verify(h.rt.selfdevControl.KeyResolver()) != nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "current mode receipt is invalid"})
		return
	}
	if request.Decision == "approve" {
		switch currentMode.Mode {
		case platform.SelfDevelopmentModeProposeOnly:
			request.ModeReceipt = currentMode.Receipt
			if h.verifyConsumedModeReceipt(currentMode.Receipt, operationID, request) != nil {
				writeAPIJSON(w, http.StatusConflict, apiError{Error: "current proposal mode does not carry this consumed accept_once decision"})
				return
			}
		case platform.SelfDevelopmentModeAcceptOnce:
			expectedPending := strings.TrimSpace(*request.ExpectedPendingTransitionRef)
			if currentMode.OperationID != operationID || currentMode.BundleDigest != request.BundleDigest ||
				currentMode.ExpectedDesiredEventHead != request.ExpectedDesiredEventHead ||
				currentMode.ExpectedEffectiveEventHead != request.ExpectedEffectiveEventHead ||
				currentMode.ExpectedPendingTransitionRef != expectedPending ||
				currentMode.ExpectedDesiredStateCommitment != request.ExpectedDesiredStateCommitment ||
				currentMode.ExpectedEffectiveStateCommitment != request.ExpectedEffectiveStateCommitment {
				writeAPIJSON(w, http.StatusConflict, apiError{Error: "current accept_once mode does not bind this decision"})
				return
			}
			consumptionDigest, digestErr := selfdevprotocol.DecisionBindingDigest(selfDevelopmentDecisionBinding(operationID, request))
			if digestErr != nil {
				writeAPIJSON(w, http.StatusConflict, apiError{Error: "decision mode binding is invalid"})
				return
			}
			currentMode, modeErr = h.rt.selfdevControl.SetSelfDevelopmentMode(r.Context(), platform.SetSelfDevelopmentModeRequest{
				Mode: platform.SelfDevelopmentModeProposeOnly, ExpectedGeneration: currentMode.Generation,
				IdempotencyKey: "accept-once-consumed:" + operationID + ":" + consumptionDigest,
			})
		default:
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "current mode does not authorize approval"})
			return
		}
	}
	request.ModeReceipt = currentMode.Receipt
	if modeErr != nil || h.verifyDecisionModeReceipt(computerID, operationID, currentMode.Mode, currentMode.Receipt, request) != nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "current signed mode does not authorize this decision"})
		return
	}
	modeBytes, modeErr := request.ModeReceipt.CanonicalBytes()
	if modeErr != nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "mode receipt encoding failed"})
		return
	}
	modeReceiptDigest = computerevent.DigestBytes(modeBytes)
	if operation.State != selfdev.StateAwaitingApproval {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "self-development operation is not awaiting approval"})
		return
	}
	expectedCanonicalHead := ""
	if !found {
		head, headErr := h.rt.store.Head(r.Context(), computerID)
		if headErr != nil || head == nil || head.DesiredEventHead != request.ExpectedDesiredEventHead ||
			head.EffectiveEventHead != request.ExpectedEffectiveEventHead ||
			head.PendingTransitionRef != expectedPendingTransitionRef ||
			head.DesiredStateCommitment != request.ExpectedDesiredStateCommitment ||
			head.EffectiveStateCommitment != request.ExpectedEffectiveStateCommitment {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "decision head binding changed"})
			return
		}
		expectedCanonicalHead = head.CanonicalEventHead
	}
	if !found {
		eventID, eventErr := computerevent.NewEventID()
		if eventErr != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create decision event"})
			return
		}
		event = computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: computerID,
			EventKind: expectedKind, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), IdempotencyKey: eventIdempotency,
			RequestCommitment: computerevent.ZeroHead, TrajectoryID: operation.TrajectoryID, CapsuleID: operation.CapsuleID,
			ParentEventID: operation.OperationID,
			PreviousHead:  expectedCanonicalHead,
			ActorProfile:  agentprofile.Super, AuthorityRef: "external-owner:" + ownerID, PrivacyClass: "owner",
			ExpectedDesiredEventHead: request.ExpectedDesiredEventHead, ExpectedEffectiveEventHead: request.ExpectedEffectiveEventHead,
			ExpectedPendingTransitionRef:     expectedPendingTransitionRef,
			ExpectedDesiredStateCommitment:   request.ExpectedDesiredStateCommitment,
			ExpectedEffectiveStateCommitment: request.ExpectedEffectiveStateCommitment,
			RequireExpectedHead:              true,
			PayloadCommitment:                computerevent.ZeroHead, ProposedEffectRef: operation.BundleDigest, DecisionRef: decisionRef,
			VerifierRefs: []string{request.VerifierRef}, ReducerVersion: computerevent.ReducerVersionV1,
		}
		input := computerevent.TransitionInput{}
		if request.Decision == "approve" {
			target, targetErr := computerevent.CanonicalJSON(map[string]string{"base_head": operation.BaseHead, "bundle_digest": operation.BundleDigest})
			if targetErr != nil {
				writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to bind target state"})
				return
			}
			input.TargetStateCommitment = computerevent.DigestBytes(target)
		}
		payloads := []computerevent.EventPayload{{
			Content: modeBytes, MediaType: "application/vnd.choir.mode-receipt+json",
			PrivacyClass: "owner", Direction: computerevent.EventPayloadInput,
		}}
		if request.Decision == "reject" {
			payloads = append(payloads, computerevent.EventPayload{
				Content: []byte(request.Reason), MediaType: "text/plain; charset=utf-8",
				PrivacyClass: "private", Direction: computerevent.EventPayloadOutput, Private: true,
			})
		}
		_, artifactDigests, appendErr := h.rt.eventAppender.AppendNewPayloadSet(r.Context(), event, input, payloads, h.rt.privateArtifactCipher)
		if appendErr == nil && (len(artifactDigests) == 0 || artifactDigests[0] != modeReceiptDigest) {
			appendErr = fmt.Errorf("mode receipt artifact binding mismatch")
		}
		if appendErr != nil {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: appendErr.Error()})
			return
		}
		event, found, err = h.rt.store.EventByIdempotency(r.Context(), computerID, eventIdempotency)
		if err != nil || !found {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "decision event projection unavailable"})
			return
		}
	}
	eventDigest, err := event.Digest()
	if err != nil || event.EventKind != expectedKind || event.DecisionRef != decisionRef || event.ProposedEffectRef != operation.BundleDigest {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "durable decision event binding mismatch"})
		return
	}
	transition, transitionFound, transitionErr := h.rt.store.FinalizedDecisionForOperation(r.Context(), computerID, operation.OperationID, operation.TrajectoryID, operation.CapsuleID)
	if transitionErr != nil || !transitionFound || transition.Request.EventDigest != eventDigest {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "exact decision transition projection unavailable"})
		return
	}
	decision, bindingErr := verifyFinalizedSelfDevelopmentDecision(operation, transition)
	if bindingErr != nil || decision.NextState != nextState {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "durable decision transition binding mismatch"})
		return
	}
	operation, err = h.rt.selfdevOperations.Transition(r.Context(), computerID, operation.OperationID, selfdev.StateAwaitingApproval, nextState, func(next *selfdev.Operation) error {
		next.DecisionActor = decision.Actor
		next.DecisionEvent = eventDigest
		next.DesiredHead, next.EffectiveHead = transition.Request.Next.DesiredEventHead, transition.Request.Next.EffectiveEventHead
		next.DecisionReceipt = transition.Receipt.ReceiptID
		next.ModeReceipt = decision.ModeReceiptDigest
		return nil
	})
	if err != nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: err.Error()})
		return
	}
	if operation.State == selfdev.StateAccepted {
		go h.rt.reconcileSelfDevelopmentMaterialization(context.Background())
	}
	writeAPIJSON(w, http.StatusOK, operation)
}

func selfDevelopmentDecisionRef(request selfDevelopmentDecisionRequest) (string, error) {
	request.ModeReceipt = nil
	requestBytes, err := computerevent.CanonicalJSON(request)
	if err != nil {
		return "", err
	}
	return computerevent.DigestBytes(requestBytes), nil
}

func exactSelfDevelopmentDecisionRequestMatches(event computerevent.Event, computerID, operationID, ownerID string, expectedKind computerevent.EventKind, decisionRef string, request selfDevelopmentDecisionRequest) bool {
	expectedPending := ""
	if request.ExpectedPendingTransitionRef != nil {
		expectedPending = strings.TrimSpace(*request.ExpectedPendingTransitionRef)
	}
	return event.ComputerID == computerID && event.ParentEventID == operationID &&
		event.EventKind == expectedKind && event.AuthorityRef == "external-owner:"+ownerID &&
		event.ProposedEffectRef == request.BundleDigest && event.DecisionRef == decisionRef &&
		len(event.VerifierRefs) == 1 && event.VerifierRefs[0] == request.VerifierRef &&
		event.ExpectedDesiredEventHead == request.ExpectedDesiredEventHead &&
		event.ExpectedEffectiveEventHead == request.ExpectedEffectiveEventHead &&
		event.ExpectedPendingTransitionRef == expectedPending &&
		event.ExpectedDesiredStateCommitment == request.ExpectedDesiredStateCommitment &&
		event.ExpectedEffectiveStateCommitment == request.ExpectedEffectiveStateCommitment
}

func (h *APIHandler) verifyStartModeReceipt(computerID, mode string, current, request *computerevent.Receipt) error {
	if h == nil || h.rt == nil || h.rt.selfdevControl == nil || current == nil || request == nil ||
		current.ReceiptKind != "ModeReceipt" || current.Verify(h.rt.selfdevControl.KeyResolver()) != nil {
		return fmt.Errorf("signed current mode receipt is required")
	}
	currentBytes, currentErr := current.CanonicalBytes()
	requestBytes, requestErr := request.CanonicalBytes()
	computer, _ := current.KindFields["computer_id"].(string)
	newMode, _ := current.KindFields["new_mode"].(string)
	if currentErr != nil || requestErr != nil || !bytes.Equal(currentBytes, requestBytes) ||
		computer != computerID || newMode != mode || mode != "propose_only" {
		return fmt.Errorf("mode receipt does not match current proposal authority")
	}
	return nil
}

func (h *APIHandler) verifyDecisionModeReceipt(computerID, operationID, mode string, current *computerevent.Receipt, request selfDevelopmentDecisionRequest) error {
	if current == nil || request.ModeReceipt == nil || current.ReceiptKind != "ModeReceipt" ||
		current.Verify(h.rt.selfdevControl.KeyResolver()) != nil {
		return fmt.Errorf("signed current mode receipt is required")
	}
	currentBytes, currentErr := current.CanonicalBytes()
	requestBytes, requestErr := request.ModeReceipt.CanonicalBytes()
	field := func(name string) string {
		value, _ := current.KindFields[name].(string)
		return value
	}
	if currentErr != nil || requestErr != nil || !bytes.Equal(currentBytes, requestBytes) ||
		field("computer_id") != computerID || field("new_mode") != mode || mode != "propose_only" {
		return fmt.Errorf("mode receipt does not match current authority")
	}
	if request.Decision == "approve" {
		return h.verifyConsumedModeReceipt(request.ModeReceipt, operationID, request)
	}
	if request.Decision != "reject" || field("old_mode") == "accept_once" || field("consumed_operation_id") != "" {
		return fmt.Errorf("current mode does not authorize rejection")
	}
	return nil
}

func (h *APIHandler) verifyConsumedModeReceipt(receipt *computerevent.Receipt, operationID string, request selfDevelopmentDecisionRequest) error {
	if receipt == nil || h.rt == nil || h.rt.selfdevControl == nil || receipt.ReceiptKind != "ModeReceipt" ||
		receipt.Verify(h.rt.selfdevControl.KeyResolver()) != nil {
		return fmt.Errorf("valid consumed accept_once mode receipt is required")
	}
	issuedAt, err := time.Parse(time.RFC3339Nano, receipt.IssuedAt)
	consumedExpiry, expiryErr := time.Parse(time.RFC3339Nano, fmt.Sprint(receipt.KindFields["consumed_expires_at"]))
	if err != nil || expiryErr != nil || issuedAt.After(consumedExpiry) {
		return fmt.Errorf("consumed accept_once mode receipt was issued outside its authorization window")
	}
	field := func(name string) string {
		value, _ := receipt.KindFields[name].(string)
		return value
	}
	expectedConsumptionDigest, digestErr := selfdevprotocol.DecisionBindingDigest(selfDevelopmentDecisionBinding(operationID, request))
	expectedConsumptionKey := "accept-once-consumed:" + strings.TrimSpace(operationID) + ":" + expectedConsumptionDigest
	if digestErr != nil || field("old_mode") != "accept_once" || field("new_mode") != "propose_only" ||
		field("consumed_operation_id") != operationID || field("consumed_bundle_digest") != request.BundleDigest ||
		field("consumed_desired_event_head") != request.ExpectedDesiredEventHead ||
		field("consumed_effective_event_head") != request.ExpectedEffectiveEventHead ||
		field("consumed_pending_transition_ref") != strings.TrimSpace(*request.ExpectedPendingTransitionRef) ||
		field("consumed_desired_state_commitment") != request.ExpectedDesiredStateCommitment ||
		field("consumed_effective_state_commitment") != request.ExpectedEffectiveStateCommitment ||
		field("idempotency_key") != expectedConsumptionKey {
		return fmt.Errorf("consumed accept_once mode receipt binding mismatch")
	}
	return nil
}

func selfDevelopmentDecisionBinding(operationID string, request selfDevelopmentDecisionRequest) selfdevprotocol.DecisionBinding {
	return selfdevprotocol.DecisionBinding{
		OperationID: operationID, Decision: request.Decision, IdempotencyKey: request.IdempotencyKey,
		BundleDigest: request.BundleDigest, VerifierRef: request.VerifierRef, Reason: request.Reason,
		ExpectedDesiredEventHead: request.ExpectedDesiredEventHead, ExpectedEffectiveEventHead: request.ExpectedEffectiveEventHead,
		ExpectedPendingTransitionRef:     request.ExpectedPendingTransitionRef,
		ExpectedDesiredStateCommitment:   request.ExpectedDesiredStateCommitment,
		ExpectedEffectiveStateCommitment: request.ExpectedEffectiveStateCommitment,
	}
}

func (h *APIHandler) ensureSelfDevelopmentRun(r *http.Request, operation selfdev.Operation, ownerID, prompt string) (selfdev.Operation, error) {
	if operation.State != selfdev.StateRequested {
		return operation, nil
	}
	runs, err := h.rt.store.ListRunsBySelfDevelopmentOperation(r.Context(), ownerID, operation.OperationID, 2)
	if err != nil {
		return operation, fmt.Errorf("resolve self-development run: %w", err)
	}
	if len(runs) > 1 {
		return operation, fmt.Errorf("self-development operation resolves to multiple runs")
	}
	if len(runs) == 0 {
		boundPrompt := fmt.Sprintf("Self-development operation %s on computer %s. Preserve this exact operation identity in all implementation and verifier work.\n\n%s", operation.OperationID, operation.ComputerID, prompt)
		_, err = h.rt.StartRunWithMetadata(r.Context(), boundPrompt, ownerID, map[string]any{
			runMetadataAgentProfile:                agentprofile.Super,
			runMetadataAgentRole:                   agentprofile.Super,
			runMetadataTrajectoryID:                operation.TrajectoryID,
			"request_source":                       "self_development_operation",
			"self_development_operation_id":        operation.OperationID,
			"self_development_computer_id":         operation.ComputerID,
			"self_development_prompt_artifact_ref": operation.PromptArtifactRef,
		})
		if err != nil {
			return operation, fmt.Errorf("start self-development run: %w", err)
		}
	}
	return h.rt.selfdevOperations.Transition(r.Context(), operation.ComputerID, operation.OperationID, selfdev.StateRequested, selfdev.StateExecuting, nil)
}

func selfDevelopmentContainsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func (h *APIHandler) startSelfDevelopmentRollback(w http.ResponseWriter, r *http.Request, ownerID, computerID string) {
	var request selfDevelopmentRollbackRequest
	if h.rt.eventAppender == nil || h.rt.store == nil || h.rt.selfdevRoute == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "rollback authority unavailable"})
		return
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || !computerevent.IsSHA256(request.ExpectedDesiredHead) ||
		!computerevent.IsSHA256(request.CurrentAppliedHead) || !computerevent.IsSHA256(request.ToAppliedHead) || request.ToAppliedHead == request.CurrentAppliedHead ||
		!computerevent.IsSHA256(strings.TrimSpace(request.PriorMaterialization)) ||
		!computerevent.IsSHA256(strings.TrimPrefix(strings.TrimSpace(request.PriorCheckpoint), "checkpoint:sha256:")) || !strings.HasPrefix(strings.TrimSpace(request.PriorCheckpoint), "checkpoint:sha256:") ||
		request.ExpectedRouteGeneration == 0 || strings.TrimSpace(request.IdempotencyKey) == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "complete rollback bindings are required"})
		return
	}
	canonicalRequest, err := computerevent.CanonicalJSON(request)
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid rollback request"})
		return
	}
	commitment := computerevent.DigestBytes(canonicalRequest)
	if existing, found, lookupErr := h.rt.selfdevOperations.GetByIdempotency(r.Context(), computerID, request.IdempotencyKey); lookupErr != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "rollback lookup failed"})
		return
	} else if found {
		if existing.RequestCommitment != commitment || (existing.State != selfdev.StateRollbackPending && existing.State != selfdev.StateRolledBack) {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "rollback idempotency binding changed"})
			return
		}
		writeAPIJSON(w, http.StatusOK, existing)
		return
	}
	eventIdempotency := "selfdev-rollback-requested-" + request.IdempotencyKey
	existingRollbackEvent, eventFound, err := h.rt.store.EventByIdempotency(r.Context(), computerID, eventIdempotency)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "rollback event lookup failed"})
		return
	}
	head, err := h.rt.store.Head(r.Context(), computerID)
	eventDigest := ""
	if eventFound {
		eventDigest, _ = existingRollbackEvent.Digest()
	}
	freshHead := !eventFound && head != nil && head.PendingTransitionRef == "" && head.DesiredEventHead == request.ExpectedDesiredHead && head.EffectiveEventHead == request.CurrentAppliedHead
	replayHead := eventFound && head != nil && head.PendingTransitionRef == eventDigest && head.DesiredEventHead == eventDigest && head.EffectiveEventHead == request.CurrentAppliedHead
	if err != nil || !freshHead && !replayHead {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "rollback head preconditions changed"})
		return
	}
	targetEvent, found, err := h.rt.store.EventByDigest(r.Context(), computerID, request.ToAppliedHead)
	if err != nil || !found || (targetEvent.EventKind != computerevent.EventGenesisImported && targetEvent.EventKind != computerevent.EventMaterializationApplied && targetEvent.EventKind != computerevent.EventRollbackApplied) || !computerevent.IsSHA256(targetEvent.ResultingEffectiveCommitment) {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "rollback target is not a prior applied event"})
		return
	}
	target, err := h.rt.selfdevOperations.GetByEffectiveHead(r.Context(), computerID, request.ToAppliedHead)
	if err != nil || target.MaterializationReceipt != request.PriorMaterialization || target.CheckpointRef != request.PriorCheckpoint ||
		!computerevent.IsSHA256(target.ReleaseDigest) || target.CodeRef == "" || target.ArtifactProgramRef == "" ||
		len(target.VerifierRefs) == 0 || !computerevent.IsSHA256(target.VerifierRefs[0]) || target.RouteReceipt == "" {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "rollback target receipts and immutable inputs do not match"})
		return
	}
	routeSlotID, err := routeledger.RouteSlotID(h.rt.selfdevRouteOwnerID, h.rt.selfdevRouteDesktopID)
	if err != nil || h.rt.selfdevRoute == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "rollback route authority unavailable"})
		return
	}
	route, err := h.rt.selfdevRoute.ResolveComputerVersionRoute(r.Context(), routeSlotID)
	if err != nil || route.Slot.Generation != request.ExpectedRouteGeneration {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "rollback route generation changed"})
		return
	}
	if eventFound {
		if existingRollbackEvent.EventKind != computerevent.EventRollbackRequested || existingRollbackEvent.ProposedEffectRef != target.BundleDigest || existingRollbackEvent.DecisionRef != request.ToAppliedHead {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "rollback event binding changed"})
			return
		}
	} else {
		eventID, eventErr := computerevent.NewEventID()
		if eventErr != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "rollback event identity unavailable"})
			return
		}
		event := computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: computerID,
			EventKind: computerevent.EventRollbackRequested, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
			IdempotencyKey: eventIdempotency, TrajectoryID: target.TrajectoryID, CapsuleID: target.CapsuleID,
			ActorProfile: agentprofile.Super, AuthorityRef: "owner:" + ownerID, RequestCommitment: commitment,
			PayloadCommitment: commitment, PrivacyClass: "owner", ProposedEffectRef: target.BundleDigest,
			DecisionRef: request.ToAppliedHead, ReducerVersion: computerevent.ReducerVersionV1,
		}
		if _, err := h.rt.eventAppender.AppendNew(r.Context(), event, computerevent.TransitionInput{TargetStateCommitment: targetEvent.ResultingEffectiveCommitment}, nil); err != nil {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: err.Error()})
			return
		}
	}
	nextHead, err := h.rt.store.Head(r.Context(), computerID)
	if err != nil || nextHead == nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "rollback projection unavailable"})
		return
	}
	operation, err := h.rt.selfdevOperations.StartRollback(r.Context(), selfdev.RollbackStartRequest{
		ComputerID: computerID, IdempotencyKey: request.IdempotencyKey, RequestCommitment: commitment,
		RollbackEvent: nextHead.CanonicalEventHead, DecisionActor: ownerID, CurrentDesired: nextHead.DesiredEventHead,
		CurrentEffective: nextHead.EffectiveEventHead, Target: target, RouteGeneration: request.ExpectedRouteGeneration,
	})
	if err != nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: err.Error()})
		return
	}
	go h.rt.reconcileSelfDevelopmentMaterialization(context.Background())
	writeAPIJSON(w, http.StatusCreated, operation)
}

func (h *APIHandler) readKernelCapabilityReceipt(w http.ResponseWriter, r *http.Request, computerID string) {
	rt := h.rt
	if rt.selfdevUpdater == nil || rt.selfdevRoute == nil || rt.selfdevRouteOwnerID == "" ||
		rt.selfdevRouteDesktopID == "" || rt.selfdevRealizationID == "" || rt.selfdevUpdaterRoot == "" {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "kernel capability authority unavailable"})
		return
	}
	slotID, err := routeledger.RouteSlotID(rt.selfdevRouteOwnerID, rt.selfdevRouteDesktopID)
	if err != nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "computer route identity unavailable"})
		return
	}
	resolution, err := rt.selfdevRoute.ResolveComputerVersionRoute(r.Context(), slotID)
	if err != nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "computer route identity unavailable"})
		return
	}
	manifest, err := updater.ReadCurrentManifest(rt.selfdevUpdaterRoot)
	if err != nil {
		baselineRoot := filepath.Clean(strings.TrimSpace(os.Getenv("CHOIR_BASELINE_RELEASE_ROOT")))
		if !strings.HasPrefix(baselineRoot, "/nix/store/") {
			writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "current immutable release unavailable"})
			return
		}
		manifest, err = updater.BuildBaselineManifest(baselineRoot, computerID, string(resolution.Slot.Current.CodeRef), string(resolution.Slot.Current.ArtifactProgramRef))
		if err != nil {
			writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "current immutable release unavailable"})
			return
		}
	}
	report, err := rt.selfdevUpdater.KernelCapabilities(r.Context(), updater.KernelCapabilityRequest{
		ComputerVersion: resolution.Slot.Current, ReleaseDigest: manifest.ContentDigest,
	})
	if err != nil || updater.VerifyKernelCapabilityReport(report, computerID, rt.selfdevRealizationID, resolution.Slot.Current, time.Now().UTC()) != nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "kernel capability receipt unavailable"})
		return
	}
	writeAPIJSON(w, http.StatusOK, report)
}

func (h *APIHandler) HandleSelfDevelopmentRestartHandoff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.Header.Get("X-Internal-Updater") != "true" || !requestIsLoopback(r) {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "not found"})
		return
	}
	if h.rt == nil || h.rt.selfdevControl == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "restart credential authority unavailable"})
		return
	}
	path := strings.TrimSpace(os.Getenv("CHOIR_RESTART_CREDENTIAL_HANDOFF"))
	if path == "" {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "restart credential handoff path unavailable"})
		return
	}
	if err := h.rt.selfdevControl.WriteRestartHandoff(r.Context(), path); err != nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "restart credential handoff failed"})
		return
	}
	writeAPIJSON(w, http.StatusNoContent, nil)
}
func requestIsLoopback(request *http.Request) bool {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		return false
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}
