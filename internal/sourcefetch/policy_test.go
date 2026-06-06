package sourcefetch

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
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
