# Data system cutover: C0 drills on d@e, 2026-10-30

Class: green (evidence record, no runtime change). Rollback: revert.

Scope: the C0 drills that the ledger owed after C5/C6 (kill -9 durability, stale-epoch append,
hibernate and wake, bounded peak memory) on the drill computer d@e only:
VM `vm-70070cf7337d45f2d46eafc83b9783b9`, computer `computer-59c902c5d46caecc0a180a8557164e72`,
user `c061de6f-e4ea-4b28-b126-aa2f066dd890`, desktop `primary`. The paper and the owner's computer
were only read (procmem, console grep, `/proc`), never acted on.

Release on d@e during the drills: `3c1d0767a2ba` (boot at 16:32 UTC); the deploy of `4e6f28d7154`
refreshed all three computers at 16:55-16:56 UTC during the memory drill.
Raw logs: scratchpad `drills/` (acks-*.log, fails-*.log, heads-*.log, receipts-*, verify-*, list-*.json,
hwm-*.log). Times are UTC; client and Node B clocks agree to within a few ms (same NTP) except where noted.

Summary:

| Drill | Result |
| --- | --- |
| 1 kill -9, durable write (precommit claims), 3 runs | pass: 1011 acknowledged, 0 lost |
| 1 kill -9, `/api/files` write (run A) | FAIL: 313 of 313 pre-kill acknowledged writes lost (see defect D1) |
| 2 stale-epoch append | pass at the capability fence (runtime probe with captured real requests); the in-append writer-epoch check (C3) is not built |
| 3 hibernate and wake | pass, with one note (wake appends one `key_revoked`) |
| 4 bounded peak memory | blocked/inconclusive: workload-confounded, no controlled data-size experiment possible; numbers below |
| first snapshot on d@e | found: 16:35:27, head 12772, 61,730,816 bytes in 1.836 s |

## Method note

Commands: choir CLI built at scratchpad `choir`, `CHOIR_API_KEY=$CHOIR_TEST_D_E_API_KEY`,
`-computer computer-59c9...`. Host tape head read on Node B from the control DB
(`computer_event_heads`, `computer_event_append_receipts`) with the password read from
`/var/lib/go-choir/secrets/pg-app-password` on the host, never on a command line. Kill:
`ssh node-b sudo kill -9 $(cat .../vm-70070cf7.../firecracker.pid)`. Recovery: after SIGKILL the
computer goes `degraded` and a request does not wake it
(`computer_waking_failed ... a computer in state "degraded" is not woken by a request`); the owner
act `choir computer recover -idempotency-key ...` brought it back in 9-12 s in every run.

## Drill 1: kill -9 durability

Failure defined beforehand (fixed before each run):

- F1: any write acknowledged to the client before the SIGKILL is absent, or differs when read back through the public API after recovery.
- F2: the host tape head sequence observed after recovery is below the highest sequence sampled before the kill, or any of the last 300 pre-kill receipt rows (sequence, kind, event_digest) differs afterwards.
- F3: the computer does not return to `active` and serve within 5 minutes of the recovery act.
- Not a failure: a write in flight at the kill that is unacknowledged, whether or not it landed.

Harness: 4 parallel writers, ~17 acknowledged writes a second, each logging `ACK` only when the CLI
returned success with the product's acknowledgement body; a sampler read the host head every ~0.4 s;
the SIGKILL fell at a chosen delay after the start; the owner act `computer recover` followed
~10 s later; writers kept running; then every ACK was read back.

Write choice. I started with `choir files put` (run A), which turned out not to be a
durable-on-ack write (D1). Runs B-D use `choir precommit add` (a precommitment record: one SQLite
commit in the guest store, `synchronous=FULL` via `dbx`, one host tape event per claim, ack after
the commit), idempotency key per write, read back with `choir precommit list` and compared on id and text.

| Run | Write | SIGKILL (Node B clock) | Recover act done | Acked before kill | Lost | Post-recovery acks | Head samples (seq) |
| --- | --- | --- | --- | --- | --- | --- | --- |
| A | files put | 16:37:07.207-.216 | 16:37:56.27 (first try 16:37:47 used the wrong flags) | 312 (+1 at +4 ms, clock skew) | 313 (all, zero-length files) | 753, all intact | 12777 -> 14931, monotone |
| B | precommit add | 16:43:28.633-.641 | 16:43:50.00 | 259 | 0 | 282 | 14932 -> 15475, monotone |
| C | precommit add | 16:45:45.403-.412 | 16:46:04.25 | 89 | 0 | 0 (see D2) | 15475 -> 15568, monotone |
| D | precommit add | 16:48:41.434-.441 | 16:49:00.30 | 381 | 0 | 0 (all four writers hung in flight, see D2) | 15569 -> 15956, monotone |

