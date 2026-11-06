# MA6.6: code mode with per-desk import sets (2026-11-06)

Class red, tier 2. Rollback: revert. Plan row:
docs/design/modular-architecture-plan-2026-11-04.md, MA6.6; rulings R26 (each
desk has its own least-privilege yaegi import set; code mode: a hundred tool
calls in one cell with one model round trip) and R42 (third-party Go modules
gated by capability); MA2.3 (the capability list carries the import set).
Built on MA6.4 (every desk worker under gVisor) and MA6.5 (one stdlib set,
`internal/gomodule.Stdlib()`, and package desk `go_imports`).

## 1. What existed, and what this slice adds

Before this slice:

- the package part of every desk's set was one list, `gomodule.Stdlib()` plus
  `choir` (MA6.5), carried to every worker by four call sites as
  `DefaultSafeStdlibPackagesList()`;
- the choir verbs a cell can call were already derived from the desk's grants
  (`deskmanifest` resolve computes `Desk.Verbs`; `ChoirScope.ChoirExports`
  binds only those);
- a package desk's `go_imports` reached the static check through the module
  bundle (MA6.5);
- the worker built its allowlist from whatever list it was carried, and an
  empty list fell back to the default set;
- the daemon served a cell's host calls one at a time, with no count bound,
  and every call added a row to the cell's `desk.cell` trace event.

This slice adds:

1. **The derivation, named and in one place** (`yaegikernel.DeskImportSet`,
   section 2): a desk's set is `{Packages, Choir, GoImports}`, computed from
   its manifest row, with a digest.
2. **The worker re-derives it and refuses a wider list** (`DeskAllowlist`):
   the daemon's carried list may only narrow the set; an empty list is the
   desk's own set, not a default; a blank or outside package refuses the
   worker at build (fail closed).
3. **A per-cell host-call bound and bounded host parallelism** (section 4),
   from measurement.
4. **The intent quota set deliberately** (section 5): 16 to 64.
5. **The trace stays bounded**: the `desk.cell` event keeps the first 256
   egress rows and counts every call (`egress_calls`, `egress_omitted` by
   verb).
6. **Package desks' sets are computed from the hashed document**
   (`apppackage.Manifest.DeskImportSets`), so one surface hash means one set
   (section 3).

## 2. The derivation rule: grant to import set

| Part | Every desk gets | A grant adds | Source |
|---|---|---|---|
| Packages | `choir` (the precommitment-records protocol package) and the desk stdlib set: `math`, `math/big`, `sort`, `strings`, `strconv`, `encoding/json`, `sync`, `context`, `slices`, hashes, compression, csv/xml and the rest of `gomodule.Stdlib()` (54 packages) | nothing | `gomodule.Stdlib`, closed list |
| Choir | the observation tier: `ReadFile`, `ListDir`, `Context`, `Records`, `Pack`, `Help`, `CellOut` | its verbs (`deskmanifest` grant catalogue: `act.cast` adds `Cast`, `world.read` adds `Read`, `SearchStart`, ...; `updates.read` adds `Updates`); a read-only scope (research roles, a management task, an unknown role) drops filesystem mutation | `Desk.Verbs`, `ChoirScope.ChoirExports` |
| GoImports | none | a package desk's declared, verified `go_imports` (MA6.5, R42) | the module bundle |

Why no grant adds a package: R26 says most desks import nothing with effects,
and with the MA6.4 sandbox in place no desk needs to. Every effect a grant
carries (the network, the filesystem in a capsule, a canonical write, the
product API) already reaches a cell as a choir verb that the daemon serves and
checks frame by frame (`admitFrame`, `FrameAllowedAt`). A stdlib package with
effects (`os`, `net`, `net/http`, `os/exec`) would be a second route around
the frame check, so the rule is that effects are verbs and never packages.
The choir part is therefore the effectful part of the set, and the part that
differs between desks.

Considered and not taken: narrowing the pure base per grant (for example
`encoding/xml` and `compress/*` only for `world.read`). Pure packages reach
nothing; narrowing them buys attack surface only against yaegi symbol bugs,
which the gVisor sandbox already bounds, and costs the expressivity MA6.5
widened for (R13). Named residual N1.

The computed sets of the built-in desks (pinned by
`TestMA66EachDeskComputesItsOwnImportSet`):

