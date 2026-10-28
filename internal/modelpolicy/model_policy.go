package modelpolicy

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/modelcatalog"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/vocabmigrate"
)

const (
	MetadataProvider        = "llm_provider"
	MetadataModel           = "llm_model"
	MetadataReasoningEffort = "llm_reasoning_effort"
	MetadataMaxTokens       = "llm_max_tokens"
	MetadataPolicySource    = "llm_policy_source"
	MetadataPolicyError     = "llm_policy_error"
	MetadataPolicyOverlayID = "llm_policy_overlay_id"
	// MetadataSpec is what was asked for (a pool or a model);
	// MetadataAssignment says how the model was chosen (hash, explicit,
	// retired_remapped); MetadataFallbacks lists the failover order as
	// provider/model, comma-separated; MetadataThread is the hash key.
	MetadataSpec       = "llm_spec"
	MetadataAssignment = "llm_assignment"
	MetadataFallbacks  = "llm_fallbacks"
	MetadataThread     = "llm_thread"

	VerifierRole           = "verifier"
	MultimodalVerifierRole = "verifier_multimodal"

	modelPolicyOverlayRelativeDir = "System/model-policy-overlays"

	defaultChatGPTProvider  = "chatgpt"
	defaultChatGPTMiniModel = "gpt-6-luna"

	// policyVersionLine marks a policy file written from the current
	// platform default. A file without it is rewritten at load; a file
	// carrying ownerEditedLine is the owner's and is kept.
	policyVersionLine = "# platform-policy-version: 2"
	ownerEditedLine   = "# owner-edited"
)

// catalogPools is the pool table Assign and the parser read: modelcatalog.Pools
// in production, a seam for tests that need a default pool of several members.
var catalogPools = modelcatalog.Pools

// defaultPoolSpec is the spec every role uses by default.
var defaultPoolSpec = modelcatalog.PoolSpecPrefix + modelcatalog.DefaultPool

