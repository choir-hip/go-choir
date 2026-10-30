package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/hostbuild"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// P5 (docs/archive/reports/p5-host-build-layered-apply-2026-10-15.md): a frozen,
// accepted candidate that touched Go input runs only as a host-built layered
// release. The capsule's own bin/autoputer is a test artifact on this path.

// errHostBuildPending keeps an accepted operation accepted while the host
// builds; the reconciler's retry wake polls again (G8: desks keep working,
// the actor hold covers only materializing).
var errHostBuildPending = errors.New("host build pending")

// hostBuildFailure is a terminal host build outcome: the operation fails
// with it and the computer keeps its current release (G3).
type hostBuildFailure struct {
	Reason string
	Detail string
	// HostRecorded marks a failure the host recorded as a failed job (the
	// P4 poster votes reject on it); every other failure is the guest's own
	// (no job, or a done job the guest could not verify or stage).
	HostRecorded bool
}

func (f hostBuildFailure) Error() string {
	return fmt.Sprintf("host build failed (%s): %s", f.Reason, f.Detail)
}

// selfdevHostBuilder is the guest's view of the corpusd host build route and
// the platform-update blob store.
type selfdevHostBuilder interface {
	Submit(ctx context.Context, req hostbuild.Request) (hostbuild.Job, int, error)
	Status(ctx context.Context, computerID, jobID string) (hostbuild.Job, int, error)
	FetchBlob(ctx context.Context, computerID, digest string) (io.ReadCloser, error)
}

type corpusdHostBuilder struct{ client *computerevent.HTTPClient }

func (c corpusdHostBuilder) Submit(ctx context.Context, req hostbuild.Request) (hostbuild.Job, int, error) {
	return hostbuild.Client{Doer: c.client}.Submit(ctx, req)
}

func (c corpusdHostBuilder) Status(ctx context.Context, computerID, jobID string) (hostbuild.Job, int, error) {
	return hostbuild.Client{Doer: c.client}.Status(ctx, computerID, jobID)
}

func (c corpusdHostBuilder) FetchBlob(ctx context.Context, computerID, digest string) (io.ReadCloser, error) {
	return c.client.FetchBlobRaw(ctx, computerID, digest)
}

func (rt *Runtime) hostBuilder() selfdevHostBuilder {
	if rt.selfdevHostBuilder != nil {
		return rt.selfdevHostBuilder
	}
	if client, ok := rt.eventPayloadReader.(*computerevent.HTTPClient); ok && client != nil {
		return corpusdHostBuilder{client: client}
	}
	return nil
}

// guestImageManifestPath is the booted base image manifest the host build is
// bound to.
func (rt *Runtime) guestImageManifestPath() string {
	if rt.selfdevGuestImageManifest != "" {
		return rt.selfdevGuestImageManifest
	}
	return strings.TrimSpace(os.Getenv("CHOIR_GUEST_IMAGE_MANIFEST"))
}

// selfDevelopmentBundleBinaryInputs returns the workspace/platform paths the
// bundle changed that are inputs of the autoputer binary, with the same
// classifier Freeze uses to decide binary reuse. Empty: a frontend-only
// candidate, which keeps the plain release path (G2).
func selfDevelopmentBundleBinaryInputs(bundle transaction.CapsuleEffectBundle) []string {
	changes := make([]capsule.FileChange, 0, len(bundle.OrderedFileEffects))
	for _, effect := range bundle.OrderedFileEffects {
		changes = append(changes, capsule.FileChange{Path: effect.Path})
	}
	return capsule.SourceChangesTouchingBinary(changes)
}

// hostBuiltRelease is a verified host build staged in the updater's
// incoming store, ready to mint.
type hostBuiltRelease struct {
	Dir   string
	Job   hostbuild.Job
	Files []updater.ManifestFile
}

