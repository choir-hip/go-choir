package modelpolicy

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

const (
	MetadataProvider        = "llm_provider"
	MetadataModel           = "llm_model"
	MetadataReasoningEffort = "llm_reasoning_effort"
	MetadataMaxTokens       = "llm_max_tokens"
	MetadataPolicySource    = "llm_policy_source"
	MetadataPolicyError     = "llm_policy_error"
	MetadataPolicyOverlayID = "llm_policy_overlay_id"

	VerifierRole           = "verifier"
	MultimodalVerifierRole = "verifier_multimodal"

	modelPolicyOverlayRelativeDir = "System/model-policy-overlays"

	defaultDeepSeekProvider          = "deepseek"
	defaultFireworksProvider         = "fireworks"
	defaultXiaomiProvider            = "xiaomi"
	defaultMimoTextModel             = "mimo-v2.5"
	defaultMimoProModel              = "mimo-v2.5-pro"
	defaultConductorModel            = "deepseek-v4-flash"
	defaultSuperModel                = "deepseek-v4-pro"
	defaultFlashForegroundReasoning  = "medium"
	defaultChatGPTProvider           = "chatgpt"
	defaultChatGPTMiniModel          = "gpt-5.4-mini"
	defaultChatGPTForegroundModel    = "gpt-5.5"
	defaultTerminalFallbackModel     = "gpt-5.4-mini"
	defaultTerminalFallbackReasoning = "low"
	legacyFireworksFlashModel        = "accounts/fireworks/models/deepseek-v4-flash"
	legacyFireworksProModel          = "accounts/fireworks/models/deepseek-v4-pro"
)

// ManagerConfig supplies the computer-owned policy path and the server-owned
// provider dependencies used by verification tools.
type ManagerConfig struct {
	PolicyPath     string
	ProviderConfig provideriface.Config
	Provider       provideriface.Provider
}

// Manager owns policy loading, overlays, and the last-valid policy cache.
type Manager struct {
	config ManagerConfig

	mu        sync.Mutex
	lastValid map[string]Policy
}

// NewManager constructs an independent model-policy owner.
func NewManager(config ManagerConfig) *Manager {
	return &Manager{
		config:    config,
		lastValid: make(map[string]Policy),
	}
}

// Policy is a parsed model policy.
type Policy struct {
	Defaults provideriface.LLMSelection
	Roles    map[string]provideriface.LLMSelection
	Source   string
}

type policyOverlay struct {
	ID        string
	ExpiresAt time.Time
	Defaults  provideriface.LLMSelection
	Roles     map[string]provideriface.LLMSelection
	Source    string
}

// Load returns the current base policy. Invalid policy updates retain the last
// valid policy for the same path (or owner when no path is configured).
func (m *Manager) Load(_ context.Context, ownerID string) (Policy, error) {
	cfg := m.providerConfig()
	path := strings.TrimSpace(m.config.PolicyPath)
	fallback := fallbackPolicy(cfg)
	if path == "" {
		return fallback, nil
	}
	if err := ensureDefaultPolicyFile(path, cfg); err != nil {
		return fallback, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fallback, fmt.Errorf("read model policy: %w", err)
	}
	policy, err := parsePolicy(string(raw), path)
	cacheKey := path
	if cacheKey == "" {
		cacheKey = ownerID
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		if cached, ok := m.lastValid[cacheKey]; ok {
			return cached, fmt.Errorf("model policy invalid, using previous valid policy: %w", err)
		}
		return fallback, fmt.Errorf("model policy invalid, using platform fallback: %w", err)
	}
	m.lastValid[cacheKey] = policy
	return policy, nil
}

// Resolve loads the policy, applies an optional safe overlay, and resolves role.
func (m *Manager) Resolve(ctx context.Context, ownerID, role, overlayID string) (provideriface.LLMSelection, error) {
	policy, err := m.loadWithOverlay(ctx, ownerID, overlayID)
	return policy.Resolve(role), err
}

