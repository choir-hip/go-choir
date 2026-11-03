# Capsules overlay the exact computer state; the source tree follows the deploy (2026-11-03)

Status: design, revision 4: revision 3 accepted at tier 2 (Devin round 3 accept_with_nits; nits folded in below); revision 4 folds in the owner's "no credentials on computers" input and the S0 inventory. Mutation class of this
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
`SpawnSpec` gains `ComputerState bool`, set only by the engineering
assignment's spawn from its durable binding (as `VerifierBundleDir` is);
research Exec, desk-tool and verifier spawns never set it, and the
executor mounts `computerLower` only when it is set.

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
classifier's ignore list and `walkUpperdir` returns `SkipDir` at
`computer` (so nothing below it is stat'd or hashed),
so a write there is scratch: it never rejects a Freeze, is never carried,
and dies with the capsule. What reaches the computer is unchanged: the
source patch and the screens, through Freeze, verify and apply.

### 1. The files lower: a hardlink snapshot under the files write lock

- **One write path.** Every write under the files root goes through one
  helper (the existing `writeOwnerFileDurable`, moved to a small package)
  that writes a temporary file and renames it, under the read side of one
  process-wide files lock. `texture_import`, the model-policy default, the
  screens writer, the maildir (`NewMaildir(filesRoot/mail)`, `run.go:647`,
  `maildir.go`) and hydration move onto it; the hydration root swap takes
  the lock too. A repo gate (`check-owner-files-writes`) refuses
  `os.WriteFile`, `os.Create`, `os.OpenFile` with a write flag, `os.Rename`,
  `os.Remove*` and `os.Mkdir*` in the packages that hold the files root (autoputer,
  textureowner, modelpolicy, agentcore screens), except inside the helper.
  Consistency is bounded by writers in the autoputer process; nothing else
  writes the files root today.
- **The snapshot** takes the write side of that lock, walks the files root
  with `O_NOFOLLOW` opens relative to each parent descriptor, and hardlinks
  each kept regular file into `<capsule state dir>/computer-lower/files`
  (directories root-owned 0755, so the capsule reads but cannot write the
  lower itself),
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
- **Left out, and named in the spawn receipt:** `Source/platform`;
  symlinks, sockets and devices; files without `o+r`; the helper's
  temporary names (`.*.tmp-*`, `.screen-*`, `.choir-hydrate-*`); and the
  `mail/` tree (the owner's emails are in the keep set; a Mail round gets
  them by a later owner decision, not by default).
- **Why `Source/platform` is not under `/computer/files`.** The computer's
  source is the capsule's editable `/workspace/platform` (the source
  lower), kept current by S4. A second, read-only copy at
  `/computer/files/Source/platform` would invite edits there, and every
  write under `/computer` is scratch that dies with the capsule, so those
  edits would be thrown away. The leave-out is also load-bearing for
  consistency: `git reset --hard` writes that tree outside the files lock.
- **No credential-name list** (owner, 2026-11-03: "there shouldn't be
  credentials on computers anyway"). The S0 inventory
  (`docs/evidence/capsule-overlay-s0-measurement-2026-11-03.md`) found no
  credential in the files store or the store of d@e; every piece of
  computer identity material (the privacy key, the two signer keys, the
  gateway token, the event envelope) lives under `/mnt/persistent` or
  `/run`, which stay masked. So the snapshot has no speculative name
  blocklist. A credential that does reach a computer is a problem to
  remove from the computer (the one found, the Claude delegate token on
  three other computers, is
  `docs/problems/claude-delegate-token-on-three-computers-2026-11-03.md`),
  not a name to hide from the capsule.
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
  `computer/state.db`, with `computer/` and the file owned by the capsule's
  mapped uid (65534) as the release leaf is. It is a private copy, so it
  needs no lower: SQLite in the capsule writes it and its WAL in place,
  with no copy-up of a second full file. One file, one charge.
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
  `tokens_before`, a count) or blanked in the copy before the capsule
  starts. S0 found 13 matches and none holds a credential; the desktop
  session ids are layout keys the client picks (`X-Choir-Session`), not
  an authenticator, so they are allowlisted, not blanked. A new matching
  column fails the test until it is ruled on.
