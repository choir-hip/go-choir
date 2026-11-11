package modelcatalog

import (
	"fmt"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"strings"
)

const (
	DefaultMaxOutputTokens     = 65536
	DefaultContextWindowTokens = 200000
)

// ModelInfo describes a supported model and its associated provider.
type ModelInfo struct {
	// ID is the model identifier used in API requests (provider-specific).
	ID string `json:"id"`

	// DisplayName is a human-readable name for logging and UI.
	DisplayName string `json:"display_name"`

	// Provider is the provider name that serves this model (e.g., "zai",
	// "fireworks", "bedrock").
	Provider string `json:"provider"`

	// MaxOutputTokens is the maximum output tokens for this model.
	MaxOutputTokens int `json:"max_output_tokens"`

	// ContextWindowTokens is the advertised or platform-assumed input context
	// window used for runtime pressure and compaction policy.
	ContextWindowTokens int `json:"context_window_tokens,omitempty"`

	// Modalities names the input modalities Choir sends this model. It is
	// filled by SupportedModels from the vision default and the text-only
	// opt-out list (TextOnlyModels); catalog entries do not set it.
	Modalities []string `json:"modalities,omitempty"`

	// AdapterModalities names modalities Choir's adapter for this provider
	// serializes. Every adapter serializes text and images (user turns).
	AdapterModalities []string `json:"adapter_modalities,omitempty"`

	// TextOnlyReason is why the model is opted out of image input; empty
	// for a vision model.
	TextOnlyReason string `json:"text_only_reason,omitempty"`

	// RecommendedFor names role/purpose hints for model policy UIs.
	RecommendedFor []string `json:"recommended_for,omitempty"`
}

// Input modalities.
const (
	ModalityText  = "text"
	ModalityImage = "image"
)

// TextOnlyModels is the explicit image opt-out list: catalogued model id to
// the reason it may not receive an image. Owner 2026-10-15: vision is on by
// default for every model the catalog configures; a model goes here only
// when it is known text-only or its image support through our route is
// UNVERIFIED. Delete a line once scripts/vision_probe.py --model <id>
// passes on staging (docs/archive/problems/models-configured-text-only-2026-10-15.md).
func TextOnlyModels() map[string]string {
	const zaiText = "believed text-only: Z.AI ships vision as separate GLM-*V models (UNVERIFIED for this version)"
	const unverified = "UNVERIFIED: no doc or code confirms image input through OpenCode for this model"
	return map[string]string{
		"glm-5.2":                         zaiText,
		"glm-5.1":                         zaiText,
		"glm-5-turbo":                     zaiText,
		"glm-5.3-flash":                   unverified,
		"muse-spark-1.3-contributor":      unverified,
		"muse-spark-1.3-contributor-free": unverified,
		"hy3":                             unverified,
		"qwen3.7-max":                     unverified,
		"cat-jam-free":                    unverified,
		"longcat-2.5-preview-free":        unverified,
		"step-5-preview-free":             unverified,
		"iceman-contributor-free":         unverified,
		"beam-free":                       unverified,
		"recursive/flint-v1.0:free":       "text-only: the OpenRouter listing gives text input only (2026-11-05)",
		"swe-2-flash":                     "text-only: the Devin model catalog lists text input only for swe-2-flash (2026-11-09)",
	}
}

// SupportsImage reports whether Choir may send an image to modelID: the
// model is catalogued (configured) and not on the text-only opt-out list.
// An uncatalogued id is refused: Choir knows nothing about it.
func SupportsImage(modelID string) bool {
	return ImageRefusal(modelID) == ""
}

// ImageRefusal explains why modelID may not receive an image, or "" when
// it may.
func ImageRefusal(modelID string) string {
	modelID = strings.TrimSpace(modelID)
	for _, model := range SupportedModels() {
		if model.ID != modelID {
			continue
		}
		if model.TextOnlyReason == "" {
			return ""
		}
		return fmt.Sprintf("model %q is opted out of image input in the Choir model catalog (%s)", modelID, model.TextOnlyReason)
	}
	return fmt.Sprintf("model %q is not in the Choir model catalog; image input requires a catalogued model", modelID)
}

