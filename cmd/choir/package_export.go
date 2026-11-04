package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// choir package export (docs/design/library-package-propagation-2026-10-26.md
// 10.2): write one verified package release into a local git worktree as a
// commit carrying its provenance trailers, for the reviewed path to main.
// It pushes nothing.
//
// Ways it can fail, written before the code:
//
//	F1  wrong id or a record that does not verify: the host answers another
//	    release, or a record whose sha256 is not the id; refused before any
//	    git call (release_unverified);
//	F2  blob bytes that are not the record's (another blob, truncated), or a
//	    rebuilt subtree that is not record.source.tree: refused, nothing is
//	    written to the worktree (pkgrelease tree_mismatch / blob_mismatch);
//	F3  a path escape in a blob: a tree_patch entry outside the package root
//	    (../, another package), a symlink, submodule or .gitattributes:
//	    refused by the confinement check (content_not_confined);
//	F4  --onto has moved: its package subtree is not the release's parent
//	    subtree, so the export would silently revert a change main made
//	    after the release was cut (onto_moved);
//	F5  a package --onto does not hold (a new or third-party package)
//	    without --new-package (not_first_party), or a release manifest whose
//	    is not a valid first-party choir.package/v2 manifest, which main's build gate refuses
//	    (manifest_not_first_party);
//	F6  a dirty worktree, or one whose HEAD's package subtree differs from
//	    --onto's: the commit would carry or revert unrelated changes
//	    (worktree_dirty, head_not_onto);
//	F7  the commit lands but is not the release: its package subtree is not
//	    the release tree, or it touches a path outside the package: checked
//	    after commit and reported as a failure (scripts/check-package-export
//	    is the same check in CI).

type exportRefusal struct{ reason, detail string }

func (e *exportRefusal) Error() string { return e.reason + ": " + e.detail }

func refuseExport(reason, format string, args ...any) error {
	return &exportRefusal{reason: reason, detail: fmt.Sprintf(format, args...)}
}

func packageExportCommand() *command {
	return &command{
		path: []string{"package", "export"}, summary: "Write a verified package release into a local git worktree as one commit with its provenance trailers (pushes nothing)",
		desc: "Fetches the release record and its tree patch with your key, verifies both against the release id, refuses if --onto's package directory " +
			"is not the release's parent (onto_moved), replaces frontend/src/packages/<name>/ with the release tree verbatim, commits with the " +
			"Choir-Package-Release, Choir-Package-Track, Choir-Publisher-Computer, Choir-Publisher-Operation and Choir-Tree trailers, and prints the " +
			"commit and a PR body. --source-patch rebuilds the tree instead from the publisher's `choir self-dev patch` output applied to the record's built_on; " +
			"the tree id check against the record is the verification either way.",
		params: []param{
			pStr("release", "Package release id (the record's sha256)").req(),
			pStr("worktree", "The git worktree to commit in (clean)").req(),
			pStr("onto", "The ref the export is proposed onto; its package directory must be the release's parent (default HEAD)"),
			pBool("new_package", "Allow a package --onto does not hold (an owner's new app the owner chose to upstream)"),
			pStr("source_patch", "Rebuild from this verified self-dev source patch file instead of the catalog's tree patch"),
			pStr("message", "The commit's subject line (default names the release, mutation class and rollback)"),
			pList("trailer", "An extra commit trailer, \"Key: value\" (repeatable)"),
		},
		usage:   "--release and --worktree are required",
		scopes:  selfDevRead,
		cliOnly: "writes a commit into a local git worktree; a hosted MCP server has no local worktree",
		handler: packageExport,
	}
}

// fetchPackageRelease reads one release and verifies its record against
// the id (the check package show makes).
func fetchPackageRelease(x *call, releaseID string) (packageReleaseView, pkgrelease.Record, error) {
	var view packageReleaseView
	if !computerevent.IsSHA256(releaseID) {
		return view, pkgrelease.Record{}, refuseExport("release_unverified", "%q is not a release id (64 hex characters)", releaseID)
	}
	if err := x.c.do(http.MethodGet, "/api/catalog/releases/"+url.PathEscape(releaseID), nil, &view); err != nil {
		return view, pkgrelease.Record{}, err
	}
	record, err := pkgrelease.Decode(view.Record)
	if err == nil {
		err = pkgrelease.VerifyID(record, releaseID)
	}
	if err == nil && view.ReleaseID != releaseID {
		err = fmt.Errorf("the host answered release %q, not %q", view.ReleaseID, releaseID)
	}
	if err != nil {
		return view, pkgrelease.Record{}, fmt.Errorf("package release %s does not verify: %w", releaseID, err)
	}
	return view, record, nil
}

