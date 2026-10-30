# A second Freeze replays a changed tree silently (R9-1), 2026-10-26

Status: open; fix design below. Mutation class of the fix: red (Freeze and
candidate identity). Source: [selfdev trace reviews, Round 9](../evidence/selfdev-trace-reviews-2026-10-25.md) (R9-1).

## Problem

Staging, window manager round 2 (selfdev-7fc55caa, 2026-10-26):

- 07:49:59 the engineer froze the candidate: bundle `359777e1`, binary
  reused from the running build.
- 07:50:02 the Freeze result listed 17 `frontend/test-results/` files, left in
  `/workspace/platform` by a Playwright job at 07:03:43.
- 07:50:06 the engineer removed them in the capsule (the capsule was writable:
  Freeze thaws it before it returns).
- 07:50:10 the engineer called Freeze again. It answered the frozen
  operation's handle, bundle `359777e1` and state, which reads as success.
- The Complete then said the files were removed. The candidate under review
  still carries them.

The code: `freezeCapsuleEffectBundle` (internal/agentcore/tools_capsule.go)
answers any Freeze on an operation already frozen, verified or awaiting
approval with that operation's bundle, before it looks at the capsule.
`settleFreezePrecommits` treats that answer as a replay and settles nothing.
The engineer gets no sign that the edit is not in the candidate.

## Why the replay exists

The only caller is the reducer's staged Freeze (`commitFreezeIntent`). A
crash never resumes work, so no boot path redrives a Freeze. A desk does call
Freeze again in two cases that are normal: a cell whose later act failed after
its Freeze committed (`withCommittedFreeze` tells the desk the freeze
committed), and a retried Freeze plus Complete cell. In both the tree has not
changed, and the answer must stay what it is today.

## Design

On the replay branch only, before answering, Freeze compares the capsule's
current tree with the frozen candidate:

1. Read the bundle the operation names (`readBoundSelfDevelopmentBundle`:
   final `bundle.json` from verified onward, else the draft; bound to the
   operation). When both files exist in the frozen state (the finalizer wrote
   `bundle.json` and has not yet renamed the operation), the final bytes do
   not hash to the frozen digest, so a final that does not bind falls back to
   the draft.
2. The frozen tree's digest is the worktree digest of the latest receipt the
   bundle cites (build recipe, tests, toolchain). At the first Freeze the
   certified binder required exactly that receipt to have observed the frozen
   tree, so no new field is stored.
3. Read the live tree without quiescing: a new executor read,
   `InspectGrantedWorktree`, returns the digest of `/workspace/platform` (the
   same `digestCapsuleWorktree` the receipts and the R1 pre-check use), the
   capsule's diff, and the live release tree's files (`ReleaseFiles`: path,
   sha256 of the content and mode, hashed as `StageGrantedRelease` hashes
   them for the bundle's `RuntimeFiles`; the workspace digest does not cover
   `var/lib/artifact/release/`). No quiesce means no thaw, so a capsule that
   the terminal saga froze stays frozen. The read is bound to the
   operation: the handle must resolve to `operation.CapsuleID` (else the
   check refuses), the diff is classified under that resolved ID, and no
   job of any run may be running in that capsule (`RunningJobInCapsule`; the
   run-scoped `RunningJob` check at the top of Freeze covers only the
   current run).
4. Classify the live diff with the same classifier and compare its classified
   groups (not Ignored, not Unknown: `/tmp` and toolchain caches do not enter
   a bundle) with the bundle's groups, by path, kind and mode.
   The live release files are compared with `bundle.RuntimeFiles` by path,
   sha256 and mode; an added, removed or modified file is listed as
   `added|removed|modified release <path>`. A `bin/autoputer` the freeze
   reused from the running build (`ReleaseBinary.Origin` reused) was staged
   beside the capsule's release, so its absence from the live tree is not a
   removal.
