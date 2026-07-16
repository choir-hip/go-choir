package computerversion

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
)

func TestImmutableInputsPinAndResolve(t *testing.T) {
	catalog, closeCatalog := openTestInputCatalog(t)
	defer closeCatalog()
	now := time.Date(2026, 7, 16, 4, 0, 0, 0, time.UTC)

	closure, err := NewCodeClosure("d87bdc446ecc28585c3bc08d4d469b9f94d3c246", []CodeArtifact{
		{Name: "sandbox", SHA256: hash64('a'), URI: contentAddressedURI("nix-store+sha256", hash64('a'), "nix/store/sandbox")},
		{Name: "rootfs", SHA256: hash64('b'), URI: contentAddressedURI("artifact+sha256", hash64('b'), "images/rootfs.ext4")},
	}, now)
	if err != nil {
		t.Fatalf("new code closure: %v", err)
	}
	pinnedClosure, err := catalog.PinCode(context.Background(), closure)
	if err != nil {
		t.Fatalf("pin code closure: %v", err)
	}
	if pinnedClosure.Ref != closure.Ref {
		t.Fatalf("pinned code ref = %q, want %q", pinnedClosure.Ref, closure.Ref)
	}

	program, err := NewArtifactProgram([]ArtifactProgramEntry{
		{Kind: "embedded_dolt_export", ContentSHA256: hash64('c'), ArtifactURI: contentAddressedURI("artifact+sha256", hash64('c'), "owner/texture.sql")},
		{Kind: "actor_recovery_log", ContentSHA256: hash64('d'), ArtifactURI: contentAddressedURI("artifact+sha256", hash64('d'), "owner/state-actor.db")},
	}, now)
	if err != nil {
		t.Fatalf("new artifact program: %v", err)
	}
	pinnedProgram, err := catalog.PinArtifactProgram(context.Background(), program)
	if err != nil {
		t.Fatalf("pin artifact program: %v", err)
	}
	if pinnedProgram.Ref != program.Ref || pinnedProgram.Entries[1].PreviousEntryHash != pinnedProgram.Entries[0].EntryHash {
		t.Fatalf("pinned artifact program chain mismatch: %+v", pinnedProgram)
	}

	resolvedCode, err := catalog.ResolveCode(context.Background(), closure.Ref)
	if err != nil {
		t.Fatalf("resolve code: %v", err)
	}
	resolvedProgram, err := catalog.ResolveArtifactProgram(context.Background(), program.Ref)
	if err != nil {
		t.Fatalf("resolve artifact program: %v", err)
	}
	if err := resolvedCode.Verify(); err != nil {
		t.Fatalf("verify resolved code: %v", err)
	}
	if err := resolvedProgram.Verify(); err != nil {
		t.Fatalf("verify resolved program: %v", err)
	}
}

func TestImmutableInputsSurviveStoreRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.db")
	now := time.Date(2026, 7, 16, 4, 0, 0, 0, time.UTC)
	closure, err := NewCodeClosure(hash64('f'), []CodeArtifact{{Name: "sandbox", SHA256: hash64('1'), URI: contentAddressedURI("nix-store+sha256", hash64('1'), "nix/store/sandbox")}}, now)
	if err != nil {
		t.Fatal(err)
	}
	program, err := NewArtifactProgram([]ArtifactProgramEntry{{Kind: "base_journal", ContentSHA256: hash64('2'), ArtifactURI: contentAddressedURI("artifact+sha256", hash64('2'), "journal")}}, now)
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	catalog := NewSQLInputCatalog(first.DB(), declarationOnlyTestVerifier{})
	if err := catalog.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.PinCode(context.Background(), closure); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.PinArtifactProgram(context.Background(), program); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = second.Close() }()
	restarted := NewSQLInputCatalog(second.DB())
	if got, err := restarted.ResolveCode(context.Background(), closure.Ref); err != nil || got.Ref != closure.Ref {
		t.Fatalf("resolve code after restart: got=%+v err=%v", got, err)
	}
	if got, err := restarted.ResolveArtifactProgram(context.Background(), program.Ref); err != nil || got.Ref != program.Ref {
		t.Fatalf("resolve program after restart: got=%+v err=%v", got, err)
	}
}

