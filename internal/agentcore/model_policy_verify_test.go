package agentcore

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/imageblob"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// POST /api/model-policy/verify is the owner's (and, through
// choir.ProductAPI, management's) path to one model call with an image
// (docs/archive/problems/models-configured-text-only-2026-10-15.md, desk seam).
// Ways it can fail, written before the code:
//   - an unauthenticated caller spends the owner's model budget;
//   - inline base64 is echoed in the response or sent on in the model
//     message instead of a content-hash ref (FM11);
//   - an opted-out model is called anyway (FM2);
//   - a provider outage reads as the caller's mistake (400), or a refused
//     image as an outage (502);
//   - management gains the policy routes along with verify.

type verifyRouteProvider struct {
	request provideriface.ToolLoopRequest
	calls   int
	err     error
}

func (p *verifyRouteProvider) Execute(context.Context, *types.RunRecord, provideriface.EventEmitFunc) error {
	return nil
}
func (p *verifyRouteProvider) ProviderName() string { return "capture" }
func (p *verifyRouteProvider) CallWithTools(_ context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.request = req
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "a white square", Model: req.Model,
		Usage: provideriface.TokenUsage{InputTokens: 90, OutputTokens: 4}}, nil
}

func verifyRouteHandler(t *testing.T, p *verifyRouteProvider) (*APIHandler, *imageblob.Store) {
	t.Helper()
	store := imageblob.New(t.TempDir())
	rt := &Runtime{modelPolicy: modelpolicy.NewManager(modelpolicy.ManagerConfig{Provider: p, ImageStore: store})}
	return &APIHandler{rt: rt}, store
}

func postVerify(h *APIHandler, body string, auth bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/model-policy/verify", strings.NewReader(body))
	if auth {
		req.Header.Set("X-Authenticated-User", "owner-vision")
	}
	rec := httptest.NewRecorder()
	h.HandleModelPolicyRouter(rec, req)
	return rec
}

func whitePNGBase64(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestModelPolicyVerifyRouteSendsImageByRef(t *testing.T) {
	p := &verifyRouteProvider{}
	h, store := verifyRouteHandler(t, p)
	encoded := whitePNGBase64(t)
	body := `{"role":"verifier","provider":"chatgpt","model":"gpt-6-luna","prompt":"What colour is this square?","image_base64":"` + encoded + `"}`

	if rec := postVerify(h, body, false); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated = %d", rec.Code)
	}
	if p.calls != 0 {
		t.Fatal("unauthenticated request reached the provider")
	}

	rec := postVerify(h, body, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("verify = %d %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), encoded[:24]) {
		t.Fatal("response echoes image bytes")
	}
	var out struct {
		ImageRef    string `json:"image_ref"`
		Model       string `json:"model"`
		TextExcerpt string `json:"text_excerpt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.ImageRef, "artifact:sha256:") || out.Model != "gpt-6-luna" || out.TextExcerpt != "a white square" {
		t.Fatalf("result = %+v", out)
	}
	sent := string(p.request.Messages[0])
	if strings.Contains(sent, encoded[:24]) || !strings.Contains(sent, out.ImageRef) {
		t.Fatalf("model message = %s", sent)
	}
	if _, mimeType, err := store.Get(out.ImageRef); err != nil || mimeType != "image/png" {
		t.Fatalf("stored image: %q %v", mimeType, err)
	}

	// The stored ref can be scored again without resending the bytes.
	again := postVerify(h, `{"role":"verifier","provider":"opencode-go","model":"deepseek-v4.1-flash","prompt":"again","image_ref":"`+out.ImageRef+`"}`, true)
	if again.Code != http.StatusOK || p.request.Model != "deepseek-v4.1-flash" {
		t.Fatalf("by ref = %d %s", again.Code, again.Body)
	}
}

func TestModelPolicyVerifyRouteStatuses(t *testing.T) {
	p := &verifyRouteProvider{}
	h, _ := verifyRouteHandler(t, p)
	encoded := whitePNGBase64(t)
	rec := postVerify(h, `{"role":"verifier","provider":"opencode-go","model":"cat-jam-free","prompt":"x","image_base64":"`+encoded+`"}`, true)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "opted out") || p.calls != 0 {
		t.Fatalf("opted-out = %d %s calls=%d", rec.Code, rec.Body, p.calls)
	}
	rec = postVerify(h, `{"role":"verifier","provider":"chatgpt","model":"gpt-6-luna","prompt":"x","image_ref":"artifact:sha256:`+strings.Repeat("0", 64)+`"}`, true)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "not in this computer's image store") {
		t.Fatalf("missing ref = %d %s", rec.Code, rec.Body)
	}
	p.err = errors.New("gateway call failed: status 503")
	rec = postVerify(h, `{"role":"verifier","provider":"chatgpt","model":"gpt-6-luna","prompt":"x"}`, true)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("provider outage = %d %s", rec.Code, rec.Body)
	}
}

func TestProductAPIAdmitsOnlyTheVerifyPolicyRoute(t *testing.T) {
	if err := validateProductAPIRoute(http.MethodPost, "/api/model-policy/verify"); err != nil {
		t.Fatalf("POST verify refused: %v", err)
	}
	if err := validateProductAPIRoute(http.MethodGet, "/api/model-policy/verify"); err == nil {
		t.Fatal("GET verify admitted")
	}
	if err := validateProductAPIRoute(http.MethodGet, "/api/model-policy/resolve"); err == nil {
		t.Fatal("policy resolve admitted to a desk")
	}
}
