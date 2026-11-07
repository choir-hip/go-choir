package pkgrelease

import (
	"sort"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// DraftFacts is the content of a drafted package release that a package
// decision binds (docs/design/package-operation-decision-2026-11-06.md 2.2):
// the package name and root, the subtree id, the changed paths, the manifest
// summary and the parent. Provenance a draft only claims (BuiltOn) and the
// operation's own receipts are not content, so the seat that derives the
// draft from the frozen patch and the publisher that posts the record
// compute the same digest.
type DraftFacts struct {
	Name            string   `json:"name"`
	Root            string   `json:"root"`
	Tree            string   `json:"tree"`
	ChangedPaths    []string `json:"changed_paths"`
	Manifest        Manifest `json:"manifest"`
	ManifestVersion string   `json:"manifest_version"`
	Parent          Parent   `json:"parent"`
}

// Facts is the content of a draft of the package name.
func (d DraftSource) Facts(name string) DraftFacts {
	return DraftFacts{Name: name, Root: d.Root, Tree: d.Tree, ChangedPaths: d.Paths, Manifest: d.Manifest, ManifestVersion: d.ManifestVersion, Parent: d.Parent}
}

// DraftFacts is the content of a record, in the form DraftSource.Facts gives.
func (r Record) DraftFacts() DraftFacts {
	r = r.Normalize()
	return DraftFacts{
		Name: r.Package.Name, Root: r.Source.Root, Tree: r.Source.Tree, ChangedPaths: r.Source.ChangedPaths,
		Manifest: r.Manifest, ManifestVersion: r.Package.ManifestVersion, Parent: r.Parent,
	}
}

// Digest is the draft content digest: the sha256 of the canonical JSON of the
// facts, changed paths sorted.
func (f DraftFacts) Digest() (string, error) {
	f.ChangedPaths = append([]string{}, f.ChangedPaths...)
	sort.Strings(f.ChangedPaths)
	raw, err := computerevent.CanonicalJSON(f)
	if err != nil {
		return "", refuse(ReasonInvalid, "%v", err)
	}
	return computerevent.DigestBytes(raw), nil
}