5. If the digests, the classified sets and the release files match, replay as today, byte for
   byte. If not, refuse with an error that names the bundle, lists up to 20
   changed paths (added, removed, modified since the freeze) or, when only
   content changed, both digests, and says the frozen candidate is what will
   be reviewed: Complete to end this round; a later change needs a new round.
6. If the comparison cannot be made (bundle not retained, receipts unreadable,
   capsule unreadable), refuse and say so. A refusal changes nothing, so it
   is the safe default: the desk can still Complete.

No schema change, no mode flag. The frozen bundle and the operation are not
touched, and nothing is unfrozen.

### Complete after the refusal

Complete does not consult the operation's state beyond naming its bundle in
the report's evidence refs (`assignmentOutcomeRefs`). The refusal mutates
nothing, so the R1 pre-check and the terminal saga run as before. A
`completed` Complete must cite a receipt that observed the current tree (the
certified rule, unchanged); `blocked` or `failed` bind as observed.

### Is the capsule meant to be writable after Freeze?

Yes, by current design: Freeze quiesces to take the diff and its deferred
`ThawGranted` returns the capsule to active "so the desk can finish". The
terminal saga re-quiesces it at Complete. So after a Freeze the desk can keep
editing, and its Complete then certifies a tree (the assignment's final
subject) that is not the candidate's tree. That split is the deeper defect:
two subjects for one round. Keeping the capsule frozen (read-only) after the
operation freezes would close it, but every capsule verb (`exec`, jobs,
`write_file`, delegate) would need a frozen-capsule answer, and Complete's
receipts would have to come from before the freeze. That is a larger change
and is reported, not implemented here. This fix makes the split visible at
the moment the desk tries to fold a change in.

## Failure modes, written before the code

- An unchanged tree refuses (digest or classifier drift): retries break.
  Test: an unchanged tree replays the exact map.
- A changed tree still replays: the defect stays. Test: a removal after the
  freeze refuses and names the paths.
- Content change with the same path set replays. Test: digest-only change
  refuses and names both digests.
- Ignored or unknown churn outside `/workspace/platform` (`/tmp`, caches)
  refuses a retry. Test: a `/tmp` file added after the freeze still replays.
  (Inside `/workspace/platform` every file counts, ignored or not: the digest
  covers the whole tree, as the certified Complete binding does.)
- The check quiesces and thaws a capsule the terminal saga froze. Avoided:
  the read does not change capsule state.
- The refusal strands the assignment. Test: Complete records a terminal fate
  after the refusal, naming the frozen bundle.
- A same-path rebuild of a release file (`bin/autoputer`) replays: the
  workspace digest and the groups do not see it. Test: it refuses, naming
  `modified release bin/autoputer`; a reused running binary still replays.
- The read answers for another capsule, or for a tree a job of another run is
  writing. Tests: both refuse before the tree is read.
- Both bundle files present in the frozen state fail the digest check and
  refuse an honest retry. Test: it replays.
- The list floods the cell. Bounded at 20 paths plus a count.

## Residuals

- Classifier drift: the live diff is classified by the current classifier
  and compared with the bundle's groups; a classifier change between the
  freeze and the retry would refuse a retry over an unchanged tree. Accepted
  (a refusal is the safe default); not compared by `classifier_digest`.
- The 512-byte `RecordIntentError` cap truncates a long refusal that is
  recorded for the reducer's staged Freeze; the cell's own result is whole.

## Rollback

Revert the fix commit: the replay branch answers the frozen operation
unconditionally again. No data or schema to migrate.


## Merged from docs/problems/freeze-rebuilds-the-binary-after-an-apply-2026-10-26.md (2026-10-26)

**Freeze rebuilds the Go binary for a frontend-only follow-up after an apply.**
Date: 2026-10-26. Status: open; documented before the fix. Mutation class of the fix: orange (capsule Freeze reuse rule, additive).

(Full text in git history at the path above.)
