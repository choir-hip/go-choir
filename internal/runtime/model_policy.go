package runtime

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/modelcatalog"
)

const (
	runMetadataLLMProvider        = "llm_provider"
	runMetadataLLMModel           = "llm_model"
	runMetadataLLMReasoningEffort = "llm_reasoning_effort"
	runMetadataLLMMaxTokens       = "llm_max_tokens"
	runMetadataLLMPolicySource    = "llm_policy_source"
	runMetadataLLMPolicyError     = "llm_policy_error"
	runMetadataLLMPolicyOverlayID = "llm_policy_overlay_id"

	defaultModelPolicyRelativePath = "System/model-policy.toml"
	modelPolicyOverlayRelativeDir  = "System/model-policy-overlays"

	// Keep generated foreground defaults on broadly available gateway providers.
	// Per-computer policy files may still override these roles through product state.
	defaultDeepSeekProvider         = "deepseek"
	defaultFireworksProvider        = "fireworks"
	defaultXiaomiProvider           = "xiaomi"
	defaultMimoTextModel            = "mimo-v2.5"
	defaultMimoProModel             = "mimo-v2.5-pro"
	defaultConductorModel           = "deepseek-v4-flash"
	defaultSuperModel               = "deepseek-v4-pro"
	defaultResearcherVTextModel     = "deepseek-v4-flash"
	defaultFlashForegroundReasoning = "medium"
	defaultVerifierModel            = "deepseek-v4-pro"
	defaultMultimodalVerifierModel  = "mimo-v2.5"
	defaultChatGPTProvider          = "chatgpt"
	defaultTerminalFallbackModel    = "gpt-5.4-mini"
	defaultTerminalFallbackReasoning = "low"
	legacyFireworksFlashModel       = "accounts/fireworks/models/deepseek-v4-flash"
	legacyFireworksProModel         = "accounts/fireworks/models/deepseek-v4-pro"
	legacyFireworksKimiModel        = "accounts/fireworks/models/kimi-k2p6"
	modelPolicyRoleVerifier         = "verifier"
	modelPolicyRoleVerifierMulti    = "verifier_multimodal"
)

