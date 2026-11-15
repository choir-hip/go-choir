package gateway

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/gatewayruntime"
)

// The guest client and the gateway name the same header.
func TestJevSiteHeaderIsTheGuestClientsHeader(t *testing.T) {
	if JevSiteHeader != gatewayruntime.JevSiteHeader {
		t.Fatalf("gateway reads %q, the guest client sends %q", JevSiteHeader, gatewayruntime.JevSiteHeader)
	}
}
