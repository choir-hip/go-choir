# Mission Portfolio Ledger

## 2026-06-12 — Architecture-First Recut

Claim/scope: the portfolio should optimize for durable actor architecture and
old-code deletion before product-facing success. Product surfaces are useful
as falsifiers only after the substrate can carry their meaning. Scope is docs
and mission sequencing; no runtime behavior change.

Move: shift observer from product path to architecture spine. Rewrote the
portfolio Parallax State so M1-M4 are the core cutover spine, M5 is a
post-spine falsifier, M6/M8 are promotion substrate, and M7 is review UI on
top of real promotion/rollback.

Expected ΔV: 0 for implementation, +observer evidence. The portfolio should
not claim architectural descent from a docs recut, but future passes should
avoid descent-free product detours.

Actual ΔV: 0. Current portfolio V is 7: M2, M3, M4, M5, M6, M8, and M7 remain
unsettled. M9 and M1 stay done; M5 remains deferred until after M2-M4.

Receipt:
- Updated `docs/mission-portfolio-2026-06-11.md` with an
  architecture-first revision section.
- Updated the recommended order to M9 -> M1 -> M2 -> M3 -> M4 -> M5 -> M6
  -> M8 -> M7, with M10/M11/M12 side tracks only when they do not distract
  from the spine.
- Marked mission kinds: spine, falsifier, promotion substrate, review
  surface, side track.
- Reframed M4 around removing residual RunContinuation record/API/event
  surfaces because M1a already deleted the synthesis decision layer.

Open edge: execute M2 next and prove old messaging mechanisms are deleted or
carried only as named temporary shims inside one landing batch.

## 2026-06-13 - M2 Settled, M3 Prepared

Claim/scope: portfolio spine advances only after M2's post-review repair lands
on staging; no product-surface detour.

Move: recorded M2 settlement in `docs/archive/mission-messaging-cutover-v0.md`,
updated portfolio V from 7 to 6, marked M2 done, and compiled the M3 lifecycle
paradoc at `docs/archive/mission-lifecycle-cutover-v0.md`.

Actual Delta V: -1 at the portfolio level. Next spine mission is M3 lifecycle
cutover.

Receipts:
- pushed/deployed M2 repair commit
  `794d28dd76ff00a2ae27c98a14dbce9e34834695`;
- CI run `27455953966` passed;
- Node B deploy job `81160546255` passed;
- `https://choir.news/health` reported proxy and sandbox deployed at the same
  SHA;
- deployed lifecycle/prompt-bar acceptance passed:
  `GO_CHOIR_RUN_DEPLOYED_LIFECYCLE=1 CHOIR_DEPLOYED_BASE_URL=https://choir.news pnpm --dir frontend exec playwright test tests/adaptive-lifecycle-control-deployed.spec.js --project=chromium --reporter=list`.

## 2026-06-14 - Mission Corpus Graph And M3.1 Gate

Claim/scope: after docs truth v1, the portfolio should use
`docs/mission-graph.yaml` as the mission-corpus index and should route fresh
agents through M3.1 before M3 proper. Scope is docs/graph hygiene only; no
runtime behavior change.

Move: indexed all mission-shaped docs in the mission graph without rewriting
historical MissionGradient reports, added the M3.1 recovery node between M2
and M3, marked M3 blocked behind M3.1, and updated the portfolio next move and
execution order.

Expected Delta V: -1 by removing handoff ambiguity around the next spine move.
Actual Delta V: -1. Portfolio V is now 7 because M3.1 is a real recovery gate
that must settle before M3 can reduce lifecycle V.

Receipts:
- `docs/mission-graph.yaml` now indexes the mission corpus;
- `docs/archive/mission-lifecycle-cutover-m3.1-v0.md` carries the active goal string;
- `docs/archive/mission-lifecycle-cutover-v0.md` names the M3.1 recovery gate.

Open edge: historical mission docs remain indexed evidence, not converted
Parallax doctrine. Direct rewrites should be limited to active/open missions
or to factual drift that changes a future route.

## 2026-06-15 - M3.2 Prompt/Decision Notes Gate

Claim/scope: M3.1 is settled as the emergency forced-workflow repair, but M3
should not resume until VText has an off-document decision channel and prompt
defaults explain delegation pressure without forcing tool choreography or
polluting canonical documents. Scope is docs/graph routing only; the behavior
mission remains open.

Move: created `docs/archive/mission-vtext-prompt-decision-notes-m3.2-v0.md`, added its
ledger, inserted it between M3.1 and M3 in `docs/mission-graph.yaml`, marked
M3.1 settled in the graph, and updated the portfolio next move.

Expected Delta V: 0 net at portfolio level. M3.1 closure removes one gate, and
M3.2 adds the durable prompt/decision-observability gate before M3. Actual
Delta V: 0. Portfolio V remains 7.

Receipts:
- `docs/archive/mission-vtext-prompt-decision-notes-m3.2-v0.md`
- `docs/mission-vtext-prompt-decision-notes-m3.2-v0.ledger.md`
- `docs/mission-graph.yaml`
- `go run ./cmd/doccheck` completed report-only: 204 docs, 800 warnings,
  2619ms.

Open edge: M3.2 implementation must start with Problem Documentation First
because it touches protected VText tools/prompts, runtime schema, Trace/event
projection, logs, and VText UI.
