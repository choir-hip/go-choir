package projectionbase

// S4 of docs/problems/paper-never-checkpointed-2026-10-27.md: incremental
// base publication. Written before the code. Ways the parts form could fail,
// each pinned below:
//   - a publish rewrites parts that did not change (no saving);
//   - a reused part is collected by GC between reuse and the manifest
//     (reuse must refresh its mtime);
//   - a manifest names a path outside the install dir (zip-slip);
//   - a corrupt, truncated or missing part installs silently;
//   - an older reader takes a parts base for a tar base (must fail closed
//     on the descriptor, before any download);
//   - the stream_seq lease untrust marker is lost in the parts form;
//   - a computer publishes the tar form (the parts form is the only
//     publish form, owner 2026-10-28), or a tar base already published
//     stops installing or seeding;
//   - install over HTTP has no way to fetch a part (must refuse, not fall
//     back);
//   - GC deletes a part some on-disk manifest names.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/storeschema"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func partCount(t *testing.T, artifactsRoot string) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(artifactsRoot, "sha256", PartsNamespace))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			n++
		}
	}
	return n
}

// The rollout selector (owner 2026-10-28: progressive, on for at least one
// computer) uses vmctl's *_ONLY token semantics. An empty list selects no
// one; a class token needs a known class.
func TestPartsSelectorTokens(t *testing.T) {
	de := "computer-59c902c5d46caecc0a180a8557164e72"
	only := ParseComputerSelector(de + ", class:test\n")
	if len(only) != 2 {
		t.Fatalf("parsed %q", only)
	}
	cases := []struct {
		computer, class string
		want            bool
	}{
		{de, "", true},
		{"computer-other", "", false},
		{"computer-other", "test", true},
		{"computer-other", "primary", false},
	}
	for _, c := range cases {
		if got := SelectsComputer(only, c.computer, c.class); got != c.want {
			t.Fatalf("SelectsComputer(%s, %q) = %v, want %v", c.computer, c.class, got, c.want)
		}
	}
	if SelectsComputer(nil, de, "test") || SelectsComputer(ParseComputerSelector("class:"), "x", "") {
		t.Fatal("empty selector selected a computer")
	}
	if !SelectsComputer(ParseComputerSelector("*"), "anything", "") {
		t.Fatal("* did not select")
	}
}

