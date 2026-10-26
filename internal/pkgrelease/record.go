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
	"errors"

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

var errNotBuilt = errors.New("pkgrelease: not built")

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

// Normalize fills nil slices so a record has one canonical form, and sorts
// ChangedPaths.
func (r Record) Normalize() Record { return r }

// Canonical returns the record's canonical JSON (normalized first).
func (r Record) Canonical() ([]byte, error) { return nil, errNotBuilt }

// ReleaseID is the sha256 (lowercase hex) of the canonical record.
func (r Record) ReleaseID() (string, error) { return "", errNotBuilt }

// Validate checks the record's shape.
func (r Record) Validate() error { return errNotBuilt }

// Decode reads exactly one record, refusing unknown fields and trailing
// content.
func Decode(raw []byte) (Record, error) { return Record{}, errNotBuilt }

// VerifyID validates the record and requires its canonical digest to equal
// releaseID.
func VerifyID(record Record, releaseID string) error { return errNotBuilt }

// VerifyBlob requires raw to fit the cap and hash to blob.
func VerifyBlob(blob Blob, raw []byte) error { return errNotBuilt }
