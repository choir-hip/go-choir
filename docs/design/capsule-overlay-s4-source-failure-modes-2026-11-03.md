# Capsule overlay S4: the source tree follows the deploy, failure modes (2026-11-03)

Status: build notes for slice S4 of
`docs/design/capsule-overlay-computer-state-2026-11-03.md` (section 5),
written before the code (CLAUDE.md, Testing). Mutation class of this doc:
green. The change is red: source projection, the Cast gate, the guest
source repository.

S4 deletes `sourceproj.Enabled`, `CHOIR_SOURCE_PROJECTION`, the settings
`source.projection` and `source.rebase_offer`, and the `texture.root_desk`
conjunct of the rebase path. Reconcile then runs on every computer: at boot,
after the base fetch, and at every Cast. It saves before any reset, and it
grafts a stack onto a moved base when the base left the stack's paths alone.
It also fixes RV-03 (residual register 2026-10-26).

## Failure modes

1. **Owner edits in Source/platform are lost.** `ensureHead` runs
   `git reset --hard` over a dirty tree today. Guard: before any reset, the
   tracked changes (work tree and index, staged new files included) are
   committed in a scratch index to `refs/choir/saved/<commit>`, and the
   reset proceeds only after the save is on the tape. Test: a modified, a
   deleted and a staged new file come back from the saved ref.
2. **An untracked file the expected tree tracks is overwritten.**
   `reset --hard` overwrites an untracked file at a path the target tracks.
   Guard: such files (ignored ones too) join the saved commit. Untracked
   files the target does not track are left alone and not saved. Test.
3. **A foreign HEAD becomes unreachable.** A commit made on the computer
   branch by something else, or the creation commit on the first
   projection, is moved away from. Guard: a HEAD no `refs/choir/` ref names
   is kept at `refs/choir/saved/<its commit>`. Test.
4. **A saved ref is overwritten.** Guard: the name is the commit, created
   with `update-ref <ref> <commit> ""` (create only); a ref that already
   holds the same commit is success, any other value is an error. Test:
   the same dirt saved twice gives one ref and one fact.
5. **The save is not on the tape and the reset runs anyway.** Guard: the
   reconcile input carries a save hook; the runtime appends a
   `recovery_recorded` event (`source.projection_saved`, the refs, the kind,
   `From`) and the reset runs only when the hook returns nil. A failed append
   fails the reconcile: the Cast is refused with `source_projection_failed`.
   Test: a failing hook leaves the dirty tree and HEAD as they were.
6. **Saves loop.** Guard: the saved commit is deterministic (fixed identity,
   the HEAD commit's date, a message from its inputs), a save happens only
   before a reset, and the hook is told only of refs it created. Test.
7. **A graft hides a conflict.** Guard: graft only when no path the patch
   touches differs between B and B' (literal pathspecs, so a directory that
   B' put at a file's path counts as a difference). The candidate is still
   built and verified on B'+P'. Test: a touched path changed in B' stays
   `stack_base_moved`.
8. **A directory/file clash at an ancestor.** P adds `a/b` and B' adds a
   file `a`: the touched path `a/b` is equal (absent) in B and B'. Guard:
   the graft's `update-index` refuses the clash without `--replace`, and a
   refusal is "not graftable", not an error. Test.
9. **The grafted commit is not deterministic,** so every reconcile moves
   HEAD. Guard: the commit is `projectionCommit` of the operation on B'.
   Test: two reconciles give one commit and the second changes nothing.
10. **A Freeze from a grafted commit is based on B,** which the host
    builder refuses after the deploy. Guard: the grafted commit's parent is
    B' and its `Choir-Stack-Base` trailer names B'. Test: `Cumulative` from
    a capsule on it returns base B'.
11. **A graft equal to B'.** Guard: reported `upstreamed`, HEAD is B'
    (Contained normally answers this first). Test.
12. **The Cast at `stack_rematerialized` hides the fact that the served
    build is still the stack on B.** Guard: the ordinary Cast is admitted
    and a system observation commitment record, addressed to management,
    names the stack, B, B' and what approving the candidate approves. Its
    id is deterministic per Cast, so a replay records nothing new. Test.
13. **`stack_base_moved` with no live root desk is silent.** Guard: the
    ordinary Cast refusal gains `rebase_needs_root: no live root desk to
    Cast the rebase`. Test.
14. **Stored values of the deleted settings.** A computer may hold
    `source.projection=false` or `source.rebase_offer=false`. Guard: the
    keys leave `KnownComputerSettings`, so nothing reads a stored value and
    a write of the key is refused as unknown. `scripts/proof-u4-rebase`
    stops setting them.
15. **RV-03: a 403 parks the base fetch until the next boot.** A GitHub
    rate-limit or abuse 403 is not "no such commit". Guard: 403 is
    retryable (the back-off schedule); 401 and 404 stay permanent. Test.
16. **Projection on a computer whose base fetch cannot work** (no remote, a
    dev build). The states are `no_booted_base` (nothing moves, no refusal)
    and `base_objects_missing` (Casts refused, as on b@c today). This is
    the intended loud failure; the ring walk starts at d@e.
17. **Binary reuse rides on projection.** The reuse-by-input-diff gate was
    `sourceproj.Enabled`. It is now always on. It compares the running
    binary's commit with the capsule source commit, so a graft (source
    B'+P', binary from B+P) still builds when B' changed a binary input.

## Rollback

Revert. The switches come back, off. Saved refs, `refs/choir/base` and the
`From` on the tape are the handles (`git reset --hard <ref>`).
