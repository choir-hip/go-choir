package modelpolicy

import (
	"context"
	"encoding/json"
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
		Role:    "super",
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
		"provider":"fireworks",
		"model":"accounts/fireworks/models/deepseek-v4-flash",
		"prompt":"verify this text-only evidence"
	}`))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("provider calls = %d", provider.calls)
	}
	request := provider.request
	if request.Provider != "fireworks" || request.Model != "accounts/fireworks/models/deepseek-v4-flash" || request.MaxTokens != 0 {
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

func TestVerifyModelCapabilityUsesPolicySelections(t *testing.T) {
	provider := &capturingModelVerifyProvider{}
	manager := NewManager(ManagerConfig{
		PolicyPath: filepath.Join(t.TempDir(), "System", "model-policy.toml"),
		Provider:   provider,
	})
	tool := NewVerifyModelCapabilityTool(manager)
	output, err := tool.Func(verifierContext(), json.RawMessage(`{
		"role":"verifier_multimodal",
		"prompt":"describe this screenshot",
		"image_url":"https://example.com/screen.png"
	}`))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if provider.request.Provider != "xiaomi" || provider.request.Model != "mimo-v2.5" {
		t.Fatalf("provider request = %+v", provider.request)
	}
	message := string(provider.request.Messages[0])
	if !strings.Contains(message, `"type":"image"`) || !strings.Contains(message, `"kind":"url"`) {
		t.Fatalf("multimodal message = %s", message)
	}
	if !strings.Contains(output, `"image_input":true`) || !strings.Contains(output, `"role":"verifier_multimodal"`) {
		t.Fatalf("result = %s", output)
	}
}

func TestVerifyModelCapabilityUsesDeterministicFixture(t *testing.T) {
	provider := &capturingModelVerifyProvider{}
	tool := NewVerifyModelCapabilityTool(NewManager(ManagerConfig{Provider: provider}))
	output, err := tool.Func(verifierContext(), json.RawMessage(`{
		"role":"verifier_multimodal",
		"provider":"xiaomi",
		"model":"mimo-v2.5",
		"prompt":"describe the fixture",
		"image_fixture":"red_pixel_png"
	}`))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	message := string(provider.request.Messages[0])
	if !strings.Contains(message, verifierRedPixelPNGBase64) || !strings.Contains(message, `"mime_type":"image/png"`) || !strings.Contains(output, `"image_input":true`) {
		t.Fatalf("fixture message/result = %s / %s", message, output)
	}
}

func TestVerifyModelCapabilityRejectsBadImagesBeforeProvider(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "text-only model",
			input: `{"role":"verifier","provider":"fireworks","model":"accounts/fireworks/models/deepseek-v4-pro","prompt":"describe","image_url":"https://example.com/screen.png"}`,
			want:  "text-only",
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

func TestRegisterVerifyModelCapabilityTool(t *testing.T) {
	registry := toolregistry.NewToolRegistry()
	manager := NewManager(ManagerConfig{Provider: &capturingModelVerifyProvider{}})
	if err := RegisterVerifyModelCapabilityTool(registry, manager); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, ok := registry.Lookup("verify_model_capability"); !ok {
		t.Fatal("registered tool missing")
	}
}
