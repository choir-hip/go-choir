# Choir Autopaper Activation — Attempt Report (2026-07-11)

## What this document is

A post-mortem investigation of the ~12th failed attempt to activate Autopaper
end-to-end, executed as a 12-hour autonomous run against
`docs/definitions/choir-autopaper-activation-2026-07-10.md` (2026-07-10
~17:30Z through 2026-07-11 ~06:40Z, 50 commits on
`claude/choir-autopaper-activation-debug-ck3dxd`). This report does **not**
propose or apply fixes. Its job is to name the incorrect cornerstones — the
architectural assumptions that make every attempt fail regardless of how
competent the executing agent is — and to evaluate the operator's three
hypotheses about why.

Primary evidence: the definition document's own evidence ledger and run
checkpoint (which the run maintained meticulously), the commit history of the
attempt branch, and direct source inspection cited inline.

## What the 12 hours actually bought

The run was not aimless. It executed the receding-horizon loop faithfully and
peeled a real failure onion, one layer per deploy cycle:

1. Guest reboot loop → localized to `backfillOGFromSQL` pre-listen stall.
2. Per-record legacy `runs` replay on every boot → resumable migration.
3. Migration blocking listener startup → listener-first ordering.
4. Stale concurrent VM ensure killing a healthy newer generation → per-VM
   lifecycle serialization.
5. Deploy verifier conflating workflow SHA with sandbox artifact SHA →
   artifact-aware verification.
6. Sourcecycled reusing a completed run receipt from an older cycle → status
   reconciliation through the proxy.
7. Dangling Universal Wire edition alias → alias bootstrap repair.
8. Reconciler dispatched with no cycle lineage → lineage-carrying dispatch.
9. Sourcecycled `MemoryStore` losing the entire queue on restart → durable
   dispatch state.
10. Event migration monopolizing the store's one connection → three
    consecutive scheduling workarounds (bounded scans, yields, one-event
    steps).
11. Blocked/passivated/completed-with-live-trajectory processor states
    freezing all admission → capacity release + cycle-fair ordering.
12. Reconciler completing "successfully" while unable to read its inputs, and
    later while its one mandatory canonical write was silently cancelled by
    same-channel rewarm deduplication → run ended mid-repair here.

Along the way `/api/universal-wire/stories` **did** return real stories several
times (edition `3b9cdc8b` with two CDC docs at ~04:47Z; edition revision
`0ad9f2d9` at ~07:1xZ; edition revision `3c96faec` at ~09:30Z) — but only in
narrow windows, observed by curl diagnostics, and each window was destroyed by
the next fix's deploy. The mission's own completion item 5 (stories produced by
the **reconciler**) was never true.

Score for the window: ~23 code commits, of which **18 were substrate**
(store/migration/startup: 9; VM lifecycle/deploy/CI verifier: 4; sourcecycled
dispatch ledger: 5) and only **5 touched the editorial product path**. At
least six deploy attempts failed for verifier/substrate reasons unrelated to
the change being deployed. The run spent roughly three-quarters of its budget
proving the computer, not the newspaper.

## The incorrect cornerstones

These are the load-bearing assumptions that were wrong before the run started.
Every symptom in the ledger traces to one of them. They are ordered by how
much of the 12 hours each one consumed.

### C1. A single-connection embedded Dolt is the entire platform data plane

`internal/store/texture.go:437` (`configureEmbeddedDoltDB`) pins the embedded
Dolt handle to `SetMaxOpenConns(1)`. Runtime health, objectgraph reads,
Texture writes, processor admission counting, and legacy migration all
serialize on that one connection. Consequences observed directly:

- Any long-running store work makes `/health` time out, which vmctl reads as
  guest death (C3), which triggers recovery, which restarts the work.
- Four consecutive commits (`d376169`, `c4320c8`, `774b272`, `949342e`) were
  attempts to *schedule around* the single connection — bounded scans,
  `Gosched`/sleep yields, one-event steps, post-step delays. The ledger itself
  concludes: "voluntary sleep is not an availability contract." Correct — and
  neither is any cooperative-yield scheme on a 1-connection pool. This is not
  a tuning problem; a data plane that cannot serve a health probe concurrently
  with background work cannot host an always-on platform, full stop.

### C2. Boot-time legacy migration lives inside the serving process and gates readiness

