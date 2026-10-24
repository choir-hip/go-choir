package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/decisionpolicy"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/projectionbase"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"

	"github.com/yusefmosiah/go-choir/internal/updater"
)

const (
	replayCompletenessGuestTimeout    = 10 * time.Minute
	replayCompletenessGuestWriteGrace = time.Second
)

type selfDevelopmentDecisionRequest struct {
	Decision                         string                          `json:"decision"`
	IdempotencyKey                   string                          `json:"idempotency_key"`
	BundleDigest                     string                          `json:"bundle_digest"`
	VerifierRef                      string                          `json:"verifier_ref"`
	Reason                           string                          `json:"reason,omitempty"`
	ExpectedDesiredEventHead         string                          `json:"expected_desired_event_head"`
	ExpectedEffectiveEventHead       string                          `json:"expected_effective_event_head"`
	ExpectedPendingTransitionRef     *string                         `json:"expected_pending_transition_ref"`
	ExpectedDesiredStateCommitment   string                          `json:"expected_desired_state_commitment"`
	ExpectedEffectiveStateCommitment string                          `json:"expected_effective_state_commitment"`
	ModeReceipt                      *computerevent.Receipt          `json:"mode_receipt,omitempty"`
	QualifiedConsensus               *decisionpolicy.ConsensusBundle `json:"qualified_consensus,omitempty"`
	// ConsensusDecision is corpusd's ConsensusDecisionReceipt under a
	// standing_consensus arm (P4).
	ConsensusDecision *computerevent.Receipt `json:"consensus_decision,omitempty"`
	// ConsensusDissent is the digest of the signed reject ballot that
	// stopped a standing-consensus decision; a reject citing it records
	// the dissent as the rejection's authority (P4).
	ConsensusDissent string `json:"consensus_dissent,omitempty"`
}

// consensusDissentAuthorityPrefix is the AuthorityRef of a rejection a
// standing-consensus dissent caused (P4). Only effect_rejected carries it.
const consensusDissentAuthorityPrefix = "consensus-dissent:"

var selfDevelopmentDecisionPolicies = decisionpolicy.MustEffectsPolicyStore()

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

// extendReplayCompletenessGuestWriteDeadline keeps the guest's owner-only
// replay probe from inheriting the short global server write deadline. The
// global deadline remains fail-fast for every ordinary guest route.
func extendReplayCompletenessGuestWriteDeadline(w http.ResponseWriter) {
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(replayCompletenessGuestTimeout + replayCompletenessGuestWriteGrace))
}

