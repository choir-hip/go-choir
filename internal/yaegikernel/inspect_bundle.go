package yaegikernel

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// selfDevBundleMountPoint is the read-only lower-layer path where the host
// installs the exact frozen self-development bundle for a verifier-slot
// activation at spawn. The mount is the binding: the host writes
// binding.json alongside the bundle and the cell never supplies identity.
const selfDevBundleMountPoint = "/selfdev/bundle"

// mountedBundleBinding is the host-written binding record installed beside
// the frozen bundle. It carries the durable operation identity the mounted
// bytes answer to.
type mountedBundleBinding struct {
	OperationID  string `json:"operation_id"`
	BundleDigest string `json:"bundle_digest"`
}

// bundleDraftMirror mirrors transaction.CapsuleEffectBundle's wire shape so
// the in-cell inspection can recompute the content digest without importing
// the transaction package (which would cycle through capsule). Drift fails
// closed: a mismatched mirror produces a mismatched digest.
type bundleDraftMirror struct {
	BundleVersion           int                        `json:"bundle_version"`
	ComputerID              string                     `json:"computer_id"`
	BaseEventHead           string                     `json:"base_event_head"`
	TrajectoryRef           string                     `json:"trajectory_ref"`
	CapsuleIdentity         string                     `json:"capsule_identity"`
	CapabilityPolicyDigest  string                     `json:"capability_policy_digest"`
	SourceTreeRef           string                     `json:"source_tree_ref"`
	OrderedFileEffects      []json.RawMessage          `json:"ordered_file_effects"`
	GeneratedArtifactRefs   []string                   `json:"generated_artifact_refs"`
	BuildRecipeRef          string                     `json:"build_recipe_ref"`
	RuntimeArtifactRef      string                     `json:"runtime_artifact_ref"`
	TestReceipts            []string                   `json:"test_receipts"`
	VerifierReceipts        []string                   `json:"verifier_receipts"`
	DependencyToolchainRefs []string                   `json:"dependency_toolchain_refs"`
	ResourceReceipts        []string                   `json:"resource_receipts"`
	ContentDigest           string                     `json:"content_digest"`
	ClassifierV             string                     `json:"classifier_version"`
	ClassifierDigest        string                     `json:"classifier_digest"`
	Groups                  map[string]json.RawMessage `json:"groups"`
	Ignored                 []json.RawMessage          `json:"ignored"`
	Unknown                 []json.RawMessage          `json:"unknown,omitempty"`
	Rejected                bool                       `json:"rejected"`
	RejectReason            string                     `json:"reject_reason,omitempty"`
	RuntimeFiles            []mountedRuntimeFile       `json:"runtime_files"`
	SourcePatchSHA256       string                     `json:"source_patch_sha256,omitempty"`
	SourcePatchBaseCommit   string                     `json:"source_patch_base_commit,omitempty"`
	ReleaseBinary           *mountedReleaseBinary      `json:"release_binary,omitempty"`
	LegacyVisualEvidence    json.RawMessage            `json:"visual_evidence,omitempty"`
	Screens                 []mountedScreen            `json:"screens,omitempty"`
	Video                   *mountedVideo              `json:"video,omitempty"`
}

// mountedVideo mirrors capsule.VideoFile (the one walkthrough video,
// docs/problems/library-releases-have-no-screenshots-2026-11-04.md).
type mountedVideo struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

// mountedScreen mirrors capsule.ScreenFile (the engineer's screenshots,
// docs/problems/selfdev-screens-are-not-files-2026-11-03.md).
type mountedScreen struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Blank  bool   `json:"blank,omitempty"`
}

// mountedReleaseBinary mirrors capsule.ReleaseBinaryProvenance.
type mountedReleaseBinary struct {
	Origin           string `json:"origin"`
	SHA256           string `json:"sha256"`
	RunningCommit    string `json:"running_commit,omitempty"`
	SourceBaseCommit string `json:"source_base_commit,omitempty"`
}

type mountedRuntimeFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Mode   uint32 `json:"mode"`
}

