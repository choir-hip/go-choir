# The paper is wedged on an orphan prepared event (2026-10-28)

Status: open; documented before the recovery restart. Class of a fix: red
(the computer event appender, Trace/tape). Found by the owner ("autopaper
has been down for 5 hours"), not by any alert.

## Evidence (read-only, 21:0x to 21:2x UTC)

- The paper (`computer-1355b83bced1613eb048ee28d66688a8`,
  `vm-1669020adcece51c2f5ff9b6df399ef5`, booted about 11:48 UTC on
  f7e215e54) answers `/health` `ready` with `running_runs` 0, but every
  write fails. The current console log holds about 880 copies of
  `computer event projection: prepare: Error 1062: duplicate unique key
  given: [computer-1355b83bced1613eb048ee28d66688a8,4642560]`.
- What it blocks: every source poll (dead letters for venturebeat, cisa
  advisories, bsky icij.org…), the source schedule sweep (`sweep failed
  (8 in a row)` by 20:27), and the management desk inbox
  (`after 65 deferrals`, delivery-poison).
- corpusd's last story activity from the paper is 14:56 UTC; publishes
  per hour on corpusd were 36, 25, 31, 25, 21 for 10:00 to 14:00, then
  none.
- vmctl logged the paper unhealthy repeatedly from about 14:03 to 14:57,
  and host pressure (`signals=memory,io`) at 14:49 and 14:53/14:55. No
  deploy, timer or squash ran in that window (last deploy 12:27,
  0bf89ed84). The earliest 1062 cannot be dated: the four kept console
  files start at 15:01.
- The ChatGPT credit exhaustion is later and unrelated.

## Why (code, verified unless marked)

- `ComputerEventAppender.Append` (`internal/computerevent/appender.go`)
  checks that the platform head and the embedded head agree, pins the
  event, then `Prepare` inserts a `prepared` row into
  `computer_event_index` at `sequence = head + 1`, then calls the platform
  CAS. On a CAS error it calls `DiscardPrepared` and ignores its error
  (`_ =`, appender.go:783).
- The appends keep passing the `sameHead` check, so the platform head and
  the embedded head still agree at 4642559: the platform never accepted
  4642560. The row at 4642560 is therefore an orphan `prepared` row whose
  CAS failed and whose discard did not happen.
- Every later append computes the same sequence with a different
  idempotency key, so `Prepare` hits the unique key on
  `(computer_id, sequence)` and fails before the CAS. Nothing in the
  running process clears the row; only `RecoverPrepared` does, and it runs
  at boot/reconstruct. An orphan at `platform head + 1` whose
  `previous_head` is the platform head is discarded there.
- Hypothesis, unverified: the discard reuses the CAS's `ctx`; if the CAS
  failed on a deadline during the 14:4x pressure, the discard fails at
  once on the same expired context. No log from before 15:01 survives to
  confirm.

## Recovery

A restart of the paper so `RecoverPrepared` discards the orphan. Owner
go-ahead 21:2x UTC. The owner also asked that every desk run on Synthetic
DeepSeek while ChatGPT credits are out (about 90 minutes), then a 1:1 pool.

## Fix direction (not started)

- The running appender must not wedge on its own orphan: discard on a
  fresh, bounded context, and treat a 1062 on `Prepare` at head + 1 as
  "recover prepared now" rather than an error forever.
- The paper went five hours without anyone noticing: publish-rate or
  write-failure alerting is missing.
