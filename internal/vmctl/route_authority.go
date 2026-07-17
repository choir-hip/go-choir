package vmctl

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
)

type RouteResolution struct {
	Slot              routeledger.Slot                `json:"slot"`
	LatestReceipt     routeledger.TransitionReceipt   `json:"latest_receipt"`
	TransitionReceipt *routeledger.TransitionReceipt  `json:"transition_receipt,omitempty"`
	CodeClosure       computerversion.CodeClosure     `json:"code_closure"`
	ArtifactProgram   computerversion.ArtifactProgram `json:"artifact_program"`
}

type routeAuthorityReader interface {
	Resolve(context.Context, string) (routeledger.Slot, routeledger.TransitionReceipt, error)
	PinAuthorizationEvidence(context.Context, routeledger.AuthorizationEvidence) (routeledger.AuthorizationEvidence, error)
	VerifyTransitionEvidence(context.Context, routeledger.TransitionCommand) error
}

type RouteAuthority struct {
	ledger       routeAuthorityReader
	sqlLedger    *routeledger.SQLLedger
	memoryLedger *routeledger.MemoryLedger
	inputs       computerversion.ImmutableInputResolver
	promotionKey ed25519.PublicKey
	mutationMu   sync.Mutex
}

// NewRouteAuthority accepts the one concrete durable ledger implementation so
// production callers cannot hide split route/evidence stores behind an interface.
func NewRouteAuthority(ledger *routeledger.SQLLedger, inputs computerversion.ImmutableInputResolver) (*RouteAuthority, error) {
	if ledger == nil || inputs == nil {
		return nil, fmt.Errorf("vmctl route authority: durable SQL route/evidence ledger and immutable input resolver are required")
	}
	return &RouteAuthority{ledger: ledger, sqlLedger: ledger, inputs: inputs}, nil
}

func newMemoryRouteAuthority(ledger *routeledger.MemoryLedger, inputs computerversion.ImmutableInputResolver) (*RouteAuthority, error) {
	if ledger == nil || inputs == nil {
		return nil, fmt.Errorf("vmctl route authority: memory route/evidence ledger and immutable input resolver are required")
	}
	return &RouteAuthority{ledger: ledger, memoryLedger: ledger, inputs: inputs}, nil
}

func (a *RouteAuthority) SetPromotionAuthorityPublicKey(publicKey ed25519.PublicKey) error {
	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("vmctl promotion authority: Ed25519 public key is required")
	}
	a.promotionKey = append(ed25519.PublicKey(nil), publicKey...)
	return nil
}

func (a *RouteAuthority) Resolve(ctx context.Context, slotID string) (RouteResolution, error) {
	if a == nil || a.ledger == nil || a.inputs == nil {
		return RouteResolution{}, fmt.Errorf("vmctl route authority: not configured")
	}
	slot, receipt, err := a.ledger.Resolve(ctx, strings.TrimSpace(slotID))
	if err != nil {
		return RouteResolution{}, err
	}
	if receipt.RouteSlotID != slot.ID || receipt.CommittedGeneration != slot.Generation || !routeledger.SameVersion(receipt.New, slot.Current) {
		return RouteResolution{}, fmt.Errorf("vmctl route authority: route receipt join failed")
	}
	closure, program, err := a.resolveVersionInputs(ctx, slot.Current)
	if err != nil {
		return RouteResolution{}, err
	}
	return RouteResolution{Slot: slot, LatestReceipt: receipt, CodeClosure: closure, ArtifactProgram: program}, nil
}

func (a *RouteAuthority) resolveVersionInputs(ctx context.Context, version computerversion.ComputerVersion) (computerversion.CodeClosure, computerversion.ArtifactProgram, error) {
	closure, err := a.inputs.ResolveCode(ctx, version.CodeRef)
	if err != nil {
		return computerversion.CodeClosure{}, computerversion.ArtifactProgram{}, fmt.Errorf("vmctl route authority: resolve CodeRef: %w", err)
	}
	program, err := a.inputs.ResolveArtifactProgram(ctx, version.ArtifactProgramRef)
	if err != nil {
		return computerversion.CodeClosure{}, computerversion.ArtifactProgram{}, fmt.Errorf("vmctl route authority: resolve ArtifactProgramRef: %w", err)
	}
	if closure.Ref != version.CodeRef || program.Ref != version.ArtifactProgramRef {
		return computerversion.CodeClosure{}, computerversion.ArtifactProgram{}, fmt.Errorf("vmctl route authority: immutable input join failed")
	}
	if err := closure.Verify(); err != nil {
		return computerversion.CodeClosure{}, computerversion.ArtifactProgram{}, fmt.Errorf("vmctl route authority: verify CodeRef: %w", err)
	}
	if err := program.Verify(); err != nil {
		return computerversion.CodeClosure{}, computerversion.ArtifactProgram{}, fmt.Errorf("vmctl route authority: verify ArtifactProgramRef: %w", err)
	}
	return closure, program, nil
}

