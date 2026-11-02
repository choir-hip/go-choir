# Tier 2 review: the store header/body split, 2026-11-02

Candidate: 857adb9ef..dcbe7b4c4 (the split as landed). Seats: Devin and DeepSeek v4.1 flash, running separately on a frozen worktree.

Round 1. Both seats: send_back on one blocking-real defect. Telemetry partition files were split in place inside store.Open after the boot gate stopped ticking. The host's stall detector (120 s of frozen progress) or the 20 s budget on a retry boot with no gate could wedge a boot with a large partition on every retry. The preflight also omitted the partition rewrite space. Fixed in b80fe36bd (deployed in 81f09e677). Also tested on a reflink copy of d@e: 13/13 differential checks green.

Round 1, non-blocking, deferred as named residuals:
- finalize has no differential interlock;
- dropPresplitIfLow can drop .presplit early under disk pressure;
- syncDir swallows errors;
- every migration error becomes the typed refusal;
- the rollback recipe leaves split debris;
- the swap's -wal check is unreachable;
- a zero-byte state.db is read as new layout;
- the !old + sidecar + missing .presplit path skips postSplitSwap;
- a torn .split-attempts counts as the maximum.

Round 2 (re-review of b80fe36bd, Devin, one seat because round 1 found a blocking-real defect): **approve**, confidence high.
- Adopted, to be built and held until the S6 meter day ends (a guest deploy reboots the paper):
  - filter retired partitions out of the rewrite set;
  - raise the peak allowance toward 4x and pin the VACUUM temp to the store directory;
  - clamp the batch loop against overflow;
  - drop the progress off-by-one;
  - correct the read-only comment;
  - add a BootPhase per partition file;
  - add a sparse-rowid batch test.
- Deferred: detection opens partition files read-write before the gate binds, so a hot WAL left by a killed copy is recovered before the gate exists. Bounded by the 20 s budget, and rare.

Settlement: the review gate for `complete` is met (round 1 blocking fixed, round 2 approve). `complete` still waits on S6.