// inspectMountedBundle verifies the mounted frozen bundle: the binding
// record, the draft's declared content digest (recomputed over the canonical
// draft with content_digest and verifier_receipts cleared, matching
// CapsuleEffectBundle.ComputeContentDigest), and every runtime file's
// sha256. It returns the canonical receipt fields the retired
// inspect_self_development_bundle tool returned; execution_receipts carries
// the receipt REFS only (bodies embed occurred_at and are never compared).
func inspectMountedBundle() (map[string]any, error) {
	return inspectMountedBundleAt(selfDevBundleMountPoint)
}

// InspectMountedBundleAt inspects a frozen bundle laid out under root. It
// lets the producer's package pin that this mirror reads every field it
// writes.
func InspectMountedBundleAt(root string) (map[string]any, error) {
	return inspectMountedBundleAt(root)
}

func inspectMountedBundleAt(root string) (map[string]any, error) {
	rawBinding, err := os.ReadFile(filepath.Join(root, "binding.json"))
	if err != nil {
		return nil, fmt.Errorf("choir: inspect_bundle: mounted bundle binding unavailable")
	}
	var binding mountedBundleBinding
	if err := json.Unmarshal(rawBinding, &binding); err != nil ||
		strings.TrimSpace(binding.OperationID) == "" || !computerevent.IsSHA256(strings.TrimSpace(binding.BundleDigest)) {
		return nil, fmt.Errorf("choir: inspect_bundle: mounted bundle binding is invalid")
	}
	rawDraft, err := os.ReadFile(filepath.Join(root, "bundle.draft.json"))
	if err != nil {
		return nil, fmt.Errorf("choir: inspect_bundle: immutable bundle draft unavailable")
	}
	var record bundleDraftMirror
	decoder := json.NewDecoder(bytes.NewReader(rawDraft))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return nil, fmt.Errorf("choir: inspect_bundle: invalid frozen bundle draft")
	}
	if record.ContentDigest != binding.BundleDigest {
		return nil, fmt.Errorf("choir: inspect_bundle: draft digest does not match mounted binding")
	}
	// Recompute the content digest exactly as
	// CapsuleEffectBundle.ComputeContentDigest does: canonical JSON with
	// content_digest and verifier_receipts cleared.
	recompute := record
	recompute.ContentDigest = ""
	recompute.VerifierReceipts = []string{}
	canonical, err := computerevent.CanonicalJSON(recompute)
	if err != nil {
		return nil, fmt.Errorf("choir: inspect_bundle: canonical draft: %w", err)
	}
	if computerevent.DigestBytes(canonical) != record.ContentDigest {
		return nil, fmt.Errorf("choir: inspect_bundle: draft content digest mismatch")
	}
	for _, file := range record.RuntimeFiles {
		clean := filepath.Clean(filepath.FromSlash(file.Path))
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
			return nil, fmt.Errorf("choir: inspect_bundle: unsafe runtime file path %q", file.Path)
		}
		input, err := os.Open(filepath.Join(root, clean))
		if err != nil {
			return nil, fmt.Errorf("choir: inspect_bundle: frozen runtime file unavailable: %s", file.Path)
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, input)
		closeErr := input.Close()
		if copyErr != nil || closeErr != nil || hex.EncodeToString(hash.Sum(nil)) != file.SHA256 {
			return nil, fmt.Errorf("choir: inspect_bundle: frozen runtime file digest mismatch: %s", file.Path)
		}
	}
	screens, err := inspectMountedScreens(root, binding.OperationID, record.Screens)
	if err != nil {
		return nil, err
	}
	video, err := inspectMountedVideo(root, record.Video)
	if err != nil {
		return nil, err
	}
	executionRefs := append([]string{record.BuildRecipeRef}, record.TestReceipts...)
	executionRefs = append(executionRefs, record.DependencyToolchainRefs...)
	runtimeFiles := make([]map[string]any, 0, len(record.RuntimeFiles))
	for _, file := range record.RuntimeFiles {
		runtimeFiles = append(runtimeFiles, map[string]any{"path": file.Path, "sha256": file.SHA256, "mode": file.Mode})
	}
	sort.Slice(runtimeFiles, func(i, j int) bool { return runtimeFiles[i]["path"].(string) < runtimeFiles[j]["path"].(string) })
	out := map[string]any{
		"operation_id": binding.OperationID, "content_digest": record.ContentDigest,
		"source_tree_ref": record.SourceTreeRef, "runtime_artifact_ref": record.RuntimeArtifactRef,
		"base_event_head": record.BaseEventHead, "runtime_files": runtimeFiles,
		"build_recipe_ref": record.BuildRecipeRef, "test_receipts": record.TestReceipts,
		"dependency_toolchain_refs": record.DependencyToolchainRefs, "resource_receipts": record.ResourceReceipts,
		"execution_receipts": executionRefs,
		"classifier_version": record.ClassifierV, "classifier_digest": record.ClassifierDigest, "groups": record.Groups,
	}
	if binary := record.ReleaseBinary; binary != nil {
		// Where bin/autoputer came from: built in the capsule, or this
		// computer's running binary reused for a change with no Go input.
		out["release_binary"] = map[string]any{
			"origin": binary.Origin, "sha256": binary.SHA256,
			"running_commit": binary.RunningCommit, "source_base_commit": binary.SourceBaseCommit,
		}
	}
	if len(screens) > 0 {
		out["screens"] = screens
	}
	if video != nil {
		out["video"] = video
	}
	return out, nil
}

