package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

const ManifestVersion = 1

var ErrIdempotencyConflict = errors.New("updater idempotency conflict")

type ManifestFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Mode   uint32 `json:"mode"`
}

type ReleaseManifest struct {
	Version            int            `json:"version"`
	ComputerID         string         `json:"computer_id"`
	AcceptedEventHead  string         `json:"accepted_event_head"`
	CodeRef            string         `json:"code_ref"`
	ArtifactProgramRef string         `json:"artifact_program_ref"`
	EventSchemaVersion uint64         `json:"event_schema_version"`
	ReducerVersion     uint64         `json:"reducer_version"`
	Marker             string         `json:"marker"`
	Files              []ManifestFile `json:"files"`
	ContentDigest      string         `json:"content_digest"`
}

type ApplyRequest struct {
	ComputerID        string          `json:"computer_id"`
	RealizationID     string          `json:"realization_id"`
	OperationID       string          `json:"operation_id"`
	IdempotencyKey    string          `json:"idempotency_key"`
	RequestCommitment string          `json:"request_commitment"`
	AcceptedEventHead string          `json:"accepted_event_head"`
	SourceDir         string          `json:"source_dir"`
	Manifest          ReleaseManifest `json:"manifest"`
}

type BaselineImportRequest struct {
	ComputerID        string          `json:"computer_id"`
	RealizationID     string          `json:"realization_id"`
	IdempotencyKey    string          `json:"idempotency_key"`
	RequestCommitment string          `json:"request_commitment"`
	SourceDir         string          `json:"source_dir"`
	Manifest          ReleaseManifest `json:"manifest"`
}

type ApplyResult struct {
	ReleaseDigest          string                 `json:"release_digest"`
	PriorReleaseDigest     string                 `json:"prior_release_digest,omitempty"`
	MaterializationReceipt computerevent.Receipt  `json:"materialization_receipt"`
	HealthReceipt          computerevent.Receipt  `json:"health_receipt"`
	RecoveryReceipt        *computerevent.Receipt `json:"recovery_receipt,omitempty"`
	Outcome                string                 `json:"outcome"`
}

type ServiceManager interface {
	Restart(context.Context) error
	RecoveryRestart(context.Context) error
	CleanupRecoveryCredential(context.Context) error
}

type HealthProber interface {
	Probe(context.Context, string, ReleaseManifest) ([]string, error)
}

type Updater struct {
	mu            sync.Mutex
	root          string
	computerID    string
	realizationID string
	service       ServiceManager
	health        HealthProber
	signingKey    computerevent.SigningKey
	now           func() time.Time
}

