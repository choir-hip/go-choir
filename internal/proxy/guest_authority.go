package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/release"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// maxGuestCaptureRequestBytes matches corpusd's capture request bound.
const maxGuestCaptureRequestBytes = 64 << 20

func trustedInternalProxyTransport(r *http.Request) bool {
	return trustedInternalProxyTransportFor(r, testing.Testing())
}

// trustedInternalProxyTransportFor: loopback or a unix socket; TEST-NET-1
// (httptest's RemoteAddr) only inside a Go test binary, never in
// production (SEC-11; hostauth.TestNetworkAllowed).
func trustedInternalProxyTransportFor(r *http.Request, inTestBinary bool) bool {
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
	if host == "127.0.0.1" || host == "::1" {
		return true
	}
	ip := net.ParseIP(host).To4()
	return inTestBinary && ip != nil && ip[0] == 192 && ip[1] == 0 && ip[2] == 2
}

// objectGraphInternalAuth is the route's hostauth cap; unset is enforce.
func (h *Handler) objectGraphInternalAuth() hostauth.Mode {
	if h == nil || h.cfg == nil || h.cfg.ObjectGraphInternalAuth == "" {
		return hostauth.ModeEnforce
	}
	return h.cfg.ObjectGraphInternalAuth
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
		// Loopback is a location, not a credential: a host caller also
		// presents the host credential (SEC-11;
		// docs/problems/loopback-is-a-credential-2026-10-14.md). The route's
		// own mode lets it observe while the proxy enforces elsewhere.
		if r.Header.Get(hostauth.MarkerHeader) != "true" {
			return fmt.Errorf("internal caller required")
		}
		if !hostauth.AuthorizeAtMost(r, h.objectGraphInternalAuth()) {
			return fmt.Errorf("internal caller credential required")
		}
		return nil
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
		// Published Autopaper stories and opinion pieces are public: any
		// bound guest searches them (research's SearchWireCorpus). This one
		// path is the whole carve-out; corpusd serves only pieces a reader
		// can already open.
		if r.URL.Path == "/internal/platform/published/search" {
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
	if r.URL.Path == "/internal/platform/captures" || r.URL.Path == types.StoryVerdictsPath || r.URL.Path == types.StoryRelationsPath || r.URL.Path == types.StoryEntitiesPath {
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
		// A release is published by the owner's act, never filed by a guest
		// (Library S1 review 262, F20); so are a package release and its
		// listing (S2, L6).
		for _, kind := range []string{release.ObjectKind, pkgrelease.ObjectKind, pkgrelease.ListingKind} {
			if kinds.ObjectKind == kind || kinds.Kind == kind {
				return fmt.Errorf("releases and their listings are published through the owner's act, not written as objects")
			}
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
