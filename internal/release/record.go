// Package release is the software release record (P7,
// docs/archive/reports/p6-p8-release-adopt-2026-10-15.md): a content-addressed
// statement of one applied self-development change as source, published to
// the Autopaper object graph (Store B) by its owner from the computer that
// made it, and adopted by other computers as source onto their own trees.
//
// Owner rulings (2026-10-14): only source is ever published; the host is the
// sole builder, so a record may name the host's build digest but never
// carries executable bytes; app packages (S7) come later, so Packages is
// reserved and must be empty.
package release

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// Schema is the record's schema id.
const Schema = "choir.software_release.v1"

// ObjectKind is the Store B object kind a published record is written as.
const ObjectKind = "choir.software_release"

// MaxPatchBytes is the largest source patch a release carries: the patch
// export cap (internal/agentcore/api_self_development_patch.go).
const MaxPatchBytes = 2 << 20

// MaxRecordBytes bounds a record's canonical JSON.
const MaxRecordBytes = 256 << 10

// Typed refusals. Each names what the verifier found, so a caller can map
// it to a reason code without parsing prose.
var (
	ErrInvalid        = errors.New("release record invalid")
	ErrDigestMismatch = errors.New("release record does not hash to its release id")
	ErrPatchMismatch  = errors.New("release patch bytes do not hash to the record's patch_sha256")
	ErrPatchTooLarge  = errors.New("release patch exceeds the 2 MiB cap")
)

// Record is one release. Its id is the sha256 of its canonical JSON
// (ReleaseID). It says nothing about which computers should run it.
type Record struct {
	Schema         string          `json:"schema"`
	Publisher      Publisher       `json:"publisher"`
	Operation      Operation       `json:"operation"`
	Source         Source          `json:"source"`
	Precommitments []Precommitment `json:"precommitments"`
	// HostBuild is the host builder's digests for the publisher's own tree,
	// present when the applied release was layered (P5). Never bytes.
	HostBuild    *HostBuild   `json:"host_build,omitempty"`
	Declarations Declarations `json:"declarations"`
}

// Publisher is the computer that made and applied the change, and its owner.
type Publisher struct {
	ComputerID string `json:"computer_id"`
	OwnerID    string `json:"owner_id"`
}

// Operation is the publisher's applied self-development operation and its
// receipts on the publisher's tape.
type Operation struct {
	OperationID            string   `json:"operation_id"`
	BundleDigest           string   `json:"bundle_digest"`
	ReleaseDigest          string   `json:"release_digest"`
	CodeRef                string   `json:"code_ref"`
	DecisionEvent          string   `json:"decision_event"`
	MaterializationReceipt string   `json:"materialization_receipt,omitempty"`
	CheckpointRef          string   `json:"checkpoint_ref,omitempty"`
	RouteCertificate       string   `json:"route_certificate,omitempty"`
	VerifierRefs           []string `json:"verifier_refs"`
}

// Source is the change as source: a base commit plus a git-applicable patch
// whose bytes live in the host content store under its sha256.
type Source struct {
	BaseCommit   string   `json:"base_commit"`
	PatchSHA256  string   `json:"patch_sha256"`
	PatchBytes   int      `json:"patch_bytes"`
	TreeHash     string   `json:"tree_hash"`
	ChangedPaths []string `json:"changed_paths"`
}

// Precommitment is one of engineering's rule-settled say-do claims for the
// assignment that froze the bundle (types/commitment_rules.go). PYesPermille
// is p(yes) in thousandths: canonical JSON forbids non-integral numbers.
type Precommitment struct {
	RecordID        string `json:"record_id"`
	Kind            string `json:"kind"`
	Question        string `json:"question"`
	PYesPermille    int    `json:"p_yes_permille"`
	Settled         string `json:"settled,omitempty"`
	ResolveRecordID string `json:"resolve_record_id,omitempty"`
}

// HostBuild names the host build of the publisher's tree (P5 seam).
type HostBuild struct {
	BuilderReceiptDigest    string `json:"builder_receipt_digest"`
	ClosureDigest           string `json:"closure_digest"`
	CodeCommit              string `json:"code_commit"`
	BaseImageManifestDigest string `json:"base_image_manifest_digest,omitempty"`
}

// Declarations are what the publisher declares about the release.
type Declarations struct {
	// TestCommand is the check an adopter runs in its capsule after applying
	// the patch, declared by the owner at publish. Empty means none declared.
	TestCommand string `json:"test_command,omitempty"`
	// Packages is reserved for app packages (S7) and must be empty.
	Packages []string `json:"packages"`
}

var (
	gitObjectID  = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
	safeIdentity = regexp.MustCompile(`^[A-Za-z0-9._:@+-]{1,200}$`)
)

// Normalize fills nil slices so a record has one canonical form, and sorts
// ChangedPaths.
func (r Record) Normalize() Record {
	if r.Precommitments == nil {
		r.Precommitments = []Precommitment{}
	}
	if r.Operation.VerifierRefs == nil {
		r.Operation.VerifierRefs = []string{}
	}
	if r.Source.ChangedPaths == nil {
		r.Source.ChangedPaths = []string{}
	}
	if r.Declarations.Packages == nil {
		r.Declarations.Packages = []string{}
	}
	paths := append([]string(nil), r.Source.ChangedPaths...)
	sort.Strings(paths)
	r.Source.ChangedPaths = paths
	return r
}