// withModalities applies the vision default and the opt-out list.
func withModalities(models []ModelInfo) []ModelInfo {
	textOnly := TextOnlyModels()
	for i := range models {
		models[i].AdapterModalities = []string{ModalityText, ModalityImage}
		if reason, ok := textOnly[models[i].ID]; ok {
			models[i].Modalities = []string{ModalityText}
			models[i].TextOnlyReason = reason
			continue
		}
		models[i].Modalities = []string{ModalityText, ModalityImage}
	}
	return models
}

// SupportedModels returns the list of models Choir knows how to route through
// platform-owned providers. A new entry takes images by default; opt a
// text-only model out in TextOnlyModels.
func SupportedModels() []ModelInfo {
	return withModalities([]ModelInfo{
		// The Bedrock output maximums are the ones Bedrock names when asked
		// for more (probed 2026-11-09): 64,000 for Haiku 4.5, 128,000 for the
		// rest.
		{
			ID:              "us.anthropic.claude-haiku-4-5-20251001-v1:0",
			DisplayName:     "Claude Haiku 4.5",
			Provider:        "bedrock",
			MaxOutputTokens: 64000,
		},
		// Claude Haiku 5.5 on Bedrock (owner 2026-11-09): the paper's last
		// fallback, $0.10/M in and $0.50/M out. Adaptive thinking; the effort
		// goes in output_config.
		{
			ID:                  "us.anthropic.claude-haiku-5-5",
			DisplayName:         "Claude Haiku 5.5",
			Provider:            "bedrock",
			MaxOutputTokens:     128000,
			ContextWindowTokens: 1_000_000,
		},
		{
			ID:              "us.anthropic.claude-sonnet-4-6",
			DisplayName:     "Claude Sonnet 4.6",
			Provider:        "bedrock",
			MaxOutputTokens: 128000,
		},
		{
			ID:              "us.anthropic.claude-opus-4-6-v1",
			DisplayName:     "Claude Opus 4.6",
			Provider:        "bedrock",
			MaxOutputTokens: 128000,
		},
		{
			ID:                  "glm-5.2",
			DisplayName:         "GLM-5.2",
			Provider:            "zai",
			MaxOutputTokens:     131072,
			ContextWindowTokens: 1_000_000,
			RecommendedFor:      []string{deskmanifest.AdmitterName(), deskmanifest.PerformerName(), "verifier"},
		},
		{
			ID:              "glm-5.1",
			DisplayName:     "GLM-5.1",
			Provider:        "zai",
			MaxOutputTokens: 131072,
		},
		{
			ID:              "glm-5-turbo",
			DisplayName:     "GLM-5-Turbo",
			Provider:        "zai",
			MaxOutputTokens: 131072,
		},
		{
			ID:                  "deepseek-v4.1-flash",
			DisplayName:         "DeepSeek V4.1 Flash",
			Provider:            "opencode-go",
			MaxOutputTokens:     65536,
			ContextWindowTokens: DefaultContextWindowTokens,
			RecommendedFor:      []string{deskmanifest.PerformerName(), "verifier"},
		},
		{
			ID:                  "glm-5.3-flash",
			DisplayName:         "GLM-5.3 Flash",
			Provider:            "opencode-go",
			MaxOutputTokens:     65536,
			ContextWindowTokens: DefaultContextWindowTokens,
			RecommendedFor:      []string{deskmanifest.PerformerName(), "verifier"},
		},
		{
			ID:                  "muse-spark-1.3-contributor",
			DisplayName:         "Muse Spark 1.3 Contributor",
			Provider:            "opencode-go",
			MaxOutputTokens:     65536,
			ContextWindowTokens: DefaultContextWindowTokens,
			RecommendedFor:      []string{deskmanifest.PerformerName(), "verifier"},
		},
		{
			ID:                  "muse-spark-1.3-contributor-free",
			DisplayName:         "Muse Spark 1.3 Contributor Free",
			Provider:            "opencode-zen",
			MaxOutputTokens:     65536,
			ContextWindowTokens: DefaultContextWindowTokens,
			RecommendedFor:      []string{deskmanifest.PerformerName(), "verifier"},
		},
		{
			ID:                  "hy3",
			DisplayName:         "HY3",
			Provider:            "opencode-go",
			MaxOutputTokens:     65536,
			ContextWindowTokens: DefaultContextWindowTokens,
			RecommendedFor:      []string{deskmanifest.PerformerName()},
		},
		{
			ID:                  "qwen3.7-max",
			DisplayName:         "Qwen 3.7 Max",
			Provider:            "opencode-go",
			MaxOutputTokens:     65536,
			ContextWindowTokens: DefaultContextWindowTokens,
			RecommendedFor:      []string{deskmanifest.PerformerName(), "verifier"},
		},
		// Free opencode-go models (owner 2026-10-13: "add these to our pool,
		// and see how much usage we can get out of them"). Chat completions,
		// probed with a tool call on 2026-10-13.
		{
			ID:                  "cat-jam-free",
			DisplayName:         "Cat Jam (free)",
			Provider:            "opencode-go",
			MaxOutputTokens:     65536,
			ContextWindowTokens: 262144,
		},
		{
			ID:                  "longcat-2.5-preview-free",
			DisplayName:         "LongCat 2.5 Preview (free)",
			Provider:            "opencode-go",
			MaxOutputTokens:     65536,
			ContextWindowTokens: DefaultContextWindowTokens,
		},
		{
			ID:                  "step-5-preview-free",
			DisplayName:         "Step 5 Preview (free)",
			Provider:            "opencode-go",
			MaxOutputTokens:     65536,
			ContextWindowTokens: DefaultContextWindowTokens,
		},
		// Iceman Contributor (free), a promotional opencode-go model (owner
		// 2026-11-05: "we should use it as much as we can"). Chat
		// completions with a tool call, probed 2026-11-05: about 1.5 s,
		// cost 0. The opencode listing gives a 262K window, 131K output.
		{
			ID:                  "iceman-contributor-free",
			DisplayName:         "Iceman Contributor (free)",
			Provider:            "opencode-go",
			MaxOutputTokens:     65536,
			ContextWindowTokens: 262144,
		},
		// Beam (free) on opencode-go, probed the same day with a two-turn
		// tool loop: about 1 s a turn, cost 0; the listing gives a 1M window.
		// Of OpenCode's free models only beam and iceman answer outside the
		// OpenCode app: the Zen free tier refuses with FreeUsageLimitError.
		{
			ID:                  "beam-free",
			DisplayName:         "Beam (free)",
			Provider:            "opencode-go",
			MaxOutputTokens:     65536,
			ContextWindowTokens: 1048576,
		},

		// Synthetic (synthetic.new), probed 2026-10-22 before any traffic:
		// the window is 524288 tokens (512 Ki, prompt plus completion; a
		// request past it is a 400), not the 1M DeepSeek's own API serves;
		// max output is advertised at 65536 and unproven at the cap. Vision
		// verified with an image (docs/problems/synthetic-provider-
		// onboarding-2026-10-22.md). No RecommendedFor until the overnight
		// evidence.
		{
			ID:                  "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017",
			DisplayName:         "DeepSeek V4.1 Flash 1017 (Synthetic)",
			Provider:            "synthetic",
			MaxOutputTokens:     65536,
			ContextWindowTokens: 524288,
		},

		// River (owner 2026-11-09: self-development while Synthetic DeepSeek
		// is spent). The same DeepSeek V4.1 Flash 1017 snapshot; the window
		// matches Synthetic's until River's own is probed. In one pool only:
		// selfdev.
		{
			ID:                  "deepseek-ai/DeepSeek-V4.1-Flash-1017",
			DisplayName:         "DeepSeek V4.1 Flash 1017 (River)",
			Provider:            "river",
			MaxOutputTokens:     65536,
			ContextWindowTokens: 524288,
		},

		// Devin SWE-2 (Cognition) through the Codeium Connect-RPC backend
		// (internal/provider/devin.go). Both models are free to us until
		// their promotional ends: swe-2-flash through 2026-12-04, swe-2
		// through 2026-11-30 (internal/llmcost/pricing.go prices them at
		// zero with the same dates; revisit both on those days).
		// swe-2-flash is text only (TextOnlyModels) and does not support
		// parallel tool calls: the provider asks for one call per turn.
		// Reasoning effort selects the model UID suffix: low and medium map
		// to swe-2-flash-medium, high to -high, max/xhigh to -max, so a role
		// whose policy says reasoning = "low" still gets -medium on purpose;
		// there is no -low UID. A bare "swe-2" is refused on some plans.
		{
			ID:                  "swe-2-flash",
			DisplayName:         "SWE-2 Flash (Devin)",
			Provider:            "devin",
			MaxOutputTokens:     131072,
			ContextWindowTokens: 262144,
		},
		{
			ID:                  "swe-2",
			DisplayName:         "SWE-2 (Devin)",
			Provider:            "devin",
			MaxOutputTokens:     128000,
			ContextWindowTokens: 262000,
		},
		// OpenRouter's thinkingmachines/inkling-2-small:free: the owner's
		// intended default model (owner 2026-11-09). Listing that day:
		// 1048576-token window, 131072 max completion tokens, text and image
		// input, tools, reasoning and reasoning_effort, no tool_choice. In no
		// pool yet: OpenRouter answers it 403 "only available on agentic
		// harnesses" (docs/problems/crouton-retired-inkling-gated-2026-11-09.md).
		{
			ID:                  "thinkingmachines/inkling-2-small:free",
			DisplayName:         "Inkling 2 Small (OpenRouter free)",
			Provider:            "openrouter",
			MaxOutputTokens:     131072,
			ContextWindowTokens: 1048576,
		},
		// OpenRouter free variants (owner 2026-11-05: "add these 2 models
		// with openrouter as well"). Each passed a two-turn tool loop at
		// usage.cost 0 and every reasoning effort from minimal to max that
		// day. Flint: a 262144-token window, text only. Nemotron 3.5 Super
		// VL: a 1000000-token window, text, image and video input.
		{
			ID:                  "recursive/flint-v1.0:free",
			DisplayName:         "Flint 1.0 (OpenRouter free)",
			Provider:            "openrouter",
			MaxOutputTokens:     65536,
			ContextWindowTokens: 262144,
		},
		{
			ID:                  "nvidia/nemotron-3.5-super-vl:free",
			DisplayName:         "Nemotron 3.5 Super VL (OpenRouter free)",
			Provider:            "openrouter",
			MaxOutputTokens:     65536,
			ContextWindowTokens: 1000000,
		},

		// No DeepSeek through OpenRouter (owner, 2026-11-04: "only use Jev
		// via openrouter, and free models like crouton"). The arms that ran
		// DeepSeek V4.1 Flash there drained the account's credit.

		{
			ID:                  "gpt-6-luna",
			DisplayName:         "GPT-6 Luna",
			Provider:            "chatgpt",
			MaxOutputTokens:     65536,
			ContextWindowTokens: DefaultContextWindowTokens,
		},
	})
}