func TestImmutableInputsRejectFloatingSourceAndUnboundLocators(t *testing.T) {
	digest := hash64('a')
	if _, err := NewCodeClosure("main", []CodeArtifact{{
		Name: "sandbox", SHA256: digest, URI: contentAddressedURI("artifact+sha256", digest, "sandbox"),
	}}, time.Date(2026, 7, 16, 4, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("floating source ref became a CodeRef")
	}
	if _, err := NewArtifactProgram([]ArtifactProgramEntry{{
		Kind: "test", ContentSHA256: digest, ArtifactURI: contentAddressedURI("artifact+sha256", hash64('b'), "state"),
	}}, time.Date(2026, 7, 16, 4, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("artifact URI not bound to its declared digest became an ArtifactProgramRef")
	}
	if _, err := NewCodeClosure(hash64('f'), []CodeArtifact{{
		Name: "rootfs", SHA256: digest, URI: "file+sha256://" + digest + "/var/lib/rootfs.ext4",
	}}, time.Date(2026, 7, 16, 4, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("mutable file URI became a CodeRef")
	}
}

func TestArtifactProgramRejectsTampering(t *testing.T) {
	program, err := NewArtifactProgram([]ArtifactProgramEntry{{
		Kind: "file_manifest", ContentSHA256: hash64('e'), ArtifactURI: contentAddressedURI("artifact+sha256", hash64('e'), "owner/files"),
	}}, time.Date(2026, 7, 16, 4, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	program.Entries[0].ArtifactURI = contentAddressedURI("artifact+sha256", hash64('a'), "attacker/files")
	if err := program.Verify(); err == nil {
		t.Fatal("tampered artifact program verified")
	}
}

func TestImmutableInputResolverFailsClosed(t *testing.T) {
	catalog, closeCatalog := openTestInputCatalog(t)
	defer closeCatalog()
	if _, err := catalog.ResolveCode(context.Background(), CodeRef("code:sha256:"+hash64('f'))); !errors.Is(err, ErrInputNotFound) {
		t.Fatalf("missing code error = %v", err)
	}
	if _, err := catalog.ResolveArtifactProgram(context.Background(), ArtifactProgramRef("artifact-program:sha256:"+hash64('f'))); !errors.Is(err, ErrInputNotFound) {
		t.Fatalf("missing program error = %v", err)
	}
}

func openTestInputCatalog(t *testing.T) (*SQLInputCatalog, func()) {
	t.Helper()
	productStore, err := store.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatalf("open embedded Dolt: %v", err)
	}
	catalog := NewSQLInputCatalog(productStore.DB(), declarationOnlyTestVerifier{})
	if err := catalog.EnsureSchema(context.Background()); err != nil {
		_ = productStore.Close()
		t.Fatalf("ensure input schema: %v", err)
	}
	return catalog, func() { _ = productStore.Close() }
}

func hash64(ch byte) string {
	value := make([]byte, 64)
	for i := range value {
		value[i] = ch
	}
	return string(value)
}

type declarationOnlyTestVerifier struct{}

func (declarationOnlyTestVerifier) VerifyArtifact(_ context.Context, uri, digest string) error {
	if !validContentAddressedURI(uri, digest) {
		return errors.New("invalid digest-bound URI")
	}
	return nil
}

func TestLocalArtifactContentVerifierRefusesMissingAndMismatchedContent(t *testing.T) {
	root := t.TempDir()
	content := []byte("immutable artifact bytes")
	digest := immutableInputSHA256Hex(content)
	path := filepath.Join(root, "artifact.bin")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	verifier := NewLocalArtifactContentVerifier(root)
	uri := contentAddressedURI("artifact+sha256", digest, "artifact.bin")
	if err := verifier.VerifyArtifact(context.Background(), uri, digest); err != nil {
		t.Fatalf("verify immutable artifact: %v", err)
	}
	if err := verifier.VerifyArtifact(context.Background(), contentAddressedURI("artifact+sha256", hash64('a'), "artifact.bin"), hash64('a')); err == nil {
		t.Fatal("mismatched artifact content verified")
	}
	if err := verifier.VerifyArtifact(context.Background(), contentAddressedURI("artifact+sha256", digest, "missing.bin"), digest); err == nil {
		t.Fatal("missing artifact content verified")
	}
}
