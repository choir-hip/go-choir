package modelpolicy

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
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
	// ImageRefs names a batch of stored images for one call, each sent after
	// a "Image N:" label in order (the judge's per-shot review,
	// docs/problems/judge-checks-only-named-defects-2026-11-04.md).
	ImageRefs []string `json:"image_refs,omitempty"`
}

// MaxVerifyImageRefs bounds image_refs: the gateway admits 20 images per
// request (provider.MaxImagesPerRequest).
const MaxVerifyImageRefs = 20

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
		Description: "Run a bounded provider-backed verification prompt through a selected role/model policy, optionally with one image or a batch of stored images (image_refs), and return evidence without mutating product state.",
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
			"image_refs":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional batch of up to 20 artifact:sha256:<hex> refs, sent in order, each labelled \"Image N:\"."},
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

// VerifyOutcome is one verification call's answer and cost.
type VerifyOutcome struct {
	Role, Provider, Model, ReasoningEffort, PolicySource string
	ResponseModel, StopReason, Text                      string
	ImageInput, MaxTokensRequested, ReasoningPresent     bool
	ImageRef                                             string
	InputTokens, OutputTokens                            int
}

// VerifyImages runs one bounded verification call for the platform, as
// ownerID, over a batch of stored images (refs in order, labelled "Image N:"),
// and returns the model's whole answer. The per-shot review of a frozen
// candidate calls it (docs/problems/judge-checks-only-named-defects-2026-11-04.md).
func (m *Manager) VerifyImages(ctx context.Context, ownerID, role, prompt string, refs []string) (VerifyOutcome, error) {
	execution := toolregistry.ExecutionContextFrom(ctx)
	execution.OwnerID = ownerID
	return m.runVerify(toolregistry.WithExecutionContext(ctx, execution), verifyModelCapabilityArgs{Role: role, Prompt: prompt, ImageRefs: refs})
}

func (m *Manager) verifyModelCapability(ctx context.Context, input verifyModelCapabilityArgs) (string, error) {
	out, err := m.runVerify(ctx, input)
	if err != nil {
		return "", err
	}
	return toolregistry.ResultJSON(map[string]any{
		"status":                    "verified",
		"role":                      out.Role,
		"provider":                  out.Provider,
		"model":                     out.Model,
		"reasoning_effort":          out.ReasoningEffort,
		"policy_source":             out.PolicySource,
		"image_input":               out.ImageInput,
		"image_ref":                 out.ImageRef,
		"max_tokens_requested":      out.MaxTokensRequested,
		"stop_reason":               out.StopReason,
		"response_model":            out.ResponseModel,
		"input_tokens":              out.InputTokens,
		"output_tokens":             out.OutputTokens,
		"reasoning_content_present": out.ReasoningPresent,
		"text_excerpt":              truncateToolText(out.Text, 2000),
	})
}

// verifyImageRefusal decides whether a model may not receive an image. It is
// a variable only so a test can choose which pool member is text-only.
var verifyImageRefusal = modelcatalog.ImageRefusal