// EnrichMetadata resolves a selection unless provider and model are already
// present. The caller supplies the role because agent metadata belongs to the
// runtime, while overlay and resolved LLM keys belong to this package.
func (m *Manager) EnrichMetadata(ctx context.Context, ownerID, role string, metadata map[string]any) map[string]any {
	if metadata == nil {
		metadata = make(map[string]any)
	}
	if metadataString(metadata, MetadataProvider) != "" && metadataString(metadata, MetadataModel) != "" {
		return metadata
	}
	if strings.TrimSpace(role) == "" {
		role = agentprofile.Conductor
	}
	selection, err := m.Resolve(ctx, ownerID, role, metadataString(metadata, MetadataPolicyOverlayID))
	if err != nil {
		metadata[MetadataPolicyError] = err.Error()
	}
	if selection.Provider != "" {
		metadata[MetadataProvider] = selection.Provider
	}
	if selection.Model != "" {
		metadata[MetadataModel] = selection.Model
	}
	if selection.ReasoningEffort != "" {
		metadata[MetadataReasoningEffort] = selection.ReasoningEffort
	}
	if selection.MaxTokens > 0 {
		metadata[MetadataMaxTokens] = selection.MaxTokens
	}
	if selection.Source != "" {
		metadata[MetadataPolicySource] = selection.Source
	}
	return metadata
}

// Resolve returns the effective selection for role.
func (p Policy) Resolve(role string) provideriface.LLMSelection {
	normalized := NormalizeRole(role)
	if p.Roles != nil {
		if selection, ok := p.Roles[normalized]; ok {
			return fillSelection(selection, p.Defaults)
		}
	}
	return fillSelection(provideriface.LLMSelection{}, p.Defaults)
}

// NormalizeRole canonicalizes model-policy role aliases.
func NormalizeRole(role string) string {
	switch strings.TrimSpace(strings.ToLower(role)) {
	case "cosuper", "co_super", "co-super", "cosuper_coding", "co-super-coding":
		return agentprofile.CoSuper
	case "texture", "texture-agent":
		return agentprofile.Texture
	case "verifier", "verifier-text", "verifier_text":
		return VerifierRole
	case "verifier-multimodal", "verifier_multimodal":
		return MultimodalVerifierRole
	default:
		return strings.TrimSpace(strings.ToLower(role))
	}
}

// RuntimeConfigFallbackSelection resolves the legacy runtime-config fallback.
func RuntimeConfigFallbackSelection(cfg provideriface.Config) provideriface.LLMSelection {
	provider := strings.TrimSpace(cfg.LLMProvider)
	model := strings.TrimSpace(cfg.LLMModel)
	reasoning := strings.TrimSpace(cfg.LLMReasoningEffort)
	if provider == "" {
		provider = defaultXiaomiProvider
	}
	if model == "" {
		model = defaultMimoTextModel
	}
	return provideriface.LLMSelection{Provider: provider, Model: model, ReasoningEffort: reasoning, Source: "runtime_config"}
}

// TerminalProviderFallbackSelection returns the terminal provider fallback.
func TerminalProviderFallbackSelection() provideriface.LLMSelection {
	return provideriface.LLMSelection{
		Provider:        defaultChatGPTProvider,
		Model:           defaultTerminalFallbackModel,
		ReasoningEffort: defaultTerminalFallbackReasoning,
		Source:          "provider_precondition_terminal_fallback",
	}
}

// ProviderPreconditionFallbackSelections returns ordered cross-provider
// fallbacks for a selection that failed provider preconditions.
func ProviderPreconditionFallbackSelections(selection provideriface.LLMSelection) []provideriface.LLMSelection {
	if strings.TrimSpace(selection.Model) == "" {
		return nil
	}
	fallbacks := make([]provideriface.LLMSelection, 0, 3)
	for _, candidate := range flashPreconditionFallbackSelections(selection) {
		fallbacks = appendUniqueProviderModelFallback(fallbacks, candidate)
	}
	return appendProviderPreconditionPlatformFallback(fallbacks, selection, TerminalProviderFallbackSelection())
}