// Canonical returns the record's canonical JSON (normalized first).
func (r Record) Canonical() ([]byte, error) {
	raw, err := computerevent.CanonicalJSON(r.Normalize())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if len(raw) > MaxRecordBytes {
		return nil, fmt.Errorf("%w: canonical record exceeds %d bytes", ErrInvalid, MaxRecordBytes)
	}
	return raw, nil
}

// ReleaseID is the sha256 (lowercase hex) of the canonical record.
func (r Record) ReleaseID() (string, error) {
	raw, err := r.Canonical()
	if err != nil {
		return "", err
	}
	return computerevent.DigestBytes(raw), nil
}

// Validate checks the record's shape: every binding present and well formed,
// no reserved field used (failure modes R6, A1 in the design).
func (r Record) Validate() error {
	r = r.Normalize()
	fail := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
	}
	if r.Schema != Schema {
		return fail("schema must be %s", Schema)
	}
	if !safeIdentity.MatchString(r.Publisher.ComputerID) || !safeIdentity.MatchString(r.Publisher.OwnerID) {
		return fail("publisher computer_id and owner_id are required")
	}
	if strings.TrimSpace(r.Operation.OperationID) == "" || !computerevent.IsSHA256(r.Operation.BundleDigest) ||
		!computerevent.IsSHA256(r.Operation.ReleaseDigest) || strings.TrimSpace(r.Operation.CodeRef) == "" ||
		strings.TrimSpace(r.Operation.DecisionEvent) == "" {
		return fail("operation_id, bundle_digest, release_digest, code_ref and decision_event are required")
	}
	if !gitObjectID.MatchString(r.Source.BaseCommit) || !gitObjectID.MatchString(r.Source.TreeHash) {
		return fail("source base_commit and tree_hash must be git object ids")
	}
	if !computerevent.IsSHA256(r.Source.PatchSHA256) {
		return fail("source patch_sha256 must be a sha256")
	}
	if r.Source.PatchBytes <= 0 || r.Source.PatchBytes > MaxPatchBytes {
		return fail("source patch_bytes must be in (0, %d]", MaxPatchBytes)
	}
	if len(r.Source.ChangedPaths) == 0 {
		return fail("source changed_paths is required")
	}
	for _, path := range r.Source.ChangedPaths {
		if !safeRepoPath(path) {
			return fail("changed path %q is not a clean repository-relative path", path)
		}
	}
	if len(r.Declarations.Packages) != 0 {
		return fail("packages is reserved for app packages (S7) and must be empty")
	}
	if strings.ContainsAny(r.Declarations.TestCommand, "\x00\n\r") || len(r.Declarations.TestCommand) > 1024 {
		return fail("test_command must be one line of at most 1024 bytes")
	}
	if h := r.HostBuild; h != nil {
		if !computerevent.IsSHA256(h.BuilderReceiptDigest) || !computerevent.IsSHA256(h.ClosureDigest) || !gitObjectID.MatchString(h.CodeCommit) {
			return fail("host_build names a builder receipt digest, a closure digest and a code commit")
		}
		if h.BaseImageManifestDigest != "" && !computerevent.IsSHA256(h.BaseImageManifestDigest) {
			return fail("host_build base_image_manifest_digest must be a sha256")
		}
	}
	return nil
}

func safeRepoPath(path string) bool {
	if path == "" || strings.HasPrefix(path, "/") || strings.Contains(path, "\\") || strings.ContainsAny(path, "\x00\n") {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

// Decode reads exactly one record, refusing unknown fields and trailing
// content.
func Decode(raw []byte) (Record, error) {
	var record Record
	if len(raw) == 0 || len(raw) > MaxRecordBytes {
		return Record{}, fmt.Errorf("%w: record is empty or too large", ErrInvalid)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return Record{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if decoder.More() {
		return Record{}, fmt.Errorf("%w: trailing content", ErrInvalid)
	}
	return record.Normalize(), nil
}

// VerifyID validates the record and requires its canonical digest to equal
// releaseID (A1, R1).
func VerifyID(record Record, releaseID string) error {
	if err := record.Validate(); err != nil {
		return err
	}
	releaseID = strings.ToLower(strings.TrimSpace(releaseID))
	if !computerevent.IsSHA256(releaseID) {
		return fmt.Errorf("%w: release id must be a sha256", ErrInvalid)
	}
	got, err := record.ReleaseID()
	if err != nil {
		return err
	}
	if got != releaseID {
		return fmt.Errorf("%w: canonical digest %s, release id %s", ErrDigestMismatch, got, releaseID)
	}
	return nil
}

// VerifyPatch requires the patch bytes to fit the cap and hash to the
// record's patch_sha256 with the record's byte count (A4, R2).
func VerifyPatch(record Record, patch []byte) error {
	if len(patch) > MaxPatchBytes {
		return ErrPatchTooLarge
	}
	if computerevent.DigestBytes(patch) != record.Source.PatchSHA256 || len(patch) != record.Source.PatchBytes {
		return ErrPatchMismatch
	}
	return nil
}

// Same reports whether two records have the same canonical bytes.
func Same(a, b Record) bool {
	left, errA := a.Canonical()
	right, errB := b.Canonical()
	return errA == nil && errB == nil && bytes.Equal(left, right)
}
