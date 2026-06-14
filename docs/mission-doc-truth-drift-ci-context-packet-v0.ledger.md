# Doc Truth, Drift CI, And Context Packet - Parallax Mission Ledger

## 2026-06-13 - Problem-First Split

Claim: the stale README service topology shows that the doctrine sweep repaired
framing but not docs truth maintenance. Heresy detector CI is necessary but too
narrow; Choir also needs deterministic docs drift checks and a generated context
packet for future agents.

Move: create a broader successor paradoc before editing README/service docs or
CI scripts.

Expected ΔV: -2 by naming the larger docs truth problem and separating
deterministic checks, context packet generation, LLM advisory review, and
docs-only CI policy.

Actual ΔV: -2. The mission now has a dedicated problem record, context-packet
conjecture, invariants, variant, and next move.

Receipt: `docs/mission-doc-truth-drift-ci-context-packet-v0.md`.

Open edge: no README repair, detector script, context compiler, or CI workflow
has been implemented yet. Repair is not claimed.

## 2026-06-13 - Linked Reviewed Checker Draft

Claim: the broader docs-truth mission needs a narrower checker spec that can
iterate independently with the originating agent.

Move: link `docs/mission-doc-heresy-checker-v0.md` as the reviewed checker spec
inside the broader mission.

Expected Delta V: -1 by making the implementation-scope checker review
discoverable from the broader docs-truth paradoc.

Actual Delta V: -1. The broader paradoc now references the narrower checker
mission.

Receipt: `docs/mission-doc-truth-drift-ci-context-packet-v0.md`.

Open edge: the checker spec still needs external review before implementation.

## 2026-06-13 - Added Suggested Goal String

Claim: paradocs are easier for the owner to resume when they include a
copy-pasteable goal string in the document and the assistant response.

Move: add a mission-specific `Suggested Goal String` to the paradoc and update
the repo Parallax skill to require this for new or materially re-scoped
paradocs.

Expected Delta V: -1 by reducing handoff ambiguity for the next run.

Actual Delta V: -1. The doc-truth mission now has a copy-pasteable resume
string.

Receipt: `docs/mission-doc-truth-drift-ci-context-packet-v0.md` and
`skills/parallax/SKILL.md`.

Open edge: the installed user-level Parallax skill under `~/.codex` was not
modified by this repo docs change.