// DefaultPool names the pool a spec without a model resolves to.
const DefaultPool = "default"

// PoolSpecPrefix marks a model spec that names a pool instead of a model.
const PoolSpecPrefix = "pool:"

// PoolMember is one (provider, model) pair in a pool.
type PoolMember struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	// Weight is the member's share of hashed thread assignments; zero
	// counts as one.
	Weight int `json:"weight,omitempty"`
	// FallbackOnly members are assigned no thread; they serve only as a
	// failover for the members before them.
	FallbackOnly bool `json:"fallback_only,omitempty"`
	// Spread names a group of adjacent members whose order is chosen per
	// thread: a stable hash of the thread picks the group's first member in
	// proportion to SpreadWeight (zero counts as one) and the others follow
	// in pool order, wrapping. A member with no Spread is not part of any
	// group; a pool with no Spread is strict order. Owner 2026-11-10: the
	// default pool's paid tail is spread so Devin's downtime lands some
	// threads on Synthetic and River DeepSeek, not all on Haiku.
	Spread       string `json:"spread,omitempty"`
	SpreadWeight int    `json:"spread_weight,omitempty"`
}

// SpreadShare is the member's share inside its spread group.
func (m PoolMember) SpreadShare() int {
	if m.SpreadWeight < 1 {
		return 1
	}
	return m.SpreadWeight
}