// selfDevelopmentHostBuild returns the operation's host-built release,
// staged under incoming/host-build-<job>. A staged, verified release is
// reused without asking the host, so every pass mints the same manifest
// (G6). Otherwise it asks the host: pending returns errHostBuildPending, a
// failed job returns hostBuildFailure, a done job is verified (G4), fetched
// and re-hashed (G5), and staged.
func (rt *Runtime) selfDevelopmentHostBuild(ctx context.Context, operation selfdev.Operation, bundle transaction.CapsuleEffectBundle) (hostBuiltRelease, error) {
	if bundle.SourcePatchSHA256 == "" || strings.TrimSpace(bundle.SourcePatchBaseCommit) == "" {
		return hostBuiltRelease{}, hostBuildFailure{Reason: "patch_unavailable", Detail: "the candidate touched Go input but its bundle records no source patch and base commit"}
	}
	bundleRoot := filepath.Join(rt.selfdevUpdaterRoot, "incoming", operation.BundleDigest)
	patch, _, err := readBoundSourcePatch(filepath.Join(bundleRoot, "source.patch"), bundle.SourcePatchSHA256)
	if err != nil {
		return hostBuiltRelease{}, hostBuildFailure{Reason: "patch_unavailable", Detail: err.Error()}
	}
	request, err := rt.selfDevelopmentHostBuildRequest(operation, bundle)
	if err != nil {
		return hostBuiltRelease{}, err
	}
	jobID, err := request.JobID()
	if err != nil {
		return hostBuiltRelease{}, err
	}
	if rt.selfdevControl == nil {
		return hostBuiltRelease{}, hostBuildFailure{Reason: "platform_key_unavailable", Detail: "no platform-control key to verify a host build"}
	}
	platformKey := rt.selfdevControl.PublicKey()
	stagedDir := filepath.Join(rt.selfdevUpdaterRoot, "incoming", "host-build-"+jobID)
	if staged, ok := readStagedHostBuild(stagedDir, request, platformKey); ok {
		return staged, nil
	}

	builder := rt.hostBuilder()
	if builder == nil {
		return hostBuiltRelease{}, hostBuildFailure{Reason: hostbuild.ReasonHostBuilderUnconfigured, Detail: "this computer has no corpusd client for the host build route"}
	}
	job, status, err := builder.Status(ctx, operation.ComputerID, jobID)
	if status == http.StatusNotFound {
		request.PatchBase64 = base64.StdEncoding.EncodeToString(patch)
		job, status, err = builder.Submit(ctx, request)
		request.PatchBase64 = ""
	}
	if err != nil {
		switch {
		case status == http.StatusServiceUnavailable && strings.Contains(err.Error(), hostbuild.ReasonHostBuilderUnconfigured):
			return hostBuiltRelease{}, hostBuildFailure{Reason: hostbuild.ReasonHostBuilderUnconfigured, Detail: "the host has no build service configured"}
		case status == http.StatusTooManyRequests && strings.Contains(err.Error(), hostbuild.ReasonBuildQuotaExhausted):
			return hostBuiltRelease{}, hostBuildFailure{Reason: hostbuild.ReasonBuildQuotaExhausted, Detail: err.Error()}
		case status == http.StatusTooManyRequests:
			// queue_full / computer_busy: the host recorded nothing; stay
			// accepted and submit again on the next pass (H20).
			return hostBuiltRelease{}, fmt.Errorf("%w: job %s: %v", errHostBuildPending, jobID, err)
		case status == http.StatusBadRequest || status == http.StatusForbidden:
			return hostBuiltRelease{}, hostBuildFailure{Reason: "request_refused", Detail: err.Error()}
		default:
			// Transport or a transient host error: stay accepted, retry.
			return hostBuiltRelease{}, fmt.Errorf("host build %s: %w", jobID, err)
		}
	}
	switch job.State {
	case hostbuild.StateQueued, hostbuild.StateRunning:
		return hostBuiltRelease{}, fmt.Errorf("%w: job %s %s %s", errHostBuildPending, jobID, job.State, job.Admission)
	case hostbuild.StateFailed:
		return hostBuiltRelease{}, hostBuildFailure{Reason: job.Reason, Detail: job.Detail, HostRecorded: true}
	case hostbuild.StateDone:
	default:
		return hostBuiltRelease{}, fmt.Errorf("host build %s: unknown state %q", jobID, job.State)
	}
	if err := hostbuild.VerifyJob(job, request, platformKey); err != nil {
		return hostBuiltRelease{}, hostBuildFailure{Reason: "receipt_refused", Detail: err.Error()}
	}
	staged, err := stageHostBuild(ctx, builder, rt.selfdevUpdaterRoot, stagedDir, job)
	if err != nil {
		var failure hostBuildFailure
		if errors.As(err, &failure) {
			return hostBuiltRelease{}, failure
		}
		return hostBuiltRelease{}, fmt.Errorf("host build %s: stage: %w", jobID, err)
	}
	return staged, nil
}