func (m *Manager) providerConfig() provideriface.Config {
	return m.config.ProviderConfig
}

func (m *Manager) loadWithOverlay(ctx context.Context, ownerID, overlayID string) (Policy, error) {
	policy, err := m.Load(ctx, ownerID)
	overlayID = strings.TrimSpace(overlayID)
	if overlayID == "" {
		return policy, err
	}
	overlay, overlayErr := m.loadOverlay(overlayID)
	if overlayErr != nil {
		if err != nil {
			return policy, fmt.Errorf("%v; model policy overlay %q ignored: %w", err, overlayID, overlayErr)
		}
		return policy, fmt.Errorf("model policy overlay %q ignored: %w", overlayID, overlayErr)
	}
	merged := applyOverlay(policy, overlay)
	if err != nil {
		return merged, err
	}
	return merged, nil
}

func (m *Manager) loadOverlay(overlayID string) (policyOverlay, error) {
	overlayID = strings.TrimSpace(overlayID)
	if overlayID == "" {
		return policyOverlay{}, fmt.Errorf("overlay id is required")
	}
	if !isSafeOverlayID(overlayID) {
		return policyOverlay{}, fmt.Errorf("overlay id %q is not allowed", overlayID)
	}
	basePath := strings.TrimSpace(m.config.PolicyPath)
	if basePath == "" {
		return policyOverlay{}, fmt.Errorf("model policy path is not configured")
	}
	path := filepath.Join(filepath.Dir(basePath), filepath.Base(modelPolicyOverlayRelativeDir), overlayID+".toml")
	if filepath.Base(filepath.Dir(path)) != "model-policy-overlays" {
		return policyOverlay{}, fmt.Errorf("resolved overlay path is invalid")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return policyOverlay{}, fmt.Errorf("read overlay: %w", err)
	}
	overlay, err := parseOverlay(overlayID, string(raw), path)
	if err != nil {
		return policyOverlay{}, err
	}
	if !overlay.ExpiresAt.IsZero() && time.Now().UTC().After(overlay.ExpiresAt) {
		return policyOverlay{}, fmt.Errorf("overlay expired at %s", overlay.ExpiresAt.Format(time.RFC3339))
	}
	return overlay, nil
}

