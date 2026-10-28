# Capture cleanup compares files by inode, and inodes are reused (2026-10-28)

Mutation class of the fix: red (vmctl), tier 1.

## Evidence

CI run 40644038924 (main b224f7e38, 06:30 UTC) failed
`TestAbandonedCloneCleanupRemovesOnlyItsOwnFiles` in `internal/vmmanager`:
`capture-pause.json after the abandoned cleanup = ""`, want "newer". The
same test passed on earlier runs, so it is timing- and filesystem-dependent.

## Cause (from code)

`removeIfSame` (internal/vmmanager/data_image_capture.go) keeps a newer
capture or pause marker by `os.SameFile`, which compares device and inode
number. The abandoned-clone cleanup records the marker's identity with an
Lstat and holds no descriptor on it, and `discard` closes the clone's
descriptor before comparing. Once a file is unlinked its inode number is
free, so a file written in its place can get the same number: the
comparison then says "same file" and the cleanup deletes the newer one.

In production this needs a recovery and a new capture between a clone's
abandonment and its late return, so it is rare, but the result is a lost
pause marker (the durable record of a paused VM).

## Fix direction

Keep a descriptor open on each recorded file until after the comparison
(an open descriptor pins the inode, so its number cannot be reused): compare
before `dst.Close()` in `discard`, and hold the marker open from write to
cleanup.
