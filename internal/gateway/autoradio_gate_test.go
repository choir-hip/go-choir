package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provider"
)

// The gateway honours reasoning_off only for a no-tools call to a member of the
// compiled autoradio pool (review round 2, 4) and says what it applied
// (review round 2, 5). Failure list, written before the code:
//
//  1. a guest-claimed reasoning_off turns reasoning off for a call with tools,
//     or for a model outside the autoradio pool;
//  2. an ignored flag is not counted or logged;
//  3. the gateway does not say what it applied, so an older gateway (which
//     drops the flag silently) cannot be told from one that honoured it.

func gatewayRig(t *testing.T, river, bedrock http.HandlerFunc) (url, token string) {
	t.Helper()
	mp := provider.NewMultiProvider()
	if river != nil {
		up := httptest.NewServer(river)
		t.Cleanup(up.Close)
		rp, err := provider.NewRiverProvider(provider.RiverConfig{APIKey: "k", ModelID: riverStreamModel, BaseURL: up.URL + "/v1"})
		if err != nil {
			t.Fatal(err)
		}
		mp.Register("river", rp)
	}
	if bedrock != nil {
		up := httptest.NewServer(bedrock)
		t.Cleanup(up.Close)
		bp, err := provider.NewBedrockProvider(provider.BedrockConfig{Region: "us-east-1", ModelID: bedrockStreamModel, AuthToken: "k", BaseURL: up.URL})
		if err != nil {
			t.Fatal(err)
		}
		mp.Register("bedrock", bp)
	}
	reg := NewIdentityRegistry(time.Hour)
	h := NewMultiHandlerWithRateLimit(reg, mp, nil)
	cred, _ := reg.IssueCredential("autoputer-gate")
	mux := http.NewServeMux()
	mux.HandleFunc("/provider/v1/inference", h.HandleInference)
	gw := httptest.NewServer(mux)
	t.Cleanup(gw.Close)
	return gw.URL, cred.RawToken
}

func postInference(t *testing.T, url, token, body string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url+"/provider/v1/inference", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp, string(raw)
}

func TestGatewayHonoursReasoningOffOnlyForANoToolsCallToAnAutoradioPoolMember(t *testing.T) {
	var efforts []any
	river := func(w http.ResponseWriter, r *http.Request) {
		var b map[string]any
		_ = json.NewDecoder(r.Body).Decode(&b)
		efforts = append(efforts, b["reasoning_effort"])
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"id\":\"r\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Ok.\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}
	url, token := gatewayRig(t, river, nil)
	msg := `"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]`
	// 1. A no-tools call to the pool member: honoured and said so.
	resp, _ := postInference(t, url, token, `{"provider":"river","model":"`+riverStreamModel+`","stream":true,"reasoning_off":true,`+msg+`}`)
	if got := resp.Header.Get(AppliedHeader); got != "reasoning=off;cache_break=none" {
		t.Fatalf("(3) applied header %q", got)
	}
	// 2. The same with a tool: ignored, counted, and said so.
	before := reasoningOffIgnored.Load()
	resp, _ = postInference(t, url, token, `{"provider":"river","model":"`+riverStreamModel+`","stream":true,"reasoning_effort":"low","reasoning_off":true,"tools":[{"name":"t","description":"d","input_schema":{"type":"object"}}],`+msg+`}`)
	if got := resp.Header.Get(AppliedHeader); got != "reasoning=on;cache_break=none" {
		t.Fatalf("(1)(3) applied header %q", got)
	}
	if reasoningOffIgnored.Load() != before+1 {
		t.Fatal("(2) the ignored flag was not counted")
	}
	// 3. No flag at all keeps the floor.
	postInference(t, url, token, `{"provider":"river","model":"`+riverStreamModel+`","stream":true,"reasoning_effort":"low",`+msg+`}`)
	if len(efforts) != 3 || efforts[0] != "none" || efforts[1] != "low" || efforts[2] != "low" {
		t.Fatalf("(1) River's reasoning_effort per call: %v", efforts)
	}
}

func TestAutoradioPoolMembershipIsExactlyTheCompiledPool(t *testing.T) {
	for _, c := range []struct {
		provider, model string
		want            bool
	}{
		{"river", riverStreamModel, true},
		{"bedrock", bedrockStreamModel, true},
		{"bedrock", "us.anthropic.claude-sonnet-4-6", false},
		{"river", "another-model", false},
		{"synthetic", "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017", false},
		{"devin", "swe-2-flash", false},
		{"river", "", false},
	} {
		if got := autoradioPoolMember(c.provider, c.model); got != c.want {
			t.Errorf("(1) %s/%s = %v, want %v", c.provider, c.model, got, c.want)
		}
	}
}

func TestGatewaySaysWhetherTheCacheBreakWasAccepted(t *testing.T) {
	bedrock := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"m","model":"`+bedrockStreamModel+`","stop_reason":"end_turn","content":[{"type":"text","text":"Ok."}],"usage":{"input_tokens":1,"output_tokens":1}}`)
	}
	river := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"id\":\"r\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Ok.\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}
	url, token := gatewayRig(t, river, bedrock)
	msgs := `"messages":[{"role":"user","content":[{"type":"text","text":"page","cache_break":true},{"type":"text","text":"words"}]}]`
	resp, _ := postInference(t, url, token, `{"provider":"bedrock","model":"`+bedrockStreamModel+`","stream":true,"reasoning_off":true,`+msgs+`}`)
	if got := resp.Header.Get(AppliedHeader); got != "reasoning=off;cache_break=accepted" {
		t.Fatalf("(3) bedrock applied %q", got)
	}
	resp, _ = postInference(t, url, token, `{"provider":"river","model":"`+riverStreamModel+`","stream":true,"reasoning_off":true,`+msgs+`}`)
	if got := resp.Header.Get(AppliedHeader); got != "reasoning=off;cache_break=ignored" {
		t.Fatalf("(3) river applied %q", got)
	}
}