func isSafeOverlayID(id string) bool {
	if id == "" || len(id) > 96 {
		return false
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func defaultPolicyText(_ provideriface.Config) string {
	return `# Choir model policy
# This computer-owned file maps agent roles to provider/model choices.
# Provider secrets stay server-owned; this file names models only.
# Optional max_tokens requests an explicit per-call budget. Omit it for provider
# defaults, especially OpenAI-compatible chat completions.

[defaults]
fallback_provider = "chatgpt"
fallback_model = "gpt-5.4-mini"
reasoning = "low"

[roles.conductor]
provider = "chatgpt"
model = "gpt-5.4-mini"
reasoning = "low"

[roles.texture]
provider = "chatgpt"
model = "gpt-5.5"
reasoning = "low"

[roles.super]
provider = "chatgpt"
model = "gpt-5.5"
reasoning = "high"

[roles.vsuper]
provider = "deepseek"
model = "deepseek-v4-flash"

[roles.co-super]
provider = "deepseek"
model = "deepseek-v4-flash"

[roles.researcher]
provider = "chatgpt"
model = "gpt-5.4-mini"
reasoning = "low"

[roles.processor]
provider = "chatgpt"
model = "gpt-5.5"
reasoning = "low"

[roles.reconciler]
provider = "chatgpt"
model = "gpt-5.5"
reasoning = "low"

[roles.verifier]
provider = "deepseek"
model = "deepseek-v4-flash"
requires = ["text", "tool_use"]

[roles.verifier_multimodal]
provider = "xiaomi"
model = "mimo-v2.5"
requires = ["image", "tool_use"]
`
}

func fallbackPolicy(_ provideriface.Config) Policy {
	defaults := provideriface.LLMSelection{Provider: defaultChatGPTProvider, Model: defaultChatGPTMiniModel, ReasoningEffort: "low", Source: "platform_fallback"}
	chatGPTMini := defaults
	chatGPTForeground := provideriface.LLMSelection{Provider: defaultChatGPTProvider, Model: defaultChatGPTForegroundModel, ReasoningEffort: "high", Source: "platform_fallback"}
	chatGPTWire := provideriface.LLMSelection{Provider: defaultChatGPTProvider, Model: defaultChatGPTForegroundModel, ReasoningEffort: "low", Source: "platform_fallback"}
	return Policy{
		Defaults: defaults,
		Roles: map[string]provideriface.LLMSelection{
			agentprofile.Conductor:  chatGPTMini,
			agentprofile.Super:      chatGPTForeground,
			agentprofile.VSuper:     {Provider: defaultDeepSeekProvider, Model: defaultConductorModel, Source: "platform_fallback"},
			agentprofile.CoSuper:    {Provider: defaultDeepSeekProvider, Model: defaultConductorModel, Source: "platform_fallback"},
			agentprofile.Researcher: chatGPTMini,
			agentprofile.Texture:    chatGPTWire,
			agentprofile.Processor:  chatGPTWire,
			agentprofile.Reconciler: chatGPTWire,
			VerifierRole:            {Provider: defaultDeepSeekProvider, Model: defaultConductorModel, Source: "platform_fallback"},
			MultimodalVerifierRole:  {Provider: defaultXiaomiProvider, Model: defaultMimoTextModel, Source: "platform_fallback"},
		},
		Source: "platform_fallback",
	}
}

func ensureDefaultPolicyFile(path string, cfg provideriface.Config) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(defaultPolicyText(cfg)), 0o644)
}

func parsePolicy(raw, source string) (Policy, error) {
	policy := fallbackPolicy(provideriface.Config{})
	policy.Source = source
	policy.Defaults.Source = source
	policy.Roles = map[string]provideriface.LLMSelection{}
	section := ""
	scanner := bufio.NewScanner(strings.NewReader(raw))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}
		key, value, ok := parseAssignment(line)
		if !ok {
			return Policy{}, fmt.Errorf("line %d: expected key = value", lineNo)
		}
		switch {
		case section == "defaults":
			applyValue(&policy.Defaults, key, value)
			policy.Defaults.Source = source
		case strings.HasPrefix(section, "roles."):
			role := NormalizeRole(strings.TrimPrefix(section, "roles."))
			selection := policy.Roles[role]
			applyValue(&selection, key, value)
			selection.Source = source
			policy.Roles[role] = selection
		default:
			return Policy{}, fmt.Errorf("line %d: unknown section %q", lineNo, section)
		}
	}
	if err := scanner.Err(); err != nil {
		return Policy{}, err
	}
	if strings.TrimSpace(policy.Defaults.Provider) == "" || strings.TrimSpace(policy.Defaults.Model) == "" {
		return Policy{}, fmt.Errorf("defaults require fallback_provider and fallback_model")
	}
	for role, selection := range policy.Roles {
		if strings.TrimSpace(selection.Provider) == "" || strings.TrimSpace(selection.Model) == "" {
			return Policy{}, fmt.Errorf("role %q requires provider and model", role)
		}
	}
	return policy, nil
}

