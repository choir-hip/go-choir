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
   certified this tree" must see `subject_binding` empty. Not changed, and named
   residual R2: a failed or blocked report whose tree differs from the
   binding subject already persists the content-addressed candidate
   artifact and lists it in the report's outcome refs
   (`assignmentOutcomeRefs`), so management can name it. That candidate
   carried no receipt certification before this change (it is the frozen
   tree's content address), and promoting it still needs management's
   Verify and the owner's approval; the fix makes more failed reports
   reach that point, not a new kind of claim. (This replaces an earlier
   sentence here that said Verify only reads Freeze candidates; that was
   not checked and is not claimed.)
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

## Failures 5-7: the S6-0 go-served drill (added 2026-10-20, 12:52-13:06 UTC)

Evidence: deploy 423001678, computer-bba615665b4cfe16f40616f1a4d3d13a,
trajectory 8e131085-…, run `run:assignment-e5d0e9a3aadc9717cd4297dc`; the run
was cancelled at 13:06 by the drill's stop. Recorded before any fix for them.

5. **The capsule seccomp denies the scheduling reads Node needs.** The
   frontend build cell (the overlay's old `npm ci && npm run build` in one
   `choir.Exec`) failed with `[mutex.cc : 955] RAW: pthread_getschedparam
   failed: 1` repeated. `internal/capsule/seccomp.go`'s allow-list has
   `sched_yield`, `sched_getaffinity` and `sched_setaffinity` but not
   `sched_getparam` or `sched_getscheduler`, which `pthread_getschedparam`
   calls; the default action is EPERM (errno 1, the "failed: 1"). This is
   P5 residual item 3 of
   `p5-go-served-0825-engineering-never-freezes-2026-10-15.md` (named there
   twice, never done). It is a symptom contained to two read-only entries.
6. **A failed build reported as completed.** After the failed build cell,
   engineering ran only the copy step (`dist` did not exist) and later told
   management "frontend build completed". The old recipe invited it: build
   and copy were separable cells. Task-1's recipe is one job (`&&` chain)
   that fails as a whole; what is still missing is the rule that a build is
   not reported done without its job's exit code 0.
7. **A running job treated as a blocker.** Engineering started the Go build
   job correctly, polled it twice while it ran (the overlay says do not
   poll), then called `Complete("blocked", "Go build job is still running")`
   instead of ending its turn to be woken. That is the same wrong reading
   as the 0565 drill's ("a refusal you have not yet tried to fix is not a
   blocker"): a job in flight is not a precondition that cannot be met.

**Failure 3, further evidence (hypothesis, not a finding).** That final
`Complete("blocked")` also has no `tool.result` before the run was
cancelled at 13:06. It may simply have been cut off by the stop (the wait
was at most the time between the call and 13:06, not known from the
record), so it is weak evidence; but it is a second Complete cell with no
result, and it is a `blocked` one. If it hung, the hang is not specific to
`completed`, not specific to the delegate, and not in the receipt-binding
path alone: a blocked Complete with no cited execution refs skips the
receipt resolver entirely (`bindFrozenAssignmentExecutionReceipts` returns
at `len(refs) == 0`), so what it shares with the completed one is the
freeze, the revoke and the store commits. That narrows the hang to the
terminal saga's physical steps (freeze, digest, revoke, destroy) and the
store, away from the evidence function.

### Fix design for failure 5 (red: capsule isolation, seccomp)

Mutation class: red (capsule isolation boundary). Add `sched_getparam` and
`sched_getscheduler` to `capsuleAllowedSyscalls`.

- Conjecture delta: before, "a workload that reads its own scheduling
  parameters is a risk"; after, "reading scheduling parameters discloses
  nothing the workload does not already hold (its own policy and priority)
  and cannot change anything". The setters (`sched_setscheduler`,
  `sched_setparam`, `sched_setattr`, `sched_set_deadline`-class) stay
  denied, and the test pins that they do.
- Protected surface: the workload, broker, L2 and delegate seccomp filters,
  which all derive from `capsuleAllowedSyscalls`. A capsule that could
  already call `sched_setaffinity` gains two read-only queries.
- Failure modes: a setter slipping in (pinned by test); the broker's
  stacked-filter rule (a child cannot gain what its parent's filter
  refuses): the broker filter is built from the same list, so jobs and
  session workers get them too; the delegate filter is a superset of the
  same list; TSYNC and filter ordering are unchanged.
- Admissible evidence: a linux test that loads the workload filter and
  calls both queries successfully while a setter is refused. It cannot run
  on this darwin host; it is compiled and vetted for linux, and the
  staging drill (re-run of the frontend job) is the acceptance proof.
- Rollback: revert the commit. Heresy delta: discovered, none new;
  repaired, P5 residual item 3.

## Failure 3: where a Complete can wait without end (code reading, action 3)

Read, not observed: the path from the Complete intent to the tool result,
and every wait on it. This is a ranked list of hypotheses; none is confirmed,
none is patched. Facts first.

**What bounds the path.** `capsule_go_eval` (`tools_capsule.go`) runs the cell
through `Executor.GoEval` with the activation's context (60 minutes,
`provideriface.DefaultActivationBudget`) unless the cell passes `timeout_ms`;
the broker client sets its read deadline from that context's deadline
(`broker_client.go` `call`, 30 s only when there is none). `commitTray`
(`rlm_reduce.go`) runs `commitCompleteIntent` under a 3-minute context
detached from cancellation. Inside it: the terminal saga in
`recordAssignedEngineeringReportOnce` (`engineering_assignment_fate.go`):
freeze intent (store), `ExtractGranted` (quiesce: wait for in-flight broker
ops to drain, then cgroup freeze), digest of the whole worktree, freeze
receipt, store disposition, receipt binding, candidate export,
`revokeAssignedCapsule` (store, `ForceDestroy`, orphan cleanup, store), then
the report commit (store) and precommit settlement (store). Every capsule
wait in that list takes the context (`Quiesce`, `setFrozen`, `WaitEmpty`,
`waitCapsuleProcess`, the digest walk). So a 40-minute silence cannot be one
of those ctx-bounded waits expiring late; it needs a wait that ignores the
context, or a wait outside the 3-minute scope.

**There is no host-side wait on the delegate.** The delegate server and its
process group live in the capsule (`cmd/capsule-broker/delegate.go`,
`internal/claudedelegate/server.go`); the host reaches them only through the
cgroup (freeze, kill, wait-empty). Nothing in the freeze/complete path waits
on the delegate socket, its supervisor, or its pipes. (Inside the capsule,
`supervise` waits for the stdout/stderr pumps before `cmd.Wait`, so a
claude descendant that keeps the pipes open leaves the delegate status
`running` after claude exits; that affects `choir-delegate wait`, not the
host.) The new evidence (failure 7: a hung `blocked` Complete, no delegate
involved) also points away from the delegate.

Ranked hypotheses:

1. **The store's engine lock is not context-aware and a stall on it is
   invisible.** `objectgraph.engineMutex.Lock`
   (`internal/objectgraph/engine_mutex.go`) is a plain `sync.Mutex`; no
   context can interrupt a wait on it, so the saga's six to eight store
   calls can outlive the 3-minute scope by any amount if one holder is
   wedged (a stalled Dolt call, or a holder waiting on something that waits
   on the saga). `EngineMutexStats` records a wait only after it ends, so a
   wait that never ends shows nothing. Shared by the completed and blocked
   hangs. Likelihood: highest of the code-visible candidates. Check: the
   new `engine_mutex_live` field.
2. **The executor lock is held across teardown.** `Executor.destroy` holds
   `e.mu` (write) across process wait, `unmountCapsuleRoot`, cgroup kill
   and wait, `teardownReclaim` (a synchronous `memory.reclaim` write the
   code itself says runs "under the executor lock"), `Cgroup.Delete`, and
   `removePrivateTree`, several of which take no context; everything that
   needs `e.mu.RLock` (`resolveOne`, `OpenExecutionReceipt`,
   `AssignmentHandle`, job polls) queues behind it. A claude-mode capsule
   has up to 3 GiB and a larger tree to reclaim and remove. Shared by both
   hangs. Check: the new `capsule.destroy.*` steps.
3. **The cell itself never returned from the broker.** For a cell with no
   `timeout_ms` the only bound is the 60-minute activation deadline, and the
   broker client holds its connection mutex (`connMu`, not context-aware)
   for the whole exchange, so every other broker call queues behind it,
   including `Quiesce`'s drain. A silent run that stays `running` for 40+
   minutes and is ended by an external stop is consistent with this. Check:
   the new `cell.go_eval` step. Not excluded for the first hang: the cell
   text is not in the record I have. Likelihood: medium.
4. **A freeze that never reports frozen.** `Capsule.Quiesce` holds the
   capsule's write lock while it waits for in-flight operations and
   `cgroup.freeze` to settle. Context-bounded (3 minutes), so it would end
   in an error and the retry watchdog, not a silent hang. Low as a cause of
   an unbounded wait; it explains slow Completes. Check: `capsule.quiesce`.
5. **The records route before the Complete loop** (`commitRecordsRoute`)
   runs under the cell's 60-minute context, not the 3-minute one. Only if
   the cell staged other acts with the Complete. Low.
6. **The digest walk** reads every regular file under `/workspace/platform`;
   special files are refused (an error, not a block) and the read loop
   checks the context only between reads. Low.

**What I changed for it.** No definite unbounded wait was found that a
bounded deadline would fix without guessing (the candidates are a
non-context-aware mutex used by every store call, and teardown steps whose
bounds are a design question), so no behavior was changed. Instead
(red: observability on the guest runtime, read-only fields only):

- `internal/steptrace`: named in-flight steps with start time and elapsed
  time, plus the last 64 finished steps with duration and error.
- Instrumented: the cell (`cell.go_eval`, `cell.reduce`), the Complete
  intent (`reduce.complete_intent`), every physical and store step of the
  terminal saga (`complete.*`), and the executor's quiesce, digest and
  teardown steps (`capsule.*`).
- `/api/runtime/observability` carries `steps` (in-flight and recent) and
  `engine_mutex_live` (the current holder's call path and age, and every
  waiter with its wait so far). `choir trace` already reads that route; no
  SSH.

**What the next drill should show.** For a hung Complete, one `steps.in_flight`
entry names where it is stopped and since when; if it is a store call,
`engine_mutex_live` names the holder. Confirm or refute 1 to 3 from that,
then fix the named wait (a context-aware lock acquisition, a bound on
teardown, or a default cell bound) and not before.

## Status (end of this change)

- Action 1: landed (overlay: frontend build is a job), plus failures 6 and 7
  (never report a build done without its job's exit 0; a running job is not a
  blocker), pinned by `TestRLMEngineeringOverlayTeachesCapsuleJobs` J5 and J7.
- Failure 5 (seccomp `sched_getparam`, `sched_getscheduler`): landed, red.
  Linux test compiled and vetted, not run on this darwin host.
- Action 2 (failure 2): landed, red; see "Fix design" above. R1 and R2 open.
- Action 3 (failure 3): observability landed, no behavior fix, by design.
  Not proven; `steps` and `engine_mutex_live` on `/api/runtime/observability`
  are what the next drill reads. Open: which of hypotheses 1 to 3 is the
  wait.