// LLMSelection is the effective provider/model/reasoning tuple used for a run.
// It contains no provider secrets; credentials remain platform/server-owned.
type LLMSelection struct {
	Provider        string `json:"provider,omitempty"`
	Model           string `json:"model,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	MaxTokens       int    `json:"max_tokens,omitempty"`
	Source          string `json:"source,omitempty"`
}

func MaxOutputTokensForSelection(sel LLMSelection) int {
	return modelcatalog.MaxOutputTokensForModel(sel.Model)
}

// MaxInteractiveOutputTokensForSelection returns the per-call generation budget
// requested by foreground agent loops. This is intentionally separate from the
// model catalog maximum: catalog limits describe capability, while request
// budgets are provider protocol choices. OpenAI-compatible chat completions
// paths behave best when ordinary agent loops omit explicit generation budgets;
// the ChatGPT Codex Responses endpoint rejects max_output_tokens, so ChatGPT
// loops also omit explicit output budgets.
func MaxInteractiveOutputTokensForSelection(sel LLMSelection, role string) int {
	provider := strings.ToLower(strings.TrimSpace(sel.Provider))
	if provider == "chatgpt" {
		return 0
	}
	if sel.MaxTokens > 0 {
		return sel.MaxTokens
	}
	if provider == defaultFireworksProvider || provider == defaultDeepSeekProvider || provider == defaultXiaomiProvider {
		return 0
	}
	return MaxOutputTokensForSelection(sel)
}

type ModelPolicy struct {
	Defaults LLMSelection
	Roles    map[string]LLMSelection
	Source   string
}

type modelPolicyOverlay struct {
	ID        string
	ExpiresAt time.Time
	Defaults  LLMSelection
	Roles     map[string]LLMSelection
	Source    string
}

func DefaultModelPolicyPath(filesRoot string) string {
	root := strings.TrimSpace(filesRoot)
	if root == "" {
		return ""
	}
	return filepath.Join(root, filepath.FromSlash(defaultModelPolicyRelativePath))
}

func defaultModelPolicyText(_ Config) string {
	return fmt.Sprintf(`# Choir model policy
# This computer-owned file maps agent roles to provider/model choices.
# Provider secrets stay server-owned; this file names models only.
# Optional max_tokens requests an explicit per-call budget. Omit it for provider
# defaults, especially OpenAI-compatible chat completions.

[defaults]
fallback_provider = %q
fallback_model = %q
reasoning = "medium"

[roles.conductor]
provider = "deepseek"
model = "deepseek-v4-flash"
reasoning = "medium"

[roles.super]
provider = "deepseek"
model = "deepseek-v4-flash"
reasoning = "medium"

[roles.vsuper]
provider = "deepseek"
model = "deepseek-v4-flash"

[roles.co-super]
provider = "deepseek"
model = "deepseek-v4-flash"

[roles.researcher]
provider = "deepseek"
model = "deepseek-v4-flash"
reasoning = "medium"

[roles.vtext]
provider = "xiaomi"
model = "mimo-v2.5"
reasoning = "medium"

[roles.processor]
provider = "xiaomi"
model = "mimo-v2.5"
reasoning = "medium"

[roles.reconciler]
provider = "deepseek"
model = "deepseek-v4-flash"
reasoning = "medium"

[roles.verifier]
provider = "deepseek"
model = "deepseek-v4-flash"
requires = ["text", "tool_use"]

[roles.verifier_multimodal]
provider = "xiaomi"
model = "mimo-v2.5"
requires = ["image", "tool_use"]
`, defaultDeepSeekProvider, defaultConductorModel)
}

func legacyGeneratedModelPolicyText(cfg Config) string {
	fallbackProvider := strings.TrimSpace(cfg.LLMProvider)
	if fallbackProvider == "" {
		fallbackProvider = "chatgpt"
	}
	fallbackModel := strings.TrimSpace(cfg.LLMModel)
	if fallbackModel == "" {
		fallbackModel = "gpt-5.5"
	}
	return fmt.Sprintf(`# Choir model policy
# This computer-owned file maps agent roles to provider/model choices.
# Provider secrets stay server-owned; this file names models only.

[defaults]
fallback_provider = %q
fallback_model = %q
reasoning = %q

[roles.conductor]
provider = "chatgpt"
model = "gpt-5.5"
reasoning = "low"

[roles.super]
provider = "chatgpt"
model = "gpt-5.5"
reasoning = "medium"

[roles.vsuper]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-pro"

[roles.co-super]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-pro"

[roles.researcher]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-flash"

[roles.vtext]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-flash"

[roles.verifier]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-pro"
requires = ["text", "tool_use"]

[roles.verifier_multimodal]
provider = "fireworks"
model = "accounts/fireworks/models/kimi-k2p6"
requires = ["image", "tool_use"]
`, fallbackProvider, fallbackModel, strings.TrimSpace(cfg.LLMReasoningEffort))
}

func fallbackModelPolicy(_ Config) ModelPolicy {
	defaults := LLMSelection{
		Provider:        defaultDeepSeekProvider,
		Model:           defaultConductorModel,
		ReasoningEffort: defaultFlashForegroundReasoning,
		Source:          "platform_fallback",
	}
	flashDeepSeek := LLMSelection{
		Provider:        defaultDeepSeekProvider,
		Model:           defaultConductorModel,
		ReasoningEffort: defaultFlashForegroundReasoning,
		Source:          "platform_fallback",
	}
	flashXiaomi := LLMSelection{
		Provider:        defaultXiaomiProvider,
		Model:           defaultMimoTextModel,
		ReasoningEffort: defaultFlashForegroundReasoning,
		Source:          "platform_fallback",
	}
	return ModelPolicy{
		Defaults: defaults,
		Roles: map[string]LLMSelection{
			AgentProfileConductor:        flashDeepSeek,
			AgentProfileSuper:            flashDeepSeek,
			AgentProfileVSuper:           {Provider: defaultDeepSeekProvider, Model: defaultConductorModel, Source: "platform_fallback"},
			AgentProfileCoSuper:          {Provider: defaultDeepSeekProvider, Model: defaultConductorModel, Source: "platform_fallback"},
			AgentProfileResearcher:       flashDeepSeek,
			AgentProfileVText:            flashXiaomi,
			AgentProfileProcessor:        flashXiaomi,
			AgentProfileReconciler:       flashDeepSeek,
			modelPolicyRoleVerifier:      {Provider: defaultDeepSeekProvider, Model: defaultConductorModel, Source: "platform_fallback"},
			modelPolicyRoleVerifierMulti: {Provider: defaultXiaomiProvider, Model: defaultMultimodalVerifierModel, Source: "platform_fallback"},
		},
		Source: "platform_fallback",
	}
}

func (rt *Runtime) ensureResolvedLLMMetadata(ctx context.Context, ownerID string, metadata map[string]any) map[string]any {
	if metadata == nil {
		metadata = make(map[string]any)
	}
	if strings.TrimSpace(metadataStringValue(metadata, runMetadataLLMProvider)) != "" &&
		strings.TrimSpace(metadataStringValue(metadata, runMetadataLLMModel)) != "" {
		return metadata
	}
	role := metadataStringValue(metadata, runMetadataAgentRole)
	if role == "" {
		role = metadataStringValue(metadata, runMetadataAgentProfile)
	}
	if role == "" {
		role = AgentProfileConductor
	}

	policy, err := rt.loadModelPolicyForMetadata(ctx, ownerID, metadata)
	if err != nil {
		metadata[runMetadataLLMPolicyError] = err.Error()
	}
	selection := policy.Resolve(role)
	if selection.Provider != "" {
		metadata[runMetadataLLMProvider] = selection.Provider
	}
	if selection.Model != "" {
		metadata[runMetadataLLMModel] = selection.Model
		metadata[runMetadataModel] = selection.Model
	}
	if selection.ReasoningEffort != "" {
		metadata[runMetadataLLMReasoningEffort] = selection.ReasoningEffort
	}
	if selection.MaxTokens > 0 {
		metadata[runMetadataLLMMaxTokens] = selection.MaxTokens
	}
	if selection.Source != "" {
		metadata[runMetadataLLMPolicySource] = selection.Source
	}
	return metadata
}

func (rt *Runtime) loadModelPolicyForMetadata(ctx context.Context, ownerID string, metadata map[string]any) (ModelPolicy, error) {
	policy, err := rt.loadModelPolicy(ctx, ownerID)
	overlayID := strings.TrimSpace(metadataStringValue(metadata, runMetadataLLMPolicyOverlayID))
	if overlayID == "" {
		return policy, err
	}
	overlay, overlayErr := rt.loadModelPolicyOverlay(ctx, ownerID, overlayID)
	if overlayErr != nil {
		if err != nil {
			return policy, fmt.Errorf("%v; model policy overlay %q ignored: %w", err, overlayID, overlayErr)
		}
		return policy, fmt.Errorf("model policy overlay %q ignored: %w", overlayID, overlayErr)
	}
	merged := applyModelPolicyOverlay(policy, overlay)
	if err != nil {
		return merged, err
	}
	return merged, nil
}

func (p ModelPolicy) Resolve(role string) LLMSelection {
	normalized := normalizeModelPolicyRole(role)
	if p.Roles != nil {
		if sel, ok := p.Roles[normalized]; ok {
			return fillSelection(sel, p.Defaults)
		}
	}
	return fillSelection(LLMSelection{}, p.Defaults)
}

func fillSelection(sel, defaults LLMSelection) LLMSelection {
	if sel.Provider == "" {
		sel.Provider = defaults.Provider
	}
	if sel.Model == "" {
		sel.Model = defaults.Model
	}
	if sel.ReasoningEffort == "" {
		sel.ReasoningEffort = defaults.ReasoningEffort
	}
	if sel.MaxTokens <= 0 {
		sel.MaxTokens = defaults.MaxTokens
	}
	if sel.Source == "" {
		if defaults.Source != "" {
			sel.Source = defaults.Source
		} else {
			sel.Source = "model_policy"
		}
	}
	return sel
}

func (rt *Runtime) loadModelPolicy(_ context.Context, ownerID string) (ModelPolicy, error) {
	path := strings.TrimSpace(rt.cfg.ModelPolicyPath)
	fallback := fallbackModelPolicy(rt.cfg)
	if path == "" {
		return fallback, nil
	}
	if err := ensureDefaultModelPolicyFile(path, rt.cfg); err != nil {
		return fallback, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fallback, fmt.Errorf("read model policy: %w", err)
	}
	policy, err := parseModelPolicy(string(raw), path)
	cacheKey := path
	if cacheKey == "" {
		cacheKey = ownerID
	}
	rt.modelPolicyMu.Lock()
	defer rt.modelPolicyMu.Unlock()
	if err != nil {
		if cached, ok := rt.modelPolicies[cacheKey]; ok {
			return cached, fmt.Errorf("model policy invalid, using previous valid policy: %w", err)
		}
		return fallback, fmt.Errorf("model policy invalid, using platform fallback: %w", err)
	}
	rt.modelPolicies[cacheKey] = policy
	return policy, nil
}

func (rt *Runtime) loadModelPolicyOverlay(_ context.Context, _ string, overlayID string) (modelPolicyOverlay, error) {
	overlayID = strings.TrimSpace(overlayID)
	if overlayID == "" {
		return modelPolicyOverlay{}, fmt.Errorf("overlay id is required")
	}
	if !isSafeModelPolicyOverlayID(overlayID) {
		return modelPolicyOverlay{}, fmt.Errorf("overlay id %q is not allowed", overlayID)
	}
	basePath := strings.TrimSpace(rt.cfg.ModelPolicyPath)
	if basePath == "" {
		return modelPolicyOverlay{}, fmt.Errorf("model policy path is not configured")
	}
	path := filepath.Join(filepath.Dir(basePath), filepath.Base(modelPolicyOverlayRelativeDir), overlayID+".toml")
	if filepath.Base(filepath.Dir(path)) != "model-policy-overlays" {
		return modelPolicyOverlay{}, fmt.Errorf("resolved overlay path is invalid")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return modelPolicyOverlay{}, fmt.Errorf("read overlay: %w", err)
	}
	overlay, err := parseModelPolicyOverlay(overlayID, string(raw), path)
	if err != nil {
		return modelPolicyOverlay{}, err
	}
	if !overlay.ExpiresAt.IsZero() && time.Now().UTC().After(overlay.ExpiresAt) {
		return modelPolicyOverlay{}, fmt.Errorf("overlay expired at %s", overlay.ExpiresAt.Format(time.RFC3339))
	}
	return overlay, nil
}

func isSafeModelPolicyOverlayID(id string) bool {
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

func ensureDefaultModelPolicyFile(path string, cfg Config) error {
	if _, err := os.Stat(path); err == nil {
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if shouldMigrateLegacyGeneratedModelPolicy(string(raw), cfg) {
			return os.WriteFile(path, []byte(defaultModelPolicyText(cfg)), 0o644)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(defaultModelPolicyText(cfg)), 0o644)
}

func parseModelPolicyOverlay(id, raw, source string) (modelPolicyOverlay, error) {
	overlay := modelPolicyOverlay{
		ID:     strings.TrimSpace(id),
		Roles:  map[string]LLMSelection{},
		Source: source,
	}

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
		key, value, ok := parseModelPolicyAssignment(line)
		if !ok {
			return modelPolicyOverlay{}, fmt.Errorf("line %d: expected key = value", lineNo)
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
					return modelPolicyOverlay{}, fmt.Errorf("line %d: invalid expires_at: %w", lineNo, err)
				}
				overlay.ExpiresAt = parsed.UTC()
			default:
				return modelPolicyOverlay{}, fmt.Errorf("line %d: unknown overlay key %q", lineNo, key)
			}
		case section == "defaults":
			applyModelPolicyValue(&overlay.Defaults, key, value)
			overlay.Defaults.Source = source
		case strings.HasPrefix(section, "roles."):
			role := normalizeModelPolicyRole(strings.TrimPrefix(section, "roles."))
			sel := overlay.Roles[role]
			applyModelPolicyValue(&sel, key, value)
			sel.Source = source
			overlay.Roles[role] = sel
		default:
			return modelPolicyOverlay{}, fmt.Errorf("line %d: unknown section %q", lineNo, section)
		}
	}
	if err := scanner.Err(); err != nil {
		return modelPolicyOverlay{}, err
	}
	if strings.TrimSpace(overlay.Defaults.Provider) == "" && strings.TrimSpace(overlay.Defaults.Model) != "" {
		return modelPolicyOverlay{}, fmt.Errorf("overlay defaults require provider when model is set")
	}
	if strings.TrimSpace(overlay.Defaults.Model) == "" && strings.TrimSpace(overlay.Defaults.Provider) != "" {
		return modelPolicyOverlay{}, fmt.Errorf("overlay defaults require model when provider is set")
	}
	for role, sel := range overlay.Roles {
		if strings.TrimSpace(sel.Provider) == "" || strings.TrimSpace(sel.Model) == "" {
			return modelPolicyOverlay{}, fmt.Errorf("overlay role %q requires provider and model", role)
		}
	}
	if isEmptySelection(overlay.Defaults) && len(overlay.Roles) == 0 {
		return modelPolicyOverlay{}, fmt.Errorf("overlay must define defaults or at least one role")
	}
	return overlay, nil
}

func applyModelPolicyOverlay(base ModelPolicy, overlay modelPolicyOverlay) ModelPolicy {
	merged := ModelPolicy{
		Defaults: base.Defaults,
		Roles:    make(map[string]LLMSelection, len(base.Roles)+len(overlay.Roles)),
		Source:   base.Source,
	}
	for role, sel := range base.Roles {
		merged.Roles[role] = sel
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
	for role, overlaySel := range overlay.Roles {
		baseSel := base.Resolve(role)
		mergedSel := fillSelection(overlaySel, baseSel)
		mergedSel.Source = overlaySource
		merged.Roles[role] = mergedSel
	}
	if len(overlay.Roles) > 0 {
		merged.Source = overlaySource
	}
	return merged
}

func isEmptySelection(sel LLMSelection) bool {
	return strings.TrimSpace(sel.Provider) == "" &&
		strings.TrimSpace(sel.Model) == "" &&
		strings.TrimSpace(sel.ReasoningEffort) == "" &&
		sel.MaxTokens <= 0
}

func shouldMigrateLegacyGeneratedModelPolicy(raw string, cfg Config) bool {
	if strings.TrimSpace(raw) == strings.TrimSpace(legacyGeneratedModelPolicyText(cfg)) {
		return true
	}
	if !strings.Contains(raw, "# Choir model policy") ||
		!strings.Contains(raw, "Provider secrets stay server-owned") {
		return false
	}
	policy, err := parseModelPolicy(raw, "legacy-generated-model-policy")
	if err != nil {
		return false
	}
	if hasLegacyChatGPTFallback(policy) {
		return true
	}
	if hasGeneratedFlashNoneForegroundPolicy(policy) {
		return true
	}
	if hasGeneratedDeepSeekPolicy(policy) {
		return true
	}
	if policyNeedsFlashDualProviderUpgrade(policy) {
		return true
	}
	conductor, ok := policy.Roles[AgentProfileConductor]
	if !ok || !isModelPolicySelection(conductor, "chatgpt", "gpt-5.5", "low") {
		return hasLegacyChatGPTForegroundPin(policy)
	}
	super, ok := policy.Roles[AgentProfileSuper]
	if !ok || !isModelPolicySelection(super, "chatgpt", "gpt-5.5", "medium") {
		return hasLegacyChatGPTForegroundPin(policy)
	}
	researcher, ok := policy.Roles[AgentProfileResearcher]
	if !ok || !isModelPolicySelection(researcher, defaultFireworksProvider, legacyFireworksFlashModel, "") {
		return false
	}
	vtext, ok := policy.Roles[AgentProfileVText]
	if !ok || !isModelPolicySelection(vtext, defaultFireworksProvider, legacyFireworksFlashModel, "") {
		return false
	}
	return true
}

func hasGeneratedDeepSeekPolicy(policy ModelPolicy) bool {
	if len(policy.Roles) != 8 {
		return false
	}
	if !isModelPolicySelection(policy.Defaults, defaultDeepSeekProvider, defaultResearcherVTextModel, "medium") {
		return false
	}
	expected := map[string]LLMSelection{
		AgentProfileConductor:        {Provider: defaultDeepSeekProvider, Model: defaultConductorModel, ReasoningEffort: "medium"},
		AgentProfileSuper:            {Provider: defaultDeepSeekProvider, Model: defaultSuperModel, ReasoningEffort: "medium"},
		AgentProfileVSuper:           {Provider: defaultDeepSeekProvider, Model: defaultSuperModel},
		AgentProfileCoSuper:          {Provider: defaultDeepSeekProvider, Model: defaultSuperModel},
		AgentProfileResearcher:       {Provider: defaultDeepSeekProvider, Model: defaultResearcherVTextModel, ReasoningEffort: "medium"},
		AgentProfileVText:            {Provider: defaultDeepSeekProvider, Model: defaultResearcherVTextModel, ReasoningEffort: "medium"},
		modelPolicyRoleVerifier:      {Provider: defaultDeepSeekProvider, Model: defaultVerifierModel},
		modelPolicyRoleVerifierMulti: {Provider: defaultXiaomiProvider, Model: defaultMultimodalVerifierModel},
	}
	for role, want := range expected {
		got, ok := policy.Roles[role]
		if !ok {
			return false
		}
		if !isModelPolicySelection(got, want.Provider, want.Model, want.ReasoningEffort) {
			return false
		}
	}
	return true
}

func hasGeneratedFlashNoneForegroundPolicy(policy ModelPolicy) bool {
	if len(policy.Roles) != 8 {
		return false
	}
	if !isModelPolicySelection(policy.Defaults, defaultFireworksProvider, legacyFireworksFlashModel, "") {
		return false
	}
	expected := map[string]LLMSelection{
		AgentProfileConductor:        {Provider: defaultFireworksProvider, Model: legacyFireworksFlashModel, ReasoningEffort: "none"},
		AgentProfileSuper:            {Provider: defaultFireworksProvider, Model: legacyFireworksProModel, ReasoningEffort: "medium"},
		AgentProfileVSuper:           {Provider: defaultFireworksProvider, Model: legacyFireworksProModel},
		AgentProfileCoSuper:          {Provider: defaultFireworksProvider, Model: legacyFireworksProModel},
		AgentProfileResearcher:       {Provider: defaultFireworksProvider, Model: legacyFireworksFlashModel, ReasoningEffort: "none"},
		AgentProfileVText:            {Provider: defaultFireworksProvider, Model: legacyFireworksFlashModel, ReasoningEffort: "none"},
		modelPolicyRoleVerifier:      {Provider: defaultFireworksProvider, Model: legacyFireworksProModel},
		modelPolicyRoleVerifierMulti: {Provider: defaultFireworksProvider, Model: legacyFireworksKimiModel},
	}
	for role, want := range expected {
		got, ok := policy.Roles[role]
		if !ok {
			return false
		}
		if !isExactModelPolicySelection(got, want.Provider, want.Model, want.ReasoningEffort) {
			return false
		}
	}
	return true
}

func isExactModelPolicySelection(sel LLMSelection, provider, model, reasoning string) bool {
	return strings.TrimSpace(sel.Provider) == provider &&
		strings.TrimSpace(sel.Model) == model &&
		strings.TrimSpace(sel.ReasoningEffort) == reasoning &&
		sel.MaxTokens == 0
}

func hasLegacyChatGPTForegroundPin(policy ModelPolicy) bool {
	conductor := policy.Roles[AgentProfileConductor]
	super := policy.Roles[AgentProfileSuper]
	return isModelPolicySelection(conductor, "chatgpt", "gpt-5.5", "") ||
		isModelPolicySelection(super, "chatgpt", "gpt-5.5", "")
}

func hasLegacyChatGPTFallback(policy ModelPolicy) bool {
	return isModelPolicySelection(policy.Defaults, "chatgpt", "gpt-5.5", "")
}

func policyNeedsFlashDualProviderUpgrade(policy ModelPolicy) bool {
	if selectionUsesFireworksRouting(policy.Defaults) || selectionUsesProTierModel(policy.Defaults) {
		return true
	}
	for _, sel := range policy.Roles {
		if selectionUsesFireworksRouting(sel) || selectionUsesProTierModel(sel) {
			return true
		}
	}
	return policyUsesSingleFlashProvider(policy)
}

func policyUsesSingleFlashProvider(policy ModelPolicy) bool {
	providers := map[string]struct{}{}
	for _, sel := range policy.Roles {
		provider := strings.TrimSpace(sel.Provider)
		if provider == "" {
			continue
		}
		if provider != defaultXiaomiProvider && provider != defaultDeepSeekProvider {
			return false
		}
		providers[provider] = struct{}{}
	}
	return len(providers) == 1
}

func selectionUsesFireworksRouting(sel LLMSelection) bool {
	provider := strings.ToLower(strings.TrimSpace(sel.Provider))
	model := strings.ToLower(strings.TrimSpace(sel.Model))
	return provider == defaultFireworksProvider || strings.Contains(model, "accounts/fireworks/")
}

func selectionUsesProTierModel(sel LLMSelection) bool {
	model := strings.ToLower(strings.TrimSpace(sel.Model))
	switch model {
	case defaultMimoProModel, defaultSuperModel, legacyFireworksProModel:
		return true
	default:
		return strings.Contains(model, "deepseek-v4-pro")
	}
}

func isModelPolicySelection(sel LLMSelection, provider, model, reasoning string) bool {
	if strings.TrimSpace(sel.Provider) != provider || strings.TrimSpace(sel.Model) != model {
		return false
	}
	if reasoning == "" {
		return true
	}
	return strings.TrimSpace(sel.ReasoningEffort) == reasoning
}

func parseModelPolicy(raw, source string) (ModelPolicy, error) {
	policy := fallbackModelPolicy(Config{})
	policy.Source = source
	policy.Defaults.Source = source
	policy.Roles = map[string]LLMSelection{}

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
		key, value, ok := parseModelPolicyAssignment(line)
		if !ok {
			return ModelPolicy{}, fmt.Errorf("line %d: expected key = value", lineNo)
		}
		switch {
		case section == "defaults":
			applyModelPolicyValue(&policy.Defaults, key, value)
			policy.Defaults.Source = source
		case strings.HasPrefix(section, "roles."):
			role := normalizeModelPolicyRole(strings.TrimPrefix(section, "roles."))
			sel := policy.Roles[role]
			applyModelPolicyValue(&sel, key, value)
			sel.Source = source
			policy.Roles[role] = sel
		default:
			return ModelPolicy{}, fmt.Errorf("line %d: unknown section %q", lineNo, section)
		}
	}
	if err := scanner.Err(); err != nil {
		return ModelPolicy{}, err
	}
	if strings.TrimSpace(policy.Defaults.Provider) == "" || strings.TrimSpace(policy.Defaults.Model) == "" {
		return ModelPolicy{}, fmt.Errorf("defaults require fallback_provider and fallback_model")
	}
	for role, sel := range policy.Roles {
		if strings.TrimSpace(sel.Provider) == "" || strings.TrimSpace(sel.Model) == "" {
			return ModelPolicy{}, fmt.Errorf("role %q requires provider and model", role)
		}
	}
	return policy, nil
}

func parseModelPolicyAssignment(line string) (string, string, bool) {
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

func applyModelPolicyValue(sel *LLMSelection, key, value string) {
	switch strings.TrimSpace(key) {
	case "provider", "fallback_provider":
		sel.Provider = strings.TrimSpace(value)
	case "model", "fallback_model":
		sel.Model = strings.TrimSpace(value)
	case "reasoning", "reasoning_effort":
		sel.ReasoningEffort = strings.TrimSpace(value)
	case "max_tokens", "max_output_tokens":
		var parsed int
		if _, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &parsed); err == nil && parsed > 0 {
			sel.MaxTokens = parsed
		}
	}
}

func normalizeModelPolicyRole(role string) string {
	switch strings.TrimSpace(strings.ToLower(role)) {
	case "cosuper", "co_super", "co-super", "cosuper_coding", "co-super-coding":
		return AgentProfileCoSuper
	case "verifier", "verifier-text", "verifier_text":
		return modelPolicyRoleVerifier
	case "verifier-multimodal", "verifier_multimodal":
		return modelPolicyRoleVerifierMulti
	default:
		return strings.TrimSpace(strings.ToLower(role))
	}
}

func ResolvedLLMConfigFromMetadata(metadata map[string]any) LLMSelection {
	if metadata == nil {
		return LLMSelection{}
	}
	return LLMSelection{
		Provider:        strings.TrimSpace(metadataStringValue(metadata, runMetadataLLMProvider)),
		Model:           strings.TrimSpace(metadataStringValue(metadata, runMetadataLLMModel)),
		ReasoningEffort: strings.TrimSpace(metadataStringValue(metadata, runMetadataLLMReasoningEffort)),
		MaxTokens:       metadataIntValue(metadata, runMetadataLLMMaxTokens),
		Source:          strings.TrimSpace(metadataStringValue(metadata, runMetadataLLMPolicySource)),
	}
}

func runtimeConfigFallbackSelection(cfg Config) LLMSelection {
	provider := strings.TrimSpace(cfg.LLMProvider)
	model := strings.TrimSpace(cfg.LLMModel)
	reasoning := strings.TrimSpace(cfg.LLMReasoningEffort)
	if provider == "" {
		provider = defaultXiaomiProvider
	}
	if model == "" {
		model = defaultMimoTextModel
	}
	return LLMSelection{
		Provider:        provider,
		Model:           model,
		ReasoningEffort: reasoning,
		Source:          "runtime_config",
	}
}

func terminalProviderFallbackSelection() LLMSelection {
	return LLMSelection{
		Provider:        defaultChatGPTProvider,
		Model:           defaultTerminalFallbackModel,
		ReasoningEffort: defaultTerminalFallbackReasoning,
		Source:          "provider_precondition_terminal_fallback",
	}
}

func providerPreconditionFallbackSelections(sel LLMSelection) []LLMSelection {
	if strings.TrimSpace(sel.Model) == "" {
		return nil
	}
	fallbacks := make([]LLMSelection, 0, 3)
	for _, candidate := range flashPreconditionFallbackSelections(sel) {
		fallbacks = appendUniqueProviderModelFallback(fallbacks, candidate)
	}
	return appendProviderPreconditionPlatformFallback(fallbacks, sel, terminalProviderFallbackSelection())
}

func flashPreconditionFallbackSelections(sel LLMSelection) []LLMSelection {
	provider := strings.ToLower(strings.TrimSpace(sel.Provider))
	model := strings.TrimSpace(sel.Model)
	reasoning := firstNonEmpty(strings.TrimSpace(sel.ReasoningEffort), defaultFlashForegroundReasoning)
	flashXiaomi := LLMSelection{
		Provider:        defaultXiaomiProvider,
		Model:           defaultMimoTextModel,
		ReasoningEffort: reasoning,
		Source:          "provider_precondition_fallback",
	}
	flashDeepSeek := LLMSelection{
		Provider:        defaultDeepSeekProvider,
		Model:           defaultConductorModel,
		ReasoningEffort: reasoning,
		Source:          "provider_precondition_fallback",
	}
	switch {
	case provider == defaultFireworksProvider && model == legacyFireworksFlashModel:
		return []LLMSelection{flashXiaomi, flashDeepSeek}
	case provider == defaultFireworksProvider && model == legacyFireworksProModel:
		return []LLMSelection{flashDeepSeek, flashXiaomi}
	case provider == defaultDeepSeekProvider && (model == defaultConductorModel || model == defaultSuperModel):
		return []LLMSelection{flashXiaomi}
	case provider == defaultXiaomiProvider && (model == defaultMimoTextModel || model == defaultMimoProModel):
		return []LLMSelection{flashDeepSeek}
	default:
		return nil
	}
}

func appendUniqueProviderModelFallback(fallbacks []LLMSelection, candidate LLMSelection) []LLMSelection {
	for _, fallback := range fallbacks {
		if sameProviderModelSelection(fallback, candidate) {
			return fallbacks
		}
	}
	return append(fallbacks, candidate)
}

func appendProviderPreconditionPlatformFallback(fallbacks []LLMSelection, active, platformFallback LLMSelection) []LLMSelection {
	provider := strings.TrimSpace(platformFallback.Provider)
	model := strings.TrimSpace(platformFallback.Model)
	if provider == "" || model == "" {
		return fallbacks
	}
	source := strings.TrimSpace(platformFallback.Source)
	if source == "" {
		source = "provider_precondition_platform_fallback"
	}
	candidate := LLMSelection{
		Provider:        provider,
		Model:           model,
		ReasoningEffort: strings.TrimSpace(platformFallback.ReasoningEffort),
		MaxTokens:       platformFallback.MaxTokens,
		Source:          source,
	}
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

func sameProviderModelSelection(a, b LLMSelection) bool {
	return strings.TrimSpace(a.Provider) == strings.TrimSpace(b.Provider) &&
		strings.TrimSpace(a.Model) == strings.TrimSpace(b.Model)
}