func parseOverlay(id, raw, source string) (policyOverlay, error) {
	overlay := policyOverlay{ID: strings.TrimSpace(id), Roles: map[string]provideriface.LLMSelection{}, Source: source}
	section := ""
	scanner := bufio.NewScanner(strings.NewReader(raw))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}
		key, value, ok := parseAssignment(line)
		if !ok {
			return policyOverlay{}, fmt.Errorf("line %d: expected key = value", lineNo)
		}
		switch {
		case section == "overlay":
			switch strings.TrimSpace(key) {
			case "expires_at":
				if strings.TrimSpace(value) == "" {
					continue
				}
				parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
				if err != nil {
					return policyOverlay{}, fmt.Errorf("line %d: invalid expires_at: %w", lineNo, err)
				}
				overlay.ExpiresAt = parsed.UTC()
			default:
				return policyOverlay{}, fmt.Errorf("line %d: unknown overlay key %q", lineNo, key)
			}
		case section == "defaults":
			applyValue(&overlay.Defaults, key, value)
			overlay.Defaults.Source = source
		case strings.HasPrefix(section, "roles."):
			role := NormalizeRole(strings.TrimPrefix(section, "roles."))
			selection := overlay.Roles[role]
			applyValue(&selection, key, value)
			selection.Source = source
			overlay.Roles[role] = selection
		default:
			return policyOverlay{}, fmt.Errorf("line %d: unknown section %q", lineNo, section)
		}
	}
	if err := scanner.Err(); err != nil {
		return policyOverlay{}, err
	}
	if strings.TrimSpace(overlay.Defaults.Provider) == "" && strings.TrimSpace(overlay.Defaults.Model) != "" {
		return policyOverlay{}, fmt.Errorf("overlay defaults require provider when model is set")
	}
	if strings.TrimSpace(overlay.Defaults.Model) == "" && strings.TrimSpace(overlay.Defaults.Provider) != "" {
		return policyOverlay{}, fmt.Errorf("overlay defaults require model when provider is set")
	}
	for role, selection := range overlay.Roles {
		if strings.TrimSpace(selection.Provider) == "" || strings.TrimSpace(selection.Model) == "" {
			return policyOverlay{}, fmt.Errorf("overlay role %q requires provider and model", role)
		}
	}
	if isEmptySelection(overlay.Defaults) && len(overlay.Roles) == 0 {
		return policyOverlay{}, fmt.Errorf("overlay must define defaults or at least one role")
	}
	return overlay, nil
}

func applyOverlay(base Policy, overlay policyOverlay) Policy {
	merged := Policy{Defaults: base.Defaults, Roles: make(map[string]provideriface.LLMSelection, len(base.Roles)+len(overlay.Roles)), Source: base.Source}
	for role, selection := range base.Roles {
		merged.Roles[role] = selection
	}
	overlaySource := strings.TrimSpace(overlay.Source)
	if overlaySource == "" {
		overlaySource = "model_policy_overlay:" + overlay.ID
	}
	if !isEmptySelection(overlay.Defaults) {
		merged.Defaults = fillSelection(overlay.Defaults, base.Defaults)
		merged.Defaults.Source = overlaySource
		merged.Source = overlaySource
	}
	for role, selection := range overlay.Roles {
		mergedSelection := fillSelection(selection, base.Resolve(role))
		mergedSelection.Source = overlaySource
		merged.Roles[role] = mergedSelection
	}
	if len(overlay.Roles) > 0 {
		merged.Source = overlaySource
	}
	return merged
}

func fillSelection(selection, defaults provideriface.LLMSelection) provideriface.LLMSelection {
	if selection.Provider == "" {
		selection.Provider = defaults.Provider
	}
	if selection.Model == "" {
		selection.Model = defaults.Model
	}
	if selection.ReasoningEffort == "" {
		selection.ReasoningEffort = defaults.ReasoningEffort
	}
	if selection.MaxTokens <= 0 {
		selection.MaxTokens = defaults.MaxTokens
	}
	if selection.Source == "" {
		if defaults.Source != "" {
			selection.Source = defaults.Source
		} else {
			selection.Source = "model_policy"
		}
	}
	return selection
}

func isEmptySelection(selection provideriface.LLMSelection) bool {
	return strings.TrimSpace(selection.Provider) == "" && strings.TrimSpace(selection.Model) == "" && strings.TrimSpace(selection.ReasoningEffort) == "" && selection.MaxTokens <= 0
}

