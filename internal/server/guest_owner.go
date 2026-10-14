package server

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// EnvGuestTrustLoopback opts a single-host development stack, where the proxy
// reaches the autoputer over 127.0.0.1, into treating loopback as the host
// side of the guest boundary. The guest image never sets it: inside a computer,
// loopback is every in-guest process (terminal, zot, capsule workloads), and
// none of them is the owner.
const EnvGuestTrustLoopback = "CHOIR_GUEST_TRUST_LOOPBACK"

// ErrNoAuthenticatedOwner is returned when a request carries no owner identity
// the guest may believe.
var ErrNoAuthenticatedOwner = errors.New("missing authenticated user identity")

// GuestHostCaller is the one rule for "this request came from the host side
// of the guest boundary" (docs/problems/auth-boundary-gaps-from-surface-audit-2026-10-14.md F2):
//
//   - in-process: an empty RemoteAddr, which net/http never produces for a
//     TCP connection (the management desk's choir.ProductAPI serves its calls
//     this way);
//   - the host peer: the guest's default gateway, which is where the proxy,
//     vmctl, maild and vmmanager connect from (they dial the guest's tap
//     address);
//   - TEST-NET-1 (httptest's synthetic address) inside a Go test binary only;
//   - loopback, only with EnvGuestTrustLoopback=1 (local development).
//
// Everything else, loopback included, is an in-guest caller or a stranger.
func GuestHostCaller(r *http.Request) bool {
	if r == nil {
		return false
	}
	addr := strings.TrimSpace(r.RemoteAddr)
	if addr == "" {
		return true
	}
	remote, ok := remoteAddrIP(addr)
	if !ok {
		return false
	}
	if remote.IsLoopback() {
		return guestTrustsLoopback()
	}
	if isTestNet1(remote) {
		return hostauth.TestNetworkAllowed(addr)
	}
	hostGatewayOnce.Do(cacheHostGateway)
	gateway := hostGateway.Load()
	if gateway == nil {
		parsed, err := defaultGateway()
		if err != nil {
			log.Printf("server: refusing guest host-side request: cannot determine default gateway: %v", err)
			return false
		}
		hostGateway.CompareAndSwap(nil, &parsed)
		gateway = hostGateway.Load()
	}
	return gateway != nil && remote == *gateway
}

func guestTrustsLoopback() bool {
	return strings.TrimSpace(os.Getenv(EnvGuestTrustLoopback)) == "1"
}

// AuthenticatedOwner returns the owner the proxy authenticated, read from
// X-Authenticated-User, only when the request came from the host side of the
// guest boundary (GuestHostCaller). Every guest route that acts as the owner
// reads identity through this function; the header alone is never identity.
func AuthenticatedOwner(r *http.Request) (string, error) {
	if !GuestHostCaller(r) {
		return "", ErrNoAuthenticatedOwner
	}
	user := strings.TrimSpace(r.Header.Get("X-Authenticated-User"))
	if user == "" {
		return "", ErrNoAuthenticatedOwner
	}
	return user, nil
}
