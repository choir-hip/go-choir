package provideriface

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigReadsRuntimeEnv(t *testing.T) {
	t.Setenv("RUNTIME_PROVIDER_TIMEOUT", "3s")
	t.Setenv("RUNTIME_ACTIVATION_BUDGET", "90s")
	t.Setenv("RUNTIME_SKILLS_ROOT", "/tmp/choir-skills")
	t.Setenv("RUNTIME_TEXTURE_ACTOR_PARK_IDLE", "45s")

	cfg := LoadConfig()
	if cfg.TextureActorParkIdle != 45*time.Second {
		t.Fatalf("texture_actor_park_idle = %s, want 45s", cfg.TextureActorParkIdle)
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

func TestLoadConfigReadsEnableTestAPIs(t *testing.T) {
	t.Setenv("RUNTIME_ENABLE_TEST_APIS", "true")

	cfg := LoadConfig()
	if !cfg.EnableTestAPIs {
		t.Fatal("enable_test_apis = false, want true")
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
		QdrantDedupThreshold: 0,
	})

	if cfg.PromptRoot != filepath.Join(filepath.Dir(storePath), "prompts") {
		t.Fatalf("prompt_root = %q", cfg.PromptRoot)
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

func TestResolveFilesRootPrecedence(t *testing.T) {
	t.Setenv("AUTOPUTER_FILES_ROOT", "/environment/files")
	if got := ResolveFilesRoot("/explicit/files"); got != "/explicit/files" {
		t.Fatalf("explicit files root = %q, want %q", got, "/explicit/files")
	}
	if got := ResolveFilesRoot(""); got != "/environment/files" {
		t.Fatalf("environment files root = %q, want %q", got, "/environment/files")
	}

	t.Setenv("AUTOPUTER_FILES_ROOT", "")
	if got := ResolveFilesRoot(""); got != DefaultFilesRoot {
		t.Fatalf("default files root = %q, want %q", got, DefaultFilesRoot)
	}
}