func parseAssignment(line string) (string, string, bool) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if strings.HasPrefix(value, "[") {
		return key, value, true
	}
	value = strings.Trim(value, `"`)
	return key, value, key != ""
}

func applyValue(selection *provideriface.LLMSelection, key, value string) {
	switch strings.TrimSpace(key) {
	case "provider", "fallback_provider":
		selection.Provider = strings.TrimSpace(value)
	case "model", "fallback_model":
		selection.Model = strings.TrimSpace(value)
	case "reasoning", "reasoning_effort":
		selection.ReasoningEffort = strings.TrimSpace(value)
	case "max_tokens", "max_output_tokens":
		var parsed int
		if _, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &parsed); err == nil && parsed > 0 {
			selection.MaxTokens = parsed
		}
	}
}

func metadataString(metadata map[string]any, key string) string {
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	if stringer, ok := value.(fmt.Stringer); ok {
		return strings.TrimSpace(stringer.String())
	}
	return ""
}

func flashPreconditionFallbackSelections(selection provideriface.LLMSelection) []provideriface.LLMSelection {
	provider := strings.ToLower(strings.TrimSpace(selection.Provider))
	model := strings.TrimSpace(selection.Model)
	reasoning := strings.TrimSpace(selection.ReasoningEffort)
	if reasoning == "" {
		reasoning = defaultFlashForegroundReasoning
	}
	flashXiaomi := provideriface.LLMSelection{Provider: defaultXiaomiProvider, Model: defaultMimoTextModel, ReasoningEffort: reasoning, Source: "provider_precondition_fallback"}
	flashDeepSeek := provideriface.LLMSelection{Provider: defaultDeepSeekProvider, Model: defaultConductorModel, ReasoningEffort: reasoning, Source: "provider_precondition_fallback"}
	switch {
	case provider == defaultFireworksProvider && model == legacyFireworksFlashModel:
		return []provideriface.LLMSelection{flashXiaomi, flashDeepSeek}
	case provider == defaultFireworksProvider && model == legacyFireworksProModel:
		return []provideriface.LLMSelection{flashDeepSeek, flashXiaomi}
	case provider == defaultDeepSeekProvider && (model == defaultConductorModel || model == defaultSuperModel):
		return []provideriface.LLMSelection{flashXiaomi}
	case provider == defaultXiaomiProvider && (model == defaultMimoTextModel || model == defaultMimoProModel):
		return []provideriface.LLMSelection{flashDeepSeek}
	default:
		return nil
	}
}

func appendUniqueProviderModelFallback(fallbacks []provideriface.LLMSelection, candidate provideriface.LLMSelection) []provideriface.LLMSelection {
	for _, fallback := range fallbacks {
		if sameProviderModelSelection(fallback, candidate) {
			return fallbacks
		}
	}
	return append(fallbacks, candidate)
}

func appendProviderPreconditionPlatformFallback(fallbacks []provideriface.LLMSelection, active, platformFallback provideriface.LLMSelection) []provideriface.LLMSelection {
	provider := strings.TrimSpace(platformFallback.Provider)
	model := strings.TrimSpace(platformFallback.Model)
	if provider == "" || model == "" {
		return fallbacks
	}
	source := strings.TrimSpace(platformFallback.Source)
	if source == "" {
		source = "provider_precondition_platform_fallback"
	}
	candidate := provideriface.LLMSelection{Provider: provider, Model: model, ReasoningEffort: strings.TrimSpace(platformFallback.ReasoningEffort), MaxTokens: platformFallback.MaxTokens, Source: source}
	if sameProviderModelSelection(active, candidate) {
		return fallbacks
	}
	for _, fallback := range fallbacks {
		if sameProviderModelSelection(fallback, candidate) {
			return fallbacks
		}
	}
	return append(fallbacks, candidate)
}

func sameProviderModelSelection(a, b provideriface.LLMSelection) bool {
	return strings.TrimSpace(a.Provider) == strings.TrimSpace(b.Provider) && strings.TrimSpace(a.Model) == strings.TrimSpace(b.Model)
}
