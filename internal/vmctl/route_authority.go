package vmctl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
)

type RouteResolution struct {
	Slot              routeledger.Slot                `json:"slot"`
	LatestReceipt     routeledger.TransitionReceipt   `json:"latest_receipt"`
	TransitionReceipt *routeledger.TransitionReceipt  `json:"transition_receipt,omitempty"`
	RouteAbsent       bool                            `json:"route_absent,omitempty"`
	CodeClosure       computerversion.CodeClosure     `json:"code_closure"`
	ArtifactProgram   computerversion.ArtifactProgram `json:"artifact_program"`
}

type routeAuthorityReader interface {
	Resolve(context.Context, string) (routeledger.Slot, routeledger.TransitionReceipt, error)
	ResolveAuthorizationEvidence(context.Context, string) (routeledger.AuthorizationEvidence, error)
}

type RouteAuthority struct {
	ledger       routeAuthorityReader
	sqlLedger    *routeledger.SQLLedger
	memoryLedger *routeledger.MemoryLedger
	inputs       computerversion.ImmutableInputResolver
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

func (a *RouteAuthority) Resolve(ctx context.Context, slotID string) (RouteResolution, error) {
	if a == nil || a.ledger == nil || a.inputs == nil {
		return RouteResolution{}, fmt.Errorf("vmctl route authority: not configured")
	}
	slotID = strings.TrimSpace(slotID)
	if _, _, err := routeledger.ParseRouteSlotID(slotID); err != nil {
		return RouteResolution{}, err
	}
	slot, receipt, err := a.ledger.Resolve(ctx, slotID)
	if errors.Is(err, routeledger.ErrSlotNotFound) {
		return RouteResolution{RouteAbsent: true}, nil
	}
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

type constructedVerification struct {
	ID            string                          `json:"verification_receipt_id"`
	Verifier      string                          `json:"verifier"`
	Version       computerversion.ComputerVersion `json:"computer_version"`
	DiskReceiptID string                          `json:"disk_receipt_id"`
	VMID          string                          `json:"vm_id"`
}

type constructedBootstrapCertificate struct {
	Kind         string                  `json:"kind"`
	RouteSlotID  string                  `json:"route_slot_id"`
	Verification constructedVerification `json:"verification"`
	ApprovalRef  string                  `json:"approval_ref"`
}

type constructedRouteExecution struct {
	CandidateID      string                  `json:"candidate_id"`
	OwnerApprovalRef string                  `json:"owner_approval_ref"`
	Verification     constructedVerification `json:"verification"`
}

type constructedPromotionCertificate struct {
	RouteSlot     string                          `json:"route_slot"`
	Candidate     computerversion.ComputerVersion `json:"candidate"`
	EvidenceRef   string                          `json:"evidence_ref"`
	OwnerApproved bool                            `json:"owner_approved"`
}

// constructedOwnershipIdentity recovers the immutable construction binding
// from the canonical route/evidence ledger. Ownership JSON intentionally does
// not duplicate this authority.
func (a *RouteAuthority) constructedOwnershipIdentity(ctx context.Context, ownerID, desktopID, vmID string) (*computerversion.ComputerVersion, string, bool, error) {
	if a == nil || a.ledger == nil {
		return nil, "", false, fmt.Errorf("route/evidence authority is unavailable")
	}
	slotID, err := routeledger.RouteSlotID(ownerID, desktopID)
	if err != nil {
		return nil, "", false, err
	}
	slot, receipt, err := a.ledger.Resolve(ctx, slotID)
	if errors.Is(err, routeledger.ErrSlotNotFound) {
		return nil, "", false, nil
	}
	if err != nil {
		return nil, "", false, fmt.Errorf("resolve ownership route: %w", err)
	}
	gate, err := a.ledger.ResolveAuthorizationEvidence(ctx, string(receipt.ApprovalRef))
	if err != nil {
		return nil, "", false, fmt.Errorf("resolve route execution evidence: %w", err)
	}
	certificate, err := a.ledger.ResolveAuthorizationEvidence(ctx, string(receipt.PromotionCertificateRef))
	if err != nil {
		return nil, "", false, fmt.Errorf("resolve route certificate evidence: %w", err)
	}
	if gate.Kind != routeledger.AuthorizationEvidenceApproval ||
		certificate.Kind != routeledger.AuthorizationEvidencePromotionCertificate ||
		gate.Ref != string(receipt.ApprovalRef) ||
		certificate.Ref != string(receipt.PromotionCertificateRef) ||
		gate.RouteSlotID != slot.ID || certificate.RouteSlotID != slot.ID ||
		!routeledger.SameVersion(gate.ComputerVersion, slot.Current) ||
		!routeledger.SameVersion(certificate.ComputerVersion, slot.Current) {
		return nil, "", false, fmt.Errorf("ownership %s route/evidence refs do not join", vmID)
	}

	var execution constructedRouteExecution
	if err := json.Unmarshal(gate.Payload, &execution); err != nil ||
		strings.TrimSpace(execution.CandidateID) == "" ||
		strings.TrimSpace(execution.OwnerApprovalRef) == "" {
		return nil, "", false, fmt.Errorf("ownership %s has unrecognized route execution evidence", vmID)
	}
	ownerApproval, err := a.ledger.ResolveAuthorizationEvidence(ctx, execution.OwnerApprovalRef)
	if err != nil {
		return nil, "", false, fmt.Errorf("resolve constructed owner approval evidence: %w", err)
	}
	if ownerApproval.Kind != routeledger.AuthorizationEvidenceApproval ||
		ownerApproval.Ref != execution.OwnerApprovalRef ||
		ownerApproval.RouteSlotID != slot.ID ||
		!routeledger.SameVersion(ownerApproval.ComputerVersion, slot.Current) {
		return nil, "", false, fmt.Errorf("constructed ownership %s owner approval join failed", vmID)
	}

	var bootstrap constructedBootstrapCertificate
	if err := json.Unmarshal(certificate.Payload, &bootstrap); err == nil && bootstrap.Kind == "verified_route_bootstrap" {
		if bootstrap.RouteSlotID != slot.ID ||
			bootstrap.ApprovalRef != execution.OwnerApprovalRef ||
			bootstrap.Verification != execution.Verification {
			return nil, "", false, fmt.Errorf("constructed ownership %s bootstrap certificate join failed", vmID)
		}
	} else {
		var promotion constructedPromotionCertificate
		if err := json.Unmarshal(certificate.Payload, &promotion); err != nil ||
			promotion.RouteSlot != slot.ID ||
			!routeledger.SameVersion(promotion.Candidate, slot.Current) ||
			promotion.EvidenceRef != execution.Verification.ID ||
			!promotion.OwnerApproved {
			return nil, "", false, fmt.Errorf("constructed ownership %s promotion certificate join failed", vmID)
		}
	}
	verification := execution.Verification
	if strings.TrimSpace(verification.ID) == "" ||
		verification.Verifier != "independent-production-realization-verifier" ||
		!routeledger.SameVersion(verification.Version, slot.Current) ||
		strings.TrimSpace(verification.DiskReceiptID) == "" ||
		strings.TrimSpace(verification.VMID) != strings.TrimSpace(vmID) {
		return nil, "", false, fmt.Errorf("constructed ownership %s verification join failed", vmID)
	}
	version := slot.Current
	return &version, verification.DiskReceiptID, true, nil
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

func (a *RouteAuthority) Transition(ctx context.Context, command routeledger.TransitionCommand) (RouteResolution, error) {
	return RouteResolution{}, fmt.Errorf("vmctl route authority: every route CAS requires a signed frozen candidate")
}

func (a *RouteAuthority) transitionSelfDevelopmentWithEvidence(ctx context.Context, command routeledger.TransitionCommand, evidence []routeledger.AuthorizationEvidence) (RouteResolution, error) {
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
		slot, receipt, err = a.sqlLedger.ApplySelfDevelopmentTransition(ctx, command, evidence)
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

func (h *Handler) resolveComputerVersionRoute(ctx context.Context, userID, desktopID string) (RouteResolution, bool, error) {
	if h.routeAuthority == nil {
		if h.routeAuthorityRequired {
			return RouteResolution{}, false, fmt.Errorf("vmctl: ComputerVersion route authority is required but unavailable")
		}
		return RouteResolution{}, false, nil
	}
	slotID, err := routeledger.RouteSlotID(userID, desktopID)
	if err != nil {
		return RouteResolution{}, false, err
	}
	resolution, err := h.routeAuthority.Resolve(ctx, slotID)
	if err != nil {
		return RouteResolution{}, true, fmt.Errorf("vmctl: immutable ComputerVersion route %s unavailable: %w", slotID, err)
	}
	if resolution.RouteAbsent &&
		userID == UniversalWirePlatformOwnerID && desktopID == UniversalWirePlatformDesktopID {
		return RouteResolution{}, true, fmt.Errorf("vmctl: platform computer requires an immutable ComputerVersion route")
	}
	return resolution, true, nil
}

func (h *Handler) requireComputerVersionRoute(ctx context.Context, userID, desktopID string) error {
	resolution, known, err := h.resolveComputerVersionRoute(ctx, userID, desktopID)
	if err != nil || !known || resolution.RouteAbsent {
		return err
	}
	ownership := h.registry.GetOwnershipForDesktop(userID, desktopID)
	if ownership == nil {
		return fmt.Errorf("vmctl: immutable ComputerVersion route has no matching realized ownership")
	}
	_, _, constructed, err := h.routeAuthority.constructedOwnershipIdentity(ctx, userID, desktopID, ownership.VMID)
	if err != nil {
		return fmt.Errorf("vmctl: immutable ComputerVersion route does not join realized ownership: %w", err)
	}
	if !constructed {
		return fmt.Errorf("vmctl: immutable ComputerVersion route does not join realized ownership")
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

type ComputerVersionInputs struct {
	CodeClosure     computerversion.CodeClosure     `json:"code_closure"`
	ArtifactProgram computerversion.ArtifactProgram `json:"artifact_program"`
}

func (h *Handler) HandleResolveComputerVersionInputs(w http.ResponseWriter, r *http.Request) {
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
	var version computerversion.ComputerVersion
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&version); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid ComputerVersion"})
		return
	}
	closure, program, err := h.routeAuthority.resolveVersionInputs(r.Context(), version)
	if err != nil {
		writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, ComputerVersionInputs{CodeClosure: closure, ArtifactProgram: program})
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

func ResolveComputerVersionInputsEndpoint(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/internal/vmctl/computer-version-inputs/resolve"
}

func ResolveComputerVersionRouteEndpoint(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/internal/vmctl/computer-version-routes/resolve"
}

func ApplySelfDevelopmentRouteProjectionEndpoint(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/internal/vmctl/computer-version-routes/apply-self-development"
}
