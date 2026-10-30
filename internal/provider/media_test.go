package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Failure modes: docs/archive/problems/models-configured-text-only-2026-10-15.md.

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
	_, err := prepareReleased(t, context.Background(), "fireworks", "accounts/fireworks/models/kimi-k2p6", imageRequest("x", testPNG(t, 2, 2, color.White)))
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
			_, err := prepareReleased(t, context.Background(), tc.provider, "gpt-6-luna", req)
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
	_, err := prepareReleased(t, context.Background(), "chatgpt", "gpt-6-luna", LLMRequest{Messages: []Message{{Role: "user", Content: blocks}}})
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
	first, err := prepareReleased(t, context.Background(), "chatgpt", "gpt-6-luna", req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := prepareReleased(t, context.Background(), "chatgpt", "gpt-6-luna", req)
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
	out, err := prepareReleased(t, context.Background(), "chatgpt", "gpt-6-luna", imageRequest("gpt-6-luna", small))
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
	req, err := prepareReleased(t, context.Background(), "chatgpt", "gpt-6-luna", imageRequest("gpt-6-luna", raw))
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
	imageDecodeHook = func(string) {
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

// --- review round (bunch 116): F1 base64 inside the slot, F2 context and
// per-caller share of the decode slots ---

// slotProbe blocks every decode slot holder in the hook until open, and
// counts the holders per caller.
type slotProbe struct {
	mu     sync.Mutex
	active map[string]int
	peak   map[string]int
	gate   chan struct{}
	once   sync.Once
	wg     sync.WaitGroup
}

func newSlotProbe(t *testing.T) *slotProbe {
	t.Helper()
	p := &slotProbe{active: map[string]int{}, peak: map[string]int{}, gate: make(chan struct{})}
	imageDecodeHook = func(caller string) {
		p.mu.Lock()
		p.active[caller]++
		p.peak[caller] = max(p.peak[caller], p.active[caller])
		p.mu.Unlock()
		<-p.gate
		p.mu.Lock()
		p.active[caller]--
		p.mu.Unlock()
	}
	t.Cleanup(func() {
		p.open()
		p.wg.Wait()
		imageDecodeHook = nil
	})
	return p
}

func (p *slotProbe) open() { p.once.Do(func() { close(p.gate) }) }

func (p *slotProbe) count(caller string) (active, peak int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.active[caller], p.peak[caller]
}

func (p *slotProbe) total() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, v := range p.active {
		n += v
	}
	return n
}

// fill takes every decode slot with a caller-less NormalizeImage of an
// image it must downscale, blocked in the hook.
func (p *slotProbe) fill(t *testing.T) {
	t.Helper()
	raw := testPNG(t, MaxImageLongEdge*2, 8, color.White)
	for i := 0; i < MaxConcurrentImageDecodes; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			_, _, _ = NormalizeImage(raw)
		}()
	}
	waitFor(t, "every decode slot to be held", func() bool { return p.total() == MaxConcurrentImageDecodes })
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// F1: a base64 image is decoded from base64 inside a decode slot, so the
// up to 20 MiB raw copy of every call queued for a slot is not held
// outside the 4-slot arithmetic. Fails on 982a70f3a (beyond the new ctx
// parameter): a small base64 image is decoded and returned at once while
// every slot is held, since only a downscale took a slot.
func TestPrepareImageDecodesItsBase64InsideADecodeSlot(t *testing.T) {
	p := newSlotProbe(t)
	p.fill(t)
	req := imageRequest("gpt-6-luna", testPNG(t, 4, 4, color.White))
	done := make(chan error, 1)
	go func() {
		_, err := prepareReleased(t, context.Background(), "chatgpt", "gpt-6-luna", req)
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("a base64 image was prepared (err %v) while every decode slot was held", err)
	case <-time.After(200 * time.Millisecond):
	}
	p.open()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the image was never prepared after the slots were given back")
	}
}

// F2: a call waiting for a decode slot leaves the queue when its context
// ends, with the context's error, not a MediaError (which would be a
// non-transient 400). Fails on 982a70f3a: prepareMediaRequest takes no
// context and the wait ignores it.
func TestImageDecodeSlotWaitEndsWithTheCallsContext(t *testing.T) {
	p := newSlotProbe(t)
	p.fill(t)
	req := imageRequest("gpt-6-luna", testPNG(t, MaxImageLongEdge*2, 8, color.White))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := prepareReleased(t, ctx, "chatgpt", "gpt-6-luna", req)
		done <- err
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || IsMediaError(err) || !errors.Is(err, ErrImageDecodeWait) {
			t.Fatalf("a cancelled slot wait = %v, want ErrImageDecodeWait wrapping context.Canceled, not a MediaError", err)
		}
	case <-time.After(time.Second):
		t.Fatal("a cancelled call stayed queued for a decode slot")
	}
}

