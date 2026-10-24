package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/release"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/sourceproj"
	"github.com/yusefmosiah/go-choir/internal/updater"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// EffectiveRelease is what code this computer runs and at which head (P6,
// docs/reports/p6-p8-release-adopt-2026-10-15.md): the CLAUDE.md "Product
// restore vs platform deploy" pre-flight. Platform deploy SHA and a
// computer's effective identity diverge on purpose once computers
// self-develop, so identity is never the SHA alone.
//
// Each field reads its single authority: the manifest `current` points at
// (live), the manifest this process started with, the running binary, the
// applied operation whose release is current (its bound bundle and patch),
// the guest store's event head, and the adoption that produced the
// operation. P5's layered fields (code commit, closure, builder receipt)
// come from the same manifest; P5 reads this block rather than adding one.
type EffectiveRelease struct {
	ReleaseDigest           string `json:"release_digest,omitempty"`
	ServingReleaseDigest    string `json:"serving_release_digest,omitempty"`
	Marker                  string `json:"marker,omitempty"`
	CodeRef                 string `json:"code_ref,omitempty"`
	ArtifactProgramRef      string `json:"artifact_program_ref,omitempty"`
	Layered                 bool   `json:"layered"`
	CodeCommit              string `json:"code_commit,omitempty"`
	BaseCommit              string `json:"base_commit,omitempty"`
	ClosureDigest           string `json:"closure_digest,omitempty"`
	BuilderReceiptDigest    string `json:"builder_receipt_digest,omitempty"`
	BaseImageManifestDigest string `json:"base_image_manifest_digest,omitempty"`
	BuildCommit             string `json:"build_commit,omitempty"`
	// OperationID is the applied self-development operation whose release
	// is current; empty on a computer running its base release.
	OperationID       string `json:"operation_id,omitempty"`
	SourceBaseCommit  string `json:"source_base_commit,omitempty"`
	SourcePatchSHA256 string `json:"source_patch_sha256,omitempty"`
	// TreeHash is the git tree of the effective source: the operation's base
	// commit plus its patch, or the build commit's tree for a base release.
	TreeHash    string `json:"tree_hash,omitempty"`
	TreePending bool   `json:"tree_pending,omitempty"`
	TreeError   string `json:"tree_error,omitempty"`
	// AdoptedReleaseID names the published release the operation adopted.
	AdoptedReleaseID string              `json:"adopted_release_id,omitempty"`
	EventHead        *EffectiveEventHead `json:"event_head,omitempty"`
	ManifestError    string              `json:"manifest_error,omitempty"`
	// Diverged and SourceProjection are U1's (section 4.6): divergence is a
	// live stack, derived from the tape; a retired (upstreamed) stack is not
	// divergence. Both are absent while the switch is off or before the first
	// reconcile.
	Diverged         *bool              `json:"diverged,omitempty"`
	SourceProjection *sourceproj.Result `json:"source_projection,omitempty"`
}

// EffectiveEventHead is the computer's event chain head as the guest store
// projects it.
type EffectiveEventHead struct {
	Sequence  uint64 `json:"sequence"`
	Canonical string `json:"canonical"`
	Desired   string `json:"desired"`
	Effective string `json:"effective"`
}

// effectiveTreeEntry is one release's resolved source identity. A final
// entry is never recomputed (failure mode I4); a non-final one (the applied
// operation is not recorded yet, the apply is in flight) is retried after
// effectiveTreeRetry.
type effectiveTreeEntry struct {
	pending           bool
	final             bool
	resolvedAt        time.Time
	operationID       string
	sourceBaseCommit  string
	sourcePatchSHA256 string
	treeHash          string
	treeError         string
	adoptedReleaseID  string
}

const effectiveTreeRetry = 30 * time.Second

