# Mission 3c Ledger

## Pass 1 — Part 1: AGENTS.md revision

**Conjecture:** The AGENTS.md revision can be landed as specified (split into 3
files, 4 new rules, deletion-first, simplified mutation class) without breaking
cross-references or the docs truth checker.

**Move:** construct (batch P1.1-P1.4 + verify)

**Expected ΔV:** -5 (P1.1, P1.2, P1.3, P1.4, verify)
**Actual ΔV:** -4 (P1.1-P1.4 done; verify folded into P1 — doccheck passes,
no broken cross-refs found)

**Verdict:** supported

**Receipts:**
- Commit `55ef75bb`: split AGENTS.md (417→224 lines) + created
  `docs/agent-product-doctrine.md` (197 lines) + `docs/agent-parallax-rules.md`
  (57 lines)
- `nix develop -c go run ./cmd/doccheck`: report-only complete, 285 docs, 1038
  warnings (pre-existing), exit 0
- `grep -rn 'AGENTS\.md#' docs/`: no anchor references — no broken cross-refs
- Pre-existing WIP (`docs/production-readiness-checklist.md`) stashed separately:
  `stash@{0}: pre-existing: production-readiness-checklist actor model review WIP`

**Edges left open:**
- AGENTS.md is 224 lines, over the ~150 target. The 4 new rules + deletion-first
  + all operating rules need the space. Further compression would lose content.
- Part 2 solvency: States 1-3 are mechanical (batchable), States 4-5 are
  medium-reasoning, State 6 is high-entanglement (8 pts), States 7-8 require
  deletion + staging access. Budget: 2-3 passes remaining.