func packageExport(x *call) (any, error) {
	ctx := context.Background()
	releaseID := strings.ToLower(x.trim("release"))
	dir, err := filepath.Abs(x.trim("worktree"))
	if err != nil {
		return nil, err
	}
	g := worktreeGit{dir: dir}
	top, err := g.line(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("--worktree %s is not a git worktree: %w", dir, err)
	}
	if real, _ := filepath.EvalSymlinks(dir); real != "" {
		if realTop, _ := filepath.EvalSymlinks(top); realTop != real {
			return nil, fmt.Errorf("--worktree must be the worktree's top directory (%s)", top)
		}
	}
	if status, err := g.line(ctx, "status", "--porcelain", "--untracked-files=all"); err != nil || status != "" {
		return nil, refuseExport("worktree_dirty", "%s has uncommitted changes; commit or stash them first", dir)
	}
	head, err := g.line(ctx, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return nil, fmt.Errorf("the worktree has no HEAD commit: %w", err)
	}
	onto := x.trim("onto")
	if onto == "" {
		onto = "HEAD"
	}
	ontoCommit, err := g.line(ctx, "rev-parse", "--verify", onto+"^{commit}")
	if err != nil {
		return nil, fmt.Errorf("--onto %s is not a commit here: %w", onto, err)
	}

	view, record, err := fetchPackageRelease(x, releaseID)
	if err != nil {
		return nil, err
	}
	name, root := record.Package.Name, record.Source.Root
	if root != pkgrelease.Root(name) {
		return nil, refuseExport("release_unverified", "source.root %q is not %s", root, pkgrelease.Root(name))
	}

	// F4, F5, F6: the parent check, on --onto and on the commit's parent.
	ontoTree, ontoHas := g.subtree(ctx, ontoCommit, root)
	headTree, headHas := g.subtree(ctx, head, root)
	if ontoHas != headHas || ontoTree != headTree {
		return nil, refuseExport("head_not_onto", "HEAD %s holds %s at %s and --onto %s holds %s; check out --onto first", short(head), orNone(headTree), root, onto, orNone(ontoTree))
	}
	newPackage := !ontoHas
	switch {
	case ontoHas && ontoTree != record.Parent.Tree:
		return nil, refuseExport("onto_moved", "%s at %s is tree %s; release %s was cut from parent tree %s (%s). The publisher cuts a new release on the new base, or engineering merges the two",
			root, onto, ontoTree, short(releaseID), record.Parent.Tree, parentLabel(record.Parent))
	case newPackage && !x.flag("new_package"):
		return nil, refuseExport("not_first_party", "%s is not on %s, so %s is not a first-party package there; pass --new-package for an owner's new app the owner chose to upstream", root, onto, name)
	}

	// F1-F3: rebuild and verify the tree.
	source := "catalog_tree_patch"
	var tree string
	if path := x.trim("source_patch"); path != "" {
		source = "self_dev_source_patch"
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		tree, err = pkgrelease.ExportTreeFromSourcePatch(ctx, dir, record, raw)
		if err != nil {
			return nil, err
		}
	} else {
		raw, err := x.c.doRawBytes(http.MethodGet, "/api/catalog/releases/"+releaseID+"/blobs/"+record.Source.TreePatch.SHA256, nil)
		if err != nil {
			return nil, fmt.Errorf("fetch the tree patch: %w", err)
		}
		if tree, err = pkgrelease.ExportTreeFromBlob(ctx, dir, record, raw); err != nil {
			return nil, err
		}
	}
	if err := g.checkFirstPartyManifest(ctx, tree); err != nil {
		return nil, err
	}

	// Replace the package directory with the release tree and commit.
	message := packageExportMessage(x, releaseID, record)
	restore := func() { _, _ = g.run(ctx, nil, "reset", "-q", "--hard", head) }
	if _, err := g.run(ctx, nil, "rm", "-r", "-q", "--ignore-unmatch", "--", root); err != nil {
		restore()
		return nil, err
	}
	if _, err := g.run(ctx, nil, "read-tree", "--prefix="+root+"/", "-u", tree); err != nil {
		restore()
		return nil, err
	}
	if _, err := g.run(ctx, []byte(message), "commit", "-q", "-F", "-"); err != nil {
		restore()
		return nil, err
	}
	commit, err := g.line(ctx, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}

	// F7: the commit is the release and nothing else.
	got, has := g.subtree(ctx, commit, root)
	changed, _ := g.line(ctx, "diff-tree", "--no-commit-id", "--name-only", "-r", head, commit)
	for _, p := range strings.Fields(changed) {
		if !strings.HasPrefix(p, root+"/") {
			has = false
		}
	}
	if !has || got != record.Source.Tree {
		return nil, fmt.Errorf("commit %s does not hold exactly the release tree %s at %s (it holds %s, changes %q); reset it", short(commit), record.Source.Tree, root, orNone(got), changed)
	}
	return map[string]any{
		"commit": commit, "parent_commit": head, "onto": onto, "onto_commit": ontoCommit,
		"release_id": releaseID, "package": name, "track": record.Track, "tree": tree, "new_package": newPackage,
		"source": source, "record_verified": true, "tree_verified": true, "manifest_origin": apppackage.OriginFirstParty,
		"pr_body": packageExportPRBody(releaseID, record, view, onto, newPackage, source),
	}, nil
}