func defaultEffectiveTreeCompute(ctx context.Context, repo, commit string, patch []byte) (string, error) {
	if len(patch) == 0 {
		return release.CommitTree(ctx, repo, commit)
	}
	return release.PatchedTree(ctx, repo, commit, patch)
}

// releaseRepo is the guest's own source repository: the capsule source root.
func (rt *Runtime) releaseRepo() string {
	if rt != nil && strings.TrimSpace(rt.releaseSourceDir) != "" {
		return rt.releaseSourceDir
	}
	return strings.TrimSpace(os.Getenv("CHOIR_CAPSULE_SOURCE_ROOT"))
}

func (rt *Runtime) buildCommit() string {
	if rt != nil && rt.effectiveBuildCommit != nil {
		return rt.effectiveBuildCommit()
	}
	return strings.TrimSpace(buildinfo.Snapshot("autoputer").Commit)
}

// EffectiveRelease answers from the live authorities. It never runs git on
// the caller's path: the source identity of a release is resolved once, in
// the background, and served from the cache (I4); until then TreePending.
func (rt *Runtime) EffectiveRelease(ctx context.Context) EffectiveRelease {
	var out EffectiveRelease
	if rt == nil {
		return out
	}
	out.ServingReleaseDigest = rt.startupManifest().Digest
	out.BuildCommit = rt.buildCommit()
	if root := strings.TrimSpace(rt.selfdevUpdaterRoot); root != "" {
		if manifest, err := readCurrentManifestIdentity(root); err == nil {
			out.ReleaseDigest, out.Marker = manifest.ContentDigest, manifest.Marker
			out.CodeRef, out.ArtifactProgramRef = manifest.CodeRef, manifest.ArtifactProgramRef
			out.Layered = manifest.LayeringEntrypoint != "" && manifest.ClosureDigest != ""
			out.CodeCommit, out.BaseCommit = manifest.CodeCommit, manifest.BaseCommit
			out.ClosureDigest, out.BuilderReceiptDigest = manifest.ClosureDigest, manifest.BuilderReceiptDigest
			out.BaseImageManifestDigest = manifest.BaseImageManifestDigest
		} else if !os.IsNotExist(err) {
			out.ManifestError = err.Error()
		}
	}
	// I5: the head is read live, never cached.
	if rt.store != nil {
		if head, err := rt.store.Head(ctx, rt.cfg.ComputerID); err == nil && head != nil {
			out.EventHead = &EffectiveEventHead{Sequence: head.Sequence, Canonical: head.CanonicalEventHead, Desired: head.DesiredEventHead, Effective: head.EffectiveEventHead}
		}
	}
	entry := rt.effectiveTree(out.ReleaseDigest, out.Marker, out.BuildCommit)
	out.TreePending = entry.pending
	out.OperationID, out.SourceBaseCommit, out.SourcePatchSHA256 = entry.operationID, entry.sourceBaseCommit, entry.sourcePatchSHA256
	out.TreeHash, out.TreeError, out.AdoptedReleaseID = entry.treeHash, entry.treeError, entry.adoptedReleaseID
	if projection := rt.sourceProjectionStatus(); projection != nil {
		diverged := projection.Diverged
		out.Diverged, out.SourceProjection = &diverged, projection
	}
	return out
}

// effectiveTree returns the cached source identity of a release, starting
// its resolution in the background when there is none (or a non-final one
// has aged out).
func (rt *Runtime) effectiveTree(releaseDigest, marker, buildCommit string) effectiveTreeEntry {
	key := releaseDigest
	if key == "" {
		key = "build:" + buildCommit
	}
	rt.effectiveTreeMu.Lock()
	defer rt.effectiveTreeMu.Unlock()
	if rt.effectiveTrees == nil {
		rt.effectiveTrees = make(map[string]*effectiveTreeEntry)
	}
	if entry := rt.effectiveTrees[key]; entry != nil {
		if entry.pending || entry.final || time.Since(entry.resolvedAt) < effectiveTreeRetry {
			return *entry
		}
		previous := *entry
		entry.pending = true
		go rt.resolveEffectiveTree(key, releaseDigest, marker, buildCommit)
		previous.pending = true
		return previous
	}
	rt.effectiveTrees[key] = &effectiveTreeEntry{pending: true}
	go rt.resolveEffectiveTree(key, releaseDigest, marker, buildCommit)
	return effectiveTreeEntry{pending: true}
}

