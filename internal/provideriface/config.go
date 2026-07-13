package provideriface

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultStorePath is the local marker path used to derive the embedded Dolt
	// workspace for runtime state. Retired SQLite content is never imported.
	DefaultStorePath = "/tmp/go-choir-m3/runtime.db"

	// DefaultProviderTimeout is how long the stub provider simulates work.
	DefaultProviderTimeout = 2 * time.Second

	// DefaultActivationBudget bounds one resident activation. Expiry is a
	// terminal progress deadline, not a retry or supervision policy.
	DefaultActivationBudget = 60 * time.Minute

	// DefaultResearcherCount is the default number of researcher workers
	// the microVM topology should assume when none is configured.
	DefaultResearcherCount = 3

	// DefaultTextureWakeDebounce is the initial coalescing window for worker
	// findings before the runtime schedules the next texture synthesis.
	DefaultTextureWakeDebounce = 3 * time.Second

	// DefaultTextureActorParkIdle is how long a Texture revision actor may park
	// after an idle turn before passivating into a sleeping durable actor. The
	// parked interval is long enough to catch live researcher/super packets
	// without a cold wake run while still bounding active provider-loop residency.
	DefaultTextureActorParkIdle = 2 * time.Minute

	// DefaultRunMemoryContextThresholdTokens is zero so normal runtime
	// compaction derives from the selected model's context window. Set
	// RUNTIME_RUN_MEMORY_CONTEXT_THRESHOLD_TOKENS only for explicit diagnostics.
	DefaultRunMemoryContextThresholdTokens = 0

	// DefaultRunMemoryContextThresholdRatio is the fraction of a selected model's
	// context window where automatic run-memory compaction starts.
	DefaultRunMemoryContextThresholdRatio = 0.70

	// DefaultRunMemoryPromptReserveTokens approximates system/tool/output slack
	// until provider-tokenizer accounting is available in the runtime.
	DefaultRunMemoryPromptReserveTokens = 12000

	// DefaultRunMemoryKeepRecentTokens is the approximate amount of recent raw
	// conversation retained after a compaction checkpoint.
	DefaultRunMemoryKeepRecentTokens = 20000

	// DefaultPromotionSourceRepo is the canonical platform source repository
	// used when an existing user computer predates promotion env wiring.
	DefaultPromotionSourceRepo = "https://github.com/yusefmosiah/go-choir.git"

	// DefaultSourceLedgerRepo is the private source-lineage remote used for
	// product-visible computer/package source refs. Runtime clones for
	// integration builds still use PromotionSourceRepo unless explicitly
	// configured otherwise.
	DefaultSourceLedgerRepo = "https://github.com/yusefmosiah/choir-source-ledger.git"

	DefaultAppPromotionRuntimeBuildCommand = "mkdir -p .choir-promotion-artifacts/runtime && GOMAXPROCS=1 GOMEMLIMIT=1024MiB GOFLAGS=-p=1 go build -p 1 -o .choir-promotion-artifacts/runtime/sandbox ./cmd/sandbox"
	DefaultAppPromotionRuntimeArtifactPath = ".choir-promotion-artifacts/runtime/sandbox"
	DefaultAppPromotionUIBuildCommand      = "npm --prefix frontend ci --no-audit --no-fund && NODE_OPTIONS=--max-old-space-size=768 npm --prefix frontend run build"
	DefaultAppPromotionUIArtifactPath      = "frontend/dist"
	DefaultAppPromotionBuildTimeout        = 15 * time.Minute

	// DefaultQdrantURL is the node-b Qdrant instance URL.
	DefaultQdrantURL = "http://127.0.0.1:6333"

	// DefaultCorpusdURL is the default corpusd/proxy URL used when
	// neither RUNTIME_CORPUSD_URL nor PROXY_CORPUSD_URL is set. In
	// deployed VMs this is the proxy port (8082); in local dev it's the
	// same. The proxy routes /internal/platform/objects and
	// /internal/platform/edges to corpusd.
	DefaultCorpusdURL = "http://127.0.0.1:8082"

	// DefaultOllamaURL is the local Ollama instance URL for embeddings.
	DefaultOllamaURL = "http://localhost:11434"

	// DefaultOllamaEmbeddingModel is the default embedding model for Qdrant
	// semantic routing.
	DefaultOllamaEmbeddingModel = "batiai/qwen3-embedding:0.6b"

	// DefaultQdrantDedupThreshold is the cosine similarity score at which a
	// newly ingested capture is treated as a semantic duplicate of an
	// existing capture and dropped before processor dispatch. Spike 5
	// measured ~0.7862 as the clean-separation midpoint for 25 headlines;
	// it is a tunable baseline, not a universal constant.
	DefaultQdrantDedupThreshold = 0.7862
)