| Desk | Packages | Choir beyond observation (examples) | Not in its set (examples) |
|---|---|---|---|
| management | base | Cast, Ask, Report, ProductAPI, Stories, Feed, Decline, Delegate, Verify, Updates | Write, WriteFile, Exec, Read, SearchTexture, Put |
| texture | base | Cast, Ask, ReadDoc, ReadCapture, Write, Done, Wait, Reject, Updates | ProductAPI, Read, WriteFile, Delegate |
| research | base | Read, SearchStart, SearchWait, SaveEvidence, ReadEvidence, Research, System1, SearchTexture | Write, WriteFile, Exec, ProductAPI, Delegate |
| engineering (capsule) | base | WriteFile, Exec, Start, Poll, Stop, Freeze, Complete, Precommit | Cast, ProductAPI, Read, Write, Delegate |
| a package desk with `objects.own` | base (+ its `go_imports`) | Put, Get, List, Retract | Cast, ProductAPI, Read, WriteFile |

## 3. Where it is carried, and the surface hash

- **Built-in and app desks at run time**: the daemon passes
  `DeskAllowedPackages(role)` to the worker (`tools_desk.go`, the capsule
  broker's session config and exec-go-session stage); the worker computes
  `DeskImportSet(cfg.Role)` from its own row (an app desk's row crosses
  execve, MA4.2) and refuses a list outside it. The choir part is bound by
  `ChoirExports` from the same row.
- **Package manifests**: the capability list is the authored part; the import
  set is derived from it and never authored (MA2.3).
  `Manifest.DeskImportSets(src)` builds each desk's package set from the very
  document the behaviour surface hashes (the base plus the hashed
  `go_imports`), so on one platform version one hash means one set, and any
  change to a desk's set moves the hash (pinned by
  `TestMA66DeskImportSetIsStableUnderTheSurfaceHash`). The base itself stays
  out of the hash, as MA6.5 decided: it is a platform fact, and a platform
  release that widened it would otherwise move every package's surface.
- **The digest** (`ImportSet.Digest`, sha256 of canonical JSON) is stable
  under grant order and computation, and moves when a grant adds a verb
  (`TestMA66ImportSetDigestIsStableUnderGrantOrder`).

## 4. Frame calls: measured, then bounded

The hypothesis in the plan row was that a cell's frame calls are bounded only
by the cell deadline. Measured (macOS arm64, native worker, the real
`desk_go_eval` tool, daemon egress handler and SQLite store; reproduce with
`MA66_MEASURE=1 go test ./internal/agentcore -run TestMA66MeasureHostCalls -v`):

| Calls in one cell | Time | Per call | `desk.cell` event bytes, before | after |
|---|---|---|---|---|
| 100 | 46 ms (634 ms with the first spawn) | 0.2-0.5 ms | 29,910 | 29,910 |
| 1,000 | 194 ms | 0.19 ms | 297,080 | 76,368 |
| 5,000 | 847 ms | 0.17 ms | 1,484,282 | (refused past 1,024) |

Worker-only (fake host): 100 calls 9 ms, 1,000 60 ms, 10,000 493 ms (about
50 microseconds a frame round trip); worker RSS stable at 27 to 33 MB. Host
allocation about 29 KiB a call (5,000 calls, 143 MiB). Eight goroutines each
calling a 100 ms verb took 810 ms: the daemon served a cell's frames one at a
time, so fan-out gained nothing.

