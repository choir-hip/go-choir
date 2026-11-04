package modelpolicy

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type capturingModelVerifyProvider struct {
	request provideriface.ToolLoopRequest
	calls   int
}

func (p *capturingModelVerifyProvider) Execute(_ context.Context, task *types.RunRecord, _ provideriface.EventEmitFunc) error {
	task.Result = "execute path unused"
	return nil
}

func (p *capturingModelVerifyProvider) ProviderName() string { return "capture" }

func (p *capturingModelVerifyProvider) CallWithTools(_ context.Context, request provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.request = request
	p.calls++
	return &provideriface.ToolLoopResponse{
		StopReason:       "end_turn",
		Text:             "verification passed",
		ReasoningContent: "hidden verifier reasoning",
		Usage:            provideriface.TokenUsage{InputTokens: 11, OutputTokens: 7},
		Model:            request.Model,
	}, nil
}

func verifierContext() context.Context {
	return toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID:   "run-verify",
		OwnerID: "owner",
		Role:    "management",
	})
}

func TestVerifyModelCapabilitySchemaAndExplicitPayload(t *testing.T) {
	provider := &capturingModelVerifyProvider{}
	manager := NewManager(ManagerConfig{Provider: provider})
	tool := NewVerifyModelCapabilityTool(manager)
	if tool.Name != "verify_model_capability" {
		t.Fatalf("tool name = %q", tool.Name)
	}
	required, _ := tool.Parameters["required"].([]string)
	if len(required) != 2 || required[0] != "role" || required[1] != "prompt" {
		t.Fatalf("required schema = %#v", tool.Parameters["required"])
	}
	output, err := tool.Func(verifierContext(), json.RawMessage(`{
		"role":"verifier",
		"provider":"opencode-go",
		"model":"deepseek-v4.1-flash",
		"prompt":"verify this text-only evidence"
	}`))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("provider calls = %d", provider.calls)
	}
	request := provider.request
	if request.Provider != "opencode-go" || request.Model != "deepseek-v4.1-flash" || request.MaxTokens != 0 {
		t.Fatalf("provider request = %+v", request)
	}
	if request.System != "You are a Choir verifier. Answer only the verification prompt. Do not mutate state." || len(request.Messages) != 1 || !strings.Contains(string(request.Messages[0]), "verify this text-only evidence") {
		t.Fatalf("provider payload = %+v", request)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatal(err)
	}
	if result["status"] != "verified" || result["role"] != "verifier" || result["image_input"] != false || result["max_tokens_requested"] != false || result["reasoning_content_present"] != true || result["input_tokens"] != float64(11) || result["output_tokens"] != float64(7) {
		t.Fatalf("result = %#v", result)
	}
}

// Owner 2026-10-15: vision is on by default (problems/models-configured-
// text-only-2026-10-15.md). A pool-resolved image call lands on a member
// that may see, never on the gated cat-jam-free (FM4).
func TestVerifyModelCapabilityImageOnPoolLandsOnVisionMember(t *testing.T) {
	for i := 0; i < 20; i++ {
		provider := &capturingModelVerifyProvider{}
		manager := NewManager(ManagerConfig{
			PolicyPath: filepath.Join(t.TempDir(), "System", "model-policy.toml"),
			Provider:   provider,
		})
		tool := NewVerifyModelCapabilityTool(manager)
		if _, err := tool.Func(verifierContext(), json.RawMessage(`{
			"role":"verifier_multimodal",
			"prompt":"describe this screenshot",
			"image_fixture":"red_pixel_png"
		}`)); err != nil {
			t.Fatalf("verify: %v", err)
		}
		if m := provider.request.Model; m != "deepseek-v4.1-flash" && m != "gpt-6-luna" && m != "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017" {
			t.Fatalf("image call assigned to %q, want a vision pool member", m)
		}
	}
}

