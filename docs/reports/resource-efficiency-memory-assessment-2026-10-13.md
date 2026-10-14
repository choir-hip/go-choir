# Resource Efficiency: Memory Cluster Assessment

Date: 2026-10-13, about 00:05 UTC on the 14th. Requested by the owner
("we should really do our resource efficiency and cost mission tonight")
after the live paper computer was reclaimed under pressure
(docs/problems/live-paper-computer-reclaimed-no-computer-tiers-2026-10-13.md,
fixed at 188f98c3).

This is the clustering assessment the operating contract requires before
the next fix. Seven memory problem docs in two weeks name one subsystem.
It names the shared roots and ranks the levers. It is not a fix.

## Measured now (Node B, 2026-10-13 23:55-00:05 UTC, read-only)

The host has 31 GiB of RAM. MemAvailable was 21.7 GiB before the paper
computer woke, and 16.4 GiB five minutes after.

| computer | tier | RSS | state dir on disk |
|---|---|---|---|
| owner (candidate-fleet-e15c…) | paid | 4.4 GiB after 8.5 h | 38 GB |
| paper (vm-1669…) | platform | 5.7 GiB, 5 min after boot | 6.9 GB |
| test computer (vm-48bc…) | free | 1.3 GiB | 3.1 GB |
| test computer (vm-9579…) | free | 1.2 GiB | 0.5 GB |

- Host services: platform Dolt 1.26 GiB, corpus Dolt 0.33 GiB, corpusd,
  vmctl, proxy and maild under 0.1 GiB each.
- Ownerships: 186 in all (149 hibernated, 33 stopped, 3 active, 1
  failed). VM state on disk is 76 GB; the root disk is 67% used.
- Guest shape: `VM_INTERACTIVE_MEM_MIB=8192`, a ceiling, not a
  reservation. There is no balloon device anywhere in vmctl, so the host
  never gets back a page a guest has touched until the guest stops.
- Pressure (`nix/node-b.nix:739-748`) trips on any one of these: memory
  available below 4 GiB or 15%; memory-some avg10 above 1.0; CPU-some
  avg10 above 90; IO-some avg10 above 5.0; state-dir disk below its
  floor. Over the last 6 hours, 4 of 185 reclaim samples read
  `pressure=true`. The log does not say which signal tripped, including
  at the 21:00:10 reclaim that took the paper computer.

## The cluster

Earlier memory problem docs, in date order:
- `platform-dolt-oom-realization-cluster-2026-10-01`
- `guest-vm-embedded-dolt-memory-starved-2026-10-01`
- `texture-revision-read-flap-during-oom-2026-10-01`
- `guest-store-history-bloat-and-memory-shape-2026-10-09`
- `node-b-memory-overcommitted-swap-2026-10-09`
- `node-b-oom-from-retained-qa-computers-2026-10-10`
- `new-owner-waits-while-qa-computers-hold-node-b-memory-2026-10-13`

The 2026-10-13 paper reclaim is the latest.

They share four roots.

**R1. A guest's footprint is set by booting a bloated store, and it is
never returned.** The embedded Dolt store is about five times its live
data (guest-store-history-bloat: 27 GiB down to 6.7 GiB under an offline
GC whose durable fixes never landed). Boot replay touches much of it: the
paper computer reached 5.7 GiB within five minutes of boot. With no
balloon, that high-water mark stays with the host until the guest stops.
Every past "fix" raised the guest shape (2 GiB, then 4, 8 and 16) instead.

**R2. Test computers outlive their tests.** Retained QA computers caused
the 2026-10-10 OOM (12 Firecracker processes, about 28 GiB RSS) and the
2026-10-13 new-owner wait. The tracer now stops what it mints, but
nothing else bounds a test computer's life, and test computers have no
tier that is reclaimed before real ones.

**R3. "Pressure" mixes four signals and answers all of them by killing a
guest.** A CPU or IO stall spike, such as a Dolt checkpoint or a deploy
build, triggers the same hibernation as a memory shortage. Hibernating a
guest does not relieve IO, and hibernation is a destructive kill, so the
next wake is a crash boot that closes the computer's work. Which signal
fired is not recorded, so a reclaim cannot be attributed.

**R4. Host Dolt servers grow.** Platform Dolt peaked at 23.7 GiB in the
2026-10-01 cluster. It is at 1.26 GiB now, four hours after a restart.

## Levers, ranked

1. **R3, attribution and signal-specific reclaim.** Small and red
   (vmctl).
   - Record the tripping signals on every reclaim decision.
   - Hibernate only on memory or disk pressure. CPU and IO stall is
     reported, not answered by a kill.
   - This would have saved the paper computer before tiers existed, if
     the 21:00 trip was a stall spike. Attribution is the way to find
     out.
