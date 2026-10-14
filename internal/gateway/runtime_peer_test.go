package gateway

import (
	"net"
	"testing"
)

// The gateway's runtime-peer rule accepted the three RFC 5737
// documentation ranges in production (docs/learn/03-gateway-and-providers.md;
// the same pattern the isolation review flagged in vmctl). In a production
// binary they are refused; only loopback and Choir guest addresses pass.
func TestRuntimePeerRuleRefusesDocumentationRangesInProduction(t *testing.T) {
	for _, addr := range []string{"192.0.2.1", "198.51.100.2", "203.0.113.9"} {
		if runtimePeerAllowed(net.ParseIP(addr), false) {
			t.Fatalf("%s accepted as a runtime peer outside a test binary", addr)
		}
		if !runtimePeerAllowed(net.ParseIP(addr), true) {
			t.Fatalf("%s refused inside a test binary (handler tests rely on it)", addr)
		}
	}
	for _, addr := range []string{"127.0.0.1", "::1", "10.200.4.2", "172.16.0.2"} {
		if !runtimePeerAllowed(net.ParseIP(addr), false) {
			t.Fatalf("%s refused in production", addr)
		}
	}
	for _, addr := range []string{"8.8.8.8", "10.200.4.3", "10.199.0.2", "192.168.1.2"} {
		if runtimePeerAllowed(net.ParseIP(addr), false) {
			t.Fatalf("%s accepted in production", addr)
		}
	}
	if runtimePeerAllowed(nil, true) {
		t.Fatal("nil peer accepted")
	}
}
