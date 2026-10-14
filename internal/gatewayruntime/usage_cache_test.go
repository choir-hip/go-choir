package gatewayruntime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

// The gateway relays provider.Usage as JSON; the runtime client must keep
// the prompt-cache count and the reported bit (failure modes: the field is
// dropped at the gateway->runtime boundary because the client's private
// usage struct lacks it; an older gateway without the field breaks decoding).
func TestCallWithToolsCarriesCachedInputTokens(t *testing.T) {
	cases := []struct {
		name         string
		usage        provider.Usage
		wantCached   int
		wantReported bool
	}{
		{"reported hit", provider.Usage{InputTokens: 1000, OutputTokens: 5, CachedInputTokens: 896, CacheReported: true}, 896, true},
		{"older gateway", provider.Usage{InputTokens: 1000, OutputTokens: 5}, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"id": "r1", "text": "ok", "model": "deepseek-v4.1-flash", "stop_reason": "end_turn",
					"usage": tc.usage, "provider_name": "opencode-go",
				})
			}))
			defer server.Close()
			p := New(server.URL, "tok")
			p.SetRuntimeLLMConfig("opencode-go", "deepseek-v4.1-flash", "none")
			resp, err := p.CallWithTools(context.Background(), provideriface.ToolLoopRequest{
				System:         "s",
				Messages:       []json.RawMessage{json.RawMessage(`{"role":"user","content":[{"type":"text","text":"hi"}]}`)},
				ConversationID: "run-1",
			})
			if err != nil {
				t.Fatal(err)
			}
			if resp.Usage.InputTokens != 1000 || resp.Usage.CachedInputTokens != tc.wantCached || resp.Usage.CacheReported != tc.wantReported {
				t.Fatalf("usage = %+v, want cached %d reported %v", resp.Usage, tc.wantCached, tc.wantReported)
			}
		})
	}
}
