package sourcefetch

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestValidateURLRejectsForbiddenTargets(t *testing.T) {
	tests := []string{
		"http://localhost/internal",
		"http://127.0.0.1:8080/internal",
		"http://[::1]/internal",
		"http://10.0.0.5/internal",
		"http://172.16.0.5/internal",
		"http://192.168.1.5/internal",
		"http://169.254.169.254/latest/meta-data/",
		"http://100.64.0.1/internal",
		"http://example.com@127.0.0.1/internal",
		"file:///etc/passwd",
	}
	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			if err := ValidateURL(raw); err == nil {
				t.Fatalf("ValidateURL(%q) succeeded, want error", raw)
			}
		})
	}
}

func TestValidateURLAllowsOrdinaryPublicHTTPS(t *testing.T) {
	if err := ValidateURL("https://example.com/source?x=1#fragment"); err != nil {
		t.Fatalf("ValidateURL public https: %v", err)
	}
}

func TestValidateHostRejectsForbiddenAddresses(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "::1", "10.1.2.3", "169.254.169.254", "100.64.10.20"} {
		t.Run(host, func(t *testing.T) {
			err := ValidateHost(context.Background(), net.DefaultResolver, host)
			if err == nil || !strings.Contains(err.Error(), "forbidden address") {
				t.Fatalf("ValidateHost(%q) = %v, want forbidden address", host, err)
			}
		})
	}
}

func TestClientRedirectPolicyRejectsForbiddenTargets(t *testing.T) {
	client := Client(0)
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1/internal", nil)
	if err != nil {
		t.Fatalf("redirect request: %v", err)
	}
	if err := client.CheckRedirect(req, nil); err == nil {
		t.Fatal("CheckRedirect allowed redirect to loopback")
	}
}

// docs/problems/research-fetch-url-has-no-address-guard-2026-10-10.md (D2):
// the client checked the resolved address and then dialed the hostname, so a
// second resolution (DNS rebinding) could land on a private address. Failure
// modes pinned: the dial goes to the hostname instead of a checked address;
// a mixed public/private answer is dialed; a private answer is dialed.
func TestClientDialsTheAddressItChecked(t *testing.T) {
	cases := []struct {
		name    string
		answer  []net.IPAddr
		wantErr bool
	}{
		{"public", []net.IPAddr{{IP: net.ParseIP("151.101.0.1")}}, false},
		{"private", []net.IPAddr{{IP: net.ParseIP("10.200.0.1")}}, true},
		{"mixed", []net.IPAddr{{IP: net.ParseIP("151.101.0.1")}, {IP: net.ParseIP("127.0.0.1")}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var dialed []string
			client := clientWith(5*time.Second,
				func(context.Context, string) ([]net.IPAddr, error) { return tc.answer, nil },
				func(_ context.Context, _, address string) (net.Conn, error) {
					dialed = append(dialed, address)
					return nil, errors.New("test dial stops here")
				})
			_, err := client.Get("https://rebind.example/")
			if err == nil {
				t.Fatal("request unexpectedly succeeded")
			}
			if tc.wantErr {
				if len(dialed) != 0 {
					t.Fatalf("dialed %v for a forbidden answer", dialed)
				}
				return
			}
			if len(dialed) != 1 || dialed[0] != "151.101.0.1:443" {
				t.Fatalf("dialed %v, want the checked address 151.101.0.1:443", dialed)
			}
		})
	}
}
