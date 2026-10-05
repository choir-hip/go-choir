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
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
)

const ManifestVersion = 1

var ErrIdempotencyConflict = errors.New("updater idempotency conflict")

// ErrApplyRefused marks a validation-level refusal (4xx) the updater daemon
// returned for an apply request: malformed or mismatched request, commitment
// mismatch, idempotency conflict on an in-flight operation. It is distinct
// from transport failures (daemon unreachable, socket errors): the caller
// should degrade an operation on refusal but retry on transport failure,
// because a transport failure may hide a journal write that already
// completed the apply.
var ErrApplyRefused = errors.New("updater refused apply")

type ManifestFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Mode   uint32 `json:"mode"`
}

type ReleaseManifest struct {
	Version            int    `json:"version"`
	ComputerID         string `json:"computer_id"`
	AcceptedEventHead  string `json:"accepted_event_head"`
	CodeRef            string `json:"code_ref"`
	ArtifactProgramRef string `json:"artifact_program_ref"`
	EventSchemaVersion uint64 `json:"event_schema_version"`
	ReducerVersion     uint64 `json:"reducer_version"`
	Marker             string `json:"marker"`
	// BaseImageManifestDigest is the sha256 of the booted guest base image's
	// choir-guest-image-v1 manifest the app-layer closure resolves against.
	// When present, Apply refuses before mutation if it does not equal the
	// actually booted base's digest (base-resolution fail-closed). Empty
	// keeps the file-release baseline path (no base join yet).
	BaseImageManifestDigest string `json:"base_image_manifest_digest,omitempty"`
	// ClosureDigest is the sha256 of the app-layer narchive (nix-store
	// --export) carrying the base-absent store paths. Set when the release
	// is a layered app-layer release rather than a plain file release.
	ClosureDigest string `json:"closure_digest,omitempty"`
	// LayeringEntrypoint is the private-store-relative path to the release's
	// exec binary (e.g. "<hash>-autoputer/bin/autoputer"). The guest runtime
	// execs $UPDATER_ROOT/store/<layering_entrypoint> inside the mount-ns
	// overlay instead of the base binary.
	LayeringEntrypoint string `json:"layering_entrypoint,omitempty"`
	// StoreSchemaVersion is the persistent-store schema epoch the release was
	// built against (store.StoreSchemaVersion). The updater refuses before
	// mutation when the guest's persisted epoch is newer — a binary older
	// than its store is the vm-3dc68688 crash-loop failure class.
	StoreSchemaVersion uint64 `json:"store_schema_version,omitempty"`
	// MinStoreSchemaVersion is the oldest persisted store epoch the release
	// can migrate forward. Refused before mutation when the persisted epoch
	// is older.
	MinStoreSchemaVersion uint64 `json:"min_store_schema_version,omitempty"`
	// BaseCommit is the repo commit of the base image the release was built
	// against (guest-image-manifest build_commit). The updater refuses a
	// declared BaseCommit that differs from the booted base's — the
	// commit-level provenance half of the base join (S2-d); the digest join
	// stays the identity half.
	BaseCommit string `json:"base_commit,omitempty"`
	// CodeCommit is the repo commit the release binary was built from (S2-c
	// provenance). When set on a layered release, the updater verifies the
	// materialized entrypoint's share/go-choir/build.json commit before the
	// pointer swap — a manifest cannot claim a commit the binary lacks.
	CodeCommit string `json:"code_commit,omitempty"`
	// BuilderReceiptDigest binds the release to a specific host-builder
	// evidence receipt (sha256 of builder-receipt.json). Carried through the
	// mint so the release's provenance chain is auditable end to end.
	BuilderReceiptDigest string         `json:"builder_receipt_digest,omitempty"`
	Files                []ManifestFile `json:"files"`
	ContentDigest        string         `json:"content_digest"`
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

type ReceiptSigner interface {
	PublicKey(context.Context) (computerevent.SignerRef, ed25519.PublicKey, error)
	SignReceipt(context.Context, string, string, map[string]any, time.Time) (computerevent.Receipt, error)
}

type Updater struct {
	mu            sync.Mutex
	root          string
	computerID    string
	realizationID string
	service       ServiceManager
	health        HealthProber
	signer        ReceiptSigner
	// guestImageManifestPath is the booted base image's choir-guest-image-v1
	// manifest. Set when the updater must enforce the app-layer base join.
	guestImageManifestPath string
	now                    func() time.Time
	// storeSchemaPath is the guest persistent store's schema receipt
	// (store.StoreSchemaFile inside the Dolt workspace). When wired, a
	// release declaring a store schema window is refused before mutation if
	// the persisted epoch falls outside it.
	storeSchemaPath string
}

func New(root, computerID, realizationID string, service ServiceManager, health HealthProber, signer ReceiptSigner) (*Updater, error) {
	return newUpdater(root, computerID, realizationID, service, health, signer, "")
}

// NewWithBase wires the booted base image manifest path so Apply can enforce
// the app-layer base join (fail-closed when a release's declared base digest
// differs from the booted base's).
func NewWithBase(root, computerID, realizationID string, service ServiceManager, health HealthProber, signer ReceiptSigner, guestImageManifestPath string) (*Updater, error) {
	return newUpdater(root, computerID, realizationID, service, health, signer, guestImageManifestPath)
}

// WithStoreSchemaPath wires the guest persistent store's schema receipt so
// Apply can enforce a release's declared store schema window before any
// mutation (S2-d). Returns the updater for chaining after construction.
func (u *Updater) WithStoreSchemaPath(path string) *Updater {
	u.storeSchemaPath = filepath.Clean(strings.TrimSpace(path))
	return u
}

func newUpdater(root, computerID, realizationID string, service ServiceManager, health HealthProber, signer ReceiptSigner, guestImageManifestPath string) (*Updater, error) {
	root = filepath.Clean(root)
	if root == "." || !filepath.IsAbs(root) || strings.TrimSpace(computerID) == "" || strings.TrimSpace(realizationID) == "" || service == nil || health == nil || signer == nil {
		return nil, fmt.Errorf("updater: complete absolute root, identity, service, health probe, and isolated guest-core signer are required")
	}
	for _, dir := range []string{filepath.Join(root, "releases"), filepath.Join(root, "operations"), filepath.Join(root, "incoming")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("updater: create state: %w", err)
		}
	}
	return &Updater{root: root, computerID: computerID, realizationID: realizationID, service: service, health: health, signer: signer, guestImageManifestPath: guestImageManifestPath, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (u *Updater) Apply(ctx context.Context, request ApplyRequest) (ApplyResult, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	journalPath := filepath.Join(u.root, "operations", safeName(request.IdempotencyKey)+".json")
	journal, found, err := readJournal(journalPath)
	if err != nil {
		return ApplyResult{}, err
	}
	if found && journal.Result.Outcome != "" {
		if journal.Result.Outcome == "failed" {
			return journal.Result, errors.New(journal.Failure)
		}
		if journal.Result.Outcome == "refused" {
			return journal.Result, errors.New(journal.Failure)
		}
		return journal.Result, nil
	}
	// Pre-mutation refusals (shape, commitment, idempotency fence;
	// base/state-compat/source-trust/stage/materialize gates) journal a
	// terminal "refused" outcome BEFORE any mutation, so the caller can
	// tell them apart from mutated-then-unrestored failures and the
	// agent-side has a stable outcome to clear the pending transition.
	// Resumed replays of a refused operation return this record.
	refuse := func(reason string) (ApplyResult, error) {
		result := ApplyResult{ReleaseDigest: request.Manifest.ContentDigest, Outcome: "refused"}
		if jerr := writeJournal(journalPath, operationJournal{
			RequestCommitment:   request.RequestCommitment,
			Phase:               "refused",
			TargetReleaseDigest: request.Manifest.ContentDigest,
			StartedAt:           u.now().UTC().Truncate(time.Microsecond),
			Result:              result,
			Failure:             reason,
		}); jerr != nil {
			return ApplyResult{}, jerr
		}
		return result, errors.New(reason)
	}
	// The request commitment deliberately excludes RealizationID: a deploy
	// refresh rotates the realization epoch, and a replayed apply for an
	// already-journaled operation must still return the recorded outcome
	// instead of wedge-refusing (journal replay is the resume path; the
	// guest reboots between apply and checkpoint in that window). Fresh
	// applies keep the strict realization fence below.
	if _, verr := validateApplyRequest(request); verr != nil {
		return refuse(verr.Error())
	}
	commitment, cerr := computeApplyRequestCommitment(request)
	if cerr != nil {
		return refuse(cerr.Error())
	}
	if commitment != request.RequestCommitment {
		return refuse("updater: request commitment mismatch")
	}
	if found {
		if journal.RequestCommitment != request.RequestCommitment {
			// Journals written before the epoch-independent commitment bound
			// RealizationID inside the hash; after a VM refresh the replayed
			// request can never reproduce it. The journal is self-describing:
			// a completed/failed operation returns its recorded outcome rather
			// than wedge — the idempotency key already names the operation.
			if journal.Result.Outcome != "" {
				log.Printf("updater: journal %s carries pre-epoch commitment; returning recorded outcome", request.IdempotencyKey)
			} else {
				return refuse(ErrIdempotencyConflict.Error())
			}
		}
	} else if request.ComputerID != u.computerID || request.RealizationID != u.realizationID {
		return refuse("updater: incomplete or mismatched apply request")
	}
	if found && journal.Result.Outcome != "" {
		if journal.Result.Outcome == "failed" {
			return journal.Result, errors.New(journal.Failure)
		}
		if journal.Result.Outcome == "refused" {
			return journal.Result, errors.New(journal.Failure)
		}
		return journal.Result, nil
	}

	// App-layer base join: a layered release declares the base image manifest
	// digest its closure resolves against. Refuse before mutation when the
	// booted base differs (base-resolution fail-closed). Only enforced when the
	// updater was constructed with a base manifest path; a guest without the
	// wired base cannot prove a join and keeps the plain file-release path.
	if request.Manifest.BaseImageManifestDigest != "" && u.guestImageManifestPath != "" {
		booted, digestErr := DigestFile(u.guestImageManifestPath)
		if digestErr != nil {
			return refuse(fmt.Errorf("updater: resolve booted base image digest: %w", digestErr).Error())
		}
		if booted != request.Manifest.BaseImageManifestDigest {
			return refuse(fmt.Sprintf("updater: release base %s does not match booted base %s", request.Manifest.BaseImageManifestDigest, booted))
		}
	}
	// S2-d state-compat gate: a layered release declares the store schema
	// window and base commit it was built against. Refuse before any
	// mutation (staging, store replay, pointer swap) when the guest's
	// persisted epoch or booted base commit falls outside the declaration —
	// the vm-3dc68688 stale-binary failure class, caught pre-restart instead
	// of by a post-mortem health probe.
	if cerr := u.checkStateCompatibility(request.Manifest); cerr != nil {
		return refuse(cerr.Error())
	}

	releaseDigest := request.Manifest.ContentDigest
	sourceDir, serr := u.trustedSourceDir(request.SourceDir, releaseDigest)
	if serr != nil {
		return refuse(serr.Error())
	}
	request.SourceDir = sourceDir
	releaseDir := filepath.Join(u.root, "releases", releaseDigest)
	if serr := u.stageRelease(request.SourceDir, releaseDir, request.Manifest); serr != nil {
		return refuse(serr.Error())
	}
	// Layered release: replay the app-layer narchive into the private store
	// and GC-root it before the pointer swap. A failure here leaves `current`
	// untouched and never publishes a restart — the running release keeps
	// serving (fail-closed on the data path).
	if serr := u.materializeReleaseClosure(releaseDir, request.Manifest); serr != nil {
		return refuse(serr.Error())
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
	if journal.Phase == "pointer_swapped" && !journal.RestartPublished {
		// Publish the restart first, then mark it durably. A crash between
		// the two leaves RestartPublished=false; the next apply re-publishes
		// (restart is idempotent). The inverse order would lose the restart
		// silently on a crash in the same window.
		if restartErr := u.service.Restart(ctx); restartErr != nil {
			journal.Phase = "recovering"
			journal.Failure = fmt.Sprintf("updater: restart target release: %v", restartErr)
		} else {
			journal.RestartPublished = true
			if err := writeJournal(journalPath, journal); err != nil {
				return ApplyResult{}, err
			}
			journal.Phase = "restart_requested"
		}
		if err := writeJournal(journalPath, journal); err != nil {
			return ApplyResult{}, err
		}
	} else if journal.Phase == "pointer_swapped" {
		// Resumed: the restart already published before the prior caller died.
		// Advance past it so the probe runs; never restart the guest twice.
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
			healthReceipt, receiptErr := u.signHealthReceipt(ctx, request, releaseDigest, journal.StartedAt, completedAt, observations, "healthy")
			if receiptErr != nil {
				return ApplyResult{}, receiptErr
			}
			healthBytes, receiptErr := healthReceipt.CanonicalBytes()
			if receiptErr != nil {
				return ApplyResult{}, receiptErr
			}
			materialization, receiptErr := u.signer.SignReceipt(ctx, "MaterializationReceipt", "choir-updater", map[string]any{
				"computer_id": request.ComputerID, "realization_id": request.RealizationID,
				"accepted_or_rollback_event_head": request.AcceptedEventHead,
				"prior_release_digest":            journal.PriorReleaseDigest, "resulting_release_digest": releaseDigest,
				"health_receipt_digest": computerevent.DigestBytes(healthBytes), "outcome": "applied",
				"request_commitment": request.RequestCommitment,
			}, completedAt)
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
	recoveryReceipt, recoveryErr := u.restorePrior(ctx, request, journal.PriorReleaseTarget, journal.PriorReleaseDigest, releaseDigest, failure, completedAt, &journal, journalPath)
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

// checkStateCompatibility enforces a release's declared state-compat window
// before Apply mutates anything: the guest persistent store's schema epoch
// must satisfy [min_store_schema_version, store_schema_version], and a
// declared base_commit must equal the booted base image's build_commit.
// Plain file releases declare nothing and pass; a layered release that
// declares a window on a guest with an unwired receipt fails closed —
// compat cannot be proven.
func (u *Updater) checkStateCompatibility(manifest ReleaseManifest) error {
	if manifest.StoreSchemaVersion == 0 && manifest.MinStoreSchemaVersion == 0 {
		// No schema window declared; the base-commit join may still apply.
	} else {
		if strings.TrimSpace(u.storeSchemaPath) == "" {
			return fmt.Errorf("updater: release declares store schema window but no store schema receipt is wired")
		}
		receipt, found, readErr := u.readStoreSchema()
		if readErr != nil {
			return readErr
		}
		if !found {
			return fmt.Errorf("updater: release declares store schema window but guest store has no schema receipt")
		}
		persisted := receipt.Version
		if manifest.StoreSchemaVersion != 0 && persisted > manifest.StoreSchemaVersion {
			return fmt.Errorf("updater: guest store schema %d is newer than release's built schema %d; refusing stale binary", persisted, manifest.StoreSchemaVersion)
		}
		if manifest.MinStoreSchemaVersion != 0 && persisted < manifest.MinStoreSchemaVersion {
			return fmt.Errorf("updater: guest store schema %d is older than release's minimum migratable schema %d; refusing unmigratable store", persisted, manifest.MinStoreSchemaVersion)
		}
	}
	if manifest.BaseCommit != "" && u.guestImageManifestPath != "" {
		booted, commitErr := u.bootedBaseCommit()
		if commitErr != nil {
			return fmt.Errorf("updater: resolve booted base commit: %w", commitErr)
		}
		if booted == "" {
			return fmt.Errorf("updater: release declares base_commit %s but the booted base carries no build_commit", manifest.BaseCommit)
		}
		if booted != manifest.BaseCommit {
			return fmt.Errorf("updater: release base commit %s does not match booted base commit %s", manifest.BaseCommit, booted)
		}
	}
	return nil
}

// readStoreSchema loads the guest store's persisted schema epoch.
// found=false means no receipt — the store predates the contract or the
// workspace is fresh.
func (u *Updater) readStoreSchema() (storeschema.Receipt, bool, error) {
	receipt, found, err := storeschema.Read(u.storeSchemaPath)
	if err != nil {
		return receipt, found, fmt.Errorf("updater: %w", err)
	}
	return receipt, found, nil
}

// bootedBaseCommit parses build_commit out of the booted choir-guest-image-v1
// manifest (key=value lines). Absent key returns empty without error so a
// guest whose base predates the field degrades observably rather than
// breaking unrelated applies.
func (u *Updater) bootedBaseCommit() (string, error) {
	raw, err := os.ReadFile(u.guestImageManifestPath)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if key, value, ok := strings.Cut(strings.TrimSpace(line), "="); ok && key == "build_commit" {
			return strings.TrimSpace(value), nil
		}
	}
	return "", nil
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
	// Layered releases carry their exec entrypoint inside the release dir so
	// the current/ pointer swap moves served frontend and exec together —
	// one state authority (S2-e). restorePrior then reverts exec with the
	// same swap; a release that predates the in-dir entrypoint is backfilled
	// by ensureReleaseEntrypoint on the restore path.
	entrypoint, err := resolveLayeringEntrypoint(filepath.Join(u.root, "store"), manifest)
	if err != nil {
		return err
	}
	if entrypoint != "" {
		if err := os.WriteFile(filepath.Join(temporary, "layering-entrypoint"), []byte(entrypoint+"\n"), 0o444); err != nil {
			return fmt.Errorf("updater: stage layering entrypoint: %w", err)
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
		info, err := os.Stat(path)
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
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
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

// RestagePinnedRelease swaps `current` onto an already-pinned release without
// Apply, restorePrior, or a service restart. Tape restore uses this to restage
// SPA bytes after the Dolt witness matches.
func RestagePinnedRelease(root, releaseDigest string) error {
	root = filepath.Clean(root)
	if root == "." || !filepath.IsAbs(root) {
		return fmt.Errorf("updater: restage requires an absolute updater root")
	}
	manifest, releaseDir, err := ReadPinnedManifest(root, releaseDigest)
	if err != nil {
		return err
	}
	ensureReleaseEntrypoint(root, releaseDir, manifest)
	return swapCurrentPointer(root, releaseDir)
}

// ensureReleaseEntrypoint backfills releaseDir/layering-entrypoint for a
// layered release staged before the entrypoint moved inside the release dir
// (pre-S2-e). A release whose declared entrypoint cannot resolve against the
// private store degrades to base exec — the boot guard and the health
// identity surface that honestly — so backfill failure logs and returns
// rather than blocking the pointer swap.
func ensureReleaseEntrypoint(root, releaseDir string, manifest ReleaseManifest) {
	if manifest.LayeringEntrypoint == "" {
		return
	}
	entryPath := filepath.Join(releaseDir, "layering-entrypoint")
	if _, err := os.Lstat(entryPath); err == nil {
		return
	}
	entry, err := resolveLayeringEntrypoint(filepath.Join(root, "store"), manifest)
	if err != nil {
		log.Printf("updater: layering entrypoint %q unresolvable; base exec remains: %v", manifest.LayeringEntrypoint, err)
		return
	}
	if _, err := os.Stat(entry); err != nil {
		log.Printf("updater: layering entrypoint %s not materialized; base exec remains: %v", entry, err)
		return
	}
	// Staged releases are read-only (0555 dir). Relax the directory just long
	// enough to record the pointer, then restore the seal.
	if err := os.Chmod(releaseDir, 0o755); err != nil {
		log.Printf("updater: layering entrypoint backfill chmod: %v", err)
		return
	}
	writeErr := os.WriteFile(entryPath, []byte(entry+"\n"), 0o444)
	_ = os.Chmod(releaseDir, 0o555)
	if writeErr != nil {
		log.Printf("updater: layering entrypoint backfill: %v", writeErr)
	}
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

// VerifyReleaseFiles verifies that index.html and all files listed in manifest.Files
// exist on disk under releaseDir.
func VerifyReleaseFiles(releaseDir string, manifest ReleaseManifest) error {
	releaseDir = filepath.Clean(releaseDir)
	index := filepath.Join(releaseDir, "frontend", "index.html")
	if info, err := os.Stat(index); err != nil || info.IsDir() {
		return fmt.Errorf("updater: frontend index missing in %s", releaseDir)
	}
	for _, f := range manifest.Files {
		relPath := filepath.FromSlash(strings.TrimPrefix(strings.TrimSpace(f.Path), "/"))
		if relPath == "" {
			continue
		}
		target := filepath.Join(releaseDir, relPath)
		if info, err := os.Stat(target); err != nil || info.IsDir() {
			return fmt.Errorf("updater: release file missing: %s", relPath)
		}
	}
	return nil
}

// VerifyCurrentRelease verifies that the current symlink points to a valid release
// and all files declared in release-manifest.json exist on disk.
func VerifyCurrentRelease(root string) (ReleaseManifest, error) {
	root = filepath.Clean(root)
	target, err := os.Readlink(filepath.Join(root, "current"))
	if err != nil {
		return ReleaseManifest{}, err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	manifest, err := readReleaseManifest(target)
	if err != nil {
		return ReleaseManifest{}, err
	}
	if err := VerifyReleaseFiles(target, manifest); err != nil {
		return ReleaseManifest{}, err
	}
	return manifest, nil
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
	return computeApplyRequestCommitment(request)
}

// computeApplyRequestCommitment signs every field that identifies the work —
// except RealizationID. Realization rotates on each VM epoch (deploy refresh),
// but the operation journal it guards must outlive the rotation so a replayed
// apply can return the recorded outcome instead of wedging the operation.
func computeApplyRequestCommitment(request ApplyRequest) (string, error) {
	commitmentInput := request
	commitmentInput.RequestCommitment = ""
	commitmentInput.RealizationID = ""
	canonical, err := computerevent.CanonicalJSON(commitmentInput)
	if err != nil {
		return "", err
	}
	return computerevent.DigestBytes(canonical), nil
}

// validateApplyRequest is shape validation only — the commitment is computed
// independently in Apply so callers may validate without it. RealizationID
// equality is enforced separately in Apply for fresh requests only.
func validateApplyRequest(request ApplyRequest) (string, error) {
	if strings.TrimSpace(request.ComputerID) == "" || request.OperationID == "" || request.IdempotencyKey == "" ||
		!computerevent.IsSHA256(request.AcceptedEventHead) || request.Manifest.ComputerID != request.ComputerID ||
		request.Manifest.AcceptedEventHead != request.AcceptedEventHead || !filepath.IsAbs(request.SourceDir) {
		return "", fmt.Errorf("updater: incomplete or mismatched apply request")
	}
	return computeApplyRequestCommitment(request)
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
	return swapCurrentPointer(u.root, releaseDir)
}

func swapCurrentPointer(root, releaseDir string) error {
	temporaryBytes := make([]byte, 8)
	if _, err := rand.Read(temporaryBytes); err != nil {
		return err
	}
	temporary := filepath.Join(root, ".current-"+hex.EncodeToString(temporaryBytes))
	if err := os.Symlink(releaseDir, temporary); err != nil {
		return err
	}
	defer os.Remove(temporary)
	if err := os.Rename(temporary, filepath.Join(root, "current")); err != nil {
		return err
	}
	return syncDir(root)
}

func (u *Updater) restorePrior(ctx context.Context, request ApplyRequest, priorTarget, priorDigest, failedDigest string, cause error, completedAt time.Time, journal *operationJournal, journalPath string) (*computerevent.Receipt, error) {
	if priorTarget == "" {
		return nil, fmt.Errorf("updater: initial release failed and no prior release exists: %w", cause)
	}
	if !journal.RecoverySwapped {
		if err := u.swapCurrent(priorTarget); err != nil {
			return nil, fmt.Errorf("updater: restore prior pointer: %w", err)
		}
		journal.RecoverySwapped = true
		if err := writeJournal(journalPath, *journal); err != nil {
			return nil, err
		}
	}
	// The restored release's exec entrypoint travels inside its release dir
	// (S2-e). Read its manifest and backfill pre-S2-e dirs before publishing
	// the restart so the recovery start execs the release the pointer now
	// selects; the probe below reuses this manifest.
	priorManifest, err := readReleaseManifest(priorTarget)
	if err != nil {
		return nil, fmt.Errorf("updater: read restored release manifest: %w", err)
	}
	ensureReleaseEntrypoint(u.root, priorTarget, priorManifest)
	if !journal.RecoveryRestartPublished {
		if err := u.service.RecoveryRestart(ctx); err != nil {
			return nil, fmt.Errorf("updater: restart restored release: %w", err)
		}
		journal.RecoveryRestartPublished = true
		if err := writeJournal(journalPath, *journal); err != nil {
			return nil, err
		}
	}
	// Resumed after publish: the prior release is already restarting — just
	// probe it. Republishing would kill the guest mid-recovery forever.
	observations, err := u.health.Probe(ctx, priorDigest, priorManifest)
	if err != nil {
		return nil, fmt.Errorf("updater: restored release unhealthy: %w", err)
	}
	if err := u.service.CleanupRecoveryCredential(ctx); err != nil {
		return nil, fmt.Errorf("updater: cleanup recovery credential after restore: %w", err)
	}
	receipt, err := u.signer.SignReceipt(ctx, "UpdaterRecoveryReceipt", "choir-updater", map[string]any{
		"computer_id": request.ComputerID, "realization_id": request.RealizationID,
		"operation_id": request.OperationID, "failed_release_digest": failedDigest,
		"restored_release_digest": priorDigest, "accepted_event_head": request.AcceptedEventHead,
		"request_commitment": request.RequestCommitment, "failure": cause.Error(),
		"observation_artifact_digests": observations,
	}, completedAt)
	if err != nil {
		return nil, err
	}
	return &receipt, nil
}

func (u *Updater) signHealthReceipt(ctx context.Context, request ApplyRequest, releaseDigest string, startedAt, completedAt time.Time, observations []string, outcome string) (computerevent.Receipt, error) {
	probeContract, err := computerevent.CanonicalJSON(struct {
		EventSchemaVersion uint64 `json:"event_schema_version"`
		ReducerVersion     uint64 `json:"reducer_version"`
		Marker             string `json:"marker"`
	}{request.Manifest.EventSchemaVersion, request.Manifest.ReducerVersion, request.Manifest.Marker})
	if err != nil {
		return computerevent.Receipt{}, err
	}
	return u.signer.SignReceipt(ctx, "HealthReceipt", "choir-updater", map[string]any{
		"computer_id": request.ComputerID, "realization_id": request.RealizationID,
		"release_digest": releaseDigest, "probe_contract_digest": computerevent.DigestBytes(probeContract),
		"started_at": startedAt.Format(time.RFC3339Nano), "completed_at": completedAt.Format(time.RFC3339Nano),
		"outcome": outcome, "observation_artifact_digests": observations,
	}, completedAt)
}

type operationJournal struct {
	RequestCommitment   string `json:"request_commitment"`
	Phase               string `json:"phase"`
	PriorReleaseDigest  string `json:"prior_release_digest,omitempty"`
	PriorReleaseTarget  string `json:"prior_release_target,omitempty"`
	TargetReleaseDigest string `json:"target_release_digest"`
	// RestartPublished records that the guest-restart request actually reached
	// the service manager. Without it a resume cannot distinguish "restart
	// published, journal write lost" from "crashed before publish" and would
	// either re-restart a healthy guest or never restart at all.
	RestartPublished bool `json:"restart_published,omitempty"`
	// RecoverySwapped / RecoveryRestartPublished dedup the recovery path the
	// same way: a resumed apply while recovering must not swap or re-publish
	// the recovery restart — the prior caller's recovery is already in flight.
	RecoverySwapped          bool        `json:"recovery_swapped,omitempty"`
	RecoveryRestartPublished bool        `json:"recovery_restart_published,omitempty"`
	StartedAt                time.Time   `json:"started_at"`
	Failure                  string      `json:"failure,omitempty"`
	Result                   ApplyResult `json:"result"`
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

// JournalOutcome is the durable result recorded in an apply journal. Terminal
// reports whether the journal reached a completed or failed outcome; a
// mid-flight journal (prepared/swapped/restart-requested) still belongs to
// the updater's own resume path and reports Terminal=false.
type JournalOutcome struct {
	Result   ApplyResult
	Failure  string
	Terminal bool
}

// ReadJournalOutcome returns the journal recorded for an idempotency key
// under root. found=false means no journal file exists. The materializer
// uses it to re-drive terminal bookkeeping (applied/failed event,
// checkpoint, route) without re-entering the updater when a prior pass
// already completed the swap — e.g. a degraded operation whose refusal
// landed after the journal wrote phase=completed — and to keep repair
// possible while the updater socket is unavailable during early boot.
func ReadJournalOutcome(root, idempotencyKey string) (JournalOutcome, bool, error) {
	journal, found, err := readJournal(filepath.Join(filepath.Clean(strings.TrimSpace(root)), "operations", safeName(idempotencyKey)+".json"))
	if err != nil || !found {
		return JournalOutcome{}, false, err
	}
	return JournalOutcome{
		Result:   journal.Result,
		Failure:  journal.Failure,
		Terminal: journal.Result.Outcome != "",
	}, true, nil
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
	info, err := os.Stat(source)
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
