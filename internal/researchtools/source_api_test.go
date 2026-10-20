package researchtools

// Failure modes choir.SourceAPI must hold against (written before the
// code; docs/problems/keyed-source-apis-2026-10-20.md):
//
//  1. no gateway client on this computer: a plain error naming that, not a
//     nil dereference.
//  2. a gateway refusal (unknown endpoint, no key, budget spent) is hidden
//     or reworded: it reaches research as the gateway said it.
//  3. a large answer floods the desk's context: cut to the tool bound and
//     marked truncated, with a hint to narrow the request.
//  4. the provider, endpoint or params are changed on the way: passed
//     through as given.

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type fakeSourceAPI struct {
	provider, endpoint string
	params             map[string]any
	answer             json.RawMessage
	err                error
}

func (f *fakeSourceAPI) SourceAPI(_ context.Context, provider, endpoint string, params map[string]any) (json.RawMessage, error) {
	f.provider, f.endpoint, f.params = provider, endpoint, params
	return f.answer, f.err
}

func runSourceAPITool(t *testing.T, deps Dependencies, args string) (string, error) {
	t.Helper()
	tool := newSourceAPITool(deps)
	return tool.Func(context.Background(), json.RawMessage(args))
}

func TestSourceAPIToolWithoutClient(t *testing.T) {
	_, err := runSourceAPITool(t, Dependencies{}, `{"provider":"fred","endpoint":"series","params":{"series_id":"UNRATE"}}`)
	if err == nil || !strings.Contains(err.Error(), "not available on this computer") {
		t.Fatalf("err = %v", err)
	}
}

func TestSourceAPIToolPassesThroughAndSurfacesRefusals(t *testing.T) {
	fake := &fakeSourceAPI{err: errors.New("source api: daily budget of newsapi_ai spent; resets in 3h0m0s")}
	_, err := runSourceAPITool(t, Dependencies{SourceAPI: fake}, `{"provider":"newsapi_ai","endpoint":"articles","params":{"keyword":"台灣","count":5}}`)
	if err == nil || !strings.Contains(err.Error(), "daily budget of newsapi_ai spent") {
		t.Fatalf("err = %v", err)
	}
	if fake.provider != "newsapi_ai" || fake.endpoint != "articles" || fake.params["keyword"] != "台灣" || fake.params["count"] != float64(5) {
		t.Fatalf("passed %q %q %v", fake.provider, fake.endpoint, fake.params)
	}
}

func TestSourceAPIToolReturnsAnswer(t *testing.T) {
	fake := &fakeSourceAPI{answer: json.RawMessage(`{"provider":"fred","endpoint":"series","upstream_status":200,"body":{"seriess":[]}}`)}
	out, err := runSourceAPITool(t, Dependencies{SourceAPI: fake}, `{"provider":"fred","endpoint":"series","params":{"series_id":"UNRATE"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"seriess"`) || strings.Contains(out, "truncated") {
		t.Fatalf("out = %s", out)
	}
}

func TestSourceAPIToolCutsLargeAnswer(t *testing.T) {
	big := `{"provider":"fred","endpoint":"series_observations","upstream_status":200,"body":{"x":"` + strings.Repeat("9", sourceAPIToolMaxChars*2) + `"}}`
	fake := &fakeSourceAPI{answer: json.RawMessage(big)}
	out, err := runSourceAPITool(t, Dependencies{SourceAPI: fake}, `{"provider":"fred","endpoint":"series_observations","params":{"series_id":"UNRATE"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) > sourceAPIToolMaxChars+2048 {
		t.Fatalf("out is %d bytes, bound %d", len(out), sourceAPIToolMaxChars)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("cut answer is not JSON: %v", err)
	}
	if got["truncated"] != true || !strings.Contains(got["hint"].(string), "narrow") || got["upstream_status"] != float64(200) {
		t.Fatalf("cut answer fields: %v", got["truncated"])
	}
}