func (rt *Runtime) resolveEffectiveTree(key, releaseDigest, marker, buildCommit string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	entry := rt.computeEffectiveTree(ctx, releaseDigest, marker, buildCommit)
	entry.resolvedAt = time.Now()
	rt.effectiveTreeMu.Lock()
	rt.effectiveTrees[key] = &entry
	rt.effectiveTreeMu.Unlock()
}

func (rt *Runtime) computeEffectiveTree(ctx context.Context, releaseDigest, marker, buildCommit string) effectiveTreeEntry {
	compute := rt.effectiveTreeCompute
	if compute == nil {
		compute = defaultEffectiveTreeCompute
	}
	repo := rt.releaseRepo()
	if releaseDigest != "" && rt.selfdevOperations != nil {
		if operation, ok := rt.appliedOperationForRelease(ctx, releaseDigest); ok {
			entry := effectiveTreeEntry{final: true, operationID: operation.OperationID}
			entry.adoptedReleaseID = rt.adoptedReleaseForTrajectory(ctx, operation.TrajectoryID)
			base, patch, err := rt.boundSourcePatch(operation)
			if err != nil {
				entry.treeError = err.Error()
				return entry
			}
			entry.sourceBaseCommit, entry.sourcePatchSHA256 = base, computerevent.DigestBytes(patch)
			if len(patch) == 0 {
				entry.sourcePatchSHA256 = ""
			}
			if tree, treeErr := compute(ctx, repo, base, patch); treeErr != nil {
				entry.treeError = treeErr.Error()
			} else {
				entry.treeHash = tree
			}
			return entry
		}
		if strings.HasPrefix(marker, "selfdev-") {
			// The apply is in flight (the updater probes /health before the
			// operation records applied): not final, retried.
			return effectiveTreeEntry{treeError: "self-development release whose applied operation is not recorded yet"}
		}
	}
	entry := effectiveTreeEntry{final: true}
	if !looksLikeGitCommit(buildCommit) {
		entry.treeError = "the running build names no commit"
		return entry
	}
	tree, err := compute(ctx, repo, buildCommit, nil)
	if err != nil {
		entry.treeError = err.Error()
		return entry
	}
	entry.treeHash = tree
	return entry
}

// readCurrentManifestIdentity reads the manifest `current` points at and
// checks its content digest, without re-hashing the release's files: /health
// must not hash a runtime binary on every call. The updater verifies the
// files when it stages and swaps.
func readCurrentManifestIdentity(root string) (updater.ReleaseManifest, error) {
	target, err := os.Readlink(filepath.Join(root, "current"))
	if err != nil {
		return updater.ReleaseManifest{}, err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	raw, err := os.ReadFile(filepath.Join(target, "release-manifest.json"))
	if err != nil {
		return updater.ReleaseManifest{}, err
	}
	var manifest updater.ReleaseManifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return updater.ReleaseManifest{}, fmt.Errorf("current release manifest: %w", err)
	}
	unsigned := manifest
	unsigned.ContentDigest = ""
	canonical, err := computerevent.CanonicalJSON(unsigned)
	if err != nil || computerevent.DigestBytes(canonical) != manifest.ContentDigest {
		return updater.ReleaseManifest{}, fmt.Errorf("current release manifest: content digest mismatch")
	}
	return manifest, nil
}

