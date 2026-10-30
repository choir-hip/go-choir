# Capsule Jobs: Long-Running Exec for Engineering Capsules

Date: 2026-10-15. Mutation class: **orange** (capsule runtime behavior),
with red-adjacent edges named in §6. Branch `feat/capsule-long-exec`.

The substrate fix named in
[selfdev-freeze-receipts-refused-2026-10-15.md](../archive/problems/selfdev-freeze-receipts-refused-2026-10-15.md)
(option D, and its clustering assessment: three drill failures since
2026-10-14 share the cost of the Go build inside a capsule whose exec model
is 30 s commands in 60 s cells). `choir.Exec` is killed at 30 s
(`yaegikernel.DefaultExecTimeout`, exit -1) and a cell at 60 s, so a
self-development change that touches Go cannot build `bin/autoputer`
(minutes, a 2 GiB-peak link) or run `go test`. This blocks Station A of
[selfdev-update-architecture-2026-10-15.md](../archive/reports/selfdev-update-architecture-2026-10-15.md).

This note records the design, the failure modes written before the code,
and the residuals. It reuses the mechanics of the Claude delegate's
slice 1 ([claude-delegate-slice1-2026-10-14.md](claude-delegate-slice1-2026-10-14.md)):
detached process group, kill TERM then KILL after a grace, status file
written atomically, a bounded tail, and its open reaping residual, which
this design closes for both.

## 1. Shape (lead's design, 2026-10-15)

A job is a durable record, not a cell-owned process.

- **In-cell:** `choir.Start(argv, opts)` stages a job-start intent in the
  cell's tray, like Freeze and Complete, and returns the job id at once.
  No handle, goroutine or fd enters yaegi. `choir.Stop(id)` stages a stop.
  `choir.Poll(id)` reads the job as the cell-start snapshot showed it: a
  cheap progress check, never a wait.
- **Reducer:** on cell return the reducer hands a staged start to the
  capsule executor (guest core, outside the interpreter). The cell's tool
  result names each start's outcome (`job_results`).
- **Executor:** checks the capsule, the one-job rule and the bounds, asks
  the capsule broker to start the job, then watches it. On the job's end
  it mints the execution receipt (`capsule-exec:sha256:`), stores the log
  by content address, and calls the runtime's completion handler.
- **Broker (in the capsule):** runs the job under a supervisor stage
  (`--isolation-stage supervise`): a child subreaper that starts the
  command through a hardening stage (`--isolation-stage job-exec`: the
  capsule's workload Landlock, capability drop and seccomp filter,
  inet-admitting only in an egress capsule), pumps its combined output to
  the log with a running digest and a bounded tail, enforces the wall
  clock, and kills and reaps every descendant when the command ends.
- **Wake:** the completion handler appends a system `job` record to the
  desk's inbox (`engineering:<assignment>`, sender `job:<id>`) and signals
  the run. The engineering completion guard, which used to refuse a turn
  that ends without a terminal fate, now waits while the capsule has a
  running job and resumes the turn with the job's outcome when it ends.
  The prompt says: start the job, then do other work or end your turn;
  you will be woken.

The receipt binds argv, cwd, the environment digest (the allowlisted
environment the job ran with), exit code, the output digest, start and end
times, the job's state, and the worktree digests at start and end. It is a
`capsule-exec:sha256:` receipt: the bundle validator and the receipt
opener already accept that prefix, so no protected validator changes.

## 2. API as the desk sees it

```go
id, err := choir.Start([]string{"go", "build", "-o", "/var/lib/artifact/release/bin/autoputer", "-ldflags=-s -w", "./cmd/autoputer"},
    map[string]any{"minutes": 20})            // cwd defaults to /workspace/platform
st, err := choir.Poll(id)                    // st.State, st.ExitCode, st.Tail, st.ReceiptRef (cell-start snapshot)
err = choir.Stop(id)                         // staged; the job ends with state stopped
```

Options: `cwd` (absolute, default `/workspace/platform`), `minutes` (wall
clock, default 20, at most 45). One Start per cell; Start and Stop are
refused in a cell that stages Freeze or Complete.

States: `running`, `done` (exit 0), `failed` (non-zero exit or the command
could not start), `timeout` (the wall clock killed it), `stopped` (Stop, or
the capsule ended), `interrupted` (a guest restart closed it).

## 3. Failure modes (written before the code)

Process and resource:

- **F1. A job outlives its capsule.** Guard: the supervisor and the
  command carry `Pdeathsig: SIGKILL` from their parents; the broker is PID
  1 of the capsule's PID namespace, so its death kills every process in
  it; the executor's watcher closes the job as stopped ("the capsule
  ended") when the capsule is gone.