- **Blanked bytes do not survive.** When the scan below blanks a value,
  the copy's connection sets `PRAGMA secure_delete=ON` before the blanking,
  and the copy is then `VACUUM`ed once more, so no old cell remains in a
  free page. The S3 test plants a secret-shaped value, takes the copy, and
  greps the raw bytes of every file under `computer/` for it.
- **Payload columns** (`events.payload_json`, `choir_events.metadata`,
  `choir_event_bodies.body`, `computer_event_index.prepared_event_json`)
  are invisible to a name scan. The invariant is the one the tape already
  holds: private content is sealed, and `computerevent.LocatePrivateSecrets`
  runs on event bodies at append. S3 also runs it over every text column of
  the copy at snapshot time, as a check: a hit blanks that value and is
  named in the receipt, and a hit is also a problem to record, since the
  tape should never have held it. S0 measures what that scan costs. Sealed payloads stay
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
  `choir-updater`, the signer, verifier and updater sockets are absent.
  The S0 inventory found every piece of identity material behind those
  masks and no credential in the files store or the store. The mode rule
  keeps files the owner or a tool made unreadable out of the snapshot. The
  store copy's secret-shape scan stays as a check (section 2). The
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
  to `refs/choir/saved/<saved commit id>`, a foreign HEAD is kept at
  `refs/choir/saved/<its commit id>` (the name is the commit, so two saves
  never share a name; `update-ref` is called with an empty old value so it
  never overwrites), and both, with `From`, go on the tape as a `source.projection_saved`
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
    root to Cast the rebase. With no live root RLM, the Cast refusal gains
    the text `rebase_needs_root: no live root desk to Cast the rebase`
    (today `mintRebaseNotice` only logs it, `rebase_notice.go:189,235`).
  - Casting on B+P is rejected: its cumulative patch is based on B, which
    the builder refuses after a deploy (`base_commit_mismatch`).
  - S6 later replaces the whole-stack answer with per-package entries.

## Failure modes (written before code)

1. A files writer bypasses the helper: the capsule sees a changed lower.
   Guard: the gate; a test per allowlisted writer that it renames.
2. The walk follows a symlink out of the root. Guard: `O_NOFOLLOW` opens
   relative to the parent descriptor; symlinks are left out and named.
3. Identity material is linked. Guard: it lives under the masks, not in
   the files root (S0 inventory); the `o+r` rule; the S2 test seeds a 0600
   file and checks it is absent.
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
- **P2 no secret path.** Seed a 0600 file with a plain name holding a
  canary string. A capsule job then tries to read
  `/mnt/persistent/choir-credentials/privacy-key`, the signer keys under
  `/mnt/persistent/choir-signers/`, every file on the credential disk
  (`/run/choir-bootstrap/gateway-token`, `host-fetch.json`,
  `capsule-mirrors.json`, and the delegate's `claude-oauth-token`, which
  d@e does not have), `/run/go-choir-autoputer.env`, the signer and
  verifier sockets and `/etc/ssh`, greps its environment for token and key
  names, and greps `/computer` for the canary. Pass: every read fails, the
  seeded file is absent, the canary is absent, and the receipt counts the
  not-`o+r` leave-out. d@e is not a delegate computer, so P2 proves the
  masks there; the delegate token path on a delegate computer is a
  residual until the delegate moves behind the gateway (coordinator,
  2026-11-03).
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
| S2 | `computerLower` for engineering capsules: hardlink snapshot, the structural leave-outs, `/computer` in the classifier ignore and the walk skip, receipt fields; the I19 note and the ontology row | red, tier 2 |
| S3 | `Store.SnapshotTo`, the private copy in the upper, the S4 charge, the column scan; the autoputer starts in a capsule over the copies with no credential; the prompt names the paths | red, tier 2 |
| S4 | Delete the projection and rebase switches and the root-desk conjunct; save-before-reset; `stack_rematerialized` with the byte-identical graft and the Cast warning | red, tier 2 |

