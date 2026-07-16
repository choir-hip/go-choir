package vmctl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

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

type RouteAuthority struct {
	ledger   routeledger.Ledger
	inputs   computerversion.ImmutableInputResolver
	evidence routeledger.TransitionEvidenceResolver
}

func NewRouteAuthority(ledger routeledger.Ledger, inputs computerversion.ImmutableInputResolver, evidence routeledger.TransitionEvidenceResolver) (*RouteAuthority, error) {
	if ledger == nil || inputs == nil || evidence == nil {
		return nil, fmt.Errorf("vmctl route authority: ledger, immutable input resolver, and transition evidence resolver are required")
	}
	return &RouteAuthority{ledger: ledger, inputs: inputs, evidence: evidence}, nil
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

func (a *RouteAuthority) Transition(ctx context.Context, command routeledger.TransitionCommand) (RouteResolution, error) {
	if a == nil || a.ledger == nil || a.inputs == nil || a.evidence == nil {
		return RouteResolution{}, fmt.Errorf("vmctl route authority: not configured")
	}
	if err := command.Validate(); err != nil {
		return RouteResolution{}, err
	}
	if err := a.evidence.VerifyTransitionEvidence(ctx, command); err != nil {
		return RouteResolution{}, fmt.Errorf("vmctl route authority: transition evidence is not pinned: %w", err)
	}
	if _, _, err := a.resolveVersionInputs(ctx, command.New); err != nil {
		return RouteResolution{}, fmt.Errorf("vmctl route authority: new ComputerVersion inputs are not pinned: %w", err)
	}
	_, receipt, err := a.ledger.Transition(ctx, command)
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
	catalog, ok := a.evidence.(routeledger.TransitionEvidenceCatalog)
	if !ok {
		return routeledger.AuthorizationEvidence{}, fmt.Errorf("vmctl route authority: transition evidence catalog is read-only")
	}
	return catalog.PinAuthorizationEvidence(ctx, evidence)
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

func ResolveComputerVersionRouteEndpoint(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/internal/vmctl/computer-version-routes/resolve"
}

func TransitionComputerVersionRouteEndpoint(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/internal/vmctl/computer-version-routes/transition"
}
