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

## Status

- Action 1 (failure 4, yellow): landed. The overlay's no-Go frontend build is
  a `choir.Start` job (`sh -c`, 15 minutes); the job's `receipt_ref` is the
  `buildRecipeRef`. The recipe ends with `rm -rf node_modules dist`, so the
  job's start and end worktree digests match and the job receipt is not
  refused as "workspace changed while the job ran". No `memory_mib`: that
  option admits memory beyond the capsule for a browser capture, and an
  `npm ci` does not need it. The Go-change path now says the second (Go
  build) job starts after the frontend job ends (one job at a time). The
  prompts test pins that no recipe runs `npm ci` or `npm run build` through
  `choir.Exec`.

## Fix design: a non-completed fate does not need a certifying receipt (action 2, red)

Written before code. Mutation class: red (run acceptance and completion
evidence).

### The path today (read, not inferred)

`Complete(result, ...)` on an assignment goes through
`recordAssignedEngineeringReportOnce` (`internal/agentcore/engineering_assignment_fate.go`):
a durable freeze intent with the staged proposal, `ExtractGranted` (the
capsule is frozen), the frozen worktree digest, then
`bindFrozenAssignmentExecutionReceipts` ->
`capsule.Executor.ResolveGrantedExecutionReceipts`
(`internal/capsule/executor.go`). That resolver:

1. digests the whole frozen worktree (`digestCapsuleWorktree`);
2. for each cited ref: opens the receipt, binds run, handle, capsule and
   frozen source snapshot (`grantedExecutionBindReason`), mints and persists
   a `capsule-granted-exec` receipt that states `FinalSubjectDigest` =
   the frozen digest;
