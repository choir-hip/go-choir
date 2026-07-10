package runtime

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
)

func TestAcceptanceServingCommitPrefersCompiledArtifactIdentity(t *testing.T) {
	t.Parallel()

	build := buildinfo.Info{
		Commit:         "compiled-serving-sha",
		DeployedCommit: "mutable-release-target-sha",
	}
	if got := acceptanceServingCommit(build); got != "compiled-serving-sha" {
		t.Fatalf("acceptance serving commit = %q, want compiled-serving-sha", got)
	}
}

func TestAcceptanceServingCommitFallsBackForLegacyHealth(t *testing.T) {
	t.Parallel()

	build := buildinfo.Info{DeployedCommit: "legacy-release-sha"}
	if got := acceptanceServingCommit(build); got != "legacy-release-sha" {
		t.Fatalf("acceptance serving commit = %q, want legacy-release-sha", got)
	}
}
