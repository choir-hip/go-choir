# Mission M3.1 Ledger

## 2026-06-13 - Open Recovery Paradoc

Claim: the M3 regression is not an implementation bug in
`requiredContinuationAfterVTextEdit`, but a proof/proxy capture: the acceptance
precondition requiring researcher participation before vmctl refresh leaked into
runtime semantics and paradoc next moves.

Move: shift / document. Created M3.1 paradoc as the new source program for
regression recovery. It records the aggregate review, answers how the pivot
happened, and names immediate and long-term repair paths.

Expected Delta V: define V and reduce ambiguity around the next move. Actual
Delta V: M3.1 V initialized at 9; no code fixes yet. The main gain is observer
shift from "debug deterministic continuation" to "remove forced workflow and
repair acceptance witness."

Receipt: `docs/mission-lifecycle-cutover-m3.1-v0.md`.

Open edge: no tests or code changes in this pass. The next pass must make the
documentation checkpoint durable in git before behavior-changing fixes, then
run focused tests and runtime shards.

## 2026-06-13 - Promote VText Agentic Invariant

Claim: VText semantics are fragile enough and central enough that they must be
contractual doctrine, not inferred from scattered prompts or past behavior.

Move: construct / doctrine. Added `docs/vtext-agentic-invariants-2026-06-13.md`
and updated `AGENTS.md` so future workers must read the invariant before
touching VText tools, prompts, routing, revision creation, coagent wake
behavior, Trace/VText projection, VText run acceptance, or VText-backed
missions.

Expected Delta V: -1 by resolving the missing shared VText invariant. Actual
Delta V: -1. M3.1 V moves from 9 to 8. Code still violates the invariant until
the forced researcher continuation and related tests are removed.

Receipt: `docs/vtext-agentic-invariants-2026-06-13.md`, `AGENTS.md`,
`docs/mission-lifecycle-cutover-m3.1-v0.md`.

Open edge: documentation is necessary but not sufficient. Next move remains a
behavior rollback: remove VText researcher hard continuation, narrow
required-next-tool, rewrite tests, and verify with focused runtime tests plus
runtime shards.

## 2026-06-14 - Make M3.1 Ready As Active Graph Node

Claim: after docs truth v1, a ready paradoc must be discoverable in the mission
graph and must carry a copy-pasteable Suggested Goal String, not just a terse
path stub.

Move: construct / handoff. Added `m3.1-lifecycle-recovery` to
`docs/mission-graph.yaml`, made M3 proper depend on it, marked M3 proper
blocked in the graph, added a full Suggested Goal String here, and added a
recovery gate note to `docs/mission-lifecycle-cutover-v0.md`.

Expected Delta V: -1 against handoff ambiguity. Actual Delta V: -1. M3.1 is
ready to execute as the active preflight mission; the code/test recovery V
remains 8.

Receipt: `docs/mission-graph.yaml`,
`docs/mission-lifecycle-cutover-m3.1-v0.md`,
`docs/mission-lifecycle-cutover-v0.md`.
