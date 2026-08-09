package proxy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// requireAPIKeyComputerTarget joins a computer-selecting API-key request to the
// authenticated owner's stable ComputerID before any downstream route lookup,
// resolve, wake, dial, lifecycle intent, or product call. Registry binding is
// attenuation metadata; this vmctl ownership join is the live authority.
//
// pathComputerID is non-empty for routes whose path names a stable computer.
// requestedDesktopID is non-empty for routes that select a logical desktop
// through query, header, request body, or the default primary selector.
func (h *Handler) requireAPIKeyComputerTarget(
	w http.ResponseWriter,
	r *http.Request,
	authResult *AuthResult,
	pathComputerID string,
	requestedDesktopID string,
) (*resolvedComputerTarget, bool) {
	if authResult == nil || authResult.AuthMethod != "api_key" {
		return nil, true
	}
	boundComputerID := strings.TrimSpace(authResult.ComputerID)
	if boundComputerID == "" {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key requires an exact computer binding"})
		return nil, false
	}
	if named := strings.TrimSpace(pathComputerID); named != "" && named != boundComputerID {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key is bound to another computer"})
		return nil, false
	}
	if h.vmctlClient == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "computer ownership authority unavailable"})
		return nil, false
	}
	ownership, err := h.vmctlClient.LookupComputerContext(r.Context(), authResult.UserID, boundComputerID)
	if err != nil {
		if errors.Is(err, vmctl.ErrComputerLookupIdentityMismatch) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key computer ownership required"})
		} else {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "computer ownership authority unavailable"})
		}
		return nil, false
	}
	if ownership == nil || ownership.UserID != authResult.UserID || ownership.ComputerID != boundComputerID {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key computer ownership required"})
		return nil, false
	}
	canonicalDesktopID := strings.TrimSpace(ownership.DesktopID)
	if canonicalDesktopID == "" || canonicalDesktopID != ownership.DesktopID {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "computer ownership authority unavailable"})
		return nil, false
	}
	desktopSelectors := []string{strings.TrimSpace(requestedDesktopID)}
	if r != nil {
		for _, value := range r.URL.Query()["desktop_id"] {
			desktopSelectors = append(desktopSelectors, strings.TrimSpace(value))
		}
		for _, value := range r.Header.Values("X-Choir-Desktop") {
			desktopSelectors = append(desktopSelectors, strings.TrimSpace(value))
		}
	}
	for _, requested := range desktopSelectors {
		if requested != "" && canonicalDesktopID != requested {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key is bound to another desktop"})
			return nil, false
		}
	}
	return &resolvedComputerTarget{
		ComputerID: ownership.ComputerID,
		UserID:     ownership.UserID,
		DesktopID:  ownership.DesktopID,
		VMID:       strings.TrimSpace(ownership.VMID),
		SandboxURL: strings.TrimSpace(ownership.SandboxURL),
		State:      strings.TrimSpace(ownership.State),
		Epoch:      ownership.Epoch,
	}, true
}

// resolveSandboxURLForComputerTarget preserves the exact stable-computer join
// for API keys. Unlike cookie requests, a bearer request never re-resolves or
// wakes a logical desktop after authorization: it uses only the exact joined
// ownership's current active realization. Recovery remains an explicit
// computer route and rechecks the same stable ComputerID before effects.
func (h *Handler) resolveSandboxURLForComputerTarget(ctx context.Context, authResult *AuthResult, target *resolvedComputerTarget, desktopID string) (string, error) {
	if authResult == nil {
		return "", fmt.Errorf("authentication authority unavailable")
	}
	if authResult.AuthMethod != "api_key" {
		return h.resolveSandboxURL(ctx, authResult.UserID, desktopID)
	}
	if target == nil || target.UserID != authResult.UserID || target.ComputerID != authResult.ComputerID || target.DesktopID != desktopID {
		return "", fmt.Errorf("api key computer authority unavailable")
	}
	if target.State != "active" || target.SandboxURL == "" {
		return "", fmt.Errorf("api key computer realization unavailable")
	}
	if err := h.ensureComputerVersionRoute(ctx, target.UserID, target.DesktopID); err != nil {
		return "", err
	}
	return target.SandboxURL, nil
}
