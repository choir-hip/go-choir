package autoputer

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/recoveryplan"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// Ways the legacy-resume gate could fail: a bare stat of the .texture
// directory is true on every production computer (store.Open always creates
// the sidecar there), so a lost disk with a long chain was told "the migrator
// owns this boot" instead of installing its snapshot or refusing base_missing;
// or a real legacy Dolt workspace (a child directory holding .dolt) is no
// longer recognised and its data is skipped.

func TestSidecarOnlyTextureDirectoryDoesNotTakeTheLegacyResumeBranch(t *testing.T) {
	platform := newBootServer(&computerevent.Head{ComputerID: "computer-sidecar", Sequence: 1148333, CanonicalEventHead: strings.Repeat("a", 64)})
	server := httptest.NewServer(platform)
	defer server.Close()
	t.Setenv("CHOIR_PRIVACY_KEY_FILE", writePrivacyKey(t, "computer-sidecar"))

	storePath := filepath.Join(t.TempDir(), "runtime.db")
	sidecar := choirstore.LegacyDoltWorkspacePath(storePath)
	if err := os.MkdirAll(sidecar, 0o755); err != nil {
		t.Fatal(err)
	}
	// What store.Open keeps there: marker files, no database directory.
	if err := os.WriteFile(filepath.Join(sidecar, "schema-receipt.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, err := materializeProjectionBaseIfNeeded(context.Background(), storePath, "computer-sidecar", server.URL, testCapability, nil)
	if !errors.Is(err, recoveryplan.ErrBaseRefused) {
		t.Fatalf("a sidecar-only .texture directory took the legacy branch (err=%v); a long chain with no snapshot must refuse base_missing", err)
	}
	var refusal *ProjectionBaseRefusal
	if !errors.As(err, &refusal) || refusal.Kind != BootRefusalKindBaseMissing {
		t.Fatalf("refusal = %+v", refusal)
	}
}

func TestLegacyDoltWorkspaceStillTakesTheMigratorBranch(t *testing.T) {
	platform := newBootServer(&computerevent.Head{ComputerID: "computer-legacy", Sequence: 1148333, CanonicalEventHead: strings.Repeat("a", 64)})
	server := httptest.NewServer(platform)
	defer server.Close()
	t.Setenv("CHOIR_PRIVACY_KEY_FILE", writePrivacyKey(t, "computer-legacy"))

	storePath := filepath.Join(t.TempDir(), "runtime.db")
	if err := os.MkdirAll(filepath.Join(choirstore.LegacyDoltWorkspacePath(storePath), "choir", ".dolt"), 0o755); err != nil {
		t.Fatal(err)
	}
	plan, materialized, err := materializeProjectionBaseIfNeeded(context.Background(), storePath, "computer-legacy", server.URL, testCapability, nil)
	if err != nil {
		t.Fatalf("a legacy Dolt workspace must be left to the migrator: %v", err)
	}
	if plan.Action != recoveryplan.RecoveryResume || materialized || !strings.Contains(plan.Reason, "legacy Dolt") {
		t.Fatalf("plan = %+v materialized=%t", plan, materialized)
	}
}
