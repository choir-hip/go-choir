package gateway

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/health"
	"github.com/yusefmosiah/go-choir/internal/provider"
)

// Failure modes: docs/archive/problems/models-configured-text-only-2026-10-15.md.

func visionTestPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	img.Set(0, 0, color.RGBA{R: 7, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type fakeOpenCodeUpstream struct {
	calls atomic.Int32
	body  atomic.Value
}

func (f *fakeOpenCodeUpstream) server(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		f.body.Store(string(body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"c1","model":"deepseek-v4.1-flash","choices":[{"message":{"role":"assistant","content":"white"},"finish_reason":"stop"}],"usage":{"prompt_tokens":90,"completion_tokens":1}}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func postVisionInference(t *testing.T, h *Handler, token, model string, raw []byte) *httptest.ResponseRecorder {
	t.Helper()
	payload := ProviderRequest{
		Model:          model,
		ConversationID: "run-vision",
		Messages: []provider.Message{{Role: "user", Content: []provider.Block{
			{Type: "text", Text: "what colour?"},
			{Type: "image", Source: &provider.MediaSource{Kind: "base64", MIMEType: "image/png", Data: base64.StdEncoding.EncodeToString(raw)}},
		}}},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/inference", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.RemoteAddr = "10.200.209.2:45678"
	w := httptest.NewRecorder()
	h.HandleInference(w, req)
	return w
}

func captureVisionLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return &buf
}

// The gateway carries an image to a vision model, and its journal names
// the image by hash and size, never by its bytes (FM11).
func TestGatewayForwardsImageToVisionModelWithoutLoggingBytes(t *testing.T) {
	upstream := &fakeOpenCodeUpstream{}
	srv := upstream.server(t)
	p, err := provider.NewOpenCodeProvider(provider.OpenCodeConfig{APIKey: "k", BaseURL: srv.URL, ModelID: "deepseek-v4.1-flash", ProviderName: "opencode-go"})
	if err != nil {
		t.Fatal(err)
	}
	reg := NewIdentityRegistry(time.Hour)
	cred, _ := reg.IssueCredential("vm-vision")
	h := NewHandlerWithRateLimit(reg, p, nil)
	logs := captureVisionLog(t)

	raw := visionTestPNG(t)
	w := postVisionInference(t, h, cred.RawToken, "deepseek-v4.1-flash", raw)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	encoded := base64.StdEncoding.EncodeToString(raw)
	if sent, _ := upstream.body.Load().(string); !strings.Contains(sent, "data:image/png;base64,"+encoded) {
		t.Fatalf("upstream did not receive the image: %.300s", sent)
	}
	journal := logs.String()
	if !strings.Contains(journal, "images=1") {
		t.Fatalf("journal lacks the image count: %s", journal)
	}
	if strings.Contains(journal, encoded[:32]) {
		t.Fatalf("journal leaks image bytes: %s", journal)
	}
}

// FM1/FM2: an opted-out model is refused with 400 before the upstream, and
// the error carries no image bytes.
func TestGatewayRefusesImageToOptedOutModelWith400(t *testing.T) {
	upstream := &fakeOpenCodeUpstream{}
	srv := upstream.server(t)
	p, err := provider.NewOpenCodeProvider(provider.OpenCodeConfig{APIKey: "k", BaseURL: srv.URL, ModelID: "cat-jam-free", ProviderName: "opencode-go"})
	if err != nil {
		t.Fatal(err)
	}
	reg := NewIdentityRegistry(time.Hour)
	cred, _ := reg.IssueCredential("vm-vision")
	h := NewHandlerWithRateLimit(reg, p, nil)
	logs := captureVisionLog(t)

	raw := visionTestPNG(t)
	w := postVisionInference(t, h, cred.RawToken, "cat-jam-free", raw)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "opted out of image input") {
		t.Fatalf("body = %s", w.Body.String())
	}
	if upstream.calls.Load() != 0 {
		t.Fatalf("upstream called %d times", upstream.calls.Load())
	}
	encoded := base64.StdEncoding.EncodeToString(raw)
	if strings.Contains(logs.String(), encoded[:32]) || strings.Contains(w.Body.String(), encoded[:32]) {
		t.Fatal("refusal leaks image bytes")
	}
}

// FM1: a refused image does not open the model breaker for every computer.
func TestBreakerDoesNotCountMediaRefusals(t *testing.T) {
	p, err := provider.NewOpenCodeProvider(provider.OpenCodeConfig{APIKey: "k", BaseURL: "http://127.0.0.1:1", ModelID: "cat-jam-free", ProviderName: "opencode-go"})
	if err != nil {
		t.Fatal(err)
	}
	cbp := NewCircuitBreakingProvider(p, health.BreakerConfig{FailureThreshold: 2, OpenTimeout: time.Hour})
	req := provider.LLMRequest{Model: "cat-jam-free", ConversationID: "c", Messages: []provider.Message{{Role: "user", Content: []provider.Block{
		{Type: "image", Source: &provider.MediaSource{Kind: "base64", Data: base64.StdEncoding.EncodeToString(visionTestPNG(t))}},
	}}}}
	for i := 0; i < 4; i++ {
		_, err := cbp.Call(context.Background(), req)
		if !provider.IsMediaError(err) {
			t.Fatalf("call %d: err = %v, want a media refusal", i, err)
		}
	}
	if cbp.ModelBreaker("cat-jam-free").State() == health.StateOpen {
		t.Fatal("media refusals opened the model breaker")
	}
}

// FM6: the OpenAI-compatible route keeps image_url parts.
func TestOpenAICompatKeepsImageParts(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString(visionTestPNG(t))
	raw := `{"model":"gpt-6-luna","messages":[{"role":"user","content":[{"type":"text","text":"what is this?"},{"type":"image_url","image_url":{"url":"data:image/png;base64,` + encoded + `"}}]}]}`
	var req openAIChatCompletionRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatal(err)
	}
	gw, err := convertOpenAIChatRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	blocks := gw.Messages[0].Content
	if len(blocks) != 2 || blocks[1].Type != "image" || blocks[1].Source == nil {
		t.Fatalf("blocks = %+v", blocks)
	}
	if blocks[1].Source.Kind != "base64" || blocks[1].Source.MIMEType != "image/png" || blocks[1].Source.Data != encoded {
		t.Fatalf("source = %+v", blocks[1].Source)
	}
}
