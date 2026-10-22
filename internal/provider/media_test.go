package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Failure modes: docs/problems/models-configured-text-only-2026-10-15.md.

func testPNG(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func imageRequest(model string, raw []byte) LLMRequest {
	return LLMRequest{
		Model:          model,
		ConversationID: "run-vision",
		Messages: []Message{{
			Role: "user",
			Content: []Block{
				{Type: "text", Text: "what colour is this?"},
				{Type: "image", Source: &MediaSource{Kind: "base64", MIMEType: "image/png", Data: base64.StdEncoding.EncodeToString(raw)}},
			},
		}},
		MaxTokens: 64,
	}
}

// FM1/FM2: an opted-out model is refused before the upstream is called, as
// a typed media error (400, not retried, not counted by the breaker).
func TestOptedOutModelRefusesImageBeforeUpstream(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	p, err := NewOpenCodeProvider(OpenCodeConfig{APIKey: "k", BaseURL: server.URL, ModelID: "cat-jam-free", ProviderName: "opencode-go"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Call(context.Background(), imageRequest("cat-jam-free", testPNG(t, 4, 4, color.White)))
	if err == nil || !strings.Contains(err.Error(), "opted out of image input") {
		t.Fatalf("err = %v, want opt-out refusal", err)
	}
	if !IsMediaError(err) {
		t.Fatalf("err %T is not a MediaError", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("upstream called %d times for a refused image", calls.Load())
	}
}

// FM3: an uncatalogued model is not assumed to see.
func TestUncataloguedModelRefusesImage(t *testing.T) {
	_, err := prepareMediaRequest("fireworks", "accounts/fireworks/models/kimi-k2p6", imageRequest("x", testPNG(t, 2, 2, color.White)))
	if err == nil || !strings.Contains(err.Error(), "not in the Choir model catalog") {
		t.Fatalf("err = %v", err)
	}
}

func TestPrepareMediaRequestRejectsBadSources(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		source   *MediaSource
		want     string
	}{
		{"malformed base64", "chatgpt", &MediaSource{Kind: "base64", Data: "not standard base64"}, "valid standard base64"},
		{"relative url", "chatgpt", &MediaSource{Kind: "url", URL: "/local/screen.png"}, "absolute http(s) URL"},
		{"unresolved ref", "chatgpt", &MediaSource{Kind: "artifact_ref", Ref: "artifact:sha256:00"}, "resolved by the computer"},
		// FM5: Bedrock takes base64 only; URL fetching elsewhere is unverified.
		{"url to bedrock", "bedrock", &MediaSource{Kind: "url", URL: "https://example.com/a.png"}, "does not take image URLs"},
		{"url to opencode", "opencode-go", &MediaSource{Kind: "url", URL: "https://example.com/a.png"}, "does not take image URLs"},
		{"not an image", "chatgpt", &MediaSource{Kind: "base64", Data: base64.StdEncoding.EncodeToString([]byte("plain text, not pixels"))}, "not a PNG"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := LLMRequest{Messages: []Message{{Role: "user", Content: []Block{{Type: "image", Source: tc.source}}}}}
			_, err := prepareMediaRequest(tc.provider, "gpt-6-luna", req)
			if err == nil || !strings.Contains(err.Error(), tc.want) || !IsMediaError(err) {
				t.Fatalf("err = %v, want media error containing %q", err, tc.want)
			}
		})
	}
}

// FM7: count cap.
func TestPrepareMediaRequestCapsImageCount(t *testing.T) {
	raw := testPNG(t, 2, 2, color.White)
	var blocks []Block
	for i := 0; i <= MaxImagesPerRequest; i++ {
		blocks = append(blocks, Block{Type: "image", Source: &MediaSource{Kind: "base64", Data: base64.StdEncoding.EncodeToString(raw)}})
	}
	_, err := prepareMediaRequest("chatgpt", "gpt-6-luna", LLMRequest{Messages: []Message{{Role: "user", Content: blocks}}})
	if err == nil || !strings.Contains(err.Error(), "the limit is 20") {
		t.Fatalf("err = %v", err)
	}
}

// FM7: a large image is downscaled to the long-edge cap, deterministically
// (FM10: same bytes in, same bytes out, so the prompt cache holds), and the
// caller's request is not mutated.
func TestPrepareMediaRequestDownscalesLargeImages(t *testing.T) {
	raw := testPNG(t, 2000, 1000, color.RGBA{R: 200, G: 10, B: 10, A: 255})
	req := imageRequest("gpt-6-luna", raw)
	original := req.Messages[0].Content[1].Source.Data
	first, err := prepareMediaRequest("chatgpt", "gpt-6-luna", req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := prepareMediaRequest("chatgpt", "gpt-6-luna", req)
	if err != nil {
		t.Fatal(err)
	}
	if req.Messages[0].Content[1].Source.Data != original {
		t.Fatal("prepareMediaRequest mutated the caller's request")
	}
	got := first.Messages[0].Content[1].Source
	if got.Data != second.Messages[0].Content[1].Source.Data {
		t.Fatal("downscaling is not deterministic")
	}
	if got.MIMEType != "image/png" {
		t.Fatalf("mime = %q", got.MIMEType)
	}
	decoded, _ := base64.StdEncoding.DecodeString(got.Data)
	cfg, err := png.DecodeConfig(bytes.NewReader(decoded))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != MaxImageLongEdge || cfg.Height != MaxImageLongEdge/2 {
		t.Fatalf("downscaled to %dx%d, want %dx%d", cfg.Width, cfg.Height, MaxImageLongEdge, MaxImageLongEdge/2)
	}
	img, _ := png.Decode(bytes.NewReader(decoded))
	if r, g, b, _ := img.At(10, 10).RGBA(); r>>8 != 200 || g>>8 != 10 || b>>8 != 10 {
		t.Fatalf("colour drifted: %d %d %d", r>>8, g>>8, b>>8)
	}
	// A small image passes byte for byte.
	small := testPNG(t, 8, 8, color.White)
	out, err := prepareMediaRequest("chatgpt", "gpt-6-luna", imageRequest("gpt-6-luna", small))
	if err != nil {
		t.Fatal(err)
	}
	if out.Messages[0].Content[1].Source.Data != base64.StdEncoding.EncodeToString(small) {
		t.Fatal("small image was re-encoded")
	}
}

// FM7: a decompression bomb is refused from its header, before decoding.
func TestNormalizeImageRefusesPixelBombFromHeader(t *testing.T) {
	raw := testPNG(t, 1, 1, color.White)
	// Rewrite IHDR to claim 20000x20000 and fix its CRC.
	binary.BigEndian.PutUint32(raw[16:20], 20000)
	binary.BigEndian.PutUint32(raw[20:24], 20000)
	binary.BigEndian.PutUint32(raw[29:33], crc32.ChecksumIEEE(raw[12:29]))
	_, _, err := NormalizeImage(raw)
	if err == nil || !strings.Contains(err.Error(), "pixels") {
		t.Fatalf("err = %v, want pixel-limit refusal", err)
	}
}

func TestNormalizeImageRefusesOversizedInput(t *testing.T) {
	big := make([]byte, MaxImageInputBytes+1)
	copy(big, testPNG(t, 1, 1, color.White))
	if _, _, err := NormalizeImage(big); err == nil || !strings.Contains(err.Error(), "limit before resizing") {
		t.Fatalf("err = %v", err)
	}
}

// Request building per provider path, through each adapter's real Call.

func TestOpenCodeChatCompletionsSendsImageURLPart(t *testing.T) {
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		_ = json.NewEncoder(w).Encode(openAIChatCompletionResponse{
			ID: "chat-1", Model: "deepseek-v4.1-flash",
			Choices: []openAIChatChoice{{Message: openAIChatMessage{Content: "red"}, FinishReason: "stop"}},
		})
	}))
	defer server.Close()
	p, err := NewOpenCodeProvider(OpenCodeConfig{APIKey: "k", BaseURL: server.URL, ModelID: "deepseek-v4.1-flash", ProviderName: "opencode-go"})
	if err != nil {
		t.Fatal(err)
	}
	raw := testPNG(t, 4, 4, color.RGBA{R: 255, A: 255})
	if _, err := p.Call(context.Background(), imageRequest("deepseek-v4.1-flash", raw)); err != nil {
		t.Fatal(err)
	}
	var sent struct {
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatal(err)
	}
	var parts []map[string]any
	if err := json.Unmarshal(sent.Messages[len(sent.Messages)-1].Content, &parts); err != nil {
		t.Fatalf("user content is not parts: %s", sent.Messages[len(sent.Messages)-1].Content)
	}
	if len(parts) != 2 || parts[0]["type"] != "text" || parts[1]["type"] != "image_url" {
		t.Fatalf("parts = %v", parts)
	}
	url, _ := parts[1]["image_url"].(map[string]any)["url"].(string)
	if url != "data:image/png;base64,"+base64.StdEncoding.EncodeToString(raw) {
		t.Fatalf("image_url = %.60q", url)
	}
}

