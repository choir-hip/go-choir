# O7 check — boot cost against stored history (2026-10-09)

Gate 1 lists "O7/O12 checks". O7 (operational invariants register): boot,
recovery and per-write paths cost in proportion to pending work, never
total history. This is the first fleet-wide measurement. Counts and timings
only; no computer contents were read.

## Method

`GET /internal/vmctl/boot-timeline?computer_id=…` on Node B (host-internal)
for every computer vmctl lists (162). Ten have a boot timeline since the
host's last restart. For each: boot kind, the data image's allocated size
(a proxy for stored history), and three host marks:

- `first_healthy`: the guest answers its health probe;
- `replay_first_seen`: the host saw the guest report replay in progress;
- `guest_receipt_fetch_done`: the guest runtime reported "started" (the
  actor runtime, stores and projector are up).

## Result (18:20Z, staging, build 9ef33b77 and earlier boots)

| Computer (prefix) | Boot | Allocated data | Healthy | Runtime started |
|---|---|---|---|---|
| 03335285 | cold | 5,213 MB | 9.2 s | 30.9 s (partial receipt) |
| 7cc96dc6 | refresh | 3,103 MB | 7.4 s | 23.6 s |
| 7e6a9afd | refresh | 820 MB | 7.7 s | 9.6 s |
| dbc7adf0 | refresh | 706 MB | 7.2 s | 8.2 s |
| c0bd7f75 | cold | 406 MB | 9.9 s | 10.9 s |
| a09df437, bb7eee83, 9e4829e4, 635dc886 | cold | ~405 MB | 10.3–10.9 s | 11.5–12.9 s |
| f177676f | recover | 405 MB | — | — (recovery receipt, no health marks) |

## Reading

- **Health is history-independent.** 7–11 s at every size; cold boots
  are ~2–3 s slower than refreshes (cold has a fresh disk to attach).
- **Runtime start is not.** It grows roughly 4–5 s per GB of stored data
  above ~0.8 GB: 8–13 s small, 24 s at 3.1 GB, 31 s at 5.2 GB. The only
  computer whose replay the host saw (03335285) replayed for 0.5 s, so
  event replay is not the cost; the rest is store open and boot reconcilers
  that still read in proportion to history.
- Earlier the same day the owner-scale boot was 9–11 min, then 4.7 min,
  then 31 s (`problems/texture-list-cold-latency-2026-10-09.md`,
  `problems/texture-zombie-activations-revising-forever-2026-10-09.md`).
  The class is mostly fixed; a linear residue remains.

## Verdict and budget

O7 holds for health and is **partially violated** for runtime start: a
linear term in stored data, now seconds rather than minutes. Proposed
budget: runtime start under 60 s for any computer up to 10 GB, and health
under 15 s regardless of size. At 4–5 s/GB the 60 s budget is crossed near
12 GB, so this is a residual, not an emergency.

Residual `o7-runtime-start-scales-with-store`. Located from the guest's
own marks for 7cc96dc6 (3.1 GB, 10:58Z boot): store open 0.25 s, runtime
built 1.02 s, replay 0 rows (done at 1.03 s), lifecycle reconciled
1.06 s, then **runtime started at 16.96 s**. The whole linear term is the
~16 s inside `Runtime.Start` after lifecycle reconcile (the boot phases:
passivation, terminal-outcome reconcile, wake-outbox migration and the
other boot reconcilers). Next: per-phase timings for that span (the boot
log already prints phase durations) on the largest computer, then make
the slow phase pending-indexed. The enforcer still
missing is an alarm: the boot timeline already carries the numbers, so a
host-side check that flags a runtime start over budget would make O7
enforceable without a synthetic owner-scale store.
