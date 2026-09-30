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


## B. The ratified shape (owner, 2026-09-29)

- **The commitment scorer IS management.** Not a separate async runtime
  piece — scoring is management's job. Management can score with Jev,
  other decision models, LLMs, or any combination; Jev is the default
  starting method, escalating to an RLM sub-cast on low confidence. The
  scoring surface and the decision surface are the same desk, not two
  surfaces sharing a transport.

- **Management's routing is coupled to precommitment scoring.** The
  passthrough (Jev agrees → route as-is) or translator work management
  does between engineering and texture is downstream of the
  precommitments — engineering executes what management scored. Routing
  and scoring are one responsibility, not two stations.

- **Conductor does policy routing, not model routing.** Conductor
  classifies user intent from the prompt bar and selects which appagent
  receives it (texture is the default today; the prompt bar gets more
  adaptive over time). Conductor never picks a model — appagents and
  worker agents own model selection.

- **Model policy is an RLM module, not conductor's job.** A module gives
  desks two capabilities: (i) *persistent* per-desk model policy changes
  — e.g. texture changes its own model inside a Go cell and it sticks;
  (ii) *parametric* selection — e.g. research casts sub-RLMs and chooses
  the model at call time. This is what "RLM-driven model switching" means.

- **Precommitments go from strings to types** (confirmed): typed questions
  + frozen probabilities at `Precommit`; evidence refs at `Resolve`;
  `Disagreement` split so scorer disagreement never reaches ActingPack.

- **What still governs from the 9/27 panels:** pinned `typesafe/jev-1.13`
  (never `jev-latest`); gateway `POST /provider/v1/judgments` → OpenRouter
  `/api/alpha/decisions`; per-VM bearer + own rate bucket; scores land as
  `choir.commitment_score`; full distributions recorded; replay =
  deterministic score ID bound to the resolution.

---

## C. Metamission skeleton (post-panel, post-owner-corrections)

5-agent convergent panel reviewed 2026-09-29
(`.agentic-consensus/agentic-consensus-20260929-195929/`). Then corrected
by owner: scorer=management, conductor=policy-routing, model policy = RLM
module. Station list:

| # | Station | Delivers | Depends |
|---|---|---|---|
| M0 | **Debug + stabilize** | Redeploy owner computer onto today's build; reconcile the 7 pending mutations + 3 stale runs (counter → 0, not masked by idle-sweep busy-check); SSE auto-resubscribe (~20 lines, `frontend/src/lib/lifecycle.js` — `onerror` resumes via `?after=` cursor, no terminal error); baseline per-leg timings; problem record. No throughput bar here. | none — first |
| M-SUB | **Eager evidence stream + stall terminator** (red-class kernel — panel-adjudicated architecture below) | Emissions = a distinct durable channel record kind, written synchronously mid-cell by a new broker action `ActionEmit` — NOT tray-staged (bound-cell `choir.Message` is tray-staged today; my earlier premise that `ActionMessage` delivers mid-cell was wrong). Delivery-is-the-record: emitted evidence stands after a crash, marked by kind, never tombstoned; cell fate recorded separately so consumers join emission→outcome. Follow-ups reach research via cell-boundary inbox + cheap `choir.PollInbox` broker verb for long loops — **true mid-cell push is infeasible** (worker is blocked inside `sess.Eval`; a pushed frame can't be consumed). Stall terminator: durable `cell_fate` record on every cell exit + armed terminal deadline per activation (existing `scheduleContinuation` pattern, not a sweep) — today `ReduceCellIntents(failed)` persists nothing, that's the hole. Backpressure = admission-side emit budget; per-channel order only; fanout = per-consumer cursors on the shared channel log. | M0 |
| M0a-1 | **In-cell research verbs** | `choir.WebSearch`/`FetchURL`/evidence ops as Go verbs; egress budget rehomed (typed tools carry per-activation egress + 8GiB cap — deleting without rehoming deletes governance). Typed surface still live — controlled-comparison verify of the stall fix. | M-SUB |
| M0a-2 | **Research tool-surface deletion** | Delete the 14-tool surface; capability-parity checklist (each capability reachable via `choir.*` or dropped with reason); prompt-overlay rewrites in the same commit (`rlm_research_runtime.yaml`, `research.yaml` instruct the deleted cadence). | M0a-1 |
| M1 | **Scoreable commitments** | `Precommit`/`Resolve`/`Disagreement` from strings to types; string-commitment grandfathering stated. | parallel-safe — types/store change |
| M2 | **Model-policy RLM module + eval surface** | Persistent per-desk model policy (texture changes its own model in a Go cell) + parametric selection at cast time (research picks the sub-RLM's model). Evals as parallel RLM casts; per-turn timing/token/cost records; QA fixture/golden set. | M0a-2 for research-valid evals; M1 for scored evals |
| M3 | **Research hill-climbing — iterative research shape** | Multi-model/effort matrix on the QA family via M2's module. Success = a research pattern that is fast AND deep: continuous iterative research running for days/weeks, evidence streaming to texture, eventually feeding more than one texture — the World-Wire seed, not a single-doc loop. ("10 versions in 5 min" is a directional smell-test, not a guarantee to Goodhart.) | M2 |
| M4 | **Jev transport** | Gateway `POST /provider/v1/judgments` → OpenRouter `typesafe/jev-1.13`; per-VM bearer + rate bucket; alpha credential provisioning. | none — parallel |
| M5 | **Management = scorer + `jev.decide` + mgmt RLM-ification** | Scoring is management's job (Jev default, pluggable with other decision models/LLMs later); low confidence → RLM sub-cast w/ distribution in context; passthrough-or-translate routing between eng and texture rides the same scoring loop; `choir.commitment_score` kind; management's typed tools → `choir.*` verbs (last desk to one-tool). | M1+M4 |
| — | **World Wire** | Named entry gate: which prerequisites block + deployed ingress-to-artifact proof. | all above |

### Open questions — resolved by owner 2026-09-29

1. **M0**: redeploy the computer onto today's build first. Resolved.
2. **M0a phase split** — clarified as: sequential commits within one
   mission, not separate missions. Land the new verbs + emission while
   the typed surface still exists (verifiable, rollback-able), then the
   deletion commit. Default per panel.
3. **Emission-divergence rule** — resolved by the M-SUB architecture
   panel (`.agentic-consensus/agentic-consensus-20260929-205259/`):
   delivery-is-the-record. Emissions are a distinct durable channel kind;
   a cell that emits then crashes leaves the evidence standing (texture
   may already have read it — tombstoning can't undo that). Provisional-
   vs-committed is carried by record KIND, and a separate `cell_fate`
   record lets consumers join emission→outcome and downgrade confidence
   on orphaned evidence.