func TestPartsPublishWritesOnlyChangedFiles(t *testing.T) {
	artifactsRoot := t.TempDir()
	src := t.TempDir()
	big := strings.Repeat("table-file-bytes ", 4096)
	writeTree(t, src, map[string]string{
		"runtime.db":                                    "marker",
		"runtime.texture/" + storeschema.File:           "receipt",
		"runtime.texture/.dolt/noms/oldgen/aaaaaaaaaaa": big,
		"runtime.texture/.dolt/noms/journal":            "journal-1",
		"runtime.texture/.dolt/noms/manifest":           "manifest-1",
	})
	p := NewPublisher(artifactsRoot)
	d1, size1, st1, err := p.PublishDirParts(src)
	if err != nil {
		t.Fatalf("first parts publish: %v", err)
	}
	if size1 <= 0 || st1.NewParts == 0 || st1.NewBytes < int64(len(big)) {
		t.Fatalf("first publish stats %+v size %d", st1, size1)
	}
	first := partCount(t, artifactsRoot)

	// Age every part, then change only the small files.
	old := time.Now().Add(-48 * time.Hour)
	partsDir := filepath.Join(artifactsRoot, "sha256", PartsNamespace)
	entries, _ := os.ReadDir(partsDir)
	for _, e := range entries {
		_ = os.Chtimes(filepath.Join(partsDir, e.Name()), old, old)
	}
	writeTree(t, src, map[string]string{
		"runtime.texture/.dolt/noms/journal":  "journal-2",
		"runtime.texture/.dolt/noms/manifest": "manifest-2",
	})
	d2, _, st2, err := p.PublishDirParts(src)
	if err != nil {
		t.Fatalf("second parts publish: %v", err)
	}
	if d1 == d2 {
		t.Fatal("changed tree published the same manifest")
	}
	if st2.NewParts != 2 || st2.NewBytes != int64(len("journal-2")+len("manifest-2")) {
		t.Fatalf("second publish wrote %+v; want only the two changed files", st2)
	}
	if got := partCount(t, artifactsRoot); got != first+2 {
		t.Fatalf("parts after second publish = %d, want %d", got, first+2)
	}
	// The reused big part had its mtime refreshed, so a GC grace window
	// protects it until the manifest that names it exists.
	sum := sha256.Sum256([]byte(big))
	info, err := os.Stat(PartPath(artifactsRoot, hex.EncodeToString(sum[:])))
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().After(old.Add(time.Hour)) {
		t.Fatalf("reused part mtime %v was not refreshed", info.ModTime())
	}

	// The manifest blob lives where a tar blob lives, content-addressed.
	raw, err := os.ReadFile(filepath.Join(artifactsRoot, "sha256", Namespace, d2))
	if err != nil {
		t.Fatal(err)
	}
	if got := sha256.Sum256(raw); hex.EncodeToString(got[:]) != d2 {
		t.Fatal("manifest blob is not addressed by its own digest")
	}
	m, err := ParseManifest(raw)
	if err != nil {
		t.Fatalf("published manifest refused: %v", err)
	}
	// The lease untrust marker travels in the parts form as in the tar form.
	var sawMarker bool
	for _, e := range m.Entries {
		if e.Path == "runtime.texture/"+storeschema.LeaseUntrustedFile {
			sawMarker = true
		}
	}
	if !sawMarker {
		t.Fatal("parts manifest lacks the lease untrust marker")
	}

	// Materialize from local parts reproduces the tree.
	dst := t.TempDir()
	if err := materializeLocalBase(artifactsRoot, filepath.Join(artifactsRoot, "sha256", Namespace, d2), d2, FormatParts, dst); err != nil {
		t.Fatalf("materialize: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "runtime.texture/.dolt/noms/oldgen/aaaaaaaaaaa"))
	if err != nil || string(got) != big {
		t.Fatalf("materialized table file mismatch: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(dst, "runtime.texture/.dolt/noms/journal")); string(got) != "journal-2" {
		t.Fatalf("materialized journal = %q", got)
	}
}

func TestPartsMaterializeRefusesCorruptMissingAndEscapingParts(t *testing.T) {
	artifactsRoot := t.TempDir()
	src := t.TempDir()
	writeTree(t, src, map[string]string{"a/b.txt": "hello parts", "c.txt": "second"})
	p := NewPublisher(artifactsRoot)
	digest, _, _, err := p.PublishDirParts(src)
	if err != nil {
		t.Fatal(err)
	}
	blob := filepath.Join(artifactsRoot, "sha256", Namespace, digest)
	sum := sha256.Sum256([]byte("hello parts"))
	part := PartPath(artifactsRoot, hex.EncodeToString(sum[:]))

	// Corrupt part (same size): refused.
	if err := os.WriteFile(part, []byte("HELLO PARTS"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := materializeLocalBase(artifactsRoot, blob, digest, FormatParts, t.TempDir()); !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("corrupt part: err = %v, want ErrBaseRefused", err)
	}
	// Truncated part: refused.
	if err := os.WriteFile(part, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := materializeLocalBase(artifactsRoot, blob, digest, FormatParts, t.TempDir()); !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("truncated part: err = %v, want ErrBaseRefused", err)
	}
	// Missing part: refused.
	_ = os.Remove(part)
	if err := materializeLocalBase(artifactsRoot, blob, digest, FormatParts, t.TempDir()); !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("missing part: err = %v, want ErrBaseRefused", err)
	}
	// Manifest blob that is not its digest: refused.
	if err := materializeLocalBase(artifactsRoot, blob, strings.Repeat("0", 64), FormatParts, t.TempDir()); !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("wrong manifest digest: err = %v, want ErrBaseRefused", err)
	}

	zero := strings.Repeat("a", 64)
	for _, bad := range []string{
		`{"format":"` + ManifestFormat + `","entries":[{"path":"../x","type":"file","mode":420,"size":1,"sha256":"` + zero + `"}]}`,
		`{"format":"` + ManifestFormat + `","entries":[{"path":"/etc/x","type":"file","mode":420,"size":1,"sha256":"` + zero + `"}]}`,
		`{"format":"` + ManifestFormat + `","entries":[{"path":"a/../../x","type":"file","mode":420,"size":1,"sha256":"` + zero + `"}]}`,
		`{"format":"` + ManifestFormat + `","entries":[{"path":"x","type":"file","mode":420,"size":1,"sha256":"nothex"}]}`,
		`{"format":"` + ManifestFormat + `","entries":[{"path":"x","type":"link","mode":420}]}`,
		`{"format":"` + ManifestFormat + `","entries":[{"path":"x","type":"dir","mode":493},{"path":"x","type":"dir","mode":493}]}`,
		`{"format":"other/v9","entries":[]}`,
	} {
		if _, err := ParseManifest([]byte(bad)); err == nil {
			t.Fatalf("manifest accepted: %s", bad)
		}
	}
}

func TestPartsDescriptorFailsClosedForOlderReaders(t *testing.T) {
	d := validTestDescriptor()
	d.Format = FormatParts
	d.VocabularyVersion = VocabularyVersionForForm("", PublishFormParts)
	if err := d.Validate(); err != nil {
		t.Fatalf("parts descriptor refused: %v", err)
	}
	if d.EffectiveVocabularyVersion() != CurrentVocabularyVersion {
		t.Fatalf("effective vocabulary = %q", d.EffectiveVocabularyVersion())
	}
	// A build from before S4 only trims the live_captured suffix; what it
	// would see must be outside its known set, so it refuses the descriptor.
	if IsKnownVocabularyVersion(strings.TrimSuffix(d.VocabularyVersion, liveCapturedVocabularySuffix)) {
		t.Fatalf("vocabulary %q is known to a pre-S4 reader", d.VocabularyVersion)
	}
	root := validTestDescriptor()
	root.Origin = OriginLiveCaptured
	root.Format = FormatParts
	root.VocabularyVersion = VocabularyVersionForForm(OriginLiveCaptured, PublishFormParts)
	if err := root.Validate(); err != nil {
		t.Fatalf("captured parts descriptor refused: %v", err)
	}
	if root.EffectiveVocabularyVersion() != CurrentVocabularyVersion {
		t.Fatalf("captured effective vocabulary = %q", root.EffectiveVocabularyVersion())
	}
	// The format and its suffix agree both ways.
	mismatch := validTestDescriptor()
	mismatch.Format = FormatParts
	if err := mismatch.Validate(); err == nil {
		t.Fatal("parts format without the vocabulary suffix accepted")
	}
	mismatch = validTestDescriptor()
	mismatch.VocabularyVersion = CurrentVocabularyVersion + partsVocabularySuffix
	if err := mismatch.Validate(); err == nil {
		t.Fatal("parts suffix without the parts format accepted")
	}
	mismatch = validTestDescriptor()
	mismatch.Format = "parts/v9"
	if err := mismatch.Validate(); err == nil {
		t.Fatal("unknown format accepted")
	}
	// The tar form is unchanged.
	if VocabularyVersionForForm("", PublishFormTar) != CurrentVocabularyVersion {
		t.Fatal("tar form changed its vocabulary")
	}
}

type partsBaseSource struct {
	*fakeBaseSource
	artifactsRoot string
	asked         []string
}

func (s *partsBaseSource) DownloadPart(ctx context.Context, computerID, baseRef, partDigest string, dst io.Writer) error {
	s.asked = append(s.asked, partDigest)
	f, err := os.Open(PartPath(s.artifactsRoot, partDigest))
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(dst, f)
	return err
}

func TestRebuilderPartsFormPublishesInstallsAndSeeds(t *testing.T) {
	ctx := context.Background()
	artifactsRoot := t.TempDir()
	computerID := "computer-parts-test"
	keyMaterial := make([]byte, 32)
	for i := range keyMaterial {
		keyMaterial[i] = byte(i + 11)
	}
	digests := buildInstallChain(t, artifactsRoot, computerID)
	disk := NewDiskEventSource(artifactsRoot, computerID)

	rebuilder, err := NewRebuilder(Config{
		ComputerID: computerID, TargetHead: digests[0], ArtifactsRoot: artifactsRoot,
		ScratchDir: t.TempDir(), KeyMaterial: keyMaterial, BatchSize: 100, MemoryLimitRSS: 512 * 1024 * 1024, PublishParts: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := rebuilder.Run(ctx, fixtureReplay(NewDiskEventSource(artifactsRoot, computerID, digests[0])))
	if err != nil {
		t.Fatalf("parts rebuild: %v", err)
	}
	if result.Descriptor.Format != FormatParts || result.Descriptor.VocabularyVersion != CurrentVocabularyVersion+partsVocabularySuffix {
		t.Fatalf("descriptor format %q vocabulary %q", result.Descriptor.Format, result.Descriptor.VocabularyVersion)
	}
	blob, err := os.ReadFile(result.BlobPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(blob, []byte(`{"format":"`+ManifestFormat+`"`)) {
		t.Fatalf("parts blob is not a manifest: %.40q", blob)
	}

	// A source that cannot serve parts refuses the install; it never falls
	// back to reading the manifest as a tar.
	plain := &fakeBaseSource{sequence: 1, baseRef: result.Descriptor.BlobSHA256, descriptor: result.Descriptor, blob: blob, disk: disk}
	if _, err := InstallVerifiedBase(ctx, plain, t.TempDir(), "runtime.db", computerID, digests[2], 3); !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("install without a part source: err = %v, want ErrBaseRefused", err)
	}

	src := &partsBaseSource{fakeBaseSource: plain, artifactsRoot: artifactsRoot}
	installDir := t.TempDir()
	descriptor, err := InstallVerifiedBase(ctx, src, installDir, "runtime.db", computerID, digests[2], 3)
	if err != nil {
		t.Fatalf("parts install refused: %v", err)
	}
	if descriptor.Sequence != 1 || len(src.asked) == 0 {
		t.Fatalf("parts install: sequence %d, parts fetched %d", descriptor.Sequence, len(src.asked))
	}
	if _, err := os.Stat(filepath.Join(installDir, "runtime.texture", storeschema.LeaseUntrustedFile)); err != nil {
		t.Fatalf("installed parts base lacks the lease untrust marker: %v", err)
	}

	// A corrupt part refuses the HTTP-style install.
	m, err := ParseManifest(blob)
	if err != nil {
		t.Fatal(err)
	}
	var victim string
	for _, e := range m.Entries {
		if e.Type == "file" && e.Size > 0 {
			victim = e.SHA256
			break
		}
	}
	saved, err := os.ReadFile(PartPath(artifactsRoot, victim))
	if err != nil {
		t.Fatal(err)
	}
	flipped := append([]byte{}, saved...)
	flipped[0] ^= 0xff
	if err := os.WriteFile(PartPath(artifactsRoot, victim), flipped, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallVerifiedBase(ctx, src, t.TempDir(), "runtime.db", computerID, digests[2], 3); !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("corrupt part install: err = %v, want ErrBaseRefused", err)
	}
	if err := os.WriteFile(PartPath(artifactsRoot, victim), saved, 0o644); err != nil {
		t.Fatal(err)
	}

	// The next rebuild seeds from the parts base (host-local read) and
	// publishes again in the parts form, reusing unchanged parts.
	before := partCount(t, artifactsRoot)
	next, err := NewRebuilder(Config{
		ComputerID: computerID, TargetHead: digests[2], ArtifactsRoot: artifactsRoot,
		ScratchDir: t.TempDir(), KeyMaterial: keyMaterial, BatchSize: 100, MemoryLimitRSS: 512 * 1024 * 1024, PublishParts: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	seeded, err := next.Run(ctx, fixtureReplay(NewDiskEventSource(artifactsRoot, computerID, digests[2])))
	if err != nil {
		t.Fatalf("seeded parts rebuild: %v", err)
	}
	if seeded.Seed == nil || seeded.Seed.BlobSHA256 != result.Descriptor.BlobSHA256 {
		t.Fatalf("second rebuild did not seed from the parts base: %+v", seeded.Seed)
	}
	if seeded.Descriptor.Format != FormatParts {
		t.Fatalf("second rebuild format %q", seeded.Descriptor.Format)
	}
	t.Logf("parts before %d after %d", before, partCount(t, artifactsRoot))

	// GC liveness: every part named by an on-disk manifest is reported.
	live, err := ManifestPartsInBaseDir(artifactsRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range []string{result.BlobPath, seeded.BlobPath} {
		raw, _ := os.ReadFile(mf)
		mm, err := ParseManifest(raw)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range mm.PartDigests() {
			if _, ok := live[d]; !ok {
				t.Fatalf("part %s named by %s is not in the live set", d, filepath.Base(mf))
			}
		}
	}
}

// A tar base published before S4 still installs (over a source that also
// serves parts) and still seeds; the rebuild it seeds publishes parts.
func TestTarBaseFromBeforeS4InstallsAndSeedsAPartsBase(t *testing.T) {
	ctx := context.Background()
	artifactsRoot := t.TempDir()
	computerID := "computer-parts-legacy"
	keyMaterial := make([]byte, 32)
	for i := range keyMaterial {
		keyMaterial[i] = byte(i + 13)
	}
	digests := buildInstallChain(t, artifactsRoot, computerID)
	disk := NewDiskEventSource(artifactsRoot, computerID)
	rebuilder, err := NewRebuilder(Config{
		ComputerID: computerID, TargetHead: digests[0], ArtifactsRoot: artifactsRoot,
		ScratchDir: t.TempDir(), KeyMaterial: keyMaterial, BatchSize: 100, MemoryLimitRSS: 512 * 1024 * 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := rebuilder.Run(ctx, fixtureReplay(NewDiskEventSource(artifactsRoot, computerID, digests[0])))
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Descriptor.Format != "" || legacy.Descriptor.VocabularyVersion != CurrentVocabularyVersion {
		t.Fatalf("tar base: format %q vocabulary %q", legacy.Descriptor.Format, legacy.Descriptor.VocabularyVersion)
	}

	blob, err := os.ReadFile(legacy.BlobPath)
	if err != nil {
		t.Fatal(err)
	}
	src := &partsBaseSource{fakeBaseSource: &fakeBaseSource{sequence: 1, baseRef: legacy.Descriptor.BlobSHA256, descriptor: legacy.Descriptor, blob: blob, disk: disk}, artifactsRoot: artifactsRoot}
	if _, err := InstallVerifiedBase(ctx, src, t.TempDir(), "runtime.db", computerID, digests[2], 3); err != nil {
		t.Fatalf("tar base install refused: %v", err)
	}
	if len(src.asked) != 0 {
		t.Fatalf("tar base install fetched %d parts", len(src.asked))
	}

	next, err := NewRebuilder(Config{
		ComputerID: computerID, TargetHead: digests[2], ArtifactsRoot: artifactsRoot,
		ScratchDir: t.TempDir(), KeyMaterial: keyMaterial, BatchSize: 100, MemoryLimitRSS: 512 * 1024 * 1024, PublishParts: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	seeded, err := next.Run(ctx, fixtureReplay(NewDiskEventSource(artifactsRoot, computerID, digests[2])))
	if err != nil {
		t.Fatalf("rebuild seeded from a tar base: %v", err)
	}
	if seeded.Seed == nil || seeded.Seed.BlobSHA256 != legacy.Descriptor.BlobSHA256 {
		t.Fatalf("rebuild did not seed from the tar base: %+v", seeded.Seed)
	}
	if seeded.Descriptor.Format != FormatParts {
		t.Fatalf("rebuild after a tar seed published format %q, want parts", seeded.Descriptor.Format)
	}
}

func TestManifestPartsInBaseDirFailsClosedOnTornManifest(t *testing.T) {
	artifactsRoot := t.TempDir()
	dir := filepath.Join(artifactsRoot, "sha256", Namespace)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A tar blob is ignored; a manifest-prefixed blob that does not parse
	// fails the scan, so GC sweeps no parts that run.
	if err := os.WriteFile(filepath.Join(dir, strings.Repeat("b", 64)), []byte("runtime.db\x00\x00tar"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ManifestPartsInBaseDir(artifactsRoot); err != nil {
		t.Fatalf("tar blob broke the scan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, strings.Repeat("c", 64)), []byte(`{"format":"`+ManifestFormat+`","entries":[`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ManifestPartsInBaseDir(artifactsRoot); err == nil {
		t.Fatal("torn manifest did not fail the parts live set")
	}
}

// A captured epoch root publishes in the parts form when its computer is
// selected: the descriptor carries both suffixes and the read-back runs
// over the parts.
func TestCapturedRootInThePartsForm(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-capture-parts"
	root, mount, digests, _ := captureFixture(t, computerID, 5, 3)
	result, err := PublishCapturedBase(ctx, CaptureConfig{
		ComputerID: computerID, StoreMarker: filepath.Join(mount, "state"),
		ArtifactsRoot: root, TapeHeadAt: tapeOf(digests), PublishParts: true,
	})
	if err != nil {
		t.Fatalf("PublishCapturedBase parts: %v", err)
	}
	d := result.Descriptor
	if d.Format != FormatParts || d.VocabularyVersion != CurrentVocabularyVersion+liveCapturedVocabularySuffix+partsVocabularySuffix || d.Origin != OriginLiveCaptured {
		t.Fatalf("captured parts descriptor: format %q vocabulary %q origin %q", d.Format, d.VocabularyVersion, d.Origin)
	}
	if err := VerifyPublishedBase(ctx, root, d); err != nil {
		t.Fatalf("captured parts base does not verify: %v", err)
	}
	dst := t.TempDir()
	if err := materializeLocalBase(root, result.BlobPath, d.BlobSHA256, d.Format, dst); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dst)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, ",") != "runtime.db,runtime.texture" {
		t.Fatalf("parts base holds %v, want the store alone", names)
	}
}