Every fresh guest replays or re-verifies the relational→objectgraph backfill
before (originally) or while (after the run's repairs) serving traffic, under
a 3-minute vmctl readiness deadline that is unrelated to migration size. The
original reboot loop (17 Firecracker boots, 16 readiness timeouts in one
hour) was exactly this: unbounded startup work under a fixed liveness
deadline, with each kill restarting the work from scratch. The run made
migration resumable and deferred — genuinely good repairs — but the
cornerstone error remains: **data migration is coupled to process boot and to
the serving path's readiness**, so every deploy and every recovery re-enters
the contended-migration regime (and, via C1, degrades the product while doing
so).

### C3. "Slow" is treated as "dead", and reads have lifecycle side effects

Three instances of the same conflation:

- vmctl health-checks the guest every 15s and kill-recovers on failure, even
  when Firecracker is alive at 225% CPU doing legitimate work.
- The deploy verifier probed identity with `curl --max-time 2` for a 60-second
  window, shorter than observed store-backed guest startup — so **correct
  deploys were recorded as failures** (at least jobs 86471236444, 86512939750,
  86517633591, 86518660425, 86524844627, and the 62742ee attempt), and each
  rerun refreshed the guest again, guaranteeing non-convergence.
- Any request through `HandleSandboxProxy`/`HandleResolve` can trigger
  `EnsureUniversalWirePlatformComputer` — i.e., a read can boot or recover a
  VM. The `cb694846` observation showed the lethal form: a stale concurrent
  ensure hit its deadline and **killed a newer, healthy, already-listening
  generation** of the same VM.

Together C1+C2+C3 form a self-amplifying loop: busy store → failed probe →
kill → reboot → restart migration → busier store. This loop, not any single
bug, is the "reboot loop" the mission was chartered against.

### C4. Processor lifecycle authority is split five ways, and dedup means "at most once ever"

The run's own `root_cause_clustering_assessment` names this precisely: run
state, trajectory state, processor-resolution state, the sourcecycled durable
request ledger, and the runtime active-run counter are five projections with
no shared authority. Each pairwise disagreement froze the pipeline in a
distinct way during the window:

- `blocked` (non-terminal in runtime) had no sourcecycled continuation path →
  one provider 429 froze all admission indefinitely.
- A runtime refresh passivated a live trajectory (51 open work items) →
  sourcecycled counted it in-flight forever.
- `run state=completed` coexisted with a live unresolved trajectory →
  sourcecycled released capacity while runtime admission still counted an
  active processor and 429'd every new submission.

Compounding it, per-cycle deduplication treats **any** prior run — including a
terminal *failed* run that never reached tool iteration zero (reconciler
`e289af46`, DeepSeek circuit open) — as the cycle's one authoritative
activation. Idempotency ("don't run twice") has been implemented as
at-most-once-ever ("a transient upstream failure permanently burns the
cycle"). A news pipeline whose unit of work is unretryable by design cannot
be reliable on top of fallible model providers.

### C5. Agent completion is narrative, not artifact-verified

Two reconcilers "completed without crash or OOM" and were counted as
successes by the harness while doing no editorial work at all:

- `7aba21d6` received opaque doc/revision handles its tools could not
  resolve, searched them as literal corpus queries, found nothing, and
  completed.
- `aabf0e75` issued its mandatory existing-document revision request; the
  runtime allocated two duplicate same-channel Texture rewarms, cancelled
  both, no canonical revision was written — and the parent reconciler still
  **completed and narrated success**.

The cornerstone error: run completion authority is the agent's self-report
plus absence of crash, rather than verification that the required artifact
(a reconciler-descended canonical revision) exists. Any model — GPT, Claude,
or otherwise — will "succeed" vacuously under this contract.

### C6. Publication is never decoupled from the live substrate

`/api/universal-wire/stories` is an authenticated route that resolves, **per
request**, through proxy → `resolvePlatformTarget` → vmctl → the platform
guest's runtime → the single-connection Dolt store
(`internal/proxy/handlers.go:976-985`; `frontend/src/lib/UniversalWireApp.svelte:70`).
There is no durable published edition artifact — no static feed, no cache, no
snapshot — that survives independently of platform health. "The newspaper is
published" is therefore only true at instants when auth, proxy, vmctl, the
guest, the runtime, and the store are *simultaneously* healthy. This is the
direct mechanism behind the operator's observation that articles never
appeared in the Universal Wire app even though diagnostics saw them: every
success window was measured by curl during a lull, and every subsequent
fix-deploy refreshed the guest, restarted migration, passivated live runs,
and closed the window before a human ever loaded the app.

### C7. The mission definition itself front-loads the hardest architecture as acceptance criteria

Completion item 5 requires stories whose canonical Texture docs were produced
**by the reconciler** — a two-agent editorial pipeline (processor writes,
grounded reconciler reviews and rewrites through same-channel actor rewarm) —
before a one-agent path has ever been stable for a single day. The last ~6
hours of the run were spent on reconciler grounding, mandatory-revision
enforcement, and rewarm-cancellation semantics, while processor-produced
stories were already reaching the endpoint. The definition, not the agent,
drove that: the agent "going off track into reconciler stuff" was faithful
execution of an acceptance criterion that is itself cart-before-horse within
the mission.

## Evaluation of the operator's hypotheses

### H-A: "Autoputer with self-development first, then Autopaper" — SUPPORTED

This is the strongest hypothesis and the evidence is quantitative:

- 18 of 23 code commits and ≥6 failed deploys were spent making the
  *computer* (store, migration, VM lifecycle, deploy verification) survive
  its own product, not making news.
- The mission's completion semantics literally embed autoputer acceptance
  criteria ("platform computer stays stable for a full cycle", "sourcecycled
  does not restart") as prerequisites inside a product mission. Twelve
  attempts have each re-discovered, at product-mission prices (full
  CI + deploy + staging observation per probe, ~30–60 min per iteration),
  substrate defects that an autoputer-focused mission would surface in
  minutes with direct instrumentation.
- The repo already knew this. `specs/autoputer_lifecycle.tla:19` cites
  `docs/mission-autoputer-before-autopaper-v0.md` — the canonical-sequence
  doctrine **existed and has been deleted** from `docs/`. The sequence was
  written down and then shortcut, exactly as the operator suspects.
- The self-development loop itself is part of the unmet prerequisite: the
  deploy verifier (C3) failed correct deploys repeatedly, so even the
  *repair* loop was unreliable. An agent cannot converge on a product when
  the act of shipping a fix is itself a coin flip that destroys the state
  under test.

### H-B: "What is the mechanism why the news system isn't working at all?" — ANSWERED

There is no single mechanism; that is the finding. The pipeline is a serial
chain of roughly ten seams (source poll → projection → handoff → proxy →
VM lifecycle → runtime admission → processor run → Texture write → edition
publication → authenticated read), end-to-end success requires all of them
healthy at once (C6), several of them share one fate (C1/C3), and the repair
loop resets them (every deploy refreshes the guest, restarts migration,
passivates runs, and — before `c508ab9` — erased the dispatch queue). The
system has **no stable resting state**: it does not accumulate publication;
it re-earns the entire chain from scratch continuously. Under that
architecture, "works in a diagnostic window" and "never works when a human
looks" are the same behavior. The pipeline did produce and expose real
stories at least three times in the window — the news system is not
0%-functional; it is 0%-*retentive*.

### H-C: "Codex/GPT is trained against synthesizing non-authoritative sources" — NOT SUPPORTED

The ledger contains no content refusal, hedged synthesis, or
editorial-reluctance failure anywhere in the window. Processors repeatedly and
willingly synthesized news from RSS/Telegram sources: canonical CDC story
documents `aad3f0d2`/`fe6518d2`, docs `9d824cd2`/`3f70e054`,
`3d175eb4`/`091f31b4`, `4d1bc12a`, each with revisions and publication refs.
Every terminal failure is mechanical: readiness timeout, connection-pool
starvation, provider circuit open, HTTP 429, duplicate-rewarm cancellation,
capacity-accounting deadlock. The one behavior that superficially resembles
model timidity — reconcilers completing without editing — is fully explained
by C5 (unresolvable opaque handles, then a cancelled mandatory write) and
would afflict any model under the same contract. Switching model vendors
would change nothing; fixing completion authority would fix it for all
vendors. This hypothesis should be retired so it stops absorbing attention
across attempts.

## Why attempt 13 will also fail if run the same way

Any future attempt that starts from "activate Autopaper" will re-enter the
same funnel: it must first stabilize C1–C4 to get a proof window, will spend
its budget there, and any window it wins will close on the next deploy
because of C6. The failure is invariant to agent quality — this run's agent
was rigorous (its evidence ledger is the best artifact of the attempt) and
still consumed 12 hours converting substrate debt into documentation at a
rate of roughly one layer per deploy cycle.

The cornerstones that must be re-laid first, in dependency order (named, not
designed, per this report's scope):

1. **A data plane with real concurrency** (or a store-level foreground/
   background priority contract that health checks can verify) — replaces the
   C1 scheduling workarounds.
2. **Migration decoupled from boot and from readiness** — one-time, offline
   or job-scoped, with durable completion authority; a guest boot must be
   cheap and bounded.
3. **Lifecycle that distinguishes slow from dead and separates reads from
   recovery** — probe budgets derived from measured startup, generation-
   guarded recovery, no boot side effects on read paths; this is the
   autoputer mission proper, including a deploy/verify loop that can be
   trusted, i.e. self-development.
4. **One processor lifecycle authority** with retry semantics that
   distinguish "already succeeded" from "failed before starting".
5. **Artifact-verified agent completion** — a run is complete when its
   required artifact exists, not when it says so.
6. **Durable publication** — an edition, once produced, is servable without
   the live substrate being healthy.

Only after 1–3 (autoputer + self-development) does the Autopaper mission
shrink to what it was always supposed to be: 4–6 plus prompts. That is the
canonical sequence the deleted doctrine encoded, and the twelve attempts are
the empirical cost of shortcutting it.
