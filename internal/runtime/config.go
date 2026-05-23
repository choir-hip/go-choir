// Package runtime provides the host-process runtime engine for the go-choir
// sandbox. It manages task lifecycle, event emission, health state, and the
// HTTP API surface consumed through the authenticated proxy.
//
// Design decisions:
//   - Runs execute as direct goroutines, not subprocess CLI loops or
//     adapter-wrapper processes.
//   - Provider is an interface; the stub provider simulates execution until
//     the Bedrock/Z.AI bridge feature replaces it with a real provider.
//   - Product state is persisted through the store package's embedded Dolt
//     workspace so task handles and events survive sandbox process restarts.
//   - Health, degradation, and recovery are externally visible through the
//     /health endpoint and event stream.
package runtime

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultStorePath is the local marker/legacy-import path used to derive the
	// embedded Dolt workspace for runtime state.
	DefaultStorePath = "/tmp/go-choir-m3/runtime.db"

	// DefaultProviderTimeout is how long the stub provider simulates work.
	DefaultProviderTimeout = 2 * time.Second

	// DefaultSupervisionInterval is a legacy reserved setting kept only to avoid
	// churning tests and config plumbing during the runtime cleanup.
	DefaultSupervisionInterval = 5 * time.Second

	// DefaultResearcherCount is the default number of researcher workers
	// the microVM topology should assume when none is configured.
	DefaultResearcherCount = 3

	// DefaultVTextWakeDebounce is the initial coalescing window for worker
	// findings before the runtime schedules the next vtext synthesis.
	DefaultVTextWakeDebounce = 3 * time.Second

	// DefaultRunMemoryContextThresholdTokens is the approximate context size at
	// which the runtime creates a run-memory compaction checkpoint.
	DefaultRunMemoryContextThresholdTokens = 160000

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
)

// Config holds runtime configuration resolved from environment variables.
type Config struct {
	// SandboxID is the stable identity of this sandbox instance.
	SandboxID string

	// StorePath is the marker/legacy-import path used to derive the embedded
	// Dolt workspace for task/event persistence.
	StorePath string

	// PromptRoot is the sandbox-owned filesystem root for editable role prompts.
	PromptRoot string

	// SkillsRoot is the repo-owned filesystem root for natural-language skills
	// that should be summarized into selected agent prompts.
	SkillsRoot string

	// ProviderTimeout is the simulated work duration for the stub provider.
	ProviderTimeout time.Duration

	// SupervisionInterval is legacy reserved configuration. The old polling
	// supervisor has been deleted and this value is currently unused.
	SupervisionInterval time.Duration

	// ResearcherCount is the configured researcher worker count for this VM.
	ResearcherCount int

	// VTextWakeDebounce is the coalescing window for addressed worker findings
	// before the runtime schedules the next vtext synthesis.
	VTextWakeDebounce time.Duration

	// VmctlURL is the host-side vmctl control plane URL, used by super-only
	// lifecycle tools to request branch desktops and worker VMs.
	VmctlURL string

	// LLMProvider is the explicitly selected provider for runtime LLM calls.
	// Empty means no provider is selected by this runtime config.
	LLMProvider string

	// LLMModel is the explicitly selected model for runtime LLM calls.
	LLMModel string

	// LLMReasoningEffort is the provider-specific reasoning effort for runtime
	// LLM calls.
	LLMReasoningEffort string

	// ModelPolicyPath is the computer-owned editable text file that maps agent
	// roles to provider/model/reasoning selections. Environment LLM settings
	// remain the platform fallback; this file is the user-computer policy.
	ModelPolicyPath string

	// ObscuraPath is an optional path or executable name for the backend browser
	// provider. Empty means the backend browser substrate is not configured.
	ObscuraPath string

	// ObscuraCDPScreenshots enables the opt-in CDP screenshot substrate for the
	// backend Browser app. The default remains CLI snapshot extraction only.
	ObscuraCDPScreenshots bool

	// EnableTestAPIs exposes local-only browser test hooks. These endpoints are
	// disabled by default and should never be enabled on deployed environments.
	EnableTestAPIs bool

	// RunMemoryContextThresholdTokens controls automatic context compaction for
	// tool-loop runs. The estimator is intentionally approximate.
	RunMemoryContextThresholdTokens int

	// RunMemoryKeepRecentTokens controls how much recent raw context is kept
	// alongside each run-memory compaction checkpoint.
	RunMemoryKeepRecentTokens int

	// PromotionSourceRepo is the server-owned canonical go-choir source used
	// to create product-safe promotion integration workspaces.
	PromotionSourceRepo string

	// SourceLedgerRepo is the product-visible source lineage remote. It may be
	// private; package/adoption records name refs in this ledger.
	SourceLedgerRepo string

	// PromotionWorkspaceRoot is the server-owned root for per-candidate
	// integration workspaces. Browser callers never provide this path.
	PromotionWorkspaceRoot string

	AppPromotionRuntimeBuildCommand string
	AppPromotionRuntimeArtifactPath string
	AppPromotionUIBuildCommand      string
	AppPromotionUIArtifactPath      string
	AppPromotionBuildTimeout        time.Duration
}

// LoadConfig resolves runtime configuration from environment variables.
func LoadConfig() Config {
	storePath := envOr("RUNTIME_STORE_PATH", DefaultStorePath)
	return normalizeConfig(Config{
		SandboxID:           envOr("SANDBOX_ID", "sandbox-dev"),
		StorePath:           storePath,
		PromptRoot:          envOr("RUNTIME_PROMPT_ROOT", defaultPromptRoot(storePath)),
		SkillsRoot:          envOr("RUNTIME_SKILLS_ROOT", defaultSkillsRoot()),
		ProviderTimeout:     durationOr("RUNTIME_PROVIDER_TIMEOUT", DefaultProviderTimeout),
		SupervisionInterval: durationOr("RUNTIME_SUPERVISION_INTERVAL", DefaultSupervisionInterval),
		ResearcherCount:     intOr("RUNTIME_RESEARCHER_COUNT", DefaultResearcherCount),
		VTextWakeDebounce:   durationOr("RUNTIME_VTEXT_WAKE_DEBOUNCE", DefaultVTextWakeDebounce),
		VmctlURL:            envOr("RUNTIME_VMCTL_URL", os.Getenv("PROXY_VMCTL_URL")),
		LLMProvider:         os.Getenv("RUNTIME_LLM_PROVIDER"),
		LLMModel:            os.Getenv("RUNTIME_LLM_MODEL"),
		LLMReasoningEffort:  os.Getenv("RUNTIME_LLM_REASONING_EFFORT"),
		ModelPolicyPath:     os.Getenv("RUNTIME_MODEL_POLICY_PATH"),
		ObscuraPath:         envOr("CHOIR_OBSCURA_BIN", os.Getenv("OBSCURA_BIN")),
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
	})
}

func normalizeConfig(cfg Config) Config {
	if strings.TrimSpace(cfg.StorePath) == "" {
		cfg.StorePath = DefaultStorePath
	}
	if strings.TrimSpace(cfg.PromptRoot) == "" {
		cfg.PromptRoot = defaultPromptRoot(cfg.StorePath)
	}
	if cfg.VTextWakeDebounce <= 0 {
		cfg.VTextWakeDebounce = DefaultVTextWakeDebounce
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