func looksLikeGitCommit(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

// appliedOperationForRelease is the applied operation whose release digest
// is releaseDigest and that froze a bundle.
func (rt *Runtime) appliedOperationForRelease(ctx context.Context, releaseDigest string) (selfdev.Operation, bool) {
	operations, err := rt.selfdevOperations.ListByStates(ctx, rt.cfg.ComputerID, selfdev.StateApplied)
	if err != nil {
		return selfdev.Operation{}, false
	}
	for i := len(operations) - 1; i >= 0; i-- {
		if operations[i].ReleaseDigest == releaseDigest && operations[i].BundleDigest != "" {
			return operations[i], true
		}
	}
	return selfdev.Operation{}, false
}

// boundSourcePatch reads the operation's bound bundle and its source patch:
// the base commit and the bytes, or an empty patch when the change touched
// no source.
func (rt *Runtime) boundSourcePatch(operation selfdev.Operation) (string, []byte, error) {
	if strings.TrimSpace(rt.selfdevUpdaterRoot) == "" || !computerevent.IsSHA256(operation.BundleDigest) {
		return "", nil, errBundleUnavailable
	}
	bundleRoot := filepath.Join(rt.selfdevUpdaterRoot, "incoming", operation.BundleDigest)
	bundle, _, err := readBoundSelfDevelopmentBundle(bundleRoot, operation)
	if err != nil {
		return "", nil, err
	}
	if bundle.SourcePatchSHA256 == "" {
		return bundle.SourcePatchBaseCommit, nil, errNoSourcePatch
	}
	patch, _, err := readBoundSourcePatch(filepath.Join(bundleRoot, "source.patch"), bundle.SourcePatchSHA256)
	if err != nil {
		return "", nil, err
	}
	return bundle.SourcePatchBaseCommit, patch, nil
}

type releaseIdentityError string

func (e releaseIdentityError) Error() string { return string(e) }

const (
	errBundleUnavailable = releaseIdentityError("the applied operation's bundle is not retained on this computer")
	errNoSourcePatch     = releaseIdentityError("the applied operation changed no workspace/platform source")
)

// reportEffectiveReleaseAsync tells vmctl what this computer runs, so
// `vmctl list` shows it without probing a guest (I6). It waits for the
// release's tree (bounded), then posts; a failed post is retried a few
// times and otherwise only logged: identity reporting never blocks boot,
// apply or restore.
func (rt *Runtime) reportEffectiveReleaseAsync(reason string) {
	if rt == nil || rt.selfdevRoute == nil || strings.TrimSpace(rt.cfg.ComputerID) == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		var identity EffectiveRelease
		for i := 0; i < 60; i++ {
			identity = rt.EffectiveRelease(ctx)
			if !identity.TreePending {
				break
			}
			time.Sleep(2 * time.Second)
		}
		if !computerevent.IsSHA256(identity.ReleaseDigest) {
			return
		}
		report := vmctl.ReportedRelease{
			ReleaseDigest: identity.ReleaseDigest, CodeRef: identity.CodeRef, TreeHash: identity.TreeHash,
			BuildCommit: identity.BuildCommit, OperationID: identity.OperationID, AdoptedReleaseID: identity.AdoptedReleaseID,
			Layered: identity.Layered, CodeCommit: identity.CodeCommit,
		}
		if head := identity.EventHead; head != nil {
			report.EventHeadSequence, report.CanonicalEventHead, report.EffectiveEventHead = head.Sequence, head.Canonical, head.Effective
		}
		for attempt := 1; attempt <= 3; attempt++ {
			err := rt.selfdevRoute.ReportEffectiveRelease(ctx, rt.cfg.ComputerID, report)
			if err == nil {
				log.Printf("runtime: effective release reported to vmctl (%s): release %s tree %s", reason, shortDigest(report.ReleaseDigest), shortDigest(report.TreeHash))
				return
			}
			log.Printf("runtime: effective release report (%s) attempt %d: %v", reason, attempt, err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
			}
		}
	}()
}

func shortDigest(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}
