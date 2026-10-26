// Package pkgrelease is the package release of the Choir Library
// (docs/design/library-package-propagation-2026-10-26.md, section 3): one
// app package's own change, a content-addressed record of the package
// directory frontend/src/packages/<name>/ taken as a whole git tree, with
// the tree it replaced, the review diff, the manifest, the base it was built
// on, the publisher's receipts and settled precommitments.
//
// The record carries no executable bytes and no evidence assets. Its three
// patches live in the host content store's package namespace, and corpusd
// runs the host checks of section 3.2 (VerifyHost) on the bytes it stores,
// so a release can never carry a path outside its package root.
package pkgrelease

import (
	"bytes"
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/release"
)

// Schema is the record's schema id.
const Schema = "choir.package_release.v1"

// ObjectKind is the Store B kind a published record is written as.
const ObjectKind = "choir.package_release"

// ContentNamespace is the host content store namespace of the patches. It
// is separate from the software release namespace, so that cap is
// untouched.
const ContentNamespace = "package"

// PackagesRoot is the directory every package lives in.
const PackagesRoot = "frontend/src/packages"

// MaxBlobBytes is each patch's cap: the software release cap, until S2's
// measurement of real packages says otherwise (L27).
const MaxBlobBytes = 2 << 20

// MaxRecordBytes bounds a record's canonical JSON.
const MaxRecordBytes = 256 << 10

// EmptyTree is git's empty tree (SHA-1 object format).
const EmptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// Parent kinds.
const (
	ParentBase    = "base"
	ParentRelease = "release"
)

// Record is one package release. Its id is the sha256 of its canonical
// JSON (ReleaseID).
type Record struct {
	Schema  string  `json:"schema"`
	Package Package `json:"package"`
	// Track is a publisher's line of the package: the publisher's owner id
	// (section 3.4).
	Track  string `json:"track"`
	Parent Parent `json:"parent"`
	// ParentPatch is the parent subtree for a base parent: the binary diff
	// from the empty tree to the full-path tree that holds only the root at
	// the parent. Absent for a release parent (corpusd holds that tree), and
	// for a new package, whose parent is the empty tree.
	ParentPatch    *Blob                   `json:"parent_patch,omitempty"`
	Source         Source                  `json:"source"`
	Manifest       Manifest                `json:"manifest"`
	Publisher      Publisher               `json:"publisher"`
	Operation      Operation               `json:"operation"`
	Precommitments []release.Precommitment `json:"precommitments"`
	Declarations   Declarations            `json:"declarations"`
}

// Package names the package. ManifestVersion is the manifest's semver,
// declared and never compared.
type Package struct {
	Name            string `json:"name"`
	ManifestVersion string `json:"manifest_version"`
}

// Parent is the tree this release replaced: the package subtree in a
// platform base commit, or a listed release on the same track.
type Parent struct {
	Kind      string `json:"kind"`
	Commit    string `json:"commit,omitempty"`
	ReleaseID string `json:"release_id,omitempty"`
	Tree      string `json:"tree"`
}

// Blob is one patch in the host content store, by sha256 and length.
type Blob struct {
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
}

// Source is the package subtree and its patches.
type Source struct {
	Root string `json:"root"`
	// Tree is the git tree id of the subtree at Root.
	Tree string `json:"tree"`
	// TreePatch is the binary diff from the empty tree to the full-path tree
	// holding only Root: the release content.
	TreePatch Blob `json:"tree_patch"`
	// ReviewPatch is the binary diff from the parent subtree to the subtree,
	// at full paths, for review.
	ReviewPatch Blob `json:"review_patch"`
	// ChangedPaths is the publisher's claim of the operation's paths
	// (publisher-side, section 3.2).
	ChangedPaths []string `json:"changed_paths"`
	// BuiltOn is the platform base commit the publisher built and verified
	// the release on.
	BuiltOn string `json:"built_on"`
}

// Manifest summarizes choir-package.json at the subtree; corpusd checks it
// against the bytes.
type Manifest struct {
	SHA256      string `json:"sha256"`
	ABI         string `json:"abi"`
	RequiresSDK string `json:"requires_sdk,omitempty"`
	KindsCount  int    `json:"kinds_count"`
	OpensCount  int    `json:"opens_count"`
}