Results for B, C, D: 729 acknowledged claims before the kill, all present with identical text.
Run B's list (542 claims) held exactly the 541 acks plus one earlier probe claim, so unacknowledged
in-flight requests that failed did not land. Run C had two claims recorded at 16:45:45.334 and
.341 whose acknowledgements never reached the client (durable but unacknowledged: allowed by the rule).
In A-D, the last 300 pre-kill receipt rows (sequence, kind, digest) were byte-identical afterwards
(`receipts-pre-*` vs `receipts-post-*`: A seq 12478-12777, B 14633-14932, C 15176-15475, D identical).
The recovered guest logged `recovery resume ... (local=13413 H=13413 tail=0)` (guest and host head equal) and
`selfdev: crash boot: rollback-4dca8039-... (degraded) interrupted; settles from the updater journal, never re-applied`.
Verdict: F2 and F3 hold in all four runs; F1 holds for the SQLite-backed write in three runs at
three kill points (4 s, 9 s and 25 s after the writers started) and fails for the filesystem write.

### D1 (defect): `/api/files` PUT is acknowledged before it is durable

Run A. After recovery, `drills/A/w*-N.txt` for every write acknowledged in the 16 s before the kill
exists (the directory entry survived) but is zero bytes (sha256 `e3b0c442...` of empty input, 313 of
313; `verify-A.log`). Writes after the recovery read back intact (753 of 753). Cause (code, not trace):
`internal/autoputer/files.go:436` is `os.WriteFile(absPath, body, mode)` with no fsync; the guest ext4
delays allocation (up to ~30 s dirty expiry), so a metadata-only file survives a kill. Worse for the
tape: each put had already appended its file-change events to the host tape (head +2 per put), so the tape
records files whose content is gone. This violates "no acknowledged write lost" for the files surface.
Hypothesis for the owner's call: files are user filesystem, not the store, and the gate may not cover
them; if it does, the handler needs write + fsync file + fsync dir before the ack (and the tape event after).

### D2 (observation): requests in flight at the kill, or sent while the guest boots, hang to the client timeout

Two patterns, both ending at the CLI's 75 s timeout with no response. (a) In flight at the SIGKILL: run D, all
four writers' requests started 16:48:41.200-.218 and the kill was at .434-.441; all four ended
`context deadline exceeded` at 16:49:56.24-.26 (`D-w1-95`, `D-w2-97`, `D-w3-98`, `D-w4-95`), so run D has no
post-recovery acks; run C `C-w2-23` (started 16:45:45.240, ended 16:47:00.258) is the same, and its claim did land
(recorded 16:45:45.334) without an acknowledgement. (b) Sent within about a second of the guest becoming routable:
`B-w1-82`/`B-w2-82` 16:43:49.361 -> 16:45:04.401, `C-w1-35`/`C-w3-51` 16:46:03.296 -> 16:47:18.343,
`A-w4-135` 16:37:55.667 -> 16:39:10.690, although vmctl logged the VM booted at 16:46:04 (C). Other requests in
the same seconds got a prompt 502 `upstream unavailable` or 503 `computer_waking_failed`. None of the hung requests
that I could check landed except C-w2-23, so there is no durability harm, but a 75 s stall for an owner who retries
right after a crash or a recover is a defect candidate. No proxy-side trace was captured; the cause is a hypothesis
only (the proxy's upstream connection to the killed or booting guest is not reset or timed out).

### Other observations

- A crash leaves the computer `degraded` and requests do not wake it; an owner act is required. The deploy
  retry sweep ("deploy refresh caller=retry-sweep ... outcome=skipped") ran at 16:45:53 and my recover raced it
  (vmctl logged `destructive kill ... cause=refresh epoch=13337` of a boot the sweep had started, then rebooted as 13338); harmless here.
- Each recover/restart/wake appends one `key_revoked` event (epoch 38 at the start, 39 after A, 40 B, 41 C, 42 D, 43 restart, 44 wake).
- `choir precommit list -limit N` clamps N to 1000 and still reports `truncated: false` (`internal/calibration/handler.go:33`). Verification of D stayed sound because the newest 1000 held all 381 acks, but a caller cannot see the cut.
- The held rollback `rollback-4dca8039-...` (was "rollback_pending held" at 13:36) is now `degraded` with the "never re-applied" error after the crash boots (`self-dev list`); it no longer blocks hibernate (drill 3).

## Drill 2: stale-epoch append refused

What fences an append today: `SignedCapabilityVerifier` in `internal/platform/event_capability.go`
refuses a capability whose `RevocationEpoch` differs from the head's `credential_revocation_epoch`
(error "computer capability: revoked", HTTP 403 `computer event capability refused`). A boot,
recover, restart or wake appends `key_revoked`, which bumps the epoch, so every capability the
previous realization held is stale. The design's C3 "writer epochs carried in the capability and
checked inside the append" is not built (grep finds only the unrelated lease `writer_epoch` in
`internal/store/stream_seq_lease.go`). So the probe tests the existing fence, which is checked at
authentication, before the CAS; it does not test an in-append check.

