package gateway

import (
	"strings"
	"testing"
)

func TestUpstreamErrorMessageIsBoundedAndCannotForgeALogLine(t *testing.T) {
	if got := upstreamErrorMessage([]byte(`{"error":{"message":"questions.q.criteria: required","code":400}}`)); got != ": questions.q.criteria: required" {
		t.Fatalf("got %q", got)
	}
	forged := `{"error":{"message":"bad\n2026/11/10 gateway: jev call ok\r\u0000\u2028x"}}`
	got := upstreamErrorMessage([]byte(forged))
	if strings.ContainsAny(got, "\n\r\x00\u2028") {
		t.Fatalf("control characters survived: %q", got)
	}
	long := `{"error":{"message":"` + strings.Repeat("é", 400) + `"}}`
	if got := upstreamErrorMessage([]byte(long)); len(got) > jevUpstreamMessageMax+2 {
		t.Fatalf("not bounded: %d bytes", len(got))
	}
	for _, body := range []string{``, `not json`, `{"error":"text"}`, `{"error":{}}`} {
		if got := upstreamErrorMessage([]byte(body)); got != "" {
			t.Fatalf("body %q gave %q", body, got)
		}
	}
}
