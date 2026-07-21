package updater

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

const ActivationIntentReceiptKind = "ActivationIntentReceipt"

var ErrNoAuthorizedRelease = errors.New("updater: no authorized dynamic release")

type AdmittedRelease struct {
	ReleaseDigest string `json:"release_digest"`
	ReleaseRoot   string `json:"release_root"`
	SandboxPath   string `json:"sandbox_path"`
	SkillsRoot    string `json:"skills_root"`
	ReceiptKind   string `json:"receipt_kind"`
	ReceiptID     string `json:"receipt_id"`
}

func (u *Updater) AdmitCurrent(ctx context.Context) (AdmittedRelease, error) {
	if u == nil {
		return AdmittedRelease{}, ErrNoAuthorizedRelease
	}
	digest, releaseRoot, err := u.currentRelease()
	if err != nil || digest == "" {
		return AdmittedRelease{}, fmt.Errorf("%w: current pointer", ErrNoAuthorizedRelease)
	}
	manifest, err := readReleaseManifest(releaseRoot)
	if err != nil || manifest.ContentDigest != digest || manifest.ComputerID != u.computerID {
		return AdmittedRelease{}, fmt.Errorf("%w: current manifest", ErrNoAuthorizedRelease)
	}
	receipt, err := readActivationReceipt(u.root, digest)
	if err != nil {
		return AdmittedRelease{}, fmt.Errorf("%w: activation receipt", ErrNoAuthorizedRelease)
	}
	ref, publicKey, err := u.signer.PublicKey(ctx)
	if err != nil || ref.SignerDomain != "guest-core" || ref.KeyID == "" || len(publicKey) != ed25519.PublicKeySize ||
		len(receipt.RequiredSigners) != 1 || receipt.RequiredSigners[0] != ref || receipt.Issuer != "choir-updater" ||
		receipt.Verify(activationKeyResolver{ref: ref, key: publicKey}) != nil {
		return AdmittedRelease{}, fmt.Errorf("%w: activation signature", ErrNoAuthorizedRelease)
	}
	if activationReceiptDigest(receipt) != digest || receiptString(receipt, "computer_id") != u.computerID ||
		receiptString(receipt, "realization_id") == "" || !computerevent.IsSHA256(receiptString(receipt, "request_commitment")) {
		return AdmittedRelease{}, fmt.Errorf("%w: activation identity", ErrNoAuthorizedRelease)
	}
	switch receipt.ReceiptKind {
	case ActivationIntentReceiptKind:
		if !computerevent.IsSHA256(receiptString(receipt, "accepted_or_rollback_event_head")) ||
			receiptString(receipt, "operation_id") == "" || !validOptionalSHA256(receiptString(receipt, "prior_release_digest")) {
			return AdmittedRelease{}, fmt.Errorf("%w: activation intent", ErrNoAuthorizedRelease)
		}
	case "MaterializationReceipt":
		if !computerevent.IsSHA256(receiptString(receipt, "accepted_or_rollback_event_head")) ||
			receiptString(receipt, "outcome") != "applied" || !computerevent.IsSHA256(receiptString(receipt, "health_receipt_digest")) ||
			!validOptionalSHA256(receiptString(receipt, "prior_release_digest")) {
			return AdmittedRelease{}, fmt.Errorf("%w: materialization receipt", ErrNoAuthorizedRelease)
		}
	case "UpdaterRecoveryReceipt":
		if !computerevent.IsSHA256(receiptString(receipt, "accepted_event_head")) ||
			!computerevent.IsSHA256(receiptString(receipt, "failed_release_digest")) || receiptString(receipt, "operation_id") == "" {
			return AdmittedRelease{}, fmt.Errorf("%w: recovery receipt", ErrNoAuthorizedRelease)
		}
	default:
		return AdmittedRelease{}, fmt.Errorf("%w: receipt kind", ErrNoAuthorizedRelease)
	}
	sandboxPath := filepath.Join(releaseRoot, "bin", "sandbox")
	if info, statErr := os.Stat(sandboxPath); statErr != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return AdmittedRelease{}, fmt.Errorf("%w: sandbox executable", ErrNoAuthorizedRelease)
	}
	skillsRoot := filepath.Join(releaseRoot, "share", "go-choir", "skills")
	if info, statErr := os.Stat(skillsRoot); statErr != nil || !info.IsDir() {
		return AdmittedRelease{}, fmt.Errorf("%w: runtime skills", ErrNoAuthorizedRelease)
	}
	return AdmittedRelease{
		ReleaseDigest: digest,
		ReleaseRoot:   releaseRoot,
		SandboxPath:   sandboxPath,
		SkillsRoot:    skillsRoot,
		ReceiptKind:   receipt.ReceiptKind,
		ReceiptID:     receipt.ReceiptID,
	}, nil
}

func (u *Updater) signActivationIntent(ctx context.Context, request ApplyRequest, priorDigest, releaseDigest string, issuedAt time.Time) (computerevent.Receipt, error) {
	return u.signer.SignReceipt(ctx, ActivationIntentReceiptKind, "choir-updater", map[string]any{
		"computer_id": request.ComputerID, "realization_id": request.RealizationID,
		"accepted_or_rollback_event_head": request.AcceptedEventHead,
		"operation_id":                    request.OperationID,
		"prior_release_digest":            priorDigest, "resulting_release_digest": releaseDigest,
		"request_commitment": request.RequestCommitment,
	}, issuedAt)
}

func activationReceiptDigest(receipt computerevent.Receipt) string {
	switch receipt.ReceiptKind {
	case ActivationIntentReceiptKind, "MaterializationReceipt":
		return receiptString(receipt, "resulting_release_digest")
	case "UpdaterRecoveryReceipt":
		return receiptString(receipt, "restored_release_digest")
	default:
		return ""
	}
}

func receiptString(receipt computerevent.Receipt, name string) string {
	value, _ := receipt.KindFields[name].(string)
	return value
}
func validOptionalSHA256(value string) bool {
	return value == "" || computerevent.IsSHA256(value)
}

func activationReceiptPath(root, digest string) string {
	return filepath.Join(filepath.Clean(root), "activations", digest+".json")
}

func readActivationReceipt(root, digest string) (computerevent.Receipt, error) {
	if !computerevent.IsSHA256(digest) {
		return computerevent.Receipt{}, ErrNoAuthorizedRelease
	}
	raw, err := os.ReadFile(activationReceiptPath(root, digest))
	if err != nil {
		return computerevent.Receipt{}, err
	}
	var receipt computerevent.Receipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return computerevent.Receipt{}, err
	}
	return receipt, nil
}

func writeActivationReceipt(root, digest string, receipt computerevent.Receipt) error {
	if activationReceiptDigest(receipt) != digest || !computerevent.IsSHA256(digest) {
		return fmt.Errorf("updater: activation receipt digest mismatch")
	}
	canonical, err := receipt.CanonicalBytes()
	if err != nil {
		return err
	}
	path := activationReceiptPath(root, digest)
	temporary, err := os.CreateTemp(filepath.Dir(path), ".activation-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(canonical); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}

type activationKeyResolver struct {
	ref computerevent.SignerRef
	key ed25519.PublicKey
}

func (r activationKeyResolver) ResolveReceiptKey(domain, _ string, keyID string, _ uint64, _ time.Time) (ed25519.PublicKey, error) {
	if domain != r.ref.SignerDomain || keyID != r.ref.KeyID {
		return nil, fmt.Errorf("updater: activation signer mismatch")
	}
	return r.key, nil
}
