# Orientation — Jev/Supervision Metamission + the First Real QA Failure

2026-09-29. Written post-M11-landing. This is the understanding pass the owner
asked for before chartering the next metamission: three threads — the live
QA failure evidence, the Jev-in-management design (owner steer), and the
metamission skeleton.

---

## A. What actually happened in the QA failure (evidence, not conjecture)

Owner's prompt-bar session on `computer-03335285269bdba4f94377e56879f9e6`
(owner `5bd6de97-3b58-408c-bf89-c42c81b083de`), ~22:16–22:27Z 2026-09-29:

| t (UTC) | Event |
|---|---|
| 22:16:50 | `trajectory_started` — prompt-bar turn on doc `268ef2d0` ("What's new in ai today") |
| 22:17:04 | `conductor` run completed — `open_app` → texture doc |
| 22:18:08 | `texture_turn_committed` + `work_opened` + `control_queued` — v1 (appagent, "pre-search") committed; work item opened to `research:` desk |
| 22:18:53 | texture run `9b5ddc61` **passivated** (7.3k input tokens) |
| 22:19:09 | `control_delivered` to research desk — **last trajectory event** (reducer_seq 5) |
| 22:19–22:26 | research run `84af5a50` executes (123,490 input tokens, ~7.7 min, `reasoning=low`, model gpt-5.6-luna via chatgpt) |
| 22:26:50 | research run completed — result: *"Research remains open. Initial evidence was staged, but primary-source verification is incomplete"* |
| 22:26:50+ | **silence** — no `texture_turn_committed` for v2; no further trajectory events; reducer still at seq 5 |

What the owner saw: slow texture open → blank page → slow v0 → slow v1
pre-search → "lifecycle stream disconnected" → no v2.

### The defects this exposes (evidence-backed)

1. **Research completed without producing a turn.** The research desk ended
   its cell "remains open" — staged evidence, no resolve, no report — and
   nothing re-woke texture to render v2. This is the same failure class the
   9/29 clustering assessment fixed at the *assignment* layer; the
   trajectory/desk-consumption layer still has an open edge: an `open` work
   item + a completed desk run + no committed turn = silent stall.

2. **"Lifecycle stream disconnected"** is the frontend SSE on
   `/api/trajectories/{id}/stream`. The handler sends `: heartbeat` every
   15s (`api_trajectory.go:162`), so app-side idle timeout is not the cause.
   Candidates: proxy hop (`choir.news` → VM) recycling the SSE connection,
   or the browser's EventSource `onerror` on a transient drop. The stream
   delivers `replay_required` for cursor expiry — a clean reconnect path —
   but the UI currently surfaces the disconnect as terminal-looking rather
   than auto-resubscribing. **Frontend should treat post-snapshot `onerror`
   as reconnectable**: EventSource auto-reconnects natively; the handler
   discards that fact and calls it an error.

3. **Desk wake debt predates the QA.** `/health` shows
   `desk_pending_mutations: 7`, and three runs are `pending` since
   2026-09-28 (`management:b05f42a6`, `texture:e98f8f3f`, `texture:43ad448a`)
   — none have moved in 24h. The VM runs `b85af274` (9/28 build), predating
   today's reconcile-kick and edge-closure fixes — the stall may partly be
   the known wake-gap those repairs addressed. A redeploy + restart of this
   computer's desks is the first cheap experiment.

4. **Latency is architectural, not a tuning bug.** A research turn costs
   ~120k input tokens and ~8 minutes end-to-end (prompt→conductor→texture
   v1→research→…). "10 versions in 5 minutes" is a different architecture:
   parallel casts, streaming partial turns, or fast-path summaries — not a
   faster model.

---

## B. Jev in management — the owner steer vs the 9/27 dialectic

**Owner direction 2026-09-29:** Jev integrates into management — management
calls Jev for decisions, hitting the RLM only on low confidence, with the Jev
output exposed as a variable in the RLM context. *"Also: conductor as
router"* — Jev's `choice` primitive is the route-selection mechanism.

