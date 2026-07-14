package modelpolicy

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/modelcatalog"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
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

// NewVerifyModelCapabilityTool constructs the provider-backed model verifier.
func NewVerifyModelCapabilityTool(manager *Manager) toolregistry.Tool {
	return toolregistry.Tool{
		Name:        "verify_model_capability",
		Description: "Run a bounded provider-backed verification prompt through a selected role/model policy, optionally with one image, and return evidence without mutating product state.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
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
			var input verifyModelCapabilityArgs
			if err := json.Unmarshal(raw, &input); err != nil {
				return "", fmt.Errorf("decode verify_model_capability args: %w", err)
			}
			return manager.verifyModelCapability(ctx, input)
		},
	}
}

// RegisterVerifyModelCapabilityTool registers the verifier with registry.
func RegisterVerifyModelCapabilityTool(registry *toolregistry.ToolRegistry, manager *Manager) error {
	return registry.Register(NewVerifyModelCapabilityTool(manager))
}

func (m *Manager) verifyModelCapability(ctx context.Context, input verifyModelCapabilityArgs) (string, error) {
	if m == nil || m.config.Provider == nil {
		return "", fmt.Errorf("verify_model_capability missing runtime provider")
	}
	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		return "", fmt.Errorf("prompt must not be empty")
	}
	execution := toolregistry.ExecutionContextFrom(ctx)
	role := NormalizeRole(firstNonEmpty(input.Role, execution.Role))
	if role == "" {
		role = agentprofile.Super
	}
	selection, policySource, err := m.resolveToolModelSelection(ctx, execution.OwnerID, role, input)
	if err != nil {
		return "", err
	}

	normalized, err := normalizeVerifyImageInput(input)
	if err != nil {
		return "", err
	}
	hasImage := strings.TrimSpace(normalized.ImageURL) != "" || strings.TrimSpace(normalized.ImageBase64) != ""
	if hasImage && !catalogModelSupportsModality(selection.Model, "image") {
		return "", fmt.Errorf("model %q is text-only in Choir model catalog; image input requires a multimodal model policy", selection.Model)
	}
	messages := []json.RawMessage{buildVerificationUserMessage(prompt, normalized)}
	maxTokens := provideriface.MaxInteractiveOutputTokensForSelection(selection, role)
	response, err := toolregistry.AsToolLoopProvider(m.config.Provider).CallWithTools(ctx, provideriface.ToolLoopRequest{
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
	return toolregistry.ResultJSON(map[string]any{
		"status":                    "verified",
		"role":                      role,
		"provider":                  selection.Provider,
		"model":                     selection.Model,
		"reasoning_effort":          selection.ReasoningEffort,
		"policy_source":             policySource,
		"image_input":               hasImage,
		"max_tokens_requested":      maxTokens > 0,
		"stop_reason":               response.StopReason,
		"response_model":            response.Model,
		"input_tokens":              response.Usage.InputTokens,
		"output_tokens":             response.Usage.OutputTokens,
		"reasoning_content_present": strings.TrimSpace(response.ReasoningContent) != "",
		"text_excerpt":              truncateToolText(response.Text, 2000),
	})
}

func (m *Manager) resolveToolModelSelection(ctx context.Context, ownerID, role string, input verifyModelCapabilityArgs) (provideriface.LLMSelection, string, error) {
	var selection provideriface.LLMSelection
	policySource := "explicit"
	if strings.TrimSpace(input.Provider) == "" && strings.TrimSpace(input.Model) == "" {
		policy, err := m.Load(ctx, ownerID)
		if err != nil {
			policySource = "policy_error:" + err.Error()
		} else {
			policySource = policy.Source
		}
		selection = policy.Resolve(role)
	} else {
		selection = provideriface.LLMSelection{
			Provider:        strings.TrimSpace(input.Provider),
			Model:           strings.TrimSpace(input.Model),
			ReasoningEffort: strings.TrimSpace(input.ReasoningEffort),
			Source:          "explicit",
		}
	}
	if strings.TrimSpace(selection.Model) == "" {
		return provideriface.LLMSelection{}, "", fmt.Errorf("model is required")
	}
	if strings.TrimSpace(selection.Provider) == "" {
		selection.Provider = providerForCatalogModel(selection.Model)
	}
	if strings.TrimSpace(selection.Provider) == "" {
		return provideriface.LLMSelection{}, "", fmt.Errorf("provider is required for model %q", selection.Model)
	}
	if strings.TrimSpace(input.Provider) != "" {
		selection.Provider = strings.TrimSpace(input.Provider)
	}
	if strings.TrimSpace(input.Model) != "" {
		selection.Model = strings.TrimSpace(input.Model)
	}
	if strings.TrimSpace(input.ReasoningEffort) != "" {
		selection.ReasoningEffort = strings.TrimSpace(input.ReasoningEffort)
	}
	return selection, policySource, nil
}

func normalizeVerifyImageInput(input verifyModelCapabilityArgs) (verifyModelCapabilityArgs, error) {
	output := input
	fixture := strings.TrimSpace(input.ImageFixture)
	if fixture != "" {
		if strings.TrimSpace(input.ImageBase64) != "" || strings.TrimSpace(input.ImageURL) != "" {
			return verifyModelCapabilityArgs{}, fmt.Errorf("image_fixture cannot be combined with image_base64 or image_url")
		}
		switch fixture {
		case "red_pixel_png":
			output.ImageBase64 = verifierRedPixelPNGBase64
			output.ImageMIMEType = "image/png"
		default:
			return verifyModelCapabilityArgs{}, fmt.Errorf("unsupported image_fixture %q", fixture)
		}
	}
	if strings.TrimSpace(output.ImageURL) != "" && strings.TrimSpace(output.ImageBase64) != "" {
		return verifyModelCapabilityArgs{}, fmt.Errorf("provide only one image source: image_url, image_base64, or image_fixture")
	}
	if url := strings.TrimSpace(output.ImageURL); url != "" && !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return verifyModelCapabilityArgs{}, fmt.Errorf("image_url must be an absolute http(s) URL")
	}
	if data := strings.TrimSpace(output.ImageBase64); data != "" {
		if _, err := base64.StdEncoding.DecodeString(data); err != nil {
			return verifyModelCapabilityArgs{}, fmt.Errorf("image_base64 must be valid standard base64: %w", err)
		}
	}
	return output, nil
}

func buildVerificationUserMessage(prompt string, input verifyModelCapabilityArgs) json.RawMessage {
	content := []map[string]any{{"type": "text", "text": prompt}}
	if url := strings.TrimSpace(input.ImageURL); url != "" {
		content = append(content, map[string]any{"type": "image", "source": map[string]string{"kind": "url", "url": url}})
	}
	if data := strings.TrimSpace(input.ImageBase64); data != "" {
		mimeType := strings.TrimSpace(input.ImageMIMEType)
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
	message, _ := json.Marshal(map[string]any{"role": "user", "content": content})
	return message
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
	if modality == "image" && (strings.HasPrefix(modelID, "gpt-5.5") || strings.HasPrefix(modelID, "gpt-5.4")) {
		return true
	}
	return modality == "text"
}

func truncateToolText(text string, max int) string {
	text = strings.TrimSpace(text)
	if max <= 0 || len(text) <= max {
		return text
	}
	return text[:max] + fmt.Sprintf("\n...[truncated %d bytes]", len(text)-max)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
