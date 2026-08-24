# Recovery complete — B14 host drive + guest verification — 2026-08-24

## Outcome

computer-03335285269bdba4f94377e56879f9e6 is **active and quiesced** on
staging at the canonical head: the platform tape head 132,539
(canonical_event_head acc54c39ee05d89af13223e3b8cca195e04d7dfc8f137ce1bb27b96f657b7201,
reducer v1) was reached and verified; no rewind, no second semantic writer,
quarantine preserved, route CAS under the fencing token completed by the
product resolve flow.

Final state (choir computer status): `state: active`, `realization_epoch: 715`.
Guest /health: `{"status":"ready","runtime_health":"ready","running_runs":0,
"build":{"commit":"f7b3ccd2","deployed_at":"2026-08-24T05:40:32Z"},
"self_development_marker":"genesis-baseline"}` — effects mode remains off.

## How (chronology)

1. Phase-1 shipped (63168fb3): resumable replay (periodic durable checkpoints
   500 events/60s; 30m resume quantum), liveness/readiness split (503
   ReplayInProgress gate; 200 only at head+witness), RecoverPrepared replay-
   safe (no CAS), B11 GC deferral, host stall-gated waitForGuestReady,
   deployed guest+host together (B5).
2. A2/A3 experiment (copy of data.img): SIGKILL mid-replay resumes from the
   committed head (60,122 -> kill -> 62,651 monotonic); measured 27-82 ev/s
   on the host; full-chain finish proven. Also discovered the post-replay
   nil-executor SIGSEGV -> fixed (85fa83b4) with regression tests.
3. Real recovery boots #1-#5: guest replay 99.7k, 103.1k, 105.2k stalls
   (silent; per-event apply 3-6s at the 7 GiB workspace on the 4096 MiB guest
   = guest I/O ceiling) + boot #2 OOM crash-loop (startup milestone Dolt GC —
   fixed via RUNTIME_DOLT_GC_DISABLED=1 + 5 GiB size guard, b5cf67fd) + stall
   timeout raised to the B10 ceiling 300s (4adce9e1). Diagnostics instrumented
   (8a969044) proved the per-event apply cost (3.2-6.5s/event, 0.2-0.3 ev/s).
4. Agentic-consensus panel (12 agents, convergent): route A — host-side
   replay drive under a STRUCTURAL replay-only boundary (no runtime start, no
   lifecycle reconcile, no semantic appends — the 103-event append incident in
   experiment run 3 was exactly the full-runtime boundary); dissent: D
   (appender batching) is the strongest durable product fix; B (8-12 GiB
   guest) defensible if the definition must stay letter-faithful. B14 ratified:
   docs/definitions/choir-durable-substrate-recovery-2026-08-23.md.
5. B14 implemented (f7b3ccd2): RUNTIME_RECOVERY_REPLAY_ONLY=1 boundary in
   runReplayPhase -> reconstruct + durable checkpoint + exit; no reconcile, no
   appends.
6. Host drive (05:34-05:39Z): same-commit runtime (f7b3ccd2 package) against
   the RETAINED data.img (rollback reflink data.img.pre-hostdrive-20260824
   first; exclusive loop mount), fresh envelope exchange, replay-only drive:
   `recovery replay-only drive complete (seq=132539 committed=132539)`;
   ZERO slow applies; platform head verified 132,539 BEFORE and AFTER (no
   appends — the replay-only boundary held); clean unmount.
7. BootVM (epoch 715): local==platform -> replay no-op -> head+witness verify
   -> runtime up -> 200 ready -> resolve route CAS -> ownership active,
   running_runs=0.

## Evidence

- Receipts: recovery-replay-resume-experiment-2026-08-24.md (A2/A3),
  recovery-post-replay-cosuper-fate-nil-executor-panic-2026-08-24.md,
  recovery-guest-startup-gc-oom-crash-loop-2026-08-24.md,
  recovery-replay-guest-io-ceiling-assessment-2026-08-24.md (structural
  assessment + consensus record), this receipt.
- Commits: 63168fb3, 85fa83b4, b5cf67fd, 8a969044, 4adce9e1, f7b3ccd2
  (code + docs each landed with CI green; deploys verified by
  /var/lib/go-choir/deploy-receipt.json target_commit + guest build.json).
- Consensus: .agentic-consensus/replay-substrate-20260824/ (manifest 12/12 ok).

## Residual / follow-ups (NOT recovery blockers)

1. Guest-native recovery still hits the 4 GiB/7 GiB I/O ceiling (3-6s/event).
   Durable product fix per panel dissent: appender per-page batching (D) —
   successor work; the guest path works only when the workspace is small or
   the host drive is used.
2. Other active computer candidate-fleet-d03dacaa... has looped ~7h emitting
   "runtime: mint trajectory ... invalid genesis" (store append projection
   batch: invalid computer event transition: invalid genesis) — separate
   finding on the same projection substrate; needs its own problem receipt
   and fix (not touched — the recovery stayed scoped).
3. The rollback reflink (data.img.pre-hostdrive-20260824) is retained as the
   rollback ref; data.img.quarantine-1/-2 preserved untouched.
4. The 103 tape events 132,437-132,539 (2 key_revoked + 101
   projection_batch_recorded) from experiment run 3 remain in the tape —
   authentic CAS-accepted auth-reporting; the recovery consumed them as part
   of the head (the definition's B9 target was revised to 132,539 before the
   recovery ran; the pre-delta head 132,436/8df7efbba was reached and
   verified during A2/A3).