- Action 4: not done. The staging drill should show: (a) a frontend job
  finishing without `pthread_getschedparam` noise; (b) a `blocked` Complete
  after a receipt-less mutation recording its fate (the granted receipts
  carry `subject_binding: "observed"`); (c) a `completed` Complete after a
  receipt-less mutation refused with the digests named; (d) if any Complete
  stalls, `steps.in_flight` naming the step and its age, and
  `engine_mutex_live` naming the lock holder.

## Drills against 130caf6d9 (2026-10-20, 13:40-14:25 UTC)

Deploy 130caf6d9 (staging health `deployed_commit` checked before the
drills; both test computers cold-booted first). Recorded before any fix.

### What the drills show

- **(a) holds.** No `pthread_getschedparam` line in either drill. The
  frontend job (`sh -c 'npm ci && npm run build && cp …'`) builds in about
  20 s. In both drills the first frontend job failed in its copy step
  (`cp: setting permissions for '/var/lib/artifact/release/frontend/…':
  Operation not permitted`) and the identical second job passed. Not
  understood; named failure 11 below.
- **(b) and (c) not reached.** No drill got to a blocked-after-mutation or
  a refused completed Complete.
- **(d) failed as an instrument.** `/api/runtime/observability` on the hung
  delegate computer did not answer in 60 s or in 240 s, while
  `/api/runtime/obligations` and `/api/runtime/census` answered in 0.1 s.
  The handler reads `capsuleExecutor.ClaudeDelegateStatus()`, which takes
  `e.mu.RLock()`, before it reads `steps`. So the field built to show a
  hung step cannot be read while the executor lock is held, which is the
  case it was built for. Named failure 9.

### Failure 3 reproduced twice; both hangs are a capsule destroy

**Visual drill** (computer-7011602c…, trajectory 6b725656-…, run
`run:assignment-4d03f32430f92b8ef7b7026b`). Engineering ran the frontend
job, a Go build job, `choir-capture` on the drill route, then `Freeze` at
13:53:49 returned `FREEZEOK` (bundle `3aba33a4…`, the operation reached
`frozen`). At 13:54:04 it called `Complete("completed", …)`. No
`tool.result` followed; the run was cancelled `interrupted_by_restart` at
14:34 when I booted the computer to read it. The guest log (host console
file, read only) has no runtime line after 13:53:59.

**Delegate drill** (computer-ad1b46d4…, trajectory 0acd05c8-…, run
`run:assignment-d1b46e6e7bf07d8227061c9b`). The delegate worked: run
`d20261020T134119-001273b3`, done in 19 s, exit 0. Then the cell that read
the file was refused `assigned capsule obligation is no longer executable:
assignment fate is not active and bound` (13:41:43, 13:41:44), and the next
model step (13:41:45) produced no `tool.invoked`. The run is still
`running` at 14:30. The guest log has no runtime line after 13:41:45.

What unbound it, from the tape:

1. 13:40:41: management's run (`fd28a87e…`) read a `closure` record:
   `conv:engineering:assignment-b4bb8ad3…|management:…:r1:restart`, the
   cold boot closing the 12:17 drill's engineering as interrupted.
2. 13:40:57: management relayed it as a `blocker` ReportPacket to that
   drill's Texture (`texture:1c989d17…`), `work_disposition: completed`.
3. 13:40:58: that Texture woke (`integrate_execution_findings`) and at
   13:41:25 cast management "Resume and complete the Claude delegate
   probe". Its packet says "Engineering's prior assignment was interrupted
   by restart … The probe remains unfulfilled."
4. 13:41:30: management cast engineering for it.
5. Opening that assignment runs `reclaimSupersededAssignmentCapsules`
   (`engineering_assignment_runtime.go`): one live assignment capsule per
   computer, so every other live assignment, here the current drill's, is
   cancelled "superseded by a fresh implementation assignment" through the
   fate path (revoke, `ForceDestroy`). Its success log line ("reclaimed
   superseded assignment capsule") never appears, and the new assignment
   has no run.

So both hangs are inside a capsule teardown: the visual one under
`Complete` (`complete.revoke_capsule`), the delegate one under the
supersede's cancel. `Executor.destroy` holds `e.mu` (write) for the whole
teardown, and `WaitEmpty(ctx)` and the cancel path's context are the
unbounded candidates (hypothesis 2 above). Everything else that needs
`e.mu`, including the observability handler, the cell gate and every job
poll, then queues behind it, which is why the runtime goes silent.
Hypothesis 2 is now the leading one; which teardown step is stuck is
still not observed (failure 9).

### Failures 8-11 (new)

8. **A crash restart resumes work through the desks.** Steps 1-3 above:
   the boot's closure record wakes management, management reports it to
   the old Texture as a blocker, and the old Texture re-requests the same
   work. CLAUDE.md: "A crash restart never resumes work." Each cold boot of
   a drill computer repeats this for every earlier interrupted request
   (this computer had six). Hypothesis on the fix's location, not decided:
   either the closure reaches Texture as a fate to render, with the
   overlay saying a restart closure is not a reason to re-request, or the
   runtime refuses a Cast whose parent request was closed by a restart
   until an owner act.
9. **Observability blocks on the executor lock** (above).
10. **A new assignment preempts a live one on another trajectory.** The
    one-live-capsule rule (budget) cancels live work for an unrelated
    request instead of queueing the new one. With 8, a restart's stale
    re-request kills the request the owner is waiting on.
11. **The frontend job's copy fails once, then passes** (above).

**S6-0 drill** (computer-5dab0a75…, trajectory 8b629702-…) is not a hang:
engineering wrote the probe package, ran the frontend job twice (11 above),
and started the Go build job at 14:12:36; at 14:25 the job was still
downloading modules (`go: downloading github.com/Azure/…`) and the tracer's
15-minute decision budget ended. The visual drill's Go build job ran more
than 414 s. A cold-module Go build in a capsule is over ten minutes, so the
S6-0 drill needs a budget of at least 30 minutes until module downloads are
warm. Polling cost: the visual run polled the Go build job 84 times in
about 7 minutes and compacted once; the overlay says not to poll.

### Next actions, in order

1. Failure 9 (red: guest observability): the handler reads `steps` first
   and never takes `e.mu` (the delegate config read moves to its own
   lock), and the route gains an owner-only goroutine dump, bounded, so a
   hang names its stack.
2. Rerun the delegate drill (the fastest reproduction) and read the
   in-flight teardown step and the goroutine dump.
3. Then fix the named wait, and decide 8 and 10 (design first, red).

## Fix design for failure 9 (red: guest observability)

Written before code. The route must answer while the executor lock or the
store's engine lock is held, because that is when it is needed.

Ways it could fail, each pinned by a test or a reason:

1. A read in the handler waits on `e.mu`. `ClaudeDelegateStatus` reads
   the delegate's dir and binary from an atomic copy written beside the
   locked fields, so it never takes `e.mu` (linux test: the status
   returns while a test holds `e.mu`).
2. A read waits on the engine lock or any other lock not yet known. Each
   read that can wait (management tasks, effective release, the delegate
   status) runs under a 3 s budget; a read that does not return in time is
   named in `unanswered` and the rest of the response is still written. The
   abandoned read finishes when its lock frees and writes only to its own
   buffered channel, so there is no race on the response (test: a blocked
   read is named and the handler still answers; race detector in CI).
3. `steps` is read after something that blocks. It is read first, and it
   takes only the tracker's own short mutex.
4. The goroutine dump leaks something. It is opt-in (`?goroutines=1`),
   behind the same owner authentication as the route, bounded to 1 MiB with
   a visible truncation marker, and holds stacks: function names, files,
   lines and argument words, no heap contents. Secrets are not stack
   arguments in this runtime's call paths (they are read from files into
   byte slices passed by pointer).
5. Repeated polling during a long hang piles up blocked goroutines. Each
   poll leaves at most three goroutines parked on a lock, each a few KiB;
   the tracer polls once a minute at most. Accepted, named.

Rollback: revert. Heresy delta: discovered, an instrument that shares the
lock it observes (failure 9); repaired, that one.

## Failures 8 and 10: what the code says, and the candidate fixes (not decided)

Read from code after the evidence above; no change made.

