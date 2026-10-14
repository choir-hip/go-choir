# LR: Provisional Scaling-Cost Read

Date: 2026-10-14, 01:00 UTC. Station:
[LR](../definitions/choir-ledger-lr-resource-2026-10-13.md). This read
uses only data that already exists: the paper computer (14 sources) and
Node B. It is not the scaling-cost report. That report needs the source
sweep and the oversubscription probe, run after the overnight deploys. It
also needs the intake bugs fixed first, because they change the curves
(`docs/problems/autopaper-intake-defects-2026-10-14.md`).

## What scales with sources today

| quantity | paper computer, 14 sources | how it scales now | expected after L2a and S1b |
|---|---|---|---|
| captures | at most 280 an hour (14 × 4 polls × 5 items) | linear in sources, capped by `max_items` (defect 1 drops the rest silently) | linear in sources (lossless), with the per-poll limit visible |
| stories | 50 in about 9.7 hours; 47 of 50 opened from one capture | close to linear in captures: no consolidation at intake | sublinear: events are finite, and more sources add coverage to the same event |
| agents | 1 management, 1 Texture per story, research per story | linear in stories | Texture per story, research per topic (sublinear in stories) |
| model tokens | 64.2 M in and 2.3 M out over 1,335 calls (assessment, 2026-10-13); research 75% of input | follows runs, so linear in stories; input-dominated (28:1) | follows events and topics; lever 6 cuts research's re-sent output |
| RAM per computer | 5.6 GiB host RSS an hour after boot | set by the store and the workers, not by sources | levers below |
| disk per computer | 6.8 GB (6.9 GB at boot) | set by store history bloat (R1), about 5 times live data | store GC and compaction (LR memory slice) |
| CPU | host load 0.5 to 0.9 on 12 cores | not the constraint: model calls are remote | — |

All 251 calls in the cost API's recent window are unpriced
(`opencode-go`: cat-jam-free, longcat-2.5-preview-free,
deepseek-v4.1-flash), so `/api/costs` reports $0. Dollar figures need
prices in the cost record. The free models make tonight's real price
near zero, but the report must state the price at list rates.

## Where the RAM goes (paper computer)

- 5.6 GiB host RSS for the Firecracker process.
- The guest's main process: Sys 1.46 GB, HeapInuse 1.05 GB. 77% of the
  sampled heap is under the event appender, mostly Dolt buffer pools.
- About 4 GiB lies outside the main heap: yaegi desk-session workers
  (at most 12 kept idle, each for at most 30 minutes; busy workers are
  not counted, so the live count can exceed 12; see
  [the cost-track investigation](cost-track-sa-s3-sp-investigation-2026-10-14.md)),
  other guest processes, and
  the guest page cache from reading the store. With no balloon, the host
  never gets these pages back while the guest runs.
- For comparison: small test computers hold 1.2 to 1.3 GiB, and the
  owner's computer 4.4 GiB after 9.7 hours.
- Host: 31 GiB, 16 GiB available now, with 3 computers active.

## The oversubscription question

Earlier testing ran many oversubscribed VMs. That was before yaegi
workers, which are separate processes per desk session. A computer
oversubscribes well only if idle guests hand memory back. Today none do,
because nothing returns it: there is no balloon or free-page reporting,
the yaegi workers stay up 30 idle minutes, and boot replay reads the
whole store. Read on current data: about 4 to 5 paper-sized computers,
or about 20 small ones, fit before pressure without oversubscription.
The probe measures the real number.

## Levers, ranked by expected effect on RAM per computer

1. Free-page reporting or a balloon, so idle guests return memory.
2. Shorter idle life for yaegi workers, or fewer of them (12 is a
   ceiling, not a need). This needs a per-process RSS read inside the
   guest, which no endpoint gives today.
3. Bound the Dolt buffer pools in the event appender (426 MB of 556 MB
   sampled heap).
4. Store compaction (R1): smaller boot replay, smaller page cache.
5. KSM across guests with the same kernel and binaries.

## Plan for the report

1. Deploys tonight in order: L2a S1a (intake fixes and labels), C1e B1,
   lever 6.
2. An in-guest per-process memory read (main process, each yaegi worker,
   page cache), exposed on the existing debug surface.
3. The source sweep: N = 1, 4, 14 and 28 sources, on minted test
   computers, run one after another, 2 hours each. For each N, read
   captures, stories, agents, tokens by desk, RSS, disk and CPU, then fit
   the exponent against N.
4. The oversubscription probe: boot small test computers one by one
   until pressure, with and without the levers that have landed.
