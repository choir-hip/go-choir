# The Cost Track: SA, S3 and SP, Investigated

Date: 2026-10-14, 03:30 UTC. Owner request (2026-10-14, in session):
accelerate the deferred optimization stations, "these go hand in hand
with cost control and cost estimation"; SP stays deferred for building,
but "i want to learn what i should be doing next. the investigation part
is key."

Stations covered:
- [SA, agent density](../definitions/choir-appdev-sa-agent-density-2026-10-05.md);
- [S3, fast resume](../definitions/choir-appdev-s3-fast-resume-2026-10-01.md);
- [SP, production infrastructure](../definitions/choir-appdev-sp-production-infrastructure-2026-10-09.md).

They join [LR](../definitions/choir-ledger-lr-resource-2026-10-13.md) as
one cost track in the
[ledger metamission](../definitions/choir-ledger-metamission-2026-10-13.md).
Facts were refreshed against `main` at `dde6900f`, from code, `git log`
and evidence; every claim below cites one, and hypotheses say so.

## Owner rulings in this session (2026-10-14, about 03:00-03:40 UTC)

- SA and S3 are accelerated into the cost track beside LR.
- SP stays deferred for building ("i think sp is still deferred since it
  requires real infra changes"), but its investigation is wanted: "i
  wont make any changes, but i want to learn what i should be doing
  next. the investigation part is key."
- The shared graph of published objects is **the Autopaper object
  graph** (recorded in AGENTS.md, `58de6f7f`).
- Scaling Autopaper to several computers on one host is a question for
  10-14 or 10-15 ("we can save scaling autopaper to multiple computers
  for later tomorrow or the next day"). The section below is the
  starting analysis; it is a named residual, not a decision.

## The short answer

**What you should do next, in order:**

1. **Make an offline, encrypted copy of the escrow custodian key and the
   platform signing seed.** No new host needed. Both default to files
   under corpusd's artifacts root on Node B's one disk pair
   (`internal/platform/config.go:40-41`), and nothing in the repo
   overrides them or backs them up. Losing that disk pair makes every
   escrow unreadable. If you already hold a copy, nothing to do; the repo
   cannot tell.
2. **Send the per-token prices** of deepseek-v4.1-flash, gpt-6-luna and
   longcat-2.5-preview-free (and any other model you pay for). The price
   table only knows gpt-4 and claude-3 (`internal/llmcost/pricing.go:39-49`),
   so `/api/costs` reads $0 for everything we run.
3. **Name SP0's host.** The only second machine in the repo is node-a
   (shared with another project since 10-09; Choir's remote Nix builder,
   907 GB free; `docs/node-a-shared-host.md`). The SP file calls it "Node
   C"; that name appears nowhere else. Either clear node-a for off-host
   copies and a log sink, or rent one.
4. **Set recovery targets** (how much committed work you can lose, and
   how long the platform can be down) for the tape, Store A and the
   content store. These are the only SP decisions evidence cannot answer.

**What runs without you** (the cost track, in order): a host sampler and
model pricing wiring; a balloon / free-page-reporting probe; a memory
budget for desk workers; GOMEMLIMIT and Dolt cache bounds; automatic
store compaction; then S3 snapshots. The reasons are below.

## Five findings that change the picture

1. **"Hibernate" is a kill, not a snapshot.** `HibernateVM` kills the
   Firecracker process and resume cold-boots from the persistent
   directory (`internal/vmmanager/manager.go:933-961`; the comment says
   "Full memory snapshot/restore is a future Firecracker feature"). Under
   our restart rule a wake is therefore a crash boot that closes the
   computer's work. That is exactly what closed the paper computer's 14
   schedules on 10-13. There is no snapshot code anywhere, and
   Firecracker runs with `--no-api` (`manager.go:1878`), so S3 first
   needs a launch-path change.
2. **Freeing guest memory does not free host memory.** The Firecracker
   config has no balloon and no free-page reporting
   (`manager.go:1649-1665`). GOMEMLIMIT, shorter worker lives and Dolt
   cache bounds all shrink what the guest uses, but the host keeps the
   pages until the guest stops. **The balloon is the multiplier on every
   other memory lever**, so it is probed first.
3. **Nothing bounds memory inside the guest.** GOMEMLIMIT is not set
   anywhere; Dolt buffer pools are unbounded (77% of the paper computer's
   sampled heap sat under them,
   [provisional read](lr-scaling-cost-provisional-2026-10-14.md)); busy
   desk workers have no overall cap.
4. **The "12 desk workers" is an idle cache, not a limit.**
   `deskSessionMaxIdle = 12` (`internal/agentcore/tools_desk.go:52`) keeps
   at most 12 idle sessions warm, each for at most 30 minutes (`:49`).
   It came in with `fcb424b9` (10-10) to stop a worker leak; neither
   number was measured. Busy sessions are never counted, and research
   children get their own, so the live count can exceed 12. The real
   concurrency limits are 4 research children per research call
   (`research_children.go:25`), 4 Jev labels at a time
   (`intake_labels.go:44`), management's one turn at a time, and the
   gateway's per-computer rate limit.
5. **Nobody is alerted to anything.** No metrics endpoint, no alerting;
   the journal is vacuumed to 256 MB daily (`nix/node-b.nix:85`). The
   paper computer's reclaim at 21:00 UTC on 10-13 was found by hand.

## The cost model, as far as it is known

Per computer, cost is roughly: a fixed footprint (process, store, page
cache) + desk workers + model tokens. Per host, it is how many
footprints fit before memory pressure.

- **Fixed footprint.** Small test computers hold 1.2 to 1.3 GiB; the paper
  computer 5.6 GiB an hour after boot; the owner's computer 4.4 GiB after
  9.7 hours. Most of the paper's excess sits outside the main heap:
  workers, other processes, page cache from reading the store.
- **Tokens.** 64.2 M in and 2.3 M out over 1,335 calls on 10-13, input
  dominated 28:1, research 75% of input. Lever 6 (cell output out of the
  transcript) targets that; it deploys after B1.
- **CPU is not a constraint.** Host load 0.5 to 0.9 on 12 cores; model
  calls are remote.
- **Disk.** Store history runs about 5 times live data (R1). A manual
  compaction once took the owner's store from 9.6 GB to 1.7 GB and its
  RSS from 14.7 GB to 2.7 GB (`d339311e`). Host disk was 81% on 10-09
  and 67% on 10-14 after pruning.

What is not measured yet, and how it will be:
- per-worker private memory: procmem (`/internal/debug/pprof/procmem`,
  deployed tonight, reaches each guest on its next idle refresh) splits
  anon from file-backed memory per process;
- per-guest host memory and CPU over time: nothing reports it today
  (vmctl `/list` has no memory field); the host sampler below adds it;
- the curves against sources: LR's source sweep (N = 1, 4, 14, 28).

## Desk workers: from a count to a memory budget

The idle cache trades memory for warm sessions (a returning desk reuses
what it computed; L2a's standing topic researchers make that worth more).
The right size depends on two numbers we do not have yet: each worker's
private memory, and how often a desk returns within the retention window.

Proposed (SA item, after procmem reads on the paper computer):
- keep idle sessions while their total private memory fits a budget,
  closing the least recently used beyond it, instead of a fixed 12;
- give busy workers an admission budget, so a burst queues instead of
  pushing the guest into pressure;
- each worker re-executes the whole autoputer binary
  (`tools_desk.go:30`). Hypothesis: its text pages are shared through the
  page cache and the private cost is the Go runtime and heap. If procmem
  shows otherwise, a slimmer worker binary or a fork server raises every
  budget at once.

## When Autopaper should span several computers on one host

Not yet. It becomes the smart approach when all four hold:

1. **One computer saturates on a serial resource** that memory does not
   fix: management's one-turn-at-a-time queue, the single store writer,
   commit latency. The sweep adds two measures for this: how long
   arrivals wait for a disposition, and management's busy fraction, at
   each N. If waits grow faster than sources, the ceiling is real.
2. **The sources split along a line few events cross.** L2a's value is
   that coverage of one event in several languages meets in one place. A
   split by vertical can work; a split by language cuts exactly the
   merges L2a makes. The registered judge's same-event pairs give the
   crossing rate of any proposed split before it is made.
3. **Computers can adopt each other's publications** on the Autopaper
   object graph (published objects adopted onto the subscriber's tape).
   Designed, not built.
4. **Idle computers cost close to nothing** (S3 plus the balloon).
   Without it, N always-on computers pay N fixed footprints. With it,
   computers that sleep between polls can cost less than one big one;
   by how much depends on each computer's busy fraction, unmeasured.

One more reason, hypothesis only: deploys never restart a busy computer,
and a 24/7 paper may rarely be idle; several smaller computers would take
updates in turn. The paper computer's idle gaps will tell.

## SA, agent density: status

| Slice | Status | Evidence / gap |
|---|---|---|
| 0. Registration genesis | done | `9f6f369c` |
| 1. Management storm convergence | moved to SL | `d94ce9ef`, `e3e96067`, `41822a4c`, `7a36713c` |
| 2. Baseline | partial | LR's provisional read; the sweep and the oversubscription probe take it over |
| 3. Memory | partial | GC guard measures the store (`9858ca1d`); manual compaction (`d339311e`). Missing: automatic compaction, GOMEMLIMIT, cache bounds |
| 4. Race test, then read/write lock | partial | Dolt upgraded past the upstream race (`5b077256`); `engineMu` still serializes every query (`dolt_store.go:62-66`) |
| 5. Read-cost cuts | mostly done | `475902d7`, `c0903096`, `d94ce9ef`; unmeasured under fan-out |
| 6. Operational/versioned store split | not started | possibly partly superseded: capture bodies left Dolt (`c1d21d5e`), 17 tables retired (`8d3437b4`) |
| 7. Shapes and elasticity | partial | shapes tracked (8 GiB in `node-b.nix:741`; the 4 GiB trial `50c7074b` thrashed and was reverted, `0b577b85`); no balloon, zswap or KSM |

## S3, fast resume: status

Not started. Cold boot to healthy is 7 to 11 s across the fleet
(`docs/archive/evidence/o7-boot-cost-vs-history-2026-10-09.md`), so a snapshot
resume mostly buys **keeping work alive across a reclaim**, not speed.
Prerequisites: S1's remaining items, Firecracker's API socket, and a
quiescence fence (today the guest's planned-restart endpoint only answers
409 while runs are running, `bcc556de`; that is a check, not a fence).

Two constraints to settle before building:
- **Size.** An 8 GiB guest's memory file is up to 8 GiB of disk; 149
  computers were hibernated on 10-13. The balloon and compaction come
  first, so snapshots are of lean guests.
- **The restart rule.** A snapshot resume is not a boot; a failed load
  falls back to a cold boot. Conservative reading: hibernate writes no
  planned-restart marker, so a fallback cold boot is a crash boot and
  closes work; the fence guarantees no active work is ever frozen. This
  needs your confirmation when S3 opens.

## SP, production infrastructure: status and what it needs from you

Already in place (from L1 and earlier): deploys no longer take the site
down (`2db4bc02`; 417 of 417 probes answered during the `ecc51d79`
deploy); vmctl restarts leave guests running (`KillMode=process`,
`e55d0f0b`, `dfa4cb51`); docs pushes no longer redeploy (`6fdcf583`).
Still open in SP1: model calls are refused for the whole gateway drain
(up to 11 minutes, residual D3), and corpusd restarts without a handoff.

Not started: off-host copies of anything, log shipping, metrics, alerts,
WireGuard, secrets tooling, host roles. Note that L3's entry criterion is
"a recoverable off-host copy of the ledger"
(`choir-ledger-metamission-2026-10-13.md:137-138`), so SP0 now gates L3.

| Slice | Needs from you |
|---|---|
| SP0, off-host copies and log sink | a host (node-a with your OK, or a rental), CI/SSH access to it, a secrets-tool choice (sops-nix or agenix), an offline key procedure, recovery targets |
| SP1, zero-downtime deploys | no hardware; accept a short corpusd pause with client retry |
| SP2, failover | a standby host that can run guests, DNS control or an OVH additional IP (spend), a recovery-time target |
| SP3, cold audit archive | an OVH storage server (spend), a retention policy |
| SP4, multi-host guests | more hosts (spend); waits on fork re-keying. Not wanted before density: extra hosts buy room, not efficiency |

The SP file's open decisions, read against today's evidence:

| # | Decision | Answer |
|---|---|---|
| 1 | Control-store scale-out | one primary plus a standby: platform Dolt is 1.26 GiB and host load is low; nothing saturates |
| 2 | Content store | a replicated directory, not Garage: content addressing has one writer, and Garage adds a service to run |
| 3 | corpusd handoff | a short pause with retries: corpusd restarted within a second under L1 with the site up |
| 4 | Failover trigger | manual first |
| 5 | Recovery targets | yours |

## The cost track, in order (runs without you)

1. **Host sampler and pricing wiring.** A repo script plus a timer on
   Node B: every few minutes, per guest host memory and CPU (vmctl knows
   the Firecracker PIDs), state-dir bytes, procmem for active guests, the
   `/api/costs` delta and the pressure sample, appended to JSONL. Prices
   go into the cost record as soon as you send them.
2. **Balloon / free-page-reporting probe** on a minted test computer.
   Whether it can be set in the `--no-api` config on Firecracker 1.15.1
   with the guest kernel we build is the first question.
3. **Desk-worker memory budget**, GOMEMLIMIT and Dolt cache bounds, sized
   from procmem.
4. **Automatic store compaction** (red): shrinks disk, boot replay, page
   cache and any future snapshot.
5. **The engine lock**, only after contention is measured under load.
6. **S3 snapshots**, after 2 and 4.
7. **The store split**, last, re-scoped after the re-engineering work.

Throughout: no budget caps (owner ruling 1, "track spending, but dont set
budget caps"). Memory budgets above bound the guest's own processes for
density; they never refuse or cap model spend.

## Owner principle (2026-10-14, about 12:10 UTC)

"Just really optimizing our system till it's a fine-tuned, well-oiled
machine is what gives us the confidence to scale intelligently. The
knowledge that we're doing the best we can with the resources we have is
what encourages us to invest more resources into doing even more."

So the cost track runs to diminishing marginal returns before Autopaper
scales its utilization, and each optimization lands with a measurement
that shows its effect, so that "the best we can" is a number, not a claim.
