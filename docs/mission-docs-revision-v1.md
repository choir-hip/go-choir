# Mission M9: Docs Revision + Heresy Sweep — v1 — 2026-06-11

MissionGradient v2.0.0. Portfolio entry: `docs/mission-portfolio-2026-06-11.md` M9.
One-cut sections: none (docs mission). Sources: grand synthesis §6.1, review v2,
durable-actors doc, role-free protocol, proof-theory doc.

## Real artifact

The living canonical docs — `AGENTS.md`, `docs/README.md`, `glossary.md`,
`current-architecture.md`, `north-star.md`, `project-goals.md`,
`platform-os-app-state.md`, `computer-ontology.md`, `runtime-invariants.md`,
`implementation-scope.md` — brought into consistency with the post-cutover
ontology, plus a durable assertion/edge ledger. **Dated mission reports,
handoffs, and conjecture docs are immutable history and are NOT swept.**

## Invariants

- History is preserved: no edits to dated reports/handoffs.
- Transitional honesty: where code has not yet cut over (continuations still
  run, parent/child still in code), living docs say "being replaced, see X" —
  they do not describe the future as present (that would be its own heresy).
- No claim outruns its evidence class ("verified" never rendered as "safe").

## Conjecture Ledger

```text
id: M9-C1
claim: stale assertions in living docs regenerate bad behavior in every agent
       that reads them; sweeping them is consistency maintenance with
       compounding yield (docs are prompts for all trajectories)
test: grep-class zero-hit checks per heresy class after the sweep, plus the
      next missions (M1, M2) not re-deriving retired ontology
hyperthesis_edge:
  blind_spot: heresies expressed in vocabulary we are not grepping for
  boundary_type: frame_lock
  bound: the sweep agents also read-for-meaning, not only grep
scope_if_supported: living docs only; prompts (prompt_defaults/) are M2's
falsifier: an M1/M2 agent citing a living doc to justify retired ontology
status: active

id: M9-C2
claim: a durable assertion/invariant/edge ledger (receipts attached) is the
       missing surface that keeps doctrine and evidence joined
test: Conjecture E's tool-scope assertion, the spec results, and the open
      dialectic recorded with receipts; future docs cite the ledger
hyperthesis_edge:
  blind_spot: the ledger could rot like any doc (unmaintained = new heresy vector)
  boundary_type: resource
  bound: ledger updates are named in mission stopping conditions from now on
scope_if_supported: epistemic state only; never article/content truth
falsifier: the ledger uncited by any later mission
status: active
```

## Stopping condition

Sweeps done with per-class receipts (grep outputs); ontology chapter +
three-level table in current-architecture.md; glossary updated (new entries,
retirements, disambiguations); platform-os-app-state reconciled with shipped
reality; assertion ledger created; AGENTS.md transitional notes placed;
evidence ledger below filled; no claim outruns its class.

## Evidence ledger

| claim (scope) | evidence class | receipt |
|---|---|---|
| living docs heresy-swept for sandbox/parent-child/continuation/channel/persona/overclaim classes (the 7 delegated files) | agent sweep + residual grep counts | sweep agent report: all residual hits verified benign (code-name scoped, transitional notes, or historical classification) |
| platform-os-app-state.md describes shipped Features app, with design intent preserved under a labeled not-shipped heading (that file) | agent reconciliation, claims verified against code | reconciliation agent report; registry.ts:193-201, FeaturesApp.svelte, app_promotion.go checks |
| ontology chapter + retired-vocabulary list + three-level table added (current-architecture.md) | direct edit | current-architecture.md "Ontology (2026-06-11 Revision)" |
| messaging doctrine tied to actor model with legacy path named (current-architecture.md) | direct edit | Messaging And Routing section addition |
| glossary carries actor + conjecture vocabulary and retired terms (glossary.md) | direct edit | glossary.md new sections |
| durable epistemic state file exists: 6 assertions w/ receipts, 5 invariant candidates, 4 open edges, bimodal naming convention | direct edit | conjecture-assertion-ledger-2026-06.md |
| Features SSE handler now refreshes on owner_approved (drift found by reconciliation agent) | direct edit + frontend build | FeaturesApp.svelte handleLiveEvent; vite build green |

Conjecture ledger outcomes: M9-C1 active->supported at the grep/meaning level
(scope: the swept living set; the real falsifier — M1/M2 agents not
re-deriving retired ontology — remains open until those missions run).
M9-C2 active->supported at creation level (scope: the ledger exists and is
cited from the glossary; the rot edge stays open, bounded by the
stopping-condition rule in the skill).

Ledger-changed-behavior note (anti-decoration gate): M9-C1's frame_lock edge
("heresies in vocabulary we aren't grepping for") changed the sweep agents'
instructions from grep-only to read-for-meaning, which is what surfaced the
platform-os-app-state Proof Anchors framing fix and the SSE drift. The
ledger earned its keep this run.

## Run Checkpoint & Resumption State

status: complete
what shipped: heresy sweep across 7 living docs (agent), platform-os-app-state
  reconciliation (agent), ontology chapter + messaging update in
  current-architecture.md, glossary actor/conjecture/retired sections,
  conjecture-assertion-ledger-2026-06.md, FeaturesApp owner_approved SSE fix
what was proven: see evidence ledger; scopes stated per claim
unproven/partial: prompt_defaults still persona-framed (M2's job, by design);
  intended-architecture-next-2026-06-06.md not revised (predates the program;
  superseded in practice by the 2026-06-11 docs — flagged, not edited)
next executable probe: run M1 (trajectory model + proof mission)
rollback refs: single commit, revertable
