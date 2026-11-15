# Event appender wedges: clustering assessment (2026-11-12)

Class: green (record only). Problem record: [paper-stops-after-devin-stream-timeouts-2026-11-12](paper-stops-after-devin-stream-timeouts-2026-11-12.md). Calibration claim C56. Written by a read-only Sonnet investigator from the host control database (metadata columns only), the corpusd, vmctl and proxy journals, the paper's observability dump and the code; tags [O] observed, [I] inferred.

## 1. Verdict on C56: SUPPORTED by elimination, not directly confirmed

C56 says: corpusd committed 10914342, the guest's CAS call returned an error, `appendLockedInner` discarded the prepared row, and `RecoverPrepared` had nothing to finalize. Every observable fits. One link is missing: the guest's error line for the failed CAS. It rotated out of the guest console before capture. No fact found contradicts C56. I would put it at about 0.85 and would not settle it "true" until the error text is seen in a reproduction.

### Evidence for

1. The host committed 10914342 at 15:26:29.866Z. [O: host control DB, read-only SELECT on `computer_event_append_receipts`, metadata columns only]

```
10914340 projection_batch_recorded 15:26:29.491828
10914341 projection_batch_recorded 15:26:29.690268
10914342 projection_batch_recorded 15:26:29.866288
10914343 key_revoked               19:18:33.049335   (the refresh at 19:18:54Z)
```

   Nothing was committed between 15:26:29.866 and 19:18:33, which matches "all work stopped".
2. 10914342 sits in a normal burst: 10914335 to 10914342, one every 165 to 200 ms, from 15:26:28.6. [O: same query] It is the same writer stream as 10914341, not a separate host-side actor.
3. corpusd confirms the host head was 10914342 at 15:29:42Z: `receipt compaction computer-1355b83b... mode=active through=10914342 scanned=1963` [O: `journalctl -u go-choir-corpusd`]. The host head had not moved since.
4. The guest's local head stayed at 10914341: `in-place prepared recovery (heads differ): err=<nil> embedded_seq=10914341` and `wedge continues ... local_seq=10914341 platform_seq=10914342` [O: the paper's observability dump, recent_log, 17:41Z to 19:16Z]. The consecutive counter was 1485 at 17:42Z and 1616 at 19:16Z, and the wedge "since" stayed `2026-11-12T15:26:30Z`.
5. `err=<nil>` with the heads still apart means `RecoverPrepared` listed zero prepared rows. [I, from `RecoverPrepared`, appender.go:915-968] If a prepared row for 10914342 had survived, it would have taken the first branch (`platformHead.Sequence == request.Event.Sequence && CanonicalEventHead == request.EventDigest`). That branch re-sends the CAS, which the host answers with the stored receipt, then verifies and finalizes. It either advances the local head or returns an error. It cannot return nil and leave the heads apart. So the row was gone, and the only code that deletes it on this path is the discard after a failed CAS (appender.go:855-863), which has no check of whether the host committed.
6. The error came back within about a second of the commit. [I] "wedge began" is stamped 15:26:30Z, and the commit was at 15:26:29.866 with appends queued behind a 165 ms cadence. The guest client's timeout is 10 minutes (`hostlink.NewClient(10 * time.Minute)`, http_client.go, end of file). So this was not a 10-minute black hole. It was a fast failure: a cancelled caller ctx, a connection reset, or a 5xx or other error status after the commit.

### Paths that could have advanced the host head without the guest's CAS (the falsifier you asked for)

- `HandleComputerEventAppend` is the only HTTP route to `ComputerEventCAS.CompareAndSwap`. [O: grep, internal/platform/event_handlers.go:255; no other caller outside tests]
- The SQL that writes `computer_event_heads` is in `compareAndSwapTx` (platform/computer_events.go:287,290) and in bundle import (`internal/bundle/import.go:523,529`, the follower/import path). [O: grep] Bundle import is not used for the paper's own head. Row cadence and `event_kind` in point 2 argue against an import. [I]
- Restore and replay never write the host head: they read `Events` and write the guest projection only. [O: `reconstruct`, appender.go:986]
- A single appender exists per guest. `NewComputerEventAppender` has one non-test caller (internal/autoputer/run.go:482), and `RebindProjection` only swaps the store under `a.mu`. [O: grep] So a second local writer is not the cause.

### Alternatives checked and rejected

- Post-CAS verify failure or finalize failure. Both leave the prepared row, and `RecoverPrepared` would then either converge or return an error. It returned `err=<nil>`, so this is rejected. [I]
- Orphan prepared row (the 10-28 and 11-02 11:42Z mechanism). That produces a Prepare unique-key failure with equal heads. Here the heads differ and Prepare is never reached. Rejected.
- Guest crash: `since` and the counter run continuously in one process, with no boot. [O: recent_log]

