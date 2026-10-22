package capsule

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// One `git cat-file --batch` per pass instead of one git process per file,
// and a per-commit digest cache (docs/problems/capsule-coldstart-18s-2026-10-22.md,
// options 1 and 2). Ways it could fail, written before the code:
//   - the batch path hashes different bytes, modes or paths than the per-file
//     path, so every pinned binding's digest changes;
//   - a missing object, a short read, a header for another object, a non-blob
//     type, a missing terminator or trailing bytes is skipped or zero-filled,
//     giving a digest over a partial tree, or a partial copy left behind;
//   - the object store returns other bytes for an object id and nothing
//     notices (neither path re-hashed git objects before);
//   - a symlink, an executable bit, an empty file, or a name with spaces or a
//     newline is mangled by a line-oriented batch protocol;
//   - the cache returns a digest for another commit, survives a change of the
//     digest algorithm, caches a failure, or lets a corrupt store reach a copy;
//   - the copy stops checking itself against the commit when no expected
//     digest is given.

// perFileCommitDigest is the per-file path this lane replaces (one
// `git cat-file blob` process per tracked file), kept as the reference the
// batch path must equal.
func perFileCommitDigest(ctx context.Context, source, commit string) (string, error) {
	tracked, err := immutableCommitInventory(ctx, source, commit)
	if err != nil {
		return "", err
	}
	directories := map[string]bool{}
	entries := make([]canonicalTreeEntry, 0, len(tracked)*2)
	for _, file := range tracked {
		for dir := filepath.Dir(file.path); dir != "."; dir = filepath.Dir(dir) {
			directories[filepath.ToSlash(dir)] = true
		}
		raw, err := exec.CommandContext(ctx, "git", "-C", source, "cat-file", "blob", file.oid).Output()
		if err != nil {
			return "", err
		}
		item := canonicalTreeEntry{Path: filepath.ToSlash(file.path)}
		switch file.mode {
		case "120000":
			item.Type, item.Mode = "symlink", 0o777
			if symlinkEscapesTree(file.path, string(raw)) {
				return "", fmt.Errorf("escaping symlink %q", file.path)
			}
		case "100755":
			item.Type, item.Mode = "file", 0o755
		default:
			item.Type, item.Mode = "file", 0o644
		}
		digest := sha256.Sum256(raw)
		item.ContentDigest = hex.EncodeToString(digest[:])
		entries = append(entries, item)
	}
	for dir := range directories {
		entries = append(entries, canonicalTreeEntry{Path: dir, Type: "directory", Mode: 0o755})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	canonical, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

// perFileCopy is the per-file copy pass this lane replaces, for the timings.
func perFileCopy(ctx context.Context, source, commit, target string) error {
	tracked, err := immutableCommitInventory(ctx, source, commit)
	if err != nil {
		return err
	}
	for _, file := range tracked {
		raw, err := exec.CommandContext(ctx, "git", "-C", source, "cat-file", "blob", file.oid).Output()
		if err != nil {
			return err
		}
		path := filepath.Join(target, file.path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if file.mode == "120000" {
			if err := os.Symlink(string(raw), path); err != nil {
				return err
			}
			continue
		}
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func gitBlobOID(content []byte) string {
	hash := sha1.New()
	fmt.Fprintf(hash, "blob %d\x00", len(content))
	hash.Write(content)
	return hex.EncodeToString(hash.Sum(nil))
}

func thisRepo(t *testing.T) (string, string) {
	t.Helper()
	root, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skipf("not in a git checkout: %v", err)
	}
	commit, err := exec.Command("git", "rev-parse", "--verify", "HEAD^{commit}").Output()
	if err != nil {
		t.Skipf("no HEAD commit: %v", err)
	}
	return strings.TrimSpace(string(root)), strings.TrimSpace(string(commit))
}

func edgeCaseRepo(t *testing.T) (string, string) {
	t.Helper()
	source := t.TempDir()
	runSourceGit(t, source, "init")
	runSourceGit(t, source, "config", "user.name", "Test")
	runSourceGit(t, source, "config", "user.email", "test@example.invalid")
	files := map[string]string{
		"empty":                    "",
		"name with spaces":         "spaces\n",
		"line\nbreak":              "newline in the name\n",
		"nested/deep/plain":        "plain",
		"nested/tab\tname":         "tab",
		"binary":                   "\x00\x01\x02\n\n\x00 missing\n",
		"looks-like-header":        strings.Repeat("0", 40) + " blob 5\nhello\n",
		"nested/space dir/x y z":   "spaced dir",
		"trailing-newline-only":    "\n",
		"nested/deep/large-binary": strings.Repeat("\xff\x00batch\n", 70000),
	}
	for name, body := range files {
		path := filepath.Join(source, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "run me.sh"), []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "empty-exec"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	for link, target := range map[string]string{"link": "nested/deep/plain", "nested/up": "../empty", "nested/space link": "../name with spaces", "nix": "/nix/store/abc-x"} {
		if err := os.Symlink(target, filepath.Join(source, link)); err != nil {
			t.Fatal(err)
		}
	}
	runSourceGit(t, source, "add", ".")
	runSourceGit(t, source, "commit", "-m", "edge cases")
	commit, err := immutableGitCommitIdentity(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	return source, commit
}

// looseObjectPath is where git keeps the loose object oid in source.
func looseObjectPath(source, oid string) string {
	return filepath.Join(source, ".git", "objects", oid[:2], oid[2:])
}

func blobOIDOf(t *testing.T, source, commit, path string) string {
	t.Helper()
	tracked, err := immutableCommitInventory(context.Background(), source, commit)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range tracked {
		if file.path == path {
			return file.oid
		}
	}
	t.Fatalf("%s not tracked", path)
	return ""
}

// replaceLooseObject makes the object store answer oid with other bytes: it
// writes other's loose object file under oid's name. It returns a restore.
func replaceLooseObject(t *testing.T, source, oid string, other []byte) func() {
	t.Helper()
	otherFile := filepath.Join(t.TempDir(), "other")
	if err := os.WriteFile(otherFile, other, 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := exec.Command("git", "-C", source, "hash-object", "-w", otherFile).Output()
	if err != nil {
		t.Fatal(err)
	}
	otherOID := strings.TrimSpace(string(raw))
	path := looseObjectPath(source, oid)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := os.ReadFile(looseObjectPath(source, otherOID))
	if err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(path, 0o644)
	if err := os.WriteFile(path, replacement, 0o644); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.WriteFile(path, original, 0o444); err != nil {
			t.Fatal(err)
		}
	}
}

func requireAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial tree left at %s: %v", path, err)
	}
}

func TestBatchCommitDigestEqualsPerFileDigestOnThisRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("reads this whole repository through one git process per file")
	}
	source, commit := thisRepo(t)
	ctx := context.Background()
	reference, err := perFileCommitDigest(ctx, source, commit)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := canonicalImmutableCommitDigest(ctx, source, commit)
	if err != nil {
		t.Fatal(err)
	}
	if batch != reference {
		t.Fatalf("batch digest %s != per-file digest %s at %s", batch, reference, commit)
	}
	target := filepath.Join(t.TempDir(), "workspace", "platform")
	unlockTree(t, target)
	copied, err := copyImmutableCommitTree(ctx, source, commit, target)
	if err != nil || copied != reference {
		t.Fatalf("batch copy digest %s != per-file digest %s: %v", copied, reference, err)
	}
}

func TestBatchPathKeepsEdgeCaseTreesExact(t *testing.T) {
	source, commit := edgeCaseRepo(t)
	ctx := context.Background()
	reference, err := perFileCommitDigest(ctx, source, commit)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := canonicalImmutableCommitDigest(ctx, source, commit)
	if err != nil || batch != reference {
		t.Fatalf("batch digest %s != per-file %s: %v", batch, reference, err)
	}
	target := filepath.Join(t.TempDir(), "copy")
	unlockTree(t, target)
	copied, err := copyImmutableCommitTree(ctx, source, commit, target)
	if err != nil || copied != reference {
		t.Fatalf("copy digest %s != per-file %s: %v", copied, reference, err)
	}
	for name, want := range map[string]string{"empty": "", "line\nbreak": "newline in the name\n", "name with spaces": "spaces\n", "looks-like-header": strings.Repeat("0", 40) + " blob 5\nhello\n", "binary": "\x00\x01\x02\n\n\x00 missing\n"} {
		raw, err := os.ReadFile(filepath.Join(target, name))
		if err != nil || string(raw) != want {
			t.Fatalf("%q = %q, %v", name, raw, err)
		}
	}
	for name, perm := range map[string]os.FileMode{"run me.sh": 0o555, "empty-exec": 0o555, "empty": 0o444} {
		info, err := os.Lstat(filepath.Join(target, name))
		if err != nil || info.Mode().Perm() != perm {
			t.Fatalf("%q mode %v, %v; want %v", name, info, err, perm)
		}
	}
	for link, want := range map[string]string{"link": "nested/deep/plain", "nested/space link": "../name with spaces", "nix": "/nix/store/abc-x"} {
		got, err := os.Readlink(filepath.Join(target, link))
		if err != nil || got != want {
			t.Fatalf("symlink %q -> %q, %v", link, got, err)
		}
	}
}

func TestBatchPathRefusesMissingBlobWithNoPartialTree(t *testing.T) {
	source, commit := edgeCaseRepo(t)
	oid := blobOIDOf(t, source, commit, "nested/deep/plain")
	if err := os.Remove(looseObjectPath(source, oid)); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if digest, err := canonicalImmutableCommitDigest(ctx, source, commit); err == nil {
		t.Fatalf("digest %s over a tree with a missing blob", digest)
	}
	target := filepath.Join(t.TempDir(), "copy")
	if digest, err := copyImmutableCommitTree(ctx, source, commit, target); err == nil {
		t.Fatalf("copy %s of a tree with a missing blob", digest)
	}
	requireAbsent(t, target)
}

func TestBatchPathRefusesObjectStoreThatAnswersOtherBytes(t *testing.T) {
	source, commit := edgeCaseRepo(t)
	oid := blobOIDOf(t, source, commit, "name with spaces")
	replaceLooseObject(t, source, oid, []byte("not the committed bytes\n"))
	ctx := context.Background()
	if digest, err := canonicalImmutableCommitDigest(ctx, source, commit); err == nil {
		t.Fatalf("digest %s over bytes that do not hash to their object id", digest)
	}
	target := filepath.Join(t.TempDir(), "copy")
	if digest, err := copyImmutableCommitTree(ctx, source, commit, target); err == nil {
		t.Fatalf("copy %s of bytes that do not hash to their object id", digest)
	}
	requireAbsent(t, target)
}

func TestInventoryRefusesNonBlobEntry(t *testing.T) {
	source, _ := edgeCaseRepo(t)
	runSourceGit(t, source, "update-index", "--add", "--cacheinfo", "160000,"+strings.Repeat("ab", 20)+",submodule")
	runSourceGit(t, source, "commit", "-m", "gitlink")
	commit, err := immutableGitCommitIdentity(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if digest, err := canonicalImmutableCommitDigest(context.Background(), source, commit); err == nil {
		t.Fatalf("digest %s over a tree with a gitlink", digest)
	}
	target := filepath.Join(t.TempDir(), "copy")
	if _, err := copyImmutableCommitTree(context.Background(), source, commit, target); err == nil {
		t.Fatal("copy of a tree with a gitlink")
	}
	requireAbsent(t, target)
}

// batchFixture is a hand-built `git cat-file --batch` answer for the parser.
type batchFixture struct {
	files  []immutableGitFile
	bodies [][]byte
}

func newBatchFixture(bodies ...string) batchFixture {
	var fixture batchFixture
	for i, body := range bodies {
		fixture.files = append(fixture.files, immutableGitFile{mode: "100644", oid: gitBlobOID([]byte(body)), path: fmt.Sprintf("f%d", i)})
		fixture.bodies = append(fixture.bodies, []byte(body))
	}
	return fixture
}

func (f batchFixture) answer(i int) string {
	return fmt.Sprintf("%s blob %d\n%s\n", f.files[i].oid, len(f.bodies[i]), f.bodies[i])
}

func (f batchFixture) stream() string {
	var out strings.Builder
	for i := range f.files {
		out.WriteString(f.answer(i))
	}
	return out.String()
}

func collectBatch(stream string, files []immutableGitFile) ([]string, error) {
	var got []string
	err := parseBlobBatch(strings.NewReader(stream), files, func(_ immutableGitFile, blob io.Reader) error {
		raw, err := io.ReadAll(blob)
		got = append(got, string(raw))
		return err
	})
	return got, err
}

func TestParseBlobBatchReadsEveryBlobInOrder(t *testing.T) {
	fixture := newBatchFixture("one", "", "two\nlines\n", "\n")
	got, err := collectBatch(fixture.stream(), fixture.files)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"one", "", "two\nlines\n", "\n"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q, want %q", got, want)
	}
	// A visitor that stops early still has the rest of the blob drained and
	// checked, and the next object read from the right place.
	err = parseBlobBatch(strings.NewReader(fixture.stream()), fixture.files, func(immutableGitFile, io.Reader) error { return nil })
	if err != nil {
		t.Fatalf("unread blobs: %v", err)
	}
}

func TestParseBlobBatchFailsClosed(t *testing.T) {
	fixture := newBatchFixture("first blob\n", "second blob")
	full := fixture.stream()
	first := fixture.answer(0)
	cases := map[string]string{
		"empty output":            "",
		"truncated blob":          full[:len(full)-5],
		"truncated header":        first + fixture.files[1].oid[:20],
		"missing terminator":      first + strings.TrimSuffix(fixture.answer(1), "\n"),
		"wrong terminator":        first + strings.TrimSuffix(fixture.answer(1), "\n") + "X",
		"trailing bytes":          full + "extra",
		"missing object":          first + fixture.files[1].oid + " missing\n",
		"ambiguous object":        first + fixture.files[1].oid + " ambiguous\n",
		"tree instead of blob":    first + fmt.Sprintf("%s tree %d\n%s\n", fixture.files[1].oid, len(fixture.bodies[1]), fixture.bodies[1]),
		"header for other object": first + fmt.Sprintf("%s blob %d\n%s\n", fixture.files[0].oid, len(fixture.bodies[1]), fixture.bodies[1]),
		"bytes not the object":    first + fmt.Sprintf("%s blob %d\n%s\n", fixture.files[1].oid, len("second blub"), "second blub"),
		"size too short":          first + fmt.Sprintf("%s blob %d\n%s\n", fixture.files[1].oid, 6, "second"),
		"negative size":           first + fmt.Sprintf("%s blob -1\n\n", fixture.files[1].oid),
		"non-numeric size":        first + fmt.Sprintf("%s blob 1x\nx\n", fixture.files[1].oid),
		"extra header field":      first + fmt.Sprintf("%s blob %d extra\n%s\n", fixture.files[1].oid, len(fixture.bodies[1]), fixture.bodies[1]),
	}
	for name, stream := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := collectBatch(stream, fixture.files)
			if err == nil {
				t.Fatalf("accepted %q: %q", stream, got)
			}
		})
	}
}