// selfDevelopmentHostBuildRequest is P5's job identity for an operation,
// without the patch bytes. corpusd's P4 decision poster derives the same
// identity, so the job it votes on is the job this guest applies.
func (rt *Runtime) selfDevelopmentHostBuildRequest(operation selfdev.Operation, bundle transaction.CapsuleEffectBundle) (hostbuild.Request, error) {
	bootedBase, err := updater.DigestFile(rt.guestImageManifestPath())
	if err != nil {
		return hostbuild.Request{}, hostBuildFailure{Reason: "base_unreadable", Detail: "the booted guest image manifest is unreadable: " + err.Error()}
	}
	return hostbuild.Request{
		ComputerID: operation.ComputerID, OperationID: operation.OperationID, BundleDigest: operation.BundleDigest,
		BaseCommit: strings.ToLower(strings.TrimSpace(bundle.SourcePatchBaseCommit)), SourcePatchSHA256: bundle.SourcePatchSHA256,
		BaseImageManifestDigest: bootedBase,
	}, nil
}

// stageHostBuild fetches the closure and builder receipt, re-hashes both
// against the signed result, lists the layer's frontend from the closure,
// and renames the staged tree into place.
func stageHostBuild(ctx context.Context, builder selfdevHostBuilder, updaterRoot, stagedDir string, job hostbuild.Job) (hostBuiltRelease, error) {
	result := job.Result
	tmp, err := os.MkdirTemp(filepath.Join(updaterRoot, "incoming"), ".host-build-")
	if err != nil {
		return hostBuiltRelease{}, err
	}
	defer os.RemoveAll(tmp)
	if err := os.Chmod(tmp, 0o700); err != nil {
		return hostBuiltRelease{}, err
	}
	for name, digest := range map[string]string{"closure.nar": result.ClosureDigest, "builder-receipt.json": result.BuilderReceiptDigest} {
		if err := fetchVerifiedBlob(ctx, builder, job.ComputerID, digest, filepath.Join(tmp, name)); err != nil {
			return hostBuiltRelease{}, err
		}
	}
	blob, err := os.ReadFile(filepath.Join(tmp, "closure.nar"))
	if err != nil {
		return hostBuiltRelease{}, err
	}
	if _, err := updater.ExtractClosureSubtree(blob, filepath.Base(result.RuntimePath), "frontend", tmp); err != nil {
		return hostBuiltRelease{}, hostBuildFailure{Reason: "layer_frontend_unavailable", Detail: err.Error()}
	}
	raw, err := json.Marshal(job)
	if err != nil {
		return hostBuiltRelease{}, err
	}
	if err := os.WriteFile(filepath.Join(tmp, "host-build.json"), raw, 0o444); err != nil {
		return hostBuiltRelease{}, err
	}
	_ = os.RemoveAll(stagedDir)
	if err := os.Rename(tmp, stagedDir); err != nil {
		return hostBuiltRelease{}, err
	}
	files, err := hostBuildManifestFiles(stagedDir, job)
	if err != nil {
		return hostBuiltRelease{}, err
	}
	return hostBuiltRelease{Dir: stagedDir, Job: job, Files: files}, nil
}

