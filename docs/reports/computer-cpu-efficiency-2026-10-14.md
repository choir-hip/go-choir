# Computer CPU Efficiency: Where a Busy Computer's Cycles Go, and How to Get Under One Core

Date: 2026-10-14, 15:15 to 17:15 UTC. Owner question (about 15:15 UTC):
"how can we improve compute utilization of our computers? Running the
agentic workloads really shouldn't use that much compute. I wonder if we
can even go to less than 1 core, as well as how can we be hibernating
machines while their LLMs are processing over the API ... we should think
about what's really the hardcore limit of CPU performance, since that is
our revealed bottleneck. And I know there's security implications of going
to fractional cores ... the Go interpreter Yaegi is a question mark here."

Mutation class of this report: green (docs) plus two read-only scripts,
`scripts/cpu_host_sample.py` and `scripts/cpu_efficiency_probe.py`. It
changes nothing. Each lever names its own class.

It builds on [VM density limits](vm-density-limits-2026-10-14.md). That
report found busy computers bind a host first: the paper averaged about
1.2 cores, so an EPYC 7371 holds about 20 busy computers at about one core
each.

Every number is marked **measured** (with its window) or **estimated**.

Sources:
- read-only Node B reads, 15:17 to 15:57 UTC, all before the 16:45 owner
  demo:
  - per-thread `/proc/<pid>/task/*/stat` deltas for every Firecracker
    process;
  - per-VM KVM debugfs counters;
  - go-choir service cgroup `cpu.stat`;
  - a top-threads sweep;
  - the guest's host-only `procmem` read;
  - CPU vulnerability files and KVM module parameters;
- after 16:45, a 60-second guest Go CPU profile of the paper's main
  process, taken at 16:46:21 through the existing host-sourced
  `/internal/debug/pprof` route, with a host sample over the same window
  (`docs/evidence/computer-cpu-guest-profile-2026-10-14T1646Z.txt`);
- model-call counts from `GET /api/costs`, read with the paper's QA
  credential;
- local benchmarks on an Apple M1 (Yaegi cells, worker start, and
  `Store.AppendEvent` as a run grows). The scratch test files were deleted
  and nothing was committed from them;
- code at `6954f7bb2`;
- Firecracker's `docs/prod-host-setup.md` (main).

The paper computer is `vm-1669020adcece51c2f5ff9b6df399ef5`
(computer-1355b83b…). It has 28 source schedules (since 14:27) and runs
4 vCPUs and 8 GiB.

## The short answer

**Where the paper's ~1.2 cores go** (measured, 16:46 profile plus host
samples):

