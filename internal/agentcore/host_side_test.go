package agentcore

import "net/http"

// hostSide presents a loopback test connection as the host side of the guest
// boundary (the proxy's TEST-NET-1 stand-in, admitted only in test binaries).
// Inside a computer, loopback is an in-guest caller and never the owner
// (docs/problems/auth-boundary-gaps-from-surface-audit-2026-10-14.md F2).
func hostSide(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.RemoteAddr = "192.0.2.10:40000"
		next(w, r)
	})
}