S1 and S4 can build in parallel; S2 needs S1; S3 needs S2. P1 to P4 prove
S2 and S3; P5 proves S4; P6 proves S3. When P1 passes, the interim answer
in the problem doc (FP0 screenshots on the live arm after apply) retires.

## Rollback

- S1 to S3: revert, keeping the `removePrivateTree` commit (it chmods
  directories only; the old chmod of every file would change a live file a
  stale layer links). The copies are transient and removed with the capsule
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

- **Delegate capsules are withheld the copy** (coordinator, 2026-11-03). A
  capsule spawned in claude network mode, or one whose broker or
  environment carries the Claude delegate token, gets no `/computer`
  layers: the spawn proceeds and its receipt records `computer_state:
  "withheld: delegate capsule (credential present)"`. A correctness
  refusal, not a switch; it lifts when the delegate call moves behind the
  gateway (`docs/problems/claude-delegate-token-on-three-computers-2026-11-03.md`).
- **Exfiltration bound.** An engineering capsule now holds owner data, and
  its egress allowlist (package registries and code hosts) is the bound on
  where it can go. A request-body bound on egress for capsules with
  `/computer` is a candidate follow-up.
- **No name list in the files snapshot.** A credential an owner keeps in
  a readable file reaches the capsule. By the owner's rule it should not be
  on the computer at all; the inventory is the check, rerun per ring.
- **The guest root lower** shows world-readable guest files outside the
  masks. A narrower lowest layer (the system closure only) is a separate
  change.
- **Sealed content, document images and mail** appear locked or missing
  in the capsule.
- **The boundary in the files snapshot is file mode only.** Directories are
  flattened to 0755 (hydration makes every directory 0700), so a 0644 file
  under a 0700 directory, unreachable to uid 65534 on the live tree, is
  readable in the capsule. File modes and the predicate are the boundary.

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

## Decisions after review 2 (Devin send_back)

- Deleted rows survive in free pages (1): fixed, blanking under
  `secure_delete` and a second `VACUUM`, with a raw-byte test.
- Rulings and payload columns (2): fixed, the three session-id columns are
  blanked by name; a secret-shape scan runs over every text column of the
  copy.
- The maildir (3): named, gated (`os.Mkdir*` added), and `mail/` is left
  out of the snapshot.
- Engineering at spawn (4): `SpawnSpec.ComputerState`, from the durable
  binding.
- Saved-ref collision (5): the ref is named by the saved commit, created
  with an empty old value.
- Directory flatten (6): stated as a residual.
- Small gaps (7): capsule-uid ownership of the copy, `SkipDir`, and the
  `rebase_needs_root` refusal text.

## Review 3 (Devin accept_with_nits); nits folded into the build

- Helper temp files (1): the walk leaves out `.*.tmp-*` and `.screen-*`,
  and the read lock spans write, fsync and rename.
- The gate (2): also `os.Truncate`, `os.Chmod`, `os.Chown`, `os.Link`,
  `os.Symlink`, `os.Chtimes`. The `Source/platform` leave-out is
  load-bearing: `git reset --hard` writes it outside the lock.
- Verification (3): engineering verification must not depend on
  `/computer`; the verifier's capsule stays `sourceLower:/`.
- The copy (4): checkpoint and close before the capsule starts; no `-wal`
  or `-shm` ships; the byte test greps every file under `computer/`.
- The predicate (5, superseded by revision 4): add `.config/gh`, `.azure`, `.s3cfg`, `.boto`,
  `.pypirc`, `.gitconfig`, `Login Data`, `logins.json`, `key4.db`,
  `*.ovpn`, `*.ppk`, `*.keystore`, `*.jks`.
- Proof (6): P3 uses the real engineering spawn with the in-capsule
  autoputer running; P5 adds a same-base stacked candidate that passes the
  host build.