| part | cores | share |
|---|---|---|
| the guest's main `autoputer` process | 1.17 | 90% |
| KVM halt polling on the host | 0.07 to 0.10 | 7% |
| Yaegi desk workers, guest kernel and other guest processes together | 0.03 | 2% |
| Firecracker VMM and virtio threads | 0.007 | 0.5% |
| platform services working for the paper (corpusd and platform Dolt, the tape's round trips) | about 0.18 | in addition |

Inside the main process, **73% of CPU is the embedded Dolt engine
executing queries**. Most of that is a handful of reads that scan whole
object kinds and parse every row's JSON:
- **44%: one run-end check.** `closeRoundsOfEndedRun` → `OpenRounds` →
  `ConversationState`. At the end of every desk run it lists every
  commitment record whose `inbox`, `sender_inbox` or `conversation_id`
  matches, and each list is a scan with `JSON_EXTRACT(CAST(metadata AS JSON))`
  on every row. It runs once for the inbox and then once per conversation.
- **24%: the work around each cell, not the interpreter.** Each cell
  scans commitment records by body JSON for the claim pack, scans the
  inbox again for the cell's records, and commits the reduction.
- **8 to 10%: event and tape writes.** That is the per-event pipeline
  plus `AppendEvent`'s O(n) sequence scan.

Model-call work is **under 0.5%**: building the request, parsing the
reply, reading the transcript. Yaegi itself is about 2%.

**Is under one core possible?** Yes, by a wide margin.
- The paper made **31 model calls a minute** (624 in the 20 minutes to
  16:50), at about **2.3 CPU-seconds per model call** (2.6 with the
  platform services).
- The work a model call actually needs is estimated at 30 to 60 ms on
  today's architecture and 5 to 10 ms with a lean store. So the floor at
  today's throughput is **0.02 to 0.05 cores**.
- **Target: 0.2 to 0.3 core** after levers C1 to C7:
  - wire the existing generated-column indexes into the paged read, and
    add `inbox`, `sender_inbox` and `conversation_id`;
  - an incremental round fold;
  - claims and cell inboxes by index;
  - a KVM halt-poll setting;
  - an event-sequence high-water mark;
  - one projection per tape event instead of two.
- Most of it is S effort, with orange-class query changes. On the EPYC,
  that is about 50 busy computers by CPU instead of 20. RAM binds again
  first: about 36 at 3 GiB each on 128 GB.

**"Sleep while the model thinks" already holds for CPU.** The paper's
vCPUs are halted 73% of the time, and an idle computer costs 0.008 cores
(measured). What waiting still costs is memory and 0.07 to 0.10 cores of
host halt polling (lever C5). Hibernating mid-call trades CPU for RAM, so
it only pays for waits of more than about 60 to 120 s when RAM binds. The
gateway-parked call (C12) is worth building for durability and batch
pricing, not for CPU.

**Fractional cores** come from per-VM `cpu.weight`, which is
work-conserving. Use `cpu.max` only as a tier ceiling.
- Node B's Coffee Lake Xeon reports L1TF and MDS "SMT vulnerable".
  Firecracker's process has not opted in to indirect-branch protection.
- So for strangers' computers: core scheduling with one cookie per tenant
  (the host kernel has it), a speculation-control opt-in at launch, and no
  KSM.
- On the Zen 1 EPYC, sibling-thread exposure is much smaller (no L1TF or
  MDS), and SMT can stay on.

## 1. Where the cycles go today

### 1a. Host-side split (measured)

Window: 30 minutes, 15:26:40 to 15:56:40 UTC
(`docs/evidence/computer-cpu-host-sample-2026-10-14T1526Z.json`). It
agrees with a 5-minute window at 15:21 (same directory, `T1521Z`) and a
60-second window at 15:18. Cores are host CPU-seconds per second.

| part | paper, 30 min | 5 min | 60 s | what it is |
|---|---|---|---|---|
| vCPU threads, guest mode | **0.95** | 0.90 | 1.20 | the guest's own work: every guest process plus the guest kernel |
| vCPU threads, host kernel (KVM) | 0.09 | 0.12 | 0.09 | almost all of it is KVM halt polling (next row) |
| of which KVM halt polling | **0.094** | 0.104 | 0.098 | from debugfs `halt_poll_success_ns` + `halt_poll_fail_ns`: the host spins up to 200 µs after each guest HLT (`halt_poll_ns=200000`) |
| Firecracker VMM thread (virtio block, net, event loop) | **0.007** | 0.0075 | 0.0075 | 260 wakeups a second, almost no CPU |
| Firecracker API thread | 0 | 0 | 0 | |
| **the paper's Firecracker process** | **~1.05** | ~1.02 | ~1.31 | lifetime average at 15:17 was 1.14 (3,425 CPU-s in 3,014 s, including boot replay) |
| platform Dolt (`go-choir-platform-dolt`) | 0.12 | 0.13 | 0.12 | serves corpusd |
| corpusd | 0.06 | 0.08 | 0.09 | the platform: the guest's tape round trips land here (section 1c) |
| corpus Dolt (Store B), gateway, proxy, vmctl | 0.009, 0.004, 0.0001, ~0.001 | | | the gateway proxies every model call and costs almost nothing |
| btrfs kernel workers (compression, endio) | ~0.02 | | | 60-s top-threads sweep at 15:20 |
| **paper plus host work done for it** | **~1.2 to 1.25** | | | if most of corpusd and platform Dolt is on the paper's behalf (it is the only busy computer; public front-page reads are the rest) |

Two idle computers ran beside it (vm-48bc…, a used test computer, and a
candidate-fleet VM). Each costs **0.008 to 0.009 cores**: 430 KVM exits and
135 halt exits a second, about 0.001 core of halt polling. The density
report's estimate of 0.01 to 0.02 cores per idle VM was high. 800 idle VMs
would cost about 7 threads.

The paper's virtualization counters, per second (30 minutes):
- 10,557 KVM exits, 2,604 halt exits, 3,740 interrupt injections and
  1,128 interrupt exits. The E-2236 has no APICv (`enable_apicv=N`), so
  every interrupt costs an exit.
- 2,548 L1D flushes. L1TF mitigation is "conditional cache flushes, SMT
  vulnerable".
- 149 port-I/O exits. These are the virtio notifications; with
  `--enable-pci`, MMIO exits are 0.
- `halt_wait` was 2.94 of 4 vCPUs. **The vCPUs are halted 73% of the
  time.**
- Disk: 1.70 MB/s written (593 write syscalls a second, about 147 GB a
  day) and 1.1 MB/s read.

What this says:
- **Waiting already costs almost nothing.** A vCPU blocked on the network
  is halted. The only cost of waiting is KVM's 200 µs halt-poll spin, which
  is 0.094 cores, or 9% of the paper.
- **The VMM and virtio are not the problem.** Together they are under 1%.
- **The paper's ~1 core is real computation inside the guest:** about
  0.95 of the 1.05 cores. So the question is what the guest computes.

### 1b. Guest-side split

**Measured.** Window: 60 s from 16:46:21 UTC
(`docs/evidence/computer-cpu-guest-profile-2026-10-14T1646Z.txt`). It was
taken with `scripts/cpu_efficiency_probe.py --profile`, with the host
sample over the same window.
- Host side in that minute:
  - vCPU guest mode 1.19 cores, KVM halt polling 0.07, VMM 0.007;
  - the paper's Firecracker process 1.29 cores;
  - 34 model calls.
- The guest Go profile of the main `autoputer` process (pid 693) holds
  70.05 s of samples in 60 s: **1.17 cores**.

So, by process:

| guest process | cores | share of guest time |
|---|---|---|
| main `autoputer` daemon | **1.17** | **98%** |
| everything else: 12 Yaegi desk-session workers, guest kernel threads (ext4 journal, virtio, net), systemd, `choir-updater` | **0.03** | 2% |

Inside the main process (cumulative, overlapping):

| path | CPU-s / 60 s | share | what it does |
|---|---|---|---|
| `database/sql.(*Rows).Next` (embedded Dolt executing queries) | 50.9 | **73%** | row iteration: prolly-tree cursors, collation compares, JSON evaluation |
| `encoding/json.Unmarshal` | 28.2 | 40% | mostly `JSON_EXTRACT(CAST(metadata AS JSON))` evaluated per row by go-mysql-server, plus record decodes |
| `store.listInboxRecordsBy` → `ogListAllByMetadata` → `ListObjectsByMetadataPage` | 36.5 | **52%** | scans of `commitment_record` objects by `inbox`, `sender_inbox` or `conversation_id` |
| ↳ `recheckRecordsInbox` → `closeRoundsOfEndedRun` → `OpenRounds` → `ConversationState` | 30.7 | **44%** | **every desk run's end**: 2 scans for the inbox's conversations, then 1 scan per conversation |
| desk cell tool, host side (`newDeskGoEvalTool.func1`) | 16.9 | **24%** | around each cell: |
| ↳ `actingCommitmentPackForDesk` → `ListCommitmentClaimRecords` → `ListObjectsByOwnerAndBody` | 7.9 | 11% | every commitment record of the owner, `JSON_EXTRACT(CAST(CAST(body AS CHAR) AS JSON), '$.directive')` per row |
| ↳ `rlmReductionForDeskCall` → `loadCellRecords` → `ListInboxRecords` | 6.5 (5.7) | 9% | the cell's inbox, by the same unindexed scan |
| ↳ reduction commit, desk event | 2.2 | 3% | |
| ↳ Yaegi worker egress service (`serviceEgress`: the daemon side of `choir.*` calls) | 2.3 | 3% | |
| event and tape writes (`projectionTape.appendOps` / `AppendNewPayload` 5.7 s; `AppendEvent` 3.4 s, overlapping) | ~6 to 7 | **8 to 10%** | canonical JSON, two SERIALIZABLE transactions, platform round trips, the sequence scan |
| Go GC (background mark 4.5 s, sweep 2.0 s) plus allocation (`mallocgc` 12.3 s, inline in the above) | | 9% background; 18% allocation | allocation is driven by the scans: every row decoded into maps and strings |
| wake reconcile, actor-wake projector, Texture frame and router, HTTP API | 1 to 2 each | 1.5 to 3% each | |
| model request build and reply parse (`gatewayruntime`), run-memory transcript reads, `CanonicalJSON` | under 0.35 each (below the profile's cut), `CanonicalJSON` 0.6 | **< 0.5% each** | |
| Dolt transaction commits (`DoltTransaction.Commit`) | 1.4 | 2% | |

What it means:
- **The paper's CPU is the store, not the agents.** Almost all of it goes
  to three access paths that scan an entire object kind and evaluate JSON
  on every row.
  - Their cost grows with the number of commitment records ever written
    (captures at about 270 an hour at 28 sources, plus dispositions,
    casts and reports), not with the work being done.
  - Hypothesis, to measure: the paper's CPU per model call rises over the
    computer's life.
- **There is an existing replacement, half wired.**
  - `metadataIndexColumns` (`internal/objectgraph/dolt_store.go:150-160`)
    already maps `run_id`, `trajectory_id` and `channel_id` to indexed
    generated columns, "measured under drain: JSON_EXTRACT full scans at
    ~1.4s/call".
  - But only the unpaged `ListObjectsByMetadata` uses them (`:686-699`).
  - The paged `ListObjectsByMetadataPage`, which every
    `ogListAllByMetadata` call goes through (`internal/store/graph_store.go:138-165`),
    always uses `JSON_EXTRACT` (`dolt_store.go:765-770`).
  - And `inbox`, `sender_inbox` and `conversation_id` have no column at
    all.
  - Per CLAUDE.md's convergence rule, wiring this is the first lever (C1),
    ahead of any patch.
- **Root-cause cluster.** Three records in a week trace to the same
  substrate: the object graph used as a JSON document store with
  scan-and-filter reads under one engine mutex.
  - the Texture list going from 0.15 s to 1 to 2.7 s after idle
    (`internal/server/debug_pprof.go` header);
  - 77% of the main heap under `AppendNewPayload`
    (`docs/problems/autopaper-intake-defects-2026-10-14.md`);
  - this CPU profile.
  
  The substrate-level fix is typed, indexed access paths (generated
  columns or projection tables) for the hot reads: C1 to C4.

### 1c. What the code does per unit of work (read at `6954f7bb2`)

**Model calls do not stream.**
- Agent turns go through `RunToolLoop` (`internal/agentcore/runtime.go:3597`)
  and `gatewayruntime.Provider.CallWithTools` with `Stream: false`
  (`internal/gatewayruntime/provider.go:142`). The reply is buffered with
  `io.ReadAll` and decoded once (`:196`, `:231`).
- So there is **no per-token work in the guest**.
- The legacy streaming `Execute` path (`provider.go:90`, `:352-370`)
  appends a Dolt event per delta, with a quadratic `+=` on the text. It is
  reached only when no tool registry is configured (`runtime.go:3840-3854`).
- On the host, the gateway parses upstream SSE for providers that always
  stream: Responses API events decode into `map[string]any`
  (`internal/provider/provider.go:2546`), with quadratic `+=` on text and
  tool arguments (`:2201`, `:2427`, `:2560`, `:2578`). It measured 0.004
  cores, so this is a tidy-up, not a lever.

**Every call re-reads and re-encodes the whole run.**
- Before each call, the tool loop reads the run's whole transcript from
  embedded Dolt twice: in `compactIfNeeded` and in `contextMessages`
  (`internal/agentcore/run_memory.go:248`, `:256`). Each is a full
  `SELECT … FROM run_memory_entries WHERE loop_id=?`
  (`internal/store/run_memory.go:231-283`).
- It then decodes every message to read its role
  (`toolregistry/toolloop.go:1327`, `:1508`), decodes and canonicalizes
  every past tool input (`gatewayruntime/provider.go:434`, `:486-499`), and
  encodes the whole request.
- The average paper call carries about 22.6 k input tokens today
  (14.1 M over 624 calls, 16:30 to 16:50). On 10-13 it was 48 k (LR).
  That is about 90 KB of JSON, handled about 4 times per call in the guest
  and twice more in the gateway.
- **Measured cost: small.** This is O(run length) per call and O(n²) per
  run, but in the 16:46 profile all of it (`gatewayruntime`, run-memory
  reads) is below the 0.35-second cut, so under 0.5% of the daemon. It is
  worth fixing for long runs, not for the paper's CPU today.

**Every event appended does a scan and a pipeline.**
- `Store.AppendEvent` (`internal/store/store.go:1915-1932`) loads up to
  10,000 earlier events of the same run, with their bodies, and
  JSON-decodes each one to find `max(seq)`. That is O(n) per append and
  O(n²) per run.
- The run's `StreamSeq` was already moved to an in-memory high-water mark
  for this same reason (`:1934-1941`). The per-run `Seq` was not.
- **Measured locally** (Apple M1, plain store without the tape, 4 KB
  payloads), CPU per append:

  | prior events in the run | 1-50 | 100-200 | 200-400 | 400-800 |
  |---|---|---|---|---|
  | CPU per append | 4.5 ms | 9.6 ms | 15.5 ms | 25.6 ms |

  The floor of about 4.5 ms is one embedded-Dolt object write. The growth
  of about 0.03 ms per prior event is the scan.
- In the guest, every write then goes through the tape appender
  (`internal/computerevent/appender.go:338-397`, `:610-697`):
  - 5 HTTP round trips to corpusd (2 × `cas.Head`, `PinNonPrivatePayload`,
    `PinEvent`, `CompareAndSwap`);
  - about 6 to 10 canonical-JSON passes (`canonical.go:16-32`: marshal,
    decode to `map[string]any`, re-encode);
  - one ed25519 verify;
  - a dry-run SERIALIZABLE transaction that applies the projection and
    rolls back (`computer_events.go:96-117`);
  - an insert;
  - a second SERIALIZABLE transaction that applies the projection again
    (`:134-217`).

  `DOLT_COMMIT` runs once per 100 finalizes (`store.go:101-104`), so
  commits are not the cost.
- **Measured** (16:46 profile): the event and tape path together is 8 to
  10% of the daemon, and all Dolt transaction commits are 2%. Its host
  side, corpusd plus platform Dolt, is about 0.18 cores.
- A model call produces about 4 to 10 such events: the provider call (with
  a 2 KB system-prompt preview and 4 KB of user text,
  `toolloop.go:399-422`), the tool loop, tool invoked and result events
  (output up to 100 KiB plus `full_output` up to 512 KiB,
  `batch_executor.go:101-155`), and run-memory messages.

**Polling while idle or watched.**
- The actor dispatcher wakes every 250 ms (`internal/actor/dispatcher.go:172-173`,
  `:258`). Each wake is a cheap SQLite query, but it is 4 wakes a second
  forever.
- Each open Texture editor runs **two 1-second pollers**:
  - The doc stream (`internal/textureowner/texture_observation.go:338-379`)
    calls `textureLifecycleEventPage`, which runs `ListLifecycleEventPage`
    and then `ListLifecycleEvents` again (`:102-142`). Both call
    `ReadObjectSnapshot`, which reads **every object the owner has on the
    computer, with bodies**, in a SERIALIZABLE transaction under the global
    `engineMu` (`internal/objectgraph/dolt_store.go:314-350`;
    `internal/store/lifecycle.go:1933-1969`).
  - The trajectory stream (`agentcore/api_trajectory.go:197-228`) does one
    more such read.
  - Their cost grows with the store, not with what changed.
  - In the 16:46 profile, snapshot reads were about 2%
    (`GetLifecycleSnapshot` 1.6 s), so few or no editors were open on the
    paper.
- The actor-wake outbox audit filters `$.projected` with JSON_EXTRACT and
  no index (`store/lifecycle.go:861`; `dolt_store.go:756-770`). It runs
  every minute and on each signal. Measured at 1.7% (`sweepActorWakeOutbox`
  1.2 s).
- The Dolt GC check runs every 5 minutes (`run.go:679-685`).

**Yaegi cells** are covered in section 2. They are glue code and a small
cost.

## 2. Yaegi

How it runs (`internal/yaegikernel`, `internal/agentcore/tools_desk.go`):
- **One interpreter per desk session, not per cell** (`session.go:84-152`).
  Its life:
  - `interp.New` plus about 810 allowlisted stdlib symbols, plus the
    `choir` verb surface;
  - five prelude evals;
  - a worker **OS process** that re-execs the autoputer binary as
    `desk-session` (`desk_session_worker.go:113-171`).
- Sessions are keyed by desk agent and kept up to 30 minutes idle, at most
  12 (`tools_desk.go:86-91`). The paper had **12 worker processes** at 15:25
  (procmem: 17 to 127 MiB anon each; the main process 2.49 GiB anon).
- **Every cell is parsed and compiled.** That is 6 to 12 `go/parser` passes
  for import checks and normalization, then one incremental yaegi compile
  and an execute. Compiled programs are not cached
  (`session.go:193-361`).
- The real work in a cell is native. `choir.*` verbs cross a socket to the
  daemon (`sidecar.go:154-167`), and stdlib calls go through reflect.

**Measured locally** (Apple M1, `go test -bench`, scratch file deleted;
the Xeon E-2236 is within about ±30% per thread):

| case | yaegi | native Go | ratio |
|---|---|---|---|
| new session (interpreter + symbols + preludes) | 2.4 ms, 1.5 MB alloc | — | — |
| trivial cell (`x := 1; fmt.Println(x)`): the fixed cost per cell | 0.22 ms, 0.5 MB alloc | ~0 | — |
| glue cell: build 50 records, filter, `json.Marshal`, print | **1.28 ms**, 2.1 MB alloc | 9.6 µs | ~130× |
| the same glue cell after 500 earlier cells in the session | 1.52 ms, **4.2 MB alloc** | | +19% CPU, allocation doubles as the session ages |
| compute cell: scan 124 KB of text byte by byte | 8.7 ms | 0.10 ms | ~85× |
| worker process start (`autoputer desk-session`, package init) | ~39 ms of init clock, 689 M instructions (about 0.1 to 0.2 CPU-s on a Xeon, estimated) | | |

Measured cell durations on staging (`docs/reports/reeng-cut0-measurements-2026-10-12.md:83-88`):
- Texture cells: median 1 ms;
- research cells: median 1 to 5 ms (their long tails are waits on
  searches, not CPU).

**Share of CPU (estimate).**
- At about one cell per model call, and a few hundred to a thousand cells
  an hour, cells cost 1 to 2 CPU-seconds an hour: **about 0.001 core**.
- Worker spawns, at about 0.15 CPU-s each, add 0.003 to 0.01 cores even
  with tens of spawns an hour. These happen at research children, after
  poisoning, and after timeouts.
- So Yaegi is **well under 5%** of the paper's CPU.
- **Measured** (16:46 profile): every guest process other than the main
  daemon, the 12 Yaegi workers included, used 0.03 cores (2%). The
  daemon's side of the workers' `choir.*` calls used 3%.
- The cell path's real cost is the 24% the daemon spends **around** each
  cell (section 1b). That is store scans, not interpretation.

Options:

| option | saving | cost / trade-off | verdict |
|---|---|---|---|
| keep Yaegi as is | — | — | **keep**: it is not where the cycles go |
| cache the import check and normalization parse (one parse, not 6 to 12) | ~0.1 ms per cell | S, yellow | only with other work in the same files |
| natives for hot helpers (e.g. `choir.Ids(records)`, filters, JSON shaping) | per cell, 1 ms to µs | each helper widens the verb surface and the cell ABI | do it for helpers that show in a profile, not before |
| cap long sessions (respawn after N cells or M MB allocated) | bounds the +19% aging and the 4 MB/op churn | a respawn costs ~0.15 CPU-s and a re-prelude | after a profile shows aged sessions |
| a prefork worker pool (or a small dedicated worker binary without Dolt) | spawn ~0.15 → ~0.01 CPU-s; ~40 to 60 MiB less RSS per worker (no Dolt or grpc init) | M, orange: the worker binary must stay in lockstep with the verb surface | worth it for the RAM more than the CPU (SA) |
| a different language (Starlark-go, goja JS, Lua, wasm) | Starlark and goja are a similar 10 to 100× tree-walking or bytecode interpreter; wasm needs a compile step | rewrites every desk prompt, the cell ABI and the evidence corpus; models write Go well | **no**: the cost is not in the interpreter |

## 3. The hard floor and the target

**Today's throughput** (measured, `/api/costs`):
- 624 model calls in the 20 minutes to 16:50 UTC: **31 a minute**, about
  1,870 an hour;
- 14.1 M input tokens, an average of **22.6 k per call** (about 90 KB of
  JSON);
- in the profiled minute, 34 calls, with 53% of input tokens cached.
- Captures arrive at about 270 an hour
  (`docs/problems/management-intake-backlog-at-28-sources-2026-10-14.md`).

**Today's cost per unit** (measured, 16:46 window):
- **2.27 CPU-seconds per model call** for the VM: 1.29 cores ÷ 0.57 calls
  a second;
- **about 2.6** with corpusd and platform Dolt.

**The work a model call actually needs** (estimated, Xeon core;
micro-figures from the profile and the local benchmarks):

| step | today's architecture, done once | lean store |
|---|---|---|
| build the request from the in-memory transcript (~90 KB encode) | 1 to 2 ms | 1 to 2 ms |
| gateway: decode, convert, encode, TLS, upstream SSE parse (host) | 2 to 5 ms | 1 to 2 ms (pass-through) |
| parse the reply (~2 to 7 KB) | < 0.1 ms | < 0.1 ms |
| record 4 to 10 events (canonical JSON + SHA-256 per event; one store write each) | 20 to 45 ms (one embedded-Dolt object write measured at ~4.5 ms CPU on an M1, local benchmark) | 1 to 3 ms (append-only log, group commit, batch index) |
| the cell: Yaegi parse, compile and run (measured 0.2 to 1.5 ms) plus the cell's frame by indexed reads | 3 to 10 ms | 2 to 4 ms |
| run-end bookkeeping (close rounds) by indexed fold | 1 to 5 ms | < 1 ms |
| **total per model call** | **30 to 65 ms** | **5 to 10 ms** |

At 0.52 calls a second, that is:
- **0.015 to 0.035 cores** with today's store, done once and indexed;
- **0.003 to 0.005 cores** with a lean store;
- plus an idle base of about 0.01 core (timers, 13 Go runtimes, the
  250 ms dispatcher poll) and the captures (~270 an hour × ~10 to 20 ms of
  fetch, readability and write: < 0.002 core).

**The hard floor for the paper at today's throughput is about 0.02 to
0.05 cores.** Today it uses 1.2 to 1.3, which is **25 to 60× the floor**.

The owner's "JSON work can be a bit CPU intensive" is right in kind, but
not where expected:
- the JSON that costs is not model I/O (< 0.5%);
- it is the store re-parsing every stored record's metadata or body on
  every query (40% of the main process is in `encoding/json.Unmarshal`).

**Target and the path to it** (estimates, from the profile's shares):

| step | levers | the paper's cores after (from ~1.25 VM + 0.18 platform) |
|---|---|---|
| index the inbox and conversation lookups; fold rounds incrementally | C1, C2, C4 | ~0.6 |
| claim records by an indexed path | C3 | ~0.45 |
| no KVM halt polling | C5 | ~0.38 |
| per-run sequence high-water mark; tape done once | C6, C7 (the platform side too) | **~0.25 to 0.3**, and platform ~0.1 |
| GC follows allocation; vCPU count 2 | C9 | ~0.2 to 0.25 |
| stretch: append-only tape log, in-memory folds, slim worker | C10, C15 | ~0.1 |

- **Target: 0.2 to 0.3 core** for a 28-source paper at 1,870 calls an
  hour, and it should no longer grow with history.
- Stretch: 0.1 core.
- One core is not a ceiling Autopaper needs. A 1-core paper would have
  room for about 4 to 5 times today's call rate.

What this does to a host (EPYC 7371, 32 threads, 70% fill less 2 threads
for the platform = about 20 usable threads; 1 Xeon core ≈ 1.3 Zen 1
threads):
- **CPU:** 0.3 core per busy computer → **about 50 busy computers**,
  against about 20 today; 0.15 core → about 100.
- **RAM then binds again:** about 36 busy computers at 3 GiB on 128 GB
  (the density report's quick tier), or about 75 on 256 GB.
- So after C1 to C7, the 256 GB host and the RAM levers (density L1, L5,
  L6, L11) matter again. So do model accounts and dollars: 50 papers is
  about 94,000 model calls an hour.
## 4. "Sleep while the model thinks"

**The CPU part is already true.**
- A vCPU waiting on a model call executes HLT and is descheduled. The
  paper's vCPUs are halted 73% of the time (measured).
- An idle computer costs 0.008 to 0.009 cores (measured).
- What waiting does cost:
  - **KVM halt polling, 0.094 cores** on the paper. This is the one CPU
    cost of waiting that is left, and it has a host knob (lever C1).
  - **Memory**: the guest's 8 GiB stays resident while it waits.

**Hibernating mid-call saves memory and spends CPU.**
- A hibernate plus wake costs CPU:
  - the snapshot write: today 8 GiB, sparse about 1.2 GiB, kernel copy
    plus btrfs zstd;
  - the restore page-in through zstd;
  - the guest's resume work: clock step, credential exchange, consume,
    and the `key_revoked` append.
- Estimate: about 0.5 to 1.5 CPU-seconds per cycle with sparse snapshots,
  and several CPU-seconds today.
- CPU is the busy computer's binding resource. So hibernating mid-call
  makes CPU density **worse** unless RAM is what binds. After the density
  report's quick tier, a busy computer holds about 3 GiB and the EPYC
  binds on CPU, not RAM.

When it is worth it: when all of these are true.
1. RAM binds, measured: host MemAvailable below floor + the burst pool, or
   admission refusals.
2. **Every** in-flight occurrence on the computer is a parked model call
   or a timer. For the paper (28 sources, management, research and story
   desks), that is rare. The 30-minute window never had all 4 vCPUs halted
   for long: halt exits run 2,600 a second.
3. The expected remaining wait is above the break-even. With sparse
   snapshots, a hibernate and wake costs about 1.5 s of wall time
   (hibernate ~1 s, wake 0.34 to 0.52 s). Hibernating saves about 3 GiB ×
   the wait. Below a wait of **about 60 to 120 s**, the RSS saved is too
   small to admit another computer before this one wakes. Today's calls
   are p50 3 s and p90 10 to 15 s, so it is almost never worth it.
4. It did not hibernate in the last N minutes (hysteresis).

Where it is worth it:
- calls that are long by design: batch APIs, with hours of turnaround and
  about 50% off list prices; deep-research endpoints; multi-minute
  reasoning;
- waits for a human, which the density report's L4 (hibernate after a few
  idle minutes) already covers.

The design if we build it: **parked model calls**, lever C12.
- The gateway gets an asynchronous mode:
  - `POST /provider/v1/inference/jobs` returns a job id keyed by run id
    and an idempotency key;
  - the gateway completes the upstream call, keeps the reply durably for
    a bounded time, and records usage;
  - `GET …/jobs/{id}?wait=` long-polls for the reply.
- Today nothing like this exists (`internal/gateway/handlers.go:1047-1059`;
  all routes are synchronous). The guest client holds one HTTP request for
  up to 10 m 30 s (`gatewayruntime/provider.go:35`).
- Hibernating breaks that request. After a snapshot restore, the guest's
  TCP connection to the gateway is gone, and the call would sit until its
  timeout. So a parked call must be fetched again by job id after resume.
- The hibernate fence (`internal/agentcore/hibernate.go:105-118`,
  `:282-300`) would count "waiting on a parked call" as not in flight.
- The gateway then wakes the computer through vmctl when the reply lands.
  A normal wake has a 0.34 to 0.52 s budget.

Correctness against the restart rules (CLAUDE.md, "Restarts End Work
(Crash) Or Resume It (Planned Update)"):
- A snapshot resume is not a boot. The same memory continues, so the
  parked call's continuation is not a "resume after crash".
- If the snapshot fails to load, the fallback cold boot **is** a crash boot
  under the rule. It closes the run with the fate "interrupted by a
  restart", and the parked reply is orphaned. The gateway must still
  record its usage (money was spent) and expire it.
- No boot-time resumer is added. This matches S3's conservative reading
  (`docs/reports/cost-track-sa-s3-sp-investigation-2026-10-14.md`, S3
  section), which the owner is to confirm when S3 opens.

Parked calls are worth building for **durability and batch pricing**, not
for CPU:
- a gateway restart or drain would no longer lose in-flight calls
  (residual D3: model calls are refused for up to 11 minutes during a
  drain);
- batch APIs become usable for Autopaper's non-urgent work.

Station: S3 for the hibernate side, SP1 for the gateway, LR for the
batch-pricing case. Class: red (gateway/provider calls, run acceptance,
VM lifecycle).

## 5. Fractional cores and isolation

**Today:**
- Every Firecracker process runs in vmctl's cgroup
  (`/system.slice/go-choir-vmctl.service`), with `cpu.max` = `max` and
  `cpu.weight` = 100.
- There is no per-VM cgroup, no jailer, and no cpuset (density lever L17).
- Interactive computers get 4 vCPUs and 8 GiB; worker shapes get 2 vCPUs
  and 4 GiB (`nix/node-b.nix:783-792`).
- vCPUs do not reserve host CPU. Node B already oversubscribes: 3 VMs ×
  4 vCPUs on 12 threads. A busy computer can burst to 4 cores, and nothing
  bounds that except the host scheduler.

**How to give a computer a fraction of a core:**
- **`cpu.weight` per VM (do this first).** It is a proportional share and
  work-conserving: an idle host lets a busy computer burst, and a full
  host splits fairly. It needs L17's per-VM cgroups.
  - Weights can encode tiers: the paper and paid computers above free
    test computers.
  - Put the KVM PIT thread and the VMM thread in the VM's cgroup, as
    Firecracker's host doc recommends.
- **`cpu.max` per VM.** A hard ceiling, for example `50000 100000` for 0.5
  core, with `cpu.max.burst` to absorb spikes.
  - It is not work-conserving: a capped computer stays slow on an empty
    host.
  - CFS throttling of a multi-vCPU guest can preempt a vCPU that holds a
    guest lock. The guest has paravirt spinlocks (`CONFIG_PARAVIRT_SPINLOCKS=y`),
    which soften but do not remove this.
  - Use it only as a tier ceiling (free tier, runaway guard), not as the
    density mechanism.
- **vCPU count.**
  - Four vCPUs for a computer that needs a fraction of a core means:
    - more halt and wake cycles (2,600 halt exits a second on the paper);
    - more IPIs and timer interrupts (HZ=1000 while running, no APICv);
    - GOMAXPROCS=4 in each of the 13 guest Go processes. Each runs its own
      GC, with dedicated GC workers at 25% of Ps.
  - Two vCPUs would cut the per-vCPU tax and halve the worst-case burst.
  - Boot replay and Dolt GC would run slower (they parallelize). And a
    2-vCPU guest bursting is still bounded by the host, so 4 is not
    "safer".
  - Lever C9: measure it on a test computer before changing the paper.
- **Latency.** At 0.2 to 0.4 core average with weight-based sharing, a
  burst (boot replay, a large Texture render) still gets whole cores when
  the host has them. The risk is a host at 100% where every computer is
  bursting. Admission (density L1) and per-VM PSI (L17) are the guard,
  with a "host CPU above 80% for 60 s" stop as in the density benchmark.

**Security of sharing cores.**

What Node B reports (`/sys/devices/system/cpu/vulnerabilities`, read
15:18):
- `l1tf`: "PTE Inversion; VMX: conditional cache flushes, **SMT vulnerable**";
- `mds` and `mmio_stale_data`: "Clear CPU buffers; **SMT vulnerable**";
- `retbleed`: IBRS;
- `spectre_v2`: "IBRS; IBPB: conditional; STIBP: conditional";
- `vmscape`: "IBPB before exit to userspace";
- `/sys/devices/system/cpu/smt/control` = `on`;
- the host kernel has `CONFIG_SCHED_CORE=y`, and vmctl's cgroup shows
  `core_sched.force_idle_usec 0` (core scheduling unused);
- KSM `run=1`, but it merges nothing (density finding 5).

The threats, by layer:
- **SMT siblings (same physical core, at the same time).** L1TF and MDS
  let one hyperthread read data the other just touched, across VM
  boundaries. On this Coffee Lake Xeon that is live: two tenants' vCPUs on
  sibling threads is the real exposure today.
- **Time-slicing the same core (fractional cores).** Branch predictor and
  cache state survive a context switch.
  - IBPB and STIBP for host tasks are "conditional", meaning by prctl
    opt-in. **Measured:** the paper's Firecracker process shows
    `SpeculationIndirectBranch: conditional enabled` and
    `Speculation_Store_Bypass: thread vulnerable` (`/proc/<pid>/status`,
    15:58). It has seccomp filters (6) but has not opted in. So the
    process-switch IBPB does not cover it.
  - What remains is KVM's own barrier: KVM issues an IBPB when a physical
    CPU loads a different vCPU than it last ran. That is a reading of
    upstream KVM, not verified on 6.18.16.
  - L1D flush on VM entry is `cond` (`vmentry_l1d_flush`).
  - A launch wrapper that sets `PR_SET_SPECULATION_CTRL`
    (`PR_SPEC_INDIRECT_BRANCH`, `PR_SPEC_STORE_BYPASS`) on Firecracker
    closes the host-side gap cheaply: lever C13.
  - This exposure exists with or without fractional cores. Fractional
    cores only make cross-tenant switches more frequent.
- **Shared LLC across cores (cache timing, Prime+Probe).** Not mitigated
  by SMT-off or core scheduling. Only cache partitioning helps: Intel CAT,
  or AMD L3 QoS on the EPYC, per-CCX placement.
- **Shared memory pages.** KSM and shared pmem (density L10) enable
  Flush+Reload. Keep KSM off for guests and run L10 through a security
  panel (as the density report says).

What Firecracker recommends (`docs/prod-host-setup.md`):
- "disable SMT in production scenarios that require tenant separation";
- disable KSM;
- no swap, or secure swap;
- updated microcode early;
- one tenant per Firecracker process;
- jailer cgroups (`cpu.shares`, CFS quota) for resource control;
- run spectre-meltdown-checker.

It does not discuss core scheduling or overcommit.

**On the EPYC 7371 (Zen 1).**
- AMD is not affected by L1TF, MDS or Meltdown. So the sibling-thread
  exposure is much narrower: cross-thread Spectre v2 needs STIBP.
- Zen 1 is affected by Spectre v1/v2, SSB, Retbleed and SRSO ("Inception").
  The SRSO mitigation (safe RET or IBPB on entry) has a measurable cost on
  VM exits.
- Confirm all of this from the EPYC's own `vulnerabilities` files at
  bring-up.

**What level fits Choir's tenants:**

| tenancy | on Node B (Coffee Lake) | on the EPYC (Zen 1) |
|---|---|---|
| the owner's own computers (the paper, the owner's computer, the owner's test computers) | one tenant: sharing cores and siblings is fine | same |
| strangers' computers (other users; their computers run model-written code and engineering capsules that compile and exec arbitrary Go, so treat them as untrusted code) | **core scheduling with one cookie per tenant** (owner id, not per VM): siblings only co-run one tenant's vCPUs. Or SMT off. Plus IBPB on cross-tenant switches (check it) | core scheduling per tenant, STIBP on, SRSO mitigation on; SMT can stay on |
| a paid "dedicated" tier, if offered | cpuset-pinned whole cores, SMT siblings idle | whole CCX pinning (4 threads per CCX share an 8 MB L3), which also closes most LLC timing |

Costs:
- SMT off costs about 25 to 35% of throughput, not 50%. SMT adds about 20
  to 30% on mixed workloads, so turning it off halves the thread count
  but not the work done. The density report's "with SMT off, busy
  computers roughly halve" is too pessimistic: about 0.7× is the better
  estimate.
- Core scheduling costs the forced-idle time of siblings that have no
  same-tenant work. That is small when each tenant has ≥ 2 runnable vCPUs
  and large when tenants are mostly single-threaded. With 2-vCPU guests it
  is likely 5 to 15% (estimate). Measure it with
  `core_sched.force_idle_usec` from L17's per-VM cgroups.

Fractional cores (weights) and isolation are orthogonal:
- weights decide how much CPU a computer gets;
- core-scheduling cookies decide who may share a physical core at the
  same instant.

Both attach to the same per-VM cgroup or process. Set the cookie from
vmctl at launch with `prctl(PR_SCHED_CORE, PR_SCHED_CORE_CREATE)`, scoped
to the process (thread group), with one cookie per tenant shared through
`PR_SCHED_CORE_SHARE_TO`.

Station:
- SP: EPYC bring-up, SMT and core-scheduling decision, behind a security
  panel;
- L17 (per-VM cgroups) carries the weights.

Class: red (vmctl, VM lifecycle, host kernel).

## 6. The ordered lever list

How to read the table:
- Ordered by saving ÷ effort.
- Savings are shares of the paper's main process (1.17 cores) from the
  16:46 profile, unless marked.
- Classes: G green, Y yellow, O orange, R red. Effort: XS (hours), S (1 to
  2 days), M (a week), L (weeks).

| id | lever | mechanism | saving (paper) | effort | class | station | code |
|---|---|---|---|---|---|---|---|
| C1 | **Wire the generated-column indexes into the paged metadata read, and add `inbox`, `sender_inbox`, `conversation_id`** | `ListObjectsByMetadataPage` uses `metaColumns` like `ListObjectsByMetadata` does; extend `metadataIndexColumns`. Verify with `EXPLAIN` that Dolt seeks | up to ~52% (0.6 core) becomes index seeks plus decoding only the matching rows | S | O (database queries, guest schema DDL at boot) | SA slice 5 (read-cost cuts) / LR | `objectgraph/dolt_store.go:150-200, 686-699, 756-784`; `store/graph_store.go:138-165` |
| C2 | **Close rounds incrementally at run end** | `OpenRounds` folds every conversation the inbox ever joined (2 + C scans). Fold only the conversations the ended run touched, or keep a `conversation_heads` projection updated on record append | most of the 44% that C1 leaves; ends O(conversations × records) per run end | S to M | O (R if the fold becomes a projection on the tape path) | SA / LR | `agentcore/inbox_records.go:1011-1150`; `store/inbox.go:850-902` |
| C3 | **Claim pack from an index, not a body scan** | `ListCommitmentClaimRecords` matches `$.directive` in `body` across all of the owner's commitment records, per cell. Promote `directive` to metadata plus a generated column, or keep an open-claims projection | ~11% (0.13 core) | S | O | SA | `store/commitment.go:122-150`; `objectgraph/dolt_store.go:845-880`; `agentcore/tools_desk.go:817` |
| C4 | **Cell inbox read as a range** | `loadCellRecords` lists the whole inbox and filters by seq; read `(inbox, inbox_seq) > cursor` with an index | ~8% (0.1 core), on top of C1 | S | O | SA | `agentcore/inbox_records.go:144` (called from `rlm_reduce.go:698`); `store/inbox.go:753-770` |
| C5 | **KVM halt polling off (or 10 to 50 µs) on hosts that run many VMs** | `kvm.halt_poll_ns=0` via `boot.extraModprobeConfig`; KVM's own guidance is to reduce polling under overcommit | **measured 0.07 to 0.10 core per busy VM** (7 to 9%), ~0.001 per idle VM | XS | O/R (host kernel parameter; all guests' wake latency +µs) | LR / SP | `nix/node-b.nix` |
| C6 | **Per-run sequence high-water mark in `AppendEvent`** | as already done for `StreamSeq`: an in-memory max per run, seeded once from the newest event | ~5% now; removes the O(n²) per run (measured locally: 4.5 → 25.6 ms CPU per append from 1 to 800 events) | XS to S | R (Trace and evidence surface) | LR | `store/store.go:1915-1941` |
| C7 | **Tape append once** | compute canonical bytes and the digest once per event (6 to 10 passes today); drop the dry-run SERIALIZABLE transaction, or make it the real one; combine the pin, pin-event and CAS round trips into one platform call | ~3 to 5% guest; part of corpusd + platform Dolt's ~0.18 core host | M | R (the tape, the platform CAS protocol) | LR, with the tape owners | `computerevent/appender.go:338-397, 610-697`; `store/computer_events.go:35-233`; `computerevent/http_client.go:282-358` |
| C8 | **Texture streams by signal, one read** | the doc stream reads the owner's whole object scope twice a second per open editor (plus the trajectory stream): wake on the bus, read incrementally from the cursor | ~2% now; scales with open editors × store size (it bites the owner's computer while watched) | S | O | SA / Texture | `textureowner/texture_observation.go:102-142, 338-379`; `agentcore/api_trajectory.go:197-228` |
| C9 | **2 vCPUs for the busy class** (after C1 to C5) | fewer halt and wake cycles and IPIs, GOMAXPROCS 2 in 13 Go processes, burst bounded at 2 | estimated 5 to 10% of what is left; measure first | XS (config) | O (VM shape) | SA slice 7 | `nix/node-b.nix:791` (`VM_INTERACTIVE_CPU_COUNT`) |
| C10 | **Allocation: decode only what is used** | after C1 to C4, check `allocs`: rows decoded into `map[string]any` then re-encoded (`payloadWithPhase`, `toolLoopMessageRoles`, `canonicalJSON` of every past tool input) | GC is 9% background + 18% allocation today, mostly following the scans | S | Y/O | LR | `agentcore/runtime.go:3407-3424`; `toolregistry/toolloop.go:1327, 1508`; `gatewayruntime/provider.go:434, 486-499` |
| C11 | **Event-driven dispatcher** | replace the 250 ms poll with a wake on enqueue (keep a slow audit tick) | idle VM 0.008 → ~0.004 core (estimate); matters only at 500+ idle VMs | XS to S | O | SA / density L21 | `actor/dispatcher.go:172-173, 231-260` |
| C12 | **Parked model calls** (gateway async jobs) | section 4 | no CPU saving; durability across gateway drains and hibernates; enables batch pricing | L | R (gateway/provider calls, run acceptance, VM lifecycle) | S3 / SP1 / LR | `gateway/handlers.go`, `gatewayruntime/provider.go`, `agentcore/hibernate.go` |
| C13 | **Speculation opt-in and core scheduling per tenant** | launch wrapper: `PR_SET_SPECULATION_CTRL` (indirect branch, store bypass) and `PR_SCHED_CORE` with a per-tenant cookie | security, not saving; core scheduling costs 5 to 15% (estimate) on a host with SMT on | S | R (vmctl launch, host) | SP + security panel | `vmmanager/process_control.go` |
| C14 | **Per-VM cgroups with `cpu.weight`** (density L17) | each Firecracker (plus kvm-pit) in its own cgroup; weight by tier; `cpu.max` only as a tier ceiling | the mechanism for fractional cores, per-VM CPU accounting and force-idle measurement | M | R (vmctl) | SP | `vmmanager/process_control.go`, `nix/node-b.nix:715-745` |
| C15 | **Slim, preforked desk worker** | a worker binary without Dolt, grpc and parquet inits; a small warm pool | spawn ~0.15 → ~0.01 CPU-s; ~40 to 60 MiB less per worker (estimate) | M | O | SA (RAM more than CPU) | `agentcore/desk_session_worker.go:113-171`, `cmd/autoputer/main.go` |
| C16 | Gateway SSE tidy-up | `strings.Builder` for text and tool arguments; typed decode for Responses events | the gateway uses 0.004 cores (measured): tidy-up only | XS | O | — | `provider/provider.go:2201-2578` |

Not levers:
- replacing Yaegi (section 2);
- per-token work (there is none in the guest);
- Dolt commit frequency (one `DOLT_COMMIT` per 100 finalizes, 2% of CPU);
- VMM or virtio tuning (0.5%).

Order:
1. **C1 → C2 → C3 → C4.** One small slice each, profiled on the paper
   after each deploy with `scripts/cpu_efficiency_probe.py --profile`.
   Together they are the step from about 1.2 to about 0.45 core.
2. **C5** in the same week. It is a host knob with no code, and the
   measured 0.07 to 0.10 core per busy VM is certain.
3. **C6**, then **C7** with the tape owners.
4. **C9** and **C10**, measured.
5. **C14** and **C13** with the EPYC bring-up (SP), behind the security
   panel.
6. **C12** when S3 opens or when batch pricing is wanted.
7. C8, C11 and C15 alongside SA's density work.

Problem documentation first: the scan finding is recorded in
[guest CPU is store scans](../problems/guest-cpu-is-unindexed-store-scans-2026-10-14.md).
Fix commits should cite it.

## 7. Measurement plan: CPU per unit of agent work

The unit is **CPU-seconds per model call**: VM CPU plus the host
services' CPU on its behalf. Secondary units are CPU-seconds per cell and
per capture. The benchmark's job is to replace this report's estimates.

Tools:
- `scripts/cpu_host_sample.py` runs on the host, read-only. It gives the
  per-VM thread split, KVM counters and service cgroups:
  `ssh node-b 'python3 - 300' < scripts/cpu_host_sample.py`.
- `scripts/cpu_efficiency_probe.py` joins the host split, an optional
  guest CPU profile (`--profile`, through the host-sourced pprof route)
  and `/api/costs` call counts. It prints CPU-seconds per model call:
  `python3 scripts/cpu_efficiency_probe.py --cred <qa.json> --vm-id <vm> --seconds 120 --profile --out <dir>`.
  `/api/costs` scans only the newest 10,000 events (the paper writes more
  than that an hour), so count recent windows only.

Steps (each lever lands and then repeats steps 1 and 2):
1. **The paper, in place, read-only.** A 120-second window with
   `--profile`, three times a day: at a quiet hour, at a capture burst, and
   at a schedule tick. Record:
   - CPU-seconds per model call;
   - the profile's top 10 cumulative frames;
   - the count of `commitment_record` objects. Add this read to the
     probe: a guest-side count through a host-only route, or the inbox
     record routes.

   That record count tests the hypothesis that CPU per call grows with
   history. Acceptance for C1 to C4: CPU per call falls below 0.6
   CPU-seconds, and it stops rising with the record count.
2. **Fixed workload on test computers** (the density report's busy ladder,
   step 3). A replayed 3-source schedule plus 4 fixed prompt-bar
   questions, on the cheapest routed model, on 1, 2, 4 and 8 computers.
   Record CPU per call, per cell and per capture, plus host PSI (cpu some).
   Stop at host CPU 70% or a PSI stop.
3. **Deterministic replay** (removes model variance). Run the fixture
   runner (`scripts/l2a_fixture_run.py`) against a test computer whose
   gateway route returns recorded responses. Then CPU per call measures
   only Choir's own work, and it can gate regressions in CI later:
   - a store-level Go benchmark: run-end `OpenRounds` with N = 1k, 10k and
     100k commitment records;
   - `AppendEvent` at depths 1 to 1,000. The O(1) assertion: append #800
     costs under 2× append #1.
4. **Host knobs, A/B on test computers only.** `halt_poll_ns` 200000 vs 0:
   CPU per call and p99 cell latency. vCPU 4 vs 2 (C9): the same.
5. **Isolation cost** (EPYC bring-up, SP). Core scheduling on vs off with
   8 busy test computers: `core_sched.force_idle_usec` per VM cgroup
   (needs C14), throughput and p99 latency. SMT on vs off: the same.
6. Output: JSONL plus a summary under `docs/evidence/`, with commit, host,
   lever set and window, so each lever's receipt is a before and after
   pair.

Safety, as in the density benchmark:
- never during an owner window or a deploy;
- guest profiles are 60 to 120 s, never on a computer in an owner demo;
- never change the paper's settings, schedules or model policy;
- stop on host CPU above 80% for 60 s or memory PSI some avg10 > 1.0.

## Residuals

- **One profile.** The guest split comes from one 60-second window
  (16:46). Host shares agree across 60 s, 5 min and 30 min windows
  (vCPU guest 0.90 to 1.20 cores). The profile's shares need the three
  daily windows of step 1.
- **The record-count hypothesis is unmeasured.** That CPU per call grows
  with stored commitment records is inferred from the query shapes (full
  kind scans), not yet from a time series.
- **Host services are attributed to the paper by elimination.** corpusd
  and platform Dolt (~0.18 core) also serve public reads. A corpusd
  profile (same pprof route, loopback) would split them. It was not taken
  today.
- **Model-call floor figures are estimates.** The Dolt write cost is from
  a local M1 benchmark without the tape. The gateway figure is from the
  measured 0.004 cores, not a per-call profile.
- **Halt-poll latency effect is unmeasured.** Turning polling off adds
  wake latency in the microseconds per HLT exit; the A/B (step 4) has to
  show cell and call latency unchanged.
- **Whether KVM issues IBPB on cross-VM vCPU switches on 6.18.16 is a
  reading of upstream code, not verified.**
- **Whether the guest loads `cpuidle-haltpoll` is unverified.** The
  module is built (`CONFIG_HALTPOLL_CPUIDLE=m`, `CPU_IDLE_GOV_HALTPOLL=y`).
  If loaded, guest-side polling would count as guest time. A guest read of
  `/sys/devices/system/cpu/cpuidle/current_driver` decides it.
- **SMT-off and core-scheduling costs (25 to 35%; 5 to 15%) are
  literature-level estimates** for this workload; step 5 measures them.
