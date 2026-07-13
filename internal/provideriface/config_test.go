package provideriface

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigDefaultsResearcherCount(t *testing.T) {
	t.Setenv("SANDBOX_ID", "")
	t.Setenv("RUNTIME_STORE_PATH", "")
	t.Setenv("RUNTIME_PROVIDER_TIMEOUT", "")
	t.Setenv("RUNTIME_SUPERVISION_INTERVAL", "")
	t.Setenv("RUNTIME_ACTIVATION_BUDGET", "")
	t.Setenv("RUNTIME_RESEARCHER_COUNT", "")
	t.Setenv("RUNTIME_TEXTURE_ACTOR_PARK_IDLE", "")

	cfg := LoadConfig()
	if cfg.ResearcherCount != DefaultResearcherCount {
		t.Fatalf("researcher_count = %d, want %d", cfg.ResearcherCount, DefaultResearcherCount)
	}
	if cfg.ActivationBudget != DefaultActivationBudget {
		t.Fatalf("activation_budget = %s, want %s", cfg.ActivationBudget, DefaultActivationBudget)
	}
	if cfg.TextureActorParkIdle != DefaultTextureActorParkIdle {
		t.Fatalf("texture_actor_park_idle = %s, want %s", cfg.TextureActorParkIdle, DefaultTextureActorParkIdle)
	}
	if cfg.PromptRoot == "" {
		t.Fatal("prompt_root should not be empty")
	}
}

func TestLoadConfigReadsResearcherCount(t *testing.T) {
	t.Setenv("RUNTIME_RESEARCHER_COUNT", "5")
	t.Setenv("RUNTIME_SUPERVISION_INTERVAL", "7s")
	t.Setenv("RUNTIME_PROVIDER_TIMEOUT", "3s")
	t.Setenv("RUNTIME_ACTIVATION_BUDGET", "90s")
	t.Setenv("RUNTIME_SKILLS_ROOT", "/tmp/choir-skills")
	t.Setenv("RUNTIME_TEXTURE_ACTOR_PARK_IDLE", "45s")

	cfg := LoadConfig()
	if cfg.ResearcherCount != 5 {
		t.Fatalf("researcher_count = %d, want 5", cfg.ResearcherCount)
	}
	if cfg.TextureActorParkIdle != 45*time.Second {
		t.Fatalf("texture_actor_park_idle = %s, want 45s", cfg.TextureActorParkIdle)
	}
	if cfg.SupervisionInterval != 7*time.Second {
		t.Fatalf("supervision interval = %s, want 7s", cfg.SupervisionInterval)
	}
	if cfg.ProviderTimeout != 3*time.Second {
		t.Fatalf("provider timeout = %s, want 3s", cfg.ProviderTimeout)
	}
	if cfg.ActivationBudget != 90*time.Second {
		t.Fatalf("activation_budget = %s, want 90s", cfg.ActivationBudget)
	}
	if cfg.PromptRoot == "" {
		t.Fatal("prompt_root should not be empty")
	}
	if cfg.SkillsRoot != "/tmp/choir-skills" {
		t.Fatalf("skills_root = %q, want env value", cfg.SkillsRoot)
	}
}

func TestLoadConfigFallsBackOnInvalidResearcherCount(t *testing.T) {
	_ = os.Setenv("RUNTIME_RESEARCHER_COUNT", "-2")
	t.Cleanup(func() { _ = os.Unsetenv("RUNTIME_RESEARCHER_COUNT") })

	cfg := LoadConfig()
	if cfg.ResearcherCount != DefaultResearcherCount {
		t.Fatalf("researcher_count = %d, want fallback %d", cfg.ResearcherCount, DefaultResearcherCount)
	}
}

func TestLoadConfigReadsEnableTestAPIs(t *testing.T) {
	t.Setenv("RUNTIME_ENABLE_TEST_APIS", "true")

	cfg := LoadConfig()
	if !cfg.EnableTestAPIs {
		t.Fatal("enable_test_apis = false, want true")
	}
}

func TestLoadConfigDefaultsPromotionSourceRepoOutsideGitWorktree(t *testing.T) {
	t.Setenv("RUNTIME_PROMOTION_SOURCE_REPO", "")
	t.Setenv("RUNTIME_WORKER_REPO_REMOTE", "")
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})

	cfg := LoadConfig()
	if cfg.PromotionSourceRepo != DefaultPromotionSourceRepo {
		t.Fatalf("promotion source repo = %q, want %q", cfg.PromotionSourceRepo, DefaultPromotionSourceRepo)
	}
}

func TestLoadConfigReadsObscuraCDPScreenshots(t *testing.T) {
	t.Setenv("CHOIR_OBSCURA_CDP_SCREENSHOTS", "true")

	cfg := LoadConfig()
	if !cfg.ObscuraCDPScreenshots {
		t.Fatal("obscura_cdp_screenshots = false, want true")
	}
}

func TestNormalizeConfigPreservesExplicitZeroAndDerivesDefaults(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "runtime.db")
	cfg := NormalizeConfig(Config{
		StorePath:            storePath,
		PromotionSourceRepo:  "https://example.com/source.git",
		QdrantDedupThreshold: 0,
	})

	if cfg.PromptRoot != filepath.Join(filepath.Dir(storePath), "prompts") {
		t.Fatalf("prompt_root = %q", cfg.PromptRoot)
	}
	if cfg.PromotionWorkspaceRoot != filepath.Join(filepath.Dir(storePath), "promotion-workspaces") {
		t.Fatalf("promotion_workspace_root = %q", cfg.PromotionWorkspaceRoot)
	}
	if cfg.ActivationBudget != DefaultActivationBudget {
		t.Fatalf("activation_budget = %s, want %s", cfg.ActivationBudget, DefaultActivationBudget)
	}
	if cfg.QdrantDedupThreshold != 0 {
		t.Fatalf("qdrant_dedup_threshold = %f, want explicit zero", cfg.QdrantDedupThreshold)
	}
}

func TestLoadConfigPreservesExplicitZeroDedupThreshold(t *testing.T) {
	t.Setenv("QDRANT_DEDUP_THRESHOLD", "0")

	cfg := LoadConfig()
	if cfg.QdrantDedupThreshold != 0 {
		t.Fatalf("qdrant_dedup_threshold = %f, want explicit zero", cfg.QdrantDedupThreshold)
	}
}

func TestDefaultModelPolicyPath(t *testing.T) {
	if got := DefaultModelPolicyPath(" \t "); got != "" {
		t.Fatalf("empty root path = %q, want empty", got)
	}
	root := filepath.Join(t.TempDir(), "Files")
	if got, want := DefaultModelPolicyPath("  "+root+"  "), filepath.Join(root, "System", "model-policy.toml"); got != want {
		t.Fatalf("model policy path = %q, want %q", got, want)
	}
}

func TestResolveFilesRootPrecedence(t *testing.T) {
	t.Setenv("SANDBOX_FILES_ROOT", "/environment/files")
	if got := ResolveFilesRoot("/explicit/files"); got != "/explicit/files" {
		t.Fatalf("explicit files root = %q, want %q", got, "/explicit/files")
	}
	if got := ResolveFilesRoot(""); got != "/environment/files" {
		t.Fatalf("environment files root = %q, want %q", got, "/environment/files")
	}

	t.Setenv("SANDBOX_FILES_ROOT", "")
	if got := ResolveFilesRoot(""); got != DefaultFilesRoot {
		t.Fatalf("default files root = %q, want %q", got, DefaultFilesRoot)
	}
}
