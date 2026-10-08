package projectionbase

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/recoveryplan"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// ErrBaseRefused is the typed visible refusal for every required-base
// failure: missing, foreign, corrupt, non-ancestor, or incompatible. Recovery
// paths map it to a refusal that leaves the active route and accepted
// realization unchanged; it must never fall through to genesis replay.
var ErrBaseRefused = recoveryplan.ErrBaseRefused

// ErrReceiptUnverifiable is the typed refusal for replay sources that cannot
// verify original signed event head receipts. Offline publication never replays
// events on an unverified receipt path: a source without receipt trust fails
// closed instead of degrading to a no-op check.
var ErrReceiptUnverifiable = errors.New("event receipt verification unavailable")

// defaultReceiptSignerDomain is the corpusd EventHeadReceipt signer domain.
const defaultReceiptSignerDomain = "platform-control"

// PinnedReceiptKeyResolver is the operator trust root for offline receipt
// verification: it resolves the corpusd EventHeadReceipt signer to one pinned
// ed25519 public key. A missing key, an untrusted signer domain, or (when KeyID
// is set) a different key id fails closed. An empty KeyID pins key material
// only; an empty SignerDomain pins platform-control.
type PinnedReceiptKeyResolver struct {
	SignerDomain string
	KeyID        string
	PublicKey    ed25519.PublicKey
}

// ResolveReceiptKey implements computerevent.KeyResolver.
func (r PinnedReceiptKeyResolver) ResolveReceiptKey(domain, computerID, keyID string, sequence uint64, issuedAt time.Time) (ed25519.PublicKey, error) {
	if len(r.PublicKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("%w: pinned receipt public key is not configured", ErrReceiptUnverifiable)
	}
	wantDomain := strings.TrimSpace(r.SignerDomain)
	if wantDomain == "" {
		wantDomain = defaultReceiptSignerDomain
	}
	if strings.TrimSpace(domain) != wantDomain {
		return nil, fmt.Errorf("%w: signer domain %q is not the pinned trust root %q", ErrReceiptUnverifiable, domain, wantDomain)
	}
	if strings.TrimSpace(r.KeyID) != "" && strings.TrimSpace(keyID) != strings.TrimSpace(r.KeyID) {
		return nil, fmt.Errorf("%w: signer key %q is not the pinned key %q", ErrReceiptUnverifiable, keyID, r.KeyID)
	}
	return append(ed25519.PublicKey(nil), r.PublicKey...), nil
}

// VerifyPublishedBase proves the published blob and descriptor pair as an
// installer would consume them: the blob is re-read and digest-checked, unpacked
// to a disposable directory, and its store head and installed content witness
// must equal the descriptor. Publication runs this before the descriptor
// sidecar exists, so a mismatched witness is never advertised.
func VerifyPublishedBase(ctx context.Context, artifactsRoot string, d Descriptor) error {
	if err := d.Validate(); err != nil {
		return fmt.Errorf("%w: published descriptor refused: %v", ErrBaseRefused, err)
	}
	blobPath := filepath.Join(filepath.Clean(artifactsRoot), "sha256", Namespace, d.BlobSHA256)
	info, err := os.Stat(blobPath)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("%w: published blob %s is missing", ErrBaseRefused, d.BlobSHA256)
	}
	if info.Size() != d.BlobSizeBytes {
		return fmt.Errorf("%w: published blob size %d does not match descriptor %d", ErrBaseRefused, info.Size(), d.BlobSizeBytes)
	}
	// Unpack on the artifacts filesystem: a base blob is tens of GiB, and the
	// system temp directory may be a memory-backed tmpfs. The read-back
	// directory is hidden and removed before returning.
	dir, err := os.MkdirTemp(filepath.Clean(artifactsRoot), ".projection-base-readback-*")
	if err != nil {
		return fmt.Errorf("projection base: create read-back directory: %w", err)
	}
	defer os.RemoveAll(dir)
	if err := unpackVerified(blobPath, dir, d.BlobSHA256); err != nil {
		return err
	}
	if err := verifyInstalledHead(dir, "runtime.db", d); err != nil {
		return err
	}
	workspace := choirstore.TextureWorkspacePath(filepath.Join(dir, "runtime.db"))
	got, err := witnessForWorkspace(ctx, d.ComputerID, d.CanonicalHead, workspace)
	if err != nil {
		return fmt.Errorf("%w: read-back witness: %v", ErrBaseRefused, err)
	}
	if err := selfdevprotocol.WitnessContentMatches(got, d.VMLocalContentWitness); err != nil {
		return fmt.Errorf("%w: read-back witness mismatch: %v", ErrBaseRefused, err)
	}
	return nil
}

// VerifyForRecovery authenticates a required base as a verified accelerator
// for this computer and chain position before installation. It binds computer,
// watermark W with its canonical head, blob identity, reducer/schema/
// vocabulary compatibility, and witness — but it cannot prove ancestry alone:
// a well-formed foreign head at the same sequence passes here and is caught
// by VerifyTailHead against immutable tape.
func (d Descriptor) VerifyForRecovery(computerID, targetHead string, targetSequence uint64) error {
	if err := d.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrBaseRefused, err)
	}
	if strings.TrimSpace(computerID) == "" || d.ComputerID != strings.TrimSpace(computerID) {
		return fmt.Errorf("%w: base computer %q is not this computer", ErrBaseRefused, d.ComputerID)
	}
	if !computerevent.IsSHA256(targetHead) {
		return fmt.Errorf("%w: recovery target head must be lowercase SHA-256", ErrBaseRefused)
	}
	if targetSequence == 0 {
		return fmt.Errorf("%w: recovery target sequence must be positive", ErrBaseRefused)
	}
	if d.Sequence > targetSequence {
		return fmt.Errorf("%w: base watermark %d is after recovery target %d", ErrBaseRefused, d.Sequence, targetSequence)
	}
	if d.Sequence == targetSequence && d.CanonicalHead != strings.ToLower(strings.TrimSpace(targetHead)) {
		return fmt.Errorf("%w: base head does not match recovery target", ErrBaseRefused)
	}
	if d.ReducerVersion != computerevent.ReducerVersionV1 {
		return fmt.Errorf("%w: base reducer version %d is incompatible with live %d", ErrBaseRefused, d.ReducerVersion, computerevent.ReducerVersionV1)
	}
	if d.SchemaVersion != computerevent.SchemaVersionV1 {
		return fmt.Errorf("%w: base schema version %d is incompatible with live %d", ErrBaseRefused, d.SchemaVersion, computerevent.SchemaVersionV1)
	}
	return nil
}

// VerifyTailHead proves base ancestry from immutable tape: the first event of
// the tail (W,H] must chain exactly from the base head. A non-ancestor W —
// right computer, right sequence, foreign head — refuses here, never in
// replay mechanics. The full tail still replays event-by-event afterwards;
// this gate only admits the tail start.
func (d Descriptor) VerifyTailHead(first computerevent.Event) error {
	if first.ComputerID != d.ComputerID {
		return fmt.Errorf("%w: tail computer %q is not base computer %q", ErrBaseRefused, first.ComputerID, d.ComputerID)
	}
	if first.Sequence != d.Sequence+1 {
		return fmt.Errorf("%w: tail starts at sequence %d, want %d", ErrBaseRefused, first.Sequence, d.Sequence+1)
	}
	if first.PreviousHead != d.CanonicalHead {
		return fmt.Errorf("%w: tail does not chain from base head", ErrBaseRefused)
	}
	return nil
}
