package vmctl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

func (a *RouteAuthority) ApplySelfDevelopmentProjection(ctx context.Context, registry *OwnershipRegistry, request selfdevprotocol.ApplyRouteProjectionRequest, now time.Time) (RouteResolution, error) {
	if !strings.HasPrefix(request.Projection.DecisionScope, "computer:self_development:") {
		return RouteResolution{}, fmt.Errorf("vmctl self-development projection: owner-decision scope is required")
	}
	return a.applySignedProjection(ctx, registry, request, now, "vmctl self-development projection")
}

// ApplyPlatformFollowRouteProjection is the platform-follow evidence class:
// the same signature/join/staleness verification as the owner self-dev path,
// gated to the platform-follow scope and promote transitions. A tracking
// computer's update push uses this endpoint; it never mints or accepts an
// owner-decision scope.
func (a *RouteAuthority) ApplyPlatformFollowRouteProjection(ctx context.Context, registry *OwnershipRegistry, request selfdevprotocol.ApplyRouteProjectionRequest, now time.Time) (RouteResolution, error) {
	if request.Projection.DecisionScope != selfdevprotocol.PlatformUpdateFollowScope ||
		request.Projection.DecisionActor != selfdevprotocol.PlatformUpdateFollowActor ||
		request.Projection.Command.Kind != routeledger.TransitionPromote {
		return RouteResolution{}, fmt.Errorf("vmctl platform-follow projection: platform-follow promote scope is required")
	}
	return a.applySignedProjection(ctx, registry, request, now, "vmctl platform-follow projection")
}

func (a *RouteAuthority) applySignedProjection(ctx context.Context, registry *OwnershipRegistry, request selfdevprotocol.ApplyRouteProjectionRequest, now time.Time, errPrefix string) (RouteResolution, error) {
	if a == nil || registry == nil || now.IsZero() {
		return RouteResolution{}, fmt.Errorf("%s: complete route authority is required", errPrefix)
	}
	projection := request.Projection
	certificate, artifact, err := selfdevprotocol.RouteProjectionFromRequest(projection, request.Authorization.Receipt.IssuedAt)
	if err != nil || !reflect.DeepEqual(request.Authorization.Certificate, certificate) {
		return RouteResolution{}, fmt.Errorf("%s: certificate does not bind the exact request", errPrefix)
	}
	requestCommitment, err := selfdevprotocol.Digest(projection)
	if err != nil || request.Authorization.Receipt.Kind != selfdevprotocol.ReceiptKindRouteProjection || request.Authorization.Receipt.ComputerID != projection.ComputerID || request.Authorization.Receipt.RequestCommitment != requestCommitment || request.Authorization.Receipt.ArtifactDigest != computerevent.DigestBytes(artifact) {
		return RouteResolution{}, fmt.Errorf("%s: authorization receipt bindings are invalid", errPrefix)
	}
	publicKey, err := registry.platformControlPublicKey(ctx, request.Authorization.Receipt.Signer)
	if err != nil || request.Authorization.Receipt.Verify(publicKey) != nil || projection.Checkpoint.Receipt.Verify(publicKey) != nil {
		return RouteResolution{}, fmt.Errorf("%s: platform-control signatures refused", errPrefix)
	}
	ownerID, desktopID, err := routeledger.ParseRouteSlotID(projection.Command.RouteSlotID)
	if err != nil {
		return RouteResolution{}, err
	}
	registry.mu.RLock()
	ownership := registry.ownerships[ownershipKey(ownerID, desktopID)]
	bound := ownership != nil && ownership.UserID == ownerID && normalizeDesktopID(ownership.DesktopID) == desktopID &&
		stableComputerID(ownership.UserID, ownership.DesktopID, ownership.ComputerID) == projection.ComputerID &&
		ownership.State != VMStateStopping && ownership.State != VMStateStopped
	registry.mu.RUnlock()
	if !bound {
		return RouteResolution{}, fmt.Errorf("%s: presenter does not own the routed computer", errPrefix)
	}
	current, err := a.Resolve(ctx, projection.Command.RouteSlotID)
	if err != nil {
		return RouteResolution{}, err
	}
	if routeledger.ReceiptMatchesCommand(current.LatestReceipt, projection.Command) {
		current.TransitionReceipt = &current.LatestReceipt
		return current, nil
	}
	expiresAt, expiryErr := time.Parse(time.RFC3339Nano, projection.ExpiresAt)
	if expiryErr != nil || !now.UTC().Before(expiresAt) {
		return RouteResolution{}, fmt.Errorf("%s: certificate expired", errPrefix)
	}
	if current.Slot.Current != projection.Command.Old || current.Slot.Generation != projection.Command.ExpectedGeneration || current.Slot.LatestReceiptID == "" {
		return RouteResolution{}, routeledger.ErrStaleTransition
	}
	if projection.Command.Kind == routeledger.TransitionRollback && projection.Command.RollbackTargetReceiptID == "" {
		return RouteResolution{}, fmt.Errorf("%s: rollback target receipt is required", errPrefix)
	}
	if projection.Command.Kind != routeledger.TransitionPromote && projection.Command.Kind != routeledger.TransitionRollback {
		return RouteResolution{}, fmt.Errorf("%s: only promote or rollback projections are accepted", errPrefix)
	}
	if _, err := a.PinCode(ctx, projection.CodeClosure); err != nil {
		return RouteResolution{}, err
	}
	if _, err := a.PinArtifactProgram(ctx, projection.ArtifactProgram); err != nil {
		return RouteResolution{}, err
	}
	return a.transitionSelfDevelopmentWithEvidence(ctx, projection.Command, []routeledger.AuthorizationEvidence{projection.ApprovalEvidence, projection.PromotionEvidence})
}

func (h *Handler) HandleApplySelfDevelopmentRouteProjection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if r.Header.Get("X-Internal-Caller") != "true" {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "internal authorization required"})
		return
	}
	if h == nil || h.routeAuthority == nil || h.registry == nil {
		writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: "route authority unavailable"})
		return
	}
	var request selfdevprotocol.ApplyRouteProjectionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid self-development route projection"})
		return
	}
	resolution, err := h.routeAuthority.ApplySelfDevelopmentProjection(r.Context(), h.registry, request, time.Now().UTC())
	if err != nil {
		status := http.StatusBadRequest
		if err == routeledger.ErrStaleTransition {
			status = http.StatusConflict
		}
		writeVMCTLJSON(w, status, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, resolution)
}

// HandleApplyPlatformFollowRouteProjection serves the platform-follow
// evidence class on its own endpoint: identical verification mechanics to
// the owner self-dev path, but the decision scope must be the platform-follow
// scope and the transition must be a promote. The owner endpoint refuses
// platform scopes and this endpoint refuses owner scopes — the two evidence
// classes cannot be blended.
func (h *Handler) HandleApplyPlatformFollowRouteProjection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if r.Header.Get("X-Internal-Caller") != "true" {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "internal authorization required"})
		return
	}
	if h == nil || h.routeAuthority == nil || h.registry == nil {
		writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: "route authority unavailable"})
		return
	}
	var request selfdevprotocol.ApplyRouteProjectionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid platform-follow route projection"})
		return
	}
	resolution, err := h.routeAuthority.ApplyPlatformFollowRouteProjection(r.Context(), h.registry, request, time.Now().UTC())
	if err != nil {
		status := http.StatusBadRequest
		if err == routeledger.ErrStaleTransition {
			status = http.StatusConflict
		}
		writeVMCTLJSON(w, status, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, resolution)
}
