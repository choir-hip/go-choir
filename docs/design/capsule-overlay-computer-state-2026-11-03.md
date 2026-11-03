# Capsules overlay the exact computer state; the source tree follows the deploy (2026-11-03)

Status: design, revision 2, under review (tier 2). Mutation class of this
doc: green. The change it designs is red: capsule isolation, owner data in
a capsule, the guest store, source projection and the Cast gate. Problem:
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
  (the files root, `state.db`, `choir-credentials`, `choir-signers`,
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
  `.key`, `.p12`, `.pfx` in the release and the carried patch. It was built
  for source trees.
- **Freeze.** Every upper path is classified
  (`internal/capsule/transaction/classifier.go:96-150`); a path no rule or
  ignore entry names is `LedgerUnknown` and rejects the Freeze. The source
  patch is cut from `workspace/platform` only (`manifest.go:89`).
  `composeFreezePatch` (`source_projection.go:445-480`) turns a patch
  pinned to a projection commit into a cumulative patch from the stack base
  named by its `Choir-Stack-Base` trailer (`sourceproj/compose.go`), so the
  host builder's base join (`hostbuild/service.go:394-404`) and the
  updater's (`updater/updater.go:487-501`) see the booted base.
- **The store** is one SQLite file, `/mnt/persistent/state.db`
  (`store.StateDBPath`), with a sidecar tree `state.texture/`. It already
  has a consistent-copy primitive: `PrepareCompactCopy` (`store/compact.go`),
  `VACUUM INTO` in one read transaction with a sidecar naming the event
  head the copy holds.
- **Source projection** (`internal/sourceproj`,
  `internal/agentcore/source_projection.go`) moves Source/platform HEAD to
  the booted base (empty stack) or to the applied stack's projection
  commit. It runs at boot (`runtime.go:1054`) and at every Cast
  (`sourceProjectionCastGateFor`). It is behind `sourceproj.Enabled`: the
  env `CHOIR_SOURCE_PROJECTION` or the setting `source.projection`, on only
  for b@c. Off, Source/platform stays at its creation commit (problem 3).
- **A moved base.** When the applied stack's base is not the booted base
  and the patch is not contained in it, the state is `stack_base_moved`
  and every ordinary Cast is refused (`sourceproj.go:330-341`). The only
  admitted Cast is a U4 rebase Cast. Its notice and gate need three
  settings: `source.projection`, `source.rebase_offer` and
  `texture.root_desk` (`rebaseOfferOn`, `rebase_notice.go:96-101`), and a
  live root RLM to mint the notice.
- **Reconcile resets a dirty tree.** `ensureHead` computes `dirty` and then
  runs `git reset --hard` anyway, flagging `Repaired`
  (`sourceproj.go:390-434`); `From` goes only to the guest-local
  `facts.jsonl`.

## What the investigation and review 1 found

1. **Not every files writer replaces by rename.** The Files API
   (`writeOwnerFileDurable`, `internal/autoputer/files.go:475`), the
   screens writer (`selfdev_screens.go:122-134`) and hydration
   (`file_sync.go:436-482`) write a temporary file and rename it; hydration
   also swaps the whole root (`file_sync.go:521-556`).
   `texture_import.go:515` and `ensureDefaultPolicyFile`
   (`modelpolicy/model_policy.go:593-610`, `System/model-policy.toml`)
   write in place. A hardlink snapshot sees an in-place write.
2. **Modes.** The Files API writes 0644. Hydration keeps each file's
   recorded mode and creates every directory 0700 (`file_sync.go:489,507`).
   The capsule runs as host uid 65534.
3. **The store** schema holds no token, key or password column today, but
   `desktop_sessions` carries session ids (`schema.go:205-231`).
   Owner-private payloads are sealed with the privacy key, which lives in
   `choir-credentials`.
4. **The data volume is small and sparse.** Every block a copy writes is
   host allocation until the guest's fstrim timer returns it
   (`data_discard.go`). The store split, compaction and the S4 measured
   admission (build reservations charged at spawn) are the recent storage
   work; an uncharged per-capsule copy would undo them.

## Decision

Only engineering (self-development) capsules get the computer's state.
Research Exec and desk-tool capsules are unchanged.

The engineering capsule root becomes:

```text
upper          computer/state.db (+ state.texture/)   a private store copy, written at spawn
sourceLower    workspace/platform                     the computer's current source (unchanged)
computerLower  computer/files                         the computer's files, a point-in-time snapshot
/              the guest root                         unchanged, every mask unchanged
```

`/mnt` and `/run` stay tmpfs masks; the state appears at `/computer`, so
no mask is loosened. The broker gives engineering workloads
`CHOIR_COMPUTER_FILES=/computer/files` and
`CHOIR_COMPUTER_STATE=/computer/state.db`, and the engineering prompt
names them.

Every write in the capsule lands in its upper. `/computer/` joins the
classifier's ignore list and `walkUpperdir` skips it before stat or hash,
so a write there is scratch: it never rejects a Freeze, is never carried,
and dies with the capsule. What reaches the computer is unchanged: the
source patch and the screens, through Freeze, verify and apply.

### 1. The files lower: a hardlink snapshot under the files write lock

- **One write path.** Every write under the files root goes through one
  helper (the existing `writeOwnerFileDurable`, moved to a small package)
  that writes a temporary file and renames it, under the read side of one
  process-wide files lock. `texture_import`, the model-policy default, the
  screens writer and hydration move onto it; the hydration root swap takes
  the lock too. A repo gate (`check-owner-files-writes`) refuses
  `os.WriteFile`, `os.Create`, `os.OpenFile` with a write flag, `os.Rename`
  and `os.Remove*` in the packages that hold the files root (autoputer,
  textureowner, modelpolicy, agentcore screens), except inside the helper.
  Consistency is bounded by writers in the autoputer process; nothing else
  writes the files root today.
- **The snapshot** takes the write side of that lock, walks the files root
  with `O_NOFOLLOW` opens relative to each parent descriptor, and hardlinks
  each kept regular file into `<capsule state dir>/computer-lower/files`,
  then releases the lock. A rename-only writer never changes a linked
  inode, so the snapshot is the files tree at the instant the lock was
  held. The snapshot sits inside the capsule's own state directory, so the
  existing destroy path removes it and boot removes any a crash left.
- **Directories** are created 0755 in the snapshot. A directory mode in the
  files root is an artifact of how it was written (hydration makes every
  directory 0700), not an owner boundary: the Files app shows them all.
- **Files are linked, never chmodded or copied.** A file without `o+r` is
  left out and named, not raised to readable (raising a mode would read
  through a boundary the owner or a tool set).
- **Left out, and named in the spawn receipt:** `Source/platform` (it is
  the source lower already); every `.git` directory; symlinks, sockets and
  devices; files without `o+r`; and every path the files predicate names.
  The files predicate is `secretBearingPath` plus the credentials people
  keep in home-like trees: the directories `.aws`, `.kube`, `.docker`,
  `.ssh`, `.gnupg`, `.config/gcloud`, `.password-store`; the names
  `.git-credentials`, `.pgpass`, `.envrc`, `*.env`, `*credentials*`,
  `id_*` keys, `*.kdbx`, `*.keychain*`, cookie databases (`Cookies`,
  `cookies.sqlite`). It is a list, not a proof: residual below.
- **Cost.** One directory entry per file and no data blocks. A file deleted
  live keeps its blocks until the capsule ends. The walk time and the lock
  hold are measured in S0 and recorded per spawn.
- **Why not the live files root as a lower:** overlayfs leaves a lower
  that changes under a mount undefined. Why not a full copy: the files root
  holds media, and the volume is small.

### 2. The store: one private copy per spawn, in the upper, charged

- The store never mounts live. S3 adds `Store.SnapshotTo(dir)` on the
  `PrepareCompactCopy` machinery: `VACUUM INTO` in one read transaction,
  with the event head the copy holds, written to a temporary name and
  renamed. It also copies the `state.texture/` files `store.Open` needs
  (floor, lease, schema receipt), and nothing else of the sidecar:
  telemetry partitions and `state.image-blobs` are left out and named
  (images in documents show as missing in the capsule).
- **The copy is written straight into the capsule's upper** at
  `computer/state.db`. It is a private copy, so it needs no lower: SQLite
  in the capsule writes it and its WAL in place, with no copy-up of a
  second full file. One file, one charge.
- **No reuse.** Each spawn takes a fresh copy (spawns are a few per round).
  A reuse key would need a store-wide commit generation the store does not
  keep.
- **Charge.** The copy's size, and an upper allowance for writes under
  `/computer`, are part of the capsule's S4 disk reservation, against the
  same free-space floor compaction uses. When it does not fit, the capsule
  spawns without the store copy and the receipt names why. This is a
  resource refusal, not a switch; the paper's store is the expected case.
- **What the copy holds.** A schema test scans every column of every guest
  store table for credential-shaped names (`token`, `secret`, `key`,
  `password`, `cookie`, `session`, `nonce`, `credential`). Each match is
  either allowlisted with a reason in the test (for example
  `tokens_before`, a count) or its rows are deleted from the copy before
  the capsule starts (`desktop_sessions` until reviewed). A new matching
  column fails the test until it is classified. Sealed payloads stay
  sealed: the capsule has no privacy key, so a sealed body shows as locked.
- **Consistency across the two.** The files snapshot and the store copy are
  two instants a few seconds apart; the receipt names both (the files lock
  time, the store's event head). The files tree and the store are separate
  ledgers (computer ontology, "Do not force every change through one
  storage abstraction"); nothing promises a cross-ledger instant today.

### 3. Secrets and authority, and how I19 holds

I19: "Spawned processes never inherit Choir capabilities, credentials,
canonical-state access, or control sockets."

- **Credentials stay out, unchanged.** `/run` and `/mnt` stay masked, so
  `choir-bootstrap` (gateway token, delegate credential), the autoputer env
  file, `choir-credentials` (privacy key), `choir-signers`,
  `choir-updater`, the signer, verifier and updater sockets are absent. The
  files predicate and the mode rule keep credential files out of the files
  snapshot. The store copy drops credential-shaped columns (section 2). The
  broker env is unchanged apart from the two path names.
- **No canonical-state access.** The capsule gets copies as data. It never
  receives the files root, `state.db`, the tape, corpusd, a store handle or
  a socket. It cannot see a later change and cannot write the computer.
- **No control socket, no live handle.** Nothing here opens a path from
  the capsule namespace to a guest service.
- **Doctrine note** (lands with S2, green; it re-scopes I19, so it is the
  owner's ruling written down, not a new rule): "An engineering capsule may
  receive a read-only, point-in-time copy of the computer's state as data.
  A copy is not access: it carries no credential, no store handle, no write
  path and no live view." Add the `computerLower` row to
  `docs/computer-ontology.md` (capsules section).
- **Privacy.** What the engineer reads enters its trace and its model's
  context, as source does today. Host-owned privacy policy (I23) still
  governs receipts. The owner's ruling accepts this for owner data.

### 4. The live product: not reached from the capsule

The capsule's browser does not reach the computer's live frontend or API.
A read-only route would be the first path from the capsule namespace to a
guest service, it is a live view, and every GET on the API would need an
audit for side effects and secret reads. It is also the wrong picture: the
live computer runs the old code.

The real desktop is shown by running code in the capsule over the copies:
the candidate (after) and the unchanged source lower (before) both start
with `AUTOPUTER_FILES_ROOT=/computer/files` and the store copy, loopback
only, no gateway token, no signer. S3 makes the autoputer start that way
and fail closed on any path that needs a credential. A screenshot of the
served release itself would be a separate red design.

### 5. Source auto-update

- **Projection runs on every computer.** S4 deletes `sourceproj.Enabled`,
  `CHOIR_SOURCE_PROJECTION`, the `source.projection` setting and
  `source.rebase_offer`, and drops the `texture.root_desk` conjunct from
  the rebase path. Reconcile runs at boot, after the base fetch, and at
  every Cast. An empty-stack computer's HEAD becomes the booted base after
  each deploy, so the source lower is the source the computer runs.
  Rollout is the ring walk: d@e first.
- **Reconcile stops destroying work.** Before any `reset --hard`, S4 makes
  `ensureHead` save what it would lose: a dirty tracked tree is committed
  to `refs/choir/saved/<utc>`, a foreign HEAD is kept at the same kind of
  ref, and both, with `From`, go on the tape as a `source.projection_saved`
  fact. The reset then proceeds. The source stays current, and nothing is
  lost silently.
- **A moved base: re-materialize when byte-identical, else rebase.** S6's
  package rule ("R re-materializes byte-identically onto the new base ...
  capsules cast on it", library design 10.3) is applied to the whole stack,
  ahead of S6:
  - When every path the stack's patch touches has the same blob and mode
    in B' as in B, the patch's after-image is grafted byte-identically: the
    tree is B' with P's after-image at those paths. The projection commit
    has parent B' and `Choir-Stack-Base: B'`. The state is
    `stack_rematerialized`, reported in `/health`, and an ordinary Cast is
    admitted. The Cast's commitment ledger carries the warning: the served
    build is still the stack on B; the capsule builds on B'+P'; approving
    its candidate also approves P' on B'. `composeFreezePatch` then gives a
    cumulative patch based on B', which the host builder and the updater
    accept.
  - When any touched path differs between B and B', `stack_base_moved`
    stays a refusal of ordinary Casts, and the U4 rebase notice asks the
    root to Cast the rebase. With no live root RLM, the refusal says so.
  - Casting on B+P is rejected: its cumulative patch is based on B, which
    the builder refuses after a deploy (`base_commit_mismatch`).
  - S6 later replaces the whole-stack answer with per-package entries.

## Failure modes (written before code)

1. A files writer bypasses the helper: the capsule sees a changed lower.
   Guard: the gate; a test per allowlisted writer that it renames.
2. The walk follows a symlink out of the root. Guard: `O_NOFOLLOW` opens
   relative to the parent descriptor; symlinks are left out and named.
3. A credential file is linked. Guard: the files predicate and the `o+r`
   rule; the S2 test seeds every listed name.
4. The lock stalls Files API writes. Guard: it covers only the walk; S0
   measures it; the receipt records it.
5. The store copy fills the volume. Guard: the S4 charge against the
   compaction floor; temporary name and rename; removal on failure.
6. A crash leaves copies on disk. Guard: both live in the capsule's state
   directory, removed by destroy and at boot; a capsule never spans a
   restart (deploys refuse a live capsule).
7. A hardlinked inode is written through the overlay. Overlay copies up
   into the upper for any write; the lower sits outside the chroot and the
   Landlock set. Test: write every file in `/computer/files`, compare the
   live sha256.
8. Projection everywhere wipes owner edits in Source/platform. Guard: the
   save-before-reset ref and tape fact; test: a Files API edit to a tracked
   file survives a boot as a saved ref.
9. A graft hides a semantic conflict. Guard: graft only when B' left every
   touched path unchanged; the candidate is built and verified on B'+P'.
10. A new store column holds a credential. Guard: the column scan.
11. `/computer` writes reject Freeze. Guard: the ignore entry and the walk
    skip; P3 freezes with such writes present.

## Proof on staging, d@e first

All on d@e. No touch of the paper, the owner's computer, b@c or the four
arms.

- **P1 real files in a screenshot.** Seed d@e through `choir files put`
  with a known corpus (no personal data) that includes a nested directory.
  Cast an engineering request to start the unchanged source over
  `/computer/files` and screenshot the Files app. Pass: the PNG under
  `Self-development/<operation>/screens/` shows the seeded names, including
  the nested ones, and a listing taken inside the capsule equals the
  seeded set minus the receipt's named leave-outs.
- **P2 no secret path.** Seed `notes/.env`, `home/.aws/credentials`, a
  `.git/config` with a credential-shaped remote URL, and a 0600 file with
  a plain name holding a canary string. A capsule job then tries to read
  `/mnt/persistent/choir-credentials/privacy-key`,
  `/run/choir-bootstrap/gateway-token`, `/run/go-choir-autoputer.env`, the
  signer and verifier sockets and `/etc/ssh`, greps its environment for
  token and key names, and greps `/computer` for the canary. Pass: every
  read fails, none of the seeded files is present, the canary is absent,
  and the receipt names each leave-out.
- **P3 no write to the computer.** The job writes into `/computer/files`
  and `/computer/state.db`, and changes one source file. Pass: the writes
  succeed in the capsule; the live sha256 of each file and the live
  store's event head are unchanged; Freeze succeeds and its patch has the
  source file and no `/computer` path.
- **P4 no guest service.** From the capsule, connect to 127.0.0.1:8085,
  the guest's tap address and the host's. Pass: refused.
- **P5 source follows the deploy.** Deploy a commit; after d@e's refresh
  the next spawn's source commit equals `/health` `build_commit`. Apply a
  one-file candidate on d@e; deploy a commit that does not touch that file:
  `/health` shows `stack_rematerialized`, a Cast is admitted with the
  warning, and its approved candidate passes the host build and applies.
  Deploy one that touches the file: `stack_base_moved`, the ordinary Cast
  is refused, and the rebase notice fires. Edit a tracked source file
  through the Files API and reboot: the edit is at a saved ref and on the
  tape.
- **P6 store-backed content.** The unchanged source started over the store
  copy lists the same Texture document titles as the live computer.

## Slices

| Slice | What | Class, review |
| --- | --- | --- |
| S0 | Measure on d@e, read-only: files count and bytes, `state.db` size, `VACUUM INTO` time, walk and lock time, free space | green, none |
| S1 | One files write helper with the snapshot lock; every writer moves onto it; `check-owner-files-writes` | orange, tier 1 |
| S2 | `computerLower` for engineering capsules: hardlink snapshot, files predicate, leave-outs, `/computer` in the classifier ignore and the walk skip, receipt fields; the I19 note and the ontology row | red, tier 2 |
| S3 | `Store.SnapshotTo`, the private copy in the upper, the S4 charge, the column scan; the autoputer starts in a capsule over the copies with no credential; the prompt names the paths | red, tier 2 |
| S4 | Delete the projection and rebase switches and the root-desk conjunct; save-before-reset; `stack_rematerialized` with the byte-identical graft and the Cast warning | red, tier 2 |

S1 and S4 can build in parallel; S2 needs S1; S3 needs S2. P1 to P4 prove
S2 and S3; P5 proves S4; P6 proves S3. When P1 passes, the interim answer
in the problem doc (FP0 screenshots on the live arm after apply) retires.

## Rollback

- S1 to S3: revert. The copies are transient and removed with the capsule
  or at the next boot; no data migrates. A reverted executor goes back to
  `sourceLower:/`.
- S4: revert brings the switches back (off). A computer whose HEAD
  projection moved keeps its refs; `refs/choir/base`, the saved refs and
  the recorded `From` are the handles (`git reset --hard`).

## Heresy delta

- discovered: two files writers write owner files in place; reconcile
  resets a dirty tree while the code comments imply a repair; projection,
  rebase offer and the root-desk conjunct are switches the no-switches rule
  retires; the guest root lower shows world-readable guest files outside
  the masks.
- introduced: owner data inside engineering capsules (by the owner's
  ruling), written into I19.
- repaired (once built): stand-in content in self-development screenshots;
  a source tree that predates the deploy; silent loss of Source/platform
  edits.

## Residuals

- **Exfiltration bound.** An engineering capsule now holds owner data, and
  its egress allowlist (package registries and code hosts) is the bound on
  where it can go. A request-body bound on egress for capsules with
  `/computer` is a candidate follow-up.
- **The files predicate is a list.** A credential under a plain name with
  `o+r` gets through. A content scan of small text files at snapshot time
  is the next step if S0 shows it is cheap.
- **The guest root lower** shows world-readable guest files outside the
  masks. A narrower lowest layer (the system closure only) is a separate
  change.
- **Sealed content and document images** appear locked or missing in the
  capsule.

## Decisions after review 1 (Devin send_back, DeepSeek send_back)

- `/computer` writes rejected Freeze (Devin 1): fixed, ignore entry and
  walk skip (DeepSeek 5), P3 freezes with them.
- Raising modes and the source-shaped predicate (Devin 2, DeepSeek 1):
  fixed, no copy or chmod; a wider files predicate; P2 seeds a canary
  outside the predicate.
- Reconcile destroys dirt (Devin 3, DeepSeek 3): fixed, save-before-reset
  with a tape fact; P5 checks it.
- The root-desk conjunct (Devin 4, DeepSeek 6): fixed, dropped in S4; a
  dead root RLM is a named refusal.
- Directory modes (Devin 5): fixed, 0755.
- Store layout, key and copy-up (Devin 6 and 7, DeepSeek 4 and 5): fixed,
  `state.db` and the needed sidecar files, on `PrepareCompactCopy`, no
  reuse, written into the upper, charged against the compaction floor.
- Model-policy writer and the gate (Devin 8, DeepSeek 9): fixed, the gate
  checks write calls in the holding packages, and the root swap takes the
  lock.
- Graft vs re-apply (Devin 9): fixed, byte-identical graft only, `/health`
  state, P5 applies an approved candidate.
- I19 scope (Devin 10): fixed, engineering capsules only.
- Column scan (DeepSeek 10): fixed.
- Snapshot lifetime (DeepSeek 8): fixed, inside the capsule state dir.
- DeepSeek 2 (a projection-pinned candidate cannot apply): not taken as
  stated. `composeFreezePatch` already rebases the patch onto the
  `Choir-Stack-Base` trailer, and the graft writes B' there. P5 now proves
  an approved re-materialized candidate builds and applies.
