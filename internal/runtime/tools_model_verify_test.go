package runtime

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

type capturingModelVerifyProvider struct {
	req   ToolLoopRequest
	calls int
}

func (p *capturingModelVerifyProvider) Execute(ctx context.Context, task *types.RunRecord, emit EventEmitFunc) error {
	task.Result = "execute path unused"
	return nil
}

func (p *capturingModelVerifyProvider) ProviderName() string { return "capture" }

func (p *capturingModelVerifyProvider) CallWithTools(ctx context.Context, req ToolLoopRequest) (*ToolLoopResponse, error) {
	p.req = req
	p.calls++
	return &ToolLoopResponse{
		StopReason:       "end_turn",
		Text:             "verification passed",
		ReasoningContent: "hidden verifier reasoning",
		Usage:            TokenUsage{InputTokens: 11, OutputTokens: 7},
		Model:            req.Model,
	}, nil
}

func TestVerifyModelCapabilityExplicitTextOnlyOmitFireworksMaxTokens(t *testing.T) {
	provider := &capturingModelVerifyProvider{}
	rt := New(Config{StorePath: filepath.Join(t.TempDir(), "runtime.db")}, nil, nil, provider)
	tool := newVerifyModelCapabilityTool(rt)
	ctx := WithToolExecutionContext(context.Background(), &types.RunRecord{
		RunID:        "run-verify-text",
		OwnerID:      "owner",
		AgentProfile: AgentProfileSuper,
		AgentRole:    AgentProfileSuper,
	})

	out, err := tool.Func(ctx, json.RawMessage(`{
		"role":"verifier",
		"provider":"fireworks",
		"model":"accounts/fireworks/models/deepseek-v4-flash",
		"prompt":"verify this text-only evidence"
	}`))
	if err != nil {
		t.Fatalf("verify_model_capability: %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}
	if provider.req.Provider != "fireworks" || provider.req.Model != "accounts/fireworks/models/deepseek-v4-flash" {
		t.Fatalf("request provider/model = %s/%s", provider.req.Provider, provider.req.Model)
	}
	if provider.req.MaxTokens != 0 {
		t.Fatalf("MaxTokens = %d, want omitted/0 for Fireworks default", provider.req.MaxTokens)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result["image_input"] != false || result["max_tokens_requested"] != false || result["reasoning_content_present"] != true {
		t.Fatalf("unexpected result evidence: %#v", result)
	}
}

func TestVerifyModelCapabilityRejectsImageForTextOnlyModel(t *testing.T) {
	provider := &capturingModelVerifyProvider{}
	rt := New(Config{StorePath: filepath.Join(t.TempDir(), "runtime.db")}, nil, nil, provider)
	tool := newVerifyModelCapabilityTool(rt)
	ctx := WithToolExecutionContext(context.Background(), &types.RunRecord{
		RunID:        "run-verify-image-blocker",
		OwnerID:      "owner",
		AgentProfile: AgentProfileSuper,
	})

	_, err := tool.Func(ctx, json.RawMessage(`{
		"provider":"fireworks",
		"model":"accounts/fireworks/models/deepseek-v4-pro",
		"prompt":"describe this image",
		"image_url":"https://example.com/screen.png"
	}`))
	if err == nil || !strings.Contains(err.Error(), "text-only") {
		t.Fatalf("error = %v, want text-only modality blocker", err)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want preflight blocker", provider.calls)
	}
}

func TestVerifyModelCapabilityUsesPolicyForTextOnlyVerifier(t *testing.T) {
	provider := &capturingModelVerifyProvider{}
	dir := t.TempDir()
	rt := New(Config{
		StorePath:       filepath.Join(dir, "runtime.db"),
		ModelPolicyPath: filepath.Join(dir, "System", "model-policy.toml"),
	}, nil, nil, provider)
	tool := newVerifyModelCapabilityTool(rt)
	ctx := WithToolExecutionContext(context.Background(), &types.RunRecord{
		RunID:        "run-verify-text-policy",
		OwnerID:      "owner",
		AgentProfile: AgentProfileSuper,
	})

	out, err := tool.Func(ctx, json.RawMessage(`{
		"role":"verifier",
		"prompt":"verify this text-only runtime evidence"
	}`))
	if err != nil {
		t.Fatalf("verify_model_capability: %v", err)
	}
	if provider.req.Provider != "deepseek" || provider.req.Model != "deepseek-v4-pro" {
		t.Fatalf("request provider/model = %s/%s", provider.req.Provider, provider.req.Model)
	}
	if strings.Contains(string(provider.req.Messages[0]), `"type":"image"`) {
		t.Fatalf("text-only verifier message unexpectedly includes image: %s", string(provider.req.Messages[0]))
	}
	if !strings.Contains(out, `"role":"verifier"`) || !strings.Contains(out, `"image_input":false`) {
		t.Fatalf("result missing text-only verifier evidence: %s", out)
	}
}

func TestVerifyModelCapabilityUsesPolicyForMiMoImage(t *testing.T) {
	provider := &capturingModelVerifyProvider{}
	dir := t.TempDir()
	rt := New(Config{
		StorePath:       filepath.Join(dir, "runtime.db"),
		ModelPolicyPath: filepath.Join(dir, "System", "model-policy.toml"),
	}, nil, nil, provider)
	tool := newVerifyModelCapabilityTool(rt)
	ctx := WithToolExecutionContext(context.Background(), &types.RunRecord{
		RunID:        "run-verify-mimo",
		OwnerID:      "owner",
		AgentProfile: AgentProfileSuper,
	})

	out, err := tool.Func(ctx, json.RawMessage(`{
		"role":"verifier_multimodal",
		"prompt":"describe this screenshot",
		"image_url":"https://example.com/screen.png"
	}`))
	if err != nil {
		t.Fatalf("verify_model_capability: %v", err)
	}
	if provider.req.Provider != "xiaomi" || provider.req.Model != "mimo-v2.5" {
		t.Fatalf("request provider/model = %s/%s", provider.req.Provider, provider.req.Model)
	}
	if len(provider.req.Messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(provider.req.Messages))
	}
	raw := string(provider.req.Messages[0])
	if !strings.Contains(raw, `"type":"image"`) || !strings.Contains(raw, `"kind":"url"`) {
		t.Fatalf("message does not include image URL block: %s", raw)
	}
	if !strings.Contains(out, `"image_input":true`) {
		t.Fatalf("result missing image evidence: %s", out)
	}
}

func TestVerifyModelCapabilityUsesDeterministicImageFixture(t *testing.T) {
	provider := &capturingModelVerifyProvider{}
	dir := t.TempDir()
	rt := New(Config{
		StorePath:       filepath.Join(dir, "runtime.db"),
		ModelPolicyPath: filepath.Join(dir, "System", "model-policy.toml"),
	}, nil, nil, provider)
	tool := newVerifyModelCapabilityTool(rt)
	ctx := WithToolExecutionContext(context.Background(), &types.RunRecord{
		RunID:        "run-verify-mimo-fixture",
		OwnerID:      "owner",
		AgentProfile: AgentProfileSuper,
	})

	out, err := tool.Func(ctx, json.RawMessage(`{
		"role":"verifier_multimodal",
		"provider":"xiaomi",
		"model":"mimo-v2.5",
		"prompt":"describe the deterministic fixture",
		"image_fixture":"red_pixel_png"
	}`))
	if err != nil {
		t.Fatalf("verify_model_capability: %v", err)
	}
	if provider.req.Provider != "xiaomi" || provider.req.Model != "mimo-v2.5" {
		t.Fatalf("request provider/model = %s/%s", provider.req.Provider, provider.req.Model)
	}
	raw := string(provider.req.Messages[0])
	if !strings.Contains(raw, verifierRedPixelPNGBase64) || !strings.Contains(raw, `"mime_type":"image/png"`) {
		t.Fatalf("message does not include deterministic base64 image fixture: %s", raw)
	}
	if !strings.Contains(out, `"image_input":true`) {
		t.Fatalf("result missing image evidence: %s", out)
	}
}

func TestVerifyModelCapabilityRejectsMalformedImageInput(t *testing.T) {
	provider := &capturingModelVerifyProvider{}
	rt := New(Config{StorePath: filepath.Join(t.TempDir(), "runtime.db")}, nil, nil, provider)
	tool := newVerifyModelCapabilityTool(rt)
	ctx := WithToolExecutionContext(context.Background(), &types.RunRecord{
		RunID:        "run-verify-bad-image",
		OwnerID:      "owner",
		AgentProfile: AgentProfileSuper,
	})

	_, err := tool.Func(ctx, json.RawMessage(`{
		"role":"verifier_multimodal",
		"provider":"xiaomi",
		"model":"mimo-v2.5",
		"prompt":"describe the bad image",
		"image_base64":"not a valid image"
	}`))
	if err == nil || !strings.Contains(err.Error(), "valid standard base64") {
		t.Fatalf("error = %v, want base64 validation blocker", err)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want preflight blocker", provider.calls)
	}
}
