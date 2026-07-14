package runtimeprompts

import (
	"strings"
	"testing"
)

func TestResearcherRuntimeOverlayIncludesParallelSaturation(t *testing.T) {
	overlay := ResearcherRuntimeOverlay()
	for _, want := range []string{
		"parallel tool-call block",
		"Send another update_coagent after each additional search/fetch batch",
		"persistent communicating coagent",
	} {
		if !strings.Contains(overlay, want) {
			t.Fatalf("researcher runtime overlay missing %q: %q", want, overlay)
		}
	}
}

func TestSuperRuntimeOverlayIncludesAuthorityBoundary(t *testing.T) {
	overlay := SuperRuntimeOverlay()
	if !strings.Contains(overlay, "Super authority boundary") {
		t.Fatalf("super runtime overlay missing authority boundary: %q", overlay)
	}
}

func TestWorkerRepoBootstrapOverlayRendersCommands(t *testing.T) {
	overlay := WorkerRepoBootstrap(WorkerRepoBootstrapOptions{
		RemoteURL: "https://github.com/example/repo.git",
		BaseSHA:   "abc123",
		Bootstrap: "remote_git_clone",
	})
	for _, want := range []string{
		"base_sha: abc123",
		"git clone https://github.com/example/repo.git Source/candidate",
		"Use set -euo pipefail",
	} {
		if !strings.Contains(overlay, want) {
			t.Fatalf("worker repo bootstrap overlay missing %q: %q", want, overlay)
		}
	}
}
