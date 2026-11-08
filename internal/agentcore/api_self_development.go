package agentcore

import (
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

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/decisionpolicy"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/recoveryplan"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

const (
	replayCompletenessGuestTimeout    = 10 * time.Minute
	replayCompletenessGuestWriteGrace = time.Second
)

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
	// ConsensusDecision is corpusd's ConsensusDecisionReceipt for a computer
	// whose mode is apply.
	ConsensusDecision *computerevent.Receipt `json:"consensus_decision,omitempty"`
	// ConsensusDissent is the digest of the signed reject ballot that
	// stopped a standing-consensus decision; a reject citing it records
	// the dissent as the rejection's authority (P4).
	ConsensusDissent string `json:"consensus_dissent,omitempty"`
}

// consensusDissentAuthorityPrefix is the AuthorityRef of a rejection a
// standing-consensus dissent caused (P4). Only effect_rejected carries it.
const consensusDissentAuthorityPrefix = "consensus-dissent:"

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
	// Checkpoints are bound automatically (checkpoint_cadence.go); the manual
	// bind route is gone. What remains is the read of what was bound and the
	// owner act that reopens a closed cadence.
	if len(parts) == 3 && parts[1] == "lifecycle" && parts[2] == "checkpoints" {
		h.listComputerCheckpoints(w, r, computerID)
		return
	}
	if len(parts) == 3 && parts[1] == "lifecycle" && parts[2] == "resume-checkpoints" {
		h.resumeCheckpointCadence(w, r, computerID)
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
			if errors.Is(err, recoveryplan.ErrBaseRefused) {
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
	if len(parts) == 3 && parts[1] == "self-development" && parts[2] == "package-migration-rollback" {
		// Forced modularity M3: the owner restores a migration's snapshot.
		// An owner act (the proxy's gate): the computer routes are
		// read-only to a desk.
		h.packageMigrationRollbackOwnerAct(w, r, ownerID)
		return
	}
	if len(parts) == 3 && parts[1] == "self-development" && parts[2] == "package-migration-install" {
		// Forced modularity M3 (PM22): the owner installs a package the
		// migration set aside. An install-class owner act (the proxy's gate).
		h.packageMigrationInstallOwnerAct(w, r, ownerID)
		return
	}
	if len(parts) == 3 && parts[1] == "self-development" && parts[2] == "rebase-retry" {
		// U4 slice 8: the owner re-opens a spent rebase pair. An owner act:
		// the computer routes are read-only to a desk.
		h.retryRebaseOwnerAct(w, r, ownerID)
		return
	}
	// The verifier key read and the re-pin record do not need the
	// operation store: a computer whose store failed can still be re-pinned.
	if len(parts) == 3 && parts[1] == "self-development" && parts[2] == "verifier-key" {
		h.verifierKeyRoute(w, r, computerID, false)
		return
	}
	if len(parts) == 4 && parts[1] == "self-development" && parts[2] == "verifier-key" && parts[3] == "repinned" {
		h.verifierKeyRoute(w, r, computerID, true)
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
	case len(parts) == 3 && parts[1] == "self-development" && parts[2] == "abandonments":
		// The owner closes an open operation that will not settle.
		if r.Method != http.MethodPost {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		h.abandonSelfDevelopmentOperation(w, r, ownerID, computerID)
	case len(parts) == 5 && parts[1] == "self-development" && parts[2] == "operations" && parts[4] == "revert":
		// R3: the owner's revert of an applied operation; the guest assembles
		// the rollback bindings from its current head.
		if r.Method != http.MethodPost {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		h.revertSelfDevelopmentOperation(w, r, ownerID, computerID, strings.TrimSpace(parts[3]))
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
	case len(parts) == 5 && parts[1] == "packages" && parts[2] == "operations" && parts[4] == "release-draft":
		// Library S2: the package release this computer would publish; read
		// only.
		if r.Method != http.MethodGet {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		h.packageReleaseDraft(w, r, ownerID, computerID, strings.TrimSpace(parts[3]))
	case len(parts) == 6 && parts[1] == "packages" && parts[2] == "operations" && parts[4] == "media":
		// The bytes of one item the release draft names (the round's bound
		// screens and video); read only, for the proxy at publish.
		if r.Method != http.MethodGet {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		h.packageReleaseMedia(w, r, ownerID, computerID, strings.TrimSpace(parts[3]), parts[5])
	case len(parts) == 5 && parts[1] == "self-development" && parts[2] == "operations" && parts[4] == "release-witness":
		// S6-0: the joined release witness for any operation in any state;
		// a projection of the tape, read only.
		if r.Method != http.MethodGet {
			writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
			return
		}
		h.releaseWitness(w, r, computerID, strings.TrimSpace(parts[3]))
	case len(parts) == 3 && parts[1] == "packages" && parts[2] == "installs":
		// The Library: install or uninstall a package release; list installs.
		h.handlePackageInstalls(w, r, ownerID, computerID, "")
	case len(parts) == 3 && parts[1] == "packages" && parts[2] == "updates":
		// F1: the update sync; the proxy supplies the catalog's facts.
		h.handlePackageUpdates(w, r, ownerID, computerID)
	case len(parts) == 4 && parts[1] == "packages" && parts[2] == "installs":
		h.handlePackageInstalls(w, r, ownerID, computerID, strings.TrimSpace(parts[3]))
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
	listing := map[string]any{"operations": operations}
	// Which applied operations the computer's state still carries: an
	// operation a later revert or rollback undid still reads applied.
	if live, liveErr := h.liveOperations(r.Context(), computerID); liveErr == nil {
		ids := make([]string, 0, len(live))
		for id := range live {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		listing["live_operation_ids"] = ids
	}
	// The route slot's current generation is what a rollback's
	// expected_route_generation must equal. It is read through the resolver
	// the rollback check uses, and left out when the route does not resolve.
	if h.rt.selfdevRoute != nil {
		if slotID, slotErr := routeledger.RouteSlotID(h.rt.selfdevRouteOwnerID, h.rt.selfdevRouteDesktopID); slotErr == nil {
			if route, routeErr := h.rt.selfdevRoute.ResolveComputerVersionRoute(r.Context(), slotID); routeErr == nil && route.Slot.Generation != 0 {
				listing["route_generation"] = route.Slot.Generation
			}
		}
	}
	writeAPIJSON(w, http.StatusOK, listing)
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
		// A replay of the landed act is the same act: its owner-act record
		// replays (one record, one wake), and a first call that landed the
		// decision but not the record gets it now.
		h.rt.notifyTextureOfOwnerAct(ctx, ownerID, operation, selfDevelopmentOwnerAct(request.Decision), request.Reason)
		return operation, http.StatusOK, nil
	}
	if h.rt.selfdevControl == nil {
		return selfdev.Operation{}, http.StatusServiceUnavailable, errors.New("self-development mode authority unavailable")
	}
	currentMode, modeErr := h.rt.selfdevControl.SelfDevelopmentMode(ctx)
	if modeErr != nil {
		return selfdev.Operation{}, http.StatusConflict, errors.New("current signed mode does not authorize this decision")
	}
	// The mode row is the authority (docs/design/selfdev-apply-and-revert-2026-11-06.md
	// 3.2). No row is canonical apply at generation 0 with no receipt; a row's
	// receipt must verify.
	if currentMode.Receipt == nil {
		if currentMode.Generation != 0 {
			return selfdev.Operation{}, http.StatusConflict, errors.New("current mode receipt is missing")
		}
	} else if currentMode.Receipt.ReceiptKind != "ModeReceipt" ||
		currentMode.Receipt.Verify(h.rt.selfdevControl.KeyResolver()) != nil {
		return selfdev.Operation{}, http.StatusConflict, errors.New("current mode receipt is invalid")
	}
	// Reject is the fail-safe decision and works under any mode, off
	// included (the receipt checks above still hold); approve needs apply.
	if platform.CanonicalSelfDevelopmentMode(currentMode.Mode) != platform.SelfDevelopmentModeApply && request.Decision != "reject" {
		return selfdev.Operation{}, http.StatusConflict, errors.New("current mode does not authorize this decision")
	}
	var consensusReceiptBytes []byte
	consensusAuthority := ""
	consensusMediaType := "application/vnd.choir.consensus-decision-receipt+json"
	if request.Decision == "approve" {
		// The decision is corpusd's, not the guest's. The guest only records
		// a ConsensusDecisionReceipt the poster signed for this operation,
		// bundle and heads under the current mode; corpusd's gate admits the
		// append only for a decision the poster minted. Owner approval is
		// retired.
		decisionBytes, decisionDigest, decisionErr := verifyStandingConsensusDecision(h.rt.selfdevControl.KeyResolver(), currentMode, computerID, operationID, request)
		if decisionErr != nil {
			return selfdev.Operation{}, http.StatusConflict, decisionErr
		}
		if kindErr := checkOperationKindAgrees(operation, consensusDecisionKind(request.ConsensusDecision) == platform.ConsensusKindPackage); kindErr != nil {
			return selfdev.Operation{}, http.StatusConflict, kindErr
		}
		consensusReceiptBytes = decisionBytes
		consensusAuthority = decisionpolicy.DecisionAuthorityRef(decisionDigest)
	}
	if dissent := strings.TrimSpace(request.ConsensusDissent); dissent != "" {
		// P4: the poster's reduction refused on a signed reject ballot; the
		// guest records the rejection under that dissent, not as an owner.
		if request.Decision != "reject" || !computerevent.IsSHA256(dissent) {
			return selfdev.Operation{}, http.StatusConflict, errors.New("a consensus dissent rejection needs the dissenting ballot's digest")
		}
		consensusAuthority = consensusDissentAuthorityPrefix + dissent
	}
	if request.Decision != "approve" && request.Decision != "reject" {
		return selfdev.Operation{}, http.StatusConflict, errors.New("current mode does not authorize this decision")
	}
	modeBytes, modeErr := selfDevelopmentModeAuthorityBytes(currentMode)
	if modeErr != nil {
		return selfdev.Operation{}, http.StatusConflict, errors.New("mode receipt encoding failed")
	}
	request.ModeReceipt = currentMode.Receipt
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
			ActorProfile:  deskmanifest.AdmitterName(), AuthorityRef: selfDevelopmentDecisionAuthority(ownerID, consensusAuthority), PrivacyClass: "owner",
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
	// The act wakes the Texture of the candidate's document (SD-08); the
	// decision is already durable, so a failed wake is logged, not returned.
	h.rt.notifyTextureOfOwnerAct(ctx, ownerID, decided, selfDevelopmentOwnerAct(request.Decision), request.Reason)
	if decided.State == selfdev.StateAccepted {
		h.rt.triggerSelfDevelopmentReconcile()
	}
	return decided, http.StatusOK, nil
}

// selfDevelopmentOwnerAct names a decision as its owner-act record's act.
func selfDevelopmentOwnerAct(decision string) string {
	if decision == "approve" {
		return types.OwnerActApprove
	}
	return types.OwnerActReject
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

// selfDevelopmentModeAuthorityBytes are the bytes of the decision's input
// artifact 0: the signed mode receipt, or for a computer with no mode row the
// default-apply marker, whose digest is platform.DefaultApplyModeDigest().
func selfDevelopmentModeAuthorityBytes(mode platform.SelfDevelopmentMode) ([]byte, error) {
	if mode.Receipt == nil {
		return []byte(platform.DefaultApplyModeMarker), nil
	}
	return mode.Receipt.CanonicalBytes()
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

// rollbackRequestedEvent is the owner's rollback_requested event, shared by the
// rollback route and the revert route (api_self_development_revert.go).
func rollbackRequestedEvent(computerID, ownerID, idempotencyKey, commitment, trajectoryID, capsuleID, proposedEffectRef, decisionRef string) (computerevent.Event, error) {
	eventID, err := computerevent.NewEventID()
	if err != nil {
		return computerevent.Event{}, err
	}
	return computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: computerID,
		EventKind: computerevent.EventRollbackRequested, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: idempotencyKey, TrajectoryID: trajectoryID, CapsuleID: capsuleID,
		ActorProfile: deskmanifest.AdmitterName(), AuthorityRef: "owner:" + ownerID, RequestCommitment: commitment,
		PayloadCommitment: commitment, PrivacyClass: "owner", ProposedEffectRef: proposedEffectRef,
		DecisionRef: decisionRef, ReducerVersion: computerevent.ReducerVersionV1,
	}, nil
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
	h.openSelfDevelopmentRollback(w, r.Context(), ownerID, computerID, request)
}

// openSelfDevelopmentRollback runs the rollback open once the bindings are
// complete. The owner route and the revert route (api_self_development_revert.go)
// both end here for a target that is an earlier applied operation.
func (h *APIHandler) openSelfDevelopmentRollback(w http.ResponseWriter, ctx context.Context, ownerID, computerID string, request selfDevelopmentRollbackRequest) {
	canonicalRequest, err := computerevent.CanonicalJSON(request)
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid rollback request"})
		return
	}
	commitment := computerevent.DigestBytes(canonicalRequest)
	if existing, found, lookupErr := h.rt.selfdevOperations.GetByIdempotency(ctx, computerID, request.IdempotencyKey); lookupErr != nil {
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
	existingRollbackEvent, eventFound, err := h.rt.store.EventByIdempotency(ctx, computerID, eventIdempotency)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "rollback event lookup failed"})
		return
	}
	head, err := h.rt.store.Head(ctx, computerID)
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
	targetEvent, found, err := h.rt.store.EventByDigest(ctx, computerID, request.ToAppliedHead)
	// A genesis baseline has no recorded verification event, so a rollback to
	// it could apply and then never record its success (R7). Refuse at open
	// until a genesis evidence path exists.
	if err == nil && found && targetEvent.EventKind == computerevent.EventGenesisImported {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "rollback to the genesis baseline is not supported: it has no verification evidence to record the rollback against"})
		return
	}
	if err != nil || !found || (targetEvent.EventKind != computerevent.EventMaterializationApplied && targetEvent.EventKind != computerevent.EventRollbackApplied) || !computerevent.IsSHA256(targetEvent.ResultingEffectiveCommitment) {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "rollback target is not a prior applied event"})
		return
	}
	target, err := h.rt.selfdevOperations.GetByEffectiveHead(ctx, computerID, request.ToAppliedHead)
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
	route, err := h.rt.selfdevRoute.ResolveComputerVersionRoute(ctx, routeSlotID)
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
		event, eventErr := rollbackRequestedEvent(computerID, ownerID, eventIdempotency, commitment, target.TrajectoryID, target.CapsuleID, target.BundleDigest, request.ToAppliedHead)
		if eventErr != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "rollback event identity unavailable"})
			return
		}
		if _, err := h.rt.eventAppender.AppendNew(ctx, event, computerevent.TransitionInput{TargetStateCommitment: targetEvent.ResultingEffectiveCommitment}, nil); err != nil {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: err.Error()})
			return
		}
	}
	nextHead, err := h.rt.store.Head(ctx, computerID)
	if err != nil || nextHead == nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "rollback projection unavailable"})
		return
	}
	// The row names the rollback_requested event itself, not whatever event is
	// the head by now.
	requested, requestedFound, requestedErr := h.rt.store.EventByIdempotency(ctx, computerID, eventIdempotency)
	requestedDigest := ""
	if requestedErr == nil && requestedFound {
		requestedDigest, _ = requested.Digest()
	}
	if requestedDigest == "" {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "rollback event unavailable"})
		return
	}
	operation, err := h.rt.selfdevOperations.StartRollback(ctx, selfdev.RollbackStartRequest{
		ComputerID: computerID, IdempotencyKey: request.IdempotencyKey, RequestCommitment: commitment,
		RollbackEvent: requestedDigest, DecisionActor: ownerID, CurrentDesired: nextHead.DesiredEventHead,
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
