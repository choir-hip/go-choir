# RLM Mission Three — Executability Review Record

Date: 2026-09-11. Mode: convergent. Panel: 13 agents; 12 ok, 1 quota failure, 0 task failures.
Prompt: `.agentic-consensus/mission3-review/prompt.md` (gitignored process diagnostics).
Raw outputs: `.agentic-consensus/mission3-review/run1/` (manifest, per-agent output, commands).
Review substrate: `docs/definitions/choir-rlm-engineering-carrier-2026-09-11.md` at draft commit
`a6af6d55`, with its adjudication record
`docs/reports/choir-rlm-mission-three-consensus-2026-09-11.md`.

This record binds the panel, its verdict, and the adjudication that produced the revised
Definition. The revision is the artifact; this is its review receipt.

## Panel

| Agent | Status | Duration |
| --- | --- | --- |
| omp-muse-spark (`opencode-zen/muse-spark-1.3-contributor-free`) | ok | 66s |
| omp-ling (`opencode-zen/ling-3.0-flash-fin-free`) | ok | 81s |
| omp-glm53-flash | ok | 98s |
| opencode | ok | 123s |
| omp-gpt56-sol | ok | 205s |
| omp-cursor-grok46 | ok | 266s |
| cursor | ok | 267s |
| devin | ok | 271s |
| omp-gpt56-luna | ok | 276s |
| claude (opus) | ok | 293s |
| codex | ok | 393s |
| omp-nemotron-3-ultra | ok | 568s |
| omp-gemini38 | failed (quota) | 4s |

`omp-gemini38` failed with `Cloud Code Assist API error (429) Resource has been exhausted`. That is
a provider quota event on the Antigravity route, recorded as panel metadata, not a task failure.
Twelve substantive reviews were produced; the run used `--keep-going`.

## Verdict (unanimous)

**Not executable as written; do not charter.** Every substantive panelist reached the same verdict
independently: the scope was right and the evidence discipline was strong, but `/goal` would stall
or invent decisions at the entry gate, in two schema-incomplete acceptance items, in the settlement
transfer, in the replay proof, and in an `actuator=tools` contradiction. The revised Definition
folds every must-fix item below.

## Locally verified findings

The orchestrator re-checked each load-bearing claim in source before folding it in.

| Finding | Status | Evidence |
| --- | --- | --- |
| Both verifier gates require `RunRecord.Metadata["co_super_slot"] == "verifier"` | **verified** | `internal/agentcore/tools_capsule.go:410-412,491-493` |
| The assigned CoSuper runtime writes `assignment_kind` but never `co_super_slot`, and generic CoSuper activation is refused | **verified** | `internal/agentcore/cosuper_assignment_runtime.go:342-355`; `internal/agentcore/runtime.go:987-988` |
| No test pins either verifier gate | **verified** | no match in `internal/agentcore/*_test.go` |
| Acceptance checkpoints are keyed on tool names behind presence guards | **verified** | `internal/agentcore/run_acceptance.go:627-645` |
| The collector skips error results, so a failing tool cannot satisfy a checkpoint | **verified** | `internal/agentcore/run_acceptance.go:300-302` |
| `ReduceCellIntents` commits Complete as a mailbox envelope only | **verified** | `internal/agentcore/rlm_reduce.go:167-198` |
| The run loop's detached-terminal predicate keys on the fate tool's name | **verified** | `internal/agentcore/runtime.go:3392` |
| `actuator=tools` is the live fallback, not dead code: RLM is served only when requested AND the session worker is ready | **verified** | `cmd/capsule-broker/main.go:73-79`; `internal/capsule/actuator.go:16-21,37-47` |
| The tools branch composes `RegisterCapsuleLocalTools` (six tools, including `capsule_go_eval` and `record_assignment_result`); the RLM branch composes a separate sealed list | **verified** | `internal/agentcore/tool_profiles.go:325-358`; `internal/agentcore/tools_capsule.go:76-87` |
| Live prompts still name retired tools | **verified** | `internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml:9,56`; `engineering_runtime.yaml:7-8` |
| Mission two is `completed` at `e3396329` / CI `34571343061` with a published deployed-proof artifact, `entrypoint: false`, `next_action: none` | **verified** | predecessor Definition `:220-290`; `docs/evidence/choir-rlm-versioned-rename-deployed-proof-2026-09-11.md` |
| Mission three is absent from `mission-graph.yaml` and `doc-authority-manifest.yaml`; zero `entrypoint: true` rows exist today | **verified** | grep against both registries; `mission-graph.yaml:16-17` graph rule |
| The post-mission-two GC work landed as tracked commits through `a907f713`, and the tree is clean now | **verified** | `git log`; `git status --short` at `a907f713` |
| "No OpenCode adapter exists in Go source" (claude) | **verified** | no `opencode` match in Go source; the research note's phase-1 plan is greenfield |

