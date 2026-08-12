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
// A key with an empty ComputerID is owner-wide: it reaches every interactive
// computer the owner owns, but every request must still name its target — via
// pathComputerID, a computer_id query parameter, or an X-Choir-Computer
// header — and the exact same (UserID, ComputerID) ownership join runs against
// that named target. No implicit default targeting exists for owner-wide keys.
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
	// Owner-wide keys must name their target — via pathComputerID, a
	// computer_id query parameter, or an X-Choir-Computer header — before any
	// infrastructure check, so an unnamed request fails closed as a client
	// error. The binding is attenuation metadata; this vmctl ownership join is
	// the live authority for both key kinds.
	var ownerWideNamed string
	if boundComputerID == "" {
		named := strings.TrimSpace(pathComputerID)
		if r != nil {
			if v := strings.TrimSpace(r.URL.Query().Get("computer_id")); v != "" {
				named = v
			} else if v := strings.TrimSpace(r.Header.Get("X-Choir-Computer")); v != "" {
				named = v
			}
		}
		if pathComputerID != "" && named != "" && named != strings.TrimSpace(pathComputerID) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key computer target mismatch"})
			return nil, false
		}
		if named == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "owner-wide api key requires a named computer (computer_id)"})
			return nil, false
		}
		ownerWideNamed = named
	}
	if h.vmctlClient == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "computer ownership authority unavailable"})
		return nil, false
	}
	if boundComputerID == "" {
		ownership, err := h.vmctlClient.LookupComputerContext(r.Context(), authResult.UserID, ownerWideNamed)
		if err != nil {
			if errors.Is(err, vmctl.ErrComputerLookupIdentityMismatch) {
				writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key computer ownership required"})
			} else {
				writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "computer ownership authority unavailable"})
			}
			return nil, false
		}
		if ownership == nil || ownership.UserID != authResult.UserID || ownership.ComputerID != ownerWideNamed || ownership.Kind != "" && ownership.Kind != vmctl.VMKindInteractive {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key computer ownership required"})
			return nil, false
		}
		canonicalDesktopID := strings.TrimSpace(ownership.DesktopID)
		if canonicalDesktopID == "" {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "computer ownership authority unavailable"})
			return nil, false
		}
		// Owner-wide keys derive their desktop from the joined ownership;
		// desktop selectors must still match it when explicitly provided.
		if r != nil {
			for _, v := range r.URL.Query()["desktop_id"] {
				if s := strings.TrimSpace(v); s != "" && s != canonicalDesktopID {
					writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key is bound to another desktop"})
					return nil, false
				}
			}
			for _, v := range r.Header.Values("X-Choir-Desktop") {
				if s := strings.TrimSpace(v); s != "" && s != canonicalDesktopID {
					writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key is bound to another desktop"})
					return nil, false
				}
			}
			if requested := strings.TrimSpace(requestedDesktopID); requested != "" && requested != canonicalDesktopID && requested != vmctl.PrimaryDesktopID {
				writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key is bound to another desktop"})
				return nil, false
			}
		}
		return &resolvedComputerTarget{
			ComputerID:  ownership.ComputerID,
			UserID:      ownership.UserID,
			DesktopID:   ownership.DesktopID,
			VMID:        strings.TrimSpace(ownership.VMID),
			ComputerURL: strings.TrimSpace(ownership.ComputerURL),
			State:       strings.TrimSpace(ownership.State),
			Epoch:       ownership.Epoch,
		}, true
	}
	if named := strings.TrimSpace(pathComputerID); named != "" && named != boundComputerID {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "api key is bound to another computer"})
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
	if ownership == nil || ownership.UserID != authResult.UserID || ownership.ComputerID != boundComputerID || ownership.Kind != "" && ownership.Kind != vmctl.VMKindInteractive {
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
		ComputerID:  ownership.ComputerID,
		UserID:      ownership.UserID,
		DesktopID:   ownership.DesktopID,
		VMID:        strings.TrimSpace(ownership.VMID),
		ComputerURL: strings.TrimSpace(ownership.ComputerURL),
		State:       strings.TrimSpace(ownership.State),
		Epoch:       ownership.Epoch,
	}, true
}

// resolveComputerURLForComputerTarget preserves the exact stable-computer join
// for API keys. Unlike cookie requests, a bearer request never re-resolves or
// wakes a logical desktop after authorization: it uses only the exact joined
// ownership's current active realization. Recovery remains an explicit
// computer route and rechecks the same stable ComputerID before effects.
func (h *Handler) resolveComputerURLForComputerTarget(ctx context.Context, authResult *AuthResult, target *resolvedComputerTarget, desktopID string) (string, error) {
	if authResult == nil {
		return "", fmt.Errorf("authentication authority unavailable")
	}
	if authResult.AuthMethod != "api_key" {
		return h.resolveComputerURL(ctx, authResult.UserID, desktopID)
	}
	if target == nil || target.UserID != authResult.UserID || target.ComputerID == "" {
		return "", fmt.Errorf("api key computer authority unavailable")
	}
	if authResult.ComputerID != "" && target.ComputerID != authResult.ComputerID {
		return "", fmt.Errorf("api key computer authority unavailable")
	}
	if target.State != "active" || target.ComputerURL == "" {
		return "", fmt.Errorf("api key computer realization unavailable")
	}
	if err := h.ensureComputerVersionRoute(ctx, target.UserID, target.DesktopID); err != nil {
		return "", err
	}
	return target.ComputerURL, nil
}
