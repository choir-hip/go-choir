package agentcore

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// Fixtures shared by the P6/P7/P8 tests
// (docs/reports/p6-p8-release-adopt-2026-10-15.md): a real git source
// repository, a patch made outside it, a frozen bundle bound to an
// operation, and a current release.

func releaseGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// releaseSourceRepo is a guest source repository with one commit.
func releaseSourceRepo(t *testing.T) (repo, base string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	repo = t.TempDir()
	releaseGit(t, repo, "init", "-q")
	if err := os.MkdirAll(filepath.Join(repo, "frontend", "public"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "frontend", "public", "index.txt"), []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	releaseGit(t, repo, "add", "-A")
	releaseGit(t, repo, "commit", "-q", "-m", "base")
	return repo, releaseGit(t, repo, "rev-parse", "HEAD")
}

// releasePatch makes a change in a clone of repo and returns its patch and
// the tree a commit of the change gets. The repository sees none of it.
func releasePatch(t *testing.T, repo string, files map[string]string) (patch []byte, tree string) {
	t.Helper()
	scratch := t.TempDir()
	releaseGit(t, scratch, "clone", "-q", repo, ".")
	for path, content := range files {
		full := filepath.Join(scratch, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	releaseGit(t, scratch, "add", "-A")
	patch = []byte(releaseGit(t, scratch, "diff", "--cached", "--no-color") + "\n")
	releaseGit(t, scratch, "commit", "-q", "-m", "change")
	return patch, releaseGit(t, scratch, "rev-parse", "HEAD^{tree}")
}

// releaseFrozenBundle stages incoming/<digest>/ with a final bundle.json and
// source.patch, the way Verify leaves it, and returns the digest.
func releaseFrozenBundle(t *testing.T, updaterRoot, computerID, operationID, baseCommit string, patch []byte) string {
	t.Helper()
	return releaseFrozenBundleOn(t, updaterRoot, computerID, operationID, "trajectory-"+operationID, baseCommit, patch)
}

func releaseFrozenBundleOn(t *testing.T, updaterRoot, computerID, operationID, trajectoryID, baseCommit string, patch []byte) string {
	t.Helper()
	bundle := transaction.CapsuleEffectBundle{
		BundleVersion: 1, ComputerID: computerID, BaseEventHead: strings.Repeat("a", 64),
		TrajectoryRef: trajectoryID, CapsuleIdentity: "capsule-" + operationID,
		SourcePatchSHA256: computerevent.DigestBytes(patch), SourcePatchBaseCommit: baseCommit,
		VerifierReceipts: []string{strings.Repeat("f", 64)},
	}
	var err error
	bundle.ContentDigest, err = bundle.ComputeContentDigest()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := computerevent.CanonicalJSON(bundle)
	if err != nil {
		t.Fatal(err)
	}
	digest := computerevent.DigestBytes(raw)
	dir := filepath.Join(updaterRoot, "incoming", digest)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bundle.json"), raw, 0o400); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "source.patch"), patch, 0o444); err != nil {
		t.Fatal(err)
	}
	return digest
}

type releaseOpRow struct {
	operationID, computerID, state, bundleDigest, releaseDigest, trajectoryID string
	// idempotencyKey defaults to "key-<operationID>".
	idempotencyKey string
}

func insertReleaseOperation(t *testing.T, productStore *store.Store, row releaseOpRow) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	if row.trajectoryID == "" {
		row.trajectoryID = "trajectory-" + row.operationID
	}
	if row.idempotencyKey == "" {
		row.idempotencyKey = "key-" + row.operationID
	}
	if _, err := productStore.DB().ExecContext(context.Background(), `INSERT INTO self_development_operations
		(operation_id,computer_id,idempotency_key,request_commitment,trajectory_id,capsule_id,base_head,prompt_artifact_ref,bundle_digest,release_digest,code_ref,verifier_refs_json,decision_event,desired_head,effective_head,checkpoint_ref,route_certificate,state,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		row.operationID, row.computerID, row.idempotencyKey, strings.Repeat("d", 64), row.trajectoryID, "capsule-"+row.operationID, strings.Repeat("a", 64),
		"artifact:sha256:"+strings.Repeat("b", 64), row.bundleDigest, row.releaseDigest, "code:sha256:"+strings.Repeat("9", 64), `["verifier:`+row.operationID+`"]`,
		strings.Repeat("e", 64), strings.Repeat("c", 64), strings.Repeat("c", 64), "checkpoint:"+row.operationID, "route-cert:"+row.operationID, row.state, now, now); err != nil {
		t.Fatal(err)
	}
}

// writeCurrentRelease installs releases/<digest>/ with a valid manifest and
// points current at it.
func writeCurrentRelease(t *testing.T, updaterRoot, computerID, marker string, mutate func(*updater.ReleaseManifest)) string {
	t.Helper()
	staging := t.TempDir()
	index := []byte("<html>" + marker + "</html>")
	if err := os.MkdirAll(filepath.Join(staging, "frontend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "frontend", "index.html"), index, 0o444); err != nil {
		t.Fatal(err)
	}
	manifest := updater.ReleaseManifest{
		Version: updater.ManifestVersion, ComputerID: computerID, AcceptedEventHead: strings.Repeat("e", 64),
		CodeRef: "code:sha256:" + strings.Repeat("9", 64), ArtifactProgramRef: "artifact-program:sha256:" + strings.Repeat("8", 64),
		EventSchemaVersion: computerevent.SchemaVersionV1, ReducerVersion: computerevent.ReducerVersionV1, Marker: marker,
		Files: []updater.ManifestFile{{Path: "frontend/index.html", SHA256: computerevent.DigestBytes(index), Mode: 0o444}},
	}
	if mutate != nil {
		mutate(&manifest)
	}
	manifest, err := updater.FinalizeManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := computerevent.CanonicalJSON(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "release-manifest.json"), raw, 0o444); err != nil {
		t.Fatal(err)
	}
	releaseDir := filepath.Join(updaterRoot, "releases", manifest.ContentDigest)
	if err := os.MkdirAll(filepath.Dir(releaseDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(staging, releaseDir); err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(updaterRoot, "current")
	_ = os.Remove(current)
	if err := os.Symlink(filepath.Join("releases", manifest.ContentDigest), current); err != nil {
		t.Fatal(err)
	}
	return manifest.ContentDigest
}

func insertEventHead(t *testing.T, productStore *store.Store, computerID string, sequence uint64, canonical, effective string) {
	t.Helper()
	if _, err := productStore.DB().ExecContext(context.Background(), `REPLACE INTO computer_event_projection_heads
		(computer_id,sequence,canonical_event_head,desired_event_head,effective_event_head,pending_transition_ref,desired_state_commitment,effective_state_commitment,reducer_version,credential_revocation_epoch,updated_at)
		VALUES (?,?,?,?,?,NULL,?,?,?,0,?)`, computerID, sequence, canonical, effective, effective, strings.Repeat("7", 64), strings.Repeat("7", 64), 1, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
}

func releaseTestRuntime(t *testing.T, computerID string) (*Runtime, *store.Store, string) {
	t.Helper()
	rt, productStore := testRuntime(t)
	rt.cfg.ComputerID = computerID
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	rt.selfdevOperations = operations
	updaterRoot := t.TempDir()
	rt.selfdevUpdaterRoot = updaterRoot
	return rt, productStore, updaterRoot
}