2. **R2, a test tier.** Small and red (vmctl).
   - Computers minted by tracers and QA rank below free, are reclaimed
     first, and are refused before a free computer at memory admission.
   - It needs a durable marker of "test" that is not caller-asserted.
     Candidates: the owner's email domain, the same rule retention
     already uses (`EphemeralEmailDomains`), or a registration flag.
3. **R1, a balloon with free-page reporting.** Large and red
   (Firecracker device config plus guest kernel).
   - It returns touched-and-freed pages to the host, and is the only
     lever that lets always-on computers stop costing their boot
     high-water mark forever.
   - Needs a probe of Firecracker 1.15's balloon support with free-page
     reporting in our guest kernel before any design.
4. **R1, durable store compaction.** Large and red (guest store).
   - The offline GC proved 4x. The durable fixes in that problem doc never
     landed.
   - It shrinks boot replay, so it shrinks the footprint, the disk and
     the boot time together.
5. **R4, host Dolt.** Observe first. It is small now. The old peak came
   from the realization cluster, whose fixes landed.

Not a lever: lowering `VM_INTERACTIVE_MEM_MIB`. A 4 GiB trial thrashed
the owner's store (guest-store-history-bloat). The ceiling follows the
store size until R1 is fixed.

## Model cost (the other half of the mission): first read

Source: the paper computer's `GET /api/costs?limit=10000&detail=1`
(events 2026-10-13 19:24 to 2026-10-14 00:01 UTC; the 10,000-event limit
sets the window). Token counts only. The pricing table knows none of the
models in use, so every call is unpriced and `total_cost_usd` reads 0.
That is a gap in its own right: the cost surface reports $0.

The totals over 1,335 calls are 64.2 M input tokens and 2.3 M output
tokens. Input outweighs output 28 to 1.

| desk | calls | input tokens | output tokens |
|---|---|---|---|
| research | 757 | 48.1 M (75%) | 1.0 M |
| management | 284 | 10.5 M | 0.3 M |
| texture | 294 | 5.7 M | 1.0 M |

By model: deepseek-v4.1-flash carries 1,159 calls, gpt-6-luna 156 and
longcat-2.5-preview-free 20.

Per run, the input grows with every call:

| desk | runs | calls per run (median, max) | first-call input (median) | last-call input (median, max) |
|---|---|---|---|---|
| research | 28 | 22, 73 | 4.6 K | 94 K, 134 K |
| management | 18 | 10, 46 | 8.0 K | 30 K, 99 K |
| texture | 73 | 3, 10 | 7.1 K | 20 K, 111 K |

The longest research run's input per call, in thousands of tokens:
4, 4, 5, 5, 6, 7, 30, 34, 37, 42 … 121, 124, 125, 127, 127, 131, then
28, 31. One large tool result enters the transcript at the seventh call
(7 K to 30 K) and is re-sent on every call after it. A compaction near
130 K resets it. So a run's input cost is roughly quadratic in its call
count.

**R5 (cost root, confirmed from the trace).** The recursive desks keep
tool output in the model's transcript instead of in the cell's own
variables.

Evidence: research run `8fa7b5e9-ebcd-4d76-96fd-fbed9624f91f`, iteration
7 (20:19:30 UTC). The cell looped over six searches and printed each
whole record with `fmt.Printf("… %+v", s)`. Its `tool.result` output was
103,782 characters. The next model call's input went from 7,211 to
30,221 tokens and stayed in every later call. Nothing bounds a cell's
stdout before it enters the transcript. The RLM design point
is that the model sees references and small views while the bulk stays
in the REPL environment. Research, the desk that reads the world, pays
for this most.

Rough effect, an estimate and not a measurement: if a research call
stayed near 10 K input, research input would be about 7.6 M instead of
48 M, and the total about 24 M instead of 64 M, roughly 60% less. Prompt
caching, if the providers honor it on a stable prefix, would cut the
re-sent part further. Whether they do is unmeasured.

**Lever 6 (R5).** Keep bulk tool results out of the transcript: a cell
result above a size bound is stored in a cell variable and the model
sees its handle, length and head. Red-adjacent (prompt and runtime
contract of every desk). It needs the cross-model cell surface work
(docs/reports/cross-model-cell-errors-2026-10-13.md) to stay consistent.
Measured by this same query: research's last-call input median and
input per published story version.

**Lever 7.** Price the models in use, so `/api/costs` reports dollars.
Without that, the owner's cost amendment (roadmap decision 1) cannot be
written against real numbers.

## Proposed order

1. Lever 1 (attribution, signal-specific reclaim). Landed at d3f7d9b2.
2. Lever 2 (test tier). Committed at 1cd32a21.
3. Lever 7 (pricing) and lever 6 (tool output out of the transcript):
   the largest cost lever found.
4. The balloon probe.
5. Store compaction.

Levers 1 and 2 are host-only, so they run beside C1e's shared-runtime
cutovers (one shared-runtime cutover at a time).