func fetchVerifiedBlob(ctx context.Context, builder selfdevHostBuilder, computerID, digest, target string) error {
	body, err := builder.FetchBlob(ctx, computerID, digest)
	if err != nil {
		return fmt.Errorf("fetch blob %s: %w", digest, err)
	}
	defer body.Close()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o444)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	if _, err := io.Copy(out, io.TeeReader(body, hasher)); err != nil {
		out.Close()
		return fmt.Errorf("fetch blob %s: %w", digest, err)
	}
	if err := out.Close(); err != nil {
		return err
	}
	if got := hex.EncodeToString(hasher.Sum(nil)); got != digest {
		return hostBuildFailure{Reason: "blob_digest_mismatch", Detail: fmt.Sprintf("fetched %s hashes to %s", digest, got)}
	}
	return nil
}

// readStagedHostBuild reuses a staged host build when its record still
// verifies for this request and its files still hash to the record.
func readStagedHostBuild(stagedDir string, request hostbuild.Request, platformKey []byte) (hostBuiltRelease, bool) {
	raw, err := os.ReadFile(filepath.Join(stagedDir, "host-build.json"))
	if err != nil {
		return hostBuiltRelease{}, false
	}
	var job hostbuild.Job
	if json.Unmarshal(raw, &job) != nil || hostbuild.VerifyJob(job, request, platformKey) != nil {
		return hostBuiltRelease{}, false
	}
	for name, digest := range map[string]string{"closure.nar": job.Result.ClosureDigest, "builder-receipt.json": job.Result.BuilderReceiptDigest} {
		if got, err := updater.DigestFile(filepath.Join(stagedDir, name)); err != nil || got != digest {
			return hostBuiltRelease{}, false
		}
	}
	files, err := hostBuildManifestFiles(stagedDir, job)
	if err != nil {
		return hostBuiltRelease{}, false
	}
	return hostBuiltRelease{Dir: stagedDir, Job: job, Files: files}, true
}

// hostBuildManifestFiles lists the staged release: the closure, the builder
// receipt, the signed host build record, and the layer's frontend.
func hostBuildManifestFiles(stagedDir string, job hostbuild.Job) ([]updater.ManifestFile, error) {
	record, err := updater.DigestFile(filepath.Join(stagedDir, "host-build.json"))
	if err != nil {
		return nil, err
	}
	files := []updater.ManifestFile{
		{Path: "builder-receipt.json", SHA256: job.Result.BuilderReceiptDigest, Mode: 0o444},
		{Path: "closure.nar", SHA256: job.Result.ClosureDigest, Mode: 0o444},
		{Path: "host-build.json", SHA256: record, Mode: 0o444},
	}
	frontendRoot := filepath.Join(stagedDir, "frontend")
	err = filepath.WalkDir(frontendRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("staged frontend %s is not a regular file", path)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		digest, err := updater.DigestFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(stagedDir, path)
		if err != nil {
			return err
		}
		mode := uint32(0o444)
		if info.Mode().Perm()&0o111 != 0 {
			mode = 0o555
		}
		files = append(files, updater.ManifestFile{Path: filepath.ToSlash(rel), SHA256: digest, Mode: mode})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func effectiveReleaseFromManifest(manifest updater.ReleaseManifest) *effectiveReleaseIdentity {
	return &effectiveReleaseIdentity{
		ReleaseDigest: manifest.ContentDigest, Layered: manifest.LayeringEntrypoint != "",
		CodeCommit: manifest.CodeCommit, BaseCommit: manifest.BaseCommit, BaseImageDigest: manifest.BaseImageManifestDigest,
		ClosureDigest: manifest.ClosureDigest, BuilderReceiptDigest: manifest.BuilderReceiptDigest,
	}
}

// layeredSelfDevelopmentManifest mints the layered release for a host build.
func layeredSelfDevelopmentManifest(base updater.ReleaseManifest, built hostBuiltRelease) updater.ReleaseManifest {
	r := built.Job.Result
	base.Files = built.Files
	base.BaseImageManifestDigest = r.BaseImageManifestDigest
	base.ClosureDigest = r.ClosureDigest
	base.LayeringEntrypoint = r.LayeringEntrypoint
	base.StoreSchemaVersion = r.StoreSchemaVersion
	base.BaseCommit = r.BaseCommit
	base.CodeCommit = r.CodeCommit
	base.BuilderReceiptDigest = r.BuilderReceiptDigest
	return base
}
