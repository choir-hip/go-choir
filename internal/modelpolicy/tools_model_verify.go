package modelpolicy

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
	// ImageRef names an image in this computer's image store
	// (artifact:sha256:<hex>), e.g. a screenshot a desk stored earlier.
	ImageRef string `json:"image_ref,omitempty"`
}

// ImageStore is the computer's content-addressed image store
// (imageblob.Store). With one configured, an inline image is stored first
// and the model call carries its ref, so the bytes stay out of transcripts,
// events and responses.
type ImageStore interface {
	Put(data []byte) (string, string, error)
	Get(ref string) ([]byte, string, error)
}

const verifierRedPixelPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR42mP8z8AARQAHAQH/kX7rAAAAAElFTkSuQmCC"

// NewVerifyModelCapabilityTool constructs the provider-backed model verifier.
func NewVerifyModelCapabilityTool(manager *Manager) toolregistry.Tool {
	return toolregistry.Tool{
		Name:        "verify_model_capability",
		Description: "Run a bounded provider-backed verification prompt through a selected role/model policy, optionally with one image, and return evidence without mutating product state.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"role":             map[string]any{"type": "string", "description": "Required model-policy role to resolve, for example verifier, verifier_multimodal, researcher, super, co-super, or texture."},
			"provider":         map[string]any{"type": "string", "description": "Optional explicit provider override from the configured model catalog."},
			"model":            map[string]any{"type": "string", "description": "Optional explicit model override from the configured model catalog."},
			"reasoning_effort": map[string]any{"type": "string", "description": "Optional provider reasoning setting."},
			"prompt":           map[string]any{"type": "string"},
			"image_url":        map[string]any{"type": "string", "description": "Optional absolute http(s) image URL for multimodal verification."},
			"image_base64":     map[string]any{"type": "string", "description": "Optional valid base64 image data for multimodal verification."},
			"image_mime_type":  map[string]any{"type": "string", "description": "MIME type for image_base64, default image/png."},
			"image_fixture":    map[string]any{"type": "string", "description": "Optional deterministic image fixture for capability probes. Use \"red_pixel_png\" when the caller needs a known valid tiny PNG without supplying image_base64."},
			"image_ref":        map[string]any{"type": "string", "description": "Optional artifact:sha256:<hex> ref of an image in this computer's image store."},
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

// ModelCallError is a verification whose model call failed, as opposed to
// a request refused before the call.
type ModelCallError struct{ err error }

func (e *ModelCallError) Error() string { return e.err.Error() }
func (e *ModelCallError) Unwrap() error { return e.err }

// VerifyForOwner runs one bounded verification call for an owner-facing
// route (POST /api/model-policy/verify) with the same rules as the tool.
func (m *Manager) VerifyForOwner(ctx context.Context, ownerID string, raw json.RawMessage) (string, error) {
	var input verifyModelCapabilityArgs
	if err := json.Unmarshal(raw, &input); err != nil {
		return "", fmt.Errorf("decode verify request: %w", err)
	}
	execution := toolregistry.ExecutionContextFrom(ctx)
	execution.OwnerID = ownerID
	return m.verifyModelCapability(toolregistry.WithExecutionContext(ctx, execution), input)
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
		role = agentprofile.Management
	}
	normalized, err := normalizeVerifyImageInput(input)
	if err != nil {
		return "", err
	}
	hasImage := strings.TrimSpace(normalized.ImageURL) != "" || strings.TrimSpace(normalized.ImageBase64) != "" || strings.TrimSpace(normalized.ImageRef) != ""
	selection, policySource, err := m.resolveToolModelSelection(ctx, execution.OwnerID, role, input, hasImage)
	if err != nil {
		return "", err
	}
	if hasImage {
		if refusal := modelcatalog.ImageRefusal(selection.Model); refusal != "" {
			return "", fmt.Errorf("image input requires a multimodal model: %s", refusal)
		}
	}
	normalized, err = m.storeVerifyImage(normalized)
	if err != nil {
		return "", err
	}
	messages := []json.RawMessage{buildVerificationUserMessage(prompt, normalized)}
	maxTokens := provideriface.MaxInteractiveOutputTokensForSelection(selection, role)
	response, err := toolregistry.AsToolLoopProvider(m.config.Provider).CallWithTools(ctx, provideriface.ToolLoopRequest{
		Provider:        selection.Provider,
		Model:           selection.Model,
		ReasoningEffort: selection.ReasoningEffort,
		ConversationID:  "model-verify:" + selection.Model,

		System:    "You are a Choir verifier. Answer only the verification prompt. Do not mutate state.",
		Messages:  messages,
		MaxTokens: maxTokens,
	})
	if err != nil {
		return "", &ModelCallError{err: err}
	}
	return toolregistry.ResultJSON(map[string]any{
		"status":                    "verified",
		"role":                      role,
		"provider":                  selection.Provider,
		"model":                     selection.Model,
		"reasoning_effort":          selection.ReasoningEffort,
		"policy_source":             policySource,
		"image_input":               hasImage,
		"image_ref":                 normalized.ImageRef,
		"max_tokens_requested":      maxTokens > 0,
		"stop_reason":               response.StopReason,
		"response_model":            response.Model,
		"input_tokens":              response.Usage.InputTokens,
		"output_tokens":             response.Usage.OutputTokens,
		"reasoning_content_present": strings.TrimSpace(response.ReasoningContent) != "",
		"text_excerpt":              truncateToolText(response.Text, 2000),
	})
}

func (m *Manager) resolveToolModelSelection(ctx context.Context, ownerID, role string, input verifyModelCapabilityArgs, needsImage bool) (provideriface.LLMSelection, string, error) {
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
		// A pool spec is verified on the member this call is assigned. An
		// image call takes the first member that may see (FM4 of
		// problems/models-configured-text-only-2026-10-15.md): a text-only
		// pool member is skipped, not sent an image.
		var fallbacks []provideriface.LLMSelection
		selection, fallbacks, _ = Assign(selection, fmt.Sprintf("verify:%d", time.Now().UnixNano()))
		if needsImage && !modelcatalog.SupportsImage(selection.Model) {
			for _, fallback := range fallbacks {
				if modelcatalog.SupportsImage(fallback.Model) {
					selection = fallback
					break
				}
			}
		}
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
	sources := 0
	for _, v := range []string{output.ImageURL, output.ImageBase64, output.ImageRef} {
		if strings.TrimSpace(v) != "" {
			sources++
		}
	}
	if sources > 1 {
		return verifyModelCapabilityArgs{}, fmt.Errorf("provide only one image source: image_url, image_base64, image_fixture, or image_ref")
	}
	if ref := strings.TrimSpace(output.ImageRef); ref != "" && !strings.HasPrefix(ref, "artifact:sha256:") {
		return verifyModelCapabilityArgs{}, fmt.Errorf("image_ref must be an artifact:sha256:<hex> ref")
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

// storeVerifyImage moves an inline image into the image store and names it
// by ref, and checks a named ref exists. Without a store, an inline image
// stays inline and a ref is refused.
func (m *Manager) storeVerifyImage(input verifyModelCapabilityArgs) (verifyModelCapabilityArgs, error) {
	store := m.config.ImageStore
	if ref := strings.TrimSpace(input.ImageRef); ref != "" {
		if store == nil {
			return input, fmt.Errorf("image_ref needs this computer's image store, which is not configured")
		}
		if _, _, err := store.Get(ref); err != nil {
			return input, err
		}
		return input, nil
	}
	data := strings.TrimSpace(input.ImageBase64)
	if data == "" || store == nil {
		return input, nil
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return input, fmt.Errorf("image_base64 must be valid standard base64: %w", err)
	}
	ref, _, err := store.Put(raw)
	if err != nil {
		return input, err
	}
	input.ImageBase64 = ""
	input.ImageMIMEType = ""
	input.ImageRef = ref
	return input, nil
}

func buildVerificationUserMessage(prompt string, input verifyModelCapabilityArgs) json.RawMessage {
	content := []map[string]any{{"type": "text", "text": prompt}}
	if ref := strings.TrimSpace(input.ImageRef); ref != "" {
		content = append(content, map[string]any{"type": "image", "source": map[string]string{"kind": "artifact_ref", "ref": ref}})
	}
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