Failure defined beforehand: a request signed under the old epoch is accepted, or changes the head.

Probe (no new product tooling; a 40-line packet sniffer and a 15-line replayer on Node B, deleted afterward):
the guest posts its appends to the platform over plain HTTP on the tap network
(`http://10.201.X.1:8086/internal/computers/events/{head,pin,append,payload}`). At 16:50 I captured the raw
requests of d@e's guest (AF_PACKET on its /30 subnet, kept in a 0700 directory, never printed): three
`POST /internal/computers/events/append` (2696, 2696, 2695 body bytes) and the `pin` calls, all under
capability epoch 42.

1. Replay at epoch 42 (16:51:25): `POST .../append` of req-017 and req-005 each returned 201 with the original receipt; head unchanged at 15959 / `7d5c49c411d5` (idempotent replay, not a new event).
2. `choir computer restart` (idempotency key drill2-restart-1) at 16:51:32; completed 16:51:41; `key_revoked` seq 15960; epoch 42 -> 43; head 15962.
3. Replay of the same captured requests at 16:51:48: `append` req-017 and req-005 -> 403 `{"error":"computer event capability refused"}`; `pin` req-014 -> 403. Head unchanged afterwards: 15962 / `1888a5318335`, epoch 43.

Pass for the capability fence: the same bytes were accepted at epoch 42 and refused at 43, with no head change.
Limits: the response does not name the reason (403 only), and platform logs did not record it, so the cause
"revoked" is from the code and the epoch change, not from a log line; nothing here exercises a race between
auth and CAS. Unit/integration coverage run locally (`go test -count=1` after `eval "$(direnv export bash)"`):
`./internal/platform -run 'Capab|Credential|Revoc|Renew|EventAppend|ComputerEvent|Epoch'` ok (4.3 s, 19 top-level tests including
`TestCapabilityRenewalReturnsCurrentAndPostRevocationPair`, `TestCredentialRenewalRequiresAppendAuthority`,
`TestGuestCapabilityConsumesOnlyTheArmedGrant`) and `./internal/computerevent ./internal/store -run 'Epoch|Revoc|Barrier|Recovers|HeadsAligned|BootWith|Appender|Stale|CAS'` ok
(50 top-level tests, including `TestComputerEventProjectionPersistsRevocationEpochAcrossRestart`,
`TestBootWithALocalHeadAheadOfThePlatformRefuses`, `TestAppenderRecoversCrashAfterCanonicalCAS`).
`credential_resume_test.go:103` asserts a post-revocation capability is refused before `key_revoked` advances the epoch.
There is no Go test named for a "revoked" refusal of a stale capability at the verifier; the runtime probe above is the only direct evidence of it.

## Drill 3: hibernate and wake

Failure defined beforehand: hibernate refused or the VM not down; wake does not serve (`/health` not
`ready`); the head after wake is not the pre-hibernate head (excepting events the wake itself records, to be
named); a write after wake is not durable.

- Pre: head 15962 / `1888a5318335...`, epoch 43, state `active`.
- 16:52:08.75 `POST /internal/vmctl/hibernate` (caller header `drill3`, loopback, internal token): HTTP 200 in 1.46 s. Timeline: fence 2 ms, pause, diff snapshot 1,025 ms, firecracker stopped 1,454 ms. Memory snapshot 8,590,024,343 bytes, 311,595,008 bytes on disk (btrfs extents). `choir computer status` -> `hibernated`; no firecracker process for the VM; host head at the down state 15962 / `1888a5318335...` (identical to pre).
- 16:52:19.69 `POST /internal/vmctl/resolve` (the wake): HTTP 200 in 328 ms. vmctl: `resumed VM ... from snapshot (load=5ms wake_to_healthy=306ms clock_step=11187ms credential=delivered)`. The guest logged `agentcore: resumed from hibernate snapshot; clock stepped 11187ms; credential installed; work admission open`. It was a memory-snapshot resume, not a cold boot, and no process restarted, so no crash close and no planned-restart marker was read or written (the marker path applies to boot, not to snapshot resume). Cold boot, the marker and a planned update were not exercised by this drill (the 16:55:48 deploy refresh of d@e logged `planned_restart=true`, and I did not trace its marker).
- Wake recorded exactly one event: seq 15963 `key_revoked` at 16:52:20.006 (epoch 43 -> 44), the credential redelivery. So "head equals pre-hibernate head" holds at the down state, and after wake the head is that head plus the one named event.
- `/health` at the guest: `status ready, runtime_health ready`. Write after wake: claim `calib:drill:8f57e328992b3e6e` recorded 16:52:34, seq 15964 `projection_batch_recorded`; read back from `precommit list` still present after the later restart (16:55:21) and the deploy refresh (16:55:48), head 15968 epoch 46 at 17:03.
- Before this drill the ledger said hibernate was blocked by the held rollback; after the crash boots marked it interrupted it no longer blocks.

