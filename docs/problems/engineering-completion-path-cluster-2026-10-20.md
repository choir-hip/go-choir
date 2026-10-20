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
