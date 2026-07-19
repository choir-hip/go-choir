package transaction

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
)

func TestClassifierBasic(t *testing.T) {
	c := NewClassifier()
	changes := []capsule.FileChange{
		{Path: "/var/lib/dolt/choir/items.sql", Kind: capsule.ChangeAdded, Mode: 0o644},
		{Path: "/home/user/src/main.go", Kind: capsule.ChangeModified, Mode: 0o644},
		{Path: "/tmp/cache.txt", Kind: capsule.ChangeAdded, Mode: 0o644},
		{Path: "/unknown/path.txt", Kind: capsule.ChangeAdded, Mode: 0o644},
	}

	result := c.Classify(changes)

	if len(result.Groups[LedgerDolt]) != 1 {
		t.Errorf("Dolt group: expected 1, got %d", len(result.Groups[LedgerDolt]))
	}
	if len(result.Groups[LedgerSource]) != 1 {
		t.Errorf("Source group: expected 1, got %d", len(result.Groups[LedgerSource]))
	}
	if len(result.Ignored) != 1 {
		t.Errorf("Ignored: expected 1, got %d", len(result.Ignored))
	}
	if len(result.Unknown) != 1 {
		t.Errorf("Unknown: expected 1, got %d", len(result.Unknown))
	}
	if !result.HasUnknown() {
		t.Error("HasUnknown should be true")
	}
	if result.Digest == "" {
		t.Error("Digest should not be empty")
	}
}

func TestClassifierRejectsUnknown(t *testing.T) {
	c := NewClassifier()
	builder := NewTransactionBuilder(c)

	changes := []capsule.FileChange{
		{Path: "/var/lib/dolt/choir/items.sql", Kind: capsule.ChangeAdded, Mode: 0o644},
		{Path: "/unknown/path.txt", Kind: capsule.ChangeAdded, Mode: 0o644},
	}

	record, err := builder.BuildBundleFromDiff("capsule-1", changes)
	if err != nil {
		t.Fatalf("build transaction: %v", err)
	}
	if !record.Rejected {
		t.Error("record should be rejected due to unknown paths")
	}
	if record.RejectReason == "" {
		t.Error("reject reason should not be empty")
	}
}

func TestTransactionBuilderAcceptsKnownPaths(t *testing.T) {
	c := NewClassifier()
	builder := NewTransactionBuilder(c)

	changes := []capsule.FileChange{
		{Path: "/var/lib/dolt/choir/items.sql", Kind: capsule.ChangeAdded, Mode: 0o644},
		{Path: "/home/user/src/main.go", Kind: capsule.ChangeModified, Mode: 0o644},
		{Path: "/var/lib/blob/abc123", Kind: capsule.ChangeAdded, Mode: 0o644},
	}

	record, err := builder.BuildBundleFromDiff("capsule-1", changes)
	if err != nil {
		t.Fatalf("build transaction: %v", err)
	}
	if record.Rejected {
		t.Errorf("record should not be rejected, but: %s", record.RejectReason)
	}
	if record.CapsuleIdentity != "capsule-1" {
		t.Errorf("capsule ID: got %q, want %q", record.CapsuleIdentity, "capsule-1")
	}
	if record.ClassifierV != "v1" {
		t.Errorf("classifier version: got %q, want %q", record.ClassifierV, "v1")
	}
	if len(record.Groups["Dolt"]) != 1 {
		t.Errorf("Dolt group: expected 1, got %d", len(record.Groups["Dolt"]))
	}
	if len(record.Groups["Source"]) != 1 {
		t.Errorf("Source group: expected 1, got %d", len(record.Groups["Source"]))
	}
	if len(record.Groups["Blob"]) != 1 {
		t.Errorf("Blob group: expected 1, got %d", len(record.Groups["Blob"]))
	}
}

func TestCapsuleEffectBundleContentDigestBindsCompleteSchemaBeforeDetachedVerifier(t *testing.T) {
	digest := func(fill byte) string { return string(make([]byte, 0)) + repeatHex(fill) }
	bundle := CapsuleEffectBundle{
		BundleVersion: 1, ComputerID: "computer-1", BaseEventHead: digest('a'),
		TrajectoryRef: "trajectory-1", CapsuleIdentity: "capsule-1", CapabilityPolicyDigest: digest('b'),
		SourceTreeRef:         "source-tree:sha256:" + digest('c'),
		OrderedFileEffects:    []ChangeRecord{{Path: "var/lib/artifact/release/bin/sandbox", Kind: "added", Mode: 0o755}},
		GeneratedArtifactRefs: []string{"artifact:sha256:" + digest('d')},
		BuildRecipeRef:        "capsule-exec:sha256:" + digest('e'),
		RuntimeArtifactRef:    "runtime-artifact:sha256:" + digest('f'),
		TestReceipts:          []string{"capsule-exec:sha256:" + digest('1')}, VerifierReceipts: []string{},
		DependencyToolchainRefs: []string{"capsule-exec:sha256:" + digest('2')},
		ResourceReceipts:        []string{"resource:sha256:" + digest('3')},
		RuntimeFiles:            []capsule.FrozenReleaseFile{{Path: "bin/sandbox", SHA256: digest('d'), Mode: 0o755}},
		Groups:                  map[string][]ChangeRecord{}, Ignored: []ChangeRecord{},
	}
	var err error
	bundle.ContentDigest, err = bundle.ComputeContentDigest()
	if err != nil || bundle.Validate(false) != nil {
		t.Fatalf("valid draft refused: digest=%s err=%v validate=%v", bundle.ContentDigest, err, bundle.Validate(false))
	}
	bundle.VerifierReceipts = []string{digest('4')}
	if err := bundle.Validate(true); err != nil {
		t.Fatal(err)
	}
	tampered := bundle
	tampered.BuildRecipeRef = "capsule-exec:sha256:" + digest('5')
	if err := tampered.Validate(true); err == nil {
		t.Fatal("bundle accepted a build recipe substitution")
	}
}

func repeatHex(fill byte) string {
	out := make([]byte, 64)
	for index := range out {
		out[index] = fill
	}
	return string(out)
}