func (h *APIHandler) handleSelfDevelopmentRoute(w http.ResponseWriter, r *http.Request, ownerID, computerID string, parts []string) {
	if strings.TrimSpace(r.Header.Get("X-Authenticated-Computer")) != computerID {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "authenticated computer binding required"})
		return
	}
	if len(parts) == 3 && parts[1] == "lifecycle" && parts[2] == "replace-workspace" {
		h.replaceComputerWorkspace(w, r, computerID)
		return
	}
	if len(parts) == 3 && parts[1] == "lifecycle" && parts[2] == "bootstrap-chain" {
		h.bootstrapComputerChain(w, r, ownerID, computerID)
		return
	}
	if len(parts) == 3 && parts[1] == "lifecycle" && parts[2] == "rematerialize-from-tape" {
		extendReplayCompletenessGuestWriteDeadline(w)
		h.rematerializeComputerFromTape(w, r, computerID)
		return
	}
	if len(parts) == 3 && parts[1] == "lifecycle" && parts[2] == "restore" {
		extendReplayCompletenessGuestWriteDeadline(w)
		h.restoreComputer(w, r, computerID)
		return
	}
	if len(parts) == 3 && parts[1] == "lifecycle" && parts[2] == "checkpoint" {
		extendReplayCompletenessGuestWriteDeadline(w)
		h.bindComputerCheckpoint(w, r, computerID)
		return
	}
	if len(parts) == 3 && parts[1] == "lifecycle" && parts[2] == "import-residue-snapshot" {
		extendReplayCompletenessGuestWriteDeadline(w)
		h.importResidueSnapshot(w, r, computerID)
		return
	}
	if len(parts) == 3 && parts[1] == "lifecycle" && parts[2] == "settle-producer-reports" {
		h.settleProducerReports(w, r, ownerID, computerID)
		return
	}
	if len(parts) == 3 && parts[1] == "lifecycle" && parts[2] == "producer-reports" {
		h.listPendingProducerReports(w, r, ownerID, computerID)
		return
	}
	if h == nil || h.rt == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "self-development authority unavailable"})
		return
	}
	if len(parts) == 3 && parts[1] == "self-development" && parts[2] == "replay-completeness" {
		if r.Method != http.MethodGet {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		extendReplayCompletenessGuestWriteDeadline(w)
		report, err := h.rt.ReplayCompleteness(r.Context(), computerID)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, projectionbase.ErrBaseRefused) {
				status = http.StatusConflict
			} else if errors.Is(err, ErrReplayCompletenessUnavailable) {
				status = http.StatusServiceUnavailable
			}
			writeAPIJSON(w, status, apiError{Error: err.Error()})
			return
		}
		writeAPIJSON(w, http.StatusOK, report)
		return
	}
	if h.rt.selfdevOperations == nil {
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
		switch r.Method {
		case http.MethodGet:
			h.listSelfDevelopmentOperations(w, r, computerID)
		default:
			// C1e B1: the public start-operation API is gone. A candidate
			// opens only when an engineering assignment's Freeze opens it.
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		}
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
	case len(parts) == 5 && parts[1] == "self-development" && parts[2] == "operations" && parts[4] == "source-patch":
		if r.Method != http.MethodGet {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		h.exportSelfDevelopmentSourcePatch(w, r, computerID, strings.TrimSpace(parts[3]))
	case len(parts) == 5 && parts[1] == "self-development" && parts[2] == "operations" && parts[4] == "visual-evidence":
		if r.Method != http.MethodGet {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		h.listSelfDevelopmentVisualEvidence(w, r, computerID, strings.TrimSpace(parts[3]))
	case len(parts) == 5 && parts[1] == "self-development" && parts[2] == "operations" && parts[4] == "release-draft":
		// P7: the release record this computer would publish; read only.
		if r.Method != http.MethodGet {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		h.releaseDraft(w, r, ownerID, computerID, strings.TrimSpace(parts[3]))
	case len(parts) == 5 && parts[1] == "self-development" && parts[2] == "operations" && parts[4] == "release-witness":
		// S6-0: the joined release witness for any operation in any state;
		// a projection of the tape, read only.
		if r.Method != http.MethodGet {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		h.releaseWitness(w, r, computerID, strings.TrimSpace(parts[3]))
	case len(parts) == 3 && parts[1] == "self-development" && parts[2] == "adoptions":
		// P8: adopt a published release as source; list adoptions.
		h.handleReleaseAdoptions(w, r, ownerID, computerID, "")
	case len(parts) == 4 && parts[1] == "self-development" && parts[2] == "adoptions":
		h.handleReleaseAdoptions(w, r, ownerID, computerID, strings.TrimSpace(parts[3]))
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
	default:
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "computer route not found"})
	}
}

// listSelfDevelopmentOperations answers the computer's operations, newest
// first, optionally narrowed by ?state= (repeatable). It is how a caller finds
// a candidate a freeze opened without re-deriving its id.
func (h *APIHandler) listSelfDevelopmentOperations(w http.ResponseWriter, r *http.Request, computerID string) {
	states := selfdev.AllStates
	if requested := r.URL.Query()["state"]; len(requested) > 0 {
		states = make([]string, 0, len(requested))
		for _, state := range requested {
			state = strings.TrimSpace(state)
			if !slices.Contains(selfdev.AllStates, state) {
				writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "unknown self-development operation state: " + state})
				return
			}
			states = append(states, state)
		}
	}
	operations, err := h.rt.selfdevOperations.ListByStates(r.Context(), computerID, states...)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list self-development operations"})
		return
	}
	slices.Reverse(operations)
	writeAPIJSON(w, http.StatusOK, map[string]any{"operations": operations})
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

func (h *APIHandler) decideSelfDevelopmentOperation(w http.ResponseWriter, r *http.Request, ownerID, computerID, operationID string) {
	var request selfDevelopmentDecisionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid self-development decision"})
		return
	}
	operation, status, err := h.decideSelfDevelopment(r.Context(), ownerID, computerID, operationID, request)
	if err != nil {
		writeAPIJSON(w, status, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, status, operation)
}