3. after the loop, refuses unless the chronologically latest cited receipt's
   own `WorktreeDigest` equals the frozen digest ("does not bind the final
   frozen subject").

The same resolver serves `choir.Freeze` (`tools_capsule.go`), where the
check is the point: a freeze claims "this frozen tree was built and tested".
It is applied to every terminal fate, including `blocked` and `failed`,
which claim nothing about the subject. The visual drill's three timed-out
build cells changed the worktree with no receipt, so the latest receipt
(`b437b6e5…`) could never equal the frozen digest (`041e339e…`); the saga had
already frozen the capsule durably, so every later cell was refused ("no
longer executable") and the run ended only by restart.

Zero receipts is not the trap: the caller returns early when the report
cites no commands (`len(refs) == 0`), so `Complete("blocked")` citing nothing
already records. The resolver's own "capsule execution evidence is required"
is unreachable from the fate path and stays for a strict caller.

### Design

A resolver argument, `SubjectBinding`, chosen by the caller from the report's
result and never by the model:

- `certified` (default, the existing behavior): the latest cited receipt must
  carry the frozen digest. Used by `choir.Freeze` and by `Complete` with
  result `completed`. Any result that is not exactly `failed` or `blocked`
  (including an unknown or empty one) is certified: the relaxed path is an
  allow-list of two values.
- `observed` (result `failed` or `blocked`): each cited receipt is still
  bound to the run, handle, capsule and frozen source snapshot, and an
  unknown or foreign ref still refuses (a false citation is not an honest
  report). The frozen subject digest and the latest receipt's worktree digest
  are recorded side by side and are not compared. The granted receipt
  carries `subject_binding: "observed"` and `latest_receipt_worktree_digest`
  (both `omitempty`, so every existing granted receipt keeps its digest).
  The execution attestation carries `subject_binding: "observed"` and its
  `worktree_digest` is the receipt's own observed tree rather than the
  frozen one; `final_subject_digest` is still the frozen subject. The store
  accepts `observed` only on a `failed` or `blocked` report, and a
  `completed` report still requires `worktree_digest == final_subject_digest`
  and no `subject_binding`.

Zero receipts: kept as is. A blocked or failed Complete that cites no command
records its fate; the resolver in `observed` mode returns an empty list for
empty refs instead of "evidence is required". A cited-but-invalid ref still
refuses, which is not an honest-failure trap because the desk can drop or fix
the ref, and the refusal names the nearest issued ref (0a8c77cbc).

### Failure modes

1. A completed claim slips through the relaxed path. The mode is derived from
   `report.Result`, which is in the proposition digest (`ComputeTerminalPropositionDigest`)
   and hence in the report id and freeze intent, so a `blocked` proposal
   cannot be replayed as `completed` (a different digest conflicts at the
   slot gate and at the freeze-intent check). The relaxed path is an
   allow-list of `failed` and `blocked`. The store re-checks the attestation
   against the report's result, so a caller that passed the wrong mode still
   cannot produce a `completed` report with an observed attestation.
2. A blocked fate promoted or reused as evidence of a subject. A blocked or
   failed report whose worktree differs from the binding subject already
   records a content-addressed candidate (`PersistGrantedCandidate`), and the
   reducer maps it to a failed assignment, never `completed`
   (`reducerAssignmentOutcome`); `CertifiesOriginalSubject` requires a
   completed verification. The new attestation says in its own field that no
   receipt certifies the subject, so a reader that wants "a receipt
   certified this tree" must see `subject_binding` empty. Management's Verify
   still re-derives from a Freeze candidate (strict) and never from a
   Complete-time freeze.
3. Replay. Deterministic: the granted receipt ref and the attestation ref
   are digests of the content, which now includes `subject_binding`, so the
   same inputs give the same refs and a replay is idempotent (one existing
   quirk, unchanged: a second grant of the same raw receipt embeds the
   first grant's `granted_receipt_ref` in its `execution`, so its ref
   differs from the first). A `certified`
   and an `observed` granted receipt of the same raw receipt are different
   artifacts; `persistReceiptArtifact` refuses a same-ref different-content
   write, and the refs differ, so neither overwrites the other.
4. Crash restart. No new resumer. The pending proposal and the freeze intent
   behave as today: a crash closes the open work at boot with the fate
   "interrupted by a restart" and does not re-dispatch (CLAUDE.md, Restarts
   End Work). The staged-proposal resume path (`resumeStrandedFrozenAssignmentCommit`,
   planned-restart/watchdog) re-enters the same function with the stored
   report, whose result is unchanged, so it picks the same mode.
5. Freeze versus Complete. `choir.Freeze` stays `certified`, thaws the
   capsule and leaves it live. A desk that froze, then wrote without a
   receipt, then completes `completed` is refused as today; the same desk
   completing `blocked` now records. The Freeze candidate is not touched.
6. The overlay's terminal-fate retry loop. The loop was the overlay saying
   "finish with blocked" while blocked was refused. With the fix a blocked
   Complete binds. Residual, not fixed here: a refusal after the durable
   freeze intent (a cited ref that does not bind) still strands the
   capsule, because the intent is written before the refs are checked and a
   retry with different refs is a different proposition. The correct fix is
   a pre-flight of the cited refs (open and bind, no frozen state needed)
   before the freeze intent. Named residual R1.

### Red ceremony

- Conjecture delta: before, "a terminal report is admissible only if the
  latest cited receipt certifies the frozen tree"; after, "that is required
  of a claim about the subject (completed, Freeze); a report that claims
  failure needs only that its cited receipts are this run's, this handle's,
  this capsule's".
- Protected surfaces: run acceptance and completion evidence (the resolver,
  the attestation schema and its store validation), the engineering terminal
  saga. Not touched: Texture canonical writes, vmctl, auth, gateway,
  checkpoint/route projection, deployment routing.
- Admissible evidence: tests that build the visual drill's case
  (receipts, a receipt-less mutation, then `blocked` binds and `completed`
  is refused) in the capsule package and in the store; a staging drill
  (action 4) is the acceptance proof.
- Rollback: revert the commit. New JSON fields are `omitempty` and read by
  nothing else; records written with `subject_binding: "observed"` become
  unreadable to an older store only for attestation validation, and only on
  failed or blocked reports.
- Heresy delta: discovered, the completion-evidence check conflated a claim
  about the subject with a report of failure (this section); introduced,
  none expected; repaired, that conflation for `failed` and `blocked`.
  Residual R1 (cited refs checked after the durable freeze intent) is
  discovered and not repaired.

- Action 2 (failure 2, red): landed in code. `capsule.SubjectBinding`
  (`certified` default, `observed`), `bindGrantedReceipts` (portable, in
  `granted_receipt.go`), `Executor.ResolveGrantedExecutionReceiptsFor`,
  `assignmentSubjectBinding` (allow-list of `failed` and `blocked`), the
  `subject_binding` field on the granted receipt, the execution attestation
  and the evidence packet, and the store's attestation validation. Tests:
  `internal/capsule/granted_subject_binding_test.go` (the visual drill's
  case with real tree digests: receipts, a receipt-less mutation,
  observed binds and certified is refused), a linux executor test of the
  same through `ResolveGrantedExecutionReceiptsFor` (compiled and vetted on
  linux, not run on this darwin host),
  `internal/store/engineering_subject_binding_test.go` (a completed report
  cannot carry an observed attestation or a mismatched tree),
  `internal/agentcore/engineering_assignment_subject_binding_test.go`.
  Open: R1 (cited refs checked after the durable freeze intent), R2 (the
  content-addressed candidate of a failed report). Not proven until the
  staging drill (action 4).
