# Texture desk activation contract: undelivered-update respawn loop and fatal write gate

Date: 2026-09-28
Status: **problem documented; design panel-adjudicated; implementation pending owner ratification.**
Mutation class: **red** (Texture canonical writes, lifecycle reducer semantics, run acceptance).
Cluster rollup for: `texture-incorporate-deadlock-settled-producer-2026-09-28.md`,
`texture-desk-decision-kind-wait-rejected-2026-09-28.md`,
`vmctl-pressure-reclaim-mid-respawn-gap-2026-09-28.md`,
`vmctl-idle-sweep-mid-desk-respawn-2026-09-28.md` — same substrate, four receipts.

## The contract being restored

Owner direction (2026-09-28, in-session): Texture is the semantic control
plane. The versioned doc replaces chat.

- Owner edits are diffs against the doc; every revision is canonical.
- Desks inside the VM never edit the doc. They send evidence and messages
  upstream only.
- Texture's desk activates on any new information — owner revision, worker
  evidence, control delivery — and is **biased to land a revision on every
  activation**, even minimal (evidence transclusion counts; the doc is the
  belief state of record).
- Texture turns user ideas into falsifiable precommitments that trigger
  downstream work and get scored. Texture can open inquiries proactively.
- Steering off-target workers happens through messages/controls, not
  lifecycle plumbing.
- Principals are plural: human owner, external harness agents via choir
  CLI/API, and Choir-in-Choir peer computers acting as user — all through
  the same owner-revision surface.

Target cadence: a revision per semantic event, roughly 1–5 minutes on a
healthy desk.

## The failure cluster's common cause

Current machinery: producer reports queue as lifecycle updates
`Disposition=pending`; `ReconcileAgentWakeLocked` re-arms whenever
`len(pending) > 0 || ownerHeadPending` (`texture_controller.go:745-791`);
an update leaves `pending` only when `ApplyTextureTurn` names it in
`update_dispositions[]` (`texture_turn.go:477-490`); the required-write
gate fails a run after 2 unsuccessful write-tool attempts
(`toolloop.go:658-713`).

Every receipt this week traced to the same wedge: **an update stays pending
forever if the desk's write path slips**, and each failed run respawns the
desk (~45 s cadence, 33+ consecutive failures observed, ~zero revisions
committed in 40 min on staging today).

Mechanism detail: the run-fatal gate + verbatim retry + disposition-required
inbox + reconcile-on-pending compound into an unbounded respawn loop with no
circuit breaker.

## Panel-adjudicated design (7/7 convergence)

Consensus run: `.agentic-consensus/agentic-consensus-20260928-112330/`
(prompt `.agentic-consensus/texture-desk-contract-20260928.md`; succeeded:
claude, codex, gpt-6-sol, gemini-3.8, glm-5.3, devin, gpt-6-luna; failed:
5 CLI/route outages).

1. **Bind at dispatch, consume at commit.** Bound updates get
   `DeliveredToRunID` written at reconcile (control-direction bind already
   exists); they terminalize inside the committed `ApplyTextureTurn`. A
   pre-commit crash leaves them pending for redelivery — at-least-once.
2. **Un-named bound updates default to terminal `delivered`** (new
   disposition), `disposition_ref` = the committed revision/turn — never
   auto-`incorporated` (would falsely claim semantic use; `incorporated`
   requires `work_result_ref` against producer work). `incorporated` /
   `rejected` stay available as explicit signal.
3. **Bounded redelivery:** run terminal with no commit → unbind +
   `delivery_attempts++`; at cap (~2–3) → terminal `delivered` with
   `delivery_attempts_exhausted` + durable `activation_failed` event.
   Nothing drops silently; poison packets can't loop.
4. **Owner-head asymmetry:** crashed run does NOT consume
   `ownerHeadPending` (an unanswered owner directive is product-visible);
   same cap → `activation_failed` event, head stays armed.
5. **Write gate non-fatal:** keep the reminder turn; a no-write end is a
   completed no-op activation. Deterministic validation rejections never
   retry (same-call replay under `call_id` can't self-correct).
6. **Precommitments:** already first-class (`choir.commitment_record` OG
   objects); apply turn mints them atomically; doc renders the ledger.
7. **Principal authority:** `author_kind=agent` + provenance suffices at
   the turn reducer; scoping belongs at auth. **Verified gap:**
   `PendingTextureOwnerRevision` requires `AuthorKind == AuthorUser`
   (`texture_owner_revision.go`) — agent-principal diffs would not wake
   the desk today. Widen to non-desk authors.

## Open adjudication points (owner)

- **Fallback revision vs `activation_failed` event on breaker trip.**
  Claude proposed a runtime-authored minimal revision ("interpretation
  deferred" transclusion under Texture's writer identity); devin/glm/luna
  preferred the durable event without a synthetic revision — the doc is
  belief state, not a receipt log, and runtime-authored text claims a
  semantic authorship no desk performed. Recommendation: event, not
  revision.
- **`delivered` vs `incorporated` default** — majority keeps them distinct
  (scoring signal); Gemini argued full collapse to `incorporated` since the
  desk needn't name anything.
- **Digest determinism:** auto-added default dispositions must be
  deterministic or excluded from the turn digest — `Inbound` is part of
  command identity; nondeterministic defaults break replay (devin).

## Repair log

- **2026-09-30 — owner-revision trigger non-disposal (defect #5 of the M1
  deployed proof).** Root cause: a texture desk `desk_go_eval` cell that staged
  no `ApplyTexture`/`decide` intent reduced its tray and left the
  `texture_activation` occurrence pending; `TextureActorOccurrencePostcondition`
  (DocumentRevision arm) requires either head advancement or a
  `texture_turn_committed` event whose `artifactRefs[1]` equals the trigger
  head, neither of which an intent-less cell ever produced. The wake re-armed
  on every reconcile (`without disposing exact trigger`, ~30 s cadence).
  Repair: `Runtime.consumeIdleTextureTrigger` (`internal/agentcore/tools_desk.go`)
  auto-commits a `no_semantic_change` decide turn (base = current head) when a
  texture-profile cell commits no `IntentTextureApply`, emitting the consumed
  marker. Gated to `runHasProfile(rec, agentprofile.Texture)`; tolerates
  already-consumed/not-pending/missing-doc so a non-authoring cell never
  fails over a disposed wake. Landed in `a08defc0` (consume), `6f685d6f`
  (tolerance), `7b78b9e7` (texture-profile gate). This is the point-5
  adjudicated path — "a no-write end is a completed no-op activation" — for the
  owner-revision arm only; the pending-update disposition machinery (points
  1–3) remains open.

## Adjacent defects (independent of this fix)

- Retry replays the identical call verbatim under same `call_id` — must
  return the tool error to the model and let it re-compose.
- `run_system.yaml` still encourages off-document decisions; conflicts with
  the ratified revise-on-information contract.
- Every run terminal outcome must unconditionally terminalize its pending
  mutation row (passivation covers it; plain failure paths need an audit).
