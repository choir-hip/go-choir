package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// maxGuestCaptureRequestBytes matches corpusd's capture request bound.
const maxGuestCaptureRequestBytes = 64 << 20

func trustedInternalProxyTransport(r *http.Request) bool {
	if r == nil {
		return false
	}
	remoteAddr := strings.TrimSpace(r.RemoteAddr)
	if remoteAddr == "" || remoteAddr == "@" || strings.HasPrefix(remoteAddr, "/") {
		return true
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	return host == "127.0.0.1" || host == "::1" || strings.HasPrefix(host, "192.0.2.")
}

func (h *Handler) boundProxyGuest(r *http.Request) (*vmctl.GuestOwnership, error) {
	if h == nil || h.vmctlClient == nil {
		return nil, fmt.Errorf("guest caller binding unavailable")
	}
	ownership, err := h.vmctlClient.LookupGuestContext(r.Context(), r.RemoteAddr)
	if err != nil || ownership == nil || !ownership.Found || strings.TrimSpace(ownership.ComputerID) == "" {
		return nil, fmt.Errorf("guest caller is not bound to a live computer")
	}
	return ownership, nil
}

func (h *Handler) requirePlatformObjectGraphCaller(r *http.Request) error {
	if trustedInternalProxyTransport(r) {
		if r.Header.Get("X-Internal-Caller") == "true" {
			return nil
		}
		return fmt.Errorf("internal caller required")
	}
	ownership, err := h.boundProxyGuest(r)
	if err != nil {
		return err
	}
	if r.Method == http.MethodGet {
		// Captures and their bodies are shared Autopaper content: any bound
		// guest reads them (W1b2).
		if strings.HasPrefix(r.URL.Path, "/internal/platform/captures/") {
			return nil
		}
		// An object list is scoped to the guest's own owner and computer;
		// corpusd sees this request as the proxy's, so the proxy enforces it
		// (auth-boundary-gaps F5). No guest lists edges.
		if r.URL.Path == "/internal/platform/objects" {
			query := r.URL.Query()
			if strings.TrimSpace(query.Get("owner")) != strings.TrimSpace(ownership.UserID) ||
				strings.TrimSpace(query.Get("computer")) != strings.TrimSpace(ownership.ComputerID) {
				return fmt.Errorf("guest object list must name its own owner and computer")
			}
			return nil
		}
		return fmt.Errorf("guest object graph reads must use a collection route")
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxGuestCaptureRequestBytes+1))
	if err != nil {
		return fmt.Errorf("read object graph request: %w", err)
	}
	if len(body) > maxGuestCaptureRequestBytes {
		return fmt.Errorf("object graph request too large")
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if r.URL.Path == "/internal/platform/captures" || r.URL.Path == types.StoryVerdictsPath {
		// corpusd sees the proxy as an internal caller, so the guest's
		// ownership of the captures it writes is checked here, as for
		// object writes.
		var captures struct {
			OwnerID    string `json:"owner_id"`
			ComputerID string `json:"computer_id"`
		}
		if err := json.Unmarshal(body, &captures); err != nil ||
			strings.TrimSpace(captures.OwnerID) != strings.TrimSpace(ownership.UserID) ||
			strings.TrimSpace(captures.ComputerID) != strings.TrimSpace(ownership.ComputerID) {
			return fmt.Errorf("guest caller is not bound to capture owner and computer")
		}
		return nil
	}
	if strings.HasSuffix(r.URL.Path, "/objects") {
		var object objectgraph.Object
		if err := json.Unmarshal(body, &object); err != nil ||
			strings.TrimSpace(object.OwnerID) != strings.TrimSpace(ownership.UserID) ||
			strings.TrimSpace(object.ComputerID) != strings.TrimSpace(ownership.ComputerID) {
			return fmt.Errorf("guest caller is not bound to object owner and computer")
		}
		// corpusd sees this write as the proxy's, so the proxy keeps guests
		// off the generic route for captures: they go through the capture
		// route, which owns their identity and keeps bodies out of rows. A
		// PUT names the kind as object_kind, a POST create as kind.
		var kinds struct {
			ObjectKind string `json:"object_kind"`
			Kind       string `json:"kind"`
		}
		_ = json.Unmarshal(body, &kinds)
		if kinds.ObjectKind == string(objectgraph.WebCaptureObjectKind) || kinds.Kind == string(objectgraph.WebCaptureObjectKind) {
			return fmt.Errorf("captures are written through /internal/platform/captures")
		}
		return nil
	}
	var edge objectgraph.Edge
	if err := json.Unmarshal(body, &edge); err != nil {
		return fmt.Errorf("invalid object graph edge request")
	}
	for _, id := range []string{edge.FromID, edge.ToID} {
		_, ownerID, _, err := objectgraph.ParseCanonicalID(id)
		if err != nil || strings.TrimSpace(ownerID) != strings.TrimSpace(ownership.UserID) {
			return fmt.Errorf("guest caller is not bound to edge owner")
		}
	}
	return nil
}