// decideSelfDevelopment is the decision path the owner's route and the
// standing-consensus driver share: it appends the decision event the
// current mode authorizes and moves the operation, answering an HTTP status.
func (h *APIHandler) decideSelfDevelopment(ctx context.Context, ownerID, computerID, operationID string, request selfDevelopmentDecisionRequest) (selfdev.Operation, int, error) {
	request.Decision, request.IdempotencyKey, request.BundleDigest, request.VerifierRef = strings.TrimSpace(request.Decision), strings.TrimSpace(request.IdempotencyKey), strings.TrimSpace(request.BundleDigest), strings.TrimSpace(request.VerifierRef)
	request.ExpectedDesiredEventHead = strings.TrimSpace(request.ExpectedDesiredEventHead)
	request.ExpectedEffectiveEventHead = strings.TrimSpace(request.ExpectedEffectiveEventHead)
	request.ExpectedDesiredStateCommitment = strings.TrimSpace(request.ExpectedDesiredStateCommitment)
	request.ExpectedEffectiveStateCommitment = strings.TrimSpace(request.ExpectedEffectiveStateCommitment)
	if request.ExpectedPendingTransitionRef == nil {
		return selfdev.Operation{}, http.StatusBadRequest, errors.New("expected_pending_transition_ref is required")
	}
	expectedPendingTransitionRef := strings.TrimSpace(*request.ExpectedPendingTransitionRef)
	if operationID == "" || request.IdempotencyKey == "" || !computerevent.IsSHA256(request.BundleDigest) || request.VerifierRef == "" ||
		!computerevent.IsSHA256(request.ExpectedDesiredEventHead) || !computerevent.IsSHA256(request.ExpectedEffectiveEventHead) ||
		!computerevent.IsSHA256(request.ExpectedDesiredStateCommitment) || !computerevent.IsSHA256(request.ExpectedEffectiveStateCommitment) ||
		(request.Decision != "approve" && request.Decision != "reject") || (request.Decision == "approve" && strings.TrimSpace(request.Reason) != "") || (request.Decision == "reject" && strings.TrimSpace(request.Reason) == "") {
		return selfdev.Operation{}, http.StatusBadRequest, errors.New("complete approve or reject decision binding is required")
	}
	operation, err := h.rt.selfdevOperations.Get(ctx, computerID, operationID)
	if err != nil {
		return selfdev.Operation{}, http.StatusNotFound, errors.New("self-development operation not found")
	}
	if operation.BundleDigest != request.BundleDigest || !selfDevelopmentContainsString(operation.VerifierRefs, request.VerifierRef) {
		return selfdev.Operation{}, http.StatusConflict, errors.New("decision binding does not match frozen operation")
	}
	decisionRef, err := selfDevelopmentDecisionRef(request)
	if err != nil {
		return selfdev.Operation{}, http.StatusBadRequest, errors.New("invalid self-development decision")
	}
	eventIdempotency := "selfdev-decision-" + computerevent.DigestBytes([]byte(computerID+"\x00"+operationID+"\x00"+request.IdempotencyKey))
	event, found, err := h.rt.store.EventByIdempotency(ctx, computerID, eventIdempotency)
	if err != nil {
		return selfdev.Operation{}, http.StatusConflict, err
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
			return selfdev.Operation{}, http.StatusConflict, errors.New("mode receipt encoding failed")
		}
		modeReceiptDigest = computerevent.DigestBytes(modeBytes)
	}
	if found {
		transition, transitionFound, transitionErr := h.rt.store.FinalizedDecisionForOperation(ctx, computerID, operation.OperationID, operation.TrajectoryID, operation.CapsuleID)
		if transitionErr != nil || !transitionFound || transition.Request.EventDigest == "" {
			return selfdev.Operation{}, http.StatusConflict, errors.New("durable decision transition projection unavailable")
		}
		decision, bindingErr := verifyFinalizedSelfDevelopmentDecision(operation, transition)
		if bindingErr != nil || decision.NextState != nextState ||
			!exactSelfDevelopmentDecisionRequestMatches(event, computerID, operationID, ownerID, expectedKind, decisionRef, request) {
			return selfdev.Operation{}, http.StatusConflict, errors.New("decision idempotency key reused with different binding")
		}
		if operation.State == selfdev.StateAwaitingApproval {
			operation, _, bindingErr = h.rt.recoverSelfDevelopmentDecision(ctx, operation)
			if bindingErr != nil {
				return selfdev.Operation{}, http.StatusConflict, bindingErr
			}
		}
		return operation, http.StatusOK, nil
	}
	if h.rt.selfdevControl == nil {
		return selfdev.Operation{}, http.StatusServiceUnavailable, errors.New("self-development mode authority unavailable")
	}
	currentMode, modeErr := h.rt.selfdevControl.SelfDevelopmentMode(ctx)
	if modeErr != nil {
		return selfdev.Operation{}, http.StatusConflict, errors.New("current signed mode does not authorize this decision")
	}
	if currentMode.Receipt == nil || currentMode.Receipt.ReceiptKind != "ModeReceipt" ||
		currentMode.Receipt.Verify(h.rt.selfdevControl.KeyResolver()) != nil {
		return selfdev.Operation{}, http.StatusConflict, errors.New("current mode receipt is invalid")
	}
	var consensusReceiptBytes []byte
	consensusAuthority := ""
	consensusMediaType := "application/vnd.choir.qualified-consensus-receipt+json"
	if request.ConsensusDecision != nil {
		consensusMediaType = "application/vnd.choir.consensus-decision-receipt+json"
	}
	if request.Decision == "approve" {
		switch currentMode.Mode {
		case platform.SelfDevelopmentModeProposeOnly:
			request.ModeReceipt = currentMode.Receipt
			if h.verifyConsumedModeReceipt(currentMode.Receipt, operationID, request) != nil {
				return selfdev.Operation{}, http.StatusConflict, errors.New("current proposal mode does not carry this consumed accept_once decision")
			}
		case platform.SelfDevelopmentModeAcceptOnce:
			expectedPending := strings.TrimSpace(*request.ExpectedPendingTransitionRef)
			if currentMode.OperationID != operationID || currentMode.BundleDigest != request.BundleDigest ||
				currentMode.ExpectedDesiredEventHead != request.ExpectedDesiredEventHead ||
				currentMode.ExpectedEffectiveEventHead != request.ExpectedEffectiveEventHead ||
				currentMode.ExpectedPendingTransitionRef != expectedPending ||
				currentMode.ExpectedDesiredStateCommitment != request.ExpectedDesiredStateCommitment ||
				currentMode.ExpectedEffectiveStateCommitment != request.ExpectedEffectiveStateCommitment {
				return selfdev.Operation{}, http.StatusConflict, errors.New("current accept_once mode does not bind this decision")
			}
			consumptionDigest, digestErr := selfdevprotocol.DecisionBindingDigest(selfDevelopmentDecisionBinding(operationID, request))
			if digestErr != nil {
				return selfdev.Operation{}, http.StatusConflict, errors.New("decision mode binding is invalid")
			}
			currentMode, modeErr = h.rt.selfdevControl.SetSelfDevelopmentMode(ctx, platform.SetSelfDevelopmentModeRequest{
				Mode: platform.SelfDevelopmentModeProposeOnly, ExpectedGeneration: currentMode.Generation,
				IdempotencyKey: platform.AcceptOnceConsumptionKeyPrefix + operationID + ":" + consumptionDigest,
			})
		case platform.SelfDevelopmentModeQualifiedConsensus:
			expectedPending := strings.TrimSpace(*request.ExpectedPendingTransitionRef)
			if currentMode.OperationID != operationID || currentMode.BundleDigest != request.BundleDigest ||
				currentMode.ExpectedDesiredEventHead != request.ExpectedDesiredEventHead ||
				currentMode.ExpectedEffectiveEventHead != request.ExpectedEffectiveEventHead ||
				currentMode.ExpectedPendingTransitionRef != expectedPending ||
				currentMode.ExpectedDesiredStateCommitment != request.ExpectedDesiredStateCommitment ||
				currentMode.ExpectedEffectiveStateCommitment != request.ExpectedEffectiveStateCommitment {
				return selfdev.Operation{}, http.StatusConflict, errors.New("current qualified_consensus mode does not bind this decision")
			}
			if request.QualifiedConsensus == nil {
				return selfdev.Operation{}, http.StatusConflict, errors.New("qualified consensus bundle is required")
			}
			if request.QualifiedConsensus.Now == "" {
				request.QualifiedConsensus.Now = time.Now().UTC().Format(time.RFC3339Nano)
			}
			receipt, reduceErr := selfDevelopmentDecisionPolicies.ReduceBundle(*request.QualifiedConsensus)
			if reduceErr != nil {
				return selfdev.Operation{}, http.StatusConflict, reduceErr
			}
			if receipt.ReceiptDigest != currentMode.ConsensusReceiptDigest || receipt.PolicyDigest != currentMode.PolicyDigest {
				return selfdev.Operation{}, http.StatusConflict, errors.New("qualified consensus receipt does not match armed mode")
			}
			stored := receipt
			stored.ReceiptDigest = ""
			receiptJSON, receiptErr := computerevent.CanonicalJSON(stored)
			if receiptErr != nil || computerevent.DigestBytes(receiptJSON) != receipt.ReceiptDigest {
				return selfdev.Operation{}, http.StatusConflict, errors.New("qualified consensus receipt encoding failed")
			}
			consensusReceiptBytes = receiptJSON
			consensusAuthority = decisionpolicy.AuthorityRef(receipt)
			subjectDigest, subjectErr := request.QualifiedConsensus.Subject.Digest()
			wantSubject := decisionpolicy.EffectSubject{
				ComputerID: computerID, OperationID: operationID, BundleDigest: request.BundleDigest,
				DesiredEventHead: request.ExpectedDesiredEventHead, EffectiveEventHead: request.ExpectedEffectiveEventHead,
				PendingTransitionRef: expectedPending, DesiredStateCommitment: request.ExpectedDesiredStateCommitment,
				EffectiveStateCommitment: request.ExpectedEffectiveStateCommitment,
				EffectClass:              request.QualifiedConsensus.Subject.EffectClass,
			}
			wantDigest, wantErr := wantSubject.Digest()
			if subjectErr != nil || wantErr != nil || subjectDigest != wantDigest || receipt.SubjectDigest != wantDigest {
				return selfdev.Operation{}, http.StatusConflict, errors.New("qualified consensus subject does not bind this operation")
			}
			consumptionDigest, digestErr := selfdevprotocol.DecisionBindingDigest(selfDevelopmentDecisionBinding(operationID, request))
			if digestErr != nil {
				return selfdev.Operation{}, http.StatusConflict, errors.New("decision mode binding is invalid")
			}
			currentMode, modeErr = h.rt.selfdevControl.SetSelfDevelopmentMode(ctx, platform.SetSelfDevelopmentModeRequest{
				Mode: platform.SelfDevelopmentModeProposeOnly, ExpectedGeneration: currentMode.Generation,
				IdempotencyKey: platform.QualifiedConsensusConsumptionKeyPrefix + operationID + ":" + consumptionDigest,
			})
		case platform.SelfDevelopmentModeStandingConsensus:
			// P4: the decision is corpusd's, not the guest's. The guest only
			// records a ConsensusDecisionReceipt the poster signed for this
			// operation, bundle and heads under the current standing arm;
			// the mode stays armed (no consumption), and corpusd's gate
			// admits the append only for a decision the poster minted.
			decisionBytes, decisionDigest, decisionErr := verifyStandingConsensusDecision(h.rt.selfdevControl.KeyResolver(), currentMode, computerID, operationID, request)
			if decisionErr != nil {
				return selfdev.Operation{}, http.StatusConflict, decisionErr
			}
			consensusReceiptBytes = decisionBytes
			consensusAuthority = decisionpolicy.DecisionAuthorityRef(decisionDigest)
		default:
			return selfdev.Operation{}, http.StatusConflict, errors.New("current mode does not authorize approval")
		}
	}
	if dissent := strings.TrimSpace(request.ConsensusDissent); dissent != "" {
		// P4: the poster's reduction refused on a signed reject ballot; the
		// guest records the rejection under that dissent, not as an owner.
		if request.Decision != "reject" || currentMode.Mode != platform.SelfDevelopmentModeStandingConsensus || !computerevent.IsSHA256(dissent) {
			return selfdev.Operation{}, http.StatusConflict, errors.New("a consensus dissent rejection needs a standing arm and the dissenting ballot's digest")
		}
		consensusAuthority = consensusDissentAuthorityPrefix + dissent
	}
	request.ModeReceipt = currentMode.Receipt
	if modeErr != nil || h.verifyDecisionModeReceipt(computerID, operationID, currentMode.Mode, currentMode.Receipt, request) != nil {
		return selfdev.Operation{}, http.StatusConflict, errors.New("current signed mode does not authorize this decision")
	}
	modeBytes, modeErr := request.ModeReceipt.CanonicalBytes()
	if modeErr != nil {
		return selfdev.Operation{}, http.StatusConflict, errors.New("mode receipt encoding failed")
	}
	modeReceiptDigest = computerevent.DigestBytes(modeBytes)
	if operation.State != selfdev.StateAwaitingApproval {
		return selfdev.Operation{}, http.StatusConflict, errors.New("self-development operation is not awaiting approval")
	}
	expectedCanonicalHead := ""
	if !found {
		head, headErr := h.rt.store.Head(ctx, computerID)
		if headErr != nil || head == nil || head.DesiredEventHead != request.ExpectedDesiredEventHead ||
			head.EffectiveEventHead != request.ExpectedEffectiveEventHead ||
			head.PendingTransitionRef != expectedPendingTransitionRef ||
			head.DesiredStateCommitment != request.ExpectedDesiredStateCommitment ||
			head.EffectiveStateCommitment != request.ExpectedEffectiveStateCommitment {
			return selfdev.Operation{}, http.StatusConflict, errors.New("decision head binding changed")
		}
		expectedCanonicalHead = head.CanonicalEventHead
	}
	if !found {
		eventID, eventErr := computerevent.NewEventID()
		if eventErr != nil {
			return selfdev.Operation{}, http.StatusInternalServerError, errors.New("failed to create decision event")
		}
		event = computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: computerID,
			EventKind: expectedKind, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), IdempotencyKey: eventIdempotency,
			RequestCommitment: computerevent.ZeroHead, TrajectoryID: operation.TrajectoryID, CapsuleID: operation.CapsuleID,
			ParentEventID: operation.OperationID,
			PreviousHead:  expectedCanonicalHead,
			ActorProfile:  agentprofile.Management, AuthorityRef: selfDevelopmentDecisionAuthority(ownerID, consensusAuthority), PrivacyClass: "owner",
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
				return selfdev.Operation{}, http.StatusInternalServerError, errors.New("failed to bind target state")
			}
			input.TargetStateCommitment = computerevent.DigestBytes(target)
		}
		payloads := []computerevent.EventPayload{{
			Content: modeBytes, MediaType: "application/vnd.choir.mode-receipt+json",
			PrivacyClass: "owner", Direction: computerevent.EventPayloadInput,
		}}
		if len(consensusReceiptBytes) > 0 {
			payloads = append(payloads, computerevent.EventPayload{
				Content: consensusReceiptBytes, MediaType: consensusMediaType,
				PrivacyClass: "owner", Direction: computerevent.EventPayloadInput,
			})
		}
		if request.Decision == "reject" {
			// The rejection reason is an owner-visible decision record on an
			// "owner"-class event. Pinning it as a private payload puts the pin
			// receipt's privacy_class at "private" while ValidateEventPins
			// requires the event's own class — every reject CAS-refused at pin
			// verification (7d635330 introduced both sides). Pin owner, not private.
			payloads = append(payloads, computerevent.EventPayload{
				Content: []byte(request.Reason), MediaType: "text/plain; charset=utf-8",
				PrivacyClass: "owner", Direction: computerevent.EventPayloadOutput,
			})
		}
		_, artifactDigests, appendErr := h.rt.eventAppender.AppendNewPayloadSet(ctx, event, input, payloads, h.rt.privateArtifactCipher)
		if appendErr == nil && (len(artifactDigests) == 0 || artifactDigests[0] != modeReceiptDigest) {
			appendErr = fmt.Errorf("mode receipt artifact binding mismatch")
		}
		if appendErr == nil && len(consensusReceiptBytes) > 0 {
			want := computerevent.DigestBytes(consensusReceiptBytes)
			if len(artifactDigests) < 2 || artifactDigests[1] != want {
				appendErr = fmt.Errorf("consensus receipt artifact binding mismatch")
			}
		}
		if appendErr != nil {
			return selfdev.Operation{}, http.StatusConflict, appendErr
		}
		event, found, err = h.rt.store.EventByIdempotency(ctx, computerID, eventIdempotency)
		if err != nil || !found {
			return selfdev.Operation{}, http.StatusInternalServerError, errors.New("decision event projection unavailable")
		}
	}
	eventDigest, err := event.Digest()
	if err != nil || event.EventKind != expectedKind || event.DecisionRef != decisionRef || event.ProposedEffectRef != operation.BundleDigest {
		return selfdev.Operation{}, http.StatusConflict, errors.New("durable decision event binding mismatch")
	}
	transition, transitionFound, transitionErr := h.rt.store.FinalizedDecisionForOperation(ctx, computerID, operation.OperationID, operation.TrajectoryID, operation.CapsuleID)
	if transitionErr != nil || !transitionFound || transition.Request.EventDigest != eventDigest {
		return selfdev.Operation{}, http.StatusInternalServerError, errors.New("exact decision transition projection unavailable")
	}
	decision, bindingErr := verifyFinalizedSelfDevelopmentDecision(operation, transition)
	if bindingErr != nil || decision.NextState != nextState {
		return selfdev.Operation{}, http.StatusConflict, errors.New("durable decision transition binding mismatch")
	}
	if h.rt.selfdevDecisionBeforeTransition != nil {
		h.rt.selfdevDecisionBeforeTransition(ctx, operation.OperationID)
	}
	decided, err := h.rt.selfdevOperations.Transition(ctx, computerID, operation.OperationID, selfdev.StateAwaitingApproval, nextState, func(next *selfdev.Operation) error {
		next.DecisionActor = decision.Actor
		next.DecisionEvent = eventDigest
		next.DesiredHead, next.EffectiveHead = transition.Request.Next.DesiredEventHead, transition.Request.Next.EffectiveEventHead
		next.DecisionReceipt = transition.Receipt.ReceiptID
		next.ModeReceipt = decision.ModeReceiptDigest
		return nil
	})
	if err != nil {
		// M7 derivable continuations: the post-commit observer may have
		// raced this transition — the drain's recoverSelfDevelopmentDecision
		// applies the same finalized decision, and on an approve may already
		// be materializing it. If the durable operation now carries this
		// exact decision, the request already succeeded. Transition answers
		// a zero operation on error, so the re-read uses the id read above.
		if !errors.Is(err, selfdev.ErrConflict) {
			return selfdev.Operation{}, http.StatusConflict, err
		}
		current, getErr := h.rt.selfdevOperations.Get(ctx, computerID, operation.OperationID)
		mismatch := "get (" + fmt.Sprint(getErr) + ")"
		if getErr == nil {
			mismatch = selfDevelopmentDecisionNotCarried(current, transition, nextState)
		}
		if mismatch != "" {
			log.Printf("selfdev decision: operation %s: %s lost its transition and the operation does not carry this decision: %s mismatch (operation state=%q decision_event=%q decision_receipt=%q; this decision event=%q receipt=%q next_state=%q): %v",
				operation.OperationID, request.Decision, mismatch, current.State, current.DecisionEvent, current.DecisionReceipt,
				eventDigest, transition.Receipt.ReceiptID, nextState, err)
			return selfdev.Operation{}, http.StatusConflict, err
		}
		decided = current
	}
	if decided.State == selfdev.StateAccepted {
		h.rt.triggerSelfDevelopmentReconcile()
	}
	return decided, http.StatusOK, nil
}

