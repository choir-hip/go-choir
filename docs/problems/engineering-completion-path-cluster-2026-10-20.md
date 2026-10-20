# Engineering's completion path: a cluster of four failures in one day (2026-10-20)

Root Cause Clustering (CLAUDE.md): four failures in the same subsystem, the
path from engineering's last cell to a terminal `Complete`, seen on staging
today. Written before any further patch.

## What is now proven (deploy 423001678, 12:17 UTC)

The owner's request reaches engineering, and the Claude delegate works end
to end inside the engineering capsule:

- Claude delegate drill, computer-ad1b46d4…, trajectory 72f3631e-…, run
  `run:assignment-b4bb8ad3234ee857827ce09e`. Engineering printed the Cast
  Body, which now carries the Texture request verbatim; it briefed the
  delegate with the exact path, line and test; delegate run
  `d20261020T121809-4458e36c` wrote the file in 7 s (4 turns, $0.033);
  `written_files` reported
  `/workspace/platform/docs/probes/claude-delegate-20261020T121703.md`,
  `exists: true`, 41 bytes; engineering's own `ReadFile` of that absolute
  path printed `claude delegate probe 20261020T121703 ok`.
- Visual drill, computer-7011602c…, trajectory 7ab6038d-…: engineering
  read the request and followed it (precommitted `visual_captured` on the
  drill route, wrote the page, recorded the toolchain).

## The four failures

1. **Retyped receipt ref** (0565 drill, 11:37). `Complete` cited a ref with
   three characters dropped and was refused "executor receipt unavailable"
   without naming the ref; the desk retried blind. Fixed in 0a8c77cbc (the
   refusal names the ref and the nearest issued one); not yet observed.
2. **A blocked Complete cannot bind** (visual drill, 12:17 deploy).
   `Complete("blocked", …)` citing the toolchain cell was refused: "assignment
   command evidence unavailable after durable freeze: capsule execution
   evidence does not bind the final frozen subject: latest receipt observed
   b437b6e5…, frozen subject is 041e339e…". Three timed-out build cells had
   changed the worktree after the last receipt. The next cells were refused
   "assigned capsule obligation is no longer executable", and the run ended
   only by `interrupted_by_restart`. A desk whose mutation failed without a
   receipt cannot report that it is blocked.
3. **A completed Complete never returns** (delegate drill). The `Complete
   ("completed", …)` cell citing three receipts has no `tool.result` 40+
   minutes later; the run stays `running`. Cause unknown: the guest's logs
   are not reachable without SSH (the console log holds 16 lines). Code
   reading: the evidence path digests the whole worktree
   (`digestCapsuleWorktree`) before the receipt loop; the 0565 drill's
   blocked Complete reached the receipt check quickly, so the digest alone
   is not obviously the hang. Hypothesis, not a finding.
4. **The overlay's frontend recipe is a 30-second Exec** (visual drill).
   `rlm_engineering_runtime.yaml` line 56 says a cold `npm ci` is a job,
   never a `choir.Exec` (killed at 30 s); line 58's worked recipe runs
   `npm ci && npm run build` in one `choir.Exec`. Engineering followed line
   58; three attempts hit `failed to read broker response … i/o timeout`
   (the broker read-timeout class, P5 residual).

## Assessment

- 1 and 4 are symptoms (a message, a recipe) with cheap, contained fixes.
- 2 is a substrate defect in the completion contract: the evidence check
  that guards a *completed* claim (the latest cited receipt certifies the
  frozen subject) is applied to every terminal fate. A blocked or failed
  report claims nothing about the subject, so binding it to the subject
  makes honest failure impossible after any receipt-less mutation, and
  the overlay's terminal-fate retry then loops. The fate-sharing standing
  question applies: the run's ability to end depends on evidence that its
  failure destroyed.
- 3 is unexplained and blocks every completed engineering assignment if it
  reproduces. It needs observation before a patch: a guest-readable trace
  of where `Complete` spends its time (an observability field or a
  host-readable log), per the no-SSH operability standing question.
- No existing replacement for the completion evidence path was found
  unwired. The release witness (S6-0) reads the same receipts but does
  not decide.

## Next actions, in order

1. Overlay: the frontend recipe becomes a job (yellow).
2. Completion contract: a non-completed fate binds its cited receipts to
   the run, handle and capsule only, and records the frozen subject digest
   beside them without requiring the latest receipt to certify it (red:
   run acceptance; design and failure modes first).
3. Observation for 3: where a `Complete` cell is, while it runs, readable
   from the host (red: observability on the guest runtime); then the fix.
4. Rerun both drills.

Mutation classes as above. Rollback: revert each commit.
