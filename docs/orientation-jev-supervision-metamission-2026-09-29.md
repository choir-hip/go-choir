# Orientation — Jev/Supervision Metamission + the First Real QA Failure

2026-09-29. Written post-M11-landing. This is the understanding pass the owner
asked for before chartering the next metamission: three threads — the live
QA failure evidence, the Jev-in-management design (owner steer), and the
metamission skeleton.

---

## A. What actually happened in the QA failure (evidence, not conjecture)

Owner's prompt-bar session on `computer-03335285269bdba4f94377e56879f9e6`
(owner `5bd6de97-3b58-408c-bf89-c42c81b083de`), ~22:16–22:27Z 2026-09-29:

| t (UTC) | Event | Δ |
|---|---|---|
| 22:16:50 | `trajectory_started` — prompt-bar turn on doc `268ef2d0` ("What's new in ai today") | — |
| 22:17:04 | `conductor` run completed — `open_app` → texture doc | **+14s** (routing) |
| 22:18:08 | `texture_turn_committed` + `work_opened` + `control_queued` — v1 (appagent, pre-search) committed; work item opened to `research:` | **+64s** (texture turn 1) |
| 22:18:53 | texture run `9b5ddc61` passivated (7.3k input tokens) | — |
| 22:19:09 | `control_delivered` to research — **last trajectory event** (reducer_seq 5) | +16s delivery |
| 22:19–22:26 | research run `84af5a50`: **123,490 input tokens**, ~5–9 sequential `stream=false` provider calls, `reasoning=low`, `gpt-5.6-luna` via chatgpt | **~7.7 min** |
| 22:26:50 | research completed — "Research remains open. Initial evidence was staged, but primary-source verification is incomplete" | — |
| 22:26:50+ | **silence** — no `texture_turn_committed` for v2; no report-back wake; reducer still at seq 5 | stall |

### Per-leg diagnosis

1. **Prompt→conductor (14s)**: the target is <1s+app-open — the gap is VM
   wake + conductor call, not model latency.

2. **Texture v1 (64s)**: one texture cell — prompt-build + `desk_go_eval` +
   `ApplyTexture` commit. Not model-bound; the whole cell is serial.
   Fast-model swap helps proportionally, not structurally.

3. **Research 470s — the real defect.** `stream=false`, `reasoning=low`,
   and a serial tool loop where each `tool_use` call re-sends the full
   message history: call N carries N-1 tool results. 123k input tokens is
   context compounding, not work volume. The fix is not a faster model —
   it's research authoring a **Go cell** that loops search→emit-evidence→
   deepen inside one `desk_go_eval` call (the owner's design: texture
   precommits and delegates in one cell; researchers send evidence back as
   they go; texture revises per update and may re-delegate).

4. **No v2 — the missing report-back leg.** Research staged evidence but
   never delivered a report that re-woke texture. Two compounding causes:
   (a) research never called `choir.Report`/`ReportPacket` — it treats
   channel delivery as optional; (b) even if it had, tray intents commit at
   cell `End` — evidence can't reach texture until the cell finishes, so
   "send evidence + keep searching" can't stream today. Plus the trajectory
   reducer has been at seq 5 since `control_delivered`: an `open` work item
   + a completed desk run + no committed turn = silent stall — the
   assignment-layer gap the 9/29 clustering assessment fixed is still open
   at the desk-consumption layer.

5. **"Lifecycle stream disconnected"** = frontend SSE on
   `/api/trajectories/{id}/stream` dropped post-snapshot. The handler sends
   `: heartbeat` every 15s (`api_trajectory.go:162`) — not app-side idle.
   Candidates: proxy hop (`choir.news` → VM) recycling the SSE connection,
   or browser EventSource `onerror` on a transient drop. The stream delivers
   `replay_required` for cursor expiry — a clean reconnect path — but the
   UI surfaces a reconnectable drop as terminal-looking instead of
   auto-resubscribing.

6. **Desk wake debt predates the QA.** `/health` shows
   `desk_pending_mutations: 7`, and three runs `pending` since 2026-09-28
   (`management:b05f42a6`, `texture:e98f8f3f`, `texture:43ad448a`). The VM
   runs `b85af274` (9/28 build), predating today's reconcile-kick and
   edge-closure fixes — the stall may partly be the known wake-gap.
   Redeploy + restart is the first cheap experiment.

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

## C. Metamission skeleton — consensus-adjudicated order

Reviewed by a 5-agent convergent panel (codex, gpt6-sol, glm53-flash,
gemini38, devin; 2026-09-29, `.agentic-consensus/agentic-consensus-20260929-195929/`).
Verdict: skeleton sound, but M0a hid a red-class kernel station, the silent
stall had no owner, and management's RLM-ification was missing. Adjudicated
order (each a separate `/goal` file):

