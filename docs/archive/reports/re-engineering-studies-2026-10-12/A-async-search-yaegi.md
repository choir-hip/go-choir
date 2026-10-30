# Async search with Go cells: how cells run, what can arrive mid-run, and a design

Research note, 2026-10-12. Read-only. No repo file was edited. Three probes ran as
`go test -overlay` files that live only in the scratchpad
(`scratchpad/yprobe/zz_probe_test.go`, `zz_egress_test.go`, `zz_store_test.go`),
so the repo tree stayed clean (`git status --short` is empty).

Owner direction this note works under: search is asynchronous. No waiting for
every provider and no grouping into waves. The model responds as each
provider's results arrive and may report progress to Texture each time. No
budgets and no runtime-owned research job: the research RLM (root and
sub-RLMs) owns research. Provider failures are normal and are reported as
"M of N search providers active", not as "degraded".

Labels: **[verified]** means read in code and, where marked, run in a probe.
**[inferred]** means reasoned from code but not executed.

---

## 1. How a Go cell executes

### 1.1 The carrier chain

`desk_go_eval` (internal/agentcore/tools_desk.go:254-421)
→ `deskSessionWorkers.acquire` (tools_desk.go:120-162) gets or starts a
**separate host subprocess** (`SpawnDeskSessionWorker`,
internal/yaegikernel/desk_session_worker.go:95-152; it re-execs the daemon as
`desk-session` and talks over a unix socketpair on fd 3)
→ `DeskSessionWorker.EvalCell` (desk_session_worker.go:294-367) writes one
`SessionFrame` (session_loop.go:25-49)
→ the worker's `RunSessionLoopFramedWithHooks` (session_loop.go:312-350)
→ `serveCell` (session_loop.go:141-184)
→ `Session.Eval` (internal/yaegikernel/session.go:147-292) on **one persistent
`interp.Interpreter`** (session.go:30-53, 81-125).

### 1.2 Persistence across cells within a run [verified, probe]

Yes. One interpreter serves every cell until the session dies (session.go:18-22,
"retains variables across sequential eval cells"). Imports, types, vars,
closures, **channels, and running goroutines all survive** between cells. The
probe confirmed each of these:

- cell A started three goroutines that send on `ch` after 50, 300 and 900 ms,
  and returned at the first arrival (`=> 1`, 53 ms);
- a later cell drained `ch` with a `select{default:}` closure, and a later
  cell still saw all three arrivals in `got`;
- a closure `next := func(timeout) string { select {...} }` defined in one
  cell blocked correctly in later cells: 102 ms, 252 ms, 600 ms, then `done`.

Limits found in the same probe:

- A cell that mixes a `type`/`func` declaration with statements fails to
  compile ("expected declaration, found ch"; "expected '(', found drain").
  Declare types in their own cell. Use `f := func(){}` closures, not
  `func f(){}`, inside statement cells.
- `sync` and `context` are **not** in the allowlist
  (internal/yaegikernel/allowlist.go:10-27). Model code has goroutines,
  channels, `select` and `time`, but no `WaitGroup` or `Mutex`.

### 1.3 Persistence across runs [verified, code]

Yes, by design (PD2). The session key is the agent id (`"agent:"+AgentID`,
tools_desk.go:108-116), so the next run of the same desk reuses the REPL.
A session ends on any of these (tools_desk.go:42-53, 177-202):

- a process restart (sessions live only in memory);
- a poisoned cell;
- a timed-out cell;
- 30 minutes idle;
- LRU eviction beyond 12 idle sessions.

If the desk's session is busy with another run's cell, a parallel run gets
its own session (tools_desk.go:126-129). Research children always get
private sessions that are released when the child ends
(research_children.go:204-205).

Incidental [inferred]: the session's broker handle is issued once, with
`time.Hour` TTL (choir.go:120), and verified with an expiry check on every verb
(handles.go:113). A desk session kept alive across runs for more than an hour
would therefore have every `choir.*` verb refused with "handle has expired".
Nothing in `acquire` caps session age. This was not checked on staging.

### 1.4 Goroutines still running when a cell returns [verified, probe + code]