- **F2. A job leaves orphans** (a backgrounded child, a `setsid`). Guard:
  the supervisor is a child subreaper (`PR_SET_CHILD_SUBREAPER`); after the
  command exits it kills the command's process group, kills every child
  re-parented to it (found through `/proc`, which the job-exec stage cannot
  read but the supervisor can), and reaps until `wait4` says no children
  remain, bounded by 10 s. Orphans it could not reap are counted in the
  outcome.
- **F3. Zombies accumulate in the broker** (slice 1's residual: the
  broker is PID 1 and reaps nothing). Guard: jobs and the delegate both run
  under the supervisor, which reaps its whole subtree; the broker reaps
  only its direct child, the supervisor, through `exec.Cmd.Wait`.
- **F4. Two heavy jobs exhaust 2 GiB.** Guard: one running job per
  capsule, refused in the executor and again in the broker.
- **F5. A job runs forever.** Guard: the broker's wall-clock timer stops
  the supervisor (SIGTERM, which it forwards to the command group, then
  SIGKILL after a 5 s grace); a backstop SIGKILLs the supervisor's group
  10 s later. The executor refuses a wall clock above 45 min.
- **F6. Output exhausts the disk or the context.** Guard: the log is
  capped at 64 MiB (bytes past the cap are counted, not stored, and the
  outcome says truncated); the tail is 4 KiB in broker memory; only the
  tail reaches a cell, the record and the resumed turn.
- **F7. The job runs with more authority than a cell.** Guard: the
  job-exec stage applies the same workload Landlock, capability drop and
  seccomp filter as the session worker, with inet sockets only in an
  egress capsule (its network mode); the environment is the direct-exec
  allowlist (PATH, HOME, TMPDIR, LANG, the C toolchain and ICU flags, the
  egress proxy settings), never a caller's variables.
- **F8. A job reads or tampers with its supervisor** (to forge an exit
  code). Guard: the command runs in a nested Landlock domain, which cannot
  ptrace (or open `/proc/<pid>/fd` of) a process outside it; the
  supervisor sets itself non-dumpable; the outcome travels on a pipe the
  command never inherits; the digest is computed by the broker from the
  stream, not read back from the file.
- **F9. The cwd is wrong** (the 2239 drill: `choir.Exec` runs at `/`).
  Guard: cwd is explicit, defaults to `/workspace/platform`, must be
  absolute and clean; the receipt binds it.
- **F10. The capsule is frozen** (Freeze quiesces the cgroup) while a job
  runs, or the job writes the tree while Freeze extracts it. Guard: Freeze
  refuses while a job runs ("stop it or wait"); the watcher skips a tick
  while the capsule is not active.

Receipts and Freeze:

- **F11. A failed or timed-out job fills a build or test slot** (the
  2239 drill cited a timed-out build as its toolchain receipt). Guard:
  Freeze refuses any cited receipt whose exit code is not 0 or whose job
  state is not done, in all three slots, naming the slot, the ref and the
  outcome. A cell receipt keeps its meaning: a cell that panicked has a
  non-zero exit code and is refused the same way.
- **F12. A job's receipt claims a tree it did not run on** (the desk edits
  the workspace while a test job runs). Guard: the receipt binds the
  worktree digest at start and at end; Freeze refuses a job receipt in the
  build or test slot whose two digests differ.
- **F13. A receipt is minted from data a cell can forge.** Guard: the
  executor mints from the broker's in-memory status (exit code, digest,
  times), read over the capability-verified broker RPC; the worktree
  digests are the executor's own; nothing is read from files a cell can
  write, except the log copy, which is opened with `RESOLVE_BENEATH |
  RESOLVE_NO_SYMLINKS` and stored only when its digest equals the broker's.
- **F14. Two receipts for one job, or a receipt for a job of another run.**
  Guard: the executor mints once per job, keyed by job id; the receipt
  binds the run, the handle digest, the capsule and the source digest, as
  every execution receipt does.

Wake and lifecycle:

- **F15. The desk polls in a loop and burns its turn.** Guard: Poll is a
  cell-start snapshot, so polling in a loop shows nothing new; the prompt
  says to end the turn; the completion guard waits for the job.
- **F16. The desk is never woken.** Guard: the completion handler signals
  the run's waiter in process, and the job record lands in the inbox
  durably, so a run that was mid-cell sees it in its next `choir.Records()`.
- **F17. A wait outlives the run's budget.** Guard: the wait is bounded by
  the run's context (the activation budget) and by the job's deadline plus
  one minute; on either the guard falls through to its old reminder.
- **F18. Waiting counts against the guard's two retries.** Guard: a wait
  that ended because a job finished is not a retry (`Waited`); waits are
  bounded separately (32 per run).
