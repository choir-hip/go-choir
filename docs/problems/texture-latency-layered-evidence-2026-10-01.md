# Texture Latency — Layered Evidence & Research Map

**Date:** 2026-10-01 · **Status:** diagnosed; remediation proposals open.
**Context:** after commit `243665b4` (edge/PK read paths) and `aea62d05`
(frontend stream/load fixes) deployed, the editor still showed 0.5–13s
latency on `choir.news`. This doc maps every layer of the request path
with measured evidence, then lists research directions ordered by
evidence strength.

## Measured layer budget (user guest `03335285`, commit `f563200e`, during desk drain)

| Layer | Evidence | Cost |
|---|---|---|
| Browser → proxy (choir.news) | TLS+route | ~10–80ms baseline |
| `api.resolve` — proxy→vmctl `ResolveDesktopContext` + `ensureComputerVersionRoute` | staging health counters `avg=1392ms max=20147ms` | **~1.4s typical, up to 20s** — two vmctl RPCs per request, no cache |
| `api.upstream` — proxy→guest HTTP | staging health `avg=5110ms max=46575ms` | **~5s typical** during guest load |
| Guest handler + embedded-Dolt read | guest `/health` direct = 7ms idle; **1–3.7s under drain** | **0–3.7s**, load-dependent |
| Mutation/apply work sharing the guest | `desk_pending_mutations` 13→5→37, `running_runs` 0→7 | the drain *is* the load that stalls reads |

**Sum:** a quiet-system request ≈ resolve(1.4s) + upstream(≈idle) + guest(≈10ms) ≈ **1.4s floor**. Under desk load the same request hits 10s+ because both proxy-upstream and guest-CPU contention add seconds each.

## What `243665b4` actually fixed (and didn't)

- Eliminated the per-request `JSON_EXTRACT` full-kind scans → each individual
  guest read is now µs-scale when the engine is free.
- **Did not fix:** reads still serialize on the shared `engineMu` against
  *all* concurrent writes and other reads; `ReadObjectSnapshot` still scans
  every `og_objects` row for the whole computer per call; the proxy still
  pays 2 uncached vmctl RPCs per request; a busy guest (provider turns,
  projection appends, boot work) still stalls everything.

## Divergent-consensus hypotheses → adjudicated

Panel run `.agentic-consensus/agentic-consensus-20261001-074717`
(codex, claude-opus, gpt-6-sol, glm-5.3, devin + 2 OMP models; 6 runners
failed/skipped). Clustered into families; findings verified in code:

### Family A — frontend state machine (VERIFIED, fixed in `aea62d05`)
- `loadRevisionAt` no try/catch, no generation guard → wedged nav / stale
  overwrite. **Fixed.**
- Initial `connect(0)` SSE failure → closed=true + listeners removed →
  stream permanently dead until reload. **Fixed.**
- `onEvent` fired `getLifecycleSnapshot` uncoalesced per event → burst
  stampede of full-computer snapshots. **Fixed (coalesced to ≤2 reads).**
- Editor clears only exact-string banner errors on restore — residual
  cosmetic, documented not blocking.

### Family B — guest store still scans whole computer (VERIFIED, open)
- `ReadObjectSnapshot(owner, computer)` runs `SELECT … WHERE owner_id=? AND
  computer_id=?` over **all** `og_objects`, serializable TX, under
  `engineMu`. Called by `GetLifecycleSnapshot` per SSE refresh, per editor
  tab, per event. Largest remaining read cost. **Open — highest-leverage
  next fix.**
- `listTextureSourceEntitiesForRevision` lists all entities for the scope
  then filters in memory (indexed, but still whole-scope). Secondary.

### Family C — proxy resolution is uncached (VERIFIED, open)
- `resolveComputerURL` does `ensureComputerVersionRoute` (vmctl RPC) +
  `ResolveDesktopContext` (vmctl RPC) **per request**, with retry window.
  `api.resolve avg 1392ms`. **Open — cheapest win: TTL cache.**