func selfDevelopmentDecisionAuthority(ownerID, consensusAuthority string) string {
	if strings.TrimSpace(consensusAuthority) != "" {
		return consensusAuthority
	}
	return "external-owner:" + ownerID
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
	wantAuthority := "external-owner:" + ownerID
	if dissent := strings.TrimSpace(request.ConsensusDissent); dissent != "" {
		wantAuthority = consensusDissentAuthorityPrefix + dissent
	} else if request.ConsensusDecision != nil {
		if raw, err := request.ConsensusDecision.CanonicalBytes(); err == nil {
			wantAuthority = decisionpolicy.DecisionAuthorityRef(computerevent.DigestBytes(raw))
		}
	} else if request.QualifiedConsensus != nil {
		if request.QualifiedConsensus.Selection.PolicyDigest != "" {
			if receipt, err := selfDevelopmentDecisionPolicies.ReduceBundle(*request.QualifiedConsensus); err == nil {
				wantAuthority = decisionpolicy.AuthorityRef(receipt)
			}
		}
	}
	return event.ComputerID == computerID && event.ParentEventID == operationID &&
		event.EventKind == expectedKind && event.AuthorityRef == wantAuthority &&
		event.ProposedEffectRef == request.BundleDigest && event.DecisionRef == decisionRef &&
		len(event.VerifierRefs) == 1 && event.VerifierRefs[0] == request.VerifierRef &&
		event.ExpectedDesiredEventHead == request.ExpectedDesiredEventHead &&
		event.ExpectedEffectiveEventHead == request.ExpectedEffectiveEventHead &&
		event.ExpectedPendingTransitionRef == expectedPending &&
		event.ExpectedDesiredStateCommitment == request.ExpectedDesiredStateCommitment &&
		event.ExpectedEffectiveStateCommitment == request.ExpectedEffectiveStateCommitment
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
		field("computer_id") != computerID || field("new_mode") != mode {
		return fmt.Errorf("mode receipt does not match current authority")
	}
	if mode == platform.SelfDevelopmentModeStandingConsensus {
		// P4: under a standing arm the guest may always reject; an approve
		// was verified against the poster's decision receipt above.
		if request.Decision == "approve" && request.ConsensusDecision == nil {
			return fmt.Errorf("standing consensus approval requires the poster's decision")
		}
		if request.Decision != "approve" && request.Decision != "reject" {
			return fmt.Errorf("current mode does not authorize this decision")
		}
		return nil
	}
	if mode != "propose_only" {
		return fmt.Errorf("mode receipt does not match current authority")
	}
	if request.Decision == "approve" {
		return h.verifyConsumedModeReceipt(request.ModeReceipt, operationID, request)
	}
	// A reject is the fail-safe decision and needs no grant, so it stays
	// available for every candidate until it is applied. The one receipt that
	// refuses it is this operation's own approval consumption: that receipt is
	// the approval's proof and must not authorize the opposite decision on the
	// same operation. A consumption of another operation, or an owner disarm
	// of an arm, says nothing about this one
	// (docs/problems/selfdev-reject-refused-after-accept-once-consumed-2026-10-24.md).
	if request.Decision != "reject" || modeReceiptConsumesOperation(field("idempotency_key"), operationID) {
		return fmt.Errorf("current mode does not authorize rejection")
	}
	return nil
}