// inspectMountedVideo re-hashes the bundle's video under
// <root>/evidence/screens/. nil for a bundle with none.
func inspectMountedVideo(root string, video *mountedVideo) (map[string]any, error) {
	if video == nil {
		return nil, nil
	}
	if !computerevent.IsSHA256(video.SHA256) || video.Name != filepath.Base(video.Name) || !strings.HasSuffix(video.Name, ".webm") {
		return nil, fmt.Errorf("choir: inspect_bundle: video %q is not a content-addressed WebM", video.Name)
	}
	data, err := os.ReadFile(filepath.Join(root, "evidence", "screens", video.Name))
	if err != nil {
		return nil, fmt.Errorf("choir: inspect_bundle: video unavailable: %s", video.Name)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != video.SHA256 || int64(len(data)) != video.Bytes {
		return nil, fmt.Errorf("choir: inspect_bundle: video digest mismatch: %s", video.Name)
	}
	return map[string]any{"name": video.Name, "sha256": video.SHA256, "bytes": video.Bytes}, nil
}

// inspectMountedScreens re-hashes every screen the bundle cites under
// <root>/evidence/screens/, so a verifier cites what it inspected. Each row
// names the file the runtime saved it as in the computer's file tree and the
// asset ref a vision call takes. nil for a bundle with none.
func inspectMountedScreens(root, operationID string, screens []mountedScreen) ([]map[string]any, error) {
	if len(screens) == 0 {
		return nil, nil
	}
	out := make([]map[string]any, 0, len(screens))
	for _, screen := range screens {
		if !computerevent.IsSHA256(screen.SHA256) || screen.Name != filepath.Base(screen.Name) || !strings.HasSuffix(screen.Name, ".png") {
			return nil, fmt.Errorf("choir: inspect_bundle: screen %q is not a content-addressed PNG", screen.Name)
		}
		data, err := os.ReadFile(filepath.Join(root, "evidence", "screens", screen.Name))
		if err != nil {
			return nil, fmt.Errorf("choir: inspect_bundle: screen unavailable: %s", screen.Name)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != screen.SHA256 || int64(len(data)) != screen.Bytes {
			return nil, fmt.Errorf("choir: inspect_bundle: screen digest mismatch: %s", screen.Name)
		}
		out = append(out, map[string]any{
			"name": screen.Name, "sha256": screen.SHA256, "bytes": screen.Bytes, "width": screen.Width, "height": screen.Height, "blank": screen.Blank,
			"file": SelfDevelopmentScreensDir(operationID) + "/" + screen.Name, "image_ref": "artifact:sha256:" + screen.SHA256,
		})
	}
	return out, nil
}

// SelfDevelopmentScreensDir is where the runtime saves an operation's screens
// in the computer's file tree, relative to the files root.
func SelfDevelopmentScreensDir(operationID string) string {
	return "Self-development/" + strings.TrimSpace(operationID) + "/screens"
}