// SlotWeight is the member's assignment weight: zero for a fallback-only
// member, else at least one.
func (m PoolMember) SlotWeight() int {
	if m.FallbackOnly {
		return 0
	}
	if m.Weight < 1 {
		return 1
	}
	return m.Weight
}

// Pools returns the named model pools, each an ordered list of catalog
// entries. A thread is assigned one member and fails over to the next in
// order. Owner 2026-10-12: every desk runs on a mix of deepseek and luna
// (docs/model-pools-design-2026-10-12.md). Owner 2026-10-13: the free
// opencode-go models join the pool to measure how much they carry and
// whether they meet the bar; gpt-6-luna sits last so a thread that fails
// over from it wraps to deepseek. step-5-preview-free stays catalog-only
// until its upstream answers (429 "endpoint unavailable" when probed).
// longcat-2.5-preview-free was measured and left the pool on 2026-10-14:
// 21.8 s median and 115 s p90 per call on the paper computer, up to 932 s,
// against 2.5 s and 10 s for gpt-6-luna; a management turn on it held the
// paper for 38 minutes (docs/archive/evidence/model-latency-paper-2026-10-14.md).
// cat-jam-free left the pool on 2026-10-15 (owner: a dual pool of
// deepseek-v4.1-flash and gpt-6-luna). An OpenCode key is parked for every
// model after any 429, so its free-tier 429s parked the keys deepseek uses
// (docs/archive/reports/chatgpt-cache-ratio-2026-10-15.md §7.3). The owner weighted
// it 1:3 the same day. Owner 2026-10-16: "we are using Opencode too much. We
// will hit our weekly limit soon. Let's go 100% gpt-6 luna for now." Owner
// 2026-10-21: "We have Opencode quota now"; the dual pool is back at 1:3, so
// the same kinds of task run on both models and the tool-call failures they
// share are fixed in the tool surface rather than tuned to one model.
// Owner 2026-10-22: the paper runs half Synthetic DeepSeek 1017, half ChatGPT
// overnight, "to see how much of my synthetic usage I end up using": the
// synthetic-luna pool, 1:1. Owner 2026-10-23 (UTC): "Just ChatGPT luna and
// synthetic DeepSeek 1017 tonight. I'd rather them fail than fallback. We
// are still pre-launch" and "Don't use Opencode as fallback tonight." So
// synthetic-luna fails over to nothing (NoFailover), and the default pool is
// gpt-6-luna alone tonight: opencode-go deepseek-v4.1-flash left it (its
// catalog entry stays), so no thread falls back to OpenCode.
// Owner 2026-10-23 (14:4x UTC): "synthetic is using much faster... we should
// go to chatgpt luna 100% with synthetic only as fallback for now": the
// luna-synthetic pool. gpt-6-luna takes every thread; Synthetic's DeepSeek
// 1017 is a fallback-only member, reached only when luna fails, and the pool
// fails over in the normal way (it is not NoFailover). The default and
// synthetic-luna pools are unchanged.
// Owner 2026-10-28: the default pool, which every desk of a new computer
// names, is "a pool 50-50 between ChatGPT and synthetic DeepSeek ...
// falling back to one or the other if one is unavailable": Synthetic's
// DeepSeek 1017 and gpt-6-luna at 1:1, failing over in the normal way, so
// a thread whose member is out of quota moves to the other.
// Owner 2026-11-05: the free promotional iceman-contributor-free "we should
// use it as much as we can ... run autopaper with it in a pool with gpt",
// "in addition to crouton", then "use all that work in a pool", then
// OpenRouter's free flint and nemotron. The default pool, which the paper
// and every other computer name, is the two free OpenCode models that
// answer outside the OpenCode app (iceman and beam), OpenRouter's
// crouton-alpha, flint and nemotron, and gpt-6-luna, one share each,
// failing over in order; Synthetic's DeepSeek 1017 (its quota ran out on
// 2026-11-05) is a fallback-only last member.
// Owner 2026-11-09: "it shouldn't cycle through all these models that we
// already know in advance are not going to work. It should go straight to the
// one that does work, which currently is the Crouton Alpha ... we should be
// using Crouton Alpha for almost everything right now, and we can also use
// Synthetic DeepSeek ... seldomly, or save it for getting the timing of
// things" (a benchmark). The default pool leads with crouton-alpha, the only
// member assigned threads; iceman, beam, flint, nemotron and luna stay behind
// it as fallback-only members. The tool loop skips a member the computer's
// availability record holds (quota or a recent failure) until its hold ends
// (docs/problems/pool-routing-walks-known-dead-members-2026-11-09.md).
// Owner 2026-11-09: "I didn't realize we were going to run out of synthetic
// DeepSeek usage. I guess that means we ended up using it for auto paper,
// which I didn't really want". Evidence: 8,315 Synthetic calls from the
// paper's VM since 2026-11-08 versus 1,099 from d@e, ~50/h rising to 400-760/h
// after ead8873b6 made the loop skip held members and a crouton outage fell
// through to Synthetic, the default pool's last member. The default pool has
// no Synthetic member now; a crouton outage waits (the quota incident waits up
// to 18 min for the earliest reset). Synthetic stays in the catalog and in the
// named pools below, for a computer or benchmark that names it.
//
// Owner 2026-11-09: "We shouldn't have outages if we can handle it. Crouton
// and other free models ideally. Then haiku for paper and synthetic DeepSeek
// and river for self-dev (with sonnet delegate for Eng). And let's use low
// reasoning for everything (except sonnet which can be med)". So the default
// pool (the paper and ordinary computers) is crouton-alpha, the free
// fallbacks, gpt-6-luna, then Bedrock Claude Haiku 5.5 last, with no
// Synthetic and no River. (Superseded for selfdev later that day: see the
// Devin paragraph below and the SelfDevPool comment; selfdev is now Synthetic
// then River only.)
//
// Owner 2026-11-09: stealth/crouton-alpha was retired by OpenRouter that day
// and is gone from every pool; the members that followed it lead in their
// existing order. The owner's intended lead is
// thinkingmachines/inkling-2-small:free. It is catalogued but held out of
// every pool only because OpenRouter answers it 403 ("only available on
// agentic harnesses"); a dead lead is the bug being fixed. When the gate opens
// it becomes the lead of the default and selfdev pools by a one-line change
// (docs/problems/crouton-retired-inkling-gated-2026-11-09.md).
//
// Owner 2026-11-09 (later): Devin's swe-2-flash leads the default pool, ahead
// of iceman, because the free pool capacity is exhausted and the
// paper runs on the paid tail. Every previous member stays, in order, as
// fallback-only: a Devin refusal (quota, revoked session) holds the member
// and the loop moves to iceman and on. Devin authenticates with one person's
// Devin CLI session, an unofficial client of a private API; the pool must
// degrade to the other members the moment it refuses.
func Pools() map[string][]PoolMember {
	return map[string][]PoolMember{
		DefaultPool: {
			// Owner 2026-11-09: swe-2-flash leads (Devin, free until
			// 2026-12-04); every previous member follows in order. A role at
			// reasoning "low" still runs swe-2-flash-medium: the provider
			// maps low to the -medium UID (devin.go devinModelUID), by
			// design. Image turns skip it (text only) and land on the first
			// vision-capable member.
			{Provider: "devin", Model: "swe-2-flash", Weight: 1},
			{Provider: "opencode-go", Model: "iceman-contributor-free", FallbackOnly: true},
			{Provider: "opencode-go", Model: "beam-free", FallbackOnly: true},
			{Provider: "openrouter", Model: "recursive/flint-v1.0:free", FallbackOnly: true},
			{Provider: "openrouter", Model: "nvidia/nemotron-3.5-super-vl:free", FallbackOnly: true},
			{Provider: "chatgpt", Model: "gpt-6-luna", FallbackOnly: true},
			// Owner 2026-11-10: "I don't want DeepSeek before haiku but spread
			// the load during Devin's downtime"; "use river and synthetic just
			// a bit in autopaper. Like 10% each". The paid tail is one spread
			// group: Haiku about 80% of threads, Synthetic DeepSeek 10%, River
			// DeepSeek 10%, by thread hash, each falling through to the others.
			{Provider: "bedrock", Model: "us.anthropic.claude-haiku-5-5", FallbackOnly: true, Spread: "paid-tail", SpreadWeight: 8},
			{Provider: "synthetic", Model: "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017", FallbackOnly: true, Spread: "paid-tail", SpreadWeight: 1},
			{Provider: "river", Model: "deepseek-ai/DeepSeek-V4.1-Flash-1017", FallbackOnly: true, Spread: "paid-tail", SpreadWeight: 1},
		},
		// Owner 2026-11-11: "Voice prompts should go to autoradio desk which
		// should have a fast model that responds quickly", then "We should
		// actually use River DeepSeek rather than Bedrock Haiku for this",
		// then "we should have Bedrock as the fallback after River DeepSeek. I
		// don't want to use Devin Flash for this because it's already
		// overloaded with AutoPaper." So the pool is exactly River, then
		// Bedrock Haiku 5.5. Synthetic is reserved for self-development and
		// beam is often parked. A refusal here is audible silence, so a member
		// is tried the moment the one before it refuses. Bedrock does not
		// stream (provider.BedrockProvider.Stream makes one whole call), so as
		// the fallback it delivers its answer at once. Thinking is never turned
		// off on either member (owner 2026-11-09), so the caller budgets for it
		// (docs/design/autoradio-desk-fast-turns-2026-11-11.md section 5).
		AutoradioPool: {
			{Provider: "river", Model: "deepseek-ai/DeepSeek-V4.1-Flash-1017", Weight: 1},
			{Provider: "bedrock", Model: "us.anthropic.claude-haiku-5-5", FallbackOnly: true},
		},
		SyntheticLunaPool: {
			{Provider: "synthetic", Model: "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017", Weight: 1},
			{Provider: "chatgpt", Model: "gpt-6-luna", Weight: 1},
		},
		LunaSyntheticPool: {
			{Provider: "chatgpt", Model: "gpt-6-luna", Weight: 1},
			{Provider: "synthetic", Model: "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017", FallbackOnly: true},
		},
		SelfDevPool: {
			// Owner 2026-11-09: "use deepseek via Synthetic and River for
			// self-dev". Synthetic's DeepSeek leads and takes the threads;
			// River's DeepSeek is the fallback. No free members and no Devin
			// here (this replaces the earlier free-first selfdev pool). River
			// carries a $10 a day budget (owner 2026-11-10: "$1/day cap can be lifted";
			// the gateway's paid budget, paid_budget.go).
			{Provider: "synthetic", Model: "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017", Weight: 1},
			{Provider: "river", Model: "deepseek-ai/DeepSeek-V4.1-Flash-1017", FallbackOnly: true},
		},
	}
}

