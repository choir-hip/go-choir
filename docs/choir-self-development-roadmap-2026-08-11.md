# Roadmap: Supervised Self-Development with Effects

**Date:** 2026-08-11
**Status:** converged consensus recommendation; not promoted doctrine
**Mutation class:** green (documentation only)
**Derived from:** three-round agentic consensus panel (divergent → lateral →
convergent) over the existing substrate; evidence in
`.agentic-consensus/self-dev-roadmap/`

## Thesis

The first self-development-with-effects proof runs on the **existing substrate**
(frozen `CapsuleEffectBundle` → independent verify → owner/rule settlement →
materialize → checkpoint → route → forward rollback). The RLM/Yaegi actor work
is an **authoring capability upgrade that comes strictly after the proof** — it
constructs the same bundle, it does not create new authority. The vision's
correction spine ("correction as an ordinary write") is the acceptance bar, so
the first proof is **accept A → admissible evidence falsifies A → B supersedes A
→ restart proves B**, not a single static write.

The panel was unanimous on the shape: **effects-first on existing substrate,
RLM strictly after, CTS superseded as `blocked_incomplete`, durable
owner-issued computer-bound key as Mission 0, staging rehearsal as a mandatory
gate.** The one divergence (E1 vs E2 proof bar) resolved toward **E2** with E1
retained as the Mission-3 rehearsal gate, not the Definition finish line.

## Mission sequence

| # | Mission | Class | Rollback |
|---|---------|-------|----------|
| 0 | Owner presence → mint one durable, narrow, **computer-bound** key; prove correct-target / wrong-target 403 / effects-OFF; recover **or explicitly retire** retained computer epoch `8253` | red | revoke key; no lifecycle/effect writes on failure (existing fail-closed ceremony) |
| 1 | **CTS disposition (C1):** owner-ratify supersession; fold evidence into successor; update `ACTIVE.md`, `mission-graph.yaml`, `doc-authority-manifest.yaml` | green | restore prior registry rows + ACTIVE pointer |
| 2 | Author effects Definition: D1-via-bundle envelope, E2 acceptance, mode ladder, rehearsal gates (problem-doc first) | green | delete/revert Definition + registry entries |
| 3 | Product-path plumbing + **staging rehearsal** `propose → accept_once → materialize → rollback` (and restart-durable read of applied state) with live flip gated | orange→red | mode CAS → `off`; selfdev rollback path; git revert through landing loop |
| 4 | **Live vision proof (E2):** accept A (model-policy bundle) → admissible evidence falsifies A → correction B supersedes A → restart proves B on tape | red | forward rollback txn + original policy bytes + mode `off` |
| 5 | RLM Phase 1 (Yaegi in disposable CoSuper capsules) as **authoring upgrade** — memo already preserves effects-OFF posture for that phase | orange/red | disable Yaegi mount; capsules remain networkless; no Accept/Materialize in interpreter |

Do not reopen CTS's full supervision choreography as a prerequisite for Mission
4. CTS's own `not_done_when` forbids effects while OFF; finishing CTS cannot
deliver the vision proof.

## RLM placement verdict

**After.** Vision's mission order never names RLM; memo Phase 1 explicitly keeps
effects-OFF; mode CAS / API / materializer / updater / frozen bundle already
exist. RLM upgrades *who authors* the bundle, not *whether* the computer can
land one. Minority RLM-first routes (Gemini opt2, Devin opt2, Gemini lateral)
were rejected: they serialize an authoring upgrade ahead of an already-built
landing path and contradict both the vision order and the RLM memo's own
effects-OFF Phase 1.

## First effects definition

**D1 content × D2 envelope** (reject bare D1 helpers; defer D3):

| Field | Decision |
|---|---|
| **What changes** | Computer-scoped **model policy** bytes (`System/model-policy.toml` / overlay) — durable, activation-read, reversible ("aircraft trim") |
| **Envelope** | Frozen `CapsuleEffectBundle` → accept under `accept_once` → materializer → `updater.ReleaseManifest` |
| **Authority** | Owner-granted mode CAS + decision binding + verifier receipts; human final settlement |
| **Rollback** | Self-development rollback path + exact prior policy SHA restore (CTS already pinned `7192b8b1…`) |
| **Not allowed as "the proof"** | Direct `/tmp/cts-acceptance-model-policy-*.py` PUT shortcuts — those are orange provider ceremonies, not the self-development spine |

## CTS disposition verdict

**C1 — supersede CTS as `blocked_incomplete`** (registry hygiene; evidence
retained). Precedent: `choir-cli-self-development-2026-07-16`
(`superseded`/`superseded_incomplete`). Crashed-prime panel already concluded:
do not resume CTS to `goal.complete()`. **Reject C2** (keep active-but-blocked):
ACTIVE still points agents at CTS; keeping it active is zombie-execution +
misdirection, and CTS cannot lawfully turn effects on. **Requires owner
ratification** (standing question 1).

