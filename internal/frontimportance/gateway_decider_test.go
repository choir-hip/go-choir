package frontimportance

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/gateway"
	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// SEC-16: the scorer reaches Jev through the gateway and holds no provider
// key. Ways it can fail, written before the code:
//  1. it still builds its own OpenRouter transport (the key stays in the
//     unit): the default decider must be the gateway client, and direct mode
//     must be an explicit opt-in that needs a key
//     (TestDeciderIsTheGatewayByDefault);
//  2. the request leaves without the host-internal marker and credential, so
//     the gateway refuses it (TestGatewayDeciderSendsAnInternalCallerRequest);
//  3. an error from the gateway is swallowed, or its body is echoed without
//     a bound (TestGatewayDeciderReportsRefusals);
//  4. the answer is altered: the scorer needs the exact upstream body
//     (TestGatewayDeciderSendsAnInternalCallerRequest).

func jevRequest() gateway.JudgmentRequest {
	return gateway.JudgmentRequest{
		State:     json.RawMessage(`{"story":{"title":"t"}}`),
		Questions: map[string]json.RawMessage{"q": json.RawMessage(`{"type":"choice","choices":["a","b"]}`)},
	}
}

func TestGatewayDeciderSendsAnInternalCallerRequest(t *testing.T) {
	const token = "scorer-test-token-0123456789abcdef0123456789ab"
	var gotHeader http.Header
	var gotBody []byte
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader, gotPath, gotMethod = r.Header.Clone(), r.URL.Path, r.Method
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = io.WriteString(w, `{"model":"m","answers":{"q":{"choice":"a"}}}`)
	}))
	defer srv.Close()
	// The test server is on 127.0.0.1 with an ephemeral port: the credential
	// goes only to loopback ports hostauth lists.
	u, _ := url.Parse(srv.URL)
	t.Setenv(hostauth.EnvPorts, u.Port())
	restore := hostauth.SetForTesting(hostauth.ModeEnforce, token)
	defer restore()

	d := &gatewayDecider{url: srv.URL + "/provider/v1/host/judgments", client: srv.Client()}
	raw, err := d.Decide(context.Background(), scorerComputerID, jevRequest())
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"model":"m","answers":{"q":{"choice":"a"}}}` {
		t.Fatalf("the answer was altered: %s", raw)
	}
	if gotMethod != http.MethodPost || gotPath != "/provider/v1/host/judgments" {
		t.Fatalf("%s %s", gotMethod, gotPath)
	}
	if gotHeader.Get(hostauth.MarkerHeader) != "true" {
		t.Fatalf("no internal-caller marker: %v", gotHeader)
	}
	if gotHeader.Get(hostauth.Header) != token {
		t.Fatalf("no host-internal credential: %v", gotHeader)
	}
	if gotHeader.Get("X-Choir-Host-Caller") != "front-importance" {
		t.Fatalf("no caller label: %v", gotHeader)
	}
	if !strings.Contains(string(gotBody), `"questions"`) || strings.Contains(string(gotBody), "front-importance") {
		t.Fatalf("body %s", gotBody)
	}
}

func TestGatewayDeciderReportsRefusals(t *testing.T) {
	big := strings.Repeat("x", 5000)
	for name, handler := range map[string]http.HandlerFunc{
		"forbidden":   func(w http.ResponseWriter, r *http.Request) { http.Error(w, big, http.StatusForbidden) },
		"disabled":    func(w http.ResponseWriter, r *http.Request) { http.Error(w, "Jev judgments are disabled", 503) },
		"bad gateway": func(w http.ResponseWriter, r *http.Request) { http.Error(w, "x", 502) },
	} {
		srv := httptest.NewServer(handler)
		d := &gatewayDecider{url: srv.URL, client: srv.Client()}
		_, err := d.Decide(context.Background(), scorerComputerID, jevRequest())
		srv.Close()
		if err == nil {
			t.Fatalf("%s: no error", name)
		}
		if len(err.Error()) > 400 {
			t.Fatalf("%s: the error echoes an unbounded body (%d bytes)", name, len(err.Error()))
		}
	}
	d := &gatewayDecider{url: "http://127.0.0.1:1/never", client: http.DefaultClient}
	if _, err := d.Decide(context.Background(), scorerComputerID, jevRequest()); err == nil {
		t.Fatal("an unreachable gateway returned no error")
	}
}

func TestDeciderIsTheGatewayByDefault(t *testing.T) {
	get := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	// The key is present in the environment (the unit still loads it until the
	// nix change): the default must still be the gateway.
	d, mode, err := newDecider(get(map[string]string{"OPENROUTER_API_KEY": "fake"}), http.DefaultClient)
	if err != nil || mode != "gateway" {
		t.Fatalf("default = %q, %v; want gateway", mode, err)
	}
	g, ok := d.(*gatewayDecider)
	if !ok || g.url != defaultGatewayJudgmentsURL {
		t.Fatalf("default decider %#v", d)
	}
	d, _, _ = newDecider(get(map[string]string{"FRONT_IMPORTANCE_GATEWAY_URL": "http://127.0.0.1:9/x"}), http.DefaultClient)
	if d.(*gatewayDecider).url != "http://127.0.0.1:9/x" {
		t.Fatalf("url override ignored")
	}
	// The removed direct switch no longer changes the mode.
	if _, mode, err := newDecider(get(map[string]string{"FRONT_IMPORTANCE_DIRECT": "1", "OPENROUTER_API_KEY": "fake"}), http.DefaultClient); err != nil || mode != "gateway" {
		t.Fatalf("FRONT_IMPORTANCE_DIRECT=1 = %q, %v; want gateway", mode, err)
	}
	// A gateway URL that is not loopback is refused: the internal credential
	// must never be sent off the host.
	if _, _, err := newDecider(get(map[string]string{"FRONT_IMPORTANCE_GATEWAY_URL": "https://example.com/x"}), http.DefaultClient); err == nil {
		t.Fatal("a non-loopback gateway URL was accepted")
	}
}