func (a *RouteAuthority) prepareBootstrap(slotID string, verification computerversion.RealizationVerificationReceipt, approval OwnerPromotionApproval, preparedAt time.Time) (FrozenRouteBootstrapCandidate, error) {
	if err := approval.verify(a.promotionKey, slotID, verification); err != nil {
		return FrozenRouteBootstrapCandidate{}, err
	}
	ownerID, _, err := routeledger.ParseRouteSlotID(slotID)
	if err != nil || approval.RouteSlotID != slotID || approval.OwnerID != ownerID {
		return FrozenRouteBootstrapCandidate{}, fmt.Errorf("vmctl promotion authority: approval does not bind bootstrap route owner")
	}
	payload, err := json.Marshal(approval)
	if err != nil {
		return FrozenRouteBootstrapCandidate{}, err
	}
	evidence, err := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidenceApproval, slotID, verification.Version, payload, approval.ApprovedAt)
	if err != nil {
		return FrozenRouteBootstrapCandidate{}, err
	}
	return buildFrozenRouteBootstrapCandidate(slotID, verification, evidence, preparedAt)
}

func (a *RouteAuthority) preparePromotion(ctx context.Context, slotID string, verification computerversion.RealizationVerificationReceipt, approval OwnerPromotionApproval, preparedAt time.Time) (FrozenRoutePromotionCandidate, error) {
	if err := approval.verify(a.promotionKey, slotID, verification); err != nil {
		return FrozenRoutePromotionCandidate{}, err
	}
	ownerID, _, err := routeledger.ParseRouteSlotID(slotID)
	if err != nil || approval.RouteSlotID != slotID || approval.OwnerID != ownerID {
		return FrozenRoutePromotionCandidate{}, fmt.Errorf("vmctl promotion authority: approval does not bind route owner")
	}
	payload, err := json.Marshal(approval)
	if err != nil {
		return FrozenRoutePromotionCandidate{}, err
	}
	approvalEvidence, err := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidenceApproval, slotID, verification.Version, payload, approval.ApprovedAt)
	if err != nil {
		return FrozenRoutePromotionCandidate{}, err
	}
	current, err := a.Resolve(ctx, slotID)
	if err != nil {
		return FrozenRoutePromotionCandidate{}, fmt.Errorf("vmctl promotion: resolve current route: %w", err)
	}
	return buildFrozenRoutePromotionCandidate(current, verification, approvalEvidence, preparedAt)
}

func decodeOwnerApproval(evidence routeledger.AuthorizationEvidence) (OwnerPromotionApproval, error) {
	var approval OwnerPromotionApproval
	if err := json.Unmarshal(evidence.Payload, &approval); err != nil {
		return OwnerPromotionApproval{}, fmt.Errorf("vmctl route authority: decode owner approval: %w", err)
	}
	return approval, nil
}

func (a *RouteAuthority) verifyFrozenOwnerApproval(slotID string, verification computerversion.RealizationVerificationReceipt, evidence routeledger.AuthorizationEvidence) error {
	var approval OwnerPromotionApproval
	if err := json.Unmarshal(evidence.Payload, &approval); err != nil {
		return fmt.Errorf("vmctl route authority: decode owner approval: %w", err)
	}
	if err := approval.verify(a.promotionKey, slotID, verification); err != nil {
		return err
	}
	if evidence.Kind != routeledger.AuthorizationEvidenceApproval || evidence.RouteSlotID != slotID || evidence.ComputerVersion != verification.Version || !evidence.CreatedAt.Equal(approval.ApprovedAt) {
		return fmt.Errorf("vmctl route authority: owner approval evidence bindings are invalid")
	}
	return nil
}