// Publisher is the computer that made and applied the change, and its owner.
type Publisher struct {
	OwnerID    string `json:"owner_id"`
	ComputerID string `json:"computer_id"`
}

// Operation is the publisher's applied self-development operation and its
// receipts on the publisher's tape.
type Operation struct {
	OperationID            string   `json:"operation_id"`
	BundleDigest           string   `json:"bundle_digest"`
	DecisionEvent          string   `json:"decision_event"`
	MaterializationReceipt string   `json:"materialization_receipt,omitempty"`
	CheckpointRef          string   `json:"checkpoint_ref,omitempty"`
	VerifierRefs           []string `json:"verifier_refs"`
}

// Declarations are what the publisher declares about the release.
type Declarations struct {
	TestCommand string `json:"test_command,omitempty"`
}

// Root is a package's directory.
func Root(name string) string { return PackagesRoot + "/" + name }

var (
	gitObjectID     = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
	safeIdentity    = regexp.MustCompile(`^[A-Za-z0-9._:@+-]{1,200}$`)
	manifestVersion = regexp.MustCompile(`^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$`)
)

// Normalize fills nil slices so a record has one canonical form, and sorts
// ChangedPaths.
func (r Record) Normalize() Record {
	if r.Precommitments == nil {
		r.Precommitments = []release.Precommitment{}
	}
	if r.Operation.VerifierRefs == nil {
		r.Operation.VerifierRefs = []string{}
	}
	paths := append([]string{}, r.Source.ChangedPaths...)
	sort.Strings(paths)
	r.Source.ChangedPaths = paths
	return r
}

