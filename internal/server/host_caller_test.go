package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHostSourcedCallerAllowsLocalAndSyntheticCallers(t *testing.T) {
	for _, remoteAddr := range []string{"", "@", "@autoputer", "/run/autoputer.sock", "127.0.0.1:8080", "[::1]:8080", "192.0.2.17:8080"} {
		t.Run(remoteAddr, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = remoteAddr
			if !HostSourcedCaller(req) {
				t.Fatalf("HostSourcedCaller(%q) = false, want true", remoteAddr)
			}
		})
	}
}

func TestHostSourcedCallerRejectsNilAndMalformedRemoteAddress(t *testing.T) {
	if HostSourcedCaller(nil) {
		t.Fatal("HostSourcedCaller(nil) = true, want false")
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "not-an-address"
	if HostSourcedCaller(req) {
		t.Fatal("HostSourcedCaller accepted malformed remote address")
	}
}

func TestParseDefaultGatewayDecodesLittleEndianRouteGateway(t *testing.T) {
	routeTable := strings.NewReader("Iface\tDestination\tGateway \tFlags\neth0\t00000000\t0102C80A\t0003\n")
	gateway, err := parseDefaultGateway(routeTable)
	if err != nil {
		t.Fatalf("parseDefaultGateway() error = %v", err)
	}
	if got, want := gateway.String(), "10.200.2.1"; got != want {
		t.Fatalf("gateway = %s, want %s", got, want)
	}
}

func TestParseDefaultGatewayRequiresGatewayRoute(t *testing.T) {
	routeTable := strings.NewReader("Iface\tDestination\tGateway \tFlags\neth0\t00000000\t00000000\t0001\n")
	if _, err := parseDefaultGateway(routeTable); err == nil {
		t.Fatal("parseDefaultGateway() succeeded without a gateway route")
	}
}
