# Capsules overlay the exact computer state; the source tree follows the deploy (2026-11-03)

Status: design, under review (tier 2). Mutation class of this doc: green.
The change it designs is red: capsule isolation, owner data in a capsule,
the guest store, source projection and the Cast gate. Problem:
`docs/problems/selfdev-arm-rounds-fail-at-freeze-and-capsules-lack-real-content-2026-11-03.md`
(problems 2 and 3, and its "Owner ruling" section).

Owner, 2026-11-03 ~04:25Z: "capsules should be an overlay over the exact
computer state", and "computer source tree should auto update."

Owner, 2026-11-03: "i hate the pattern of all these switches"; new
behaviour ships on, and rollout is the ring-ordered deploy (CLAUDE.md, "No
switches; deploys roll out by ring").

Code refs are at `02775597e`.

## What exists

- **The capsule root** is one overlayfs mount with lowers
  `sourceLower:/` (`internal/capsule/executor.go:658-666`). `sourceLower`
  holds `workspace/platform`, a digest-pinned copy of the committed HEAD of
  `/mnt/persistent/files/Source/platform` (`CHOIR_CAPSULE_SOURCE_ROOT`,
  executor.go:582-623). `/` is the guest root (`CHOIR_CAPSULE_LOWER_ROOT`).
  The upper is per capsule under `/mnt/persistent/capsule-state`.
- **Masks.** `prepareCapsuleRoot` mounts tmpfs over `/run`, `/mnt`, `/proc`
  and `/sys` (executor.go:2495-2510), binds a per-capsule `/tmp`, binds
  `/nix/store` and `/run/current-system` read-only, and writes its own
  `/etc/passwd`, `group`, `hosts`, `nsswitch.conf`. So `/mnt/persistent`
  (the files root, the store, `choir-credentials`, `choir-signers`,
  `choir-updater`) and `/run` (`choir-bootstrap` with the gateway token,
  `go-choir-autoputer.env`, the signer and verifier sockets) are absent.
  The applied release is hidden by an opaque capsule-owned
  `var/lib/artifact/release` in the upper (871c3c145,
  `capsule_release_dir.go`).
- **Network.** The capsule has its own network namespace (executor.go:856).
  Egress is only the per-capsule proxy on 127.0.0.1:3128. It refuses a host
  that resolves to any non-public address (`egress.go:403-412`); only the
  named host routes (`goproxy.choir.internal`, `mirror.choir.internal`)
  reach a private address. The guest's own services are unreachable.
- **Secret paths.** One predicate, `secretBearingPath`
  (`carried_patch.go:59`, 85250d067), refuses `.env*`, `.npmrc`, `.netrc`,
  `credentials.json`, `auth.json`, `id_rsa`, `id_ed25519` and `.pem`,
  `.key`, `.p12`, `.pfx` in the release and the carried patch.
  `secretBearingRefusal` (`secret_refusal.go`) refuses Exec and GoEval
  payloads with a secret shape.
- **What the patch carries.** Freeze reads the upper and keeps only
  `workspace/platform` (`manifest.go:89`) plus the screens leaf.
- **Source projection** (`internal/sourceproj`,
  `internal/agentcore/source_projection.go`) moves Source/platform HEAD to
  the booted base (empty stack) or to the applied stack's projection
  commit. It runs at boot (`runtime.go:1054`) and at every Cast
  (`sourceProjectionCastGateFor`). It is behind `sourceproj.Enabled`: the
  env `CHOIR_SOURCE_PROJECTION` or the setting `source.projection`, on only
  for b@c. Off, Source/platform stays at its creation commit (problem 3).
- **A moved base.** When the applied stack's base is not the booted base
  and the patch is not contained in it, the state is `stack_base_moved`
  and every ordinary Cast is refused (`sourceproj.go:330-341`,
  `Refusal()`). The only admitted Cast is a U4 rebase Cast, behind a second
  switch, `source.rebase_offer` (`source_projection.go:289-345`). The host
  builder needs a patch based on the booted base
  (`bootedBaseCommit`, source_projection.go:58).

## What the investigation found that the design must handle

1. **Not every files writer replaces by rename.** The Files API
   (`writeOwnerFileDurable`, `internal/autoputer/files.go:475`), the
   screens writer (`selfdev_screens.go:122-134`) and hydration
   (`file_sync.go:436-482`, and the whole-root swap at 531-539) write a
   temporary file and rename it. `texture_import.go:515` writes the
   Texture shortcut manifest in place with `os.WriteFile` (truncate, same
   inode). A hardlink snapshot taken before that write would see it change.
2. **Modes.** Files are written 0644 by default, but a file keeps the mode
   it was uploaded or hydrated with (`file_sync.go`, `entry.Mode`). The
   capsule runs as host uid 65534. A 0600 file in a hardlink snapshot is
   unreadable in the capsule, and chmod on a hardlink changes the live file.
3. **The guest store** (`RUNTIME_STORE_PATH=/mnt/persistent/state`, the
   SQLite product store) is live with WAL. Its schema
   (`internal/store/schema.go`) holds no token, key or password column
   today; owner-private payloads are sealed with the privacy key, which
   lives in `choir-credentials`.
4. **The data volume is small and sparse.** Every block a copy writes is
   host allocation until the guest's fstrim timer returns it
   (`data_discard.go`). The store split (header/body) and the S4 measured
   admission (build reservations charged at spawn) are the recent storage
   work; a design that copies the store per capsule without a charge would
   undo both.

## Decision

The capsule root becomes three lowers, top to bottom:

```text
sourceLower    workspace/platform        the computer's current source (unchanged mechanism)
computerLower  computer/files            the computer's files, a point-in-time snapshot
               computer/state/store.db   the computer's product store, a point-in-time copy
/              the guest root            unchanged, with every mask unchanged
```

All three are read-only lowers. Every write in the capsule lands in its
upper. The upper reaches the computer only through Freeze, verify and
apply, and Freeze carries only `workspace/platform` and the screens leaf,
as today. A write under `/computer` is scratch: it dies with the capsule
and is counted (not carried) in the Freeze manifest.

`/mnt` and `/run` stay tmpfs masks. The computer's state appears at
`/computer`, not at `/mnt/persistent`, so no mask is loosened. The broker
gives every workload `CHOIR_COMPUTER_FILES=/computer/files` and
`CHOIR_COMPUTER_STORE=/computer/state/store.db`, and the engineering
prompt names them.

### 1. The files lower: a hardlink snapshot under the files write lock

- **One write path.** Every write under the files root goes through one
  helper (the existing `writeOwnerFileDurable`, moved to a small package)
  that writes a temporary file and renames it, under the read side of one
  process-wide files lock. `texture_import.go` and the hydration paths move
  onto it. A repo gate (`check-owner-files-writes`) refuses a call to
  `provideriface.ResolveFilesRoot` outside an allowlist of readers and the
  helper.
- **The snapshot** takes the write side of that lock, walks the files root
  and hardlinks each regular file into
  `capsule-state/<boot>/computer/<spawn>/files`, then releases the lock.
  A rename-only writer never changes a linked inode, so the snapshot is the
  files tree at the instant the lock was held. Directories are created, not
  linked.
- **Left out:** `Source/platform` (it is the source lower already), every
  path `secretBearingPath` names, symlinks, sockets and devices. Each
  left-out path is named in the spawn receipt (count, and the paths
  relative to the root).
- **Unreadable modes are copied, not linked.** A file without `o+r` is
  copied and the copy made 0444; the live file is untouched. Bytes copied
  are in the receipt.
- **Cost.** One directory entry per file and no data blocks. A file deleted
  live keeps its blocks until the capsule ends. The walk time is measured
  in S0 and recorded per spawn.
- **Why not the live files root as a lower:** overlayfs leaves a lower
  that changes under a mount undefined. Why not a full copy: the files root
  holds media, and the volume is small.

### 2. The store lower: a store-owned snapshot copy, charged at spawn

- The store never mounts live. The store gains one method,
  `SnapshotTo(path)`, that writes a consistent standalone copy in one read
  transaction (`VACUUM INTO` for each database file the store owns,
  partitions included, after the store split). The capsule sees
  `/computer/state/store.db` (and its partitions beside it) as a read-only
  lower; SQLite in the capsule opens it and writes its WAL and any changes
  into the upper.
- **Reuse.** A copy is keyed by the store's change counter
  (`PRAGMA data_version` plus the event-index head). A later spawn whose
  key is unchanged links the same copy. The copy is removed with the last
  capsule that uses it.
- **Charge.** The copy's size is part of the capsule's S4 disk reservation.
  When it does not fit (the copy needs more than a quarter of the free
  space on `/mnt/persistent`), the capsule spawns without the store lower
  and the receipt names why. This is a resource refusal, not a switch: the
  paper's store is the expected case.
- **What the copy holds.** The product store has no credential column
  today. A schema test lists every table of the guest store with a class
  (`product`, `private_sealed`, `machine`); a new table fails the test
  until it is classified, and a `credential` class is refused in the
  store, not stripped from the copy. Sealed payloads stay sealed: the
  capsule has no privacy key, so a sealed body shows as locked. That is I19
  working, not a gap.
- **Consistency across the two lowers.** The files snapshot and the store
  copy are two instants a few seconds apart; the receipt names both (the
  files lock time, the store's event head). The files tree and the store
  are separate ledgers (computer ontology, "Do not force every change
  through one storage abstraction"), and nothing promises a cross-ledger
  instant today.

### 3. Secrets and authority, and how I19 holds

I19: "Spawned processes never inherit Choir capabilities, credentials,
canonical-state access, or control sockets."

- **Credentials stay out, unchanged.** `/run` and `/mnt` stay masked, so
  `choir-bootstrap` (gateway token, delegate credential), the autoputer env
  file, `choir-credentials` (privacy key), `choir-signers`,
  `choir-updater`, the signer, verifier and updater sockets are absent. The
  files snapshot drops secret-bearing paths with the release's predicate.
  The store copy carries no credential class (section 2). The broker env is
  unchanged.
- **No canonical-state access.** The capsule gets copies as data. It never
  receives the files root, the store file, the tape, corpusd, a store
  handle or a socket. It cannot see a later change and cannot write the
  computer. Its writes go to its upper, and only the source patch leaves
  through Freeze, verify and apply.
- **No control socket, no live handle.** Nothing in this design opens a
  path from the capsule namespace to a guest service.
- **Doctrine note** (lands with S2, green): add to I19, "A spawned process
  may receive a read-only, point-in-time copy of the computer's state as
  data in its lower layers. A copy is not access: it carries no credential,
  no store handle, no write path and no live view." Add the
  `computerLower` row to `docs/computer-ontology.md` (capsules section).
- **Privacy.** What the engineer reads enters its trace and its model's
  context, as source does today. Host-owned privacy policy (I23) still
  governs receipts. The owner's ruling accepts this for owner data.

### 4. The live product: not reached from the capsule

The capsule's browser does not reach the computer's live frontend or API.
A read-only route would be the first path from the capsule namespace to a
guest service, it is a live view (I19 as noted above), and every GET on the
API would need an audit for side effects and secret reads. It is also the
wrong picture: the live computer runs the old code.

The real desktop is shown instead by running code in the capsule over the
snapshot: the candidate (after) and the unchanged source lower (before)
both start with `AUTOPUTER_FILES_ROOT=/computer/files` and the store copy,
loopback only, no gateway token, no signer. S3 makes the autoputer start
that way and fail closed on any path that needs a credential. If a later
need appears (a screenshot of the served release itself), it is a separate
red design.

### 5. Source auto-update

- **Projection runs on every computer.** S4 deletes `sourceproj.Enabled`,
  `CHOIR_SOURCE_PROJECTION` and the `source.projection` setting, and
  `source.rebase_offer` with them. Reconcile runs at boot, after the base
  fetch, and at every Cast, as it does on b@c. An empty-stack computer's
  Source/platform HEAD becomes the booted base after each deploy, so the
  source lower is the source the computer runs. Rollout is the ring walk:
  d@e first.
- **A moved base: re-materialize, then cast with a warning.** S6's rule for
  packages ("R re-materializes byte-identically onto the new base ...
  capsules cast on it", library design 10.3) is applied to the whole
  stack, ahead of S6:
  - When the stack's patch applies cleanly to the booted base B' (the
    existing `patchedTree`: `git apply --cached` with exact context, no
    three-way merge, no fuzz), the projection commit is B'+P'
    (deterministic, like `projectionCommit`), the new state is
    `stack_rematerialized`, and an ordinary Cast is admitted. The Cast's
    commitment ledger carries the warning: the served build is still the
    stack on B; the capsule builds on B'+P'; approving its candidate also
    approves P' on the new base.
  - When it does not apply cleanly, `stack_base_moved` stays a refusal of
    ordinary Casts, and the U4 rebase notice (now always on) asks the root
    to Cast the rebase. The conflict is the work.
  - Casting on the old base B+P with a warning is rejected: the host
    builder needs a patch on the booted base, so that candidate could not
    apply.
  - S6 later replaces the whole-stack answer with per-package entries.
- **Source/platform working tree.** Reconcile already refuses to move a
  dirty tracked tree (`workTreeDirty`). Nothing else writes there.

## Failure modes (written before code)

1. A files writer bypasses the helper and writes in place: the capsule
   sees a changed lower. Guard: the repo gate; a test that every writer in
   the allowlist renames.
2. The walk follows a symlink out of the files root. Guard: `O_NOFOLLOW`
   opens relative to the parent descriptor, as `prepareCapsuleLeafDir`
   does; symlinks are left out and named.
3. A secret-bearing file is linked. Guard: the predicate runs on every path
   before the link; the S2 test seeds each name the predicate lists.
4. The snapshot holds the files lock too long and Files API writes stall.
   Guard: the lock covers only the walk; S0 measures it; the receipt
   records it.
5. The store copy fills the volume. Guard: the S4 charge and the
   quarter-of-free bound; the copy is written to a temporary name and
   renamed, and removed on any failure.
6. A crash leaves snapshots on disk. Guard: boot removes
   `capsule-state/<old boot>` as it does today; a capsule never spans a
   restart (deploys refuse a live capsule).
7. A hardlinked inode is written through the overlay. Overlay opens a
   copy-up in the upper for any write; the lower directory sits outside the
   chroot and the Landlock set. Test: write every file in `/computer/files`
   in a capsule, then compare the live file's sha256.
8. Projection everywhere resets a computer's Source/platform. Guard: the
   empty-stack reconcile records `From` (the rollback handle); a dirty tree
   is refused, not reset.
9. Re-materialization hides a semantic conflict that applies cleanly.
   Guard: the candidate is built and verified on B'+P' before apply, and
   the Texture warning names P'.
10. A new store table holds a credential. Guard: the classification test.

## Proof on staging, d@e first

All on d@e. No touch of the paper, the owner's computer, b@c or the four
arms.

- **P1 real files in a screenshot.** Seed d@e with a known corpus through
  `choir files put` (no personal data, as on the arms). Cast an engineering
  request to start the unchanged source over `/computer/files` and
  screenshot the Files app. Pass: the PNG under
  `Self-development/<operation>/screens/` shows the seeded names, and the
  spawn receipt's files listing digest equals a listing taken through
  `/api/files` at the spawn time.
- **P2 no secret path.** A capsule job tries to read
  `/mnt/persistent/choir-credentials/privacy-key`,
  `/run/choir-bootstrap/gateway-token`, `/run/go-choir-autoputer.env`, the
  signer and verifier sockets, `/etc/ssh`, and a seeded
  `notes/.env` (which must be absent from `/computer/files`), and greps its
  environment for token and key names. Pass: every read fails, the env has
  none, and the receipt names `notes/.env` as left out.
- **P3 no write to the lower.** The job writes into `/computer/files` and
  `/computer/state/store.db`. Pass: the writes succeed in the capsule, the
  live sha256 of each file and the live store's event head are unchanged,
  and Freeze's patch has no `/computer` path.
- **P4 no credential, no guest service.** From the capsule, connect to
  127.0.0.1:8085, the guest's tap address and the host's. Pass: refused
  (namespace, then the proxy's non-public refusal).
- **P5 source follows the deploy.** Deploy a commit; after d@e's refresh,
  the next spawn's source commit equals `/health` `build_commit`. Then
  apply a one-file candidate on d@e, deploy a commit that does not touch
  that file: the state is `stack_rematerialized` and a Cast is admitted
  with the warning on the ledger. Deploy one that does touch it: the state
  is `stack_base_moved`, the ordinary Cast is refused, and the rebase
  notice fires.
- **P6 store-backed content.** The unchanged source started over the store
  copy lists the same Texture document titles as the live computer.

## Slices

| Slice | What | Class, review |
| --- | --- | --- |
| S0 | Measure on d@e, read-only: files count and bytes, store size, `VACUUM INTO` time, walk time, free space | green, none |
| S1 | One files write helper with the snapshot lock; `texture_import` and hydration move onto it; `check-owner-files-writes` gate | orange, tier 1 |
| S2 | `computerLower` in the executor: files hardlink snapshot, leaves-out and copies, `/computer/files`, spawn receipt fields, Freeze counts `/computer` writes; the I19 note and the ontology row | red, tier 2 |
| S3 | `store.SnapshotTo`, the store lower with reuse and the S4 charge, the table classification test; the autoputer starts in a capsule over the snapshot with no credential; the engineering prompt names the paths | red, tier 2 |
| S4 | Delete the projection and rebase-offer switches; `stack_rematerialized` and the Cast warning | red, tier 2 |

S1 and S4 are independent and can build in parallel; S2 needs S1; S3 needs
S2. P1 to P4 prove S2 and S3; P5 proves S4; P6 proves S3. When P1 passes,
the interim answer in the problem doc (FP0 screenshots on the live arm
after apply) is retired.

## Rollback

- S1 to S3: revert. Snapshots are transient and removed at the next boot;
  no data migrates. A reverted executor goes back to `sourceLower:/`.
- S4: revert brings the switches back (off). A computer whose HEAD
  projection moved keeps its refs; `refs/choir/base` and the recorded
  `From` are the handles (`git reset --hard`), as today.

## Heresy delta

- discovered: `texture_import.go` writes an owner file in place (no
  durable rename); projection and rebase offer are switches the
  no-switches rule retires; the guest root lower still shows the guest's
  `/etc`, `/var`, `/home` and `/root` to the capsule, bounded only by file
  modes (P2 checks `/etc/ssh`; narrowing that lower is a residual).
- introduced: owner data inside capsules (by the owner's ruling), stated
  in I19.
- repaired (once built): stand-in content in self-development screenshots;
  a source tree that predates the deploy.

## Residuals

- **Exfiltration bound.** The capsule now holds owner data, and its egress
  allowlist (package registries) is the bound on where it can go. A
  request-body bound on egress while `/computer` is present is a candidate
  follow-up.
- **The guest root lower** shows world-readable guest files outside the
  masks. A narrower lowest layer (the system closure only) is a separate
  change.
- **Sealed content** appears locked in the capsule. Showing it would need a
  state key in the capsule, which I19 forbids.
