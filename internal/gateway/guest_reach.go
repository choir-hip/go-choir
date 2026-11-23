// Guest-reach decisions of this package, moved here whole from handlers.go so the
// security-epoch gate (nix/guest-security-surfaces.txt) sees a change to them. A
// change to this file changes what a guest computer may reach on the host.
// Keep it to those decisions; put anything else in another file.

package gateway

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/hostfetch"
	"github.com/yusefmosiah/go-choir/internal/server"
)

// authenticateAutoputer validates the Bearer token from the Authorization
// header against the identity registry. Returns the autoputer ID if valid
// (VAL-GATEWAY-003).
func (h *Handler) authenticateAutoputer(r *http.Request) (string, error) {
	computerID, err := h.authenticateBearer(r)
	if err != nil {
		return "", err
	}
	// SEC-05: the bearer works only from its computer's own address.
	if err := h.checkBearerBinding(computerID, r); err != nil {
		return "", err
	}
	return computerID, nil
}

// authenticateBearer is authenticateAutoputer without the SEC-05 binding: the
// bearer and the runtime-peer rule only, writing nothing. The Jev route uses
// it with its own pin check and 403 (jevPeerAllowed); the Anthropic relay
// uses it with a pin check of its own that ignores the rollout mode.
func (h *Handler) authenticateBearer(r *http.Request) (string, error) {
	computerID, _, err := h.authenticateBearerIdentity(r)
	return computerID, err
}

// authenticateBearerIdentity is authenticateBearer that also returns the
// product computer id the bearer's identity was issued for ("" for one
// issued before it was recorded). The first value stays the VM id every
// route and the pin key by; only computer-keyed authorities read the second.
func (h *Handler) authenticateBearerIdentity(r *http.Request) (string, string, error) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", "", fmt.Errorf("missing authorization header")
	}

	if !strings.HasPrefix(auth, "Bearer ") {
		return "", "", fmt.Errorf("invalid authorization scheme")
	}

	token := strings.TrimPrefix(auth, "Bearer ")
	if token == "" {
		return "", "", fmt.Errorf("empty bearer token")
	}

	computerID, productComputerID, err := h.registry.ValidateCredentialIdentity(token)
	if err != nil {
		return "", "", err
	}
	if !isAuthorizedRuntimePeer(r) {
		return "", "", fmt.Errorf("gateway access is limited to choir runtime peers")
	}
	return computerID, productComputerID, nil
}

func isAuthorizedRuntimePeer(r *http.Request) bool {
	ip := remoteIP(r)
	if ip == nil {
		return false
	}
	return runtimePeerAllowed(ip, testing.Testing())
}

// runtimePeerAllowed is the runtime-peer rule with the test-binary switch
// explicit, so a test can assert the production rule: loopback or a Choir
// guest address, and the RFC 5737 documentation ranges (httptest's
// RemoteAddr) only inside a test binary, never in production.
func runtimePeerAllowed(ip net.IP, inTestBinary bool) bool {
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || isChoirGuestPeer(ip) || (inTestBinary && hostauth.IsDocumentationNetwork(ip))
}

func remoteIP(r *http.Request) net.IP {
	if r == nil {
		return nil
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return net.ParseIP(host)
}

// Choir guests boot on host tap networks with the host side at *.1 and the
// guest side at *.2. Keep this aligned with vmmanager's bounded private
// allocation pool so a leaked bearer token is not enough to call the gateway
// from arbitrary hosts.
func isChoirGuestPeer(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	return (ip4[0] == 172 && ip4[2] == 0 && ip4[3] == 2) ||
		(ip4[0] == 10 && ip4[1] >= 200 && ip4[1] <= 215 && ip4[3] == 2)
}

// RegisterRoutes registers all gateway routes on the given server.
func RegisterRoutes(s *server.Server, h *Handler) {
	s.SetHealthHandler(h.HandleHealth)
	// M22b / C20: per-service health endpoint. Public (no auth), exposes
	// only coarse status — never credentials or upstream bodies.
	s.HandleFunc("/health/{service}", h.HandleServiceHealth)
	s.HandleFunc("/provider/v1/inference", h.HandleInference)
	s.HandleFunc("/provider/v1/judgments", h.HandleJudgment)
	// SEC-16: host jobs (front-importance) reach Jev through the gateway
	// with the host-internal credential instead of holding the key.
	s.HandleFunc("/provider/v1/host/judgments", h.HandleHostJudgment)
	s.HandleFunc("/provider/openai/v1/chat/completions", h.HandleOpenAIChatCompletions)
	s.HandleFunc("/provider/openai/v1/models", h.HandleOpenAIModels)
	s.HandleFunc("/provider/v1/models", h.HandleModels)
	// SEC-08 Option K: the Claude delegate's Messages API calls, relayed with
	// the gateway-held API key. Dark unless a key, identities and a window
	// are configured (anthropic_relay.go).
	s.HandleFunc("/provider/anthropic/", h.HandleAnthropicRelay)
	s.HandleFunc("/provider/v1/search", h.HandleSearch)
	s.HandleFunc("/provider/v1/search/health", h.HandleSearchHealth)
	s.HandleFunc("/provider/v1/search/health/reset", h.HandleSearchHealthReset)
	s.HandleFunc("/provider/v1/source-api", h.HandleSourceAPI)
	s.HandleFunc(hostfetch.Path, h.HandleHostFetch)
	s.HandleFunc("/provider/v1/credentials/issue", h.HandleIssueCredential)
	s.HandleFunc("/provider/v1/credentials/ensure", h.HandleEnsureCredential)
	s.HandleFunc("/provider/v1/spend", h.HandlePaidSpend)
	s.HandleFunc("/provider/v1/breakers", h.HandleProviderBreakers)
	s.HandleFunc("/provider/v1/breakers/reset", h.HandleProviderBreakerReset)
}
