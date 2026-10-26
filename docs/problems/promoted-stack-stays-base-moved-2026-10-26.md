# A promoted stack stays stack_base_moved when main edited the same file (2026-10-26)

Status: fix built on lane contained3 (bc2998cec tests, d14b7d58f code, review 292 tier 2: Devin approve, DeepSeek ship, no false positive found; the two cheap lows folded in). Residuals named by the review: added or renamed paths that main later edited, and a file the patch fills from empty that main then edited, stay exact (safe false negatives; U4 remains the general answer). Class of the fix: red (source
projection decides whether a computer may cast), review tier 2.

## Evidence (staging, b@c, computer-6c50d6e5)

- Window manager rounds 1 to 5 were applied on b@c (selfdev-1380320d at 09:01:34,
  selfdev-d81c1246 at 09:28:18), both based on df5480a77.
- Both were promoted to main byte for byte from the candidates' patches: round 4
  as d550f8edd (train 6, 21c212d0f), round 5 as aa21b024e (train 7). The patch
  applied cleanly onto main; every touched file except one is byte-identical to
  the applied candidate.
- The exception is frontend/src/lib/Desktop.svelte. Train 5 (033dc4d53, the
  prompt-bar continue chip) edited it on main between the candidates' base and
  the promotion. Round 4 adds three lines to it (passing the resize anchor),
  and those three lines are on main.
- b@c refreshed onto 21c212d0f at 09:29 and onto aa21b024e at 09:38, and onto
  9da69a5d2 at 09:52. Every time, observability reported
  `stack_base_moved: the stack is based on df5480a77 but this computer booted
  <base> and the stack's patch is not in it`.
- The cast gate then refused. Reader round 6 (trajectory d4702c90, sent 09:39:25)
  was refused twice by management for "a source mismatch" (09:40:21, 09:40:53).
  No operation opened. b@c cannot self-develop until this is cleared.

## Cause

`sourceproj.Contained` (internal/sourceproj/contained.go) is exact by design:
every path the patch touches must have, in the booted base, the very blob the
patch leaves. Its comment names the trade: a reverse-apply could be fooled by a
duplicated block, so the test gives up "the case where main has since edited
the same file elsewhere", and that false negative "lands safely as
stack_base_moved (an owner act or U4 rebases)". Neither exists: U4 is designed
(docs/design/computer-update-architecture-rnd-2026-10-24.md, "U4 design") and
not built, and there is no owner act that retires a stack. So the safe landing
is a computer that cannot cast until the next promotion that happens to leave
the file byte-identical.

This is not rare. Promotion runs after the apply, and any lane that lands in
the meantime and touches the same file causes it. Here it was one shared shell
file (Desktop.svelte) touched by two unrelated workstreams in one night.

This is the live form of SD-01 (residual register): the interim the register
names, vmctl refusing to refresh a computer with an unpromoted stack, would not
have helped here, because the stack was promoted.

## Fix design (three-way containment)

For each path the patch touches, take three blobs: the patch base's (`B`), the
patch's after-image (`P`), and the booted base's (`B'`). The path is contained
when a three-way merge of the change B to P into B' is clean (no conflict) and
its result is exactly B'. Paths the patch deletes are contained when absent in
B'; paths it adds are contained when B' has the same blob (no merge base). Mode
must match P's.

Why this is sound where reverse-apply is not. diff3 aligns B against P and B
against B'. If B' already made the patch's change at that place, both sides
carry the identical change, which diff3 takes once, and the result is B'. If
B' only has a duplicated block somewhere else, the patch's change is not on
B's side of that alignment, so the merge applies it and the result differs
from B'. That is "not contained", which is the safe answer.

Failure modes, written before code:
- Adjacent edits on main next to a patch hunk conflict and give "not
  contained" (false negative, safe; U4 remains the general answer).
- A partly upstreamed patch: some hunks present, others not. The merge applies
  the missing ones, the result differs from B', so "not contained" (correct).
- Whitespace or line-ending normalization: merge with no normalization, and
  compare raw bytes.
- B's objects missing in the guest repo: no three-way possible; fall back to
  today's exact test (never a looser one).
- Binary files: exact test only.
- Cost: one merge per touched path at boot; bounded by the patch's path count.

Exact matches stay contained as today, so the change only widens from
stack_base_moved to upstreamed, never the other way.

Proof on staging: b@c at its next boot after the deploy reports `upstreamed`
for selfdev-d81c1246 and a Texture request reaches engineering. A regression
test pins the duplicated-block case (a stack whose change is absent but whose
block is duplicated elsewhere stays not contained).

Rollback: revert the commit; computers return to today's exact test, which can
only make a stack not contained again (stack_base_moved), never retire one
that is not upstreamed.

## Staging proof (2026-10-26)

Train 9 (583c93065) deployed. b@c booted it at 10:52 and observability
reported `upstreamed: the operation's patch is already in the booted base
583c93065` for selfdev-d81c1246 at 10:52:05 (it was stack_base_moved on
21c212d0f, aa21b024e and 9da69a5d2). The Reader round 6 request sent at 10:52:35
(trajectory 277d4b93) is the cast check.
