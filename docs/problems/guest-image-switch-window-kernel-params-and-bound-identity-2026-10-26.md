# Between a NixOS switch and the vmctl restart, a cold boot or a snapshot wake can pair mismatched guest bytes (H1, H6; 2026-10-26)

Status: problem record, written before the S1b tests and code. Found by
review 306 (Devin F1, DeepSeek finding 7) of the D2 design
([density-d2-snapshots-survive-deploys-2026-10-26.md](../design/density-d2-snapshots-survive-deploys-2026-10-26.md)
§11 H1 and H6, §4.4, M11 and M20). Mutation class of this document: green.
The repair (slice S1b) is red, switch-independent, and lands with D2 off.
Heresy delta: discovered H1, H6; introduced none; repaired none yet.

## The window

The NixOS activation script `go-choir-guest-image` repoints the symlink
`/var/lib/go-choir/guest` at the new guest image (nix/node-b.nix:2965). The
deploy restarts `go-choir-vmctl` afterwards (`.github/scripts/node-b-ci-deploy`
:508-516). Between the two, which lasts seconds to minutes, the running vmctl
holds config read at its own start and the link already names the new image.

## H1: kernel parameters are read once, at vmctl start

`internal/vmmanager/config.go:47-49` reads `VM_KERNEL_PARAMS_FILE` once, into
`m.cfg.KernelParams`. `applyLaunchDefaults` (manager.go:1232-1257) fills a
launch from `m.cfg`, but the kernel, initrd, rootfs and store disk are paths
through the stable link, so they name the new image's files at launch. In the
window a cold boot therefore pairs the new kernel and store disk with the old
`init=<store path>` argument, which is absent from the new erofs. The code
path is certain. That it has caused failed boots is a hypothesis: confirm by
matching boot failures in the vmctl journal against the switch-to-restart
window of each deploy.

## H6: a snapshot can load against store-disk bytes it was not taken against

A guest launched before the switch, hibernated and woken inside the window,
passes every check:

- the launch hash is rebuilt by `expectedLaunchConfig` from the same stale
  `m.cfg.KernelParams` and the same link paths, so it matches;
- `diskIdentity` stats the configured path at hibernate (snapshot.go:435-446,
  sampled in `commitSnapshot` after Firecracker stops) and again at wake; the
  path is the stable link, so both times it names the new erofs, and the
  identity matches.

Guest memory then loads against store-disk bytes it was never taken against.
That corrupts the computer without an error. The comment on
`snapshotBoundPaths` (snapshot.go:448-454) says this is guarded; it guards
only a replacement between hibernate and wake, not between launch and
hibernate. Verified for direct launches; the jailed path was not traced, and
the S1b guard does not depend on it, because it compares link targets, not
file identities. Whether it has happened is unobserved.

## Repair (S1b), independent of the D2 switch

1. `applyLaunchDefaults` and `expectedLaunchConfig` read the kernel-params
   file at each launch (`m.kernelParams()`), falling back to the cached value
   when the file is unset or unreadable.
2. Every launch writes `<vm>/guest-release.json` with `image_target`, the
   `readlink` of `VM_GUEST_RELEASE_LINK` taken once at launch.
3. `commitSnapshot` copies it into `SnapshotMeta.LaunchImageTarget`;
   `validateSnapshot` refuses a legacy meta whose recorded target differs from
   the link's target now, or whose link cannot be read, with
   `image_moved_since_launch`. A meta without the field validates as today.
4. Outside the window, the Firecracker config and the launch hash are
   byte-identical to today's, so the repair voids no snapshot that today's
   code would keep.

## Failure modes

- A guest booted in the window with the old `init=` path (H1): pinned by
  the read-at-each-launch test and by the fallback test (unreadable file keeps
  the cached value).
- A snapshot loaded against the wrong erofs (H6): pinned by launch with
  target A, repoint to B, hibernate, wake: refused `image_moved_since_launch`;
  and an unreadable link at wake refuses the same way (review 313 F4).
- A repair that voids good snapshots: pinned by the golden test (S1b test 4)
  and the old-meta test (S1b test 3).

## Protected surfaces, evidence, rollback

Surfaces: `internal/vmmanager` launch and snapshot validation. Evidence: unit
tests with a temp link directory; staging reading is FP0's. Rollback: git
revert; a reverted vmctl ignores the new meta field.
