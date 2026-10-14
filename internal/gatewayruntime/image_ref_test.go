package gatewayruntime

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

// FM11: a transcript carries an image ref; only the body to the gateway
// carries the bytes, and the caller's messages are not rewritten.
func TestCallWithToolsInlinesImageRefsForTheGatewayOnly(t *testing.T) {
	pngBytes := []byte("\x89PNG\r\n\x1a\nnot really decoded here")
	ref := "artifact:sha256:" + strings.Repeat("ab", 32)
	var got llmRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		writeJSON(t, w, llmResponse{ID: "r", Text: "a white square", StopReason: "end_turn", Model: got.Model})
	}))
	defer server.Close()

	p := New(server.URL, "token")
	p.SetImageResolver(func(r string) ([]byte, string, error) {
		if r != ref {
			return nil, "", fmt.Errorf("unknown ref %s", r)
		}
		return pngBytes, "image/png", nil
	})
	raw := json.RawMessage(`{"role":"user","content":[{"type":"text","text":"what is this?"},{"type":"image","source":{"kind":"artifact_ref","ref":"` + ref + `"}}]}`)
	before := string(raw)
	if _, err := p.CallWithTools(context.Background(), provideriface.ToolLoopRequest{
		Provider: "chatgpt", Model: "gpt-6-luna", ConversationID: "c",
		Messages: []json.RawMessage{raw},
	}); err != nil {
		t.Fatal(err)
	}
	if string(raw) != before {
		t.Fatal("caller's message was rewritten")
	}
	img := got.Messages[0].Content[1]
	if img.Type != "image" || img.Source == nil || img.Source.Kind != "base64" || img.Source.MIMEType != "image/png" {
		t.Fatalf("image block = %+v", img)
	}
	if img.Source.Data != base64.StdEncoding.EncodeToString(pngBytes) {
		t.Fatal("gateway did not receive the stored bytes")
	}
}

// FM12: an unknown ref fails the call before the gateway is reached.
func TestCallWithToolsFailsOnUnresolvableImageRef(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
	}))
	defer server.Close()
	p := New(server.URL, "token")
	p.SetImageResolver(func(string) ([]byte, string, error) { return nil, "", fmt.Errorf("not in store") })
	_, err := p.CallWithTools(context.Background(), provideriface.ToolLoopRequest{
		Model: "gpt-6-luna", ConversationID: "c",
		Messages: []json.RawMessage{json.RawMessage(`{"role":"user","content":[{"type":"image","source":{"kind":"artifact_ref","ref":"artifact:sha256:00"}}]}`)},
	})
	if err == nil || !strings.Contains(err.Error(), "resolve image") {
		t.Fatalf("err = %v", err)
	}
	if calls.Load() != 0 {
		t.Fatal("gateway was called with an unresolved image")
	}
}