// F2: one caller holds or waits on at most MaxImageDecodesPerCaller of
// the decode slots, so another caller's image is decoded while the first
// keeps more queued. Fails on 982a70f3a: there is no caller in the
// context, and one caller takes every slot.
func TestImageDecodesOneCallerHoldsAtMostItsShareOfTheSlots(t *testing.T) {
	p := newSlotProbe(t)
	raw := testPNG(t, MaxImageLongEdge*2, 8, color.White)
	prepare := func(caller string, done chan<- error) {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			_, err := prepareReleased(t, WithImageDecodeCaller(context.Background(), caller), "chatgpt", "gpt-6-luna", imageRequest("gpt-6-luna", raw))
			done <- err
		}()
	}
	hog := make(chan error, 2*MaxConcurrentImageDecodes)
	for i := 0; i < 2*MaxConcurrentImageDecodes; i++ {
		prepare("computer-hog", hog)
	}
	waitFor(t, "the hog to hold its share", func() bool { a, _ := p.count("computer-hog"); return a >= MaxImageDecodesPerCaller })
	time.Sleep(100 * time.Millisecond)
	if a, _ := p.count("computer-hog"); a != MaxImageDecodesPerCaller {
		t.Fatalf("one caller holds %d decode slots, want at most %d", a, MaxImageDecodesPerCaller)
	}
	other := make(chan error, 1)
	prepare("computer-other", other)
	waitFor(t, "another caller's image to take a slot", func() bool { a, _ := p.count("computer-other"); return a == 1 })
	p.open()
	for i := 0; i < 2*MaxConcurrentImageDecodes; i++ {
		if err := <-hog; err != nil {
			t.Fatal(err)
		}
	}
	if err := <-other; err != nil {
		t.Fatal(err)
	}
	if _, peak := p.count("computer-hog"); peak > MaxImageDecodesPerCaller {
		t.Fatalf("one caller held %d decode slots at once, want at most %d", peak, MaxImageDecodesPerCaller)
	}
}

// F1: the log summary hashes a base64 image without decoding it whole.
// Fails on 982a70f3a: SummarizeMedia decodes the whole image (8 MiB here)
// outside any slot.
func TestSummarizeMediaHashesWithoutDecodingTheWholeImage(t *testing.T) {
	raw := bytes.Repeat([]byte{0x89, 'P', 'N', 'G'}, 2<<20) // 8 MiB
	msgs := imageRequest("m", raw).Messages
	want := sha256Prefix(raw)
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	stats := SummarizeMedia(msgs)
	runtime.ReadMemStats(&after)
	if stats.Images != 1 || stats.Bytes != len(raw) || len(stats.Refs) != 1 || stats.Refs[0] != want {
		t.Fatalf("summary = %+v, want 1 image of %d bytes named %s", stats, len(raw), want)
	}
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 1<<20 {
		t.Fatalf("SummarizeMedia allocated %d bytes for an 8 MiB image, want under 1 MiB", alloc)
	}
	if got := SummarizeMedia([]Message{{Content: []Block{{Type: "image", Source: &MediaSource{Kind: "base64", Data: "not base64!"}}}}}); len(got.Refs) != 1 || got.Refs[0] != "invalid" {
		t.Fatalf("invalid base64 summary = %+v, want ref invalid", got)
	}
}

func sha256Prefix(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])[:12]
}

// prepareReleased is prepareMediaRequest with its output bytes given back
// when the test ends.
func prepareReleased(t *testing.T, ctx context.Context, providerName, modelID string, req LLMRequest) (LLMRequest, error) {
	out, release, err := prepareMediaRequest(ctx, providerName, modelID, req)
	t.Cleanup(release)
	return out, err
}

// --- FP0 round 5 (F7): the downscaled output budget ---