func (a *RouteAuthority) Transition(ctx context.Context, command routeledger.TransitionCommand) (RouteResolution, error) {
	return RouteResolution{}, fmt.Errorf("vmctl route authority: every route CAS requires a signed frozen candidate")
}

func (a *RouteAuthority) transitionAuthorized(ctx context.Context, command routeledger.TransitionCommand) (RouteResolution, error) {
	if a == nil || a.ledger == nil || a.inputs == nil {
		return RouteResolution{}, fmt.Errorf("vmctl route authority: not configured")
	}
	if err := command.Validate(); err != nil {
		return RouteResolution{}, err
	}
	if err := a.ledger.VerifyTransitionEvidence(ctx, command); err != nil {
		return RouteResolution{}, fmt.Errorf("vmctl route authority: transition evidence is not pinned: %w", err)
	}
	if _, _, err := a.resolveVersionInputs(ctx, command.New); err != nil {
		return RouteResolution{}, fmt.Errorf("vmctl route authority: new ComputerVersion inputs are not pinned: %w", err)
	}
	if a.memoryLedger == nil {
		return RouteResolution{}, fmt.Errorf("vmctl route authority: raw transition is unavailable on the durable authority")
	}
	_, receipt, err := a.memoryLedger.Transition(ctx, command)
	if err != nil {
		return RouteResolution{}, err
	}
	resolution, err := a.Resolve(ctx, command.RouteSlotID)
	if err != nil {
		return RouteResolution{}, fmt.Errorf("vmctl route authority: resolve committed route: %w", err)
	}
	resolution.TransitionReceipt = &receipt
	return resolution, nil
}

func (a *RouteAuthority) transitionAuthorizedWithEvidence(ctx context.Context, command routeledger.TransitionCommand, evidence []routeledger.AuthorizationEvidence) (RouteResolution, error) {
	if a == nil || a.ledger == nil || a.inputs == nil {
		return RouteResolution{}, fmt.Errorf("vmctl route authority: not configured")
	}
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()
	if err := command.Validate(); err != nil {
		return RouteResolution{}, err
	}
	if _, _, err := a.resolveVersionInputs(ctx, command.New); err != nil {
		return RouteResolution{}, fmt.Errorf("vmctl route authority: new ComputerVersion inputs are not pinned: %w", err)
	}
	var slot routeledger.Slot
	var receipt routeledger.TransitionReceipt
	var err error
	if a.sqlLedger != nil {
		slot, receipt, err = a.sqlLedger.ApplySignedTransition(ctx, command, evidence)
	} else if a.memoryLedger != nil {
		slot, receipt, err = a.memoryLedger.TransitionWithEvidence(ctx, command, evidence)
	} else {
		err = fmt.Errorf("vmctl route authority: no route mutation ledger is configured")
	}
	if err != nil {
		return RouteResolution{}, err
	}
	resolution, err := a.Resolve(ctx, command.RouteSlotID)
	if err != nil {
		return RouteResolution{}, fmt.Errorf("vmctl route authority: resolve committed route: %w", err)
	}
	if resolution.Slot != slot {
		return RouteResolution{}, fmt.Errorf("vmctl route authority: committed slot does not match atomic transition result")
	}
	resolution.TransitionReceipt = &receipt
	return resolution, nil
}

func (a *RouteAuthority) applyFrozenBootstrap(ctx context.Context, candidate FrozenRouteBootstrapCandidate, acceptance G3PromotionAcceptance) (RouteResolution, error) {
	if err := candidate.Validate(); err != nil {
		return RouteResolution{}, err
	}
	if err := a.verifyFrozenOwnerApproval(candidate.RouteSlotID, candidate.Verification, candidate.ApprovalEvidence); err != nil {
		return RouteResolution{}, err
	}
	if err := acceptance.verifyBootstrap(a.promotionKey, candidate); err != nil {
		return RouteResolution{}, err
	}
	if _, _, err := a.ledger.Resolve(ctx, candidate.RouteSlotID); !errors.Is(err, routeledger.ErrSlotNotFound) {
		if err == nil {
			return RouteResolution{}, routeledger.ErrStaleTransition
		}
		return RouteResolution{}, err
	}
	approval, err := decodeOwnerApproval(candidate.ApprovalEvidence)
	if err != nil {
		return RouteResolution{}, err
	}
	execution, gate, err := newAuthorizedRouteExecution(candidate.ID, string(routeledger.TransitionBootstrap), approval, candidate.Verification, candidate.CertificateEvidence.Ref, acceptance, candidate.PreparedAt, candidate.Bootstrap)
	if err != nil {
		return RouteResolution{}, err
	}
	command := execution.command(gate)
	return a.transitionAuthorizedWithEvidence(ctx, command, []routeledger.AuthorizationEvidence{gate, candidate.ApprovalEvidence, candidate.CertificateEvidence})
}