### Family D — reads share one mutex with everything (VERIFIED, open)
- `ogReadStore.ShareEngineMutex(ogStore)` → reads queue behind every
  write, snapshot, and provider-adjacent append. Under a desk drain
  (observed 37 pending / 7 runs) even `/health` took 1–3.7s on the LAN.
  **Open — the architectural question.**

### Family E — transport/proxy path issues (speculative, low evidence)
- Proxy SSE buffering/timeout, browser per-tab connection caps, auth
  renewal (`fetchWithRenewal` shared-promise) serializing behind a hung
  renewal. **Unverified — needs a HAR + proxy access log correlation.**

### Family F — VM lifecycle vs. load (observed)
- The user's VM respawned mid-probe (IP 10.200.97.2 → 10.200.101.2, guest
  commit bumped `0e9946a6`→`f563200e`). Requests during respawn show
  multi-second variability. Explains intermittent bursts, not steady state.

## Research directions, ordered by expected payoff

1. **Scope `ReadObjectSnapshot` to the trajectory.** It already filters
   in-memory by `trajectoryID`; add a `trajectory_id` metadata/edge index
   so the query returns only that trajectory's objects. Single biggest
   guest read cost; benefits every lifecycle endpoint and every SSE tick.
   (Panel: gpt-6-sol #1/#2, codex #6.)

2. **Cache the proxy→vmctl route resolution.** Per-(user,desktop) TTL of
   a few seconds; invalidate on route-change events if vmctl emits them.
   Removes ~1.4s from every authenticated call. Low risk, boring design.
   (Panel: claude H2/H3.)

3. **Decouple read from write engine.** Investigate whether `ogReadStore`
   can drop `ShareEngineMutex` and run as a separate read connection /
   read replica Dolt session. If embedded Dolt supports concurrent
   read-only sessions, reads stop queueing behind mutation writes.
   Requires Dolt-capability verification first; the shared mutex may be
   load-bearing for correctness. (Panel: gpt-6-sol #4, glm H5.)

4. **Bound the snapshot refresh rate server-side.** Even with the frontend
   coalesce, the SSE tick + multiple tabs still produce N× full snapshots.
   Consider a per-(trajectory,client) ETag/`snapshot_cursor` short-circuit
   that returns 304 when unchanged, or a projection cache keyed on the
   last durable head. (Panel: gpt-6-sol #2, glm H3/H10.)

5. **Instrument `engineMu` wait vs. hold + provider-call segments.**
   Cheapest discriminating probe per the panel: log
   `op, wait_ms, hold_ms` around the mutex and around `intercept`
   (projection append / network CAS). Turns 'slow' into a measured
   breakdown instead of a guess. (Panel: codex "single probe first".)

6. **Frontend: pending indicator + dead-revision retry.** `loadRevisionAt`
   now guards, but a transient failure still requires a re-click. Cheap:
   auto-retry once, and show a spinner while a revision fetch is in
   flight. (Panel: devin, glm H2/H12.)

## What was ruled out

- The guest binary itself — it's on current code; direct `/health` was
  7ms when idle.
- The store's per-query cost for the *fixed* paths — edge/PK lookups are
  now index-shaped; residual cost is `ReadObjectSnapshot` + mutex
  contention, not the repaired scans.
- A deterministic frontend wedge — the three UI bugs above shipped fixes
  in `aea62d05`; remaining frontend cost is server-bound.

## Next probe to run first

On the user's guest during a desk drain, time a `GET /health` (no store)
against a `GET /api/texture/revisions/{id}` (store) **while** logging
`engineMu` wait/hold around both. That splits "guest CPU saturated" from
"read queued behind a long store operation" in one observation, and it
directly adjudicates direction 3 (separate read engine) versus 1+4
(reduce the snapshot's scope and frequency).
