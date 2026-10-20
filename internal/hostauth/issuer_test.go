package hostauth

// Failure modes the issuer credential must hold against (written before the
// code; blast-radius design finding F1):
//
//  1. the shared internal credential alone passes the issuer gate in
//     enforce: refused.
//  2. a missing, wrong, or doubled issuer token passes in enforce: refused.
//  3. observe refuses anything (a computer that cannot boot): never; it
//     logs and allows.
//  4. the issuer token reaches the handler or a guest: stripped at ingress;
//     the client attaches it only to corpusd on loopback.
//  5. the issuer token leaks into a log line: never.
//  6. enforce with no issuer credential loaded: every issuance refused
//     (fail closed).

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const issuerToken = "fedcba9876543210fedcba9876543210fedcba9876543210"

func serveIssuer(t *testing.T, internal []string, issuer []string) (internalOK, issuerOK, issuerHeaderSeen bool) {
	t.Helper()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		internalOK = Authorize(r)
		issuerOK = AuthorizeIssuer(r)
		issuerHeaderSeen = r.Header.Get(IssuerHeader) != ""
	})
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8086/internal/computers/credentials/issue", nil)
	req.RemoteAddr = "127.0.0.1:40000"
	for _, v := range internal {
		req.Header.Add(Header, v)
	}
	for _, v := range issuer {
		req.Header.Add(IssuerHeader, v)
	}
	Middleware(inner).ServeHTTP(httptest.NewRecorder(), req)
	return internalOK, issuerOK, issuerHeaderSeen
}

func TestIssuerGateMatrix(t *testing.T) {
	cases := []struct {
		name     string
		mode     Mode
		loaded   string
		internal []string
		issuer   []string
		want     bool
	}{
		{"enforce internal credential alone refused", ModeEnforce, issuerToken, []string{testToken}, nil, false},
		{"enforce internal token sent as issuer refused", ModeEnforce, issuerToken, []string{testToken}, []string{testToken}, false},
		{"enforce right issuer token", ModeEnforce, issuerToken, []string{testToken}, []string{issuerToken}, true},
		{"enforce wrong issuer token", ModeEnforce, issuerToken, []string{testToken}, []string{strings.Repeat("y", len(issuerToken))}, false},
		{"enforce two issuer headers", ModeEnforce, issuerToken, []string{testToken}, []string{issuerToken, issuerToken}, false},
		{"enforce none loaded refuses", ModeEnforce, "", []string{testToken}, []string{issuerToken}, false},
		{"observe missing allowed", ModeObserve, issuerToken, []string{testToken}, nil, true},
		{"observe wrong allowed", ModeObserve, issuerToken, []string{testToken}, []string{"nope"}, true},
		{"off missing allowed", ModeOff, "", []string{testToken}, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureLog(t)
			defer SetForTesting(ModeEnforce, testToken)()
			defer SetIssuerForTesting(tc.mode, tc.loaded)()
			internalOK, got, seen := serveIssuer(t, tc.internal, tc.issuer)
			if !internalOK {
				t.Fatalf("internal gate refused a valid internal credential")
			}
			if got != tc.want {
				t.Fatalf("AuthorizeIssuer = %v, want %v", got, tc.want)
			}
			if seen {
				t.Fatalf("issuer header reached the handler; it must be stripped at ingress")
			}
			if strings.Contains(logs.String(), issuerToken) || strings.Contains(logs.String(), testToken) {
				t.Fatalf("a token reached the log: %q", logs.String())
			}
			if !tc.want && !strings.Contains(logs.String(), IssuerHeader) {
				t.Fatalf("refusal log does not name the issuer header: %q", logs.String())
			}
		})
	}
}

func TestMarkIssuerOnlyToCorpusdOnLoopback(t *testing.T) {
	defer SetIssuerForTesting(ModeEnforce, issuerToken)()
	cases := []struct {
		url  string
		want bool
	}{
		{"http://127.0.0.1:8086/internal/computers/credentials/issue", true},
		{"http://localhost:8086/internal/computers/credentials/issue", true},
		{"http://127.0.0.1:8083/internal/vmctl/list", false},     // vmctl, not the issuer
		{"http://127.0.0.1:9001/internal/computers/x", false},    // a guest DNAT port
		{"http://10.0.0.5:8086/internal/computers/x", false},     // not loopback
		{"https://example.com:8086/internal/computers/x", false}, // the internet
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, tc.url, nil)
		req.Header.Set(IssuerHeader, "stale-value-from-a-caller")
		MarkIssuer(req)
		got := req.Header.Get(IssuerHeader)
		if tc.want && got != issuerToken {
			t.Fatalf("%s: issuer token not attached", tc.url)
		}
		if !tc.want && got != "" {
			t.Fatalf("%s: issuer header sent (%q); it must go only to corpusd on loopback", tc.url, got)
		}
	}
}

func TestMarkIssuerWithoutCredentialSendsNothing(t *testing.T) {
	defer SetIssuerForTesting(ModeEnforce, "")()
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8086/internal/computers/credentials/issue", nil)
	MarkIssuer(req)
	if req.Header.Get(IssuerHeader) != "" {
		t.Fatalf("issuer header sent with no credential loaded")
	}
}

func TestIssuerStatusNeverShowsToken(t *testing.T) {
	defer SetIssuerForTesting(ModeEnforce, issuerToken)()
	if s := IssuerStatus(); strings.Contains(s, issuerToken) || !strings.Contains(s, "mode=enforce") {
		t.Fatalf("IssuerStatus = %q", s)
	}
}
