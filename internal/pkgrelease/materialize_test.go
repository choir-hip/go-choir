package pkgrelease

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Materialize (runtime install, S12), failure modes written before the code:
//
//	M1  bytes that are not the record's are written;
//	M2  a tree reaching outside the package root is written (any part);
//	M3  a secret-bearing tree is written;
//	M4  the files written are not the release's bytes, or land outside
//	    dir/<root>;
//	M5  a non-empty target directory is overwritten.

func TestMaterializeWritesTheVerifiedTreeOnly(t *testing.T) {
	f := newFixture(t)
	record, blobs := f.release(f.tree(changed("<p>reader v2</p>\n")))
	ctx := context.Background()

	// M1
	dir := t.TempDir()
	wantReason(t, Materialize(ctx, record, blobs.TreePatch[:len(blobs.TreePatch)-1], dir), ReasonBlobMismatch)
	assertEmpty(t, dir)

	// M2
	outside := f.tree(map[string]entry{testRoot + "/App.svelte": {"100644", "<p>v2</p>\n"}, "frontend/src/app.css": {"100644", "body{color:red}\n"}})
	forged := f.fullDiff(EmptyTree, outside)
	bad := record
	bad.Source.TreePatch = *blobOf(forged)
	wantReason(t, Materialize(ctx, bad, forged, dir), ReasonContentNotConfined)
	assertEmpty(t, dir)

	// M3
	srec, sblobs := f.release(f.tree(changed("GITHUB_TOKEN=ghp_" + strings.Repeat("A", 36) + "\n")))
	wantReason(t, Materialize(ctx, srec, sblobs.TreePatch, dir), ReasonSecretFound)
	assertEmpty(t, dir)

	// M4
	if err := Materialize(ctx, record, blobs.TreePatch, dir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, testRoot, "App.svelte"))
	if err != nil || string(got) != "<p>reader v2</p>\n" {
		t.Fatalf("M4: App.svelte = %q (%v)", got, err)
	}
	top, _ := os.ReadDir(dir)
	if len(top) != 1 || top[0].Name() != "frontend" {
		t.Fatalf("M4: top level holds %v", top)
	}

	// M5
	if err := Materialize(ctx, record, blobs.TreePatch, dir); err == nil {
		t.Fatal("M5: a non-empty directory was written again")
	}
}

func assertEmpty(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("%s holds %v (%v) after a refusal", dir, entries, err)
	}
}