## Proof bar verdict

**E2 for claiming the vision.** Vision's load-bearing spine is "correction as an
ordinary write," and "How Missions Flow" asks for a bounded correction loop, not
a single apply. **E1 is a hard rehearsal gate inside Mission 3**, not the
Definition finish line. Closing registries on E1 alone would ratify landing
machinery while missing the spine.

## Adjudication of standing dissents

1. **D1 = config theater?** **Merge: keep D1, require E2.** Fatal to
   *E1+D1-as-vision*, not fatal to D1 content. Aircraft-trim *plus*
   A→falsify→B→restart exercises fork/settle/supersede on a reversible
   flying-state surface.
2. **Human/Go-authored ≠ self-development?** **Reject as Phase-1 blocker;
   accept as RLM success criterion.** First proof needs computer-applied change
   under granted rules, tape-legible, restart-durable. "Model authored the Go"
   is Mission 5's bar, not Mission 4's.
3. **Materialization stranding?** **Accept as gate, not footnote.**
   `accept_once` is one-shot; failure → Degraded/Failed. Mission 3 rehearsal
   must PASS before Mission 4 flip.
4. **Owner key as Mission 0?** **Accept.** ACTIVE + CTS current state: no usable
   bearer; epoch 8253 unrecovered; every product-path mission dies at headed
   Chrome + Touch ID. Put the durable computer-bound key before any staging
   proof.

## Top 3 roadmap-killing risks

1. **No owner-issued computer-bound bearer** → Gate: Mission 0 PASS (key +
   wrong-target denial + effects OFF).
2. **Bad first materialization strands the computer** → Gate: Mission 3
   accept→materialize→rollback(/restart) PASS before live `accept_once`.
3. **False vision claim (E1 or policy-PUT theater)** → Gate:
   Definition/`goal.complete` requires E2 tape (A, falsifying evidence, B
   supersession, post-restart B); refuse registry close otherwise.

## Evidence / assumptions

- Vision proof target + correction spine + mission order: `docs/choir-vision.md`
- Modes default off / `accept_once` expiry: `internal/platform/self_development_modes.go`
- Materializer → updater ladder: `internal/agentcore/self_development_materializer.go`
- Decision binding (both heads, verifier refs, mode receipt): `internal/agentcore/self_development_decision_binding.go`
- Frozen envelope: `internal/capsule/transaction/builder.go`
- RLM Phase 1 preserves effects-OFF: `docs/memo-persistent-rlm-actors-2026-08-09.md`
- CTS forbids effects while OFF; blocked on owner presence; policy helpers
  prepared not executed: CTS Definition current state
- CTS "don't resume": `docs/choir-crashed-prime-session-review-2026-08-09.md`
- Owner ratification required for settlement: `docs/standing-questions.md` §1
- Updater release/rollback: `internal/updater/updater.go`

**Assumption:** rival-proposal / supersession semantics needed for E2 are
expressible on the existing event/decision/selfdev operation graph without
inventing a new settlement subsystem. If Mission 2 pre-flight finds that gap,
document it before coding (problem-documentation-first) — do not silently
downgrade to E1.

## Consensus record

- **Divergent** (5/8 succeeded: gemini36, devin, cursor, deepseek-v4-flash,
  gpt56-sol): families (a) effects-first/RLM-after, (b) RLM-first, (c) parallel
  lanes; CTS supersede majority; effect content dimensions. Failure modes:
  grok45 provider key, codex + opencode 300s timeout.
- **Lateral** (6/8 succeeded: + opencode): reframes — constitutional-commit
  problem not agent-intelligence (gpt56); smallest accepted append on canonical
  event chain, central-bank settlement analogy (cursor); decouple from physical
  owner presence (gemini); model policy as first effect / aircraft-trim
  (opencode); effect machinery already built, missing ceremony + plumbing
  (deepseek).
- **Convergent** (3/8 full verdicts: cursor, gemini36, devin — all HIGH
  confidence): unanimous on A-over-B, C1, RLM-after, Mission 0, D1-via-bundle;
  cursor + gemini on E2-as-vision-bar, devin on E1-with-E2-next (resolved above).
  Failures: grok45 provider key, codex 300s timeout, gpt56-sol 300s timeout,
  deepseek empty (timeout), opencode truncated mid-investigation (no verdict).
- Panel risk gates and adjudication tables are preserved verbatim in
  `.agentic-consensus/self-dev-roadmap/{divergent,lateral,convergent}/*.out`.

## Next actions

1. Owner: ratify Mission 0 (durable computer-bound key ceremony) and C1
   (CTS supersession) — the two gates everything else waits on.
2. After ratification: read `docs/standing-questions.md` and draft the successor
   Definition (Mission 2 spine) via `skill://definition`; CTS supersede with
   evidence folding (Mission 1) as the first committed action.
3. Mission 3 staging rehearsal before any live flip; E2 tape before any
   registry close.