func TestChatGPTResponsesSendsInputImage(t *testing.T) {
	p := &ChatGPTProvider{modelID: "gpt-6-luna", reasoning: "low"}
	raw := testPNG(t, 4, 4, color.RGBA{B: 255, A: 255})
	req, err := prepareMediaRequest("chatgpt", "gpt-6-luna", imageRequest("gpt-6-luna", raw))
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(p.buildRequestBody(req, "gpt-6-luna"))
	want := `{"image_url":"data:image/png;base64,` + base64.StdEncoding.EncodeToString(raw) + `","type":"input_image"}`
	if !strings.Contains(string(data), want) {
		t.Fatalf("responses body lacks input_image part: %.400s", data)
	}
}

func TestBedrockSendsAnthropicImageBlock(t *testing.T) {
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(anthropicResponse{ID: "msg", Content: []anthropicResponseBlock{{Type: "text", Text: "green"}}, StopReason: "end_turn"})
	}))
	defer server.Close()
	model := "us.anthropic.claude-haiku-4-5-20251001-v1:0"
	p := &BedrockProvider{
		region: "us-east-1", modelID: model, authToken: "t", anthropicV: "bedrock-2023-05-31",
		httpClient: &http.Client{Timeout: 10 * time.Second, Transport: &rewriteTransport{target: server.URL, original: "https://bedrock-runtime.us-east-1.amazonaws.com"}},
	}
	raw := testPNG(t, 4, 4, color.RGBA{G: 255, A: 255})
	if _, err := p.Call(context.Background(), imageRequest(model, raw)); err != nil {
		t.Fatal(err)
	}
	want := `{"type":"image","source":{"data":"` + base64.StdEncoding.EncodeToString(raw) + `","media_type":"image/png","type":"base64"}`
	if !strings.Contains(string(body), want) {
		t.Fatalf("bedrock body lacks anthropic image block: %.400s", body)
	}
}