### What is still unknown

- Which error the CAS returned. Candidates: caller ctx cancelled (the append's `ctx` is the caller's, so a turn or poll deadline cancels it); a TCP reset while the guest was under load (vmctl health checks of the guest failed at 15:25:32, 15:26:02, 15:26:26, 15:27:17, and the proxy's reads of it timed out at 15:25:50 and 15:26:26 [O: vmctl and proxy journals]); a host 503 (`dbx.IsRetryable`, event_handlers.go) or a host-side ctx cancellation (the handler passes `r.Context()` into the serializable tx) that surfaced after the commit. I could not identify the calling code path for the 8 batches 10914335 to 10914342. The newsie poll marked in flight at 15:26:07Z started about 22 s earlier [O: problem record]. That is suggestive only.
- corpusd does not log appends or client disconnects, so the host side cannot show a dropped reply. [O: the 15:20 to 15:32 corpusd journal has 43 lines, none about appends]
- The `livePayloads` entry of the failed append (plaintext kept after failure by design, appender.go:515-520) is a small leak while wedged. [I]

## 2. The earlier wedges

| Date | Record | Signature | Same mechanism as 11-12? |
|---|---|---|---|
| 2026-09-29 | docs/archive/definitions/choir-jev-supervision-metamission-2026-09-29.md (line ~255) | "an OOM-kill mid-append committed the platform CAS but killed the embedded finalize"; boot replay fixed it (`local=174099 H=174100 tail=1`) | No. Process death after CAS. A prepared row survives and boot recovers it. Same family (host one ahead), different step. |
| 2026-10-28 | docs/problems/paper-wedged-on-orphan-prepared-event-2026-10-28.md | `prepare: Error 1062 duplicate unique key` at head+1, equal heads, about 880 times; fix: restart | No. The CAS failed, the discard failed (or shared an expired ctx), and the orphan blocked Prepare. Heads were equal. |
| 2026-11-02 06:20Z | docs/problems/paper-wedged-on-post-cas-projection-failure-2026-11-02.md ("What happened") | Platform head 5819266 (corpusd compaction `through=5819266` at 06:20:06Z), embedded 5819265; every append `projection repair required`; boot replay applied exactly 1 event | Probably yes. It is identical in shape to 11-12: host one ahead, local one behind, no readable cause. The record attributes it to a post-CAS finalize failure, but "the original error text is lost" and the only trace is the head split. A finalize failure leaves a prepared row. At boot, `RecoverPrepared` in replay mode discards prepared rows and the replay re-applies them, so "replay applied 1 events" fits both a surviving prepared row and a missing one. The record's own evidence does not tell the two apart. [I] |
| 2026-11-02 11:42Z | same record, "Second wedge" | `UNIQUE constraint failed` at 5892982, equal heads, 819 times | No. Orphan prepared row, as 10-28. The in-place recovery (17afb82f2) and the `WithoutCancel` discard fix this one. |
| 2026-11-12 15:26:30Z | docs/problems/paper-stops-after-devin-stream-timeouts-2026-11-12.md | host one ahead, local one behind, `err=<nil>` recovery | This one. |

Summary: the 11-02 "post-CAS finalize" fix (`recoverPreparedInPlaceLocked`) repairs only a surviving prepared row. The 11-02 06:20Z and 11-12 wedges are both "host ahead, nothing local to resend" and are not repaired by anything running in the guest. Both lasted until a boot. [I]

## 3. Clustering assessment: where local and host heads can diverge and never reconverge while running

The appender is `appendLockedInner` (appender.go:765). Recovery means `recoverPreparedInPlaceLocked` -> `RecoverPrepared`. Steps in order:

| # | Step | Failure | Local vs host after | Does recovery handle it while running? | Evidence |
|---|---|---|---|---|---|
| 1 | read both heads | error | no change | n/a | code |
| 2 | heads differ, in-place recovery | no prepared row and local behind | local N-1, host N | **No.** `RecoverPrepared` never fetches an event. It only sees prepared rows. This is the 11-12 wedge. | [O] 11-12 log; code |
| 2b | heads differ, local ahead | host rolled back, imported, or restored | local N, host N-1 | No. By design (rewind fence); boot also refuses. | [O] test `TestBootWithALocalHeadAheadOfThePlatformRefuses`; no known occurrence |
| 3 | pin event (HTTP) | error | none; orphan pin is content-addressed | n/a | code |
| 4 | `Prepare` | unique key (orphan row), disk error | prepared row may exist | Yes for an orphan at head+1: recover and retry once. A disk error stays a wedge kind `prepare failed`. | [O] tests `TestAppenderClearsOrphanPreparedRowOnPrepareConflict`; 10-28 and 11-02 records |
| 5 | CAS, error returned | **commit unknown**: ctx cancel, reset, timeout, 503, decode error | commit may have happened | **No** when the host committed: the row is discarded at once (appender.go:855-863). | 11-12 evidence above |
| 5a | CAS, definite refusal (400, 403, 409, "idempotency commitment changed") | not committed | local = host | Yes (discard is right). | code |
| 5b | discard fails or is cancelled | prepared row stays | prepared row at N | Yes: head+1 and matching previous head -> discard; head == digest -> re-send and finalize. | code; 11-02 fix uses `WithoutCancel` + 30 s |
| 6 | verify head receipt | bad signature, unknown key | prepared row, host N | Retries the same verify; if it fails again, `RecoverPrepared` returns the error and `recoveryFailedAt` sets a 30 s cooldown. Not self-healing. | code |
| 7 | `finalizeProjection` | transient (SQLite lock, ctx cancel) | prepared row, host N | Yes: re-send CAS (host returns the stored receipt), finalize. | [O] `TestAppenderRepairsPostCASFinalizeFailureOnNextAppend` |
| 7b | finalize, poison batch | deterministic failure | prepared row, host N | No. Wedge kind "finalize: projection poison"; cooldown. | [O] `TestAppenderPoisonFinalizeDoesNotSilentlyRetryTheSameCrash` |
| 7c | finalize, unreadable sealed batch | applies as "no change" | local = host, but the caller gets an error and may retry | Converged; the caller sees a spurious failure. | code (appender.go:872-874); `unreadable_batch_test.go` |
| 8 | crash between steps | process dies | any of the above | Boot: replay mode discards prepared rows and re-applies the tape; non-replay mode finalizes or discards. Handled. | code; 09-29 record |
| 9 | recovery blocked by `reconstructing` or `replayActive` | returns `ErrNeedsProjectionRepair` | unchanged | Transient only. | [O] `TestAppenderInPlaceRecoveryRefusesDuringReconstruct` |
| 10 | `RebindProjection` (rematerialize) | new store's head may differ | unknown | Not checked in this audit beyond reading the call (agentcore/rematerialize.go:265). | code |
| 11 | store paths that read `HeadsAligned` (stream_seq lease) | refuse when heads differ | n/a | Fail closed; they recover as soon as the heads match. | code |

Count: the wedges on the paper in the last 15 days were three of four steps in this table (4, 5, 7). The contract's clustering threshold (three in a week) was met on 11-02. Two of the five paper wedges (11-02 06:20Z, 11-12) fall at step 2/5 and have no running-guest repair at all. The structural cause is one design choice: **the guest's local ledger of in-flight appends (the prepared row) is the only record that a commit might have happened, and the CAS error path deletes it before asking the host.** The host is authoritative and can answer in one `Head` read or an idempotent re-send.

## 4. Fault-injection failure list

Harness needed first (none exists today): `memoryCAS` (computerevent_test.go:751) gets hooks to (a) commit then return an error, (b) return an error without committing, (c) commit after a delay (a late commit), (d) fail `Head`. `memoryProjection` (:700) gets hooks to fail Prepare, Discard, and Finalize with a count. A "boot" helper builds a new appender over the same two memory stores. Expected states are written as `local / host / prepared rows` after ONE recovery pass (the next append, or a boot) unless said otherwise. N is the sequence being appended.

| Case | Inject X at step Y | Today | Expected (target) |
|---|---|---|---|
| F1 | CAS commits at the host, returns a transport error (reset) at step 5 | local N-1 / host N / 0 (wedged; recovery does nothing) **FAILS** | local N / host N / 0; the caller gets the receipt or an error whose state already converged |
| F1b | same, but the error is `context.Canceled` (caller ctx cancelled after commit) | same as F1 **FAILS** | local N / host N / 0 |
| F1c | same, but the host returns 503 after commit | same as F1 **FAILS** | local N / host N / 0 |
| F2 | CAS errors before commit, definite (400/403/409) | local N-1 / host N-1 / 0 | same |
| F2b | CAS errors before commit, unclear (reset before the host saw it) | local N-1 / host N-1 / 0 | same; the row is discarded or kept for recovery, never leaving a stuck row |
| F3 | CAS error unclear; host commit lands AFTER the guest's probe (late commit) | **FAILS** (as F1) | prepared row retained, not discarded on a probe that says "not committed"; after the commit lands, local N / host N / 0 by the next pass |
| F4 | CAS error not committed; discard also fails | local N-1 / host N-1 / 1 | after the pass: local N-1 / host N-1 / 0 |
| F5 | F1 plus the discard fails | local N-1 / host N / 1 | after the pass: local N / host N / 0 (works today; it is the only F1 variant that does) |
| F6 | `PinEvent` fails | local N-1 / host N-1 / 0 | same; orphan pin is harmless |
| F7 | `Prepare` fails once (unique key, then disk full) | recovered once, else wedge kind `prepare failed` | orphan: converge; disk: refuse with a named kind, no state change |
| F8 | crash after Prepare, before CAS | boot: row discarded | local N-1 / host N-1 / 0 |
| F9 | crash after CAS commit, before verify | boot (replay mode): discard + replay applies N | local N / host N / 0 |
| F9b | same, boot in non-replay mode | `RecoverPrepared` re-sends, finalizes | local N / host N / 0 |
| F10 | verify fails after commit (key unknown) | wedge, cooldown | no discard, no advance, a distinct loud wedge kind; local N-1 / host N / 1 |
| F11 | finalize fails once after verify | exists, passes | local N / host N / 0 |
| F12 | finalize fails always (poison) | exists, passes | wedged "finalize: projection poison", cooldown, row kept |
| F13 | finalize cancelled mid-transaction | like F11 | local N / host N / 0 |
| F14 | two appends race while the heads differ | serialized by `a.mu` | the first converges, the second appends on N+1; exactly one recovery |
| F14b | recovery vs `Reconstruct` | refused (exists) | unchanged |
| F14c | recovery vs `HoldAppends` (checkpoint restore) | `HoldAppends` takes `a.mu`; no overlap | add a test that catch-up cannot run inside it |
| F15 | host ahead by k = 1, 3, `EventReplayPageSize+1` events, no prepared row (a pre-existing wedge) | local N-k / host N / 0 wedged **FAILS** | local N / host N / 0 after one pass; for k above one page, the pass may need several pages but must end aligned |
| F16 | catch-up where the payload fetch fails halfway | n/a | partial progress kept (local = last applied), wedge stays, no corruption, the next pass resumes |
| F17 | catch-up where event N-k+1's `PreviousHead` != local canonical head (divergence) | n/a | refuse; no state change; wedge kind "divergent chain"; never auto-heal a fork |
| F18 | local head ahead of host | wedged (exists as boot test) | refuse in place; no catch-up; no change |
| F19 | a live append arrives while catch-up runs | n/a | it waits on `a.mu`; afterwards binds to the new head |
| F20 | corpusd restarts mid-append (refused dial) | handled by hostlink resend (exists) | unchanged |
| F21 | the same `CASRequest` re-sent twice (host side) | no host test found by grep | host returns the stored receipt and does not add a row; two re-sends with a changed commitment get a 409 |
| F22 | `Head` probe fails after an unclear CAS | n/a | row kept; error returned; no discard |
| F23 | wedged for 1,600 appends | `livePayloads` keeps a plaintext per failed append (appender.go:515-520) [I] | bounded memory |

## 5. Sketch of the two fixes (no code)

### (a) Classify an unclear CAS by the host, not by assumption

Change: `appendLockedInner` (appender.go:855-863) and `HTTPClient.CompareAndSwap` (http_client.go:138).

1. Make the client return a typed error that separates definite statuses (400, 403, 409, 422) from unclear ones (transport error, ctx error, 5xx, decode error). Today only 409 is typed (`ErrCASConflict`); everything else is a string.
2. On a definite refusal: discard as now.
3. On an unclear error: do not discard. First re-send the SAME request on a fresh `context.WithoutCancel` plus a bounded timeout. The host checks `existingComputerEventReceipt` before the head check and again inside the transaction (computer_events.go:~225 and ~245), under a per-computer lock. A re-send therefore returns the original receipt if the first attempt committed, or commits it now. A conflict reply means it did not commit. That removes the guess and the late-commit race (F3). If the re-send is also unclear, keep the prepared row and return the error. The next append's `RecoverPrepared` already re-sends when the host head equals the row's digest (the committed case) and discards when the host is at `previous_head` (the uncommitted case).
4. Alternative with less behaviour change: probe `cas.Head` instead of re-sending. It is cheaper but has the late-commit race, so I prefer the re-send.
5. When the re-send succeeds, continue with verify and finalize and return the receipt (the caller's append succeeded).

What existing tests cover: finalize crash recovery (`...CrashAfterCanonicalCAS`, `...PostCASFinalize...`), orphan prepared row, reconstruct block, poison. Not covered: any commit-then-error CAS (F1 to F3, F22). `memoryCAS` already stores receipts by idempotency key (:759), so a re-send test works with a small hook.

Risks:
- The re-send runs while `a.mu` is held, so all appends wait up to the bound. This is a deadline, not a limit on calls, and it replaces a permanent wedge.
- Keeping a prepared row on an unclear error makes the next append's `Prepare` rely on the orphan-row path (head+1 conflict). That path exists and is tested.
- A boot-time re-send of a stale prepared row is NOT proposed. `RecoverPrepared` keeps discarding rows at `previous_head`, because the crash-restart rule in CLAUDE.md says a crash never resumes work.
- Red surface (tape, append path): two review seats.

### (b) In-place catch-up replay from the host while running

Change: add a catch-up step to `recoverPreparedInPlaceLocked` (appender.go:883), after `RecoverPrepared`, when the heads still differ and local is behind (`headSeq(local) < headSeq(platform)`).

1. Hoist the `apply` closure in `reconstruct` (appender.go:1042-1088) into a method that does Reduce, `sameHead` against `record.Request.Next`, `VerifyEventHeadReceipt`, `Prepare`, `finalizeProjection`. Both reconstruct and the in-place step call it.
2. In place: hold `a.mu` (the caller already does), take pages from `a.cas` if it implements `PagedEventSource` (the production `HTTPClient` does, http_client.go `EventsPage`), starting after the local head. Stop when aligned. Stop and keep the wedge on any mismatch.
3. Do NOT set `a.reconstructing` (it would make the same function refuse itself, see appender.go:884-886). Instead guard against being entered from `reconstruct`: reconstruct already has the flag set when it runs, so the existing early return covers it.
4. Payloads: `resolveProjectionBatch` reads `livePayloads` first and otherwise fetches by digest from the host (pinned before the CAS), so the missing event's payload is available. The live finalize path (`FinalizeBatch`, not `FinalizeReplayBatch`) is strict. The store's legacy-row compatibility applies only to replay mode. A recent event should pass; if not, that is a poison, so keep the wedge and report it.

Tests to extend: `TestAppenderInPlaceRecoveryRefusesDuringReconstruct`, the 11-02 tests, `heads_aligned_test.go` (local ahead must remain refused), and the `append_barrier_test.go` entry-point test. New: F15 to F19.

Risks:
- Two writers. The in-place step is the second writer of the local projection beside the append path, and the boot replay is the third. All three must exclude each other: `a.mu` covers append and in-place, `reconstructing` covers boot. The store's `eventMu` serializes finalize. Any local write that bypasses the tape (direct SQL) would break the assumption; the repo's own comments say unbound stores are test-only.
- Replay vs live: a gap of many events holds `a.mu` for the whole catch-up (every writer waits). Bound it by page count per attempt with the `recoveryFailedAt` cooldown, and resume on the next append.
- Divergence: an event whose `PreviousHead` does not match must refuse, never fix up (F17). The host is authoritative only for a linear extension.
- The post-commit observer (M7, derived continuation) is fired only by live appends. Caught-up events would skip it. Decide whether to fire it for them; the sweep that wedged on 11-12 re-arms on its own schedule, but other derived work could be missed. [I]
- Caller side effects: the caller that saw the error may have retried with a new event ID after catch-up. Projection upserts are mostly idempotent; I did not audit every op kind.
- Health and alert are separate: the wedge reporting (7a9d87889) exists, and the liveness alert goes nowhere. Neither is addressed by (a) or (b).

### Order and classes

Fix (a) removes the cause of this wedge. Fix (b) removes the whole class for a wedge that (a) misses, and brings the 11-02 06:20Z case in. Both are red (tape, append path), so each needs its conjecture delta, rollback, and two review seats. Write the failure list in section 4 as tests first, as the contract requires; F1, F3, F15 and F17 fail on today's code.

## Method notes

- Host DB access: `ssh node-b`, then `sudo -u postgres psql -d control` (peer auth, no secrets read). Transactions were `BEGIN READ ONLY`. Only `sequence`, `event_kind`, `created_at`, and counts were selected. No digest, receipt, hash, key, or payload column was read.
- Guest log source: the paper's observability dump (captured 19:14Z). I read only the `recent_log` lines about the appender. The first "wedge began" line and any CAS error line before it are not in the 512-line window.
- I did not run any Go tests.
