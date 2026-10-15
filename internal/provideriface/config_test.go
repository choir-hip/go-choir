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

func TestNormalizeConfigDerivesDefaults(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "runtime.db")
	cfg := NormalizeConfig(Config{StorePath: storePath})

	if cfg.PromptRoot != filepath.Join(filepath.Dir(storePath), "prompts") {
		t.Fatalf("prompt_root = %q", cfg.PromptRoot)
	}
	if cfg.ActivationBudget != DefaultActivationBudget {
		t.Fatalf("activation_budget = %s, want %s", cfg.ActivationBudget, DefaultActivationBudget)
	}
}

// K, the management task slots (management delegation S2b, FM30): unset or
// below 1 is the default 4, a large value is capped, and the env sets it.
func TestManagementTaskSlotsDefaultEnvAndBounds(t *testing.T) {
	if DefaultManagementTaskSlots != 4 {
		t.Fatalf("DefaultManagementTaskSlots = %d, want 4 (O7)", DefaultManagementTaskSlots)
	}
	if got := NormalizeConfig(Config{}).ManagementTaskSlots; got != DefaultManagementTaskSlots {
		t.Fatalf("default slots = %d, want 4", got)
	}
	if got := NormalizeConfig(Config{ManagementTaskSlots: -2}).ManagementTaskSlots; got != DefaultManagementTaskSlots {
		t.Fatalf("negative slots = %d, want the default", got)
	}
	if got := NormalizeConfig(Config{ManagementTaskSlots: 50}).ManagementTaskSlots; got != MaxManagementTaskSlots {
		t.Fatalf("large slots = %d, want the cap %d", got, MaxManagementTaskSlots)
	}
	if got := NormalizeConfig(Config{ManagementTaskSlots: 1}).ManagementTaskSlots; got != 1 {
		t.Fatalf("explicit K = 1 = %d, want 1 (S2a's behaviour)", got)
	}
	t.Setenv("RUNTIME_MANAGEMENT_TASK_SLOTS", "2")
	if got := LoadConfig().ManagementTaskSlots; got != 2 {
		t.Fatalf("env slots = %d, want 2", got)
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