- **F19. A crash resumes a job.** Guard (CLAUDE.md restart rule): jobs live
  in the capsule, which does not survive a guest restart; the executor
  persists a record per job and at construction closes every open one as
  interrupted ("interrupted by a restart"). Nothing re-dispatches a job.
- **F20. A job is started for a desk that is not engineering, or after
  the assignment ended.** Guard: Start is in engineering's module set only;
  the reducer refuses a job intent from any other desk and requires the
  current assigned capsule (the same obligation check every cell runs).

## 4. Tests

Linux-only parts run in Docker (`golang:1.26`), as slice 1's did.

- capsulejob: start, poll and complete; the wall-clock kill; Stop; the
  one-job rule; orphan reaping (a `setsid` grandchild and a backgrounded
  one); the cwd; the log cap, digest and tail; the supervisor's outcome.
- capsule: the job receipt (fields bound, prefix, re-opened after a
  restart); interrupted closure at construction.
- agentcore: a done job's receipt satisfies the Freeze slot check; a
  failed, timed-out, or tree-changed one is refused per slot; the job
  record's shape; the guard waits for a running job and resumes.
- yaegikernel: Start stages one intent with its id and defaults; quotas;
  Poll reads the snapshot; engineering-only.
- runtimeprompts: the overlay teaches start, wait and cite for Go builds
  and tests, and keeps the no-Go path.

## 5. Residuals

- **R1. A cgroup per job.** Not built. cgroup v2's no-internal-processes
  rule means a child cgroup with its own memory limit needs the capsule's
  processes moved into a leaf (`capsule/main`) and the subtree delegated to
  the broker's mapped uid: a capsule lifecycle change (red). The capsule's
  own cgroup still bounds the job's memory, CPU and pids, and the one-job
  rule keeps two heavy builds apart. The supervisor plus the process group
  give kill-all without it.
- **R2. A persistent per-computer GOCACHE.** Not built. Today the job's
  GOCACHE is `/tmp/.cache/go-build` inside the capsule (`HOME=/tmp`), gone
  with the capsule. A shared writable cache across capsules is a cache
  poisoning channel: a capsule could write an entry under a valid action
  key that a later trusted build reuses. The safe shape is a read-only seed
  (built by the computer's own trusted build, never by a capsule) mounted as
  an overlay lower layer under `/var/cache/choir-go` (a path the classifier
  ignores, so it is never part of the candidate), with each capsule's writes
  in its own upper layer. The seam: the job environment's GOCACHE is the one
  place to point at it.
- **R3. Memory leases.** The executor has a job admission hook
  (`SetJobAdmission`) called before a job starts and released when it ends:
  the seam for virtio-mem leases (branch `feat/memory-hotplug`). Nothing
  installs it here.
- **R4. A rule-settled precommitment on job start** ("succeeds within N
  minutes"). Not built; `choir.Precommit` with `tests_pass` still settles
  from the receipts Freeze cites.
- **R5. A job's wall clock can exceed the run's remaining activation
  budget.** The run then ends while the job runs; the capsule's end stops
  the job. Not clamped here.
- **R6. The watcher shares the broker connection** with cells (one
  in-flight RPC per connection), so completion is noticed at most one cell
  late while a cell runs.

## 6. Mutation class and protected surfaces

Orange overall: capsule runtime behavior (a new broker verb set and
stages, executor job lifecycle), a new in-cell verb set, and prompt
framing. Red-adjacent edges, named: the Freeze evidence check is tightened
(refusals added for failed, timed-out and tree-changed receipts; nothing
it refused before is now accepted); the capsule broker gains verbs and
stages (guest TCB); the Claude delegate's process launch goes through the
supervisor. Rollback: revert the commits on this branch; no stored data
migrates (receipt fields are additive and omitted when empty, so existing
receipts re-open with the same digest).

Heresy delta: discovered — slice 1's broker reaps no orphans (it is PID 1);
engineering inbox wakes have no handler (a record addressed to a parked
engineering run wakes nothing); introduced — none known; repaired — the
reaping gap (F2, F3) and failed receipts filling Freeze slots (F11).

## 7. A Go-change drill

`scripts/reeng_tracer.mjs --scenario engineering` asks only for a static
frontend file, which never touches a Go input, so it never exercises a
job. The Go-change drill is the same scenario with `--change go`: the
story asks for one new Go file and its test, so Freeze must build
`bin/autoputer` (`release_binary_origin` built, not reused) and the desk
must use jobs for the build and the test:

```
node scripts/reeng_tracer.mjs --label selfdev-jobs-<n> --scenario engineering --change go \
  --expect-sha <sha> --timeout-min 45
```

The claim adds: the frozen operation's bundle has a built release binary,
and its build recipe and test receipts are job receipts (command `job ...`)
with exit code 0.