func (a *RouteAuthority) applyFrozenPromotion(ctx context.Context, candidate FrozenRoutePromotionCandidate, acceptance G3PromotionAcceptance, rollback bool) (RouteResolution, error) {
	if err := candidate.Validate(); err != nil {
		return RouteResolution{}, err
	}
	if err := a.verifyFrozenOwnerApproval(candidate.Route.Slot.ID, candidate.Verification, candidate.ApprovalEvidence); err != nil {
		return RouteResolution{}, err
	}
	if err := acceptance.verify(a.promotionKey, candidate); err != nil {
		return RouteResolution{}, err
	}
	current, err := a.Resolve(ctx, candidate.Route.Slot.ID)
	if err != nil {
		return RouteResolution{}, err
	}
	plan := candidate.Promote
	if rollback {
		if err := verifyPromotedSuccessor(current, candidate, acceptance); err != nil {
			return RouteResolution{}, err
		}
		plan = candidate.Rollback
	} else if current.Slot.Generation != candidate.Route.Slot.Generation || !routeledger.SameVersion(current.Slot.Current, candidate.Route.Slot.Current) || current.Slot.LatestReceiptID != candidate.Route.Slot.LatestReceiptID {
		return RouteResolution{}, routeledger.ErrStaleTransition
	}
	approval, err := decodeOwnerApproval(candidate.ApprovalEvidence)
	if err != nil {
		return RouteResolution{}, err
	}
	execution, gate, err := newAuthorizedRouteExecution(candidate.ID, string(plan.Kind), approval, candidate.Verification, candidate.CertificateEvidence.Ref, acceptance, candidate.PreparedAt, plan)
	if err != nil {
		return RouteResolution{}, err
	}
	command := execution.command(gate)
	evidence := []routeledger.AuthorizationEvidence{gate, candidate.ApprovalEvidence, candidate.CertificateEvidence}
	return a.transitionAuthorizedWithEvidence(ctx, command, evidence)
}

func verifyPromotedSuccessor(current RouteResolution, candidate FrozenRoutePromotionCandidate, acceptance G3PromotionAcceptance) error {
	approval, err := decodeOwnerApproval(candidate.ApprovalEvidence)
	if err != nil {
		return err
	}
	execution, gate, err := newAuthorizedRouteExecution(candidate.ID, string(routeledger.TransitionPromote), approval, candidate.Verification, candidate.CertificateEvidence.Ref, acceptance, candidate.PreparedAt, candidate.Promote)
	if err != nil {
		return err
	}
	command := execution.command(gate)
	receipt := current.LatestReceipt
	if current.TransitionReceipt != nil {
		receipt = *current.TransitionReceipt
	}
	if current.Slot.Generation != candidate.Route.Slot.Generation+1 || !routeledger.SameVersion(current.Slot.Current, candidate.Verification.Version) || current.Slot.LatestReceiptID != receipt.ID || !transitionReceiptMatchesCommand(receipt, command) {
		return routeledger.ErrStaleTransition
	}
	return nil
}

func (a *RouteAuthority) PinCode(ctx context.Context, closure computerversion.CodeClosure) (computerversion.CodeClosure, error) {
	catalog, ok := a.inputs.(computerversion.ImmutableInputCatalog)
	if !ok {
		return computerversion.CodeClosure{}, fmt.Errorf("vmctl route authority: immutable input catalog is read-only")
	}
	return catalog.PinCode(ctx, closure)
}