- Spec (7): the layer root is `<state dir>/computer-lower`, holding
  `computer/files`; a saved ref that already exists with the same commit is
  success; a graft equal to B' reports `upstreamed`; lock order is the
  files lock, then the maildir's own mutex.

## Revision 4: owner input and the S0 inventory

Owner, 2026-11-03: "there shouldn't be credentials on computers anyway."

- S0 inventoried the secret-shaped material on d@e (names, owners, modes,
  sizes and kinds only): identity material only, all under the masks; no
  credential that unlocks something outside the computer.
- The files predicate's name lists and the three session-id blankings are
  cut; the leave-outs are structural (`Source/platform`, links and
  specials, modes, helper temporaries, `mail/`). The store copy's
  secret-shape scan and the column-name test stay as checks.
- Fleet-wide, the Claude delegate token is the one class (b) credential
  found; it is a problem doc proposing removal behind the gateway, and the
  overlay waits on it before the ring that holds those computers.
- The note on why `Source/platform` is not under `/computer/files` is in
  section 1.

## Build: S1 failure modes (written before code)

S1 adds `internal/ownerfiles`: the one write path for the files root, and
the process-wide lock (read side for every write, write side for the S2
snapshot).

1. **A recursive read lock deadlocks.** Go's `RWMutex` forbids taking the
   read side twice: a waiting snapshot blocks the second `RLock`. Guard:
   compound writes (the maildir delivery, the hydration root swap) take
   one `Tx` and call only its methods; the package functions each take
   their own. Test: a `Tx` doing many writes while `Exclusive` waits
   finishes, and `Exclusive` then runs.
2. **A writer still writes in place.** Guard: texture import and the
   model-policy default move to `WriteFile` (temporary file, fsync,
   rename). Test: a hardlink taken before a rewrite keeps the old bytes,
   for each moved writer.
3. **Durability regresses.** The C0 kill drill needs write, chmod, fsync,
   rename, fsync of the directory. Guard: the helper keeps that order and
   one sync hook (`ownerfiles.Sync`) the existing durability tests drive.
4. **A mode changes.** The Files API keeps the request's mode, texture
   import, the model policy and the screens keep 0644, hydration keeps the
   recorded mode, the maildir keeps 0600 files and 0700 directories.
5. **The lock is held across the network.** Hydration fetches chunks
   inside its write callback; that file's write holds the read side for
   its fetch. It writes into a staging tree outside the root during
   fresh-volume boot only, so a snapshot can wait one file's fetch, at
   most; the root swap is the one root-visible step and holds the lock
   for two renames. The backup removal runs after the lock is released.
6. **The gate misses a writer.** Guard: `scripts/check-owner-files-writes`
   scans every non-test Go file of `internal/autoputer`,
   `internal/textureowner`, `internal/modelpolicy` and
   `internal/agentcore/selfdev_screens.go` for `os` write calls
   (`WriteFile`, `Create`, `CreateTemp`, `OpenFile`, `Rename`, `Remove`,
   `RemoveAll`, `Mkdir`, `MkdirAll`, `MkdirTemp`, `Truncate`, `Chmod`,
   `Chown`, `Lchown`, `Link`, `Symlink`, `Chtimes`), `ioutil.WriteFile`
   and the `unix` write calls. A file that never touches the files root
   is allowlisted by name with its reason, so a new file in those
   packages is gated by default. Its selftest plants a raw write in a
   gated file and expects a failure.
7. **The lock order inverts.** The maildir takes the files lock, then its
   own mutex; nothing takes them the other way round.

## Build: S2 failure modes (written before code)

S2 adds the files layer: `<capsule state dir>/computer-lower/computer/files`,
a hardlink snapshot of the files root, mounted between the source lower and
the guest root for implementation assignments only.