func (m *Manager) runVerify(ctx context.Context, input verifyModelCapabilityArgs) (VerifyOutcome, error) {
	if m == nil || m.config.Provider == nil {
		return VerifyOutcome{}, fmt.Errorf("verify_model_capability missing runtime provider")
	}
	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		return VerifyOutcome{}, fmt.Errorf("prompt must not be empty")
	}
	execution := toolregistry.ExecutionContextFrom(ctx)
	role := NormalizeRole(firstNonEmpty(input.Role, execution.Role))
	if role == "" {
		role = deskmanifest.AdmitterName()
	}
	normalized, err := normalizeVerifyImageInput(input)
	if err != nil {
		return VerifyOutcome{}, err
	}
	hasImage := strings.TrimSpace(normalized.ImageURL) != "" || strings.TrimSpace(normalized.ImageBase64) != "" || strings.TrimSpace(normalized.ImageRef) != "" || len(normalized.ImageRefs) > 0
	selection, fallbacks, policySource, err := m.resolveToolModelSelection(ctx, execution.OwnerID, role, input, hasImage)
	if err != nil {
		return VerifyOutcome{}, err
	}
	if hasImage {
		if refusal := verifyImageRefusal(selection.Model); refusal != "" {
			return VerifyOutcome{}, fmt.Errorf("image input requires a multimodal model: %s", refusal)
		}
	}
	normalized, err = m.storeVerifyImage(normalized)
	if err != nil {
		return VerifyOutcome{}, err
	}
	messages := []json.RawMessage{buildVerificationUserMessage(prompt, normalized)}
	loop := toolregistry.AsToolLoopProvider(m.config.Provider)
	// A run fails over through its pool's order when a member answers
	// circuit open, quota, a rate limit or a transport failure
	// (toolregistry.IsModelFailoverError). The verify call does the same: it
	// stops at the first success, skips members that may not see an image,
	// and records the member that answered. A pinned model and a NoFailover
	// pool have no fallbacks, so they are tried once.
	var tried []string
	var lastErr error
	called := 0
	for i, candidate := range append([]provideriface.LLMSelection{selection}, fallbacks...) {
		if i > 0 {
			if hasImage {
				if refusal := verifyImageRefusal(candidate.Model); refusal != "" {
					tried = append(tried, fmt.Sprintf("%s/%s skipped: %s", candidate.Provider, candidate.Model, refusal))
					continue
				}
			}
			// A caller's effort override applies to every member, as a run
			// stamps the primary's effort on its fallbacks.
			if candidate.ReasoningEffort == "" || strings.TrimSpace(input.ReasoningEffort) != "" {
				candidate.ReasoningEffort = selection.ReasoningEffort
			}
		}
		called++
		maxTokens := provideriface.MaxInteractiveOutputTokensForSelection(candidate, role)
		response, err := loop.CallWithTools(ctx, provideriface.ToolLoopRequest{
			Provider:        candidate.Provider,
			Model:           candidate.Model,
			ReasoningEffort: candidate.ReasoningEffort,
			ConversationID:  "model-verify:" + candidate.Model,

			System:    "You are a Choir verifier. Answer only the verification prompt. Do not mutate state.",
			Messages:  messages,
			MaxTokens: maxTokens,
		})
		if err != nil {
			lastErr = err
			tried = append(tried, fmt.Sprintf("%s/%s failed: %v", candidate.Provider, candidate.Model, err))
			// The caller's own deadline or cancel is not a member's failure:
			// failing over on a dead context only repeats it.
			if !toolregistry.IsModelFailoverError(err) || ctx.Err() != nil {
				break
			}
			continue
		}
		return VerifyOutcome{
			Role: role, Provider: candidate.Provider, Model: candidate.Model, ReasoningEffort: candidate.ReasoningEffort,
			PolicySource: policySource, ImageInput: hasImage, ImageRef: normalized.ImageRef, MaxTokensRequested: maxTokens > 0,
			StopReason: response.StopReason, ResponseModel: response.Model, Text: response.Text,
			InputTokens: response.Usage.InputTokens, OutputTokens: response.Usage.OutputTokens,
			ReasoningPresent: strings.TrimSpace(response.ReasoningContent) != "",
		}, nil
	}
	if called == 1 && len(tried) == 1 {
		return VerifyOutcome{}, &ModelCallError{err: lastErr}
	}
	return VerifyOutcome{}, &ModelCallError{err: fmt.Errorf("verify call failed on %d pool members: %s: %w", called, strings.Join(tried, "; "), lastErr)}
}

func (m *Manager) resolveToolModelSelection(ctx context.Context, ownerID, role string, input verifyModelCapabilityArgs, needsImage bool) (provideriface.LLMSelection, []provideriface.LLMSelection, string, error) {
	var selection provideriface.LLMSelection
	var poolFallbacks []provideriface.LLMSelection
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
		selection, fallbacks, _ = policy.Assign(selection, fmt.Sprintf("verify:%d", time.Now().UnixNano()))
		poolFallbacks = fallbacks
		if needsImage && verifyImageRefusal(selection.Model) != "" {
			for i, fallback := range fallbacks {
				if verifyImageRefusal(fallback.Model) == "" {
					selection = fallback
					poolFallbacks = fallbacks[i+1:]
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
		return provideriface.LLMSelection{}, nil, "", fmt.Errorf("model is required")
	}
	if strings.TrimSpace(selection.Provider) == "" {
		selection.Provider = providerForCatalogModel(selection.Model)
	}
	if strings.TrimSpace(selection.Provider) == "" {
		return provideriface.LLMSelection{}, nil, "", fmt.Errorf("provider is required for model %q", selection.Model)
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
	return selection, poolFallbacks, policySource, nil
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
	if len(output.ImageRefs) > 0 {
		sources++
	}
	if sources > 1 {
		return verifyModelCapabilityArgs{}, fmt.Errorf("provide only one image source: image_url, image_base64, image_fixture, image_ref, or image_refs")
	}
	if len(output.ImageRefs) > MaxVerifyImageRefs {
		return verifyModelCapabilityArgs{}, fmt.Errorf("image_refs takes at most %d images", MaxVerifyImageRefs)
	}
	refs := make([]string, 0, len(output.ImageRefs))
	for _, ref := range output.ImageRefs {
		ref = strings.TrimSpace(ref)
		if !strings.HasPrefix(ref, "artifact:sha256:") {
			return verifyModelCapabilityArgs{}, fmt.Errorf("each image_refs entry must be an artifact:sha256:<hex> ref")
		}
		refs = append(refs, ref)
	}
	output.ImageRefs = refs
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
	if len(input.ImageRefs) > 0 {
		if store == nil {
			return input, fmt.Errorf("image_refs needs this computer's image store, which is not configured")
		}
		for _, ref := range input.ImageRefs {
			if _, _, err := store.Get(ref); err != nil {
				return input, err
			}
		}
		return input, nil
	}
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
	for i, ref := range input.ImageRefs {
		content = append(content,
			map[string]any{"type": "text", "text": fmt.Sprintf("Image %d:", i+1)},
			map[string]any{"type": "image", "source": map[string]string{"kind": "artifact_ref", "ref": ref}})
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
