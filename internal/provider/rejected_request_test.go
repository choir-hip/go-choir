package provider

import (
	"strings"
	"testing"
)

func TestRejectedReasonRedactsAndTruncates(t *testing.T) {
	cases := map[string]string{
		`{"error":{"type":"invalid_request_error","message":"context length 180000 exceeds 131072"}}`: "invalid_request_error context length 180000 exceeds 131072",
		`{"error":"bad api_key=sk-abcdefghijklmnop"}`:                                                 "bad [redacted]",
		`{"message":"see https://x.test/v1?key=abc123 Bearer abc.def"}`:                               "see https://x.test/v1[redacted] [redacted]",
		`not json`: "unparsed body",
	}
	for body, want := range cases {
		if got := rejectedReason([]byte(body)); got != want {
			t.Fatalf("rejectedReason(%s) = %q, want %q", body, got, want)
		}
	}
	long := `{"message":"` + strings.Repeat("x", 500) + `"}`
	if got := rejectedReason([]byte(long)); len(got) > 245 {
		t.Fatalf("reason not truncated: %d bytes", len(got))
	}
}
