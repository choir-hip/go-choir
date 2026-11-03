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

## Clustering assessment 2: the rebase path (problems 10, 14, 16)

Three rebase-path defects in an hour (10: no notice without a root; 14: the
content check refuses a hunk the new base carries; 16: pool's notice id
reached engineering admission as
`rebase-notice:selfdev-rebase:selfdev-c4ac7712…:eb1fb6408…:1`). They do not
share one cause. 10 was an addressee (fixed). 14 is the content verifier's
model of a rebase: it knew only "P survives", never "P is already upstream".
16 is the carry: the notice's identity travels from the notice record to the
Texture's Cast packet, to management's spec and its objective, and each hop
is a model copying a string out of prose (the notice body prints the Cast to
make; the Texture prompt says "copy the notice id exactly"). Admission then
compares the strings byte for byte, and the timer's answered-check does too,
so one bad copy refuses the rebase and also hides the answer from the timer,
which then waits out the 30-minute window. Claude's and deepseek's ids were
copied correctly; pool's Texture added its sender inbox (`selfdev-rebase:`)
as a segment. That is a model behavior on a substrate that asks a model to
carry an identity.

Substrate question: should the notice travel as a typed record id end to
end? Yes. The notice record is the authority; a desk names it, the runtime
resolves the naming to the record and writes the record's own id:
- Texture's Cast: the reducer resolves `rebase_notice` (the last three
  segments: operation, base, attempt) to a notice record on the tape sent
  to this Texture, writes that record's id on the packet, and refuses the
  cell when none matches, so the model learns at once;
- management's Cast: the runtime takes the notice from the parent Texture
  Cast when the spec names none, resolves one it does name, and fills
  rebase_of and base from the record and the id into the objective when
  missing; nothing a model types has to match byte for byte;
- admission and the timer compare resolved ids, so a tape that already
  holds a mangled parent (pool's) is read as the notice it names.
A refused rebase re-mints promptly: the timer looks again every stay-up
interval (5 minutes) instead of once per answer window, so a closed
answering round is followed by the next attempt within minutes.
14 is not covered by this; it is a separate rule in the verifier and lands
as its own commit.

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
- 15 (2026-11-03): problem: an inline comment after a model-policy value became part of the value; evidence: luna engineering `reasoning = "high"  # owner-edited: ... (2026-11-03)` reached chatgpt as `hig...03)`, 400 on every call 11:22-11:28Z. Fix: the policy parser strips unquoted `#` comments (quotes respected), and a reasoning effort outside none/minimal/low/medium/high/xhigh/max is dropped to the default effort with a log line and a `llm_policy_error` diagnostic.
- 14 (2026-11-03): problem: a rebase whose candidate drops a hunk the new base already carries fails Freeze with hunk_missing; evidence: claude selfdev-22655878 failed 11:15:12Z on frontend/src/lib/app-sdk.ts (round 3 added the auth and media re-exports; eb1fb6408 carries the same names from e5915e0ca in another shape and place, so the 3-way merge re-adds them and the candidate, correctly, does not). Fix: a missing pure-addition hunk is upstreamed when B' carries its content (all its code tokens are in lines B' added to the path, its string literals are in B', and it brings a word B lacked); the candidate must then pass the conflict rules against P without those hunks, on conflicting and clean paths alike; the hunks are recorded on the verdict, the lineage receipt, the source_projection_rebased fact, the owner's apply record and Freeze's result (`rebase_upstreamed`). Residual: a hunk whose meaning is its place in the file, with every token in lines main added elsewhere, can be dropped (visible on the record).
- 14 (2026-11-03): problem: a rebase whose candidate drops a hunk the new base already carries fails Freeze with hunk_missing; evidence: claude selfdev-22655878 failed 11:15:12Z on frontend/src/lib/app-sdk.ts (round 3 added the auth and media re-exports; eb1fb6408 carries the same names from e5915e0ca in another shape and place, so the 3-way merge re-adds them and the candidate, correctly, does not). Fix: a missing pure-addition hunk is upstreamed when B' carries its content (all its code tokens are in lines B' added to the path, its string literals are in B', and it brings a word B lacked); the candidate must then pass the conflict rules against P without those hunks, on conflicting and clean paths alike; the hunks are recorded on the verdict, the lineage receipt, the source_projection_rebased fact, the owner's apply record and Freeze's result (`rebase_upstreamed`). Review (3 rounds) added: values must stand on one line the new base added beside every word of their line, and comment text the new base added is never evidence. Residuals (visible on the record): a value-free hunk whose tokens all stand in lines main added elsewhere, or a line after a closed `/* */` on the same line, can be dropped; a hunk's own comment lines drop with its code; a hunk that brings no word B lacked is refused (safe direction).
- 16 (2026-11-03): problem: pool's rebase notice id reached management as `rebase-notice:selfdev-rebase:selfdev-c4ac7712…:eb1fb6408…:1`, admission refused it and the right id, and no rebase opened; evidence: pool trajectory 5dc6a6ff, management's report 10:50:16Z; claude's and deepseek's ids (10:52Z) were copied right. Fix: the notice travels as the record's own id (clustering assessment 2): the Texture reducer resolves a Cast's rebase_notice to a notice on the tape sent to that Texture and writes its id, or refuses the cell; management's Cast takes the notice from its parent when the spec names none, resolves one it names, and fills rebase_of, base and the objective from the record; admission and the timer compare resolved ids; the timer looks again every stay-up interval, so a refused rebase re-mints within minutes, not after the 30-minute window.
- 19 (2026-11-03): problem: TestOwnerActWakeHasNoSwitch raced in CI (376636a90, race shard 7): newP4Fixture wrote rt.selfdevControl (standing_consensus_e2e_test.go:267) while a reconcile drain started by the genesis appends' post-commit observer read it (self_development_materializer.go:394); test-only, production sets these fields before boot. Fix: the derivable fixture's observer drains under fx.reconfigure and late wiring goes through fx.configure (newP4Fixture, the host-build fixture, the realization-id overrides).
- 20 (2026-11-03): problem: a tape read during seal-time compaction failed as corruption; evidence: CI 376636a90 race shard 7, TestCompactConcurrentReaders: "index seg-00000004.idx has no segment file". Finding, not hypothesis: compaction renames the copy over the segment, and a readdir across that rename may list neither name (btrfs stops at the index seen at open; the replacement gets a newer one); corpusd runs StartSealCompaction while it serves reads (internal/platform/tape_segments_write.go). Fix: Reader.list confirms an index's segment by its path (rename keeps the path resolving) before it calls the pair corrupt; a genuinely missing or non-regular segment stays ErrCorrupt (C4b). CI runs tests with TMPDIR=/dev/shm; the window is opendir to getdents (the cutoff is taken at open), so a twelve-entry directory hits it; not reproduced on the agents VM (kernel 6.18 tmpfs and ext4, 100 runs before the fix). Residual (review, both seats): the symmetric miss (a seal's new idx name skipped) reads correctly but can refuse OpenView and Appender.begin as "unsealed below the highest"; a pair missed whole can answer ErrNotFound; follow-up.