| # | Station | Delivers | Depends |
|---|---|---|---|
| M0 | **Debug + stabilize** | Redeploy owner computer onto today's build; reconcile the 7 pending mutations + 3 stale pending runs (counter → 0, not masked by the idle-sweep busy-check); SSE auto-resubscribe as a named fix (~20 lines, `frontend/src/lib/lifecycle.js` — `onerror` resumes via `?after=` cursor instead of terminal error); baseline per-leg timings; problem record. *No throughput bar here.* | none — first |
| M-SUB | **Duplex cell IPC + eager emission + stall terminator** | (red-class kernel) Eager channel delivery mid-cell: evidence reaches texture while the cell still runs; ledger/commitment stays staged-atomic at cell end. Mid-cell inbox refresh so texture follow-ups reach a live research cell. Emission-divergence contract (cell emits then crashes → texture holds a delivered non-ledger message: rule stated). **Desk-consumption liveness**: every completed cell emits a terminal report-or-failure event that advances the reducer — the silent-stall gap gets an owner. Shared by research, texture follow-ups, management sub-cast, conductor routing. | M0 |
| M0a-1 | **In-cell research verbs** | `choir.WebSearch`/`FetchURL`/evidence-ops as Go-callable verbs; egress budget rehomed onto the cell substrate (the typed tools carry per-activation egress + 8GiB cap — deleting them without rehoming deletes governance). Typed surface still live — verify stall fix in controlled comparison. | M-SUB |
| M0a-2 | **Research tool-surface deletion** | Delete the 14-tool surface; capability-parity checklist as acceptance (each deleted capability reachable via `choir.*` or explicitly dropped with reason); prompt-overlay rewrites (`rlm_research_runtime.yaml`, `promptstore/defaults/research.yaml` instruct the deleted cadence — must ship in the same commit or it's an instant regression). | M0a-1 |
| M1 | **Scoreable commitments** | `Precommit` freezes typed questions + probabilities; `Resolve` carries evidence refs; `Disagreement` split; string-commitment grandfathering stated. | parallel-safe with M-SUB→M0a — types/store change |
| M2 | **Model/policy eval surface + instrumentation** | Model/provider/effort desk-chosen; evals as parallel RLM casts (fan-out size, owning desk, fixture location, score-matrix artifact all named); per-turn timing/token/cost records (folds most of old M6); eval fixture/golden set for the QA prompt family. Commitment-backed evals need M1; raw instrumentation doesn't. | M0a-2 for research-specific validity; M1 for scored evals |
| M3 | **Research hill-climbing** | Multi-model/effort matrix on the QA family. **Owns the 10-in-5 bar**: ≥10 distinct committed texture revisions within 300s of prompt submission end-to-end, each revision attributable to a research emission; per-leg timings are diagnostics, not the bar. | M2 |
| M4 | **Jev transport** | Gateway `POST /provider/v1/judgments` → OpenRouter `typesafe/jev-1.13` pinned; per-VM bearer + rate bucket; alpha-endpoint credential provisioning named. | none — parallel throughout |
| M5a | **Async commitment scorer** | Reconciler scoring frozen commitments; `choir.commitment_score` OG kind; ActingPack isolation verified; full distributions retained; `confidence` never gates. | M1+M4 |
| M5b | **`jev.decide` + management RLM-ification** | `jev.decide` in-cell verb; low-confidence → RLM sub-cast w/ distribution as context var; management's typed tools (`report_to_texture`, `cancel_co_super_assignment`, assignment family) become `choir.*` verbs — last desk reaches the one-tool doctrine. | M5a |
| M5c | **Conductor `choice`-routing** | Route selection as a Jev `choice` question over M2's desk-chosen config surface; incremental override of `model-policy.toml`, not wholesale replacement (doctrine already schedules the broker-mediated cutover — this follows M5a's distribution evidence). Conductor call path (desk? direct judgments call?) resolved at charter. | M5b+M2 |
| — | **World Wire** | Resumes on these foundations. Named entry gate required: which prerequisites actually block + a deployed ingress-to-artifact proof. | all above |

### Open decisions for the owner

1. **M0 scope**: restart/redeploy + retest vs instrument-first (panel:
   redeploy first — cheap, and today's build carries the wake repairs).
2. **M0a phase split**: land emission+verbs while the typed surface still
   exists (rollback point, isolates the fix variable), or one cutover.
3. **Management RLM-ification**: fold into M5b (panel majority) vs a thin
   standalone station after M5c.
4. **Conductor call path**: is conductor route selection a `jev.decide`
   inside management's cell, or a direct judgments call from the conductor
   run profile? Conductor is not a desk — the fork changes M5c's shape.