// Canonical returns the record's canonical JSON (normalized first).
func (r Record) Canonical() ([]byte, error) {
	raw, err := computerevent.CanonicalJSON(r.Normalize())
	if err != nil {
		return nil, refuse(ReasonInvalid, "%v", err)
	}
	if len(raw) > MaxRecordBytes {
		return nil, refuse(ReasonInvalid, "canonical record exceeds %d bytes", MaxRecordBytes)
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

// Validate checks the record's shape: every binding present and well
// formed, the root the package's, the track the publisher's, and the parent
// one of its two kinds.
func (r Record) Validate() error {
	r = r.Normalize()
	fail := func(format string, args ...any) error { return refuse(ReasonInvalid, format, args...) }
	if r.Schema != Schema {
		return fail("schema must be %s", Schema)
	}
	if !objectgraph.ValidPackageName(r.Package.Name) {
		return fail("package name %q is not a package name", r.Package.Name)
	}
	if !manifestVersion.MatchString(r.Package.ManifestVersion) {
		return fail("package manifest_version %q is not MAJOR.MINOR.PATCH", r.Package.ManifestVersion)
	}
	if !safeIdentity.MatchString(r.Publisher.OwnerID) || !safeIdentity.MatchString(r.Publisher.ComputerID) {
		return fail("publisher owner_id and computer_id are required")
	}
	// A track is one publisher's line (3.4); the choir track is main's and is
	// never published through the Library.
	if r.Track != r.Publisher.OwnerID {
		return fail("track %q is not the publisher's owner id", r.Track)
	}
	if !gitObjectID.MatchString(r.Parent.Tree) {
		return fail("parent tree must be a git object id")
	}
	switch r.Parent.Kind {
	case ParentBase:
		if !gitObjectID.MatchString(r.Parent.Commit) || r.Parent.ReleaseID != "" {
			return fail("a base parent names a commit and no release")
		}
		// A new package's parent is the empty tree and carries no patch.
		if (r.ParentPatch == nil) != (r.Parent.Tree == EmptyTree) {
			return fail("a base parent carries parent_patch unless its tree is the empty tree")
		}
	case ParentRelease:
		if !computerevent.IsSHA256(r.Parent.ReleaseID) || r.Parent.Commit != "" || r.ParentPatch != nil {
			return fail("a release parent names a release id, no commit and no parent_patch")
		}
	default:
		return fail("parent kind %q is not base or release", r.Parent.Kind)
	}
	if r.Source.Root != Root(r.Package.Name) {
		return fail("source root %q is not %s", r.Source.Root, Root(r.Package.Name))
	}
	if !gitObjectID.MatchString(r.Source.Tree) || !gitObjectID.MatchString(r.Source.BuiltOn) {
		return fail("source tree and built_on must be git object ids")
	}
	if r.Source.Tree == r.Parent.Tree {
		return refuse(ReasonUnchanged, "the package subtree %s is its parent's", r.Source.Tree)
	}
	blobs := []struct {
		name string
		blob Blob
	}{{"tree_patch", r.Source.TreePatch}, {"review_patch", r.Source.ReviewPatch}}
	if r.ParentPatch != nil {
		blobs = append(blobs, struct {
			name string
			blob Blob
		}{"parent_patch", *r.ParentPatch})
	}
	for _, b := range blobs {
		if err := validBlob(b.name, b.blob); err != nil {
			return err
		}
	}
	if len(r.Source.ChangedPaths) == 0 {
		return fail("source changed_paths is required")
	}
	for _, path := range r.Source.ChangedPaths {
		if !safeRepoPath(path) || !strings.HasPrefix(path, r.Source.Root+"/") {
			return fail("changed path %q is not a clean path under %s", path, r.Source.Root)
		}
	}
	if !computerevent.IsSHA256(r.Manifest.SHA256) || strings.TrimSpace(r.Manifest.ABI) == "" || r.Manifest.KindsCount < 0 || r.Manifest.OpensCount < 0 {
		return fail("manifest names a sha256, an abi and non-negative counts")
	}
	if strings.TrimSpace(r.Operation.OperationID) == "" || !computerevent.IsSHA256(r.Operation.BundleDigest) || strings.TrimSpace(r.Operation.DecisionEvent) == "" {
		return fail("operation_id, bundle_digest and decision_event are required")
	}
	if strings.ContainsAny(r.Declarations.TestCommand, "\x00\n\r") || len(r.Declarations.TestCommand) > 1024 {
		return fail("test_command must be one line of at most 1024 bytes")
	}
	return nil
}

func validBlob(name string, b Blob) error {
	if !computerevent.IsSHA256(b.SHA256) {
		return refuse(ReasonInvalid, "%s sha256 must be a sha256", name)
	}
	if b.Bytes > MaxBlobBytes {
		return refuse(ReasonBlobTooLarge, "%s is %d bytes; the cap is %d (2 MiB)", name, b.Bytes, MaxBlobBytes)
	}
	if b.Bytes <= 0 {
		return refuse(ReasonInvalid, "%s is empty", name)
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
		return Record{}, refuse(ReasonInvalid, "record is empty or too large")
	}
	if err := decodeStrict(raw, &record); err != nil {
		return Record{}, err
	}
	return record.Normalize(), nil
}

func decodeStrict(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return refuse(ReasonInvalid, "%v", err)
	}
	if decoder.More() {
		return refuse(ReasonInvalid, "trailing content")
	}
	return nil
}

// VerifyID validates the record and requires its canonical digest to equal
// releaseID.
func VerifyID(record Record, releaseID string) error {
	if err := record.Validate(); err != nil {
		return err
	}
	releaseID = strings.ToLower(strings.TrimSpace(releaseID))
	if !computerevent.IsSHA256(releaseID) {
		return refuse(ReasonInvalid, "release id must be a sha256")
	}
	got, err := record.ReleaseID()
	if err != nil {
		return err
	}
	if got != releaseID {
		return refuse(ReasonInvalid, "the record hashes to %s, not to the release id %s", got, releaseID)
	}
	return nil
}

// VerifyBlob requires raw to fit the cap and hash to blob with its length.
func VerifyBlob(blob Blob, raw []byte) error {
	if len(raw) > MaxBlobBytes {
		return refuse(ReasonBlobTooLarge, "the blob is %d bytes; the cap is %d (2 MiB)", len(raw), MaxBlobBytes)
	}
	if computerevent.DigestBytes(raw) != blob.SHA256 || len(raw) != blob.Bytes {
		return refuse(ReasonBlobMismatch, "the blob bytes do not hash to %s with %d bytes", blob.SHA256, blob.Bytes)
	}
	return nil
}