1. **Cleanup changes a live file.** `removePrivateTree` chmods every regular
   file 0600 before `RemoveAll`; on a hardlink that is the live owner file.
   Guard: it chmods directories only (unlinking never needs a file's mode).
   Test: destroy a capsule whose layer links a 0644 file; the live file
   keeps 0644 and its bytes. Rollback (review 1, DeepSeek 2): the
   `removePrivateTree` change is its own commit, landed before the layer;
   a rollback of S2 reverts the layer commit and keeps that one, so no
   binary that builds layers can be followed by one that chmods them.
2. **The walk follows a link out of the root or links a special file.**
   Guard: `openat` with `O_NOFOLLOW|O_DIRECTORY` relative to the parent
   descriptor, `fstatat` with `AT_SYMLINK_NOFOLLOW`, `linkat` without
   `AT_SYMLINK_FOLLOW`, regular files only. Test: a symlink to a file
   outside the root, a symlinked directory, a FIFO and a socket are left
   out and named.
3. **A file the capsule must not read is linked.** Guard: files without
   `o+r` are left out and named, never chmodded. Test: a 0600 file with a
   canary is absent from the layer.
4. **The live tree changes during the walk.** Guard: the walk runs under
   `ownerfiles.Exclusive`; `Source/platform`, which git writes outside the
   lock, is left out. Test: a writer looping through the helper during the
   walk never leaves a torn or half-renamed file in the layer.
5. **The layer cannot be built** (different filesystems: `EXDEV`; a failed
   walk). Guard: the partial layer is removed and the capsule starts
   without `/computer/files`; the receipt names why. A capsule always
   starts; the copy is data, not a precondition.
6. **The capsule writes the live tree through the layer.** Overlay writes
   land in the upper; the lower is outside the chroot. Directories in the
   layer are owned by the capsule's mapped uid, so the capsule can add,
   rename and remove entries (all in its upper); linked files keep their
   owner and mode, so an owner file is read-only in the capsule. Test: a
   capsule job creates, replaces by rename and deletes under
   `/computer/files`, tries an in-place write of a linked file; the live
   tree's sha256 and modes are unchanged.
7. **`/computer` writes reject Freeze or ride the patch.** Guard: the
   classifier ignores `/computer/` and `walkUpperdir` skips `computer`.
   Test: a manifest walk of an upper with `computer/files/x` and
   `computer/state.db` has no `computer` entry; classifying `/computer/x`
   is ignored.
8. **A spawn that is not an implementation assignment gets the layer.**
   Guard: `SpawnSpec.ComputerState`, set only in the implementation
   assignment's spawn from its durable binding; research, desk-tool and
   verifier spawns never set it. Test: the spawn spec built for a
   verification binding has it false.
9. **The credential disk or identity material reaches `/computer`.**
   Guard: they live under `/run` and `/mnt/persistent`, outside the files
   root; `/run` and `/mnt` stay masked. P2 asserts the delegate token path
   and the credential disk are unreadable from an engineering capsule
   (coordinator, 2026-11-03); d@e is not a delegate computer, so P2 tests
   the masks there and the delegate case is a residual until the delegate
   moves behind the gateway
   (`docs/problems/claude-delegate-token-on-three-computers-2026-11-03.md`).

Build decisions (deviations from the text above, smaller in effect):

- **No broker env change.** The paths are fixed (`/computer/files`,
  `/computer/state.db`) and the engineering prompt names them, so the
  broker env stays exactly as it is. `/computer/receipt.json` (read-only,
  in the layer) holds the receipt for the engineer and for P1 and P2.
- **Directory owner.** Section 1 said root-owned 0755; the build makes them
  the capsule's mapped uid, 0755, so P3's writes succeed in the upper. The
  lower is outside the capsule's mount namespace either way.

## Build: S3 failure modes (written before code)

S3 adds the store copy (`Store.SnapshotTo`, written into the capsule's upper
at `computer/state.db` with `computer/state.texture/`), the disk charge, the
column-name test and the secret-shape scan, the in-capsule autoputer recipe,
and the prompt text.

1. **The copy is torn or ships a WAL.** Guard: `VACUUM INTO` in one read
   transaction to a temporary name, `journal_mode=DELETE` and close before
   the rename, so the copy is one file. Test: after `SnapshotTo`, `dir` holds
   `state.db` and `state.texture/` only, no `-wal`, `-shm` or temporary.
2. **A blanked value survives in a free page or another file.** Guard:
   `secure_delete=ON` before any blanking, then a second `VACUUM`. Test:
   plant a secret-shaped value in a text column, take the copy, grep the raw
   bytes of every file under the copy's directory.
3. **A blanking cannot be applied** (a trigger or constraint refuses the
   update). Guard: the copy fails, the capsule starts without it, and the
   receipt names why; a value the scan flagged never ships unblanked.
4. **A new column holds a credential.** Guard: the column-name test fails
   until the column is ruled on, each ruling with its reason.
5. **The copy fills the volume.** Guard: the charge. The estimate is the
   store's live pages; the capsule state volume must keep its 4 GiB floor
   (above compaction's 2 GiB) after the copy and an upper allowance of
   512 MiB; otherwise the capsule starts without the copy and the receipt
   says so. The paper's 23 GB store is the expected refusal. A failed copy
   leaves no file.
6. **The live store is disturbed.** The copy is one read transaction on its
   own connection (writers never wait; the WAL is pinned for its length),
   then a checkpoint, as compaction does. The copy's pragmas and blanking
   run on the copy only. Test: the live store's head and a row are unchanged
   and it keeps accepting writes during and after the copy.
7. **The capsule reads the store's sidecar secrets or telemetry.** Guard:
   only `store-floor.json`, `store-schema.json` and the lease-untrust marker
   are copied; telemetry partitions and image blobs are left out and named.
8. **The in-capsule autoputer reaches a credential or the live computer.**
   Guard: it runs in the capsule's network namespace with no gateway token,
   no signer, no credential envelope, no platform URL; `run.go` starts those
   only when their variables name them, and the capsule environment names
   none. Loopback trust (`CHOIR_GUEST_TRUST_LOOPBACK=1`) is set only on that
   process, inside the capsule, where loopback is the capsule. Test: start
   the autoputer binary over a store copy and a files tree with only the
   capsule recipe's variables; it serves the Files API on loopback and its
   log shows no credential path opened.
9. **The copy's ownership blocks the capsule.** Guard: `computer/`,
   `state.db` and `state.texture/` are chowned to the capsule's mapped uid,
   as the release leaf is.
10. **The two instants are mistaken for one.** The receipt names both: the
    files lock time and the store's event head and sequence.

## Build: S2 review round 1 and S3 decisions

S2 round 1: Devin accept_with_nits, DeepSeek send_back (two blocking).

- A failed link counted as a leave-out while the receipt said built
  (DeepSeek 1): fixed. Any failure to build a kept entry fails the walk and
  the layer is refused; only a file that vanished is skipped. Each link is
  re-checked (same device and inode, regular, `o+r`) and unlinked if the
  entry changed (DeepSeek 7). A mount inside the files root is left out as
  `other_filesystem` (Devin 7a). The walk checks the spawn's context.
- The revert hazard (DeepSeek 2, Devin 1): the `removePrivateTree` change
  is its own commit; the rollback keeps it.
- A refused layer had no receipt (DeepSeek 3): the receipt is now written
  in the upper at `computer/receipt.json` for every spawn that asked, with
  the files and store results and each refusal.
- Not-`o+r` file names reached the capsule in the receipt (Devin 2): they
  are counted, never named.
- Literal leave-out paths (DeepSeek 5): the source leave-out is the
  configured source root (`CHOIR_CAPSULE_SOURCE_ROOT`) relative to the
  files root, and the maildir and the snapshot share `ownerfiles.MailDir`.
- The files root is resolved through symlinks at configuration, and a root
  that overlaps the capsule state dir is refused (Devin 4).
- The receipt records the lock wait as well as the hold (Devin 6).
- The spawn spec is built by one function the test exercises (Devin 5,
  DeepSeek 6).

Taken as residuals:

- **The executor mutex.** Spawn holds it throughout, as it does for the
  source copies; the files walk (about 3,200 links on d@e) and the store
  copy (about 200 MB) run under it (DeepSeek 4). Moving the layer before the
  lock is a follow-up if spawn latency shows it.
- **Egress from a capsule that holds the copy** (Devin 3): a claude-mode
  implementation capsule would hold the copy, the delegate token and
  `api.anthropic.com` egress; it is now withheld the copy (Residuals,
  first item). It is the coordinator's ruling to keep the
  copy and move the delegate behind the gateway
  (`docs/problems/claude-delegate-token-on-three-computers-2026-11-03.md`);
  until then the ring walk does not reach the delegate computers with this.
- **Live-inode side effects** (Devin 7b, c): capsule reads may update atime
  under `relatime`, and link counts rise while a capsule lives. Neither
  changes bytes, mode or owner.

S3 decisions:

- **The disk charge is an admission check.** There is no disk reservation
  in the S4 scheduling work to join (its reservation is host memory). The
  copy needs twice the store's live pages plus a 512 MiB upper allowance,
  and the capsule state volume must keep its 4 GiB floor after it (above
  compaction's 2 GiB); otherwise the receipt says `does not fit` and the
  capsule starts without the copy.
- **The copy's sidecar** carries the floor, the schema receipt, the
  lease-untrust marker and the two vocabulary files that travel with
  published bases (the cutover report arms the write fence on reopen);
  telemetry partitions and image blobs are named as left out.
- **No blanking by column name.** The column-name test rules on all 13
  matches (none holds a credential); the secret-shape scan of every text
  and blob value is the check, blanking each hit to
  `[redacted:<digest prefix>]` under `secure_delete`, then a second
  `VACUUM`. A blanking the store refuses fails the copy.
- **The in-capsule autoputer** is the ordinary binary with
  `RUNTIME_STORE_PATH=/computer/state`, `AUTOPUTER_FILES_ROOT=/computer/files`,
  `AUTOPUTER_PORT=8082` (Vite's `/api` proxy) and
  `CHOIR_GUEST_TRUST_LOOPBACK=1` on that process only, inside the capsule's
  network namespace. `run.go` starts the credential, signer, gateway, file
  sync and capsule paths only when their variables name them; none is set.
  The capsule browser recipe says this; the engineering overlay points at
  it. A local smoke is in the S0 evidence doc.

## Build: review of S2 round 2 and S3 round 1

Devin: accept_with_nits (no blocking finding; "none" judged the honest
security effect: the epoch pushes fixes to hibernated releases, and an older
release lacks the layer). Folded in: `SnapshotTo` refuses the live store's
own directory (1); the source-root containment check uses `pathWithin` (3).
Recorded here:

- **Scan coverage** (4): the secret-shape scan sees cleartext patterns
  only. Sealed private payloads are the safe case (the capsule has no
  privacy key); a future compressed or encoded column is outside it, and the
  five carried sidecar files (floor, schema receipt, lease marker, the two
  vocabulary files) are ids and schema records copied unscanned.
- **The floor watch** (2): `vacuumIntoCopy` watches the live store's volume
  (the pinned WAL); the capsule charge covers the copy's side. They are one
  volume on every guest today.
- **A crash mid-copy** (5) can leave `.state.db.snapshot-tmp`, unblanked,
  in the upper of a capsule that never mounted; the boot-scoped cleanup of
  the state directory removes it with the capsule's other state.

DeepSeek on the same round: accept_with_nits ("none" honest, no epoch
raise). Folded in: a blanking that changes no row fails the copy, and the
second VACUUM runs on any hit (1); a bind mount of the same device inside
the files root is left out by statx mount id (2); a receipt that cannot be
written is logged and the capsule still starts (3); the placeholder digest
is salted per copy (4); the sidecar copy refuses a symlink (6); only an
implementation capsule's recipe names `/computer`, a verification capsule
keeps the mock bullet (7); the recipe says the in-capsule autoputer trusts
any caller on the capsule's loopback, the network namespace being that
boundary (8); the floor is read again after the copy (9). Recorded: the
scan misses double-encoded JSON and base64 or compressed plaintext (5, as
Devin 4 above).