// AutoradioPool names the pool of the Autoradio desk's spoken answers: River's
// DeepSeek V4.1 Flash 1017 leads and Bedrock Haiku 5.5 is the only fallback.
const AutoradioPool = "autoradio"

// LunaSyntheticPool names the pool that runs every thread on gpt-6-luna and
// falls back to Synthetic's DeepSeek V4.1 Flash 1017 only when luna fails.
const LunaSyntheticPool = "luna-synthetic"

// SyntheticLunaPool names the pool that splits threads between Synthetic's
// DeepSeek V4.1 Flash 1017 and gpt-6-luna.
const SyntheticLunaPool = "synthetic-luna"

// SelfDevPool names the self-development pool (owner 2026-11-09: "use
// deepseek via Synthetic and River for self-dev"): Synthetic's DeepSeek V4.1
// Flash 1017 leads and takes the threads, River's DeepSeek is the only
// fallback (River's gateway budget is $10 a day, owner 2026-11-10: "$1/day cap can be lifted"). No free members, no Devin, no Haiku.
const SelfDevPool = "selfdev"

// NoFailover reports whether a pool's threads never move to another member
// when their model fails: the member that fails, fails. The default rule is
// the next member in order (docs/model-pools-design-2026-10-12.md §5).
func NoFailover(pool string) bool {
	return strings.TrimSpace(pool) == SyntheticLunaPool
}