func (a *RouteAuthority) PinArtifactProgram(ctx context.Context, program computerversion.ArtifactProgram) (computerversion.ArtifactProgram, error) {
	catalog, ok := a.inputs.(computerversion.ImmutableInputCatalog)
	if !ok {
		return computerversion.ArtifactProgram{}, fmt.Errorf("vmctl route authority: immutable input catalog is read-only")
	}
	return catalog.PinArtifactProgram(ctx, program)
}

func (a *RouteAuthority) PinAuthorizationEvidence(ctx context.Context, evidence routeledger.AuthorizationEvidence) (routeledger.AuthorizationEvidence, error) {
	return a.ledger.PinAuthorizationEvidence(ctx, evidence)
}

func (h *Handler) requireComputerVersionRoute(ctx context.Context, userID, desktopID string) error {
	if h.routeAuthority == nil {
		if h.routeAuthorityRequired {
			return fmt.Errorf("vmctl: ComputerVersion route authority is required but unavailable")
		}
		return nil
	}
	slotID, err := routeledger.RouteSlotID(userID, desktopID)
	if err != nil {
		return err
	}
	resolution, err := h.routeAuthority.Resolve(ctx, slotID)
	if err != nil {
		return fmt.Errorf("vmctl: immutable ComputerVersion route %s unavailable: %w", slotID, err)
	}
	if own := h.registry.GetOwnershipForDesktop(userID, desktopID); own != nil && own.SnapshotKind == "constructed-computer-version" {
		if !own.ConstructionCommitted || validateConstructedOwnership(own) != nil {
			return fmt.Errorf("vmctl: constructed lifecycle is not finalized for D-ROUTE")
		}
		if *own.ConstructionVersion != resolution.Slot.Current {
			return fmt.Errorf("vmctl: constructed realization ComputerVersion does not match D-ROUTE")
		}
	}
	return nil
}

func (h *Handler) AuthorizeComputerVersionRoute(ctx context.Context, userID, desktopID string) error {
	return h.requireComputerVersionRoute(ctx, userID, desktopID)
}

func (h *Handler) RequireRouteAuthority() {
	h.routeAuthorityRequired = true
}

func (h *Handler) SetRouteAuthority(authority *RouteAuthority) {
	h.routeAuthority = authority
}

func (h *Handler) HandlePinComputerVersionCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "internal caller required"})
		return
	}
	if h.routeAuthority == nil {
		writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: "ComputerVersion route authority unavailable"})
		return
	}
	defer r.Body.Close()
	var closure computerversion.CodeClosure
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&closure); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid code closure"})
		return
	}
	pinned, err := h.routeAuthority.PinCode(r.Context(), closure)
	if err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, pinned)
}

func (h *Handler) HandlePinComputerVersionArtifactProgram(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "internal caller required"})
		return
	}
	if h.routeAuthority == nil {
		writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: "ComputerVersion route authority unavailable"})
		return
	}
	defer r.Body.Close()
	var program computerversion.ArtifactProgram
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&program); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid artifact program"})
		return
	}
	pinned, err := h.routeAuthority.PinArtifactProgram(r.Context(), program)
	if err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, pinned)
}

func (h *Handler) HandlePinComputerVersionAuthorizationEvidence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "internal caller required"})
		return
	}
	if h.routeAuthority == nil {
		writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: "ComputerVersion route authority unavailable"})
		return
	}
	defer r.Body.Close()
	var evidence routeledger.AuthorizationEvidence
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&evidence); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid authorization evidence"})
		return
	}
	pinned, err := h.routeAuthority.PinAuthorizationEvidence(r.Context(), evidence)
	if err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, pinned)
}

type prepareRoutePromotionRequest struct {
	RouteSlotID  string                             `json:"route_slot_id"`
	Construction computerversion.ConstructionResult `json:"construction"`
	Approval     OwnerPromotionApproval             `json:"approval"`
}

type disposeConstructedCandidateRequest struct {
	RouteSlotID   string                          `json:"route_slot_id"`
	RealizationID string                          `json:"realization_id"`
	Version       computerversion.ComputerVersion `json:"computer_version"`
	DiskReceiptID string                          `json:"disk_receipt_id"`
}