func packageExportMessage(x *call, releaseID string, record pkgrelease.Record) string {
	subject := x.trim("message")
	if subject == "" {
		subject = fmt.Sprintf("%s: export package release %s (track %s) verbatim (orange; rollback: revert)", record.Package.Name, short(releaseID), short(record.Track))
	}
	var b strings.Builder
	b.WriteString(subject + "\n\n")
	fmt.Fprintf(&b, "Choir-Package-Release: %s\n", releaseID)
	fmt.Fprintf(&b, "Choir-Package-Track: %s\n", record.Track)
	fmt.Fprintf(&b, "Choir-Publisher-Computer: %s\n", record.Publisher.ComputerID)
	fmt.Fprintf(&b, "Choir-Publisher-Operation: %s\n", record.Operation.OperationID)
	fmt.Fprintf(&b, "Choir-Tree: %s\n", record.Source.Tree)
	for _, t := range x.list("trailer") {
		if t = strings.TrimSpace(t); t != "" {
			b.WriteString(t + "\n")
		}
	}
	return b.String()
}

func packageExportPRBody(releaseID string, record pkgrelease.Record, view packageReleaseView, onto string, newPackage bool, source string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Exports package release `%s` of `%s` verbatim into `%s`.\n\n", releaseID, record.Package.Name, record.Source.Root)
	fmt.Fprintf(&b, "- Track `%s`, publisher computer `%s`, operation `%s`\n", record.Track, record.Publisher.ComputerID, record.Operation.OperationID)
	fmt.Fprintf(&b, "- Built on `%s`; parent %s\n", record.Source.BuiltOn, parentLabel(record.Parent))
	fmt.Fprintf(&b, "- Tree `%s` (Choir-Tree), rebuilt from the %s and checked against the record\n", record.Source.Tree, strings.ReplaceAll(source, "_", " "))
	if newPackage {
		fmt.Fprintf(&b, "- A new package on `%s` (--new-package)\n", onto)
	}
	fmt.Fprintf(&b, "- %d changed paths, review patch %d bytes, %d precommitments, %d media items\n",
		len(record.Source.ChangedPaths), record.Source.ReviewPatch.Bytes, len(record.Precommitments), len(record.Media))
	if view.PublishedAt != "" {
		fmt.Fprintf(&b, "- Published %s\n", view.PublishedAt)
	}
	b.WriteString("\nVerify with `scripts/check-package-export`.\n")
	return b.String()
}

func parentLabel(p pkgrelease.Parent) string {
	if p.Kind == pkgrelease.ParentRelease {
		return "release " + short(p.ReleaseID) + " (tree " + short(p.Tree) + ")"
	}
	if p.Tree == pkgrelease.EmptyTree {
		return "none (a new package at base " + short(p.Commit) + ")"
	}
	return "base " + short(p.Commit) + " (tree " + short(p.Tree) + ")"
}

func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

func orNone(tree string) string {
	if tree == "" {
		return "no directory"
	}
	return tree
}

// worktreeGit runs git in the worktree with the user's own config (the
// commit carries the user's identity).
type worktreeGit struct{ dir string }

func (g worktreeGit) run(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", g.dir}, args...)...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(stderr.String()+" "+err.Error()))
	}
	return out, nil
}

func (g worktreeGit) line(ctx context.Context, args ...string) (string, error) {
	out, err := g.run(ctx, nil, args...)
	return strings.TrimSpace(string(out)), err
}

// subtree is the tree at root in commit; has is false when there is no
// directory there.
func (g worktreeGit) subtree(ctx context.Context, commit, root string) (string, bool) {
	out, err := g.line(ctx, "rev-parse", "--verify", "-q", commit+":"+root)
	if err != nil || out == "" {
		return "", false
	}
	if kind, err := g.line(ctx, "cat-file", "-t", out); err != nil || kind != "tree" {
		return "", false
	}
	return out, true
}

// checkFirstPartyManifest requires the release's choir-package.json to be a
// valid choir.package/v2 manifest as main would build it: the origin is not a
// manifest field (a manifest that states one is refused as publisher-declared),
// and what main builds is first-party, so the manifest is validated as such.
func (g worktreeGit) checkFirstPartyManifest(ctx context.Context, tree string) error {
	raw, err := g.run(ctx, nil, "cat-file", "blob", tree+":choir-package.json")
	if err != nil {
		return refuseExport("manifest_not_first_party", "the release holds no choir-package.json")
	}
	if _, err := apppackage.ParseManifest(raw, apppackage.OriginFirstParty); err != nil {
		return refuseExport("manifest_not_first_party", "the release's choir-package.json is not a manifest main builds (frontend/scripts/check-packages.mjs): %v", err)
	}
	return nil
}