// Lookup returns the catalog entry for (provider, model).
func Lookup(provider, modelID string) (ModelInfo, bool) {
	provider, modelID = strings.TrimSpace(provider), strings.TrimSpace(modelID)
	for _, model := range SupportedModels() {
		if model.Provider == provider && model.ID == modelID {
			return model, true
		}
	}
	return ModelInfo{}, false
}

// ProviderCatalogued reports whether the catalog lists any model for
// provider. Providers it does not list are not validated by model.
func ProviderCatalogued(provider string) bool {
	provider = strings.TrimSpace(provider)
	for _, model := range SupportedModels() {
		if model.Provider == provider {
			return true
		}
	}
	return false
}

// MaxOutputTokensForModel returns the maximum output tokens for a model ID.
// Falls back to DefaultMaxOutputTokens if the model is not found in the catalog.
func MaxOutputTokensForModel(modelID string) int {
	modelID = strings.TrimSpace(modelID)
	for _, model := range SupportedModels() {
		if model.ID == modelID && model.MaxOutputTokens > 0 {
			return model.MaxOutputTokens
		}
	}
	return DefaultMaxOutputTokens
}

// MaxOutputTokensKnown returns the catalog's maximum output tokens for a
// model ID and whether the catalog lists one.
func MaxOutputTokensKnown(modelID string) (int, bool) {
	modelID = strings.TrimSpace(modelID)
	for _, model := range SupportedModels() {
		if model.ID == modelID && model.MaxOutputTokens > 0 {
			return model.MaxOutputTokens, true
		}
	}
	return 0, false
}

// ContextWindowTokensForModel returns the advertised context window size for a
// model ID. Falls back to DefaultContextWindowTokens if the model is not found
// or does not advertise a window.
func ContextWindowTokensForModel(modelID string) int {
	modelID = strings.TrimSpace(modelID)
	for _, model := range SupportedModels() {
		if model.ID == modelID && model.ContextWindowTokens > 0 {
			return model.ContextWindowTokens
		}
	}
	return DefaultContextWindowTokens
}