func New(root, computerID, realizationID string, service ServiceManager, health HealthProber, signingKey computerevent.SigningKey) (*Updater, error) {
	root = filepath.Clean(root)
	if root == "." || !filepath.IsAbs(root) || strings.TrimSpace(computerID) == "" || strings.TrimSpace(realizationID) == "" || service == nil || health == nil || signingKey.SignerDomain != "guest-core" || signingKey.KeyID == "" || len(signingKey.PrivateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("updater: complete absolute root, identity, service, health probe, and guest-core signer are required")
	}
	for _, dir := range []string{filepath.Join(root, "releases"), filepath.Join(root, "operations"), filepath.Join(root, "incoming")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("updater: create state: %w", err)
		}
	}
	return &Updater{root: root, computerID: computerID, realizationID: realizationID, service: service, health: health, signingKey: signingKey, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (u *Updater) Apply(ctx context.Context, request ApplyRequest) (ApplyResult, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	commitment, err := validateApplyRequest(u.computerID, u.realizationID, request)
	if err != nil {
		return ApplyResult{}, err
	}
	if commitment != request.RequestCommitment {
		return ApplyResult{}, fmt.Errorf("updater: request commitment mismatch")
	}
	journalPath := filepath.Join(u.root, "operations", safeName(request.IdempotencyKey)+".json")
	journal, found, err := readJournal(journalPath)
	if err != nil {
		return ApplyResult{}, err
	}
	if found && journal.RequestCommitment != request.RequestCommitment {
		return ApplyResult{}, ErrIdempotencyConflict
	}
	if found && journal.Result.Outcome != "" {
		if journal.Result.Outcome == "failed" {
			return journal.Result, errors.New(journal.Failure)
		}
		return journal.Result, nil
	}
	releaseDigest := request.Manifest.ContentDigest
	sourceDir, err := u.trustedSourceDir(request.SourceDir, releaseDigest)
	if err != nil {
		return ApplyResult{}, err
	}
	request.SourceDir = sourceDir
	releaseDir := filepath.Join(u.root, "releases", releaseDigest)
	if err := u.stageRelease(request.SourceDir, releaseDir, request.Manifest); err != nil {
		return ApplyResult{}, err
	}
	if !found {
		priorDigest, priorTarget, priorErr := u.currentRelease()
		if priorErr != nil {
			return ApplyResult{}, priorErr
		}
		journal = operationJournal{
			RequestCommitment: request.RequestCommitment, Phase: "prepared",
			PriorReleaseDigest: priorDigest, PriorReleaseTarget: priorTarget,
			TargetReleaseDigest: releaseDigest, StartedAt: u.now().UTC().Truncate(time.Microsecond),
		}
		if err := writeJournal(journalPath, journal); err != nil {
			return ApplyResult{}, err
		}
	}
	if journal.TargetReleaseDigest != releaseDigest {
		return ApplyResult{}, ErrIdempotencyConflict
	}
	if journal.Phase == "prepared" {
		if err := u.swapCurrent(releaseDir); err != nil {
			return ApplyResult{}, err
		}
		journal.Phase = "pointer_swapped"
		if err := writeJournal(journalPath, journal); err != nil {
			return ApplyResult{}, err
		}
	}
	if journal.Phase == "pointer_swapped" {
		if err := u.service.Restart(ctx); err != nil {
			return ApplyResult{}, err
		}
		journal.Phase = "restart_requested"
		if err := writeJournal(journalPath, journal); err != nil {
			return ApplyResult{}, err
		}
	}
	completedAt := u.now().UTC().Truncate(time.Microsecond)
	if journal.Phase == "restart_requested" {
		observations, probeErr := u.health.Probe(ctx, releaseDigest, request.Manifest)
		if probeErr == nil {
			if cleanupErr := u.service.CleanupRecoveryCredential(ctx); cleanupErr != nil {
				return ApplyResult{}, fmt.Errorf("updater: cleanup recovery credential: %w", cleanupErr)
			}
			healthReceipt, receiptErr := u.signHealthReceipt(request, releaseDigest, journal.StartedAt, completedAt, observations, "healthy")
			if receiptErr != nil {
				return ApplyResult{}, receiptErr
			}
			healthBytes, receiptErr := healthReceipt.CanonicalBytes()
			if receiptErr != nil {
				return ApplyResult{}, receiptErr
			}
			materialization, receiptErr := computerevent.NewSignedReceipt("MaterializationReceipt", "choir-updater", map[string]any{
				"computer_id": request.ComputerID, "realization_id": request.RealizationID,
				"accepted_or_rollback_event_head": request.AcceptedEventHead,
				"prior_release_digest":            journal.PriorReleaseDigest, "resulting_release_digest": releaseDigest,
				"health_receipt_digest": computerevent.DigestBytes(healthBytes), "outcome": "applied",
				"request_commitment": request.RequestCommitment,
			}, []computerevent.SigningKey{u.signingKey}, completedAt)
			if receiptErr != nil {
				return ApplyResult{}, receiptErr
			}
			journal.Result = ApplyResult{ReleaseDigest: releaseDigest, PriorReleaseDigest: journal.PriorReleaseDigest, MaterializationReceipt: materialization, HealthReceipt: healthReceipt, Outcome: "applied"}
			journal.Phase = "completed"
			if err := writeJournal(journalPath, journal); err != nil {
				return ApplyResult{}, err
			}
			return journal.Result, nil
		}
		journal.Phase = "recovering"
		journal.Failure = probeErr.Error()
		if err := writeJournal(journalPath, journal); err != nil {
			return ApplyResult{}, err
		}
	}
	if journal.Phase != "recovering" {
		return ApplyResult{}, fmt.Errorf("updater: invalid operation journal phase %q", journal.Phase)
	}
	failure := errors.New(journal.Failure)
	recoveryReceipt, recoveryErr := u.restorePrior(ctx, request, journal.PriorReleaseTarget, journal.PriorReleaseDigest, releaseDigest, failure, completedAt)
	result := ApplyResult{ReleaseDigest: releaseDigest, PriorReleaseDigest: journal.PriorReleaseDigest, Outcome: "failed", RecoveryReceipt: recoveryReceipt}
	if recoveryErr != nil {
		return result, errors.Join(failure, recoveryErr)
	}
	journal.Result = result
	journal.Phase = "completed"
	if err := writeJournal(journalPath, journal); err != nil {
		return result, err
	}
	return result, failure
}

func (u *Updater) trustedSourceDir(source, releaseDigest string) (string, error) {
	resolved, err := filepath.EvalSymlinks(filepath.Clean(source))
	if err != nil {
		return "", fmt.Errorf("updater: resolve source directory: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("updater: source directory is unavailable")
	}
	incoming, err := filepath.EvalSymlinks(filepath.Join(u.root, "incoming"))
	if err != nil {
		return "", fmt.Errorf("updater: resolve incoming root: %w", err)
	}
	if relative, relErr := filepath.Rel(incoming, resolved); relErr == nil && relative != "." &&
		!strings.HasPrefix(relative, ".."+string(os.PathSeparator)) && !filepath.IsAbs(relative) {
		if info.Mode().Perm()&0o077 != 0 {
			return "", fmt.Errorf("updater: incoming source directory must be private")
		}
		return resolved, nil
	}
	releases, err := filepath.EvalSymlinks(filepath.Join(u.root, "releases"))
	if err != nil {
		return "", fmt.Errorf("updater: resolve releases root: %w", err)
	}
	relative, relErr := filepath.Rel(releases, resolved)
	if relErr != nil || !computerevent.IsSHA256(relative) || strings.Contains(relative, string(os.PathSeparator)) ||
		!computerevent.IsSHA256(releaseDigest) || info.Mode().Perm()&0o222 != 0 {
		return "", fmt.Errorf("updater: source directory is outside root-owned incoming or pinned release stores")
	}
	return resolved, nil
}

func (u *Updater) stageRelease(sourceDir, releaseDir string, manifest ReleaseManifest) error {
	if err := verifyManifest(sourceDir, manifest); err != nil {
		return err
	}
	if _, err := os.Lstat(releaseDir); err == nil {
		return verifyManifest(releaseDir, manifest)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	temporary, err := os.MkdirTemp(filepath.Join(u.root, "releases"), ".stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	for _, file := range manifest.Files {
		source := filepath.Join(sourceDir, filepath.FromSlash(file.Path))
		target := filepath.Join(temporary, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if err := copyRegularFile(source, target, fs.FileMode(file.Mode)&0o555); err != nil {
			return err
		}
	}
	manifestBytes, err := computerevent.CanonicalJSON(manifest)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(temporary, "release-manifest.json"), manifestBytes, 0o444); err != nil {
		return err
	}
	if err := chmodTreeReadOnly(temporary); err != nil {
		return err
	}
	if err := os.Rename(temporary, releaseDir); err != nil {
		if _, statErr := os.Stat(releaseDir); statErr == nil {
			return verifyManifest(releaseDir, manifest)
		}
		return err
	}
	return syncDir(filepath.Dir(releaseDir))
}

func verifyManifest(root string, manifest ReleaseManifest) error {
	if manifest.Version != ManifestVersion || !computerevent.IsSHA256(manifest.AcceptedEventHead) || !computerevent.IsSHA256(manifest.ContentDigest) || manifest.ComputerID == "" || manifest.CodeRef == "" || manifest.ArtifactProgramRef == "" || manifest.EventSchemaVersion == 0 || manifest.ReducerVersion == 0 || manifest.Marker == "" || len(manifest.Files) == 0 {
		return fmt.Errorf("updater: invalid release manifest")
	}
	files := append([]ManifestFile(nil), manifest.Files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	if !sameManifestOrder(files, manifest.Files) {
		return fmt.Errorf("updater: manifest files are not deterministically ordered")
	}
	for _, file := range files {
		if !safeRelativePath(file.Path) || !computerevent.IsSHA256(file.SHA256) || file.Mode&0o7000 != 0 {
			return fmt.Errorf("updater: unsafe manifest file %q", file.Path)
		}
		path := filepath.Join(root, filepath.FromSlash(file.Path))
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("updater: manifest file %q is unavailable or not regular", file.Path)
		}
		digest, err := fileSHA256(path)
		if err != nil || digest != file.SHA256 {
			return fmt.Errorf("updater: manifest digest mismatch for %q", file.Path)
		}
	}
	unsigned := manifest
	unsigned.ContentDigest = ""
	canonical, err := computerevent.CanonicalJSON(unsigned)
	if err != nil || computerevent.DigestBytes(canonical) != manifest.ContentDigest {
		return fmt.Errorf("updater: manifest content digest mismatch")
	}
	return nil
}

func readReleaseManifest(releaseDir string) (ReleaseManifest, error) {
	raw, err := os.ReadFile(filepath.Join(releaseDir, "release-manifest.json"))
	if err != nil {
		return ReleaseManifest{}, err
	}
	var manifest ReleaseManifest
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return ReleaseManifest{}, err
	}
	if err := verifyManifest(releaseDir, manifest); err != nil {
		return ReleaseManifest{}, err
	}
	return manifest, nil
}

func BuildBaselineManifest(sourceDir, computerID, codeRef, artifactProgramRef string) (ReleaseManifest, error) {
	sourceDir = filepath.Clean(sourceDir)
	var files []ManifestFile
	err := filepath.WalkDir(sourceDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == sourceDir || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("updater: baseline contains non-regular file %q", path)
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		relative, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		files = append(files, ManifestFile{Path: filepath.ToSlash(relative), SHA256: hex.EncodeToString(hash.Sum(nil)), Mode: uint32(info.Mode().Perm())})
		return nil
	})
	if err != nil {
		return ReleaseManifest{}, fmt.Errorf("updater: baseline source is unavailable: %w", err)
	}
	if len(files) == 0 {
		return ReleaseManifest{}, fmt.Errorf("updater: baseline source is empty")
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return FinalizeManifest(ReleaseManifest{
		Version: ManifestVersion, ComputerID: strings.TrimSpace(computerID), AcceptedEventHead: computerevent.ZeroHead,
		CodeRef: strings.TrimSpace(codeRef), ArtifactProgramRef: strings.TrimSpace(artifactProgramRef),
		EventSchemaVersion: computerevent.SchemaVersionV1, ReducerVersion: computerevent.ReducerVersionV1,
		Marker: "genesis-baseline", Files: files,
	})
}
func (u *Updater) ImportBaseline(request BaselineImportRequest) (ReleaseManifest, error) {
	if u == nil {
		return ReleaseManifest{}, fmt.Errorf("updater: invalid baseline import")
	}
	sourceDir := filepath.Clean(request.SourceDir)
	trustedBaseline := strings.HasPrefix(sourceDir, "/nix/store/") || strings.HasPrefix(sourceDir, filepath.Join(u.root, "incoming")+string(os.PathSeparator))
	if request.ComputerID != u.computerID || request.RealizationID != u.realizationID ||
		strings.TrimSpace(request.IdempotencyKey) == "" || !trustedBaseline {
		return ReleaseManifest{}, fmt.Errorf("updater: invalid baseline import")
	}
	commitment, err := baselineImportCommitment(request)
	if err != nil || commitment != request.RequestCommitment {
		return ReleaseManifest{}, fmt.Errorf("updater: baseline import commitment mismatch")
	}
	if current, err := ReadCurrentManifest(u.root); err == nil {
		if current.ContentDigest == request.Manifest.ContentDigest {
			return current, nil
		}
		return ReleaseManifest{}, ErrIdempotencyConflict
	} else if !errors.Is(err, os.ErrNotExist) {
		return ReleaseManifest{}, err
	}
	releaseDir := filepath.Join(u.root, "releases", request.Manifest.ContentDigest)
	if err := u.stageRelease(filepath.Clean(request.SourceDir), releaseDir, request.Manifest); err != nil {
		return ReleaseManifest{}, err
	}
	if err := u.swapCurrent(releaseDir); err != nil {
		return ReleaseManifest{}, err
	}
	return request.Manifest, nil
}

func ComputeBaselineImportCommitment(request BaselineImportRequest) (string, error) {
	return baselineImportCommitment(request)
}

func baselineImportCommitment(request BaselineImportRequest) (string, error) {
	request.RequestCommitment = ""
	canonical, err := computerevent.CanonicalJSON(request)
	if err != nil {
		return "", err
	}
	return computerevent.DigestBytes(canonical), nil
}

func ReadPinnedManifest(root, releaseDigest string) (ReleaseManifest, string, error) {
	if !computerevent.IsSHA256(releaseDigest) {
		return ReleaseManifest{}, "", fmt.Errorf("updater: invalid pinned release")
	}
	releaseDir := filepath.Join(filepath.Clean(root), "releases", releaseDigest)
	manifest, err := readReleaseManifest(releaseDir)
	if err != nil || manifest.ContentDigest != releaseDigest {
		return ReleaseManifest{}, "", fmt.Errorf("updater: pinned release unavailable")
	}
	return manifest, releaseDir, nil
}

func ReadCurrentManifest(root string) (ReleaseManifest, error) {
	root = filepath.Clean(root)
	target, err := os.Readlink(filepath.Join(root, "current"))
	if err != nil {
		return ReleaseManifest{}, err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	return readReleaseManifest(target)
}

func FinalizeManifest(manifest ReleaseManifest) (ReleaseManifest, error) {
	manifest.ContentDigest = ""
	canonical, err := computerevent.CanonicalJSON(manifest)
	if err != nil {
		return ReleaseManifest{}, err
	}
	manifest.ContentDigest = computerevent.DigestBytes(canonical)
	return manifest, nil
}

func ComputeApplyRequestCommitment(request ApplyRequest) (string, error) {
	request.RequestCommitment = ""
	canonical, err := computerevent.CanonicalJSON(request)
	if err != nil {
		return "", err
	}
	return computerevent.DigestBytes(canonical), nil
}

func validateApplyRequest(computerID, realizationID string, request ApplyRequest) (string, error) {
	if request.ComputerID != computerID || request.RealizationID != realizationID || request.OperationID == "" || request.IdempotencyKey == "" || !computerevent.IsSHA256(request.AcceptedEventHead) || request.Manifest.ComputerID != computerID || request.Manifest.AcceptedEventHead != request.AcceptedEventHead || !filepath.IsAbs(request.SourceDir) {
		return "", fmt.Errorf("updater: incomplete or mismatched apply request")
	}
	commitmentInput := request
	commitmentInput.RequestCommitment = ""
	canonical, err := computerevent.CanonicalJSON(commitmentInput)
	if err != nil {
		return "", err
	}
	return computerevent.DigestBytes(canonical), nil
}

func (u *Updater) currentRelease() (string, string, error) {
	path := filepath.Join(u.root, "current")
	target, err := os.Readlink(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}
	absolute := target
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(u.root, target)
	}
	relative, err := filepath.Rel(filepath.Join(u.root, "releases"), absolute)
	if err != nil || relative == "." || strings.Contains(relative, string(os.PathSeparator)) || !computerevent.IsSHA256(relative) {
		return "", "", fmt.Errorf("updater: current release pointer escapes release store")
	}
	return relative, absolute, nil
}

func (u *Updater) swapCurrent(releaseDir string) error {
	temporaryBytes := make([]byte, 8)
	if _, err := rand.Read(temporaryBytes); err != nil {
		return err
	}
	temporary := filepath.Join(u.root, ".current-"+hex.EncodeToString(temporaryBytes))
	if err := os.Symlink(releaseDir, temporary); err != nil {
		return err
	}
	defer os.Remove(temporary)
	if err := os.Rename(temporary, filepath.Join(u.root, "current")); err != nil {
		return err
	}
	return syncDir(u.root)
}

func (u *Updater) restorePrior(ctx context.Context, request ApplyRequest, priorTarget, priorDigest, failedDigest string, cause error, completedAt time.Time) (*computerevent.Receipt, error) {
	if priorTarget == "" {
		return nil, fmt.Errorf("updater: initial release failed and no prior release exists: %w", cause)
	}
	if err := u.swapCurrent(priorTarget); err != nil {
		return nil, fmt.Errorf("updater: restore prior pointer: %w", err)
	}
	if err := u.service.RecoveryRestart(ctx); err != nil {
		return nil, fmt.Errorf("updater: restart restored release: %w", err)
	}
	priorManifest, err := readReleaseManifest(priorTarget)
	if err != nil {
		return nil, fmt.Errorf("updater: read restored release manifest: %w", err)
	}
	observations, err := u.health.Probe(ctx, priorDigest, priorManifest)
	if err != nil {
		return nil, fmt.Errorf("updater: restored release unhealthy: %w", err)
	}
	if err := u.service.CleanupRecoveryCredential(ctx); err != nil {
		return nil, fmt.Errorf("updater: cleanup recovery credential after restore: %w", err)
	}
	receipt, err := computerevent.NewSignedReceipt("UpdaterRecoveryReceipt", "choir-updater", map[string]any{
		"computer_id": request.ComputerID, "realization_id": request.RealizationID,
		"operation_id": request.OperationID, "failed_release_digest": failedDigest,
		"restored_release_digest": priorDigest, "accepted_event_head": request.AcceptedEventHead,
		"request_commitment": request.RequestCommitment, "failure": cause.Error(),
		"observation_artifact_digests": observations,
	}, []computerevent.SigningKey{u.signingKey}, completedAt)
	if err != nil {
		return nil, err
	}
	return &receipt, nil
}

func (u *Updater) signHealthReceipt(request ApplyRequest, releaseDigest string, startedAt, completedAt time.Time, observations []string, outcome string) (computerevent.Receipt, error) {
	probeContract, err := computerevent.CanonicalJSON(struct {
		EventSchemaVersion uint64 `json:"event_schema_version"`
		ReducerVersion     uint64 `json:"reducer_version"`
		Marker             string `json:"marker"`
	}{request.Manifest.EventSchemaVersion, request.Manifest.ReducerVersion, request.Manifest.Marker})
	if err != nil {
		return computerevent.Receipt{}, err
	}
	return computerevent.NewSignedReceipt("HealthReceipt", "choir-updater", map[string]any{
		"computer_id": request.ComputerID, "realization_id": request.RealizationID,
		"release_digest": releaseDigest, "probe_contract_digest": computerevent.DigestBytes(probeContract),
		"started_at": startedAt.Format(time.RFC3339Nano), "completed_at": completedAt.Format(time.RFC3339Nano),
		"outcome": outcome, "observation_artifact_digests": observations,
	}, []computerevent.SigningKey{u.signingKey}, completedAt)
}

type operationJournal struct {
	RequestCommitment   string      `json:"request_commitment"`
	Phase               string      `json:"phase"`
	PriorReleaseDigest  string      `json:"prior_release_digest,omitempty"`
	PriorReleaseTarget  string      `json:"prior_release_target,omitempty"`
	TargetReleaseDigest string      `json:"target_release_digest"`
	StartedAt           time.Time   `json:"started_at"`
	Failure             string      `json:"failure,omitempty"`
	Result              ApplyResult `json:"result"`
}

func readJournal(path string) (operationJournal, bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return operationJournal{}, false, nil
	}
	if err != nil {
		return operationJournal{}, false, err
	}
	var journal operationJournal
	if err := json.Unmarshal(raw, &journal); err != nil {
		return operationJournal{}, true, err
	}
	return journal, true, nil
}

func writeJournal(path string, journal operationJournal) error {
	canonical, err := computerevent.CanonicalJSON(journal)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".journal-")
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

func copyRegularFile(source, target string, mode fs.FileMode) error {
	info, err := os.Lstat(source)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("updater: source file is not regular: %s", source)
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		output.Close()
		return err
	}
	if err := output.Sync(); err != nil {
		output.Close()
		return err
	}
	return output.Close()
}

func chmodTreeReadOnly(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("updater: symlink in staged release")
		}
		if entry.IsDir() {
			return os.Chmod(path, 0o555)
		}
		return os.Chmod(path, 0o444|entry.Type().Perm()&0o111)
	})
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func safeRelativePath(path string) bool {
	if path == "" || filepath.IsAbs(path) || filepath.Clean(path) != filepath.FromSlash(path) {
		return false
	}
	return path != "." && path != ".." && !strings.HasPrefix(path, "../")
}

func sameManifestOrder(sorted, original []ManifestFile) bool {
	if len(sorted) != len(original) {
		return false
	}
	for index := range sorted {
		if sorted[index] != original[index] || index > 0 && sorted[index-1].Path == sorted[index].Path {
			return false
		}
	}
	return true
}

func safeName(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func syncDir(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