// useImageOutputBudget swaps in a budget of size bytes with a per-caller
// share for the test.
func useImageOutputBudget(t *testing.T, size, share int64) *imageOutputBudget {
	t.Helper()
	b := newImageOutputBudget(size, share)
	old := imageOutput
	imageOutput = b
	t.Cleanup(func() { imageOutput = old })
	return b
}

type prepared struct {
	release func()
	err     error
}

func prepareAsync(ctx context.Context, caller string, req LLMRequest) chan prepared {
	done := make(chan prepared, 1)
	go func() {
		_, release, err := prepareMediaRequest(WithImageDecodeCaller(ctx, caller), "chatgpt", "gpt-6-luna", req)
		done <- prepared{release, err}
	}()
	return done
}

func waitPrepared(t *testing.T, what string, done chan prepared) prepared {
	t.Helper()
	select {
	case p := <-done:
		return p
	case <-time.After(3 * time.Second):
		t.Fatalf("%s was never prepared", what)
	}
	return prepared{}
}

func stillWaiting(t *testing.T, what string, done chan prepared) {
	t.Helper()
	select {
	case p := <-done:
		t.Fatalf("%s was prepared (err %v) while the output budget was full", what, p.err)
	case <-time.After(200 * time.Millisecond):
	}
}

// F7: a call's downscaled output stays counted until the call releases it;
// a call whose worst case does not fit waits, and leaves the queue with
// ErrImageDecodeWait (not a MediaError) when its context ends; an image
// passed unchanged holds nothing after the call is prepared. Fails on
// 142c17bd1: prepareMediaRequest returns no release and nothing bounds
// the output.
func TestImageOutputIsHeldUntilTheCallReleasesIt(t *testing.T) {
	b := useImageOutputBudget(t, ImageOutputPerImage, ImageOutputPerImage)
	big := imageRequest("gpt-6-luna", testPNG(t, MaxImageLongEdge*2, 8, color.White))

	small := waitPrepared(t, "an unchanged image", prepareAsync(context.Background(), "computer-a", imageRequest("gpt-6-luna", testPNG(t, 4, 4, color.White))))
	if small.err != nil || b.inUse() != 0 {
		t.Fatalf("an unchanged image: err %v, output held %d, want nil and 0", small.err, b.inUse())
	}
	small.release()

	a := waitPrepared(t, "the first call", prepareAsync(context.Background(), "computer-a", big))
	if a.err != nil {
		t.Fatal(a.err)
	}
	if held := b.inUse(); held <= 0 || held >= ImageOutputPerImage {
		t.Fatalf("after its call is prepared the output held is %d, want its downscaled base64 only (0 < n < %d)", held, ImageOutputPerImage)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancelled := prepareAsync(ctx, "computer-b", big)
	stillWaiting(t, "a call whose worst case does not fit", cancelled)
	cancel()
	c := waitPrepared(t, "a cancelled call", cancelled)
	if !errors.Is(c.err, ErrImageDecodeWait) || !errors.Is(c.err, context.Canceled) || IsMediaError(c.err) {
		t.Fatalf("a cancelled output wait = %v, want ErrImageDecodeWait wrapping context.Canceled, not a MediaError", c.err)
	}

	next := prepareAsync(context.Background(), "computer-b", big)
	stillWaiting(t, "the next call", next)
	a.release()
	a.release() // twice is once
	n := waitPrepared(t, "the next call after the first released", next)
	if n.err != nil {
		t.Fatal(n.err)
	}
	n.release()
	st := ImageMediaStatus()
	if b.inUse() != 0 || st.OutputInUse != 0 {
		t.Fatalf("after every call released, output held %d (status %d), want 0", b.inUse(), st.OutputInUse)
	}
	if st.OutputWaited < 2 || st.OutputRefused != 1 || st.OutputLimit != ImageOutputPerImage {
		t.Fatalf("status = %+v, want output_waited >= 2, output_refused 1, output_limit %d", st, ImageOutputPerImage)
	}
}

// F7: one caller holds at most its share of the output budget (its first
// call always fits); another caller's call goes ahead of its waiting one.
// Fails on 142c17bd1: no output budget.
func TestImageOutputOneCallerHoldsAtMostItsShare(t *testing.T) {
	b := useImageOutputBudget(t, 4*ImageOutputPerImage, ImageOutputPerImage)
	big := imageRequest("gpt-6-luna", testPNG(t, MaxImageLongEdge*2, 8, color.White))
	first := waitPrepared(t, "the hog's first call", prepareAsync(context.Background(), "computer-hog", big))
	if first.err != nil {
		t.Fatal(first.err)
	}
	second := prepareAsync(context.Background(), "computer-hog", big)
	stillWaiting(t, "the hog's second call over its share", second)
	other := waitPrepared(t, "another caller's call", prepareAsync(context.Background(), "computer-other", big))
	if other.err != nil {
		t.Fatal(other.err)
	}
	other.release()
	first.release()
	s := waitPrepared(t, "the hog's second call after its first released", second)
	if s.err != nil {
		t.Fatal(s.err)
	}
	s.release()
	if b.inUse() != 0 {
		t.Fatalf("output held %d after every call released, want 0", b.inUse())
	}
}

// F7: the reservation is given back on a refused image and on a panic in
// the decode, and the decode slot too. Fails on 142c17bd1: no output
// budget (and a panic in the hook leaked its decode slot).
func TestImageOutputIsGivenBackOnErrorAndPanic(t *testing.T) {
	b := useImageOutputBudget(t, 4*ImageOutputPerImage, 4*ImageOutputPerImage)
	req := imageRequest("gpt-6-luna", testPNG(t, MaxImageLongEdge*2, 8, color.White))
	req.Messages[0].Content = append(req.Messages[0].Content, Block{Type: "image", Source: &MediaSource{Kind: "base64", Data: "not base64!"}})
	_, release, err := prepareMediaRequest(context.Background(), "chatgpt", "gpt-6-luna", req)
	release()
	if !IsMediaError(err) || b.inUse() != 0 {
		t.Fatalf("a refused second image: err %v, output held %d, want a MediaError and 0", err, b.inUse())
	}

	imageDecodeHook = func(string) { panic("decoder exploded") }
	defer func() { imageDecodeHook = nil }()
	func() {
		defer func() { _ = recover() }()
		_, _, _ = prepareMediaRequest(context.Background(), "chatgpt", "gpt-6-luna", imageRequest("gpt-6-luna", testPNG(t, MaxImageLongEdge*2, 8, color.White)))
	}()
	if b.inUse() != 0 || len(imageDecodeSlots) != 0 {
		t.Fatalf("after a panic in the decode: output held %d, decode slots held %d, want 0 and 0", b.inUse(), len(imageDecodeSlots))
	}
}

// Bunch 121 D1: with the production sizes a full call's reservation
// (20 x ImageOutputPerImage, 133.3 MiB) is over one caller's share
// (96 MiB); it is granted at once when its caller holds nothing, and a
// second call of the same caller waits while another caller's goes ahead.
// Passes on f78ea9f96; fails when the first-reservation guard
// (held > 0) is removed from grantLocked.
func TestImageOutputFirstFullCallOverTheShareIsGrantedAtOnce(t *testing.T) {
	full := int64(MaxImagesPerRequest) * ImageOutputPerImage
	if full <= MaxImageOutputBytesPerCaller {
		t.Fatalf("a full call (%d) is within the share (%d); this test no longer pins the first-call rule", full, MaxImageOutputBytesPerCaller)
	}
	b := newImageOutputBudget(MaxImageOutputBytes, MaxImageOutputBytesPerCaller)
	acquire := func(ctx context.Context, caller string, n int64) chan error {
		done := make(chan error, 1)
		go func() { done <- b.acquire(ctx, caller, n) }()
		return done
	}
	select {
	case err := <-acquire(context.Background(), "computer-a", full):
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatalf("a first full call of %d bytes over the %d share waited on an empty budget", full, MaxImageOutputBytesPerCaller)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	second := acquire(ctx, "computer-a", ImageOutputPerImage)
	other := acquire(context.Background(), "computer-b", ImageOutputPerImage)
	select {
	case err := <-other:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("another caller's call waited behind a caller over its share")
	}
	select {
	case err := <-second:
		t.Fatalf("the same caller's second call was granted (err %v) while it held %d over its share", err, full)
	case <-time.After(200 * time.Millisecond):
	}
	cancel()
	if err := <-second; !errors.Is(err, ErrImageDecodeWait) {
		t.Fatalf("the cancelled second call = %v, want ErrImageDecodeWait", err)
	}
	b.release("computer-a", full)
	b.release("computer-b", ImageOutputPerImage)
	if b.inUse() != 0 {
		t.Fatalf("output held %d after every release, want 0", b.inUse())
	}
}
