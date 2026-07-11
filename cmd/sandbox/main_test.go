package main

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/runtime"
	"github.com/yusefmosiah/go-choir/internal/sandbox"
)

func TestWaitForObjectGraphBackfillDelayRequiresPostStepSignal(t *testing.T) {
	delay := make(chan time.Time, 1)
	result := make(chan bool, 1)
	go func() {
		result <- waitForObjectGraphBackfillDelay(t.Context(), delay)
	}()

	select {
	case got := <-result:
		t.Fatalf("delay returned before post-step signal: %t", got)
	default:
	}

	delay <- time.Now()
	if got := <-result; !got {
		t.Fatal("delay signal did not admit the next bounded step")
	}

	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if waitForObjectGraphBackfillDelay(cancelled, make(chan time.Time)) {
		t.Fatal("cancelled migration admitted another bounded step")
	}
}

func TestBuildRuntimeConfigPreservesHostServiceURLs(t *testing.T) {
	cfg := sandbox.Config{
		SandboxID: "vm-test",
		StorePath: "/tmp/runtime.db",
	}
	loaded := runtime.Config{
		PromptRoot:           "/prompts",
		SkillsRoot:           "/skills",
		ProviderTimeout:      7 * time.Second,
		SupervisionInterval:  3 * time.Second,
		ResearcherCount:      2,
		TextureWakeDebounce:  250 * time.Millisecond,
		TextureActorParkIdle: 45 * time.Second,
		VmctlURL:             "http://10.200.60.1:8083",
		MaildURL:             "http://10.200.60.1:8087",
		LLMProvider:          "fireworks",
		LLMModel:             "model",
		LLMReasoningEffort:   "low",
		ModelPolicyPath:      "/policy.toml",
	}

	got := buildRuntimeConfig(cfg, loaded, "/files")
	if got.SandboxID != cfg.SandboxID || got.StorePath != cfg.StorePath {
		t.Fatalf("sandbox identity/store not preserved: %+v", got)
	}
	if got.VmctlURL != loaded.VmctlURL {
		t.Fatalf("VmctlURL = %q, want %q", got.VmctlURL, loaded.VmctlURL)
	}
	if got.MaildURL != loaded.MaildURL {
		t.Fatalf("MaildURL = %q, want %q", got.MaildURL, loaded.MaildURL)
	}
	if got.TextureActorParkIdle != loaded.TextureActorParkIdle {
		t.Fatalf("TextureActorParkIdle = %s, want %s", got.TextureActorParkIdle, loaded.TextureActorParkIdle)
	}
}
