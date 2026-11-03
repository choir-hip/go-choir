# Self-development round blockers: clustering assessment and failure modes (2026-11-03)

Status: open; fixes land on branch `selfdev-round-blockers`. Mutation class of
this doc: green. Evidence and numbering are the problem record
`docs/problems/selfdev-arm-rounds-fail-at-freeze-and-capsules-lack-real-content-2026-11-03.md`
(problems 5, 8, 9, 10, 11, 12, 13) and the per-arm round docs
`docs/evidence/selfdev-files-round-*-2026-11-03.md` (FP0's docs tree). Traces
below are metadata only: guest console log lines on Node B and code reads.

## Clustering assessment (CLAUDE.md "Convergence Before Patching")

Seven symptoms in one subsystem, the self-development round after Freeze and
across a deploy. They share three causes, and two of them have an existing
mechanism that is built but not wired.

1. **A reader still depends on a default-off switch, or on the root desk that
   only a default-off switch creates** (10, 13; 12's visibility half).
   - 10: the U4 rebase notice is addressed only to the root Texture desk
     (`rebaseRoot`, `internal/agentcore/rebase_notice.go`). The root master
     exists only with `texture.root_desk` on, which is off on the arms (and on
     every computer by default). S4 deleted the switch conjunct from the rebase
     path, but not the dependency. Trace: claude, pool and deepseek all logged
     `rebase notice: none minted: rebase_needs_root: no live root desk to Cast
     the rebase` five minutes after the 07:07-07:08Z boot (claude 07:12:58Z,
     pool 07:12:49Z and 07:14:56Z, deepseek 07:12:40Z). The 06:07Z claude boot
     ran 2795b8ee, which predates the log line; no notice was minted there
     either. The hypothesis in problem 10 ("U4 mints no notice when the stack
     diverged") is refuted: the stack was `stack_base_moved` after the base
     fetch (2795b8ee and 29d47c215 both changed `frontend/src/lib/app-sdk.ts`,
     which every stack's patch touches, so the S4 graft correctly refused), and
     the timer fired; it found no root.
   - 13: owner-act records (approve, apply, reject, abandon) into the
     candidate's Texture exist (`selfdev_owner_act_wake.go`), behind
     `texture.owner_act_wakes`, off by default and off on the arms. The
     registry row already says "retire: delete the key and its readers".
   - Existing replacement, unwired: `ownerActTextureInbox` resolves the Texture
     of the document the operation's trajectory binds. The rebase notice can
     use it when no root is live; the store's admission (notice inbox equals
     the parent Cast's sender) holds unchanged, because that Texture is the
     sender of its own Cast.
2. **The round assumes one step: one applied operation per package release,
   and a computer on the host's newest base** (11, 12).
   - 11: the publisher's "own change only" check
     (`internal/pkgrelease/publisher.go`) accepts only a parent subtree equal
     to the stack base's or the track head's, so a package-confined round
     stacked on an earlier unpublished round (which changed app-sdk.ts) can
     never publish, and nothing after it can either.
   - 12: the host build (`internal/hostbuild/service.go`, `baseRefusal`) builds
     only on the host's current guest image, so during a ring walk every
     computer not yet refreshed gets `base_moved`, and under standing
     consensus that is a reject. Holding until the refresh does not work: a
     frozen, undecided capsule counts as live, so the computer does not
     refresh while it waits, and after a refresh the candidate's base is no
     longer the booted one (`base_moved_since_freeze` at apply). Existing
     replacement, unwired: Node B retains every guest release a running
     computer needs (`/var/lib/go-choir/guest-releases/<id>/`, the retention
     rule M30), each with its manifest and store disk. 516ec805 (2795b8ee),
     the base deepseek booted at 07:02Z, is retained there.
3. **The carry between Texture, management and engineering has a bound and
   a selection per hop** (5, 9, and 8 as a model behavior on top).
   - 9: engineering cuts the Texture request at 16 KiB
     (`engineeringRequestBodyMax`); Texture accepts 64 KiB of owner words
     alone (`castOwnerTextMax`).
   - 5: the runtime attaches only the owner revisions the Cast names; Texture
     names only the newest, so the request revision with the list is dropped.
   - 8: a turn woken by an owner revision commits a Write and no Cast
     (hypothesis below).

Substrate verdict: none of the seven needs a new substrate. Five are fixed by
wiring what exists (10, 12, 13) or by one bound (9, 5); 11 is one rule of the
publisher, widened within the Library design.

## Failure modes (written before code)

### 10: the rebase notice reaches the stack's own Texture when no root is live

1. The notice goes to a Texture that cannot answer (archived document, a
   trajectory not live or cancelled). Guard: the fallback requires the
   document bound to the operation's trajectory, not archived, the trajectory
   live and not cancelled; otherwise `rebase_needs_root` as today.
2. Root and document Texture both live: two candidates for the notice.
   Guard: the root is preferred, as today; the fallback runs only when no
   root is live.
3. The addressee flips between attempts (a root appears). Guard: the
   existing `rebaseAttemptClosed` closes an attempt whose inbox is not the
   current addressee, and the next attempt goes to the new one; attempts stay
   bounded by `RebaseNoticeMaxAttempts`.
4. The operation has no lifecycle trajectory (genesis, rollback) or no
   document. Guard: the resolver answers "not bound", and the refusal keeps
   `rebase_needs_root`.
5. Admission refuses the answering Cast because the notice's inbox is not
   the parent Cast's sender. Guard: the document Texture is both; a test
   drives the store admission with the fallback inbox.
6. The ordinary Texture prompt does not know the notice. Guard: one prompt
   line, like the root's: make the one Cast the notice gives, once per
   notice, copying the id exactly. The notice body already carries the
   exact `choir.Cast`.
7. A crash boot acts on a pre-boot notice. Unchanged: the timer is in
   memory, the store refuses a notice committed before a markerless boot.

### 11: a package release over earlier unpublished rounds of the same stack

1. The cumulative release carries another computer's work as ours. Guard:
   the earlier changes must come from this computer's own applied stack: the
   source before the operation is an applied projection commit with the same
   stack base as the operation's patch (`packageSourceBefore`), so the
   package subtree from base to after is this stack's.
2. The track has a head the stack does not descend from (another computer
   of the same owner published it). Guard: a head release is the parent only
   when its tree is the package subtree at a point of this stack (the base or
   an applied projection commit on the same stack base); otherwise
   `package_parent_unpublished` as today. Corpusd still refuses a stale
   parent (`track_moved`).
3. The operation's own change leaves the package. Unchanged: confinement of
   the operation's own delta (before to after) still refuses
   `package_change_not_confined`.
4. An earlier operation applied between two projections left no ref, so the
   source before is unknown. Unchanged: refused (review 274 F2).
5. The release's package files depend on a non-package change an earlier
   round made (app-sdk.ts). Only the package subtree travels, as the design
   states for any release (3.2 check 3); the adopter's build and probation
   catch a missing export (L2b). The review patch shows every carried line.
6. `changed_paths` claims the operation's paths while the release carries
   more. Guard: on a cumulative release it lists the package paths that
   differ from the parent, which the review patch shows.

### 12: the host builds on the base the computer booted

1. The host builds on a base no computer runs, or a base it cannot verify.
   Guard: the base is looked up by the computer's booted manifest digest
   among the host's current guest image and its retained releases, by the
   sha256 of each `guest-image-manifest`; a digest none of them has is
   `base_moved` as today.
2. A retained release is layered (no `storedisk.erofs`). Guard: only a
   single-image release with a store disk is used; a layered one is skipped
   (as the host's own config is single-image), and the build is refused
   `base_moved` when nothing else matches.
3. Retention deletes the release mid-build. The build fails; a release a
   running VM needs is never deleted (M30), so this is a computer that has
   already refreshed. The outcome is a reject on a stale base, as today.
4. A deploy lands mid-build. Unchanged: the builder's resolved digest must
   equal the job's.
5. The patch base is not the booted base. Unchanged: `base_commit_mismatch`
   against the resolved base's build commit.
6. The reject reason stays in corpusd's log. Guard: the guest records the
   dissent reason on the operation (`last_intent_error`) before it decides,
   and the reject reaches Texture with its reason (fix 13).

### 13: Texture hears every decision after Freeze

1. A crash boot's settlement wakes Texture (a restart starts work). Guard:
   only live call sites mint, as today; the fail record is minted by the
   live materializer and verify paths, never by the boot reconciler.
2. A replayed act mints twice. Guard: the record id is per operation and
   act.
3. Deleting the switch breaks a computer with the key stored. Guard: an
   unknown stored key is ignored on read (the retired-key behavior); the key
   leaves `KnownComputerSettings` and the registry.

### 9 and 5: one bound and the whole owner history on the carry

1. Engineering still cuts a request Texture accepted. Guard: one constant
   bounds Texture's Cast packet to management and engineering's view of it;
   a Texture Cast over it is refused at reduce, with the size, before it
   commits.
2. The attached history overflows the bound. Guard: the named revisions
   are required (refused when they alone exceed, as today); the other owner
   revisions of the trajectory are added newest first while they fit, and
   the ids left out are listed on the packet.
3. A superseded direction misleads the engineer. Each attached direction
   carries its time and kind, newest governs (the Texture prompt), and the
   order is oldest to newest.

## Problem 8: hypothesis only

The committing cell in each dropped turn called only `choir.Write`
(receipts `rlm:texture_turn:0`, no `rlm:cast:N`). Hypothesis: a turn woken by
an owner revision reads that revision as an edit of the document and treats
the Write as the whole answer; the prompt's "a cast to management carries
the owner's request" is phrased as what a Cast contains, not as an
obligation a revision creates. Nothing in the turn contract checks it. A
contract fix (a turn woken by an owner revision on a trajectory with no live
assignment commits a Cast, an Ask or a stated refusal; a commit reason that
claims a send must match an `rlm:cast` receipt) is orange and not built here.

## Ledger

One line per defect, with its fix.
- 10 (2026-11-03): problem: a stack_base_moved arm got no rebase notice; evidence: `rebase notice: none minted: rebase_needs_root` on claude, pool and deepseek 07:12-07:15Z (no root desk; texture.root_desk off). Fix: the notice goes to the stack operation's document Texture when no root is live (`rebaseTexture`); the Cast refusal names it; the Texture prompt knows the notice. Residual: owner rollback still refuses the genesis baseline.
- 11 (2026-11-03): problem: no round stacked on an unpublished round can publish; evidence: pool selfdev-c4ac7712 package_parent_unpublished 06:43Z and 07:15Z (parent selfdev-1d052791, which changed app-sdk.ts). Fix: the draft carries the stack's cumulative package change, from the base (no track head) or from a head that is the package at a point of this stack; the operation's own change must still be confined; only the package subtree travels.
- 12 (2026-11-03): problem: under standing_consensus a candidate frozen during a ring walk is rejected base_moved, and the reason is only in corpusd's log; evidence: deepseek selfdev-4ee90af9 rejected 07:05:10Z ("the computer booted base 516ec805...; the host's deployed guest image is 235acb42..."); 516ec805 is retained at guest-releases/9g59.../. Fix: the host builds on the computer's booted base when it is the deployed image or a retained single-image release; the dissent reason goes on the operation (last_intent_error) before the reject, and to Texture with fix 13.
- 13 (2026-11-03): problem: Texture never learned of decisions after Freeze; evidence: claude r3 applied 05:35Z, Texture later said "the owner should apply it"; deepseek r4 rejected 07:05:10Z, Texture said "verified and frozen" at 07:06:32Z; owner-act records were behind texture.owner_act_wakes, off. Fix: the switch is deleted (key, reader, registry row); a live apply that ends failed mints a `fail` record with the cause.
- 9 (2026-11-03): problem: engineering read 16 KiB of a Texture request Texture accepted at up to 64 KiB of owner words; evidence: luna assignment-24a775b8 (06:08Z) saw 16,384 of 20,923 bytes and froze a partial build. Fix: one bound, textureCastPacketMax (96 KiB), for a Texture Cast's whole packet at reduce and for engineering's view.
- 5 (2026-11-03): problem: a Cast naming only the newest owner revision dropped the earlier request with the list; evidence: claude r3 Cast tray-2 (04:41:57Z) named only 27c25e19, not 91b253a0. Fix: the runtime also attaches the trajectory's earlier owner words (no newer than the newest named revision, newest first within 8 revisions and 64 KiB; the rest named under owner_directions_omitted).
