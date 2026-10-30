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

	// DefaultFilesRoot is the local-development root for autoputer-owned files.
	DefaultFilesRoot = "/tmp/go-choir-files"

	// DefaultProviderTimeout is how long the stub provider simulates work.
	DefaultProviderTimeout = 2 * time.Second

	// DefaultActivationBudget bounds one resident activation. Expiry is a
	// terminal progress deadline, not a retry or supervision policy.
	DefaultActivationBudget = 60 * time.Minute

	// DefaultTextureActorParkIdle is how long a Texture revision actor may park
	// after an idle turn before passivating into a sleeping durable actor. The
	// parked interval is long enough to catch live researcher/super packets
	// without a cold wake run while still bounding active provider-loop residency.
	DefaultTextureActorParkIdle = 2 * time.Minute

	// DefaultTextureRootDigestInterval is the Texture root digest's debounce
	// and the least spacing of digest-started root runs (PD3 S1).
	DefaultTextureRootDigestInterval = 120 * time.Second

	// DefaultTextureRootDigestPerHour bounds digest-started root runs in any
	// hour (PD3 S1).
	DefaultTextureRootDigestPerHour = 20

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

	// DefaultCorpusdURL is the default corpusd/proxy URL used when
	// neither RUNTIME_CORPUSD_URL nor PROXY_CORPUSD_URL is set. In
	// deployed VMs this is the proxy port (8082); in local dev it's the
	// same. The proxy routes /internal/platform/objects and
	// /internal/platform/edges to corpusd.
	DefaultCorpusdURL = "http://127.0.0.1:8082"

	// DefaultManagementTaskSlots is K (management delegation O7, S2b): four
	// tasks at once, as research children and intake labels. Each task is
	// a desk worker (about 17 MiB) and shares the sub-RLM call spacer.
	DefaultManagementTaskSlots = 4
	// MaxManagementTaskSlots caps K for guest memory and the gateway's
	// per-computer call budget.
	MaxManagementTaskSlots = 8
)

// ResolveFilesRoot returns the effective files root. It prefers the explicit
// argument, then AUTOPUTER_FILES_ROOT, then the local-development default.
func ResolveFilesRoot(rootDir string) string {
	if rootDir == "" {
		rootDir = os.Getenv("AUTOPUTER_FILES_ROOT")
	}
	if rootDir == "" {
		rootDir = DefaultFilesRoot
	}
	return rootDir
}

// DefaultModelPolicyPath derives the computer-owned model policy path.
func DefaultModelPolicyPath(filesRoot string) string {
	root := strings.TrimSpace(filesRoot)
	if root == "" {
		return ""
	}
	return filepath.Join(root, filepath.FromSlash("System/model-policy.toml"))
}

// LoadConfig resolves runtime configuration from environment variables.
func LoadConfig() Config {
	storePath := envOr("RUNTIME_STORE_PATH", DefaultStorePath)
	return NormalizeConfig(Config{
		ComputerID:       envOr("AUTOPUTER_ID", "autoputer-dev"),
		StorePath:        storePath,
		PromptRoot:       envOr("RUNTIME_PROMPT_ROOT", defaultPromptRoot(storePath)),
		SkillsRoot:       envOr("RUNTIME_SKILLS_ROOT", defaultSkillsRoot()),
		ProviderTimeout:  durationOr("RUNTIME_PROVIDER_TIMEOUT", DefaultProviderTimeout),
		ActivationBudget: durationOr("RUNTIME_ACTIVATION_BUDGET", DefaultActivationBudget),
		TextureActorParkIdle: durationOr(
			"RUNTIME_TEXTURE_ACTOR_PARK_IDLE",
			DefaultTextureActorParkIdle,
		),
		TextureRootDigestInterval: durationOr(
			"RUNTIME_TEXTURE_ROOT_DIGEST_INTERVAL",
			DefaultTextureRootDigestInterval,
		),
		TextureRootDigestPerHour: intOr(
			"RUNTIME_TEXTURE_ROOT_DIGEST_PER_HOUR",
			DefaultTextureRootDigestPerHour,
		),
		VmctlURL:           envOr("RUNTIME_VMCTL_URL", os.Getenv("PROXY_VMCTL_URL")),
		MaildURL:           os.Getenv("RUNTIME_MAILD_URL"),
		CorpusdURL:         envOr("CHOIR_PLATFORM_URL", envOr("RUNTIME_CORPUSD_URL", envOr("PROXY_CORPUSD_URL", DefaultCorpusdURL))),
		ProxyURL:           os.Getenv("CHOIR_PROXY_URL"),
		LLMProvider:        os.Getenv("RUNTIME_LLM_PROVIDER"),
		LLMModel:           os.Getenv("RUNTIME_LLM_MODEL"),
		LLMReasoningEffort: os.Getenv("RUNTIME_LLM_REASONING_EFFORT"),
		ModelPolicyPath:    os.Getenv("RUNTIME_MODEL_POLICY_PATH"),
		EnableTestAPIs:     boolOr("RUNTIME_ENABLE_TEST_APIS", false),
		RunMemoryContextThresholdTokens: intOr(
			"RUNTIME_RUN_MEMORY_CONTEXT_THRESHOLD_TOKENS",
			DefaultRunMemoryContextThresholdTokens,
		),
		RunMemoryKeepRecentTokens: intOr(
			"RUNTIME_RUN_MEMORY_KEEP_RECENT_TOKENS",
			DefaultRunMemoryKeepRecentTokens,
		),
		ManagementTaskSlots: intOr("RUNTIME_MANAGEMENT_TASK_SLOTS", DefaultManagementTaskSlots),
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
	if cfg.RunMemoryContextThresholdTokens <= 0 {
		cfg.RunMemoryContextThresholdTokens = DefaultRunMemoryContextThresholdTokens
	}
	if cfg.RunMemoryKeepRecentTokens <= 0 {
		cfg.RunMemoryKeepRecentTokens = DefaultRunMemoryKeepRecentTokens
	}
	cfg.ManagementTaskSlots = NormalizeManagementTaskSlots(cfg.ManagementTaskSlots)
	return cfg
}

// NormalizeManagementTaskSlots bounds K: unset or below 1 is the default,
// above the cap is the cap.
func NormalizeManagementTaskSlots(k int) int {
	switch {
	case k < 1:
		return DefaultManagementTaskSlots
	case k > MaxManagementTaskSlots:
		return MaxManagementTaskSlots
	}
	return k
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
	for _, name := range []string{"cognitive-transform-portfolio"} {
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

// GatewayToken resolves the per-computer autoputer credential used to
// authenticate to the host-side gateway. In guest images the token is
// delivered on the root-only bootstrap credential disk and referenced by
// RUNTIME_GATEWAY_TOKEN_FILE (readable by the autoputer, not by non-root
// children); RUNTIME_GATEWAY_TOKEN remains the fallback for dev hosts and
// older guests. File first so a dev env never shadows the delivered file.
func GatewayToken() string {
	if path := strings.TrimSpace(os.Getenv("RUNTIME_GATEWAY_TOKEN_FILE")); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			if token := strings.TrimSpace(string(data)); token != "" {
				return token
			}
		}
	}
	return strings.TrimSpace(os.Getenv("RUNTIME_GATEWAY_TOKEN"))
}
