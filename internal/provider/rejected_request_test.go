package provider

import (
	"io"
	"log"
	"net/http"
	"os"
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
	if got := rejectedReason([]byte(long)); len(got) > 305 {
		t.Fatalf("reason not truncated: %d bytes", len(got))
	}
}

func TestBedrockRejectionLogsTypeAndMessage(t *testing.T) {
	var buf strings.Builder
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	resp := &http.Response{StatusCode: 400, Status: "400 Bad Request", Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"message":"messages.3.content.0: Expected thinking"}`))}
	resp.Header.Set("X-Amzn-Errortype", "ValidationException:http://internal.amazon.com/coral/")
	_, err := parseBedrockResponse(resp, "m")
	if err == nil || strings.Contains(err.Error(), "Expected thinking") {
		t.Fatalf("caller error must stay sanitized: %v", err)
	}
	if out := buf.String(); !strings.Contains(out, "ValidationException") || !strings.Contains(out, "Expected thinking") {
		t.Fatalf("journal lacks type/message: %s", out)
	}
}