// FM11: the log summary names images by hash and size, never by data.
func TestSummarizeMediaCarriesNoImageData(t *testing.T) {
	raw := testPNG(t, 64, 64, color.RGBA{R: 1, G: 2, B: 3, A: 255})
	req := imageRequest("gpt-6-luna", raw)
	line := SummarizeMedia(req.Messages).String()
	encoded := base64.StdEncoding.EncodeToString(raw)
	if strings.Contains(line, encoded[:24]) {
		t.Fatalf("summary leaks image data: %s", line)
	}
	if !strings.Contains(line, "images=1") || !strings.Contains(line, "image_bytes=") {
		t.Fatalf("summary = %s", line)
	}
}

// Bunch 109 (DeepSeek F1): decoded pixels are outside the gateway's body
// budget, so at most MaxConcurrentImageDecodes images are decoded at once
// process-wide. Fails on 2e9ebc5eb: there is no bound (and no hook).
func TestNormalizeImageDecodesAtMostABoundedNumberAtOnce(t *testing.T) {
	raw := testPNG(t, MaxImageLongEdge*2, 64, color.RGBA{R: 200, A: 255})
	var mu sync.Mutex
	var now, peak int
	imageDecodeHook = func() {
		mu.Lock()
		now++
		peak = max(peak, now)
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		now--
		mu.Unlock()
	}
	defer func() { imageDecodeHook = nil }()
	var wg sync.WaitGroup
	for i := 0; i < 4*MaxConcurrentImageDecodes; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := NormalizeImage(raw); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if peak == 0 || peak > MaxConcurrentImageDecodes {
		t.Fatalf("%d images decoded at once, want 1 to %d", peak, MaxConcurrentImageDecodes)
	}
}
