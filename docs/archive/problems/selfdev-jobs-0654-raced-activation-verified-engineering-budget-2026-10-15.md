# Self-Development Go Drill selfdev-jobs-0654: raced-activation fix verified; engineering lost its broker for good after one cell timeout (2026-10-15)

Date: 2026-10-15 (drill 06:54-07:40 UTC). Overnight lander, step 0.
`node scripts/reeng_tracer.mjs --label selfdev-jobs-0654 --scenario engineering --change go --expect-sha f704110ce7f9… --timeout-min 45`
on staging build `f704110ce` (docs-only commit over `da80c5fcc`, which carries
raced-activation fix 2, `44944dc72`). Computer
`computer-aaf1a244383c4a610d86030f40b68a87`, VM
`vm-6bd99e1298dffecabfa5c4957925962f`, trajectory
`824d458a-2ed5-5d2c-a055-29eeac752eb7`.
Status: **traced** (the run ledger was read, 2026-10-15 UTC afternoon). The
earlier revision of this doc classified the stall as the unrepaired broker
RPC desync; that classification was **wrong** (see Correction). No code
change here.

Evidence: `~/.config/choir-qa/traces/selfdev-jobs-0654/receipt.json`; the
stopped computer's data disk read read-only with `debugfs rdump` (Dolt store
under `state.texture/texture`, `og_objects` kinds `choir.event`,
`choir.co_super_assignment[_report]`); extracts in
`~/.config/choir-qa/traces/selfdev-jobs-0654/ledger/` (`ev.json`,
`timeline.txt`); guest console; gateway journal.

## Outcome

`acceptance.pass=false`, `reached=engineering_final_report`; failed:
frozen, final_report_names_bundle, operation_on_request_trajectory,
verified_against_named_bundle, owner_approved, applied, one_planned_restart,
route_receipt, restore_to_pre_apply_head, go_build_job_done,
go_test_job_done. The drill never reached freeze, so apply and restore were
not exercised.

## What the fix did (the thing under test)

- Console: `texture api: recovered raced activation for doc d1886c52… run
  9752fb21…` at 06:55:11 (the second-path fix firing).
- No `dispatcher: poison`, no `mutation authority mismatch`. The story reached
  management (06:55:19) and engineering (06:55:39). The jobs-0458 poison did
  not recur.

## Correction: the broker-client fix IS in this build, and it is the cause

The first revision said `5c5db2027` (broker client drops its connection after a
transport error and re-dials) "is on branch `fix/selfdev-go-drill-0101`, which
is not on main, so this build does not carry it". Wrong: it landed on main as
**`b4f778552`** (author date 2026-10-14 21:57 -0400) and
`git merge-base --is-ancestor b4f778552 f704110ce` succeeds, as it does for
`2eae77de6` (the P5 drill build); it is not an ancestor of `e83735f2a` (the
0101 build). This build carried the fix, and the ledger shows the fix
**converts a one-off cell timeout into a permanently dead broker client**.

## Ledger, assignment `assignment-e5b261a1b12e494343694d9f` (engineering, round 1, ecosystem_proxy)

`disposition: cancelled`, report `result: failed`, reason "tool loop:
completion guard `engineering_overlay_terminal_fate_required` was not
satisfied after 2 retries; assignment cancelled". Timeline:

- 06:55:54 loop.started. 06:56:05 `choir.WriteFile` of the two new
  `internal/drillmark/*` files: ok (56 and 173 bytes).
- 06:56:08-06:57:03 the prescribed frontend cell (`npm ci ... cp -a ...`)
  panics (non-zero exit, `pthread_getschedparam failed: 1` noise). The model
  retries with `cp -r`, then `rm -rf` the release dir first; 06:57:03 exit 0.
  The release tree is built.
- 06:57:07 `choir.Start(go build -o /var/lib/artifact/release/bin/autoputer)`
  `job-fe0f3ba44c80` (wall clock 20 min). 06:57:12 a second `Start(go test)`
  `job-6584a3d727f3` (refused: "job-fe0f3ba44c80 is still running; one job runs at a time in a capsule").