## Must-fix convergence and adjudication

| # | Must-fix finding | Raised by | Adjudication → where folded |
| --- | --- | --- | --- |
| 1 | Stale entry gate: mission two is already complete; "terminal deployed acceptance" names a quality, not a receipt | glm53, opencode, muse-spark, claude, cursor, devin, sol, luna, nemotron, grok, codex, ling | Adopted. `start.predecessor_receipt` + `start.entry_gate` + `start.start_correction`; `now.slice` = entry reconciliation |
| 2 | `now.status: blocked_incomplete` contradicts a mutating `now.next_action` | ling, cursor, grok, sol, codex, opencode | Adopted. `next_action` is now read-only reconciliation then charter |
| 3 | `now.next_action` holds two actions | grok, codex | Adopted. One action arc, read-only while blocked |
| 4 | `now.decision` and `now.candidate` missing; owner answers only in prose | claude, cursor, grok, luna, codex | Adopted. Six settled owner decisions with consequences, and an explicit empty candidate |
| 5 | `T5-roster` and `T5-cache-exit` lack `proves`/`evidence_class`; cache-exit's contract was misattached to `T5b` | muse-spark, glm53, claude, cursor, devin, grok, codex, nemotron | Adopted. Item contracts fixed; long fields rewritten as folded blocks so readers stop seeing truncated lines |
| 6 | Roster floor mismatch: `not_done_when` was satisfied by one model passing | claude, cursor, devin, grok, codex | Adopted. All four expected ids must pass; Muse Spark free-or-paid counts as one |
| 7 | Repairs precede the problem-documentation-first boundary (P2 before the mapping freeze) | devin, luna, grok, muse-spark, codex | Adopted. P0-define is the first acceptance item and the first mission commit |
| 8 | T4-replay assumes a harness, fixtures and identities that do not exist; in-cell reads mint a new request id per call | claude, cursor, devin, sol, codex, nemotron, muse-spark, grok | Adopted. `P4-harness` builds the substrate and captures goldens pre-cutover; equality is canonical fields, never raw bytes; the four deferred capsule operations declare fixture-based proof |
| 9 | Settlement transfer under-specified: partial-result path, detached-terminal predicate, admission-grammar special cases, single-author proof | devin, claude, codex, muse-spark, sol, luna | Adopted. `P3-settlement` names the semantics to reproduce and the single-author proofs; evidence raised to `deployed_proof` |
| 10 | `actuator=tools` contradiction: "delete the nine" versus "keep the branch alive" | claude, devin, ling, grok, codex, cursor, nemotron | Adopted with the owner's override: the five overlay names are deleted here; the four capsule operations stay functional for the fallback until R8; the tools branch is never a rollback target |
| 11 | `T3-parity` offered an escape hatch an orchestrator could self-approve | grok, devin, glm53, codex, cursor | Adopted. Carrying the checks is the requirement; a reduction is an owner decision recorded in `now.decision`, not an orchestrator choice |
| 12 | `T6-texture-design` is non-gating yet sits in `finish.acceptance` and `deliver` | sol, grok, codex, ling, cursor, muse-spark | Adopted. Moved to `finish.non_gating_artifacts`; it cannot gate `complete` |
| 13 | Red ceremony wrong: gateway/provider and run acceptance are red, not orange | sol, codex, nemotron, grok | Adopted. `boundaries.mutation_class: red` with named red subclasses, per-phase rollback, and per-surface evidence floor |
| 14 | Phase 1 rollback targets a nonexistent stub; keys need a named authorized path and revocation | claude, cursor, codex, grok, sol, devin | Adopted. Phase 1 is greenfield: revert the added adapters, remove the credential, restart, prove health; keys arrive by the product path or a recorded break-glass step |
| 15 | Deployed proofs appear before the only deploy phase; landing is circular | sol, codex, grok | Adopted. `P6-landing` runs an intermediate deploy before the deployed replay and roster proofs and names the exact deployed engineering-assignment scenario |
| 16 | Registry promotion missing: the draft claims `execution_mode: mission_orchestrator` while absent from the graph | grok, cursor, devin, glm53, codex | Adopted. `start.entry_gate.registry_promotion` requires atomic promotion in all three registries |
| 17 | Weak measures without baselines | claude, sol, nemotron, codex | Adopted. `measures` are structured with baseline, desired, decision use, and what each cannot prove |
| 18 | `T1-invariant` passes trivially today (no model input reaches prompt assembly) | claude | Adopted. The digest is computed from prompts actually assembled during roster runs, with a frozen entropy-exclusion list and a standing guard |
| 19 | `CleanGoSource` deletion touches the shared broker session path the fallback still uses | claude | Adopted. P2-repair-deletion states the non-regression for the tools branch |
| 20 | Unbounded prompt-fix loop | claude, sol, codex, grok, muse-spark | Adopted. Three shared revisions, then `blocked_incomplete` with an honest result |
| 21 | Retired names still appear in live prompts and defaults | devin, cursor | Adopted. P4-delete requires the same-commit citer sweep including prompt defaults and the RLM overlay |
| 22 | Local tests cannot prove a live singleton catalogue or fail-closed runtime behaviour | codex, nemotron | Adopted. P4-registry observes the live catalogue; P3-acceptance proves its negative on staging |
| 23 | Worktree classification: mid-cleanup WIP must be preserved, not inherited | claude, grok, codex | Adopted. `start.worktree_inventory` re-verified at `a907f713`; GC work is explicitly out of scope; re-check at charter |
| 24 | Predecessor status token `completed` is not a skill token | claude, codex, grok | Adopted. Recorded in `start.predecessor_receipt.status_token_note` so no gate keys on the token |