- **They keep running.** `Session.Eval` waits only for `interp.Execute` to
  return (session.go:223-236). Goroutines the cell spawned are ordinary Go
  goroutines in the worker process.
- **Their stdout is lost.** The interpreter's writers are `switchWriter`s
  (session.go:55-76). Each Eval points them at that cell's buffer
  (session.go:214-215) and never resets them afterwards. A late print goes
  into the previous cell's already-returned buffer. Probe: "late print from bg
  goroutine" appeared in no result.
- **They are killed only when the process dies.** That happens on a timeout
  (host `ctx.Done` → `killLocked`, which kills the process group,
  desk_session_worker.go:363-365, 454-464), on a poisoned cell (worker exits,
  session_loop.go:346-348), on `Close`, or on a restart. A panic in a cell
  poisons the session (probe: the next cell got "session poisoned, respawn
  required"), and with it every background goroutine and channel.
- `Session.Eval` calls `interp.Execute(prog)` **without a context**
  (session.go:235). In-process cancellation is impossible, so the only stop
  is the kill. `serveCell` passes `context.Background()` (session_loop.go:146).

### 1.5 Cell timeout [verified, code]

- **None by default.** `timeout_ms` is optional (tools_desk.go:257, 264).
  When it is set, `context.WithTimeout` applies (tools_desk.go:348-353).
  Expiry kills the worker, and the model is told its earlier values are gone
  (`cellTimeoutError`, tools_desk.go:879-882).
- **Otherwise the run's activation context bounds the cell.**
  `ExecuteActivationSync` sets `context.WithTimeout(ctx, rt.cfg.ActivationBudget)`
  (runtime.go:372-374). The default is 60 minutes
  (provideriface/config.go:24). `armCellTerminalDeadline` mints a durable
  timeout wake at the same deadline (continuation_schedule.go:93-109).
- **Output cap:** 2 MiB per stream (eval.go:250).
- **Value cap:** 48 000 chars for the shown value (session_loop.go:191).
- **Frame cap:** 4 MiB per frame (transport.go:54).

### 1.6 How output reaches the model [verified, code]

`serveCell` builds `SessionResult{Stdout, Stderr, Error, Receipts, Intents,
Reuse, DiagKind}`. It then appends three things to Stdout:

- `=> name: value` for bare names listed before the last statement
  (session_loop.go:172, session.go:316-340);
- `=> value` for a final bare expression (session_loop.go:173-175, 197-211);
- refusal lines (session_loop.go:176-182).

The host reduces staged intents only when the cell succeeds
(tools_desk.go:384-401, `reduction.commit`). It marshals
`{stdout, stderr, error, duration, reuse, diag, receipts, intents, session}`
as the tool result string (tools_desk.go:402-416). The tool loop appends that
as a `tool_result` user message (toolloop.go:662-670).

---

## 2. Can the model see anything before a cell returns? Can the loop inject between calls?

### 2.1 Within a cell: no [verified, code]

The model is not running while a cell runs. It sees results only at tool-result
boundaries. Mid-cell traffic exists only between the worker and the host:

- `StreamBrokerEmit` and `StreamBrokerEgress` are serviced inline by the
  host's reader goroutine (desk_session_worker.go:314-334);
- `StreamCancel` exists (session_loop.go:324-325).

None of that reaches the model until the cell's `StreamCell` result.

### 2.2 Between model calls: yes, `InjectUserTurnsFunc` exists and runs after every tool batch [verified, code]

- `toolloop.go:18-22`: "splice additional user turns into a running loop
  between model iterations… runtime-owned inbox delivery".
- After each tool batch the loop calls `injectUserTurns(false)` and persists
  the turns as `runtime_injection` (toolloop.go:672-680).
- The production injector is `coagentUpdateTurnInjectorWithInitialPhase`
  (management_controller.go:2571-2626). It derives "seen" from durable run
  memory, then appends three kinds of turn:
  - pending owner-revision turns;
  - **pending `update_coagent` packets** addressed to the agent;
  - emit notices.
- It is wired for every desk run (runtime.go:3440, 3525). It covers
  management, engineering, research and texture
  (`runSupportsCoagentUpdateInjection`, management_controller.go:2783-2793).
- The cell frame also carries the cell-start snapshot of the same pending
  records (`choir.Updates()`, `choir.Emits()`, `choir.Inbox()`;
  session_loop.go:25-49, choir.go:143-168, tools_desk.go:356-362).

### 2.3 At end of turn: park and passivation [verified, code]

- `WithParkWaiter` runs when the model returns `end_turn`
  (toolloop.go:943-990). `Continue` injects new turns and calls the model
  again. `Passivate` returns `ToolLoopPassivatedError`, and the run goes to
  `RunPassivated`.
- Actor mode (`actor_park_on_idle`, set only for Texture,
  textureowner/texture_agent_revision.go:513) passivates immediately when
  nothing is pending. A later `update_coagent` re-activates it
  (management_controller.go:2764-2778).
- Management parks while its own cast is open (management_controller.go:2690-2705).

So a durable record addressed to the desk (an `update_coagent` packet) is the
existing mechanism that both injects mid-run and wakes a parked desk. Nothing
injects into a **running cell**.

### 2.4 Research reports to Texture only at cell boundaries [verified, code]

`choir.Report` and `choir.ReportPacket` stage into the cell's tray
(choir.go:936-962, `boundTray` choir.go:839-847). The host reduces the tray
after the cell succeeds (tools_desk.go:390-399, rlm_reduce.go:708-728).

So "report to Texture as each provider arrives" means **one cell per arrival**,
each ending with a `ReportPacket`. That fits a cell-boundary arrival model
exactly.

---

## 3. Gateway fan-out today and what incremental return would take

### 3.1 Path [verified, code]

1. The cell calls `choir.WebSearch(q, n)` (choir.go:453-460). This is a
   synchronous broker call.
2. `StreamBrokerEgress` carries it to the host (sidecar.go:298-303, 320-350).
3. `cfg.Egress` (tools_desk.go:308-327) calls `researchDeps.HostEgress`
   (researchtools/host_egress.go:22-32), which runs `web_search`
   (researchtools.go:658-715). That tool charges the egress budget
   (researchtools.go:680) and floors `max_results` at 40 (researchtools.go:656, 688-691).
4. `search.Client.Search` (search/search.go:89-136) POSTs
   `/provider/v1/search` with `hostlink.NewClient(30s)` (search.go:85).
5. Gateway `HandleSearch` (gateway/handlers.go:856-934) applies the rate
   limit, then `context.WithTimeout(r.Context(), 30s)` (handlers.go:913-915),
   then `SearchClient.Search` → `searchViaPlane` (gateway/search_plane.go:197-216).
6. `searchplane.Router.Search` (searchplane/router.go:64-155) runs.

### 3.2 Router algorithm [verified, code]

- Up to `MaxWaves` waves run (env default 4, config.go:36). Each wave runs
  `ProvidersPerQuery` providers in parallel (default 2, config.go:34), picked
  round-robin from health-eligible providers (router.go:87-125, 195-208).
- Within a wave, `executeWave` (router.go:221-328) runs providers concurrently
  (errgroup) with a per-provider `RequestTimeout` (30 s, config.go:37). It
  **collects per-provider completions one at a time** on a `done` channel
  (router.go:285-306).
- When the merged result count reaches `targetMin` (40, config.go:35), a
  `WaveGrace` timer starts (1.5 s). When it fires, the rest are cancelled as
  "superseded" (router.go:299-305).
- The wave loop stops when merged ≥ target or providers run out
  (router.go:107-124).
- **One merged, deduplicated list** is returned with `Providers`,
  `Attempts`, `Waves`, `ProviderHealth` and a `Degraded` flag. Any failed,
  non-superseded provider sets `Degraded` (router.go:313-326). All-empty
  results produce an `OutageError` (router.go:127-131).

So today the gateway does parallel waves with first-good-enough cut-off and
returns once, grouped. The per-provider arrival point already exists inside
the router (router.go:291-301). It just isn't surfaced.

### 3.3 Transport [verified, code]

- `hostlink` is **plain HTTP over TCP** (hostlink.go:30-32, wraps
  `http.DefaultTransport`). The guest reaches the gateway at
  `http://<hostIP>:8084` set from the kernel param `choir.gateway_url`
  (vmmanager/manager.go:1594, 1626; nix/autoputer-vm.nix:428-429). It is not
  vsock.
- `hostlink.Transport` resends only refused dials (hostlink.go:45-85).
- `AttemptTimeout` bounds an attempt **including reading its body**
  (hostlink.go:87-99). The 30 s search client would therefore cut a long
  stream; a streaming client needs `AttemptTimeout: 0` plus a context bound.
- Streaming over this link already works: the gateway serves SSE inference
  (handlers.go:452-501, `Flush` per chunk), and the guest parses it
  (gatewayruntime/provider.go:251-262). Chunked NDJSON on the search route
  needs no new transport.

### 3.4 Options for incremental per-provider return

| Option | Gateway change | Guest change | Notes |
|---|---|---|---|
| **NDJSON stream** on `/provider/v1/search` (`Accept: application/x-ndjson` or `"stream":true`) | New `Router.Stream(ctx, q, n, onArrival)`. All eligible providers start at once: no waves, no target, no grace. One line per provider completion `{provider, status, latency_ms, results[], error}`, then a final line `{done, providers_total N, providers_answered M, cooling[]}`. Flush per line. Health recording unchanged (router.go:271-279). Handler deadline = provider timeout + ε instead of fixed 30 s. | New `search.Client.Stream` with no per-attempt body bound. | One request, one rate-limit charge (handlers.go:873). Gateway stays the authority on eligibility and health. **Recommended.** |
| Per-provider requests (`providers:["tavily"]` filter) | Small filter in `Router.Search`. | Guest fans out N requests and must learn the eligible set first (`/search/health`). | N rate-limit charges. Eligibility splits across two places. |
| Job id + poll (`/search/jobs`, the design doc's G5) | Job table and TTL at the gateway. | Poll loop. | Durable gateway state for something the guest host can hold in memory. Adds a second place a restart must close. Not needed. |

---

## 4. Design options for async search with Go cells

### Constraint discovered: concurrent `choir.*` egress from cell goroutines is broken today [verified, probe]

`egressRoundTrip` (sidecar.go:325-350) writes a request frame, then calls
`fc.ReadFrame()` with **no request-id correlation and no read lock**. The
transport comment says reads are "single-owner (one reader loop)"
(transport.go:59). The host services egress frames **one at a time, inline**
(desk_session_worker.go:331-334, 414-427).

Probe (`zz_egress_test.go`): four goroutines call egress at once over a real
socketpair against a host `EvalCell`. The first query takes 300 ms; the rest
are instant. Every run produced the same result:

- `a` got `{"echo":"c"}`, `b` got `{"echo":"a"}`, `c` got `{"echo":"b"}`:
  **results cross-wired between callers**;
- every caller waited about 300 ms: head-of-line serialization.

The research prompt invites exactly this pattern: "Use choir.WebSearch and
choir.FetchURL with parallelism appropriate to the task"
(runtimeprompts/overlays/rlm_research_runtime.yaml:25).

Between cells it is worse [inferred]. A background goroutine's egress frame
sits unread until the next `EvalCell`. The worker's session loop and the
goroutine then both block in `ReadFrame`. The loop silently drops a
non-cell frame (session_loop.go:328-329, `default: continue`), and the
goroutine treats a cell frame as "unexpected stream". The likely outcome is a
hung goroutine or a lost cell frame.

A tray verb (`ReportPacket`) called from a goroutine after its cell returned
fails "requires a bound cell" (choir.go:839-847). During a later cell it
lands, unsynchronized, in that cell's tray (choir.go:144-151).

**Consequence:** any design in which **model-authored goroutines** call
`choir.*` network verbs is unsafe until the transport multiplexes by request
id. Concurrency belongs on the host. This is a newly discovered problem.
CLAUDE.md requires documenting it before any fix.

### (a) Handle with results accumulating in interpreter state

Two variants.

- **a1, worker-side.** `search.Start` spawns worker goroutines that call
  egress. This is unsafe today (above). It would need request-id multiplexing
  in `egressRoundTrip` plus a host reader that runs between cells, which is
  a transport rewrite (red-adjacent: it carries every broker verb).
- **a2, host-side handle.** `choir.SearchStart(q)` is a short egress
  round-trip. The host opens the NDJSON stream in a host goroutine,
  accumulates arrivals in a host table keyed by handle id, and returns the
  id immediately. `choir.SearchNext(h, waitMs)` and `choir.SearchDrain(h)`
  are ordinary synchronous egress round-trips that return arrivals since a
  cursor. The interpreter holds only the handle string and whatever the
  model chooses to keep.
  - No model goroutines are needed. Today's serialized transport is fine,
    because every call is short or bounded by `waitMs`.
  - Crash semantics follow the owner rule for free: the table is host
    memory, and a restart ends the table, the desk session and the run
    together (tools_desk.go:44-48). A stale handle after any restart answers
    "unknown handle: ended by restart". Nothing resumes.

### (b) Arrivals delivered between cells by the tool loop

The host appends a one-line notice to every `desk_go_eval` result:
`search h3: tavily +10, parallel failed; 2 of 4 providers answered, 2 pending`.
This goes in the cell map at tools_desk.go:402-414, or as an extra
`injectUserTurns` message.

- **Pros:** the model notices arrivals without polling. It is cheap and
  orange. It matches the existing "chat carries ids, cell carries payload"
  rule (session_loop.go:40-48).
- **Cons:** it fires only when the model runs a cell. If the model ends its
  turn, nothing wakes it. On its own it cannot deliver payloads that the
  model will cite.

### (c) Arrivals as ledger records or packets that wake the desk

Each arrival becomes an `update_coagent` record to the research agent. The
existing injector delivers it mid-run (§2.2), and the park waiter wakes a
passivated desk (§2.3).

- **Pros:** it works across `end_turn`, and it is the existing wake path.
- **Cons:** it writes one durable ledger record per provider per query. That
  is commitment-ledger pressure for transient transport data, and it is red
  (ledger/evidence surface). Arrivals become durable, so a restart would
  find "pending" arrivals for a closed run. Boot would have to close them,
  never deliver them, which is more restart surface. Research is not an
  actor-park desk today (only Texture sets `actor_park_on_idle`), so it
  would also need a passivation policy. Too heavy for transport events.

### (d) Blocking iterator inside one cell

`h := choir.SearchStart(q); a := choir.SearchNext(h, 20000)` blocks until
the **first** arrival (or `waitMs`), then returns. The model reads it, and
its next cell calls `SearchNext` again.

- The probe shows the yaegi side works: closures blocking on `select` with
  `time.After` return at each arrival.
- It is the natural way to "respond as each provider arrives". Each
  `SearchNext` return is one model turn, and each turn can end with a
  `ReportPacket` to Texture, which commits on cell return (§2.4).
- If the model wants several queries in flight, it starts several handles in
  one cell, and `choir.SearchNextAny(hs, waitMs)` returns whichever arrives
  first.

### Yaegi limitations that matter here

- **Goroutines are real** but cannot be cancelled in-process (session.go:235,
  no ctx). Only a process kill stops them, and their output after return is
  lost (§1.4). Keep model goroutines out of the design.
- **No `sync`/`context`** (allowlist.go). `select`, channels and closures work
  (probe).
- **Decl/statement mixing fails.** Ship helpers as `choir.*` verbs, not as
  model-declared types.
- **Reflection cost is negligible.** `recordRefusals` wraps each verb in
  `reflect.MakeFunc` (choir.go:186-206), and verb args go through JSON. Both
  are microseconds against 0.2-23 s provider latency (design doc §2.3).
- **A panic or timeout poisons the session** and drops host handles keyed to
  it. Key handles by session key and release them in `deskSessionWorkers.release`
  and `reclaimLocked` (tools_desk.go:177-243).

---

## 4b. Owner addition: the RLM-native option (arrivals written into REPL state)

The owner's option (call it **R**):

- the search verb returns immediately with only a stub
  (`search s3 started: 4 providers`);
- arrivals are written into persistent REPL state, such as a `searches`
  store whose entries grow as each provider answers;
- nothing about the payload enters a tool result or a message;
- the RLM decides when and how to read the store (`len`, filter, summarize
  in a cell, fan out to sub-RLMs via `choir.Research`).

### Can host code safely inject values into the interpreter between cells? [verified, code + probe]

**There is no API to set an interpreter variable.** `Session` exposes only
`Eval`, `Close`, `ErrorHint` and the internal `showListedNames`
(session.go:147, 344, 361, 316). The only way to write a REPL variable is to
`Eval` generated source. `Eval` is serialized by `Session.mu`
(session.go:150-151), so between cells this is safe from races. But it means
generating, escaping and compiling code. A compile panic or runtime failure
**poisons the session** (session.go:197-205, 253-264) and takes every
user-defined value with it. **Do not inject by Eval.**

**The safe channel is a prebound Go object, not an interpreter variable.**
This is the pattern `choir.Updates()` already uses:

- the host sends a snapshot on the cell frame;
- the `BindCell().Begin` hook copies it into `ChoirScope` fields
  (choir.go:143-151);
- the cell reads it through a method that returns a copy (choir.go:728-739,
  whose comment calls this "the RLM contract puts the payload here — a REPL
  variable — rather than inline in the model's context window").

`End` clears those fields per cell (choir.go:153-166). A search store must
**not** be cleared; it must persist for the life of the session.

Probe `zz_store_test.go`, run with `go test -race`:

- **Mutex-guarded store, exported as a symbol.** A store with
  `Len(id)`/`Get(id)` returning copies was exported as `probe.Searches`. A Go
  goroutine appended arrivals **while a cell was running**. The cell polled
  `probe.Searches.Len("s3")` and saw 2 arrivals mid-cell after 156 ms; a
  later cell saw 3. **The race detector was clean.**
- **Raw Go map, exported as a live variable.** Exported via
  `reflect.ValueOf(&m).Elem()`, the map is live (host writes between cells
  were visible: 1, then 2). Under concurrent writes during a cell the probe
  reported **`DATA RACE` twice, and the test failed under `-race`**. Never
  expose raw maps or slices.
- **Model-declared variables** (`searches := map[...]...` in a cell) cannot
  be written by Go code at all, except by Eval. If the RLM wants its own map,
  it copies from the store.

**Where the writer lives matters.** The search runs in the **agentcore host
process**. The store has to live in the **worker process**, where the
interpreter is. The worker cannot fetch for itself:

- its environment is sanitized to `PATH`/`TMPDIR` plus session vars
  (desk_session_worker.go:126), so it has no gateway URL or token;
- egress is host-mediated by design (choir.go:453-456, "never an open
  socket").

Arrivals therefore cross the socketpair. There are three ways to do that:

1. **Snapshot on each cell frame.** This is safe with today's transport. The
   host adds `Searches []SearchArrival` (arrivals since that session's
   cursor) to `SessionFrame` (session_loop.go:25-49). `Begin` merges them
   into the persistent worker store. The store grows **at every cell
   boundary**.
2. **Explicit pull inside a cell.** `choir.SearchWait(id, ms)` is a normal
   egress round-trip. It blocks until the next arrival or `ms`, merges what
   came into the store, and returns only counts. This is also safe today:
   one synchronous call, the same shape as `WebSearch`.
3. **Host pushes frames between or during cells.** This is **unsafe today.**
   Between cells, the worker's session loop drops unknown streams
   (session_loop.go:328-329). During a cell, `egressRoundTrip` is the only
   reader and would take an arrival frame as "unexpected stream"
   (sidecar.go:339-341). Push needs a dedicated worker-side reader goroutine
   that demultiplexes by stream and request id. That is the same transport
   rewrite §4 calls for, and is not a first slice.

With 1 + 2, the store is written only by the worker's own Go code, under its
mutex. That happens either in `Begin`, before the cell executes, or inside a
`SearchWait` call made by the cell. There is no concurrent writer at all
during a cell unless the cell is the one waiting. That is stronger than the
probe needs.

### Comparison

| Criterion | **R: REPL state + stub** | (b) arrivals pushed into next tool result | (c) records/packets that wake the desk |
|---|---|---|---|
| RLM-nativeness | Highest. Payload is a variable; the model chooses `len`/filter/summarize/fan-out. This is the stated contract of `choir.Updates` (choir.go:728-732). | Low. Payload enters the context window whether useful or not. | Medium. The `update_coagent` path is RLM-shaped (ids in chat, payload in `choir.Updates()`), but each arrival becomes a ledger act. |
| Context cost | One line per cell, counts only (`s3: 3/4 answered, +2 new, 31 results`). | Up to 40 results × providers per cell: the 48 KB value cap (session_loop.go:191) is reached quickly. | Wake turn carries ids only; ledger writes per provider per query. |
| Does the model notice without a push? | **Not reliably with nothing at all.** It would have to poll by `len` in a cell, with no cue for when; a 23 s provider (design doc §2.3) gets missed if the turn ends first. **A one-line count stub on each cell result is needed** (tools_desk.go:402-414 map). The stub is metadata, not payload, which is the same rule as the emit notice line (session_loop.go:44-48). With `SearchWait` the model can also wait on purpose. | Yes. | Yes, including after `end_turn`. |
| Concurrency safety | Safe with frame snapshot + pull: only worker Go code writes, under a mutex, never from a model goroutine. Push during a cell is unsafe until the transport demuxes. | Safe (host builds the string). | Safe (durable store). |
| Persistence across cells and turns | Yes, for the session's life: across cells, across runs of the same desk (PD2, tools_desk.go:42-53), and up to 30 min idle. | Only in transcript text. | Durable. |
| Crash semantics (crash closes work, nothing resumes) | **Correct by construction.** Store, handles and host search goroutines are process memory and die with the restart; the run is closed at boot; nothing to resume or close. A poisoned or timed-out cell also drops the store (new session), so the model is told "new session" (tools_desk.go:146-148). | Correct (transcript is closed with the run). | **Extra work.** Durable pending arrivals outlive the crash; boot must close them, never deliver them; one more surface for the restart rule. |
| Wake after `end_turn` | No. Needs an in-memory, non-durable park, below. | No. | Yes. |

**The remaining gap in R is `end_turn` with searches pending.** The smallest
fix uses the existing `WithParkWaiter` (toolloop.go:943-990). For a research
run whose session has pending searches, the waiter blocks in memory until the
next arrival or until every provider has answered, then returns `Continue`
and injects the same one-line stub. It writes no durable record. A crash
during the wait kills the process, and boot closes the run. [inferred]
`runtime.go:393-397` forbids process-local continuations that outlive a boot.
A park wait inside a live activation does not outlive a boot, but this
should be stated in the mission's standing-questions answers.

---

## 5. Recommendation

**Revised after the owner addition: adopt R (REPL-state store) as the
shape.**

- Arrivals land in a persistent, mutex-guarded worker-side store exposed as
  `choir.Searches` (Len/Get/New/List, all returning copies).
- The store is fed by per-cell frame snapshots plus an explicit blocking
  `choir.SearchWait(id, ms)` pull.
- The only push is a **one-line count stub** on each cell result.
- A non-durable park wait covers `end_turn` while searches are pending.
- The gateway streams per-provider NDJSON.

This keeps a2's host-side concurrency and d's "respond at each arrival",
replaces "payload returned from Next" with "payload lands in REPL state, and
the call returns counts", and drops b's payload push.

The API sketch below is the earlier a2+d form. Under R:

- `SearchStart` returns the id and the stub;
- `SearchWait` replaces `SearchNext` and returns counts only;
- the payload is read with `choir.Searches.Get(id)`.

File list additions for R:

- `session_loop.go` `SessionFrame.Searches`;
- `choir.go` `BindCell` Begin merge into a store that `End` does **not**
  clear, plus the export;
- `tools_desk.go` per-session arrival cursor, frame attachment and stub line
  in the cell map;
- the host search table keyed by desk-session key and released in
  `deskSessionWorkers.release`/`reclaimLocked`;
- the park waiter option in `runtime.go` near 3500.

All of it is orange. The gateway stream is still red.

Earlier form, kept for the file list: host-held search handle with a blocking
per-arrival `Next` (a2 + d), plus a one-line arrival notice on every cell
result (b). Gateway streams per-provider NDJSON. No model goroutines, no
waves, no budgets.

Cell API (research and research-child roles):

```go
h := choir.SearchStart("query", 40)        // returns handle id at once
a := choir.SearchNext(h, 20000)            // blocks to the next provider arrival or 20 s
// a = {provider, status, latency_ms, results[], answered:M, total:N, pending:[...], done:bool}
choir.ReportPacket("texture", packet, "")  // optional progress, commits on cell return
```

The model loop runs one arrival per cell, and each cell may report. Several
handles can be in flight at once. After a crash restart the handles are gone
and nothing resumes, which matches the owner rule. The wording at every layer
is "M of N providers answered". `degraded` stays out of the model's view
(drop it from the projection at researchtools.go:696-711).

### Step 0: problem doc, green (CLAUDE.md "Problem Documentation First")

Document that concurrent `choir.*` egress cross-wires results and serializes
at the host. Cite sidecar.go:325-350, desk_session_worker.go:331-334, this
note's probe, and the prompt line yaml:25. Record the one-hour handle TTL
against persistent sessions (choir.go:120) as a hypothesis. A follow-up
(orange) puts a mutex around write+read in `egressRoundTrip`, or adds
request-id correlation. Until then, tell the prompt to keep `choir.*` verbs
out of `go` statements.

### Minimal first slice: per-provider arrivals end to end. Class: **red** (gateway provider calls), with orange guest parts

1. **Gateway (red)**
   - `internal/gateway/searchplane/router.go`: add `Stream(ctx, q, n,
     onArrival func(Arrival))`. It starts every eligible provider at once,
     emits on each completion (reusing router.go:242-281 per-call logic and
     health recording), and ends with the totals.
   - `internal/gateway/search_plane.go`: add a `streamViaPlane` adapter.
   - `internal/gateway/handlers.go:856-934`: when stream is requested, write
     NDJSON lines and `Flush`; bound by provider timeout + ε rather than a
     fixed 30 s. The blocking path stays unchanged for existing callers.
2. **Guest client (orange)**: `internal/search/search.go` adds
   `Stream(ctx, q, n, onArrival)` using `hostlink.NewClient(0)` (keeping the
   refused-dial retry) and an NDJSON decoder.
3. **Host handle table and verbs (orange)**
   - `internal/researchtools/` (new `search_handles.go`, wired in
     `egressToolTable` at host_egress.go:55-87): add `web_search_start` and
     `web_search_next` with an in-memory table and TTL eviction. No egress
     budget charge.
   - `internal/yaegikernel/broker_protocol.go`: new actions.
   - `broker.go:171-176`: egress dispatch.
   - `choir.go`: `SearchStart`/`SearchNext` methods, exports (~293-330),
     `deskModuleSets` research and research-child (256-263), and the handle
     action scopes (96-114).
4. **Prompt (green/yellow)**: `runtimeprompts/overlays/rlm_research_runtime.yaml:18,25`
   gets the new verbs, the one-arrival-per-cell rhythm and the "M of N"
   wording. It drops "egress budget" and "parallelism" for `WebSearch`.

Deferred: the arrival notice on cell results (b, orange, tools_desk.go:402-414),
`SearchNextAny`, removing waves from the blocking path, and deleting the
egress budget ledger (a separate owner decision).

**Rollback:** the stream mode is additive at the gateway, so reverting the
commit restores the blocking path. Existing `choir.WebSearch` is untouched.

**Acceptance (staging):** a research cell starts one handle and calls
`SearchNext` repeatedly. The test checks that:

- arrivals come back in provider-latency order (keenable about 0.2 s,
  parallel and tavily 2-4 s, chatgpt about 23 s);
- each return shows "M of N";
- no call outlives its `waitMs`;
- the gateway logs one line per attempt.
