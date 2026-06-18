package runtime

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfigDefaultsResearcherCount(t *testing.T) {
	t.Setenv("SANDBOX_ID", "")
	t.Setenv("RUNTIME_STORE_PATH", "")
	t.Setenv("RUNTIME_PROVIDER_TIMEOUT", "")
	t.Setenv("RUNTIME_SUPERVISION_INTERVAL", "")
	t.Setenv("RUNTIME_RESEARCHER_COUNT", "")
	t.Setenv("RUNTIME_TEXTURE_ACTOR_PARK_IDLE", "")

	cfg := LoadConfig()
	if cfg.ResearcherCount != DefaultResearcherCount {
		t.Fatalf("researcher_count = %d, want %d", cfg.ResearcherCount, DefaultResearcherCount)
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