This steers past the 9/27 dialectic's runtime-only scorer position (B:
cell-actuated won over A: hidden outbox). The reconciled shape:

- **Two Jev surfaces, not one.** (i) *Scorer for commitment records* —
  async, runtime-driven, supervision-only, never in acting context (the
  epistemic boundary). (ii) *Decision gate inside management's cell* —
  synchronous `jev.decide(state, questions)` as an in-cell module verb;
  on low confidence/probability, management escalates to a full RLM
  sub-cast with the Jev distribution passed in as a context variable.
  These are different authorities and different latency budgets; they share
  only the transport.

- **Epistemic boundary is preserved by kind, not by hiding.** Jev scores of
  *commitments* stay out of ActingPack (that boundary holds). Jev
  *judgments management calls for its own routing* are inputs the desk
  chose to consult — like any tool result — and may enter its RLM context.
  The boundary binds score records of others' commitments, not a desk's
  own tool calls.

- **Conductor routing via `choice`.** Model/provider/route selection is a
  `choice` question to Jev; the returned option + distribution routes the
  cast. This is where "model config via RLM code" lands — management picks
  the model per task through Jev, not a static `model-policy.toml`.

- **What the 9/27 panels still govern** (ratified): pinned `typesafe/jev-1.13`
  (never `jev-latest`); OpenRouter `/api/alpha/decisions` transport (owner
  §12.2, supersedes the direct `/v1/systemone` the panels assumed); gateway
  `POST /provider/v1/judgments` per-VM bearer + own rate bucket; scores land
  as `choir.commitment_score` (distinct OG kind); full distributions in the
  record, not flattened answers; `confidence` never gates; replay =
  deterministic score ID bound to the resolution.

- **Blocking prerequisite both panels flagged:** `Precommit` commits only a
  hypothesis string; `Resolve` only a verdict word. Scoreable input needs
  typed questions + frozen probabilities at precommit and evidence refs at
  resolve — plus the `Disagreement` split so scorer disagreement never
  reaches ActingPack.

---

## C. Metamission skeleton — constituent /goals in sequence

Ordered by dependency; each is a separate throughline `/goal` file:

| # | Station | Delivers | Depends |
|---|---|---|---|
| J0 | **Research-prompt debug** — the QA stall | Diagnosis of desk wake debt + research-no-turn + SSE reconnect; timing records per desk turn. The "10 versions in 5 minutes" bar lives here. | None — first. |
| J1 | **Scoreable commitments** | Precommit freezes typed questions + probabilities; Resolve carries evidence refs; `Disagreement` split (resolver-verdict vs scorer). | J0's findings may reshape this. |
| J2 | **Jev transport** | Gateway `POST /provider/v1/judgments` → OpenRouter `typesafe/jev-1.13`; per-VM bearer + own rate bucket; OpenRouter key via `deploy-provider-creds.sh`. | None (can parallel J0/J1). |
| J3 | **Jev-in-management** | `jev.decide` module verb in the management desk; low-confidence → RLM sub-cast with the distribution as context variable; conductor `choice`-routing for model/provider. | J1+J2. |
| J4 | **RLM-based model switching** | Model/provider/route selection moves from `model-policy.toml` into desk-chosen config; evals run as parallel RLM casts; Jev scores eval outputs. | J3. |
| J5 | **Timing evals** | Per-turn latency + cost records as commitment data; research/selfdev timing evals run through the RLM eval surface. | J4. |
| — | **World Wire** | Resumes on these foundations. | All above. |

### Open decisions for the owner

1. **J0 scope**: fix-forward on the QA stall (redeploy the owner's computer
   onto today's build first — the cheapest experiment) vs. the full timing/
   latency instrumentation mission.
2. **Does J3 land commitment-scoring AND management-gating together**, or
   scorer-async first (lower risk) with the cell-actuated gate second?
3. **Conductor-as-router**: replace model-policy.toml wholesale (J4) or
   Jev-choice overrides a static default first (incremental)?