- 06:57:14-07:03:40 the model **busy-polls**: `choir.Poll("job-fe0f3ba44c80")`
  every ~2.5 s, 126 cells, each printing a ~4 KB `go: downloading ...`
  tail. The prompt says to end the turn and be woken.
- 07:03:40 (`1cb3b5e3-e55a-4fae-8a82-20fe51727f1e`) context compaction at 166k
  tokens with `checkpoint_status: emergency_fallback` (239 messages dropped, 32
  kept). The model forgot it had built the frontend and started the build.
- 07:03:51-07:04:00 it re-wrote the files and re-ran the **first** prompt
  command `npm ci --no-audit --no-fund && npm run build && ...` while
  `go build` was running in the same capsule.
- 07:04:30.68 (`f15dae2b-2454-4bd5-8a89-bfb8f2f5f10b`) the cell exceeded the
  30 s broker read deadline:
  `failed to read broker response: read unix @->/proc/self/fd/61/broker.sock: i/o timeout`
  (console: `slow tool call ... dur=30.087s`). Under CPU contention from the
  Go build, `npm ci` took longer than the 17 s it needed at 06:56.
- 07:04:33.00 (`5d3e4cca-3e38-4d13-845b-081f117c1e5c`) and every call after,
  to 07:18:17: `failed to connect to broker socket /proc/self/fd/61/broker.sock:
  dial unix /proc/self/fd/61/broker.sock: connect: not a directory`. Even
  `choir.Records()` and `choir.Complete(...)` (07:04:39, 07:18:12, 07:18:16)
  fail. The model correctly concluded "capsule broker became unavailable".
