# Management bound a Cast to the wrong parent record when two Texture requests shared one inbox pass (2026-11-03)

Status: open. This is a finding from traces: management run d918b2c0 on b@c, and the assignment evidence. Mutation class of a fix: orange (management desk code or prompts) or red (if the platform validates the binding).

## Evidence

- Two owner revisions reached b@c's management inbox 13 seconds apart:
  - record 281, Files (new), trajectory f2643453, at 00:16:58Z;
  - record 282, the Slides (new) security round, trajectory 891e8791, at 00:17:11Z.
- Management's Cast cell at 00:17:40Z chose its parent record with a loop that kept the last Texture cast in the inbox: `for ... if r.Kind=="cast" && HasPrefix(r.From,"texture:") { recID = ... }`. That record was 282, the Slides round. The cell then Cast a Files (new) objective with 282 as its parent.
- The platform bound the Cast faithfully. assignment-47bd54b1 landed on the Slides trajectory with a Files objective. Its engineer compared the request with the tree, refused to guess, and asked management (rlm:ask:284 at 00:19:33Z). The capsule was revoked at 00:20:03Z, before management's next run (00:19:50 to 00:21:06Z) answered.
- The correct Files Cast came later, from run 70cdc230, on f2643453.

## What it shows

- The desk picks the parent record with ad hoc cell code. When a pass holds two requests, the code silently takes whichever record was last.
- Nothing checks that a Cast's objective answers its parent record's request.
- An engineer's question can outlive its capsule. The answer arrived after revocation.

## Candidate fixes (not decided)

1. The Cast tool takes the parent record explicitly and refuses an implicit one when the inbox holds more than one open Texture request.
2. The platform checks that a Cast's trajectory matches its parent record's trajectory.
3. An open engineer question holds the capsule's revocation for a bounded time.
