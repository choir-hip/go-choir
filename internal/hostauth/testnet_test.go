package hostauth

import (
	"net"
	"testing"
)

// Failure mode: the documentation ranges (RFC 5737: 192.0.2.0/24,
// 198.51.100.0/24, 203.0.113.0/24) that httptest.NewRequest uses as
// RemoteAddr were accepted as host or guest identities by production gates
// (vmctl, corpusd, maild, the gateway). They must count only in a test
// binary.
func TestDocumentationNetworkOnlyInTestBinaries(t *testing.T) {
	for _, addr := range []string{"192.0.2.1", "198.51.100.7", "203.0.113.200"} {
		ip := net.ParseIP(addr)
		if !IsDocumentationNetwork(ip) {
			t.Fatalf("%s not recognized as a documentation network", addr)
		}
		if documentationNetworkAllowed(ip, false) {
			t.Fatalf("%s accepted outside a test binary", addr)
		}
		if !documentationNetworkAllowed(ip, true) {
			t.Fatalf("%s refused in a test binary", addr)
		}
	}
	for _, addr := range []string{"127.0.0.1", "10.200.0.2", "192.0.3.1", "198.51.101.1", "::1", ""} {
		ip := net.ParseIP(addr)
		if IsDocumentationNetwork(ip) || documentationNetworkAllowed(ip, true) {
			t.Fatalf("%q wrongly treated as a documentation network", addr)
		}
	}
	if !TestNetworkAllowed("192.0.2.1") || !TestNetworkAllowed("192.0.2.1:1234") {
		t.Fatal("TestNetworkAllowed refused TEST-NET-1 inside a test binary")
	}
	if TestNetworkAllowed("203.0.113.9:4000") || TestNetworkAllowed("198.51.100.1") {
		t.Fatal("internal-route gates must treat TEST-NET-2/3 as outside callers, even in tests")
	}
	if testNet1Allowed(net.ParseIP("192.0.2.1"), false) {
		t.Fatal("TEST-NET-1 admitted outside a test binary")
	}
}