## Dissent and disagreements

- **Provider setup should stay a separate red boundary** (sol, and the earlier consensus panel's
  position). The owner overrode placement; the Definition preserves the substance: phase 1 keeps its
  own problem record, its own Landing Loop, its own rollback, and its own receipts.
- **Texture design should be a sibling artifact** (sol, plus the earlier cursor-grok and gemini38
  position). Partially adopted: it is out of `acceptance` and `deliver` entirely, so it cannot gate.
- **Model behaviour should not be the completion gate** (claude, carried from the first panel).
  Adopted as before: the structural invariant gates, and the roster is required evidence with a
  bounded, non-per-model failure response.
- **`T3-parity` should be an owner question rather than a requirement** (cursor). Rejected: the
  owner never asked to surrender the validation, so carrying it is the default and only an explicit
  owner decision may reduce it.
- **Claude's claim of six modified tracked files** was not reproducible: at `a907f713` the tree is
  clean and the GC/vmctl/store work is committed. Recorded as a mid-run observation, not a defect.

## Unverified / carried assumptions

- Live provider facts (session-header enforcement, per-model wire shape, roster results) come from
  the 2026-09-10 research note; phase 1 re-pins them with live probes.
- Whether self-development operations can be staged with effects OFF for golden capture is unknown;
  the Definition requires phase 4 to resolve it rather than assume it.
- The four `update_coagent` citer inventory items are panel assertions confirmed by a read-only
  scout sweep, not by an exhaustive gate; `P3-parity` must make the inventory machine-checkable.

## Recommendation

Charter on the entry gate alone: re-read the predecessor receipt, dispose the post-mission-two work,
promote this Definition in the three registries atomically, and date the owner charter. The first
executed action is `P0-define`, code-free. No repair, settlement, or deletion code precedes it.