type ConstructedCandidateDisposalReceipt struct {
	RouteSlotID   string                          `json:"route_slot_id"`
	RealizationID string                          `json:"realization_id"`
	Version       computerversion.ComputerVersion `json:"computer_version"`
	DiskReceiptID string                          `json:"disk_receipt_id"`
	PriorState    VMState                         `json:"prior_state"`
	DisposedAt    time.Time                       `json:"disposed_at"`
	RouteAbsent   bool                            `json:"route_absent"`
}

type disposeRoutedConstructedRealizationRequest struct {
	RouteSlotID             string                          `json:"route_slot_id"`
	ExpectedGeneration      uint64                          `json:"expected_generation"`
	ExpectedLatestReceiptID string                          `json:"expected_latest_receipt_id"`
	RealizationID           string                          `json:"realization_id"`
	Version                 computerversion.ComputerVersion `json:"computer_version"`
	DiskReceiptID           string                          `json:"disk_receipt_id"`
}

type RoutedConstructedRealizationDisposalReceipt struct {
	RouteSlotID     string                          `json:"route_slot_id"`
	RouteGeneration uint64                          `json:"route_generation"`
	LatestReceiptID string                          `json:"latest_receipt_id"`
	RealizationID   string                          `json:"realization_id"`
	Version         computerversion.ComputerVersion `json:"computer_version"`
	DiskReceiptID   string                          `json:"disk_receipt_id"`
	PriorState      VMState                         `json:"prior_state"`
	DisposedAt      time.Time                       `json:"disposed_at"`
	RoutePreserved  bool                            `json:"route_preserved"`
}

func (a *RouteAuthority) disposeUnroutedConstructedCandidate(ctx context.Context, registry *OwnershipRegistry, request disposeConstructedCandidateRequest, disposedAt time.Time) (ConstructedCandidateDisposalReceipt, error) {
	if a == nil || a.ledger == nil || registry == nil {
		return ConstructedCandidateDisposalReceipt{}, fmt.Errorf("vmctl candidate disposal: route authority and ownership registry are required")
	}
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()
	if _, _, err := a.ledger.Resolve(ctx, request.RouteSlotID); !errors.Is(err, routeledger.ErrSlotNotFound) {
		if err == nil {
			return ConstructedCandidateDisposalReceipt{}, fmt.Errorf("vmctl candidate disposal: route slot is present")
		}
		return ConstructedCandidateDisposalReceipt{}, fmt.Errorf("vmctl candidate disposal: resolve route slot: %w", err)
	}
	priorState, err := registry.disposeConstructedCandidateExact(request.RouteSlotID, request.RealizationID, request.Version, request.DiskReceiptID)
	if err != nil {
		return ConstructedCandidateDisposalReceipt{}, err
	}
	if _, _, err := a.ledger.Resolve(ctx, request.RouteSlotID); !errors.Is(err, routeledger.ErrSlotNotFound) {
		return ConstructedCandidateDisposalReceipt{}, fmt.Errorf("vmctl candidate disposal: route absence changed during disposal")
	}
	return ConstructedCandidateDisposalReceipt{RouteSlotID: request.RouteSlotID, RealizationID: request.RealizationID, Version: request.Version, DiskReceiptID: request.DiskReceiptID, PriorState: priorState, DisposedAt: disposedAt.UTC(), RouteAbsent: true}, nil
}