// LoadConfig resolves runtime configuration from environment variables.
func LoadConfig() Config {
	storePath := envOr("RUNTIME_STORE_PATH", DefaultStorePath)
	return NormalizeConfig(Config{
		SandboxID:           envOr("SANDBOX_ID", "sandbox-dev"),
		StorePath:           storePath,
		PromptRoot:          envOr("RUNTIME_PROMPT_ROOT", defaultPromptRoot(storePath)),
		SkillsRoot:          envOr("RUNTIME_SKILLS_ROOT", defaultSkillsRoot()),
		ProviderTimeout:     durationOr("RUNTIME_PROVIDER_TIMEOUT", DefaultProviderTimeout),
		ActivationBudget:    durationOr("RUNTIME_ACTIVATION_BUDGET", DefaultActivationBudget),
		SupervisionInterval: durationOr("RUNTIME_SUPERVISION_INTERVAL", 5*time.Second),
		ResearcherCount:     intOr("RUNTIME_RESEARCHER_COUNT", DefaultResearcherCount),
		TextureWakeDebounce: durationOr("RUNTIME_TEXTURE_WAKE_DEBOUNCE", DefaultTextureWakeDebounce),
		TextureActorParkIdle: durationOr(
			"RUNTIME_TEXTURE_ACTOR_PARK_IDLE",
			DefaultTextureActorParkIdle,
		),
		VmctlURL:           envOr("RUNTIME_VMCTL_URL", os.Getenv("PROXY_VMCTL_URL")),
		MaildURL:           os.Getenv("RUNTIME_MAILD_URL"),
		WirePublishURL:     os.Getenv("RUNTIME_WIRE_PUBLISH_URL"),
		CorpusdURL:         envOr("RUNTIME_CORPUSD_URL", envOr("PROXY_CORPUSD_URL", DefaultCorpusdURL)),
		LLMProvider:        os.Getenv("RUNTIME_LLM_PROVIDER"),
		LLMModel:           os.Getenv("RUNTIME_LLM_MODEL"),
		LLMReasoningEffort: os.Getenv("RUNTIME_LLM_REASONING_EFFORT"),
		ModelPolicyPath:    os.Getenv("RUNTIME_MODEL_POLICY_PATH"),
		ObscuraPath:        envOr("CHOIR_OBSCURA_BIN", os.Getenv("OBSCURA_BIN")),
		ObscuraCDPScreenshots: boolOr(
			"CHOIR_OBSCURA_CDP_SCREENSHOTS",
			boolOr("OBSCURA_CDP_SCREENSHOTS", false),
		),
		EnableTestAPIs: boolOr("RUNTIME_ENABLE_TEST_APIS", false),
		RunMemoryContextThresholdTokens: intOr(
			"RUNTIME_RUN_MEMORY_CONTEXT_THRESHOLD_TOKENS",
			DefaultRunMemoryContextThresholdTokens,
		),
		RunMemoryKeepRecentTokens: intOr(
			"RUNTIME_RUN_MEMORY_KEEP_RECENT_TOKENS",
			DefaultRunMemoryKeepRecentTokens,
		),
		PromotionSourceRepo: envOr(
			"RUNTIME_PROMOTION_SOURCE_REPO",
			os.Getenv("RUNTIME_WORKER_REPO_REMOTE"),
		),
		SourceLedgerRepo:       envOr("RUNTIME_SOURCE_LEDGER_REPO", DefaultSourceLedgerRepo),
		PromotionWorkspaceRoot: os.Getenv("RUNTIME_PROMOTION_WORKSPACE_ROOT"),
		AppPromotionRuntimeBuildCommand: envOr(
			"RUNTIME_APP_PROMOTION_RUNTIME_BUILD_COMMAND",
			DefaultAppPromotionRuntimeBuildCommand,
		),
		AppPromotionRuntimeArtifactPath: envOr(
			"RUNTIME_APP_PROMOTION_RUNTIME_ARTIFACT_PATH",
			DefaultAppPromotionRuntimeArtifactPath,
		),
		AppPromotionUIBuildCommand: envOr(
			"RUNTIME_APP_PROMOTION_UI_BUILD_COMMAND",
			DefaultAppPromotionUIBuildCommand,
		),
		AppPromotionUIArtifactPath: envOr(
			"RUNTIME_APP_PROMOTION_UI_ARTIFACT_PATH",
			DefaultAppPromotionUIArtifactPath,
		),
		AppPromotionBuildTimeout: durationOr(
			"RUNTIME_APP_PROMOTION_BUILD_TIMEOUT",
			DefaultAppPromotionBuildTimeout,
		),
		QdrantURL:               envOr("QDRANT_URL", DefaultQdrantURL),
		OllamaURL:               envOr("OLLAMA_URL", DefaultOllamaURL),
		OllamaEmbeddingModel:    envOr("OLLAMA_EMBEDDING_MODEL", DefaultOllamaEmbeddingModel),
		QdrantDedupThreshold:    float32Or("QDRANT_DEDUP_THRESHOLD", DefaultQdrantDedupThreshold),
		TracePersistenceEnabled: boolOr("RUNTIME_TRACE_PERSISTENCE_ENABLED", false),
	})
}