So the hypothesis held and is the problem: nothing but the deadline bounded
the count, a five-minute cell (management's `cell.deadline`) could make over a
million calls, and texture, research and package desks have no host cell
deadline at all. Each call cost a trace row (about 300 bytes) in one event.

Chosen bounds:

- **`MaxFrameCallsPerCell = 1024`**, enforced by the daemon in
  `DeskSessionWorker.serviceEgress` (the trust boundary, not the worker). A
  call past it gets an error the cell sees ("continue in the next cell"); the
  first refusal in a cell is traced through the refusal path. 1,024 is ten
  times the hundred code mode is for, and bounds one cell's host work to about
  200 ms of store reads and 29 MiB of allocation for cheap verbs. Calls the
  worker serves itself (`ReadFile`, `ListDir`, `Records`) are not host calls.
- **`MaxConcurrentFrameCalls = 8`**: the daemon admits each frame in order
  (slot token, epoch, frame table, bounds; a forged frame still ends the
  worker before a later frame is served), then runs its handler in a goroutine,
  at most eight at once; a full set holds the reader, so the worker's next call
  waits. Every handler finishes before the cell's result is delivered, as when
  they ran one by one. Measured after: sixteen 100 ms calls in 206 ms, peak
  eight in flight. `choir.Research` stays serial within a cell (each call can
  start 16 child runs with their own workers).
- **The trace**: the `desk.cell` event keeps the first 256 rows
  (`maxCellEgressRows`) and records `egress_calls` always and `egress_omitted`
  by verb past the bound, so the event stays under about 80 KiB.

Both constants are in the desk protocol lock as quotas (11.1.0), so lowering
either is a breaking change.

## 5. The intent quota

`MaxIntentsPerCell` was 16, set when the tray held only messages: sixteen
16 KiB bodies fill the 256 KiB tray. Code mode's reads are host calls, not
intents, so the quota does not bound reading. It does bound acts staged in a
loop: a package desk's `Put` per object, management's `Decline` per capture.
Set to **64**, where the byte quota binds at a 4 KiB average act (64 x 4 KiB =
256 KiB). The per-kind rules stay (one Complete, Freeze, Verify, Write, Done,
job start and engineering Cast per cell), the MA3.9 object quotas (10,000
live objects, 600 writes an hour) stay, and the reducer commits a cell's
intents in one records transaction. Raising a quota is a minor protocol
change (11.0.0 to 11.1.0).

## 6. Failure modes, written before the code

| | Failure | Guard | Test |
|---|---|---|---|
| F1 | a desk's set holds a package with effects, or a verb its grants do not give | the base is `gomodule.Stdlib` (closed) plus `choir`; the choir part is `ChoirExports` of the row | `TestMA66EachDeskComputesItsOwnImportSet` |
| F2 | the set moves with grant order or between computations | sorted canonical JSON; computed from the resolved row | `TestMA66ImportSetDigestIsStableUnderGrantOrder` |
| F3 | a carried list widens the worker, or an empty one falls back to a wider default | `DeskAllowlist` subset check; empty is the desk's set; blank or outside refuses the worker | `TestMA66CarriedPackagesOnlyNarrowTheSet` |
| F4 | a cell imports outside its set and the static check passes it | `checkImports` against the per-desk allowlist (fail closed on parse failure, MA6.5) | `TestMA66CellImportingOutsideItsSetIsRefusedAtTheStaticCheck` (real worker) |
| F5 | host calls unbounded, or the bound refuses a hundred | daemon-side count per cell | `TestMA66AHundredHostCallsInOneCellAndTheBound` |
| F6 | parallel host calls serialized, or answered after the cell result | semaphore of eight; wait group before the result | `TestMA66ParallelHostCallsRunInParallel` |
| F7 | the intent quota drifts | pinned to 64 and to the tray bytes | `TestMA66IntentQuotaIsDeliberate`, `TestTrayQuotas` |
| F8 | a hundred reads cost more than one model round trip, or the trace does not show it | the cell loops; `desk.cell` carries `egress_calls` | `TestMA66AHundredReadsInOneModelRoundTrip`: a management run whose first model call writes one cell of 100 `choir.Stories` reads; the run's trace has exactly two `tool_loop` events (one with the `desk_go_eval` call, one ending the turn) and one `desk.cell` event with `egress_calls` 100 and 100 rows |
| F9 | concurrent handlers race on per-run or per-cell state | the label accumulator, the cell egress log and the search store are mutex-held; `Research` stays serial | the yaegikernel tests and the agentcore desk tests under `-race` on the agents VM |
| F10 | a package desk's set differs between two releases with one hash | the set is built from the hashed document | `TestMA66DeskImportSetIsStableUnderTheSurfaceHash` |

## 7. Residuals

- **N1** No grant implies a package. If a grant ever needs one (a package
  with effects would be a design change against section 2), the table belongs
  in `gomodule` beside the stdlib list, and the package set would then vary
  by desk.
- **N2** The Library does not show a desk's import set yet. The data is
  `DeskImportSets` and `DeskImportSet`; the view is a front-end slice.
- **N3** Fan-out costs code mode now allows (review r1, S5). Several desks'
  cells already ran in parallel; within one cell the new peaks are:
  - up to 64 staged acts a cell (`MaxIntentsPerCell`), so 64 Casts, each
    waking another desk's run (engineering Casts stay one per cell);
  - eight host calls in flight a cell, each of which may be a model call
    (`System1`, `Read`, a search); `System1` calls serialize on the runtime's
    `system1Mu` while holding their semaphore slots, so a cell of eight
    System1 calls fills its slots and queues its other calls behind them;
  - `choir.Research` serial within a cell, but each call still starts up to
    16 children with their own workers, and eight cells of one computer could
    reach 16 x 8. Desk-worker admission (MA6.4) bounds the workers; nothing
    bounds the model calls, which is a code bound to set when measured, not a
    spend limit.
- **N4** Refusals are traced up to 16 a cell, then once more saying the rest
  are not (review r1, S3); the later ones are seen only in the cell's stdout.
- **N5** The egress handler's per-call cost is dominated by store reads and
  the trace row; the 29 KiB of host allocation a call is not investigated
  further here.
- **N6** Root-mode gVisor (a cgroup-delegated root spawner) was not run for
  this slice; the rootless leg was (below).

## 8. Review r1 (two seats, accept; the items fixed)

- **S1** A search's results reach the cell from the moment `startSearch`
  launches the stream, but the run was tainted `world_untrusted` only after
  it returned, so a sibling `SaveEvidence` could stamp evidence without the
  label. The run is now tainted before the search starts
  (`TestMA66r1SearchTaintsTheRunBeforeItStarts` fails without it).
- **S2, D4** Each cell's handlers run under a per-cell context. It is
  cancelled when the cell ends, when the reader stops (a read error or a
  forged frame) and in `killLocked`; an aborted cell waits up to 5 s for its
  in-flight handlers. A forged frame is traced (and trips the breaker,
  revoking leases) only after that wait, so a `RequestLease` admitted before
  it either finished before the trip (and the revocation covers it) or sees
  its cell cancelled: `requestLeaseEgress` refuses a cancelled cell, and
  revokes a grant that raced the abort. Tests:
  `TestMA66r1ForgedFrameStopsHandlersBeforeItIsTraced`,
  `TestMA66r1HostKillCancelsTheCellsHandlers`,
  `TestMA66r1AnAbortedCellIsGrantedNoLease`.
- **S3** Refused frames count toward `MaxFrameCallsPerCell`; refusals are
  traced at most 16 a cell plus one saying the rest are not.
- **S4, D5** `noteStoryLibraryShown` keeps the highest head (a CAS loop).
- **D1, the option taken** A disclosure-gated sink (`Read`, `SourceAPI`,
  `SearchStart`: `yaegikernel.DisclosureSinkActions`, pinned equal to
  `agentcore.egressDisclosure` by a test) is served only after every non-sink
  call admitted before it has finished, so the labels it is gated on include
  their taint. This is the simplest correct order: no taint sequence, just a
  wait group of in-flight reads on the frame loop. Sinks do not wait for each
  other (a sink's payload cannot carry a result still in flight), so
  research's parallel `Read`s keep their fan-out.
  `TestMA66r1ASinkIsGatedAfterEarlierReadsTaint`.
- **D2** `Tray` has a mutex: each per-kind check (one Complete, Freeze,
  Verify, Done, job start) and its stage are one step, and a closed tray (the
  cell has returned) stages nothing; the scope's tray pointer is atomic.
  `TestMA66r1ConcurrentCompletesStageOne` and, in a real worker,
  `TestMA66r1TwoGoroutinesStageOneCompleteInACell`.
- **D3** `PollSource` is serialized per source within a run (a reference
  counted lock map, `poll_source_lock.go`).
  `TestMA66r1PollsOfOneSourceAreSerialWithinARun`.

## 9. Where it ran

- macOS arm64, native worker: the MA6.6 tests in yaegikernel, apppackage and
  agentcore, and the agentcore desk subset (`-run
  'Desk|Egress|Cell|Research|Records|Intake|Stories|Trace|Isolation|Object|Package'`).
- Agents VM (amd64), `-race`, native worker: the MA6.6 yaegikernel tests and
  the agentcore desk, egress, trace, research fan-out and MA6.6 tests.
- Agents VM, rootless gVisor (`CHOIR_RUNSC`, runsc 20261102.0): every
  `TestGvisor*` test, including `TestGvisorCodeModeHostCallsAndImportSet`: 100
  host calls (1 ms handler) in one cell in 185 ms; 1,034 calls served 1,024 in
  2.09 s; 16 parallel calls joined in 9.6 ms; `encoding/gob` refused inside
  the sandbox.
