package runtime

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/modelcatalog"
)

const (
	runMetadataLLMProvider        = "llm_provider"
	runMetadataLLMModel           = "llm_model"
	runMetadataLLMReasoningEffort = "llm_reasoning_effort"
	runMetadataLLMPolicySource    = "llm_policy_source"
	runMetadataLLMPolicyError     = "llm_policy_error"

	defaultModelPolicyRelativePath = "System/model-policy.toml"

	// Keep generated foreground defaults on broadly available gateway providers.
	// Per-computer policy files may still override these roles through product state.
	defaultFireworksProvider       = "fireworks"
	defaultConductorModel          = "accounts/fireworks/models/deepseek-v4-flash"
	defaultSuperModel              = "accounts/fireworks/models/deepseek-v4-pro"
	defaultResearcherVTextModel    = "accounts/fireworks/models/deepseek-v4-flash"
	defaultMultimodalVerifierModel = "accounts/fireworks/models/kimi-k2p6"
)

// LLMSelection is the effective provider/model/reasoning tuple used for a run.
// It contains no provider secrets; credentials remain platform/server-owned.
type LLMSelection struct {
	Provider        string `json:"provider,omitempty"`
	Model           string `json:"model,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	Source          string `json:"source,omitempty"`
}

func MaxOutputTokensForSelection(sel LLMSelection) int {
	return modelcatalog.MaxOutputTokensForModel(sel.Model)
}

type ModelPolicy struct {
	Defaults LLMSelection
	Roles    map[string]LLMSelection
	Source   string
}

func DefaultModelPolicyPath(filesRoot string) string {
	root := strings.TrimSpace(filesRoot)
	if root == "" {
		return ""
	}
	return filepath.Join(root, filepath.FromSlash(defaultModelPolicyRelativePath))
}

func defaultModelPolicyText(cfg Config) string {
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
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-flash"
reasoning = "low"

[roles.super]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-pro"
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

[roles.verifier_multimodal]
provider = "fireworks"
model = "accounts/fireworks/models/kimi-k2p6"
requires = ["image", "tool_use"]
`, fallbackProvider, fallbackModel, strings.TrimSpace(cfg.LLMReasoningEffort))
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

[roles.verifier_multimodal]
provider = "fireworks"
model = "accounts/fireworks/models/kimi-k2p6"
requires = ["image", "tool_use"]
`, fallbackProvider, fallbackModel, strings.TrimSpace(cfg.LLMReasoningEffort))
}

func fallbackModelPolicy(cfg Config) ModelPolicy {
	defaults := LLMSelection{
		Provider:        strings.TrimSpace(cfg.LLMProvider),
		Model:           strings.TrimSpace(cfg.LLMModel),
		ReasoningEffort: strings.TrimSpace(cfg.LLMReasoningEffort),
		Source:          "platform_fallback",
	}
	if defaults.Provider == "" {
		defaults.Provider = "chatgpt"
	}
	if defaults.Model == "" {
		defaults.Model = "gpt-5.5"
	}
	return ModelPolicy{
		Defaults: defaults,
		Roles: map[string]LLMSelection{
			AgentProfileConductor:  {Provider: defaultFireworksProvider, Model: defaultConductorModel, ReasoningEffort: "low", Source: "platform_fallback"},
			AgentProfileSuper:      {Provider: defaultFireworksProvider, Model: defaultSuperModel, ReasoningEffort: "medium", Source: "platform_fallback"},
			AgentProfileVSuper:     {Provider: defaultFireworksProvider, Model: defaultSuperModel, Source: "platform_fallback"},
			AgentProfileCoSuper:    {Provider: defaultFireworksProvider, Model: defaultSuperModel, Source: "platform_fallback"},
			AgentProfileResearcher: {Provider: defaultFireworksProvider, Model: defaultResearcherVTextModel, Source: "platform_fallback"},
			AgentProfileVText:      {Provider: defaultFireworksProvider, Model: defaultResearcherVTextModel, Source: "platform_fallback"},
			"verifier_multimodal":  {Provider: defaultFireworksProvider, Model: defaultMultimodalVerifierModel, Source: "platform_fallback"},
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

	policy, err := rt.loadModelPolicy(ctx, ownerID)
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
	if selection.Source != "" {
		metadata[runMetadataLLMPolicySource] = selection.Source
	}
	return metadata
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

func ensureDefaultModelPolicyFile(path string, cfg Config) error {
	if _, err := os.Stat(path); err == nil {
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if string(raw) == legacyGeneratedModelPolicyText(cfg) {
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
	}
}

func normalizeModelPolicyRole(role string) string {
	switch strings.TrimSpace(strings.ToLower(role)) {
	case "cosuper", "co_super", "co-super", "cosuper_coding", "co-super-coding":
		return AgentProfileCoSuper
	case "verifier", "verifier-multimodal", "verifier_multimodal":
		return "verifier_multimodal"
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
		Source:          strings.TrimSpace(metadataStringValue(metadata, runMetadataLLMPolicySource)),
	}
}