func TestParseBlobBatchPropagatesVisitorError(t *testing.T) {
	fixture := newBatchFixture("a", "b")
	stop := errors.New("visitor refused")
	err := parseBlobBatch(strings.NewReader(fixture.stream()), fixture.files, func(immutableGitFile, io.Reader) error { return stop })
	if !errors.Is(err, stop) {
		t.Fatalf("err=%v", err)
	}
}

func TestParseBlobBatchHandsVisitorAnErrorNotEOFOnCorruptBytes(t *testing.T) {
	fixture := newBatchFixture("committed")
	stream := fmt.Sprintf("%s blob %d\n%s\n", fixture.files[0].oid, len("corrupted"), "corrupted")
	var visitorErr error
	_ = parseBlobBatch(strings.NewReader(stream), fixture.files, func(_ immutableGitFile, blob io.Reader) error {
		_, visitorErr = io.Copy(io.Discard, blob)
		return visitorErr
	})
	if visitorErr == nil {
		t.Fatal("visitor read corrupt bytes to a clean EOF")
	}
}

func TestReadCommitBlobsUsesOneGitProcessAndRefusesNonRepo(t *testing.T) {
	source, commit := edgeCaseRepo(t)
	tracked, err := immutableCommitInventory(context.Background(), source, commit)
	if err != nil {
		t.Fatal(err)
	}
	var seen int
	if err := readCommitBlobs(context.Background(), source, tracked, func(file immutableGitFile, blob io.Reader) error {
		seen++
		_, err := io.Copy(io.Discard, blob)
		return err
	}); err != nil || seen != len(tracked) {
		t.Fatalf("seen %d of %d: %v", seen, len(tracked), err)
	}
	if err := readCommitBlobs(context.Background(), t.TempDir(), tracked, func(immutableGitFile, io.Reader) error { return nil }); err == nil {
		t.Fatal("read blobs from a directory that is not a repository")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := readCommitBlobs(ctx, source, tracked, func(immutableGitFile, io.Reader) error { return nil }); err == nil {
		t.Fatal("cancelled read succeeded")
	}
}

func TestCopyWithExpectedDigestRefusesAnotherDigest(t *testing.T) {
	source, commit := edgeCaseRepo(t)
	target := filepath.Join(t.TempDir(), "copy")
	if _, err := copyImmutableCommitTreeTimed(context.Background(), source, commit, target, strings.Repeat("ab", 32), nil); err == nil {
		t.Fatal("copy accepted against another tree's digest")
	}
	requireAbsent(t, target)
}

func TestPreflightCachesTheCommitDigestAndTheCopyStillChecksTheStore(t *testing.T) {
	source, commit := edgeCaseRepo(t)
	ctx := context.Background()
	e := &Executor{stateDir: filepath.Join(t.TempDir(), "state"), sourceDir: source}
	opened, err := e.PreflightSourceSnapshot(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(opened.ArtifactRef, "capsule-source-git:"+commit+":sha256:") {
		t.Fatalf("ref %s", opened.ArtifactRef)
	}
	// The store now answers one blob with other bytes. The cast's preflight
	// proved the digest in this process, so the resume preflight trusts it...
	restore := replaceLooseObject(t, source, blobOIDOf(t, source, commit, "nested/deep/plain"), []byte("swapped"))
	resumed, err := e.PreflightSourceSnapshot(ctx, opened.ArtifactRef)
	if err != nil || resumed != opened {
		t.Fatalf("cached resume preflight %+v != %+v: %v", resumed, opened, err)
	}
	// ...a process without the proof reads the store and refuses...
	fresh := &Executor{stateDir: filepath.Join(t.TempDir(), "state"), sourceDir: source}
	if _, err := fresh.PreflightSourceSnapshot(ctx, opened.ArtifactRef); err == nil {
		t.Fatal("uncached resume preflight accepted a corrupt store")
	}
	// ...and the copy, which reads the store again, refuses what the cache
	// let through, leaving no tree.
	target := filepath.Join(t.TempDir(), "copy")
	if _, err := copyImmutableCommitTreeTimed(ctx, source, commit, target, opened.SubjectDigest, nil); err == nil {
		t.Fatal("copy from a corrupt store matched the cached digest")
	}
	requireAbsent(t, target)
	restore()
	unlockTree(t, target)
	if copied, err := copyImmutableCommitTreeTimed(ctx, source, commit, target, opened.SubjectDigest, nil); err != nil || copied != opened.SubjectDigest {
		t.Fatalf("restored copy %s: %v", copied, err)
	}
}

func TestCommitDigestCacheIsKeyedByAlgorithmVersionAndNeverCachesFailure(t *testing.T) {
	source, commit := edgeCaseRepo(t)
	ctx := context.Background()
	want, err := canonicalImmutableCommitDigest(ctx, source, commit)
	if err != nil {
		t.Fatal(err)
	}
	ref := "capsule-source-git:" + commit + ":sha256:" + want
	e := &Executor{sourceDir: source, commitDigests: map[commitDigestKey]string{
		{Version: canonicalTreeDigestVersion + 1, Commit: commit}: strings.Repeat("0", 64),
	}}
	if got, err := e.PreflightSourceSnapshot(ctx, ref); err != nil || got.SubjectDigest != want {
		t.Fatalf("another algorithm version's entry answered: %+v %v", got, err)
	}

	restore := replaceLooseObject(t, source, blobOIDOf(t, source, commit, "empty"), []byte("not empty"))
	failing := &Executor{sourceDir: source}
	if _, err := failing.PreflightSourceSnapshot(ctx, ref); err == nil {
		t.Fatal("corrupt store accepted")
	}
	restore()
	if got, err := failing.PreflightSourceSnapshot(ctx, ref); err != nil || got.SubjectDigest != want {
		t.Fatalf("a failure was cached: %+v %v", got, err)
	}
}

// TestSourcePassTimingsBeforeAfter prints what each cold-start pass costs on
// this repository, the per-file way and the batch way. It asserts nothing
// about time. Run it with CAPSULE_SOURCE_TIMINGS=1 go test -run
// TestSourcePassTimingsBeforeAfter -v ./internal/capsule.
func TestSourcePassTimingsBeforeAfter(t *testing.T) {
	if os.Getenv("CAPSULE_SOURCE_TIMINGS") == "" {
		t.Skip("set CAPSULE_SOURCE_TIMINGS=1 to print the per-pass timings")
	}
	source, commit := thisRepo(t)
	ctx := context.Background()
	tracked, err := immutableCommitInventory(ctx, source, commit)
	if err != nil {
		t.Fatal(err)
	}
	var report bytes.Buffer
	measure := func(name string, run func() error) {
		started := time.Now()
		if err := run(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		fmt.Fprintf(&report, "%-58s %8s\n", name, time.Since(started).Round(time.Millisecond))
	}
	fmt.Fprintf(&report, "repo %s commit %s: %d tracked files\n", source, commit, len(tracked))
	measure("before: commit digest, per-file (cast/resume preflight)", func() error { _, err := perFileCommitDigest(ctx, source, commit); return err })
	measure("before: copy blobs, per-file (source_blobs)", func() error { return perFileCopy(ctx, source, commit, filepath.Join(t.TempDir(), "before")) })
	measure("before: commit digest again (source_digest_commit)", func() error { _, err := perFileCommitDigest(ctx, source, commit); return err })
	e := &Executor{sourceDir: source}
	var digest string
	measure("after: commit digest, one batch (cast preflight, uncached)", func() error {
		got, err := e.PreflightSourceSnapshot(ctx, "")
		digest = got.SubjectDigest
		return err
	})
	ref := "capsule-source-git:" + commit + ":sha256:" + digest
	measure("after: resume preflight (cached)", func() error { _, err := e.PreflightSourceSnapshot(ctx, ref); return err })
	measure("after: resume preflight (uncached, one batch)", func() error {
		_, err := (&Executor{sourceDir: source}).PreflightSourceSnapshot(ctx, ref)
		return err
	})
	target := filepath.Join(t.TempDir(), "after")
	unlockTree(t, target)
	measure("after: copy + copy digest vs binding (source_blobs..)", func() error {
		_, err := copyImmutableCommitTreeTimed(ctx, source, commit, target, digest, nil)
		return err
	})
	t.Log("\n" + report.String())
}
