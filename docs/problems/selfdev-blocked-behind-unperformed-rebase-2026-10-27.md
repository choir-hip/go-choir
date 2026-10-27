# New self-development on b@c is refused behind a rebase nobody performs (2026-10-27)

Status: open; documented before any fix. **Corrected 11:3x (below): the
rebase rounds did run, and failed; the original claim that none ran was
wrong.** Mutation class of any fix: red
(self-development admission and the U4 rebase path). The owner's citation
change (trajectory 150b7de5…) is blocked by it.

## Evidence (b@c, computer-6c50d6e5…, the round's trace)

- **07:08.** b@c refreshed onto train 33 (036a216eb). FP0 opened attempt 4
  of the citation round at 07:09 (revision 0a4f9e9a…).
- **07:10:13.** The round settled blocked: "Engineering admission was
  refused before an assignment was created: the stack is based on
  d385633b… while this computer booted 036a216eb…, and the stack patch is
  not in that source." No capsule ran.
- The admission rule is `sourceProjectionCastGateFor`
  (`internal/agentcore/source_projection.go:304`): with source projection
  on, an ordinary Cast is refused while the applied stack sits on a moved
  base. Only a rebase Cast (naming the rebase notice, the operation and the
  booted base) is admitted then.
- The applied stack is selfdev-81bbd1dae686002e850c6d5732241117 (state
  applied; trajectory e9777d10…). The computer posted rebase notices for it:
  `rebase-notice:selfdev-81bbd1da…:0609209c1…:1` (train 32) and
  `…:036a216eb…:1` and `:2` (train 33). ~~No rebase round ran~~ (wrong,
  see the correction).
- Attempts 2 and 3 (01:09, 02:21) show the same pair in their trace:
  stack base df5480a7… against booted aa21b024…. So since the 00:30 refresh
  every capsule on b@c has started from a stack that is not on the booted
  base.

## What this means

Every platform deploy moves every computer's booted base. A computer with
an applied self-development stack then refuses all new self-development
until a rebase round runs, and today nothing runs it: the notice is posted
and left. On a computer that takes several deploys a day this is most of
the day.

Possible link to the npm fall-back of attempts 2 and 3: the capsule tree
was the stack (an old base plus the applied patch), not the platform tree
of the booted image. The package files are identical at df5480a7, d385633b
and aa21b024, so the base alone does not explain the `cmp` mismatch; the
applied patch might (not read).

## Options (not taken overnight; the owner's call on their test computer)

1. **Run the rebase round** the notice asks for (U4's designed path). If
   the applied work is already on main (the window manager round 5 from
   b@c is, as aa21b024), the rebase is close to a no-op.
2. **Roll b@c back to the plain release** (`choir self-dev rollback`),
   which empties the stack, so ordinary rounds start from the booted tree.
   That is a forward transaction on b@c's chain; the applied work stays in
   its history.
3. **Design:** who answers a rebase notice. Management, or the Texture
   root desk, should cast the rebase before (or instead of) refusing the
   owner's next request, and a refused round should name the notice and
   the act that clears it.

## Correction (11:3x): the rebase rounds ran and failed; the cause is containment

The 08:1x claim that no rebase round ran was read from the owner's round
alone. The root desk's trajectory (2d29bfc3…) shows the U4 path working as
designed: the root cast management on each notice, and management cast
engineering with a rebase spec. Both rounds failed:

- **selfdev-e8154ce2e363a88daf7304f55b1570c5** (06:59, onto 0609209c1):
  `rebase_content_mismatch: hunk_missing … frontend/src/lib/desktop-motion.css`.
- **selfdev-927abd8cf00b549ef1755829d6eae7e7** (07:46, onto 036a216eb):
  `Freeze refused: this capsule changed no /workspace/platform source, so
  its candidate would release the base tree unchanged`.

**Why: the stack is already upstreamed, and containment cannot see it.**
selfdev-81bbd1da is themes round 7, which FP0 promoted to main verbatim as
0db9ee271 (its patch sha256 34580f37… is the one b@c serves). The two have
the same `git patch-id --stable` (15d0945051…). But main later appended the
idle-pause rule (0096f6494, ad03f5aba) at the end of `frontend/src/app.css`,
exactly where round 7 deleted its trailing block. Reproduced locally:

- `sourceproj.Contained`'s three-way test, `git merge-file` of
  (B' = 036a216eb, B = d385633b6, P = 0db9ee271) on `app.css`, conflicts
  (rc 1): ours is the idle rule, theirs is empty. `desktop-motion.css`
  alone merges clean and is contained.
- So the stack reads `stack_base_moved`, a notice is minted, and the rebase
  round is asked to rebase a change that is already in the base. The
  correct rebase is empty; Freeze refuses an empty capsule, and
  `VerifyRebase` (the same three-way merge) would refuse the base-side
  resolution as `kept_base_unverifiable`/`hunk_missing` anyway. Every
  future notice fails the same way: a deadlock, not a missing actor.

The design named "an edit on main adjacent to a hunk" as a *safe* false
negative of `Contained`. It is not safe: with U4 live, a false negative is a
rebase that cannot succeed, and the computer's self-development stays
refused.

**This is the third round on one mechanism** (U1 exact match, then the
three-way merge of 6bc3d9aa9, now this), so the next fix is at the substrate,
not another content heuristic: a promotion is a commit on main, and git's own
answer to "is this change already upstream" is patch identity (`git cherry`,
`git rebase` skipping applied commits). Proposed: `Contained` also answers
yes when a non-merge commit in `B..B'` has P's stable patch-id. Exact
content and the three-way test stay as they are; patch identity only adds
yeses that a verbatim promotion guarantees. If the guest repository lacks the `B..B'`
history, the answer falls back to today's.

**As built (review 353, Devin and DeepSeek; both found that an exact-reverse
check misses non-exact undos, a merge sharpest of all):** the match counts
only when the content test holds at the matching commit C (the
duplicated-block pin), and then the tree diff C..booted must leave the
change alone: no touched path deleted, re-moded or binary, no line the patch
added removed, no line it removed added back (lines with a letter or digit;
a blank or a lone `}` says nothing, and the evidence has both). A revert,
exact or not, and a merge that drops the change, therefore read as not
contained, which is correct: the rebase that follows re-applies the change,
so it is not empty. Errors other than missing objects propagate. Named
limits: a promotion folded into one commit with another edit of the same
paths, or split over two commits, has a different patch-id and falls back
(the old false negative); the walk is one path-limited `git log -p` over
`B..B'`, only when the content test says no.

Options 1 and 2 above stand as written; option 1 (the owner chose it,
"Rebase") cannot succeed until containment sees the promotion.
