//go:build comprehensive

package promotion

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestAppPromotionBaseRefPrefersPackageLedgerBase(t *testing.T) {
	t.Setenv("RUNTIME_WORKER_REPO_BASE_SHA", "deployed-head")
	pkg := types.AppChangePackageRecord{
		ManifestJSON:    json.RawMessage(`{"source_ledger_base_ref":"package-base-sha"}`),
		SourceActiveRef: "source-active",
	}
	rec := types.AppAdoptionRecord{
		TargetActiveSourceRefAtCandidateStart: "target-start",
	}
	if got := appPromotionBaseRef(pkg, rec, "target-cutover"); got != "package-base-sha" {
		t.Fatalf("base ref = %q, want package ledger base", got)
	}
}

func TestAppPromotionBaseRefNormalizesLedgerGitToken(t *testing.T) {
	t.Setenv("RUNTIME_WORKER_REPO_BASE_SHA", "deployed-head")
	const deployedSHA = "357d20670af512af5fdb4eb04b310527047d2e5c"
	pkg := types.AppChangePackageRecord{
		ManifestJSON:    json.RawMessage(`{"source_ledger_base_ref":"git:go-choir-candidate@357d20670af512af5fdb4eb04b310527047d2e5c\n"}`),
		SourceActiveRef: "source-active",
	}
	rec := types.AppAdoptionRecord{
		TargetActiveSourceRefAtCandidateStart: "target-start",
	}
	if got := appPromotionBaseRef(pkg, rec, "target-cutover"); got != deployedSHA {
		t.Fatalf("base ref = %q, want checkoutable sha %q", got, deployedSHA)
	}
}

func TestAppPromotionBaseRefNormalizesPlainGitSHARef(t *testing.T) {
	t.Setenv("RUNTIME_WORKER_REPO_BASE_SHA", "deployed-head")
	const deployedSHA = "f3bcd44d6ac651e73138a53d636af47da0c5a606"
	pkg := types.AppChangePackageRecord{
		ManifestJSON:    json.RawMessage(`{"source_ledger_base_ref":"git:f3bcd44d6ac651e73138a53d636af47da0c5a606"}`),
		SourceActiveRef: "source-active",
	}
	rec := types.AppAdoptionRecord{
		TargetActiveSourceRefAtCandidateStart: "target-start",
	}
	if got := appPromotionBaseRef(pkg, rec, "target-cutover"); got != deployedSHA {
		t.Fatalf("base ref = %q, want checkoutable sha %q", got, deployedSHA)
	}
}

func TestAppPromotionBaseRefNormalizesLedgerRoleToken(t *testing.T) {
	t.Setenv("RUNTIME_WORKER_REPO_BASE_SHA", "deployed-head")
	const deployedSHA = "b11ed4f2f517b2f1a7a3d8a054b17490b76510ec"
	pkg := types.AppChangePackageRecord{
		ManifestJSON:    json.RawMessage(`{"source_ledger_base_ref":"base:b11ed4f2f517b2f1a7a3d8a054b17490b76510ec"}`),
		SourceActiveRef: "source-active",
	}
	rec := types.AppAdoptionRecord{
		TargetActiveSourceRefAtCandidateStart: "target-start",
	}
	if got := appPromotionBaseRef(pkg, rec, "target-cutover"); got != deployedSHA {
		t.Fatalf("base ref = %q, want checkoutable sha %q", got, deployedSHA)
	}
}

func TestAppPromotionBaseRefSkipsProductOnlyComputerRefs(t *testing.T) {
	t.Setenv("RUNTIME_WORKER_REPO_BASE_SHA", "deployed-head")
	pkg := types.AppChangePackageRecord{
		ManifestJSON:    json.RawMessage(`{"source_ledger_base_ref":"refs/computers/user-a/active"}`),
		SourceActiveRef: "refs/platform-computers/default/active",
	}
	rec := types.AppAdoptionRecord{
		TargetActiveSourceRefAtCandidateStart: "refs/computers/user-b/active",
	}
	if got := appPromotionBaseRef(pkg, rec, "refs/heads/app-adoptions/candidate"); got != "app-adoptions/candidate" {
		t.Fatalf("base ref = %q, want checkoutable branch", got)
	}
}

func TestAppPromotionBuildEnvUsesWorkspaceScratchPaths(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "promotion-workspaces")
	candidateDir := filepath.Join(root, "adoption-1")
	env, scratchRoot, err := appPromotionBuildEnv(candidateDir)
	if err != nil {
		t.Fatalf("appPromotionBuildEnv: %v", err)
	}
	if want := filepath.Join(candidateDir, ".choir-promotion-scratch"); scratchRoot != want {
		t.Fatalf("scratch root = %q, want %q", scratchRoot, want)
	}

	scratchKeys := []string{"TMPDIR", "GOTMPDIR", "GOCACHE", "XDG_CACHE_HOME"}
	for _, key := range scratchKeys {
		value := promotionTestEnvValue(env, key)
		if !strings.HasPrefix(value, scratchRoot+string(os.PathSeparator)) {
			t.Fatalf("%s = %q, want under scratch root %q", key, value, scratchRoot)
		}
		if _, err := os.Stat(value); err != nil {
			t.Fatalf("%s dir %q not prepared: %v", key, value, err)
		}
	}

	cacheRoot := filepath.Join(root, ".choir-promotion-cache")
	for _, key := range []string{"GOMODCACHE", "NPM_CONFIG_CACHE"} {
		value := promotionTestEnvValue(env, key)
		if !strings.HasPrefix(value, cacheRoot+string(os.PathSeparator)) {
			t.Fatalf("%s = %q, want under cache root %q", key, value, cacheRoot)
		}
		if _, err := os.Stat(value); err != nil {
			t.Fatalf("%s dir %q not prepared: %v", key, value, err)
		}
	}

	report, err := runAppPromotionShellCommand(context.Background(), t.TempDir(), env, "env-check", `test -d "$GOTMPDIR" && test -d "$GOCACHE" && test -d "$GOMODCACHE" && test -d "$NPM_CONFIG_CACHE"`)
	if err != nil {
		t.Fatalf("promotion command with build env failed: %+v err=%v", report, err)
	}
}

func TestDefaultAppPromotionBuildCommandsUseBuildCapableMemoryCaps(t *testing.T) {
	t.Parallel()
	if !strings.Contains(provideriface.DefaultAppPromotionRuntimeBuildCommand, "GOMEMLIMIT=1024MiB") {
		t.Fatalf("runtime promotion build command should use the build-capable memory cap: %s", provideriface.DefaultAppPromotionRuntimeBuildCommand)
	}
	if !strings.Contains(provideriface.DefaultAppPromotionUIBuildCommand, "--max-old-space-size=768") {
		t.Fatalf("UI promotion build command should use the build-capable memory cap: %s", provideriface.DefaultAppPromotionUIBuildCommand)
	}
}

func TestTruncateAppPromotionOutputPreservesHeadAndTail(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", 13000) + "compiler-tail-error"
	got := truncateAppPromotionOutput(long)
	if len(got) > 12050 {
		t.Fatalf("truncated output too long: %d", len(got))
	}
	if !strings.Contains(got, "compiler-tail-error") {
		t.Fatalf("truncated output lost compiler tail: %q", got[len(got)-200:])
	}
	if !strings.Contains(got, "truncated middle") {
		t.Fatalf("truncated output missing marker: %q", got)
	}
}

func promotionTestEnvValue(env []string, key string) string {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	return ""
}
