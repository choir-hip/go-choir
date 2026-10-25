package capsule

// A capsule cannot delete a platform file
// (docs/definitions/choir-selfdev-readiness-metamission-2026-10-24.ledger.md,
// entry 22:4x; operation selfdev-48ce01b9 failed before freeze with
// "capsule source patch reads workspace/platform/frontend/src/lib/EpubApp.svelte:
// no such file or directory").
//
// Ways this can fail, written before the fix:
//
//  1. A lower-layer file the capsule deletes leaves an overlay whiteout in
//     the upper: a character device 0:0. The walk records it as a "device"
//     and diffManifests reports it ChangeAdded, so emitSourcePatch reads a
//     merged path that does not exist and the freeze fails.
//  2. A whiteout reported as ChangeModified (the last commit held the
//     path as a copied-up file) is the same bug in another spelling: the
//     path is deleted, never added or modified.
//  3. Repeated snapshots: a whiteout present in both the last and the
//     current manifest is not a new change (no phantom deletion each
//     freeze); a whiteout that disappears from the manifest (nothing in
//     an overlay does this, but a manifest is data) is not a deletion
//     either.
//  4. A whiteout replaced by a real file is the file's change (added or
//     modified), never a deletion; replaced by a directory the directory
//     is the change and the dir carries its opaque marker.
//  5. A file the capsule created and then deleted was upper-only: no
//     whiteout, no manifest entry, no source change at all.
//  6. A whiteout over a lower DIRECTORY deletes every file under it; an
//     opaque directory (a lower directory removed and recreated) deletes
//     every lower entry the upper does not hold; a lower directory
//     replaced by a file deletes the files under it. Reading these as
//     only "a directory changed" ships a patch that leaves the files.
//  7. A deleted file renders as a proper deletion (`deleted file mode`,
//     `+++ /dev/null`), git apply removes it, an executable lower file is
//     no exception, and a deleted symlink (which the builder refuses to
//     patch) is refused at freeze with a named reason rather than
//     mis-rendered.
//  8. Opaque scaffolding the spawn itself creates (the release dir) must
//     stay out of the change list; expansion is limited to the pinned
//     workspace/platform source and never touches the release tree.
//  9. (bunch221 review) A marker on a strict ancestor of the pinned source
//     (a whiteout at workspace, an opaque workspace, workspace replaced by
//     a file) hides the whole workspace/platform tree; skipping it freezes
//     a patch that does not reproduce the candidate.
// 10. (bunch221 review) A whiteout is the walk's fact (Type "whiteout"),
//     carried on the change; a char-device mode alone is not one.
// 11. (bunch221 review) The kernel's other whiteout spelling, an empty
//     regular file carrying user.overlay.whiteout, must read as a whiteout
//     too, or the deletion returns as an added empty file.

import (
	"os"
	"testing"
)

const whiteoutMode = uint32(os.ModeDevice | os.ModeCharDevice | 0o000)

func whiteoutEntry(path string) FileManifest {
	return FileManifest{Path: path, Type: "whiteout", Mode: whiteoutMode}
}

func fileEntry(path, hash string) FileManifest {
	return FileManifest{Path: path, Type: "file", Mode: 0o644, Size: 3, Hash: hash}
}

func kindsByPath(changes []FileChange) map[string]ChangeKind {
	out := map[string]ChangeKind{}
	for _, c := range changes {
		out[c.Path] = c.Kind
	}
	return out
}

func TestDiffManifestsWhiteoutIsDeletedNeverAdded(t *testing.T) {
	current := []FileManifest{whiteoutEntry("workspace/platform/frontend/src/lib/EpubApp.svelte")}
	changes := diffManifests(nil, current)
	if len(changes) != 1 || changes[0].Kind != ChangeDeleted || !changes[0].Whiteout {
		t.Fatalf("a whiteout diffs as %+v, want one ChangeDeleted carrying Whiteout (failure modes 1, 10)", changes)
	}
}

func TestDiffManifestsWhiteoutOverCommittedUpperFileIsOneDeletion(t *testing.T) {
	path := "workspace/platform/a.go"
	changes := diffManifests([]FileManifest{fileEntry(path, "aa")}, []FileManifest{whiteoutEntry(path)})
	if len(changes) != 1 || changes[0].Kind != ChangeDeleted {
		t.Fatalf("copied-up file then deleted diffs as %+v, want one ChangeDeleted (failure mode 2)", changes)
	}
}

func TestDiffManifestsRepeatedWhiteoutIsNotANewChange(t *testing.T) {
	path := "workspace/platform/a.go"
	same := []FileManifest{whiteoutEntry(path)}
	if changes := diffManifests(same, same); len(changes) != 0 {
		t.Fatalf("a whiteout already committed diffs as %+v (failure mode 3)", changes)
	}
	if changes := diffManifests(same, nil); len(changes) != 0 {
		t.Fatalf("a vanished committed whiteout diffs as %+v, want nothing (failure mode 3)", changes)
	}
}

func TestDiffManifestsWhiteoutReplacedByFileIsTheFilesChange(t *testing.T) {
	path := "workspace/platform/a.go"
	changes := diffManifests([]FileManifest{whiteoutEntry(path)}, []FileManifest{fileEntry(path, "bb")})
	if len(changes) != 1 || changes[0].Kind != ChangeModified {
		t.Fatalf("whiteout replaced by a file diffs as %+v, want one ChangeModified (failure mode 4)", changes)
	}
	changes = diffManifests(nil, []FileManifest{fileEntry(path, "bb")})
	if len(changes) != 1 || changes[0].Kind != ChangeAdded {
		t.Fatalf("a new file diffs as %+v, want one ChangeAdded", changes)
	}
}

func TestDiffManifestsCreatedThenDeletedUpperFileIsNoChange(t *testing.T) {
	// Upper-only file deleted again: the overlay leaves nothing behind, so
	// the current manifest has no entry for it.
	if changes := diffManifests(nil, nil); len(changes) != 0 {
		t.Fatalf("create-then-delete diffs as %+v (failure mode 5)", changes)
	}
}

func TestDiffManifestsCarriesOpaqueOnDirectories(t *testing.T) {
	dir := FileManifest{Path: "workspace/platform/d", Type: "dir", Mode: uint32(os.ModeDir | 0o755), Opaque: true}
	changes := diffManifests(nil, []FileManifest{dir})
	if len(changes) != 1 || !changes[0].Opaque || changes[0].Kind != ChangeAdded {
		t.Fatalf("opaque dir diffs as %+v, want one added change carrying Opaque (failure mode 6)", changes)
	}
	plain := dir
	plain.Opaque = false
	changes = diffManifests([]FileManifest{plain}, []FileManifest{dir})
	if len(changes) != 1 || !changes[0].Opaque || changes[0].Kind != ChangeModified {
		t.Fatalf("a directory that became opaque diffs as %+v, want one modified change carrying Opaque", changes)
	}
	if changes := diffManifests([]FileManifest{dir}, []FileManifest{dir}); len(changes) != 0 {
		t.Fatalf("an opaque dir already committed diffs as %+v", changes)
	}
}
