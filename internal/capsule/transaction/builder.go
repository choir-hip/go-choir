package transaction

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// TransactionBuilder classifies capsule diffs into immutable effect bundles.
// Durable semantic ordering belongs to ComputerEventAppender, never a process-local tape.
type TransactionBuilder struct {
	classifier *Classifier
}

// NewTransactionBuilder creates a new transaction builder with the given classifier.
func NewTransactionBuilder(classifier *Classifier) *TransactionBuilder {
	return &TransactionBuilder{classifier: classifier}
}

// CapsuleEffectBundle is the complete immutable proposal envelope. ContentDigest
// commits the effect payload with VerifierReceipts cleared; detached verifier
// receipts sign that digest, and the final bundle digest commits both.
type CapsuleEffectBundle struct {
	BundleVersion           int                         `json:"bundle_version"`
	ComputerID              string                      `json:"computer_id"`
	BaseEventHead           string                      `json:"base_event_head"`
	TrajectoryRef           string                      `json:"trajectory_ref"`
	CapsuleIdentity         string                      `json:"capsule_identity"`
	CapabilityPolicyDigest  string                      `json:"capability_policy_digest"`
	SourceTreeRef           string                      `json:"source_tree_ref"`
	OrderedFileEffects      []ChangeRecord              `json:"ordered_file_effects"`
	GeneratedArtifactRefs   []string                    `json:"generated_artifact_refs"`
	BuildRecipeRef          string                      `json:"build_recipe_ref"`
	RuntimeArtifactRef      string                      `json:"runtime_artifact_ref"`
	TestReceipts            []string                    `json:"test_receipts"`
	VerifierReceipts        []string                    `json:"verifier_receipts"`
	DependencyToolchainRefs []string                    `json:"dependency_toolchain_refs"`
	ResourceReceipts        []string                    `json:"resource_receipts"`
	ContentDigest           string                      `json:"content_digest"`
	ClassifierV             string                      `json:"classifier_version"`
	ClassifierDigest        string                      `json:"classifier_digest"`
	Groups                  map[string][]ChangeRecord   `json:"groups"`
	Ignored                 []ChangeRecord              `json:"ignored"`
	Unknown                 []ChangeRecord              `json:"unknown,omitempty"`
	Rejected                bool                        `json:"rejected"`
	RejectReason            string                      `json:"reject_reason,omitempty"`
	RuntimeFiles            []capsule.FrozenReleaseFile `json:"runtime_files"`
}

// ChangeRecord is a single file change in the transaction record.
type ChangeRecord struct {
	Path string `json:"path"`
	Kind string `json:"kind"` // "added", "modified", "deleted"
	Mode uint32 `json:"mode"`
}

// BuildBundleFromDiff classifies and deterministically orders one capsule diff.
func (b *TransactionBuilder) BuildBundleFromDiff(capsuleID string, changes []capsule.FileChange) (*CapsuleEffectBundle, error) {
	result := b.classifier.Classify(changes)
	record := &CapsuleEffectBundle{
		BundleVersion: 1, CapsuleIdentity: capsuleID,
		ClassifierV: result.Version, ClassifierDigest: result.Digest,
		Groups: make(map[string][]ChangeRecord), Ignored: toChangeRecords(result.Ignored),
		Unknown: toChangeRecords(result.Unknown), OrderedFileEffects: toChangeRecords(changes),
	}
	sort.Slice(record.OrderedFileEffects, func(i, j int) bool {
		if record.OrderedFileEffects[i].Path == record.OrderedFileEffects[j].Path {
			return record.OrderedFileEffects[i].Kind < record.OrderedFileEffects[j].Kind
		}
		return record.OrderedFileEffects[i].Path < record.OrderedFileEffects[j].Path
	})
	for kind, groupChanges := range result.Groups {
		record.Groups[kind.String()] = toChangeRecords(groupChanges)
	}
	if result.HasUnknown() {
		record.Rejected = true
		record.RejectReason = fmt.Sprintf("unknown paths rejected at commit time: %d paths", len(result.Unknown))
	}
	return record, nil
}

func (r CapsuleEffectBundle) ComputeContentDigest() (string, error) {
	r.ContentDigest = ""
	r.VerifierReceipts = []string{}
	raw, err := computerevent.CanonicalJSON(r)
	if err != nil {
		return "", err
	}
	return computerevent.DigestBytes(raw), nil
}

func (r CapsuleEffectBundle) Validate(final bool) error {
	contentDigest, err := r.ComputeContentDigest()
	if err != nil || r.BundleVersion != 1 || strings.TrimSpace(r.ComputerID) == "" ||
		!computerevent.IsSHA256(r.BaseEventHead) || strings.TrimSpace(r.TrajectoryRef) == "" ||
		strings.TrimSpace(r.CapsuleIdentity) == "" || !computerevent.IsSHA256(r.CapabilityPolicyDigest) ||
		!strings.HasPrefix(r.SourceTreeRef, "source-tree:sha256:") || !computerevent.IsSHA256(strings.TrimPrefix(r.SourceTreeRef, "source-tree:sha256:")) ||
		!strings.HasPrefix(r.RuntimeArtifactRef, "runtime-artifact:sha256:") || !computerevent.IsSHA256(strings.TrimPrefix(r.RuntimeArtifactRef, "runtime-artifact:sha256:")) ||
		!validExecutionRef(r.BuildRecipeRef) || len(r.OrderedFileEffects) == 0 || !validImmutableRefs(r.GeneratedArtifactRefs) ||
		!validExecutionRefs(r.TestReceipts) || !validExecutionRefs(r.DependencyToolchainRefs) || !validImmutableRefs(r.ResourceReceipts) ||
		len(r.RuntimeFiles) == 0 || r.Rejected || r.ContentDigest != contentDigest {
		return fmt.Errorf("capsule effect bundle: complete immutable effect bindings are required")
	}
	if final && !validImmutableRefs(r.VerifierReceipts) {
		return fmt.Errorf("capsule effect bundle: independent verifier receipt is required")
	}
	return nil
}

func validExecutionRefs(refs []string) bool {
	if len(refs) == 0 {
		return false
	}
	for _, ref := range refs {
		if !validExecutionRef(ref) {
			return false
		}
	}
	return true
}

func validExecutionRef(ref string) bool {
	return strings.HasPrefix(ref, "capsule-exec:sha256:") &&
		computerevent.IsSHA256(strings.TrimPrefix(ref, "capsule-exec:sha256:"))
}

func validImmutableRefs(refs []string) bool {
	if len(refs) == 0 {
		return false
	}
	for _, ref := range refs {
		if !validImmutableRef(ref) {
			return false
		}
	}
	return true
}

func validImmutableRef(ref string) bool {
	ref = strings.TrimSpace(ref)
	if computerevent.IsSHA256(ref) {
		return true
	}
	separator := strings.LastIndexByte(ref, ':')
	return separator > 0 && computerevent.IsSHA256(ref[separator+1:])
}

// toChangeRecords converts a slice of capsule.FileChange to ChangeRecord.
func toChangeRecords(changes []capsule.FileChange) []ChangeRecord {
	if len(changes) == 0 {
		return nil
	}
	records := make([]ChangeRecord, len(changes))
	for i, change := range changes {
		records[i] = ChangeRecord{
			Path: change.Path,
			Kind: change.Kind.String(),
			Mode: uint32(change.Mode),
		}
	}
	return records
}