- 07:04:40-07:18:07 (13.4 min, no event): the turn ended while `job-fe0f3ba44c80`
  was running, so `engineeringJobWait` (`internal/agentcore/capsule_jobs.go:211`)
  waits for the job's end, bounded by the job's wall clock plus `jobWaitSlack`
  (1 min): 06:57:07 + 20 min + 1 min = **07:18:07**, which is the exact time of
  the next event, `loop.retry engineering_overlay_terminal_fate_required`
  (`4d77ee83-1729-48f3-92e6-d06dec4e445a`). The job-end wake could not arrive
  earlier either (the job watcher's `job_status` also rides the dead client).
- 07:18:07-07:18:21 two guard retries, both `Complete` calls fail on the
  broker; 07:18:21 `loop.cancelled`
  (`cecb8e48-1b85-4470-ba4f-4150671a6f2e`). The report is the system's own
  `cancel-report`, not an engineering report.

## Root cause of the dead broker (code, high confidence)

`startBrokerLocked` (`internal/capsule/executor.go:549`) calls
`unixSocketAddress(hostSocket)`. Capsule state sits deep on the data disk, so
it returns `/proc/self/fd/<N>/broker.sock` through an open **directory
descriptor**, and `defer closeDialSocket()` closes that descriptor when the
function returns. `NewBrokerClient(dialSocket, ...)` stores the string and
keeps one connection for the capsule's life. Before `b4f778552` the client
never dialed again, so the stale path was harmless. `b4f778552` drops the
connection after any transport or framing error and re-dials the stored path;
descriptor 61 was closed long ago (and has been reused by something that is not
that directory, hence `ENOTDIR`), so every re-dial fails for the rest of the
capsule's life. The same string is baked into the error text
(`/proc/self/fd/61/...`), which is how the ledger shows it.

The fix's tests passed because the test sockets are short paths, so the
descriptor-path branch never ran (hypothesis; the unit tests were not re-read
here). The earlier hang class (shifted answers) is gone; the replacement
failure is worse, because the whole assignment is lost at the first timeout.

## Aggravators seen in the same run (not the cause)

- Busy-polling plus no compaction summary (`emergency_fallback`): the model
  lost state at 07:03:40. Engineering overlay prompt problem.
- A 30 s per-call broker deadline is shorter than `npm ci` under a concurrent
  Go build.

## Proposed fix (not implemented; red, capsule)

Substrate: the broker client's address must not depend on a descriptor that
the caller closes. Either keep the directory descriptor open for the
capsule's lifetime (close it in `Destroy`) or give the client a re-dial
function that re-derives the address (open dir, dial, close) each time.
Add a test whose socket path exceeds 107 bytes and which re-dials after an
injected timeout. No unwired replacement. Confidence: high (about 90%), the
mechanism is visible in both the ledger error text and the code; it is not yet
reproduced on a Linux guest. Reasonable follow-on: engineering prompt to end
the turn instead of `Poll` loops.

Judgment for the lander (revised): this is a regression exposed by the broker-client fix
(`b4f778552`), not an unlanded 0101 fix; the
raced-activation behaviour improved. The self-development Go drill is blocked
by this and by the engineering-capsule defects in
[p5-go-served-0825](p5-go-served-0825-engineering-never-freezes-2026-10-15.md).

## Heresy delta

discovered: stale descriptor path in the re-dialing broker client; introduced:
none in this doc (the regression is in `b4f778552`); repaired: none.

## Failure modes considered for the fix

Written before the code (repo testing rule). The fix: the broker client keeps
the **real** socket path and derives the short `/proc/self/fd/<N>/...` address
on every dial (open the directory, dial, close), instead of storing an address
whose descriptor the caller closed. Class red (capsule broker). Rollback: git
revert of the fix commit.

Ways the system could fail, and what the fix and tests do about each:

1. **Re-dial through a closed descriptor** (the bug). The stored address
   names a descriptor that is closed or reused, so `connect` fails with
   `ENOTDIR`/`ENOENT` for the rest of the capsule's life. Fix: no stored fd
   path. Test: a socket path longer than 107 bytes, a dropped connection
   (injected read timeout), then a successful call, plus a cycle of closing
   and reusing low descriptors between the calls to prove no dependence on a
   particular fd number.
2. **Descriptor leak.** Deriving the address per dial opens a directory each
   time; the readiness loop and every re-dial would leak one fd if the release
   is skipped on a failed dial. Test: the count of `/proc/self/fd` entries is
   unchanged after many failed and successful dials.
3. **Descriptor reuse during the dial.** The directory fd is held open until
   `connect` returns, so nothing can reuse its number while the kernel
   resolves the path. The derivation is per call, so concurrent dials each
   hold their own fd (connections are serialized by `connMu` anyway).
4. **Short paths regress.** A path of 107 bytes or fewer must still be used
   verbatim (the common test path). Existing tests keep covering it.
5. **Name too long even through a descriptor.** `unixSocketAddress` returns
   an error; the dial error must say so and not panic.
6. **The capsule is gone (socket directory removed).** The dial must fail
   with an error naming the real socket path, not an fd path, so the next
   ledger shows the actual cause. Test asserts the error text has no
   `/proc/self/fd`.
7. **Keep-the-fd-open alternative.** Holding one directory fd for the
   capsule's life would also work, but leaks if any teardown path (failed
   spawn, `Destroy` without a broker) forgets to close it, and ties the
   client's validity to a descriptor again. Rejected in favour of per-dial
   derivation.
8. **Concurrent `Close`/`Connect`.** Unchanged semantics: a closed client
   never re-dials; the existing client tests keep covering this.
9. **Test-only coverage gap that hid the bug.** The `b4f778552` tests used
   short temp-dir sockets, so the descriptor branch never ran. The new test
   uses a deep path on purpose and asserts its premise (a plain dial of the
   path fails with `EINVAL`/`ENAMETOOLONG`).
10. **Platform.** `/proc/self/fd` and `unixSocketAddress` are linux-only
    (`//go:build linux`); the new test is a linux test. On a darwin
    workstation it can be compiled (`GOOS=linux go vet`) but not run; run it
    in a Linux container.

Not changed here, by design: the 30 s per-call broker deadline, and the
busy-polling aggravator (an engineering-prompt problem, noted above).
