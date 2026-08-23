# Recover-current replay checkpoint bottleneck

Date: 2026-08-23
Mutation class: red

## Problem

A fresh `recover_current` data image reaches the guest with the privacy key but no local event projection. Guest startup replays the canonical computer event chain through `ComputerEventAppender`. `Store.FinalizeReplayBatch` currently calls `CALL DOLT_COMMIT('-Am', ...)` after every replayed event. The per-event Dolt checkpoint dominates recovery time and causes the guest readiness budget to expire before port 8085 opens.

## Evidence

On staging computer `computer-03335285269bdba4f94377e56879f9e6`, canonical head sequence was `132436`. With `EventReplayPageSize=1024`, the fresh guest still failed during replay at sequence `1699` after the 10-minute bootstrap context expired:

```text
autoputer: reconstruct computer event authority: computer event appender: replay prepare sequence 1699: computer event projection: prepare: Error 1105: context deadline exceeded
```

The same recovery attempt transmitted approximately 566 MB over the guest tap and remained CPU-bound for more than 15 minutes; `/health` never opened. The VM was then killed by `VM_BOOT_READY_TIMEOUT`. The failure is not the HTTP page size after the 1024-page deployment; it is the per-event local Dolt checkpoint/write path.

## Consequence

Increasing the boot timeout only postpones failure and holds the ownership/recovery request for the full duration. Recovery needs one durable Dolt checkpoint after the replay transaction stream, not one checkpoint per replayed event. Semantic event verification and projection ordering remain required; only replay checkpoint frequency changes.

The first checkpoint repair was not active on the guest boot path: `autoputer.Run` called `appender.Reconstruct` without enabling the appender's replay-only finalization mode, so the normal `Finalize` path continued to checkpoint every event. The 30-minute retry later confirmed the same mechanism: an out-of-memory kill followed by `replay finalize sequence 3213: runtime store: dolt checkpoint: Error 1105: context deadline exceeded`.

After boot replay mode was enabled, the guest still hit the unchanged 10-minute `bootstrapCtx` at sequence 2277 (`2026-08-23 11:33:21Z`), while `VM_BOOT_READY_TIMEOUT` was already 30 minutes. The local replay path therefore needs a bootstrap budget aligned with the boot budget.

With the replay-only finalization mode enabled and a 30-minute bootstrap budget, the fresh guest still accumulated roughly 1.8 GiB RSS and the kernel killed `autoputer` after about 15 minutes. The source client materialized the entire 132,436-event chain in one `Events` slice before applying it; the subsequent restart loop surfaced only the generic projection-repair error. Recovery must page and apply records incrementally instead of retaining the complete chain.

## Repair boundary

The repair will suppress per-event `DOLT_COMMIT` calls for the explicit replay-only projection path and issue one final checkpoint after the canonical replay reaches its target. Live append/finalize paths retain their existing checkpoint behavior. Effects remain OFF and canonical corpusd events are not rewound.
