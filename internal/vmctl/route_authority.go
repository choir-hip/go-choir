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
	if _, err := h.routeAuthority.Resolve(ctx, slotID); err != nil {
		return fmt.Errorf("vmctl: immutable ComputerVersion route %s unavailable: %w", slotID, err)
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