// modeReceiptConsumesOperation reports whether a mode receipt's idempotency
// key is the guest's consumption of an accept_once or qualified_consensus arm
// by approving the named operation (the keys verifyConsumedModeReceipt binds).
func modeReceiptConsumesOperation(idempotencyKey, operationID string) bool {
	operationID = strings.TrimSpace(operationID)
	if operationID == "" {
		return false
	}
	return strings.HasPrefix(idempotencyKey, platform.AcceptOnceConsumptionKeyPrefix+operationID+":") ||
		strings.HasPrefix(idempotencyKey, platform.QualifiedConsensusConsumptionKeyPrefix+operationID+":")
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
	oldMode := field("old_mode")
	expectedConsumptionKey := platform.AcceptOnceConsumptionKeyPrefix + strings.TrimSpace(operationID) + ":" + expectedConsumptionDigest
	if oldMode == platform.SelfDevelopmentModeQualifiedConsensus {
		expectedConsumptionKey = platform.QualifiedConsensusConsumptionKeyPrefix + strings.TrimSpace(operationID) + ":" + expectedConsumptionDigest
	}
	if digestErr != nil || (oldMode != platform.SelfDevelopmentModeAcceptOnce && oldMode != platform.SelfDevelopmentModeQualifiedConsensus) || field("new_mode") != "propose_only" ||
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
			ActorProfile: agentprofile.Management, AuthorityRef: "owner:" + ownerID, RequestCommitment: commitment,
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
	h.rt.triggerSelfDevelopmentReconcile()
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
	if err != nil {
		writeKernelCapabilityUnavailable(w, updater.KernelCapabilityFailureCode(err))
		return
	}
	if err := updater.VerifyKernelCapabilityReport(report, computerID, rt.selfdevRealizationID, resolution.Slot.Current, time.Now().UTC()); err != nil {
		writeKernelCapabilityUnavailable(w, "verification_refused")
		return
	}
	writeAPIJSON(w, http.StatusOK, report)
}

func writeKernelCapabilityUnavailable(w http.ResponseWriter, reason string) {
	writeAPIJSON(w, http.StatusServiceUnavailable, apiError{
		Error: "kernel capability receipt unavailable", Reason: reason,
	})
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