// NormalizeConfig applies runtime defaults to a directly constructed Config.
func NormalizeConfig(cfg Config) Config {
	if strings.TrimSpace(cfg.StorePath) == "" {
		cfg.StorePath = DefaultStorePath
	}
	if strings.TrimSpace(cfg.PromptRoot) == "" {
		cfg.PromptRoot = defaultPromptRoot(cfg.StorePath)
	}
	if cfg.ActivationBudget <= 0 {
		cfg.ActivationBudget = DefaultActivationBudget
	}
	if cfg.TextureWakeDebounce <= 0 {
		cfg.TextureWakeDebounce = DefaultTextureWakeDebounce
	}
	if cfg.RunMemoryContextThresholdTokens <= 0 {
		cfg.RunMemoryContextThresholdTokens = DefaultRunMemoryContextThresholdTokens
	}
	if cfg.RunMemoryKeepRecentTokens <= 0 {
		cfg.RunMemoryKeepRecentTokens = DefaultRunMemoryKeepRecentTokens
	}
	if strings.TrimSpace(cfg.PromotionWorkspaceRoot) == "" {
		cfg.PromotionWorkspaceRoot = filepath.Join(filepath.Dir(cfg.StorePath), "promotion-workspaces")
	}
	if strings.TrimSpace(cfg.SourceLedgerRepo) == "" {
		cfg.SourceLedgerRepo = DefaultSourceLedgerRepo
	}
	if strings.TrimSpace(cfg.PromotionSourceRepo) == "" {
		if wd, err := os.Getwd(); err == nil {
			if _, statErr := os.Stat(filepath.Join(wd, ".git")); statErr == nil {
				cfg.PromotionSourceRepo = wd
			}
		}
	}
	if strings.TrimSpace(cfg.PromotionSourceRepo) == "" {
		cfg.PromotionSourceRepo = DefaultPromotionSourceRepo
	}
	if strings.TrimSpace(cfg.AppPromotionRuntimeBuildCommand) == "" {
		cfg.AppPromotionRuntimeBuildCommand = DefaultAppPromotionRuntimeBuildCommand
	}
	if strings.TrimSpace(cfg.AppPromotionRuntimeArtifactPath) == "" {
		cfg.AppPromotionRuntimeArtifactPath = DefaultAppPromotionRuntimeArtifactPath
	}
	if strings.TrimSpace(cfg.AppPromotionUIBuildCommand) == "" {
		cfg.AppPromotionUIBuildCommand = DefaultAppPromotionUIBuildCommand
	}
	if strings.TrimSpace(cfg.AppPromotionUIArtifactPath) == "" {
		cfg.AppPromotionUIArtifactPath = DefaultAppPromotionUIArtifactPath
	}
	if cfg.AppPromotionBuildTimeout <= 0 {
		cfg.AppPromotionBuildTimeout = DefaultAppPromotionBuildTimeout
	}
	if strings.TrimSpace(cfg.QdrantURL) == "" {
		cfg.QdrantURL = DefaultQdrantURL
	}
	if strings.TrimSpace(cfg.OllamaURL) == "" {
		cfg.OllamaURL = DefaultOllamaURL
	}
	if strings.TrimSpace(cfg.OllamaEmbeddingModel) == "" {
		cfg.OllamaEmbeddingModel = DefaultOllamaEmbeddingModel
	}
	// QdrantDedupThreshold is intentionally not normalized here: a zero value
	// means semantic dedup is disabled (the default for test configs that do
	// not opt in). LoadConfig applies the production default via envOr when
	// QDRANT_DEDUP_THRESHOLD is unset; an explicit "0" disables the pass.
	return cfg
}

func defaultPromptRoot(storePath string) string {
	if strings.TrimSpace(storePath) == "" {
		storePath = DefaultStorePath
	}
	return filepath.Join(filepath.Dir(storePath), "prompts")
}

func defaultSkillsRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for dir := wd; dir != ""; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "skills")
		if hasRuntimeSkillFiles(candidate) {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return ""
}

func hasRuntimeSkillFiles(root string) bool {
	if strings.TrimSpace(root) == "" {
		return false
	}
	for _, name := range []string{"mission-gradient", "cognitive-transform-portfolio"} {
		if _, err := os.Stat(filepath.Join(root, name, "SKILL.md")); err != nil {
			return false
		}
	}
	return true
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func durationOr(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func intOr(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return fallback
	}
	return n
}

func boolOr(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func float32Or(key string, fallback float32) float32 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseFloat(v, 32)
	if err != nil {
		return fallback
	}
	return float32(n)
}