// FM11: with the image store configured, the model call and the response
// carry the image's ref, not its bytes.
func TestVerifyModelCapabilityStoresInlineImageAndSendsRef(t *testing.T) {
	provider := &capturingModelVerifyProvider{}
	store := &memImageStore{blobs: map[string][]byte{}}
	tool := NewVerifyModelCapabilityTool(NewManager(ManagerConfig{Provider: provider, ImageStore: store}))
	out, err := tool.Func(verifierContext(), json.RawMessage(`{
		"role":"verifier","provider":"chatgpt","model":"gpt-6-luna",
		"prompt":"what colour?","image_base64":"`+verifierRedPixelPNGBase64+`"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	sent := string(provider.request.Messages[0])
	if strings.Contains(sent, verifierRedPixelPNGBase64[:20]) || !strings.Contains(sent, `"kind":"artifact_ref"`) {
		t.Fatalf("model call message = %s", sent)
	}
	if strings.Contains(out, verifierRedPixelPNGBase64[:20]) || !strings.Contains(out, `"image_ref":"artifact:sha256:`) {
		t.Fatalf("result = %s", out)
	}
	if len(store.blobs) != 1 {
		t.Fatalf("store holds %d blobs", len(store.blobs))
	}
}

type memImageStore struct{ blobs map[string][]byte }

func (m *memImageStore) Put(data []byte) (string, string, error) {
	ref := "artifact:sha256:" + strings.Repeat("0", 64)
	m.blobs[ref] = data
	return ref, "image/png", nil
}

func (m *memImageStore) Get(ref string) ([]byte, string, error) {
	data, ok := m.blobs[ref]
	if !ok {
		return nil, "", fmt.Errorf("image %s is not in this computer's image store", ref)
	}
	return data, "image/png", nil
}

// FM2: the explicit opt-out list holds on an explicit model choice.
func TestVerifyModelCapabilityRefusesImageOnOptedOutModel(t *testing.T) {
	provider := &capturingModelVerifyProvider{}
	tool := NewVerifyModelCapabilityTool(NewManager(ManagerConfig{Provider: provider}))
	_, err := tool.Func(verifierContext(), json.RawMessage(`{
		"role":"verifier","provider":"opencode-go","model":"cat-jam-free",
		"prompt":"describe","image_fixture":"red_pixel_png"
	}`))
	if err == nil || !strings.Contains(err.Error(), "opted out") {
		t.Fatalf("err = %v, want opt-out refusal", err)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d", provider.calls)
	}
}

func TestVerifyModelCapabilityRefusesImageOnExplicitTextOnlyModel(t *testing.T) {
	// FM3: an uncatalogued model is not assumed to see, even when named
	// explicitly.
	provider := &capturingModelVerifyProvider{}
	tool := NewVerifyModelCapabilityTool(NewManager(ManagerConfig{Provider: provider}))
	_, err := tool.Func(verifierContext(), json.RawMessage(`{
		"role":"verifier_multimodal",
		"provider":"xiaomi",
		"model":"mimo-v2.5",
		"prompt":"describe the fixture",
		"image_fixture":"red_pixel_png"
	}`))
	if err == nil {
		t.Fatalf("expected the image input to refuse on a text-only catalog model")
	}
	if !strings.Contains(err.Error(), "multimodal") {
		t.Fatalf("refusal = %v, want a multimodal capability refusal", err)
	}
	if provider.calls != 0 {
		t.Fatalf("refused image must not reach the provider: calls=%d", provider.calls)
	}
}

func TestVerifyModelCapabilityRejectsBadImagesBeforeProvider(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "uncatalogued model",
			input: `{"role":"verifier","provider":"fireworks","model":"accounts/fireworks/models/deepseek-v4-pro","prompt":"describe","image_url":"https://example.com/screen.png"}`,
			want:  "not in the Choir model catalog",
		},
		{
			name:  "malformed base64",
			input: `{"role":"verifier_multimodal","provider":"xiaomi","model":"mimo-v2.5","prompt":"describe","image_base64":"not valid"}`,
			want:  "valid standard base64",
		},
		{
			name:  "relative URL",
			input: `{"role":"verifier_multimodal","provider":"xiaomi","model":"mimo-v2.5","prompt":"describe","image_url":"/screen.png"}`,
			want:  "absolute http(s)",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := &capturingModelVerifyProvider{}
			tool := NewVerifyModelCapabilityTool(NewManager(ManagerConfig{Provider: provider}))
			_, err := tool.Func(verifierContext(), json.RawMessage(test.input))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			if provider.calls != 0 {
				t.Fatalf("provider calls = %d", provider.calls)
			}
		})
	}
}

// The judge's per-shot review (docs/problems/judge-checks-only-named-
// defects-2026-11-04.md): image_refs sends a batch in order, each image after
// its "Image N:" label, refuses a ref the store does not hold, a second
// image source, and more than the gateway's 20 images.
func TestVerifyModelCapabilityImageRefsSendsALabelledBatch(t *testing.T) {
	refA, refB := "artifact:sha256:"+strings.Repeat("a", 64), "artifact:sha256:"+strings.Repeat("b", 64)
	store := &memImageStore{blobs: map[string][]byte{refA: {1}, refB: {2}}}
	provider := &capturingModelVerifyProvider{}
	manager := NewManager(ManagerConfig{Provider: provider, ImageStore: store})
	out, err := manager.VerifyImages(context.Background(), "owner", "verifier_multimodal", "judge each", []string{refA, refB})
	if err != nil {
		t.Fatal(err)
	}
	if out.Text != "verification passed" || !out.ImageInput || out.InputTokens != 11 {
		t.Fatalf("outcome = %+v", out)
	}
	sent := string(provider.request.Messages[0])
	ia, la, ib, lb := strings.Index(sent, `"Image 1:"`), strings.Index(sent, refA), strings.Index(sent, `"Image 2:"`), strings.Index(sent, refB)
	if ia < 0 || !(ia < la && la < ib && ib < lb) {
		t.Fatalf("batch message order wrong: %s", sent)
	}
	tool := NewVerifyModelCapabilityTool(manager)
	for name, input := range map[string]string{
		"missing ref":  `{"role":"verifier_multimodal","prompt":"p","image_refs":["artifact:sha256:` + strings.Repeat("c", 64) + `"]}`,
		"two sources":  `{"role":"verifier_multimodal","prompt":"p","image_ref":"` + refA + `","image_refs":["` + refB + `"]}`,
		"not a ref":    `{"role":"verifier_multimodal","prompt":"p","image_refs":["https://x/y.png"]}`,
		"over the cap": `{"role":"verifier_multimodal","prompt":"p","image_refs":[` + strings.TrimSuffix(strings.Repeat(`"`+refA+`",`, 21), ",") + `]}`,
	} {
		calls := provider.calls
		if _, err := tool.Func(verifierContext(), json.RawMessage(input)); err == nil {
			t.Fatalf("%s: admitted", name)
		}
		if provider.calls != calls {
			t.Fatalf("%s: the model was called", name)
		}
	}
}
