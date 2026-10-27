# New self-development on b@c is refused behind a rebase nobody performs (2026-10-27)

Status: open; documented before any fix. Mutation class of any fix: red
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
  `…:036a216eb…:1` and `:2` (train 33). No rebase round ran, and
  management settled the owner's round blocked instead of answering the
  notice.
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
