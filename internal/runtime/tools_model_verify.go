package runtime

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/modelcatalog"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

type verifyModelCapabilityArgs struct {
	Role            string `json:"role,omitempty"`
	Provider        string `json:"provider,omitempty"`
	Model           string `json:"model,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	Prompt          string `json:"prompt"`
	ImageURL        string `json:"image_url,omitempty"`
	ImageBase64     string `json:"image_base64,omitempty"`
	ImageMIMEType   string `json:"image_mime_type,omitempty"`
	ImageFixture    string `json:"image_fixture,omitempty"`
}

const verifierRedPixelPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR42mP8z8AARQAHAQH/kX7rAAAAAElFTkSuQmCC"

func newVerifyModelCapabilityTool(rt *Runtime) Tool {
	return Tool{
		Name:        "verify_model_capability",
		Description: "Run a bounded provider-backed verification prompt through a selected role/model policy, optionally with one image, and return evidence without mutating product state.",
		Parameters: jsonSchemaObject(map[string]any{
			"role":             map[string]any{"type": "string", "description": "Required model-policy role to resolve, for example verifier, verifier_multimodal, researcher, super, vsuper, co-super, or texture."},
			"provider":         map[string]any{"type": "string", "description": "Optional explicit provider override from the configured model catalog."},
			"model":            map[string]any{"type": "string", "description": "Optional explicit model override from the configured model catalog."},
			"reasoning_effort": map[string]any{"type": "string", "description": "Optional provider reasoning setting."},
			"prompt":           map[string]any{"type": "string"},
			"image_url":        map[string]any{"type": "string", "description": "Optional absolute http(s) image URL for multimodal verification."},
			"image_base64":     map[string]any{"type": "string", "description": "Optional valid base64 image data for multimodal verification."},
			"image_mime_type":  map[string]any{"type": "string", "description": "MIME type for image_base64, default image/png."},
			"image_fixture":    map[string]any{"type": "string", "description": "Optional deterministic image fixture for capability probes. Use \"red_pixel_png\" when the caller needs a known valid tiny PNG without supplying image_base64."},
		}, []string{"role", "prompt"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in verifyModelCapabilityArgs
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode verify_model_capability args: %w", err)
			}
			return rt.verifyModelCapability(ctx, in)
		},
	}
}

func (rt *Runtime) verifyModelCapability(ctx context.Context, in verifyModelCapabilityArgs) (string, error) {
	if rt == nil || rt.provider == nil {
		return "", fmt.Errorf("verify_model_capability missing runtime provider")
	}
	prompt := strings.TrimSpace(in.Prompt)
	if prompt == "" {
		return "", fmt.Errorf("prompt must not be empty")
	}
	role := normalizeModelPolicyRole(nonEmpty(in.Role, stringFromToolContext(ctx, toolCtxRole)))
	if role == "" {
		role = AgentProfileSuper
	}
	ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
	selection, policySource, err := rt.resolveToolModelSelection(ctx, ownerID, role, in)
	if err != nil {
		return "", err
	}

	normalized, err := normalizeVerifyImageInput(in)
	if err != nil {
		return "", err
	}
	hasImage := strings.TrimSpace(normalized.ImageURL) != "" || strings.TrimSpace(normalized.ImageBase64) != ""
	if hasImage && !catalogModelSupportsModality(selection.Model, "image") {
		return "", fmt.Errorf("model %q is text-only in Choir model catalog; image input requires a multimodal model policy", selection.Model)
	}
	messages := []json.RawMessage{buildVerificationUserMessage(prompt, normalized)}
	maxTokens := provideriface.MaxInteractiveOutputTokensForSelection(selection, role)
	resp, err := asToolLoopProvider(rt.provider).CallWithTools(ctx, ToolLoopRequest{
		Provider:        selection.Provider,
		Model:           selection.Model,
		ReasoningEffort: selection.ReasoningEffort,
		System:          "You are a Choir verifier. Answer only the verification prompt. Do not mutate state.",
		Messages:        messages,
		MaxTokens:       maxTokens,
	})
	if err != nil {
		return "", err
	}
	return toolResultJSON(map[string]any{
		"status":                    "verified",
		"role":                      role,
		"provider":                  selection.Provider,
		"model":                     selection.Model,
		"reasoning_effort":          selection.ReasoningEffort,
		"policy_source":             policySource,
		"image_input":               hasImage,
		"max_tokens_requested":      maxTokens > 0,
		"stop_reason":               resp.StopReason,
		"response_model":            resp.Model,
		"input_tokens":              resp.Usage.InputTokens,
		"output_tokens":             resp.Usage.OutputTokens,
		"reasoning_content_present": strings.TrimSpace(resp.ReasoningContent) != "",
		"text_excerpt":              truncateToolText(resp.Text, 2000),
	})
}

func normalizeVerifyImageInput(in verifyModelCapabilityArgs) (verifyModelCapabilityArgs, error) {
	out := in
	fixture := strings.TrimSpace(in.ImageFixture)
	if fixture != "" {
		if strings.TrimSpace(in.ImageBase64) != "" || strings.TrimSpace(in.ImageURL) != "" {
			return verifyModelCapabilityArgs{}, fmt.Errorf("image_fixture cannot be combined with image_base64 or image_url")
		}
		switch fixture {
		case "red_pixel_png":
			out.ImageBase64 = verifierRedPixelPNGBase64
			out.ImageMIMEType = "image/png"
		default:
			return verifyModelCapabilityArgs{}, fmt.Errorf("unsupported image_fixture %q", fixture)
		}
	}
	if strings.TrimSpace(out.ImageURL) != "" && strings.TrimSpace(out.ImageBase64) != "" {
		return verifyModelCapabilityArgs{}, fmt.Errorf("provide only one image source: image_url, image_base64, or image_fixture")
	}
	if url := strings.TrimSpace(out.ImageURL); url != "" && !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return verifyModelCapabilityArgs{}, fmt.Errorf("image_url must be an absolute http(s) URL")
	}
	if data := strings.TrimSpace(out.ImageBase64); data != "" {
		if _, err := base64.StdEncoding.DecodeString(data); err != nil {
			return verifyModelCapabilityArgs{}, fmt.Errorf("image_base64 must be valid standard base64: %w", err)
		}
	}
	return out, nil
}

func (rt *Runtime) resolveToolModelSelection(ctx context.Context, ownerID, role string, in verifyModelCapabilityArgs) (LLMSelection, string, error) {
	var selection LLMSelection
	policySource := "explicit"
	if strings.TrimSpace(in.Provider) == "" && strings.TrimSpace(in.Model) == "" {
		policy, err := rt.loadModelPolicy(ctx, ownerID)
		if err != nil {
			policySource = "policy_error:" + err.Error()
		} else {
			policySource = policy.Source
		}
		selection = policy.Resolve(role)
	} else {
		selection = LLMSelection{
			Provider:        strings.TrimSpace(in.Provider),
			Model:           strings.TrimSpace(in.Model),
			ReasoningEffort: strings.TrimSpace(in.ReasoningEffort),
			Source:          "explicit",
		}
	}
	if strings.TrimSpace(selection.Model) == "" {
		return LLMSelection{}, "", fmt.Errorf("model is required")
	}
	if strings.TrimSpace(selection.Provider) == "" {
		if provider := providerForCatalogModel(selection.Model); provider != "" {
			selection.Provider = provider
		}
	}
	if strings.TrimSpace(selection.Provider) == "" {
		return LLMSelection{}, "", fmt.Errorf("provider is required for model %q", selection.Model)
	}
	if strings.TrimSpace(in.Provider) != "" {
		selection.Provider = strings.TrimSpace(in.Provider)
	}
	if strings.TrimSpace(in.Model) != "" {
		selection.Model = strings.TrimSpace(in.Model)
	}
	if strings.TrimSpace(in.ReasoningEffort) != "" {
		selection.ReasoningEffort = strings.TrimSpace(in.ReasoningEffort)
	}
	return selection, policySource, nil
}

func buildVerificationUserMessage(prompt string, in verifyModelCapabilityArgs) json.RawMessage {
	content := []map[string]any{{"type": "text", "text": prompt}}
	if url := strings.TrimSpace(in.ImageURL); url != "" {
		content = append(content, map[string]any{
			"type":   "image",
			"source": map[string]string{"kind": "url", "url": url},
		})
	}
	if data := strings.TrimSpace(in.ImageBase64); data != "" {
		mimeType := strings.TrimSpace(in.ImageMIMEType)
		if mimeType == "" {
			mimeType = "image/png"
		}
		content = append(content, map[string]any{
			"type": "image",
			"source": map[string]string{
				"kind":      "base64",
				"mime_type": mimeType,
				"data":      data,
			},
		})
	}
	msg, _ := json.Marshal(map[string]any{"role": "user", "content": content})
	return msg
}

func providerForCatalogModel(modelID string) string {
	for _, info := range modelcatalog.SupportedModels() {
		if info.ID == strings.TrimSpace(modelID) {
			return info.Provider
		}
	}
	return ""
}

func catalogModelSupportsModality(modelID, modality string) bool {
	modelID = strings.TrimSpace(modelID)
	modality = strings.TrimSpace(modality)
	for _, info := range modelcatalog.SupportedModels() {
		if info.ID != modelID {
			continue
		}
		for _, candidate := range info.Modalities {
			if candidate == modality {
				return true
			}
		}
		return false
	}
	switch {
	case modality == "image" && (strings.HasPrefix(modelID, "gpt-5.5") || strings.HasPrefix(modelID, "gpt-5.4")):
		return true
	default:
		return modality == "text"
	}
}

func truncateToolText(text string, max int) string {
	text = strings.TrimSpace(text)
	if max <= 0 || len(text) <= max {
		return text
	}
	return text[:max] + fmt.Sprintf("\n...[truncated %d bytes]", len(text)-max)
}