Verdict: pass.

## Drill 4: bounded peak memory

Gate: no operation's memory grows with data size. Failure defined beforehand: the peak of boot, the
derived-key validation or the snapshot producer on the paper (30.7 GB store) exceeds d@e (61 MB store)
by a factor that tracks the data ratio.

Instrument: the guest's host-only `GET /internal/debug/pprof/procmem` (per-process RSS, VmHWM, anon, meminfo),
called from Node B with the internal token; read-only on the paper and the owner's computer. The vmctl balloon
statistics are disabled (`stats_polling_interval_s 0`), the VM cgroup leaves have no memory controller
files, and the firecracker process RSS is the whole guest (8.3 GB HWM on the paper) because the guest page cache fills it,
so these were not usable. The autoputer process VmHWM is process-lifetime, so right after a boot it is the boot peak.

The deploy at 16:55-16:56 UTC refreshed all three computers, so each autoputer is a fresh process (paper 16:55:45, d@e 16:55:58, owner's 16:56:12):

| Computer | Store | autoputer HWM at ~+60-110 s | later | Cached |
| --- | --- | --- | --- | --- |
| d@e | 61 MB (first snapshot 61.7 MB) | 163,132 KiB | flat 163,132 KiB through +4 min | 0.6 GB |
| owner's | ~2.1 GB snapshot | 352,160 KiB | 655,312 KiB by +3.5 min (anon jumped 82 -> 559 MB at 16:58:31-16:59:01, cause not identified) | 0.8 -> 1.9 GB |
| paper | 30.7 GB (`state.db`) | 365,964 KiB | 477,356 KiB at +4 min; 509,588 at 17:00:21; 1,288,104 KiB at 17:01:42 | 2.9 -> 7.3 GB |

The cold boot of d@e from the 16:55:21 restart: HWM 154,552 KiB at +4 s, 162,388 KiB at +25 s, stable.
Earlier lifetime peaks (age 25-60 min): d@e 162,184 KiB, owner's 770,688 KiB, paper 1,967,312 KiB.

Reading: the boot-time peak is 163 MB (61 MB store), 352 MB (2 GB store), 366 MB (30.7 GB store); the data ratio
is about 500x and the boot peak ratio about 2.2x. The paper's spikes after boot coincide with agent work in its log
(texture turns and story verdicts at 17:01:53-59; the derived-key pass was running at 68 rows/s, 24,800 of 160,489 rows, its own anon memory
not separable), so the lifetime numbers measure the workload, not an operation. No controlled run exists: I could not
grow d@e's store to the paper's size (the precommit write caps a claim at 4,000 characters; Texture writes start model runs), and the
migration (done offline, earlier) and the snapshot producer were not available to measure:

- Derived-key validation: runs at every boot; memory not separable from the process.
- Snapshot producer: d@e's one snapshot (16:35:27, 61.7 MB, 1.836 s) ran in a process that I killed in run A, so no HWM; its interval is 24 h, so it cannot be re-triggered on d@e without a reset. The paper's deployed producer refused (16:42:17 and 16:52:17: store 31,222,710,272 bytes over the 8,589,934,592-byte cap), so nothing was measured on the 30 GB store; its fresh boot logged no snapshot line in the first 6 minutes.
- Migration memory: no harness on the real system (ledger already says so).

Verdict: blocked. Evidence is consistent with sublinear growth at boot (2.2x for 500x data), which is not the same as bounded;
the gate is not shown to hold or fail. The decisive experiment is a controlled one: the same boot and the validator on a copy of the paper's
store (or d@e reset and grown to 30 GB) with VmHWM read at the boundaries of each phase, which needs a store-size knob that does not exist.

## First snapshot on d@e

`console.log` line 1887: `2026/10/30 16:35:27 runtime: snapshot taken: first snapshot at head 12772: 61730816 bytes in 1.836s, event 01a18daa-8e12-7cfb-8eab-2008...`
(boot 16:32:55, the 3-minute initial delay plus the first tick). No later snapshot line: after it, `Newest` finds
the snapshot and the 24 h interval is not due.

## Not done

- Stale-epoch append against an in-append writer-epoch check (not built); a race between auth and CAS.
- A kill during a planned-restart marker window, and a kill during the derived-key pass at boot (the pass was short on d@e).
- Controlled bounded-memory runs for migration, derived-key validation and the snapshot producer.
- The differential, the keep-set and the paper-publishes/mail drills (outside this task).