// ManagerConfig supplies the computer-owned policy path and the server-owned
// provider dependencies used by verification tools.
type ManagerConfig struct {
	PolicyPath     string
	ProviderConfig provideriface.Config
	Provider       provideriface.Provider
	// ImageStore holds images verification calls send by ref; nil keeps
	// inline images inline and refuses refs.
	ImageStore ImageStore
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
	thread := ThreadKey(metadata)
	spec := SpecOf(selection)
	selection, fallbacks, assignment := Assign(selection, thread)
	if NormalizeRole(role) == ManagementTaskRole {
		// A pinned task never fails over to a free or preview model (R2-9).
		// Owner 2026-10-28: every desk falls back to the other model when
		// one is out, so the pin fails over to the default pool's other
		// members only.
		fallbacks = managementTaskFallbacks(selection)
	}
	if assignment == AssignmentUnknownPool {
		// Fail closed, visibly: never run the role on another pool's model.
		metadata[MetadataPolicyError] = joinPolicyErrors(metadata, fmt.Sprintf("model policy names unknown pool %q", strings.TrimPrefix(strings.TrimSpace(selection.Model), modelcatalog.PoolSpecPrefix)))
	}
	metadata[MetadataSpec] = spec
	metadata[MetadataAssignment] = assignment
	metadata[MetadataThread] = thread
	if len(fallbacks) > 0 {
		metadata[MetadataFallbacks] = FormatFallbacks(fallbacks)
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

func joinPolicyErrors(metadata map[string]any, text string) string {
	if existing := metadataString(metadata, MetadataPolicyError); existing != "" {
		return existing + "; " + text
	}
	return text
}

// ThreadKey is the key a pool assignment hashes: the Texture agent for a
// Texture run (a document keeps one model), otherwise the trajectory. A run
// with neither gets a fresh random key.
func ThreadKey(metadata map[string]any) string {
	if agentID := metadataString(metadata, "agent_id"); strings.HasPrefix(agentID, "texture:") {
		return agentID
	}
	if trajectory := metadataString(metadata, "trajectory_id"); trajectory != "" {
		return trajectory
	}
	return fmt.Sprintf("random:%d", time.Now().UnixNano())
}

// SpecOf renders a selection as the spec it was asked as.
func SpecOf(selection provideriface.LLMSelection) string {
	model := strings.TrimSpace(selection.Model)
	if strings.HasPrefix(model, modelcatalog.PoolSpecPrefix) || strings.TrimSpace(selection.Provider) == "" {
		return model
	}
	return strings.TrimSpace(selection.Provider) + "/" + model
}

// Assign resolves a selection to one concrete model for thread. A pool spec
// hashes the thread onto a member; the members after it are the fallbacks,
// except in a pool that is NoFailover (no fallbacks at all).
// A concrete model the catalog has retired is remapped to the default pool.
// A concrete model is a pin: it has no fallbacks, in any pool.
// A pool the catalog does not name is not remapped: it returns the selection
// unresolved with AssignmentUnknownPool.
func Assign(selection provideriface.LLMSelection, thread string) (provideriface.LLMSelection, []provideriface.LLMSelection, string) {
	model := strings.TrimSpace(selection.Model)
	assignment := "explicit"
	pool := ""
	switch {
	case strings.HasPrefix(model, modelcatalog.PoolSpecPrefix):
		pool = strings.TrimPrefix(model, modelcatalog.PoolSpecPrefix)
		assignment = "hash"
	case modelcatalog.ProviderCatalogued(selection.Provider):
		if _, ok := modelcatalog.Lookup(selection.Provider, model); !ok {
			pool = modelcatalog.DefaultPool
			assignment = "retired_remapped"
		}
	}
	if pool == "" {
		// A concrete pin fails rather than falls back (owner 2026-10-23,
		// R2): it names one model, so it gets none of any pool's members.
		return selection, nil, assignment
	}
	members, ok := catalogPools()[pool]
	if !ok || len(members) == 0 {
		// An unknown pool is not the default pool. The selection comes back
		// unresolved (still "pool:<name>", no provider, no fallbacks) so a
		// caller that skipped the parser's check fails visibly.
		return selection, nil, AssignmentUnknownPool
	}
	index := weightedIndex(members, fnv32(thread))
	ordered := poolSelections(rotate(members, index), selection)
	if modelcatalog.NoFailover(pool) {
		// The member that fails, fails: no fallbacks, not even the default
		// pool's (owner 2026-10-23: "I'd rather them fail than fallback").
		return ordered[0], nil, assignment
	}
	return ordered[0], ordered[1:], assignment
}

// AssignmentUnknownPool is the assignment Assign reports for a pool spec the
// catalog does not name.
const AssignmentUnknownPool = "unknown_pool"

// FormatFallbacks renders fallbacks as provider/model, comma-separated.
func FormatFallbacks(fallbacks []provideriface.LLMSelection) string {
	parts := make([]string, 0, len(fallbacks))
	for _, fallback := range fallbacks {
		parts = append(parts, fallback.Provider+"/"+fallback.Model)
	}
	return strings.Join(parts, ",")
}

// weightedIndex maps a thread hash onto a member, each member taking a
// share of hashes in proportion to its weight. A pool of fallback-only
// members assigns its first.
func weightedIndex(members []modelcatalog.PoolMember, hash uint32) int {
	total := 0
	for _, member := range members {
		total += member.SlotWeight()
	}
	if total == 0 {
		return 0
	}
	slot := int(hash % uint32(total))
	for i, member := range members {
		if slot < member.SlotWeight() {
			return i
		}
		slot -= member.SlotWeight()
	}
	return len(members) - 1
}

func rotate(members []modelcatalog.PoolMember, start int) []modelcatalog.PoolMember {
	out := make([]modelcatalog.PoolMember, 0, len(members))
	out = append(out, members[start:]...)
	return append(out, members[:start]...)
}

func poolSelections(members []modelcatalog.PoolMember, base provideriface.LLMSelection) []provideriface.LLMSelection {
	out := make([]provideriface.LLMSelection, 0, len(members))
	for _, member := range members {
		selection := base
		selection.Provider, selection.Model = member.Provider, member.Model
		out = append(out, selection)
	}
	return out
}

func fnv32(key string) uint32 {
	hash := uint32(2166136261)
	for i := 0; i < len(key); i++ {
		hash ^= uint32(key[i])
		hash *= 16777619
	}
	return hash
}

// The management task model (management delegation S2a, design round 2
// R2-9 and O5): a task has a 10-minute deadline, so it runs on one
// non-preview model with bounded latency, never on a pool that assigns a
// model per thread. The owner may name another provider and model for the
// role; a pool, a free model or a preview model is not honoured. The pin
// was deepseek-v4.1-flash until the owner's 3:1 GPT split (2026-10-15),
// which it kept at a fifth of the paper's calls on DeepSeek
// (docs/problems/pool-split-misses-three-to-one-management-task-pin-2026-10-15.md).
const (
	ManagementTaskRole     = agentprofile.ManagementTask
	ManagementTaskProvider = "chatgpt"
	ManagementTaskModel    = "gpt-6-luna"
)

func managementTaskSelection(p Policy) provideriface.LLMSelection {
	pin := provideriface.LLMSelection{Provider: ManagementTaskProvider, Model: ManagementTaskModel, ReasoningEffort: "high", Source: "management_task_pin"}
	selection, ok := p.Roles[ManagementTaskRole]
	if !ok {
		return pin
	}
	model := strings.ToLower(strings.TrimSpace(selection.Model))
	if strings.HasPrefix(model, modelcatalog.PoolSpecPrefix) || strings.TrimSpace(selection.Provider) == "" ||
		strings.HasSuffix(model, "-free") || strings.Contains(model, "preview") {
		return pin
	}
	return fillSelection(selection, p.Defaults)
}

// managementTaskFallbacks is the pinned task's failover order: the default
// pool's members other than the pin, in pool order, never a free or preview
// model.
func managementTaskFallbacks(pin provideriface.LLMSelection) []provideriface.LLMSelection {
	var out []provideriface.LLMSelection
	for _, member := range poolSelections(catalogPools()[modelcatalog.DefaultPool], pin) {
		model := strings.ToLower(member.Model)
		if (member.Provider == pin.Provider && member.Model == pin.Model) || strings.HasSuffix(model, "-free") || strings.Contains(model, "preview") {
			continue
		}
		out = append(out, member)
	}
	return out
}

// Resolve returns the effective selection for role.
func (p Policy) Resolve(role string) provideriface.LLMSelection {
	normalized := NormalizeRole(role)
	if normalized == ManagementTaskRole {
		return managementTaskSelection(p)
	}
	if p.Roles != nil {
		if selection, ok := p.Roles[normalized]; ok {
			return fillSelection(selection, p.Defaults)
		}
	}
	return fillSelection(provideriface.LLMSelection{}, p.Defaults)
}

// Selections returns each distinct model the policy names, defaults first
// and then roles in name order, filled from the defaults: the models this
// computer is configured to call.
func (p Policy) Selections() []provideriface.LLMSelection {
	roles := make([]string, 0, len(p.Roles))
	for role := range p.Roles {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	out := make([]provideriface.LLMSelection, 0, len(roles)+1)
	seen := make(map[string]bool, len(roles)+1)
	add := func(selection provideriface.LLMSelection) {
		if strings.HasPrefix(selection.Model, modelcatalog.PoolSpecPrefix) {
			members := catalogPools()[strings.TrimPrefix(selection.Model, modelcatalog.PoolSpecPrefix)]
			for _, member := range poolSelections(members, selection) {
				if !seen[member.Model] {
					seen[member.Model] = true
					out = append(out, member)
				}
			}
			return
		}
		if selection.Model == "" || seen[selection.Model] {
			return
		}
		seen[selection.Model] = true
		out = append(out, selection)
	}
	add(fillSelection(provideriface.LLMSelection{}, p.Defaults))
	for _, role := range roles {
		add(fillSelection(p.Roles[role], p.Defaults))
	}
	return out
}

// NormalizeRole resolves exactly the V2 live vocabulary (identity map with
// fail-closed default). V1 aliases refuse here; history decodes through the
// frozen V1 decoder, never through this function.
func NormalizeRole(role string) string {
	normalized := strings.TrimSpace(strings.ToLower(role))
	switch normalized {
	case agentprofile.Management, agentprofile.Engineering, agentprofile.Research,
		agentprofile.Conductor, agentprofile.Texture, agentprofile.Email, VerifierRole, agentprofile.ManagementTask:
		return normalized
	case MultimodalVerifierRole, "verifier-multimodal":
		// One canonical spelling: the hyphenated form normalizes to the
		// underscore policy key so a stored value can never pass the fence
		// while missing its roles.<name> section.
		return MultimodalVerifierRole
	default:
		return ""
	}
}

// decodeRoleSection resolves a persisted TOML roles.<name> section header.
// Computer-owned TOML overlays are unmarked persistence and default to V1:
// V2 names resolve through NormalizeRole; V1 names decode through the frozen
// V1 map (the file's bytes stay immutable; the vocabulary is selected at the
// decode boundary). Unknown sections refuse.
func decodeRoleSection(name string) string {
	if role := NormalizeRole(name); role != "" {
		return role
	}
	if v2, ok := vocabmigrate.ForwardV1ToV2(name); ok {
		return v2
	}
	return ""
}

// ProviderPreconditionFallbackSelections deliberately returns no alternate
// selection. A provider/model precondition failure must surface from the
// originally selected provider rather than silently changing authority.
func ProviderPreconditionFallbackSelections(_ provideriface.LLMSelection) []provideriface.LLMSelection {
	return nil
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
	return policyVersionLine + `
# Choir model policy
# This computer-owned file maps agent roles to a model spec: "pool:<name>"
# (a member is assigned per thread; the others are its fallbacks) or a
# provider and model. Add a line "# owner-edited" to keep your edits when
# the platform default changes.
# Provider secrets stay server-owned; this file names models only.
# Optional max_tokens requests an explicit per-call budget. Omit it for provider
# defaults, especially OpenAI-compatible chat completions.

[defaults]
model = "pool:default"
reasoning = "low"

[roles.conductor]
model = "pool:default"
reasoning = "low"

[roles.texture]
model = "pool:default"
reasoning = "high"

[roles.management]
model = "pool:default"
reasoning = "high"

[roles.engineering]
model = "pool:default"

[roles.research]
model = "pool:default"
reasoning = "low"

[roles.verifier]
model = "pool:default"
requires = ["text", "tool_use"]

[roles.verifier_multimodal]
model = "pool:default"
requires = ["image", "tool_use"]
`
}

func fallbackPolicy(_ provideriface.Config) Policy {
	// Owner 2026-10-12: every role runs on the default pool, assigned per
	// thread (since 2026-10-28 Synthetic DeepSeek 1017 and gpt-6-luna at 1:1,
	// each failing over to the other; modelcatalog.Pools). Texture
	// and management keep high effort; opencode-go ignores the setting.
	pool := func(effort string) provideriface.LLMSelection {
		return provideriface.LLMSelection{Model: defaultPoolSpec, ReasoningEffort: effort, Source: "platform_fallback"}
	}
	return Policy{
		Defaults: pool("low"),
		Roles: map[string]provideriface.LLMSelection{
			agentprofile.Conductor:   pool("low"),
			agentprofile.Management:  pool("high"),
			agentprofile.Engineering: pool(""),
			agentprofile.Research:    pool("low"),
			agentprofile.Texture:     pool("high"),
			VerifierRole:             pool(""),
			MultimodalVerifierRole:   pool(""),
		},
		Source: "platform_fallback",
	}
}

func ensureDefaultPolicyFile(path string, cfg provideriface.Config) error {
	raw, err := os.ReadFile(path)
	if err == nil {
		text := string(raw)
		if strings.Contains(text, policyVersionLine) || strings.Contains(text, ownerEditedLine) {
			return nil
		}
		// An older platform default (it named retired models): replace it.
		return os.WriteFile(path, []byte(defaultPolicyText(cfg)), 0o644)
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
			role := decodeRoleSection(strings.TrimPrefix(section, "roles."))
			if role == "" {
				return Policy{}, fmt.Errorf("line %d: unknown role section %q", lineNo, section)
			}
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
	if !validSelection(policy.Defaults) {
		return Policy{}, fmt.Errorf("defaults require a pool spec or a provider and model")
	}
	if pool := unknownPool(policy.Defaults); pool != "" {
		return Policy{}, fmt.Errorf("defaults name unknown pool %q", pool)
	}
	for role, selection := range policy.Roles {
		if !validSelection(selection) {
			return Policy{}, fmt.Errorf("role %q requires a pool spec or a provider and model", role)
		}
		if pool := unknownPool(selection); pool != "" {
			return Policy{}, fmt.Errorf("role %q names unknown pool %q", role, pool)
		}
	}
	return policy, nil
}

// validSelection accepts a pool spec, or a provider with a model.
func validSelection(selection provideriface.LLMSelection) bool {
	model := strings.TrimSpace(selection.Model)
	if strings.HasPrefix(model, modelcatalog.PoolSpecPrefix) {
		return len(model) > len(modelcatalog.PoolSpecPrefix)
	}
	return strings.TrimSpace(selection.Provider) != "" && model != ""
}

// unknownPool returns the pool a selection names when the catalog has no such
// pool, else "". A file that names one is invalid, not remapped.
func unknownPool(selection provideriface.LLMSelection) string {
	model := strings.TrimSpace(selection.Model)
	if !strings.HasPrefix(model, modelcatalog.PoolSpecPrefix) {
		return ""
	}
	name := strings.TrimPrefix(model, modelcatalog.PoolSpecPrefix)
	if len(catalogPools()[name]) == 0 {
		return name
	}
	return ""
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
			role := decodeRoleSection(strings.TrimPrefix(section, "roles."))
			if role == "" {
				return policyOverlay{}, fmt.Errorf("line %d: unknown role section %q", lineNo, section)
			}
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
	if pool := unknownPool(overlay.Defaults); pool != "" {
		return policyOverlay{}, fmt.Errorf("overlay defaults name unknown pool %q", pool)
	}
	if strings.TrimSpace(overlay.Defaults.Provider) == "" && strings.TrimSpace(overlay.Defaults.Model) != "" {
		return policyOverlay{}, fmt.Errorf("overlay defaults require provider when model is set")
	}
	if strings.TrimSpace(overlay.Defaults.Model) == "" && strings.TrimSpace(overlay.Defaults.Provider) != "" {
		return policyOverlay{}, fmt.Errorf("overlay defaults require model when provider is set")
	}
	for role, selection := range overlay.Roles {
		if !validSelection(selection) {
			return policyOverlay{}, fmt.Errorf("overlay role %q requires a pool spec or a provider and model", role)
		}
		if pool := unknownPool(selection); pool != "" {
			return policyOverlay{}, fmt.Errorf("overlay role %q names unknown pool %q", role, pool)
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
