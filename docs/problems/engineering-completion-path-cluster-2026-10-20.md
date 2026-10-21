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