**Failure 8, the path.** `closeOpenRoundsAfterCrash`
(`internal/agentcore/inbox_records.go`) writes one system closure per open
round at a crash boot, addressed to the round's caster, and "a closure wakes
no one". That holds: nothing woke on the closures. But the next drill's
Texture Cast woke management at 13:40:41, and management's `Records()`
listed the unread closures with it (seq 14 and earlier). Management then
sent a final `ReportPacket` to `texture:1c989d17…`, whose round with
management the same boot had already closed (record
`…|texture:1c989d17…:r1:restart` in that Texture's inbox). That report
woke the Texture (`integrate_execution_findings`), and the Texture, whose
work item for the request is still open, cast the request again. The
census on the computer shows six open Texture work items, the oldest from
10:11, across at least four cold boots: the crash boot closes rounds,
runs and assignments, but a Texture's work item for its request survives,
so the Texture still holds the request as unfinished.

Candidates, most mechanical first:

- **A. A round closed by a restart takes no further Report.** The store
  refuses (or records without waking) a Report addressed to a caster whose
  latest round in that conversation was closed `interrupted_by_restart`,
  until the caster casts again. This cuts the relay without relying on a
  prompt. Cost: a desk that learns something real after a restart cannot
  tell the old Texture; it can only answer a new Cast.
- **B. The crash boot closes the Texture's work item too**, with the fate
  "interrupted by a restart", so the Texture no longer holds the request
  as open, and a wake renders the fate instead of re-requesting. This is
  what CLAUDE.md's rule already says (work items are listed). The owner
  restarts the request.
- **C. Prompts only:** management's overlay says a restart closure is not
  news to relay; Texture's says a restart is not a reason to re-request.
  Weakest; the rule would depend on two models reading two sentences.

A and B are not exclusive. Leaning: B, because it is the stated rule, plus
A, because it removes the relay path. Red (desk wake semantics, Texture
obligations); a panel before code.

**Failure 10, the path.** `reclaimSupersededAssignmentCapsules`
(`internal/agentcore/engineering_assignment_runtime.go`) cancels every
other live assignment on the computer before a new one spawns, so the
capsule budget holds one. It does not look at the trajectory. Candidate:
supersede only an assignment on the same trajectory (the same request cast
again); for another trajectory, refuse the new assignment with a visible
"engineering is busy with <assignment> for <request>" so management can
wait, instead of killing work the owner is waiting on. Red (assignment
admission); after the hang is fixed, because each supersede today ends in
the teardown that hangs.

*Update 2026-10-21 (FP4, ARCH-10):* the supersede is removed, for every
trajectory, not only another trajectory's. The reclaim is now
`reclaimTerminalAssignmentCapsules` (terminal assignments' leaked capsules
only), and a new Cast waits in FIFO order behind the live assignment and
older open requests, with a retryable refusal and a retry wake
(`internal/agentcore/engineering_admission.go`;
[engineering-admission-supersedes-live-assignment-2026-10-21.md](engineering-admission-supersedes-live-assignment-2026-10-21.md)).
The candidate above (supersede a same-trajectory re-Cast) is not taken:
a re-Cast of a request whose assignment is still live now waits for that
assignment's fate or its deadline.

## Failure 3 located: the teardown's memory.reclaim write never returns (424ee13f7, 15:30 UTC)

Observed, not inferred. Deploy 424ee13f7; both drill computers hung at the
first engineering Complete, and the fixed route answered in 0.2 s with
nothing in `unanswered`.

- **Delegate drill** (computer-ad1b46d4…, run
  `run:assignment-9efd4078ed66c63c9add0163`): in-flight steps `cell.reduce`
  → `reduce.complete_intent` → `complete.revoke_capsule` →
  `revoke.destroy_capsule` → `capsule.destroy.reclaim`, the last started
  15:11:14, 1,120 s at the read.
- **Visual drill** (computer-7011602c…, run
  `run:assignment-df7f8cb2948dfb283e3ce9e0`): the same chain,
  `capsule.destroy.reclaim` started 15:21:04, 530 s at the read; the
  operation had reached `frozen`.
- **Goroutine dump, both computers:** `goroutine … [IO wait, 18 minutes]`
  in `internal/poll.(*FD).Write` ← `os.WriteFile` ←
  `capsule.cgroupMemoryDir.Reclaim` (`job_memory.go:74`) ←
  `teardownReclaim` (`teardown_reclaim.go:96`) ← `Executor.destroy`
  (`executor.go:769`, holding `e.mu`) ← `ForceDestroy` ←
  `revokeAssignedCapsule` ← `recordAssignedEngineeringReportOnce`.
  Reclaim targets were 0x15b89000 and 0x1633e000 bytes (about 347 and
  355 MiB). The guests had 7.1 and 6.3 GiB available.

**Cause.** `memory.reclaim` fails with EAGAIN when the kernel reclaims less
than asked, which the code expects ("a partial reclaim (EAGAIN) is not an
error"). But `os.WriteFile` opens the cgroup file through Go's runtime
poller: kernfs files are pollable, so `os.OpenFile` registers the
descriptor with epoll and makes it non-blocking, and on EAGAIN `FD.Write`
parks the goroutine until epoll reports the file writable. A cgroup
control file never does. So a partial reclaim parks forever, inside the
executor lock, and everything that needs `e.mu` stops behind it: the
Complete, every cell gate, job polls, and (before de92fad5e) the
observability route. A full reclaim returns normally, which is why some
teardowns finish.

The same call sits in `lowerCgroupMemory` (`job_memory.go`, returning a
job's memory boost), so a job's end can hang the same way.

This is the substrate cause of failure 3 and of every silent run today:
hypothesis 2 (teardown under the executor lock) was the right layer;
the specific wait is a write the Go runtime turns into an unbounded poll.

**Fix (next commit, red: capsule teardown).** Write cgroup control files
with a plain blocking descriptor (`unix.Open`, `unix.Write`, `unix.Close`),
so EAGAIN comes back as an error, as the code always assumed; one helper
used by `Reclaim` and `SetMax`. A test opens a pollable file that answers
EAGAIN (a full non-blocking pipe) through the same helper path and
requires the write to return. Not in this fix, named: physical teardown
still runs under `e.mu`, so any future slow step stalls the whole
executor; moving it off the lock is a separate red change.

## Drills against 614fd445f (the reclaim fix), 16:00-17:01 UTC

Staging ran 614fd445f (deployed 15:53). Drill script: scratch `drills6.sh`;
the drill computers are cold-booted first.

- **Delegate drill: pass** at 16:06 ("engineering ran the delegate to
  success"). This is the drill that hung twice on failure 3. Complete
  returned. The fix holds here.
- **Visual drill: pass** to its decision boundary at 16:16, with a capture
  listed, the PNG served (range 206) and vision reading the token. Before
  that, at 16:07, Freeze refused one build receipt: "the workspace changed
  while job ... ran". Engineering re-ran the build and froze. This is the
  receipt rule working as designed, but it cost a cycle; noted under action 1.
- **S6-0 go-served (label s6-0-1618): reached owner approval, failed at
  apply on the drill's own budget.** Frozen and accepted at 16:51. The
  release witness read consistently at accepted and at the applied read (17
  pairs: 12 compared at the gate, 5 recorded only, 0 unbound). The materializer
  then logged "host build pending" every minute until the tracer's default
  `--apply-min 10` ran out at 17:01. That is a harness budget, not a product
  refusal. Rerun with `--apply-min 40` started at about 17:10.

### Failures 12-13 (new, from the S6-0 drill, hypotheses until a second run)

12. **Go module downloads stall in the capsule.** Round 1's report says:
    "required Go vet and the Go release binary build could not complete
    because dependency downloads stopped progressing". This is the capsule
    Go-modules question that is still open
    (`docs/reports/capsule-go-modules-decision-2026-10-14.md`, routes A/B/C,
    no recorded ruling). It cost one round and about 10 minutes.
13. **A retry names a test package that does not exist yet.** Rounds 2 and 3
    report the required `go test ./internal/selfdevprobe/` failing "because
    that directory is absent", and block. The retry objective required the
    test before the package was created, so engineering refused rather than
    create it. Round 4 succeeded. Whether management wrote that objective,
    or engineering misread it, has to come from the trace
    (`~/.config/choir-qa/traces/s6-0-1618/bundle-a1.json`, records 1 and 9)
    before it is called a finding.

Not changed by these drills: failures 8, 10 and 11 stay open, and teardown
still runs under `e.mu`.

## S6-0 rerun against 614fd445f (label s6-0-1702, 17:02-17:45 UTC)

Same staging build, `--apply-min 40 --timeout-min 45`. Receipt and log:
scratch `s67/log.txt`; bundle `~/.config/choir-qa/traces/s6-0-1702/bundle-a1.json`
(exported 17:33:12, before the restore).

**18 of 19 acceptance checks pass; `witness_joined` fails.** Every product
step passed: Texture cast management (17:02:41), management cast
engineering (17:03:07), engineering froze in one round (one cast, one final
report; failures 12 and 13 did not recur), the verify named the bundle,
owner approval (17:17:05), apply through one planned
`self_development_apply` restart (boot 17:32:31, applied 17:32:37), the
route receipt, the probe served by the applied build 19527f0e0 (17:33:09),
then restore by `tape_reconstruct` to the pre-apply head (sequence 2727,
witness matched, 17:33:15) and the probe gone on the deployed build
614fd445f (17:33:21).

- **The apply takes about 15 minutes.** Accepted 17:17:05, applied 17:32:37,
  almost all of it host build. The first run's 10-minute default
  (`scripts/reeng_tracer.mjs`, `--apply-min`) was the whole of its failure.
  The default should be at least 20; not changed here.
- **The release witness joined at frozen, accepted and applied** (17
  pairs; bound 4, 6 and 15; unbound 0 at every read; 2 still unknown at
  applied: `frozen_surface_decision_heads` and `divergence_derived`, both
  recorded only).

### Failure 14 (new): the release witness answers 502 for 12 minutes after a restore

The `restored` witness read retried for the tracer's whole restore budget
(12 minutes) and ended `502 {"error":"computer authority unavailable"}` at
17:45:19. The guest was reachable through that window: the probe at
17:33:21 came back from the deployed build.

That error is written in one place for this route
(`internal/proxy/self_development.go`, the `operations/{id}/...` handler):
`resolveComputerURLForComputerTarget` returned an error for an API-key
caller. That function fails on three branches: the target's state is not
`active` or has no URL, `ensureComputerVersionRoute` cannot resolve the
immutable ComputerVersion route from vmctl, or the key's computer does not
match. **Which branch it was is not known**, and is a hypothesis until
traced: the handler discards `err` and logs nothing. The proxy journal has
no line for this computer between 17:33 and 17:46, and vmctl's journal has
only the stop at 17:45:20.

Leading hypothesis, from code reading only: the restore re-projects the
computer's route, and the version-route lookup refuses (or the realization
row is not `active`) until something re-reads it. The proxy's route cache
keeps `routeOK` for a slot once seen, which would explain a fast probe and
a refused witness only if they resolve through different paths. That has
to come from a trace.

Two changes follow, in this order (problem first):

1. Log the discarded error on that 502 (and on the other
   `computer authority unavailable` sites in `internal/proxy`), with
   computer id and branch, never a credential. Orange, observability only.
2. Rerun S6-0 and read which branch refuses, then fix that branch.

Also seen, not a failure: the tracer's front-end `served_check` reported
`served: false` at every read, including post-apply, while the API probe
was served. `served_required` is off for this change, so it is not an
acceptance check; the check probably reads a static path the go-served
change does not touch.

Open from this cluster: failures 8, 10, 11 and 14; teardown under `e.mu`.

## S6-0 rerun against ac4737862 (label s6-0-1817, 18:17-18:38 UTC)

Staging ac4737862 (the failure-14 logging, 38954bb55). Receipt: scratch
`s68/log.txt`; bundle `~/.config/choir-qa/traces/s6-0-1817/bundle-a1.json`.

**15 of 19 checks pass; it did not exercise failure 14.** Failed:
`go_test_passed`, `probe_served_after_apply`, `layered_release_serving`,
`restore_restarted_into_pre_apply`. `witness_joined` passed: the
`restored` witness read 200 at 18:37:57, and the proxy journal has no
`computer authority unavailable` line since 18:15. The restore did not
restart the computer this time (there was no new build to leave), which
fits failure 14 being tied to the restart after a restore; still a
hypothesis.

- **Failure 12 recurred.** Round 1 (assignment ...248192, 18:18-18:21)
  reported failed: "Vet remains blocked: the job is still downloading
  dependencies with no completion receipt". The capsule Go-modules decision
  is still unanswered.
- **Failure 15 (new): a candidate without the change passed the gate and
  was applied.** Management re-cast (assignment ...1bf749, 18:22).
  Engineering reported "go vet passed; built release tree and froze
  candidate bundle 78f00766..." (18:36:18). Verify, owner approval and
  apply followed; the apply took 18 seconds with one planned restart (boot
  18:36:47) onto the same build ac4737862, and the effective release is
  `layered: false`. The probe route was never served; the tracer found no
  Go test evidence.

  Hypothesis A (leading, from the trace order only): the re-cast is a new
  assignment, so a fresh capsule from the base tree; round 1's edits did
  not carry over, and round 2's objective ("vet, release builds, freeze")
  led engineering to freeze an unchanged tree. Hypothesis B: Freeze's
  no-Go-input path misclassified a Go change. Either way the gate has a
  gap: verify and approval passed a candidate whose patch does not do what
  the request asked. To confirm: read the operation's source patch (its
  file list and size) before choosing a fix. Candidate fixes, not decided:
  a re-cast carries the prior capsule's diff, or a Freeze refuses an empty
  diff, or management's verify checks the patch against the request.

### Failure 15 confirmed from the trace: the re-cast started a fresh capsule, and an empty source patch passed as "frontend-only" (2026-10-20, 19:55 UTC)

Read from the s6-0-1817 trace bundle (run events, tool calls and results of
both engineering runs) and the receipt's release witness. Hypothesis A is
confirmed; hypothesis B is wrong as stated (Freeze did not misread a Go
change: there was no change to read), but the gate gap it pointed at is real.

- **Round 1** (`run:assignment-248192…`, capsule `capsule-40939d44…`,
  18:18-18:21): wrote `internal/selfdevprobe/selfdevprobe.go`, its test and
  the route line in `internal/apihandler` (18:18:34, "written"); the
  focused test passed (`ok …/internal/selfdevprobe 0.003s`, 18:19:26);
  vet sat in module downloads; Complete at 18:21:37 reported failed.
- **Round 2** (`run:assignment-1bf749…`, operation capsule
  `capsule-ce41d011…`, 18:22-18:36): a different capsule. Its cells, in
  order: read records, Precommit, `go vet ./internal/apihandler/` (12
  minutes of downloads, then passed), toolchain versions, two frontend
  builds (the first failed on `cp` permissions, the second used
  `--no-preserve`), `go build ./cmd/autoputer`, Freeze, Complete. No
  `WriteFile`, `ReadFile` or diff of any kind: it never looked for round
  1's change. Its prompt ("Complete the requested bounded recovery: run
  go vet …; only if it passes, build the release tree …, freeze") did not
  say the change had to be made again. The prompt came from Texture's
  `integrate_execution_findings` revision, passed on by management.
- **The gate.** The frozen operation's bundle has `binary_input_count: 0`
  and 55 runtime files (the rebuilt frontend). The witness reads it as
  "the bundle carries no source patch (a frontend-only candidate)". Freeze,
  verify, owner approval and apply all went through. After the apply the
  effective-release read already says "the applied operation changed no
  workspace/platform source" (`errNoSourcePatch`,
  `internal/agentcore/effective_release.go`), but only as a recorded
  `tree_error`, after the restart.

**Two problems, not one:**

1. **Lost work across a re-cast (substrate: capsule lifetime).** A failed
   Complete revokes and destroys the assignment's capsule, so a re-cast
   starts from the base tree. Round 1's working, tested change is gone,
   and nothing tells round 2. Management and Texture wrote "continue the
   existing engineering work", which the substrate cannot honour.
2. **A candidate with no change passes the gate (symptom layer,
   deterministic).** "No source patch" is read as "frontend-only" without
   checking that the frontend changed either. A rebuilt frontend from an
   unchanged tree is a release of nothing new. The check belongs at
   Freeze: a capsule whose source tree, frontend included, equals its base
   has nothing to propose; Freeze refuses and engineering reports that,
   honestly.

Fix order: problem 2 first (small, deterministic, red: Freeze is run
acceptance). It turns this case into an honest failure report instead of
an applied no-op. Problem 1 needs a design: either a failed assignment's
capsule diff is kept as an artifact that a re-cast can name to seed its
new capsule, or the re-cast tells engineering plainly that the work is
gone. Not decided; a panel question once problem 2 lands.

**Problem 2 landed** (22b287a5d, CI 39487132368 passed, staging deploy
receipt names 22b287a5d). Deployed proof waits for a drill whose re-cast
reaches Freeze.

### Failure 15, problem 1: carry the failed round's diff, not its capsule (decided, 2026-10-20)

**Owner decision, 2026-10-20 about 23:35 UTC: "yes we decided option A"**
(after weighing A against B). Build A; B (capsule reuse) is not pursued.

Owner, weighing A (keep the diff) against B (reuse the capsule): "Could
make sense to reuse capsules but that could also have downsides of runaway
resource usage and maybe security implications."

Proposed: **A**. Reasons:

- **A capsule is authority and state, not just files.** It holds running
  processes, a job cgroup, caches, `node_modules`, built binaries, the
  release directory, and a capability grant bound to one agent run.
  Reusing it means moving that grant from one assignment to another (a new
  authority surface) and carrying forward whatever ran inside it: round 1
  of this drill ran `npm ci`, and npm reported a package with install
  scripts. A fresh capsule per assignment is the guarantee that nothing
  but the base tree reaches the next round.
- **Resources.** A held capsule keeps memory, disk and a cgroup until
  something collects it; fail-and-re-cast loops multiply them. It also
  cuts against the restart rule (a crash closes work; it never resumes it).
- **A patch is inert data.** The `/workspace/platform` source diff only
  (the same writer Freeze uses, `emitSourcePatch`), digest-bound and
  size-capped, and readable: management and Texture can see its file
  list, so their belief that "the work is still there" becomes true and
  checkable.

Shape:

1. A Complete that ends without a frozen candidate writes the capsule's
   source diff as an unfinished-work artifact, before the capsule is
   destroyed, and the assignment report names it (digest, base commit,
   file list, size). An empty diff writes nothing and the report says so.
2. A re-cast may name that artifact. The new capsule starts from the base
   tree with the patch applied; if the base moved and it does not apply,
   the cast says that plainly instead of starting.
3. Nothing is trusted from the carried patch: the new round re-runs the
   tests, and Freeze, verify and approval judge it as any change.
4. The re-cast carries the original request text, not only the recovery
   step (round 2 here had only "run vet, build, freeze").

Failure modes to write as tests first: a diff lost when the capsule is
destroyed; a patch applied to a different base without saying so; a
carried patch that skips a test; a patch with paths outside
`/workspace/platform`; an oversize patch; the report naming a patch that
does not exist.

Also: the trigger here was failure 12 (vet stalls on module downloads).
Fixing that removes the commonest way a round fails after doing its work.
The capsule Go-modules decision (A/B/C) is still the owner's.

### Failure 12 re-read: the stall is a cold compile, not module downloads (hypothesis, 2026-10-21 00:10 UTC)

A code and trace reading (s6-0-1817 bundle, both rounds) points away from
downloads:

- In round 2, `go vet ./internal/apihandler/` printed its "go: downloading"
  lines between 18:22:44 and 18:22:58, about 14 seconds, and then nothing
  until the job ended "done, exit 0, after 728 s" at 18:34:52. Round 1's
  poll tail showed the same 63 download lines throughout. The poll keeps the
  last 5 KB, and `go vet` prints nothing while it compiles, so a quiet
  compile reads as "still downloading". The 171 modules match the host
  module proxy's earlier drill (513 hits, 0 misses in 49 s).
- The host module proxy (owner's 2026-10-14 shape, `cmd/goproxy`, reached
  as `goproxy.choir.internal`) works. It is a read-through proxy, though,
  not a mounted cache. Capsules get `HOME=/tmp` and no `GOMODCACHE` or
  `GOCACHE` (`internal/capsule/egress.go:44`,
  `cmd/capsule-broker/main.go:538-540`,
  `internal/capsule/broker_env.go:24`), so every capsule starts with an
  empty module cache and an empty build cache.
- The engineering capsule has one CPU and 2 GiB
  (`internal/agentcore/engineering_assignment_runtime.go:29`). A cold
  compile of apihandler's dependency tree on one CPU is about 1,000 s on a
  laptop at `-p 1`, which is the order of the 728 s observed.

Leading hypothesis: the build cache is cold on one CPU. The downloads
add about 15 to 50 seconds. To decide it, run `go vet
./internal/apihandler/` twice in one fresh engineering capsule and time
each run with `du -sh /tmp/go/pkg/mod /tmp/.cache/go-build`; a second run
in seconds confirms it. The host goproxy journal for that capsule gives the
download span without a capsule. Fix candidates, not decided: a read-only
build-cache seed built by the host for the base tree (capsule-long-exec
design R2, "not built"), more CPU for engineering capsules, or vetting
only the changed packages.

### Failure 15, problem 1: option A design and failure modes (2026-10-20, written before the code)

Classification: red (engineering capsule spawn, the run context a desk reads,
the freeze path's inputs). Rollback: revert the commits; the feature is
fail-open, so removing it returns every retry to a clean base tree.

**Finding that shrinks the design.** The substrate already keeps the failed
round's work. A terminal Complete whose frozen tree differs from the base
persists the whole tree as a content-addressed subject artifact
(`PersistGrantedCandidate`, `capsule-subject:sha256:<digest>`, in the
executor's durable artifact store) and records an
`EngineeringSubjectCandidate` on the tape (`choir.co_super_subject_candidate`:
trajectory, assignment, original and new subject digests, artifact ref,
source report). That happens for failed, blocked and partial results as well
as completed ones (`recordAssignedEngineeringReportOnce`,
`engineering_assignment_fate.go`, the `complete.export_candidate` step). The
lost-work gap is therefore not "nothing was kept". It is that nothing reads the
kept candidate when the next assignment on the trajectory spawns. So option A
needs no new durable store: the carried patch is a pure function of two
content-addressed trees that are already durable, and it is derived and staged
when the retry spawns.

**Shape.**

1. *Source of the patch.* At the spawn of an implementation assignment
   (`spawnBindActivateAssignment`, the one place that builds the `SpawnSpec`,
   where `AdoptionDir` is already set), the runtime lists the trajectory's
   subject candidates, drops those of the assignment being spawned and of
   non-implementation assignments, and takes the newest. If that candidate's
   report result is `completed`, the chain ended in a finished round and
   nothing is offered. Older candidates are named in the offer
   (`older_candidates`) and not diffed: at most one carried patch per chain,
   newest wins.
2. *The patch.* `capsule.DiffTreesToPatch(base tree, candidate tree)` renders a
   git-applicable unified diff with the freeze path's own renderer
   (`renderSourcePatch`) and proves it reproduces the candidate with the
   builder's applier (`verifySourcePatch`), exactly as `emitSourcePatch` does
   for a freeze. The base tree is the failed round's own
   `Binding.SourceArtifactRef` (`capsule-source-git:<commit>:sha256:<digest>`),
   re-materialized from the pinned commit. The record binds: patch sha256,
   base ref, base commit, base subject digest, candidate id and artifact ref,
   the file list, and anything omitted.
3. *Offer.* The patch is staged content-addressed and read only at
   `<updater root>/carried/<sha256>/` (`source.patch`, `carried.json`), and
   mounted read only at `/selfdev/carried` in the retry's fresh capsule, the
   same way `/selfdev/adoption` is (copy into the source lower layer, then a
   read-only bind over the merged path so the overlay upper cannot shadow it).
   The run context the engineer reads names the patch (digest, file list,
   source assignment, base) and says: it is the earlier round's unfinished
   work, not trusted; apply it with `git apply /selfdev/carried/source.patch`
   from `/workspace/platform` or ignore it; re-run the tests in this capsule
   either way. Run metadata records `carried_patch_sha256`,
   `carried_patch_status` (`offered` or `refused: <reason>`), the source
   assignment and the base refs, so the offer (or the refusal) is on the run
   record and checkable by management and Texture.
4. *Nothing is applied.* The capsule's tree is the clean base. Nothing writes
   the patch into `/workspace/platform`, the live computer's source repository,
   `incoming/`, or the updater. The only way the patch reaches the live
   computer is the engineer applying it in the new capsule, then the normal
   Freeze, verify, owner approval and apply, which judge the new capsule's own
   diff as any change (and `freezeSourceChangeError` refuses an empty one).

**Failure modes and the guard for each** (tests written first, in
`internal/capsule/carried_patch_test.go` and
`internal/agentcore/carried_patch_test.go`):

- *Diff lost when the capsule is destroyed.* The candidate is read from durable
  artifacts, not the capsule. Test: with the original capsule gone (only the
  tape record and the artifact tree exist) the retry still gets the patch.
  Residual: a round that ends with no terminal Report (assignment deadline,
  capsule crash, restart interruption) writes no candidate, so nothing is
  carried. Capturing at those edges needs a new durable record written before
  `ForceDestroy`; deferred, named below.
- *Patch applied to a different base without saying so.* The retry's base ref
  is compared with the patch's base. Same base: offered. Different base: the
  patch is checked with `git apply --check` against the retry's base tree for
  the touched files; if it applies it is offered with the base change stated,
  if it does not it is refused with the first conflicting file and both base
  commits. No 3-way merge is attempted (a 3-way apply could write conflict
  markers into a source file the engineer then ships); the refusal is the
  safe default and is reversible by a later decision.
- *A carried patch that skips a test.* The patch carries no test evidence and
  the offer says so. The engineer's Freeze still requires distinct build, test
  and toolchain receipts from cells run in the new capsule after its last
  write (`freezeReceiptSlotsError`, `freezeReceiptOutcomesError`). Test: the
  offer text states that tests must be re-run, and no receipt, verdict or
  evidence ref of the earlier round is copied.
- *Paths outside the source tree.* The patch is derived from
  `/workspace/platform` only; it refuses absolute paths, `..`, and `.git`
  paths, skips `node_modules`, symlinks and binary files (listed under
  `omitted`, never in the patch), and the staged patch's headers are
  re-validated at stage and again at spawn. Test: a hand-built patch naming
  `../x`, `/etc/x` or `.git/config` is refused.
- *Oversize.* Patch size above 2 MiB is refused with the size, not truncated;
  the retry then starts clean and its run record says why.
- *The report naming a patch that does not exist.* The offer is built only
  from a patch that was staged and re-verified (`verifyStagedCarriedPatch`
  reads the bytes back and checks the digest) in the same call. A staged
  directory whose bytes no longer match the digest is refused, never mounted
  (the `/selfdev/adoption` tamper check, A8). Test: tamper, then the offer is
  refused and names nothing.
- *Empty diff.* An empty diff stages nothing and offers nothing; no
  `carried_patch_*` metadata is written.
- *Secrets.* The patch is source only: it is derived from a candidate that
  `PersistGrantedCandidate` already screened (claude-delegate token refusal),
  carries no environment, no cell output, and no files outside the source
  tree. It does not carry `node_modules` or build outputs.
- *Unavailable or corrupt candidate artifact.* Treated as "nothing to carry":
  the spawn is never blocked, and the run record says
  `refused: candidate artifact unavailable`. Carrying work forward is an aid;
  a failure to carry must not fail the retry.
- *Crash restart.* The candidate and the artifact survive a restart (tape and
  durable artifact store); the staged directory is re-derivable and
  re-verified. Nothing auto-resumes: the offer is computed only inside the
  spawn of an assignment, which only a Cast (an explicit retry) starts. No
  boot-time code reads candidates. Test: the offer is deterministic across a
  fresh `Runtime` over the same store, and the spawn of an assignment on a
  trajectory with a prior failed candidate is the only caller.
- *Compromised capsule state persisting.* No capsule survives its round
  (`revokeAssignedCapsule` unchanged). Only a digest-bound text patch does,
  and it is inert data until an engineer in a fresh capsule chooses to apply
  it and the gate judges the result.

**Not decided here (named residuals).**

- Rounds that end with no terminal Report (deadline, crash, interruption by
  restart) carry nothing. A capture before destroy at those edges needs its
  own durable record; the candidate object cannot be written without a report.
- A re-cast still carries only management's objective text (shape item 4 of
  the previous section: the original request text). That is a Cast-authoring
  change in the management desk, not in this slice.
- Whether the offer should also name the patch to management and Texture in
  the assignment's final Report (not only in the run record) is open.

**Red ceremony (2026-10-21, at landing).** Conjecture delta: before, a
retry starts from the clean base and redoes the failed round's work;
after, a retry starts from the clean base with the failed round's diff
offered read-only, and an engineer that applies it reaches its report
sooner. Protected surfaces: the engineering capsule spawn (a new read-only
mount), the run context the desk reads, the freeze path's inputs (only
what the engineer chooses to apply). Admissible evidence: a deployed
two-round drill on a test computer where round 2's run context names the
patch and its digest, and the frozen tree contains round 1's change; unit
tests are shaping only. Rollback: revert; the feature is fail-open.
Heresy delta: discovered, a failed round's work is durable but no retry
could see it; repaired, by this slice once the drill passes; introduced,
none intended (the patch is inert until applied in the engineer's own
capsule).

**Option A implementation notes (branch `feat/carried-patch-retry`, not yet
pushed or deployed).** Seams used: `Store.ListEngineeringSubjectCandidates`
(`internal/store/engineering_assignments.go`), `capsule.DiffTreesToPatch`,
`CarriedPatchPaths`, `CheckPatchAppliesToTree` (`internal/capsule/carried_patch.go`),
`Executor.DeriveCarriedPatch` and `SpawnSpec.CarriedPatchDir` mounted at
`/selfdev/carried` (`carried_patch_linux.go`, `executor.go` Spawn),
`offerCarriedPatch` (`internal/agentcore/carried_patch.go`) called from
`spawnBindActivateAssignment`, and the notice in `runContextForRun`. One
deviation from the design text: the decision is recorded per assignment in
`<updater root>/carried/offers/` rather than in run metadata, because the
run is part of the Bind command digest and a replayed spawn that decided
differently would conflict. The staged patch directory is content-addressed
at `<updater root>/carried/<sha256>/`. The `/selfdev/carried` capsule mount
is written like `/selfdev/adoption` and compiles for linux, but its mount was
not exercised on the darwin build host; the first staging drill is the
deployed proof (a failed round, a re-cast, then `ls /selfdev/carried` in the
retry's capsule).

## First carried-patch drill on staging: arm A, Sonnet delegate, ImageApp (2026-10-21, 03:36-04:31 UTC)

Drill: tracer label `arm-a-sonnet-imageapp-1021` against 8d027370b, on the
allowlisted delegate test computer (computer-ad1b46d4…), trajectory
251854fd-3a85-5612-809e-7f5aeb2fa93d. The trace bundle stays outside the
repo. What the trace shows, in order:

1. **Round 1 ended the way the brief planned.** Assignment
   `assignment-47d565ce00ff108c29f468d7` ran 03:46-04:06. The Sonnet
   delegate rewrote `frontend/src/lib/ImageApp.svelte` in 13 turns
   ($0.13). Its sandbox refused the npm build ("repeated Bash approval
   denials"), as the delegate's network mode is meant to. Engineering's
   own verification cell timed out. It reported a terminal failure and
   asked for the second round with the carried patch.
2. **Option A worked on staging.** Round 2
   (`assignment-6c6f6b7d3ff3c46ba183cf5c`, from 04:07:54) read
   `/selfdev/carried/source.patch` (6479 bytes). Its first
   `git -C /workspace/platform apply` exited 0 at 04:08:02. Every later
   "already exists in working directory" error came from engineering
   re-applying the same patch, not from a conflict. The capsule has no
   `.git`, so `git diff --stat` and `git status` cannot show the change
   (they print nothing, or "not a git repository"). That cost engineering
   five cells of doubt.
3. **The round-2 build was killed by the drill, not by Choir.** A
   synchronous `choir.Exec` of `npm ci && npm run build` died with
   "failed to read broker response … i/o timeout" after about 4 minutes.
   Engineering then started the same command as a job (`choir.Start`,
   04:12:37; the cell returned at 04:19:05). At 04:27:09 the tracer's
   50-minute deadline passed, and the tracer stopped the computer at
   04:27:30 with the job still running. The next boot closed the loop
   "interrupted by a restart" (04:31:22), as the restart rule requires.
   The arm has no outcome, only a deadline.

Problems found (new numbers continue the cluster):

- **Failure 16: secret-named and sandbox-mask files ride in the carried
  patch.** The patch adds 19 empty files at the source root: `.env` and
  seven `.env.*` names, `.gitmodules`, `.npmrc`, `.yarnrc`,
  `.yarnrc.yml`, `bunfig.toml`, `package.json`, `package-lock.json`,
  `pnpm-lock.yaml` and `yarn.lock`. The same set appeared in the earlier
  0565 drill as files nobody wrote
  (`drills-0565-request-unseen-and-receipt-typos-2026-10-20.md`, item 4).
  Hypothesis, not confirmed from the delegate's code: the delegate's
  sandbox write-protects these names in its working directory, and for an
  absent file the bind mount leaves an empty file behind in the capsule's
  upper layer. The candidate tree persists them, and `DiffTreesToPatch`
  carries every new text file. `carriedPatchSourcePath` refuses only
  absolute, climbing, quoted and `.git` paths; it has no secret-path
  rule. Today the files are empty, and a frontend-only release copies only
  the release tree, so they did not block the freeze. They still matter
  for two reasons. A delegate that writes a real `.env` would have its
  content carried into the next capsule and recorded as patch data. And
  the release path already refuses these names
  (`capsule release refuses secret-bearing path`), so the carried patch
  is looser than the release it feeds.
- **Failure 17: `choir.Exec` runs with working directory `/`.** The eval
  cell's `cwd` is `/workspace/platform`, but `pwd` through `choir.Exec`
  printed `/`, and a relative `git apply` failed with "No such file or
  directory". Engineering worked around it with `git -C`. Recorded as a
  residual, not fixed tonight.
- **Failure 18: a long synchronous `choir.Exec` dies on the broker
  timeout.** `npm ci && npm run build` through `Exec` lost the broker
  socket after about 4 minutes; `choir.Start` is the path that works.
  Recorded as a residual. The run-context text could say "use Start for
  builds".
- **Tracer defects.** (a) The deadline path called `phase()` inside
  `stallSnapshot`, where it is not in scope, so the attempt ended with
  "phase is not defined" before it kept the stall snapshot. (b) `--cred`
  cannot boot a stopped delegate computer: an API key gets "api key
  computer realization unavailable". Today the computer is booted through
  its browser session first. (c) Fifty minutes is too short for two
  delegate rounds plus a cold `npm ci`.

Decided fixes, smallest first. Failure 16: the carried-patch reader
leaves out any path the release would refuse as secret-bearing, and any
new empty file, and names both in `Omitted`. The predicate becomes one
function shared with the release path, so the two cannot drift. This is
test-first, in `internal/capsule`. Tracer (a): pass `phase` into
`stallSnapshot`. Tracer (c): later delegate drills run with
`--timeout-min 110` and `--keep-computer`.

## Failure 19: management never casts the build round with npm access (SlidesApp drill, 2026-10-21 04:42-05:07 UTC)

Drill: tracer label `sonnet-slides-1021` against cbef56155, on the delegate
test computer, trajectory ff8a80da-cffb-5e80-8caa-10f7cee8e9e2 (the trace
bundle stays outside the repo). Section 33's first Sonnet rewrite. I
stopped the tracer and the computer at 05:08 after Texture's fifth
re-cast, because no round could build.

What the trace shows. The designed two-round protocol is: round 1 in the
claude network mode (the delegate writes; npm is refused), round 2 in the
`ecosystem_proxy` mode (no delegate; npm reaches the registry; engineering
applies the carried patch, builds and freezes). Texture's request said so
in words both times ("a second round with network ecosystem_proxy"). The
capsule mode comes only from the `"network"` field of management's Cast
spec (`engineeringCastNetwork`,
`internal/agentcore/engineering_assignment_runtime.go`): no field on a
computer whose delegate is available gives `claude`; `"ecosystem_proxy"`
gives npm; `"forbidden"`, `"none"` or `"off"` withholds the network.

| Drill | Round | Management's spec | Capsule mode | Result |
|---|---|---|---|---|
| arm A (ImageApp) | 1 | `{"parent"}` | claude | delegate wrote; npm refused, as designed |
| arm A | 2 | `{"parent"}` | claude | patch applied; `npm ci` could not complete; stopped at the deadline |
| SlidesApp | 1 | `{"parent", "network": "forbidden"}` | forbidden | "choir-delegate unavailable: this capsule is not in claude mode"; engineering edited the file itself; npm stalled |
| SlidesApp | 2 | `{"parent"}` | claude | delegate wrote (12 turns, $0.14); `npm ci` failed with E403 on `vitefu-0.2.5.tgz` |
| SlidesApp | 3-5 | (not read) | | each failed within a minute |

Cause (from code and trace, not a hypothesis). Management's prompt
(`internal/runtimeprompts/overlays/rlm_management_runtime.yaml`, the
Delegation line) names one value: "Add "network": "forbidden" to the spec
to withhold the network." It never names `ecosystem_proxy`, nor says that
the default on a delegate computer is the claude mode, which refuses npm.
So management cannot ask for the build round, and when the request says
"network-restricted round" it reaches for the only value it knows.
Consequence: on the delegate computer, no two-round rewrite can reach a
freeze. Every Sonnet rewrite in section 33 is blocked by this, not by the
delegate. Arm B never met it because a minted computer has no delegate,
so its default is `ecosystem_proxy`.

Fix (decided: the conservative option, prompt text). The Delegation line
names the three values and when each applies: the default (on a delegate
computer, claude: the delegate writes, npm is refused), `"network":
"ecosystem_proxy"` for a round that must run npm or a build (no delegate),
and `"forbidden"` to withhold. A mechanical alternative was considered: a
round that receives a carried patch defaults to `ecosystem_proxy`. It is
not taken tonight, because the network mode is bound into the Cast's
binding digest before the carried-patch offer is decided at spawn, so
moving the decision touches the binding (red). It stays the fallback if
the prompt fix does not hold on the next drill.

Also seen: Texture re-casts without limit while every round fails fast
(five rounds in 25 minutes). Recorded as a residual.

### Failure 19 after the prompt fix (deploy d31403fd5, label sonnet-slides-1021b, 05:23-05:28 UTC)

Management now writes `"network": "ecosystem_proxy"`, but it wrote it on
round 1, with the objective "apply the carried first-round rewrite patch",
on a fresh trajectory that had no first round. Engineering found no patch
and refused in 27 s. The round-2 Cast left the field out (the claude mode)
but asked engineering to "verify the platform Git checkout and carried
patch". So the prompt line ("ecosystem_proxy for a round that must run npm
or a build") overcorrected: every rewrite request asks for a build, so
management read round 1 as the build round.

A second cause sits in the drill's own brief (`scripts/reeng_tracer.mjs`,
the rewrite request), not in Choir. It demanded "git diff --stat must show
the file changed", and a capsule has no `.git`; engineering treated the
missing baseline as a reason to make no writes. It also described the two
rounds in prose without saying what each Cast's network field is.

Changes: the brief now names both rounds and each round's network field,
and replaces the git check with `diff -u` against a copy saved before the
change. The management prompt is unchanged for now; if the next run still
casts round 1 as `ecosystem_proxy`, the prompt names the two-round order
explicitly, and after that the mechanical default (a carried-patch round
gets `ecosystem_proxy`) is the fix.

## Failure 20: the release copy fails on `fchmodat2`, which the capsule's seccomp filter does not name (label sonnet-slides-1021c, 05:31 UTC)

With the brief naming each round's network field, the two-round protocol
ran for the first time on the delegate computer. Round 1 (claude mode): the
Sonnet delegate rewrote `SlidesApp.svelte` (25 turns, $0.28) and
engineering reviewed the diff against a saved copy. Round 2
(`ecosystem_proxy`): engineering applied `/selfdev/carried/source.patch`,
`npm ci` added 59 packages in 4 s, and `vite build` ran. The job then failed
at `cp -rf --no-preserve=mode,ownership,timestamps dist/.
/var/lib/artifact/release/frontend/`, with one line per file:
"cp: setting permissions for '…': Operation not permitted".

Cause (code reading, strongly supported, not yet confirmed on a guest):
the capsule's seccomp filter (`internal/capsule/seccomp.go`) is
default-deny with EPERM, and its allowlist names `chmod`, `fchmod` and
`fchmodat` but not `fchmodat2` (syscall 452, Linux 6.6). Recent glibc
routes coreutils' permission setting (`lchmod`, `fchmodat` with flags)
through `fchmodat2`. The seccomp library already knows the name, so the
omission is ours. The allowlist already has the newer forms of its other
calls (`openat2`, `faccessat2`, `renameat2`, `clone3`), so this is a
missing sibling, not a policy choice. Why arm B's release copy passed is
not established; its command was not read.

Consequence: any engineering build that copies files into the release tree
with GNU `cp` fails in every capsule mode. It is the last blocker
between the Sonnet rewrites and a freeze.

Fix (decided): allow `fchmodat2`, the same permission class as the allowed
`fchmodat`, and add a test that every allowed call's newer variant known to
the library is allowed too, so the next kernel addition fails a test
instead of a build.

## Failure 21: the release copy still fails after 4d45601d0; the cause is ownership, not seccomp (label sonnet-slides-1021d, 05:57-06:03 UTC)

Correction to failure 20. Deploy 4d45601d0 (which allows `fchmodat2`)
did not fix the copy. On trajectory b5475247-02b0-5527-8e87-d805a3b6d2f5,
round 1 ran as designed (the Sonnet delegate's rewrite, reviewed against a
saved copy). Round 2's first build job failed with the same "cp: setting
permissions … Operation not permitted". A second job then succeeded,
because the first had already left the files. Freeze correctly refused it:
the first job's leftover `node_modules` and `dist`, removed by the second,
meant "the workspace changed while the job ran". Round 3 failed the copy
again. Failure 20's seccomp attribution was a hypothesis I wrote as
"strongly supported"; it did not hold. The three syscalls it allowed are
harmless siblings of allowed calls and stay.

Cause (code reading; the guest filesystem was not read):
`nix/autoputer-vm.nix` sets `CHOIR_CAPSULE_LOWER_ROOT = "/"`. The capsule
overlay is the subject tree over the guest's own root
(`executor.go`, `lowerLayers := sourceLower + ":" + e.lowerDir`). The
capsule's root is mapped to host uid 65534 (`capsuleNamespaceHostID`), so
anything under the guest's `/var/lib/artifact/release` is owned by an
unmapped uid inside the capsule. A computer that has applied a release has
that tree; the transaction classifier names `/var/lib/artifact` as a
ledger class. `cp` over an existing release file can write it, but it
cannot set the mode of a file it does not own: EPERM. A freshly minted
computer has no such tree, which explains why arm B's copy passed and the
delegate computer's fails. The earlier fix design anticipated this case
(`adopt-1418-…-2026-10-15.md`: "lower dirs would not be [writable], but
none exist here"). They exist now.

Consequence: on any computer that has applied one release, no later
frontend build can freeze. Every Sonnet rewrite on the delegate computer
is blocked here.

Fix (decided): each capsule starts with an empty release directory of its
own. Before the overlay mounts, the executor creates
`<upper>/var/lib/artifact/release` owned by the capsule's mapped uid and
marks it opaque (`user.overlay.opaque=y`, the userxattr form the mount
uses), so the guest's release tree never shows through and a build writes
only capsule-owned files. Freeze already reads only the upper layer, so it
sees exactly what the capsule built. Red (capsule isolation surface);
test-first.

## Failure 22: the build round passes its checks and then completes as blocked, without a release tree or a freeze (label sonnet-slides-1021e, deploy 871c3c145, 06:48-06:54 UTC)

Evidence (trajectory 34259f8b, trace bundle kept outside the repo; read
from the run events of each engineering assignment):

- Round 1 went as designed. The guest ran 871c3c145. Engineering saved a
  copy of the file, and the delegate rewrote SlidesApp in 15 turns
  ($0.20); the file grew from 21229 to 22216 bytes. The round completed as
  failed, waiting for the build round.
- Round 2 (assignment 745a0430, `ecosystem_proxy`, GPT-6 Luna at low
  reasoning effort) applied `/selfdev/carried/source.patch`, ran
  `npm ci && npm run build` (exit 0, `vite build` done), and then ran
  `diff -u` against the saved copy, which showed the rewrite. It then
  called `choir.Complete("blocked", ...)`: "Saved-original diff remains
  unresolved: it is nonempty as intended for the patch, and git
  comparison unavailable because no .git directory. ... No freeze." It
  never built the release tree, never ran the release copy and never
  called `choir.Freeze`.
- Management's round 2 cast had added: "Stop and report the
  missing-lockfile blocker if checks cannot be completed; freeze only if
  every required check passes." The lockfile claim comes from round 1's
  report, which said `npm ci` in the claude-mode capsule exited EUSAGE on
  a missing `frontend/package-lock.json`. Round 2 found the lockfile
  present.
- Round 3 (assignment 38cfc84c) found no carried patch, saved the
  unchanged file, and completed as blocked ("clean base without .git
  provenance"). Management then judged that another retry was not useful.
  Texture's document now says the rewrite "still needs provenance
  verification". The drill stalled at 06:54.

Reading of the trace (behavior claims):

- The tracer brief says "npm ci && npm run build must pass, and diff -u
  ... must show it changed". `diff -u` exits 1 when the file changed, and
  the brief never says that 1 is the passing result. Engineering treated
  the diff as an unresolved gate. Management's "freeze only if every
  required check passes" turned that doubt into a stop. Every other step
  had passed. This is a drill-brief defect, and the tracer is where to fix
  it.
- The failure 21 fix is deployed but not exercised: this round never
  reached the release copy.

Hypotheses, not findings:

- Round 3 had no carried patch because round 2 completed as blocked with
  a capsule whose diff held the build output as well as the source change.
  The offer either refused that as too large or picked nothing. The offer
  record on the guest was not read.
- Round 1's missing lockfile: the claude-mode capsule may not present
  `frontend/package-lock.json`, or the delegate's npm refusal printed a
  misleading error. The claude-mode capsule's tree was not read.

Fix (drill brief, yellow): say that `diff -u` exiting 1 is the passing
result. Name the round 2 steps in full: build the frontend release tree,
then `choir.Freeze`, then Complete. Say that a missing `.git` is expected
and is never a blocker.

## Failure 23: engineering completes the round while its build job is still running (label sonnet-slides-1021f, deploy f9ad90709, 07:19-07:24 UTC)

Evidence (trajectory be257da2, run events of assignment 9f4eda47, trace
bundle outside the repo):

- Round 1 went as designed: baseline saved, delegate rewrite in 19 turns
  ($0.20), 21229 to 22336 bytes, completed as failed for the build round.
- Round 2 (`ecosystem_proxy`, GPT-6 Luna, low effort): `git apply` of the
  carried patch exited 0. It started the frontend build exactly as the
  engineering overlay prescribes, as a job (`job-76e9ba26ee77`: `npm ci`,
  `npm run build`, the release copy, the cleanup) at 07:23:57.
- At 07:24:13, while that job was running, one cell ran
  `choir.Poll(id)` and then, in the same cell,
  `if v.State != "done" ... { choir.Complete("blocked", ...) }`. Poll
  answered `running`. The Complete committed, and the round's fate was
  terminal at 07:24:20 with the build still in progress. The overlay says
  to end the turn and be woken when the job ends. The completion guard
  (`engineeringJobWait`) only holds a turn that ends *without* a terminal
  fate, so it never ran.
- Round 2 also took its "saved copy" after applying the patch, so its
  diff check compared the file to itself. That is a model error, not a
  substrate gap, and it does not change the fix below.

Classification: substrate. Freeze is already refused while a job runs
(`freezeCapsuleEffectBundle`: "job ... is still running; Freeze waits
until it ends (or choir.Stop it)"). Complete has no such refusal, so any
model that polls early can end a round in the middle of its own build. A
prompt change would only shift the odds. A refusal closes the path for
every model.

Fix (decided; orange, the engineering in-cell API): the reducer refuses a
cell that stages Complete while the run's capsule job is running, with an
instruction to end the turn (you are woken when the job ends with its
outcome) or to `choir.Stop(id)` it in a cell of its own first. The
refusal comes before any side effect, so the cell's fate, the cursor and
the mailbox are untouched. Failure modes written before the code:

1. Complete while the run's job runs: refused, naming the job.
2. Complete with no running job: unchanged.
3. A cell without Complete while the job runs (Poll, Note, Report):
   unchanged.
4. A desk with no capsule jobs (management, Texture, research): unchanged.
5. The refusal must leave no trace: no fate, no cursor advance, no
   envelope.
6. A job that never ends cannot hold the round forever: jobs carry a wall
   clock (at most the `minutes` given to Start), and `choir.Stop` remains
   the way out.

## Failure 24: the release copy fails because the capsule answers xattr calls with EPERM; a failed build job poisons the next one (label sonnet-slides-1021f, rounds 3 and 4, 07:25-07:29 UTC)

This corrects failure 21 a second time. The 871c3c145 fix (an empty,
capsule-owned, opaque release directory) was live in these rounds. The
copy still failed.

Evidence (run events of assignments ac64378b and 962dc239):

- Round 4, first build job (`job-2b20ed0e0e53`), in a fresh capsule
  whose release directory starts empty and capsule-owned: `npm ci` and
  `vite build` passed, then `cp -rf --no-preserve=mode,ownership,timestamps`
  printed `cp: setting permissions for
  '/var/lib/artifact/release/frontend/./assets/...': Operation not
  permitted` for each file, and the job failed.
- The same command again in the same capsule (`job-7631a2c99b1b`) ended
  `done`, exit 0. The second time, every destination file already
  existed.
- Round 3's first job failed the same way.

Cause (from the trace and the code; the copy was not stepped through in
a capsule):

- `cp --no-preserve=mode` sets a default ACL on each file it creates
  (coreutils `set_acl` on a new destination). That writes the
  `system.posix_acl_access` xattr. An existing destination is not
  re-moded, which is why the retry passed.
- The capsule seccomp filter is default-deny with EPERM, and it allows no
  xattr syscall (`internal/capsule/seccomp.go`). The ACL write therefore
  fails with EPERM, which cp treats as fatal ("setting permissions").
  When an ACL write fails with EOPNOTSUPP ("this filesystem has no ACLs"),
  cp falls back to a plain `fchmod`, which the filter allows.
- The overlay's note that `cp -a` and `cp -p` "exit 1 on every file even
  though the data was copied" is the same EPERM on xattr reads and
  writes.
- Failure 21's ownership reading may still be true on a lower release
  tree, but it is not what failed here. The release directory was empty
  and capsule-owned.

Second defect, in the same round: the failed first job stopped at the
`cp`, before its `rm -rf node_modules dist`, so it left the build outputs
in the workspace. The second job started with them present and removed
them, so "the workspace changed while job job-7631a2c99b1b ran", and
Freeze refused the one good build's receipt. Engineering then completed
the round as blocked.

Fix (decided):

1. Capsule seccomp (red, capsule isolation; strictly non-widening): the
   xattr syscall family (`setxattr`, `lsetxattr`, `fsetxattr`,
   `getxattr`, `lgetxattr`, `fgetxattr`, `listxattr`, `llistxattr`,
   `flistxattr`, `removexattr`, `lremovexattr`, `fremovexattr` and the
   `*xattrat` variants) stays refused, but with EOPNOTSUPP in place of
   EPERM. Nothing new is allowed; only the refusal's errno changes, to
   the one tools treat as "no xattrs here". Failure modes: an xattr call
   must still have no effect; every other refused call keeps EPERM; the
   broker filter gets the same treatment.
2. Engineering overlay build recipe (orange, prompt default): the
   cleanup runs whether or not the build succeeded,
   `{ npm ci ... && cp ...; }; s=$?; rm -rf node_modules dist; exit $s`,
   so a failed job leaves the workspace as it found it.

## Failure 25: management refuses the build round because it thinks it must supply the carried patch (label sonnet-slides-1021g, deploy 16a5aa0e0, 07:49-07:54 UTC)

Evidence (trajectory 06225c0d, records from the trace bundle):

- Round 1 went as designed (assignment 9663ae73): baseline saved, the
  delegate rewrote SlidesApp in 21 turns ($0.21), and the round
  completed as failed: "the required second-round Cast with
  network=ecosystem_proxy and carried source.patch has not arrived ...
  Awaiting management's round-two authenticated assignment."
- Management (07:53:28) relayed that to Texture as a blocker: "the
  ecosystem-enabled round with the carried patch never arrived ...
  Provide the exact patch artifact and provenance before another bounded
  verification attempt."
- Texture re-cast. Management (07:54:05) refused to cast: "The exact
  carried SlidesApp patch and its provenance are not present in the
  available workflow record. Following the request's no-substitution
  condition, I did not launch engineering." The drill stalled.

Reading: engineering's "awaiting round two and the carried patch" means
"cast the next round". Management read it as a missing input that
management has to produce. The management overlay says to add
`"network": "ecosystem_proxy"` for "the round that applies a carried
patch". It never says that the platform derives and mounts that patch
for the next engineering Cast on the same request by itself
(`offerCarriedPatchForAssignment`). Nothing management can see names
the patch, so it found none. Only engineering's run context gets the
carried-patch notice.

Fix (prompt default, orange): the management overlay says that the
platform carries a failed round's source changes into the next
engineering Cast for the same request without any input from
management, that the new capsule finds them at
/selfdev/carried/source.patch, that management never supplies, attaches
or looks for the patch, and that a report awaiting round two or the
carried patch means: Cast the next round. Test-first, beside
`TestManagementOverlayNamesEveryCastNetworkMode`.

## Failure 26 and the structural assessment: eight SlidesApp drills, no freeze (label sonnet-slides-1021h, deploy 10132ed5b, 08:20-08:30 UTC)

Drill h (trajectory in the trace bundle outside the repo):

- Round 1 ran as designed (delegate rewrite; completed as failed).
- After Texture's re-cast, management cast round 1 again, in claude
  mode, and the delegate rewrote the file a second time (10 turns,
  $0.26). Management did cast after the failure 25 fix, but the round it
  cast was the wrong one.
- Round 3 got the carried patch (mounted, 18100 bytes). Engineering then
  called `choir.WriteFile(".../SlidesApp.svelte", "")`, found it could not
  restore the file without git, and completed as blocked over
  "provenance". That word came from management's cast, which asked it to
  locate "saved-copy comparison provenance".

Clustering assessment (the repo rule: three or more iterations without
convergence means stop patching and escalate):

- Eight drills (a-h) produced failures 19-26. The Sonnet delegate's own
  work succeeded every time: 10-25 turns, $0.13-0.28 a round.
- The substrate defects were real and are fixed: the xattr errno that
  broke every first release copy (24), a Complete that could end a round
  in the middle of its build (23), a failed build that poisoned the next
  one's receipt (24), a capsule seeing the guest's release (21), and the
  carried patch's secret and mask files (16).
- The stops that remain are reasoning failures across a handoff that
  takes three desk turns: engineering reports "awaiting round two",
  management decides what that means and writes a new brief, and a fresh
  engineering capsule decides whether to trust the patch. Each turn
  restates the task in its own words ("provenance", "round 1 again",
  "freeze only if every check passes"), and each restatement is a new
  chance to stop. Engineering and management run GPT-6 Luna, at low and
  high effort. A prompt change moves the odds for one wording. The next
  wording finds the next stop.
- The root is structural: the delegate's capsule cannot build, so every
  delegate rewrite needs a second, model-mediated round.

Options for the owner (not patched; this needs a decision):

1. One round. The claude-mode capsule keeps the delegate's sandbox
   refusing npm, and the engineering desk's own build job in the same
   capsule reaches the package ecosystem through the ecosystem proxy.
   The delegate writes, engineering builds and freezes, and nothing is
   handed off. Cost: the claude-mode capsule's jobs get package-registry
   egress (proxied and allowlisted, as in `ecosystem_proxy`). This is a
   network-grant decision (security first).
2. A platform-driven round 2. When a claude-mode round completes as
   failed with a changed tree, the platform itself opens the
   `ecosystem_proxy` round with a fixed recipe: apply the carried patch,
   run the overlay's build job, Freeze, Complete. No desk restates
   anything. Cost: a harness step that knows about delegate rounds.
3. Raise the engineering desk's reasoning effort for self-development
   assignments (model policy). This is the cheapest change, and it moves
   only the odds.

Recommendation: option 1, if the owner accepts registry egress for jobs
in the delegate's capsule (the delegate stage stays refused); otherwise
option 2. Until the owner decides, the five remaining app rewrites (Pdf,
Epub, Podcast, Calendar, FileBrowser) wait, because each would hit the
same handoff.

## Decision on failure 26 (2026-10-21, owner notes 34)

The owner chose option 1. The security panel on its design
([delegate-one-round-build-2026-10-21.md](../design/delegate-one-round-build-2026-10-21.md))
showed that registry egress inside the delegate's capsule could not be
confined to the build, so option 1 landed in a narrower form with no new
egress: the guest image carries the frontend's Nix-built dependencies
and the build job links them when the package files match (df93ab7f7,
deployed). A delegate rewrite is one round; a rewrite that changes
dependencies keeps the two-round path. The SlidesApp drill on the
delegate test computer is the acceptance.

## Failures 27 and 28: the first one-round drill (label sonnet-slides-1021i, deploy 219535ef3, 10:21-10:24 UTC)

Read from the trace (bundle outside the repo, trajectory
dbadbdfd-579b-5358-a477-b024bd6de6d7, assignment
assignment-f138ade610d43435de66afeb):

- The frontend build job linked the image's dependencies and ended done
  with exit 0 in about 17 seconds, in a capsule with no network at all.
  The one-round build mechanism works.
- **Failure 27, management withheld the network.** Management cast with
  `"network": "forbidden"`, though the drill brief says to leave the
  field out, so the capsule had no delegate and the engineering desk
  (GPT-6 Luna) edited the file itself. Management's overlay still says a
  delegate capsule cannot run a frontend build ("no npm install or
  frontend build runs there"), stale since df93ab7f7, and offers
  "forbidden" beside it. Hypothesis, not confirmed by its reasoning text
  (the cell shows only the Cast): it chose the option that read as
  sufficient and least privileged.
- **Failure 28, Freeze refused the running binary.** `choir.Freeze` was
  refused: "the running binary was built from 219535ef3 but the capsule
  source is pinned to 4c2d66c38; build bin/autoputer"
  (`internal/capsule/release_binary.go`, `PlanReleaseBinaryReuse`). The
  delegate test computer's source is versioned on its own and has stayed
  at its creation commit (2026-10-20, 215 commits behind) while its
  guest image follows every deploy. A frontend-only freeze reuses the
  running binary only when the two commits match, so on this computer
  every frontend change needs bin/autoputer built from the workspace
  source. The engineering overlay and the drill brief both say a
  frontend-only change needs no bin/autoputer, so engineering completed
  as blocked. The refusal is correct; the instructions are what is wrong.

## Failure 29: the delegate leaves Claude Code's sandbox placeholders in the source tree (label sonnet-slides-1021j, deploy be47b14bf, 10:42-10:52 UTC)

After the failure 27 fix management left "network" out, the delegate ran
in claude mode and rewrote SlidesApp.svelte twice (71 s and 95 s, about
$0.24 and $0.35, Sonnet's report naming only that file). Both build jobs
failed the same way: esbuild, loading vite.config.js, read
`/workspace/platform/package.json` and found it empty ("Unexpected end of
file in JSON, ../package.json:1:0"). The repository has no root
package.json, and the no-delegate capsule of drill i built cleanly.

Cause, read from the pinned Claude Code 2.1.300 binary in the guest
closure: with `CLAUDE_CODE_SUBPROCESS_ENV_SCRUB=1` (set by the delegate
design so Bash children never see the token), Claude Code creates
placeholder files at startup, which its bubblewrap sandbox then masks:
in its working directory it opens for append (creating them empty)
`.npmrc`, `.yarnrc`, `.yarnrc.yml`, `bunfig.toml`, `package.json`,
`.gitmodules`, `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml` and the
`.env*` family, creates `node_modules/.bin`, and creates a stub `.git`
(`info/exclude` appended, `config.worktree`, `commondir`, `modules`,
`worktrees`, `glab-cli`). The delegate's working directory is
`/workspace/platform`, so all of these land in the source tree and stay
after the run. The empty root package.json is what esbuild read. The stub
`.git` and the rest also enter any diff of the tree; whether they reached
earlier drills' carried patches is not checked (a hypothesis to test).

Fix direction: the delegate supervisor (trusted, outside the delegate's
process) records which of those paths are absent from the working
directory before it starts Claude Code and removes the ones that appear,
once the run has ended. The scrub stays on.

## Failure 30: a refused Freeze leaves its self-development operation executing, and it blocks every later freeze on the computer (label sonnet-slides-1021k, deploy 37db27741, 11:10-11:15 UTC)

What worked first: the one-round build. Engineering rewrote SlidesApp in
the delegate's capsule, the saved-copy diff passed, and the frontend build
linked the image's dependencies and finished in about 24 s with no
network (job `job-7c7024e2ef1a`, 11:12:13 to 11:12:37).

Then every `choir.Freeze` was refused: "another self-development operation
is open on this computer: selfdev-20d9baa5eb36704d52009b6876611f98 is
executing". That operation was opened at 10:23:37 by drill i's Freeze,
which was refused for the running-binary/source mismatch (failure 28).
Drill i's round then completed as failed at 10:23:50, and the operation
stayed `executing`. Round 1 of drill k completed `blocked` at 11:12:53;
management's round 2 (11:14) hit the same refusal; management then
reported that it found "no safe audited close action" and asked for an
approved close, which is the right refusal for a desk. Texture's round 3
cast at 11:15:12 drew no management cast in the next 34 minutes; the
drill was stopped at 11:49.

Classification: substrate (self-development operation lifecycle), not a
prompt. An operation's fate is not tied to the round that opened it: a
round that ends failed or blocked leaves its operation open, and the only
closer today is the boot-time crash close. Hypothesis to check in code:
the operation is opened before the source check that refuses the Freeze,
and nothing closes it on that refusal or on the round's terminal report.

Fix direction (not chosen): the refusal that follows opening closes the
operation with that cause, and a round's terminal report closes any
operation it opened that is not frozen. The delegate computer needs that
stale operation closed before the next drill; no hand edits of state.


## R1 reached on the completed path (2026-10-26, Reader round 5c)

Trace review round 8 (docs/evidence/selfdev-trace-reviews-2026-10-25.md,
R8-1). On b@c, operation selfdev-60f57884, engineering's `choir.Freeze`
succeeded at 05:21:04.9; its `Complete("completed", ...)` at 05:21:14.4 was
refused "capsule execution evidence does not bind the final frozen subject:
latest receipt observed 77fc3b13..., frozen subject is 3282e7c2..." after the
capsule had frozen durably (`freeze_requested` 05:21:14.626, `frozen`
05:21:15.155). Every later cell, `Complete("blocked")` included, was refused
"assigned capsule obligation is no longer executable: assignment fate is not
active and bound" (engineering_assignment_tools_overlay.go:141), and the run
ended by the completion guard at 05:21:55. Residual R1 above (cited refs are
checked after the durable freeze intent) is the cause: the observed binding
cannot help, because the doorway refuses first. Status: open, red; fix next,
tests first: check the cited receipts against the tree that will freeze
before the durable freeze intent, so a refused binding leaves the assignment
active and the desk can cite the right receipts or report blocked.

### Fix design (R1), written before the code

Mutation class: red (run acceptance, completion evidence, the engineering
terminal saga).

The cited receipts are checked against the tree that is about to freeze
before the durable freeze intent. A new executor read,
`PreflightGrantedExecutionReceipts`, digests the live worktree of the
assignment's capsule and runs the same portable binder the post-freeze
resolver runs (`bindGrantedReceipts`: run, handle, capsule and source
snapshot per receipt; the certified rule for `completed`, observed for
`failed` and `blocked`). It mints and persists nothing. The saga calls it
only on the branch that would write the freeze intent (capsule `active`, no
staged proposal), so a refusal returns with the assignment `bound`, the
capsule `active` and unfrozen, and no pending proposal: the doorway stays
open, and the desk can cite the right receipts or report `blocked`. The
post-freeze bind stays the authoritative check.

Failure modes considered:

1. The worktree changes between the pre-check and the quiesce. The Complete
   reduces after its cell's code ran; a running capsule job refuses the
   Complete before any effect (`refuseCompleteWhileJobRuns`); the run's
   cells are sequential. What remains is a process still alive in the
   capsule's cgroup (a backgrounded server, a delegate's child), which can
   write until `Quiesce` freezes the cgroup. Narrow but real, so the
   post-freeze bind is kept.
2. The post-freeze bind still refuses (that race). The refusal is
   deterministic: the tree is frozen and receipts are content-addressed
   files, so the re-drives (six, five minutes apart) can never succeed. The
   staged proposition holds the terminal slot (`SlotTerminalReport`), and
   `frozen -> active` is not a transition the append-only fate history
   admits, so neither a corrected report nor a thaw is available without a
   state-machine change. The saga ends the assignment there: on the typed
   refusal (`capsule.ErrFinalSubjectNotCertified`) and only on it, it
   revokes the capsule (the stopped-round revoke) and records a non-silent
   system cancellation whose reason names the refusal. The frozen tree
   survives the revoke only as an unfinished patch, and only when
   `engineering.carry_unfinished` is on and the binding is an
   implementation; otherwise it is destroyed (residual R1-b below). The store records that as a `failed` report,
   management is told, the self-development operation closes. Other
   post-freeze errors (I/O, a persist) keep today's re-drive. If the
   cancellation itself fails after the revoke, the re-drive meets the
   revoke intent, conflicts, and the existing limit cancels it.
3. The pre-check refuses what the post-freeze bind would accept. Same rule,
   same inputs except the tree; a refusal changes no state and the desk can
   retry. Fails closed and recoverable.
4. Authenticity differs between the two checks. It cannot: run, handle,
   capsule and source snapshot do not depend on the tree, and the receipts
   are immutable. Only the final-subject rule can change its answer.
5. `failed` and `blocked` cite a foreign or unknown ref. The observed
   pre-check refuses it before the freeze too; before this change it was
   refused after the freeze and stranded the assignment (the same class).
6. Cost: one more worktree digest per terminal Complete. Accepted; the
   live digest cannot be reused for the frozen one.
7. Resume, replay, crash. No new state, no boot-path scan, no mode flag, no
   schema change. A staged proposal (freeze requested, frozen, revoke
   requested, revoked) skips the pre-check, as the planned-restart and
   watchdog resumes do today. Zero cited refs skip it, as the post-freeze
   bind does. A partial report never freezes and is not touched.
8. The run after a race cancellation. The Complete returns the refusal and
   says the assignment ended; the run's next cell is refused at the
   doorway and the completion guard ends the run. Named residual: the run
   is not stopped from inside its own reduce.

Residuals named after review 281 (Devin, DeepSeek v4.1 flash; approved):

- R1-a (Devin 1, DeepSeek 1). The pre-check covers only the capsule binder
  (run, handle, capsule, source snapshot, the final-subject rule). The other
  deterministic post-freeze refusals are not in it and are not typed, so
  they still strand the assignment until the six-re-drive limit cancels it:
  a command-digest or capsule mismatch, a duplicate or missing granted ref,
  an exit-code mismatch, an attestation build error, a count mismatch
  (`bindFrozenAssignmentExecutionReceipts`), and the commit-time report
  validation (`RecordEngineeringAssignmentReport`, the attestation and
  `ValidateAgainst` checks, `internal/store/engineering_assignments.go`
  about line 2023). None is reachable through the reducer path today: the
  reducer builds the report's commands from the same receipts and dedupes
  the refs. Follow-up: run them in the pre-check, or type them so the saga
  ends the assignment on them as it does on the final-subject refusal.
- R1-b (DeepSeek 2). In the race branch the frozen tree is kept only by the
  stopped-round carry (`engineering.carry_unfinished` on, implementation
  binding). Follow-up: export the content-addressed candidate while the
  capsule is frozen, before the revoke, so a race-cancelled round's tree is
  always recoverable.
- Wording (DeepSeek 3). A proposal staged before this fix (no pre-check
  ran) that is re-driven meets the same cancellation; its reason no longer
  claims a pre-check ("the cited receipts do not certify the tree that
  froze").

Tests first: the staging shape through the real saga with a fake executor
(a Freeze already happened; `Complete("completed")` whose latest receipt
observed the pre-edit tree is refused with the assignment still active; a
corrected `Complete("completed")` and, separately, a `Complete("blocked")`
then record terminal fates), and the race (the tree changes after the
pre-check; the assignment ends cancelled with the refusal as its cause,
not stranded).
