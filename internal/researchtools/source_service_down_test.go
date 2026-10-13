package researchtools

import (
	"context"
	"net"
	"strings"
	"testing"
)

// docs/problems/desk-cell-ergonomics-from-demo-traces-2026-10-10.md (R1): the
// source service is off by default on Node B, and research got a raw
// "dial tcp …: connection refused", which it reported as an outage. Failure
// mode pinned: a refused connection does not say the service is not running
// or which verbs to use instead.
func TestSourceServiceRefusalNamesTheAlternatives(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()
	client := NewSourceClient(SourceClientConfig{BaseURL: "http://" + addr})
	_, err = client.SearchSources(context.Background(), "arXiv:2608.00009", 5)
	if err == nil {
		t.Fatal("search against a closed port succeeded")
	}
	for _, want := range []string{"not running", "choir.SearchStart", "choir.Read"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q lacks %q", err, want)
		}
	}
}