func (a *RouteAuthority) disposeRoutedConstructedRealization(ctx context.Context, registry *OwnershipRegistry, request disposeRoutedConstructedRealizationRequest, disposedAt time.Time) (RoutedConstructedRealizationDisposalReceipt, error) {
	if a == nil || a.ledger == nil || registry == nil {
		return RoutedConstructedRealizationDisposalReceipt{}, fmt.Errorf("vmctl routed realization disposal: route authority and ownership registry are required")
	}
	if request.ExpectedGeneration == 0 || strings.TrimSpace(request.ExpectedLatestReceiptID) == "" {
		return RoutedConstructedRealizationDisposalReceipt{}, fmt.Errorf("vmctl routed realization disposal: exact route receipt binding is required")
	}
	a.mutationMu.Lock()
	defer a.mutationMu.Unlock()
	beforeSlot, beforeReceipt, err := a.ledger.Resolve(ctx, request.RouteSlotID)
	if err != nil {
		return RoutedConstructedRealizationDisposalReceipt{}, fmt.Errorf("vmctl routed realization disposal: resolve route slot: %w", err)
	}
	if beforeSlot.ID != request.RouteSlotID || beforeSlot.Generation != request.ExpectedGeneration || string(beforeSlot.LatestReceiptID) != request.ExpectedLatestReceiptID || !routeledger.SameVersion(beforeSlot.Current, request.Version) || beforeReceipt.ID != beforeSlot.LatestReceiptID || beforeReceipt.RouteSlotID != beforeSlot.ID || beforeReceipt.CommittedGeneration != beforeSlot.Generation || !routeledger.SameVersion(beforeReceipt.New, beforeSlot.Current) {
		return RoutedConstructedRealizationDisposalReceipt{}, fmt.Errorf("vmctl routed realization disposal: route receipt bindings do not match")
	}
	priorState, err := registry.disposeConstructedCandidateExact(request.RouteSlotID, request.RealizationID, request.Version, request.DiskReceiptID)
	if err != nil {
		return RoutedConstructedRealizationDisposalReceipt{}, err
	}
	afterSlot, afterReceipt, err := a.ledger.Resolve(ctx, request.RouteSlotID)
	if err != nil || afterSlot != beforeSlot || afterReceipt != beforeReceipt {
		return RoutedConstructedRealizationDisposalReceipt{}, fmt.Errorf("vmctl routed realization disposal: route receipt changed during disposal")
	}
	return RoutedConstructedRealizationDisposalReceipt{
		RouteSlotID: request.RouteSlotID, RouteGeneration: beforeSlot.Generation, LatestReceiptID: string(beforeSlot.LatestReceiptID),
		RealizationID: request.RealizationID, Version: request.Version, DiskReceiptID: request.DiskReceiptID,
		PriorState: priorState, DisposedAt: disposedAt.UTC(), RoutePreserved: true,
	}, nil
}

func (h *Handler) HandleDisposeRoutedConstructedRealization(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !isInternalCaller(r) || h.routeAuthority == nil || h.registry == nil {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "exact routed realization disposal unavailable"})
		return
	}
	defer r.Body.Close()
	var request disposeRoutedConstructedRealizationRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid routed realization disposal request"})
		return
	}
	receipt, err := h.routeAuthority.disposeRoutedConstructedRealization(r.Context(), h.registry, request, time.Now().UTC())
	if err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, receipt)
}

func (h *Handler) HandleDisposeUnroutedConstructedCandidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !isInternalCaller(r) || h.routeAuthority == nil || h.registry == nil {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "exact constructed candidate disposal unavailable"})
		return
	}
	defer r.Body.Close()
	var request disposeConstructedCandidateRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid constructed candidate disposal request"})
		return
	}
	receipt, err := h.routeAuthority.disposeUnroutedConstructedCandidate(r.Context(), h.registry, request, time.Now().UTC())
	if err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, receipt)
}

func (h *Handler) HandlePrepareComputerVersionRouteBootstrap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !isInternalCaller(r) || h.routeAuthority == nil || h.construction == nil {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "signed bootstrap preparation unavailable"})
		return
	}
	defer r.Body.Close()
	var request prepareRoutePromotionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid route bootstrap request"})
		return
	}
	ownerID, computerID, err := routeledger.ParseRouteSlotID(request.RouteSlotID)
	if err != nil || request.Construction.Identity.OwnerID != ownerID || request.Construction.Identity.DesktopID != computerID || request.Construction.Identity.CandidateID != computerID {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "construction identity does not bind route slot"})
		return
	}
	verification, err := h.construction.verify(r.Context(), request.Construction)
	if err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
		return
	}
	candidate, err := h.routeAuthority.prepareBootstrap(request.RouteSlotID, verification, request.Approval, time.Now().UTC())
	if err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, candidate)
}

func (h *Handler) HandlePrepareComputerVersionRoutePromotion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "internal caller required"})
		return
	}
	if h.routeAuthority == nil || h.construction == nil {
		writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: "ComputerVersion route authority or verifier unavailable"})
		return
	}
	defer r.Body.Close()
	var request prepareRoutePromotionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid route promotion request"})
		return
	}
	ownerID, computerID, err := routeledger.ParseRouteSlotID(request.RouteSlotID)
	if err != nil || request.Construction.Identity.OwnerID != ownerID || request.Construction.Identity.DesktopID != computerID || request.Construction.Identity.CandidateID != computerID {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "construction identity does not bind route slot"})
		return
	}
	verification, err := h.construction.verify(r.Context(), request.Construction)
	if err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
		return
	}
	candidate, err := h.routeAuthority.preparePromotion(r.Context(), request.RouteSlotID, verification, request.Approval, time.Now().UTC())
	if err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, candidate)
}

