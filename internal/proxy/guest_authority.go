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
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

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

func (h *Handler) requireWirePlatformPublishCaller(r *http.Request) error {
	if trustedInternalProxyTransport(r) {
		if r.Header.Get("X-Internal-Caller") == "true" {
			return nil
		}
		return fmt.Errorf("internal caller required")
	}
	ownership, err := h.boundProxyGuest(r)
	if err != nil || ownership.UserID != vmctl.UniversalWirePlatformOwnerID || ownership.ComputerID != vmctl.UniversalWirePlatformComputerID {
		return fmt.Errorf("guest caller is not bound to the wire platform computer")
	}
	return nil
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
		if r.URL.Path == "/internal/platform/objects" || r.URL.Path == "/internal/platform/edges" {
			return nil
		}
		return fmt.Errorf("guest object graph reads must use a collection route")
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("read object graph request: %w", err)
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if strings.HasSuffix(r.URL.Path, "/objects") {
		var object objectgraph.Object
		if err := json.Unmarshal(body, &object); err != nil ||
			strings.TrimSpace(object.OwnerID) != strings.TrimSpace(ownership.UserID) ||
			strings.TrimSpace(object.ComputerID) != strings.TrimSpace(ownership.ComputerID) {
			return fmt.Errorf("guest caller is not bound to object owner and computer")
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