type applyFrozenPromotionRequest struct {
	Candidate  FrozenRoutePromotionCandidate `json:"candidate"`
	Acceptance G3PromotionAcceptance         `json:"acceptance"`
	Action     string                        `json:"action"`
}

type applyFrozenBootstrapRequest struct {
	Candidate  FrozenRouteBootstrapCandidate `json:"candidate"`
	Acceptance G3PromotionAcceptance         `json:"acceptance"`
}

func (h *Handler) HandleApplyFrozenComputerVersionBootstrap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !isInternalCaller(r) || h.routeAuthority == nil {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "signed bootstrap unavailable"})
		return
	}
	defer r.Body.Close()
	var request applyFrozenBootstrapRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid frozen bootstrap request"})
		return
	}
	resolution, err := h.routeAuthority.applyFrozenBootstrap(r.Context(), request.Candidate, request.Acceptance)
	if err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, resolution)
}

func (h *Handler) HandleApplyFrozenComputerVersionPromotion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "internal caller required"})
		return
	}
	if h.routeAuthority == nil {
		writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: "ComputerVersion route authority unavailable"})
		return
	}
	defer r.Body.Close()
	var request applyFrozenPromotionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || (request.Action != "promote" && request.Action != "rollback") {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid frozen promotion request"})
		return
	}
	resolution, err := h.routeAuthority.applyFrozenPromotion(r.Context(), request.Candidate, request.Acceptance, request.Action == "rollback")
	if err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, resolution)
}

func (h *Handler) HandleResolveComputerVersionRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "internal caller required"})
		return
	}
	if h.routeAuthority == nil {
		writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: "ComputerVersion route authority unavailable"})
		return
	}
	resolution, err := h.routeAuthority.Resolve(r.Context(), r.URL.Query().Get("route_slot_id"))
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, routeledger.ErrSlotNotFound) {
			status = http.StatusNotFound
		}
		writeVMCTLJSON(w, status, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, resolution)
}

func (h *Handler) HandleTransitionComputerVersionRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "internal caller required"})
		return
	}
	if h.routeAuthority == nil {
		writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: "ComputerVersion route authority unavailable"})
		return
	}
	defer r.Body.Close()
	var command routeledger.TransitionCommand
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&command); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid route transition"})
		return
	}
	resolution, err := h.routeAuthority.Transition(r.Context(), command)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, routeledger.ErrStaleTransition) || errors.Is(err, routeledger.ErrIdempotencyReuse) {
			status = http.StatusConflict
		} else if errors.Is(err, routeledger.ErrSlotNotFound) || errors.Is(err, computerversion.ErrInputNotFound) {
			status = http.StatusNotFound
		}
		writeVMCTLJSON(w, status, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, resolution)
}

func PinComputerVersionCodeEndpoint(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/internal/vmctl/computer-version-inputs/pin-code"
}

func PinComputerVersionArtifactProgramEndpoint(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/internal/vmctl/computer-version-inputs/pin-artifact-program"
}

func PinComputerVersionAuthorizationEvidenceEndpoint(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/internal/vmctl/computer-version-routes/pin-authorization-evidence"
}

func PrepareComputerVersionRouteBootstrapEndpoint(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/internal/vmctl/computer-version-routes/prepare-bootstrap"
}
func ApplyFrozenComputerVersionBootstrapEndpoint(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/internal/vmctl/computer-version-routes/apply-bootstrap"
}

func ApplyFrozenComputerVersionPromotionEndpoint(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/internal/vmctl/computer-version-routes/apply-promotion"
}

func PrepareComputerVersionRoutePromotionEndpoint(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/internal/vmctl/computer-version-routes/prepare-promotion"
}

func ResolveComputerVersionRouteEndpoint(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/internal/vmctl/computer-version-routes/resolve"
}

func TransitionComputerVersionRouteEndpoint(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/internal/vmctl/computer-version-routes/transition"
}
