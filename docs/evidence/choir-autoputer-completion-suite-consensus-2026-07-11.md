# Choir Autoputer Completion Suite Consensus Evidence

## Scope

Definition-gate review of `docs/definitions/choir-autoputer-completion-suite-2026-07-11.md` and its active registries/subordinate contracts before execution.

## Panel

The 2026-07-11 post-diff panel completed six independent opinions:

| Reviewer | Verdict | Material finding |
|---|---|---|
| Codex | MODIFY | Competing OG/Dolt authority; ambiguous containment/dependency semantics; delegated-work resumption not transactional. |
| Devin | MODIFY | OG/Dolt remained a competing `/goal` and graph spine. |
| OpenCode | MODIFY | OG/Dolt competing authority; audited-computer S4 exceeded the named PC-5 contract. |
| OMP GPT-5.5 | MODIFY | Competing authority; durable delegation ledger; S1 ratchet exception; S4 boundary; machine-visible membership. |
| OMP GLM-5.2 | MODIFY | OG/Dolt authority contradiction. |
| OMP Gemini 3.5 Flash | VALIDATE | No blocker found. |

Cursor was excluded from this validation run after stalling in the earlier ordering panel. Raw transcripts were generated in `/tmp/choir-grand-suite-validation-consensus`; this document is the durable adjudicated record, not a claim that `/tmp` is resumable evidence.

## Adjudication

| Finding | Decision | Repair |
|---|---|---|
| Competing OG/Dolt execution authority | Confirmed | Converted its invocation/checkpoint to the grand-suite command; classified it as a subordinate contract in ACTIVE, mission graph, and authority manifest. |
| Product-completion stale resumption authority | Confirmed | Marked its checkpoint superseded and redirected its probe/goal to grand S4. |
| Ambiguous graph containment/dependency | Confirmed | Added `entrypoint`, `member_of`, `suite_phases`, and `execution_mode`; made the grand suite the sole suite orchestrator and removed misleading member prerequisite edges. |
| Delegated work not restart-durable | Confirmed | Added pre-dispatch mutation locks, staged durable slice transactions, independent-verifier identity, restart reconciliation, a required delegation-ledger schema, and phase-checkpoint stages. |
| S1 additions impossible to disposition in prior S0 | Confirmed | Added a bounded `s1_runtime_exception_disposition` table and independent rebaseline gate before S3. |
| S4 audited-computer boundary under-specified | Confirmed | Limited PC-5 consumption, required explicit ComputerVersion/materialization/equivalence evidence, and excluded route promotion, rollback, run truth, Wails, and post-gate product wiring. |
| Runtime extraction destination preselected | Confirmed as a risk | Made destination caller-graph-driven; `internal/agentcore` is only the default when S0 evidence supports it. |
| Seam-repair lifecycle stale in manifest | Confirmed | Marked settled. |
| Consensus artifacts referenced only through `/tmp` | Confirmed | Replaced suite refs with this durable record. |

## Iterative Validation And Repair

Four subsequent six-reviewer passes were used as adversarial refinement, not
vote-counting. Confirmed findings were repaired in place:

- competing Autopaper, seam-repair, product-completion, and OG/Dolt goal strings
  were demoted to the grand-suite command;
- graph membership, one-entrypoint semantics, B0 authority persistence, and
  subordinate status became machine-visible;
- delegation gained a Git-backed single-writer journal, expected-head CAS,
  orchestrator lease/epoch, nonce reconciliation, append-only stage history,
  isolated worker delivery, role separation, and idempotent external-effect
  receipt lookup;
- rollback became per accepted atomic landing rather than whole multi-landing
  S3 phase;
- S1 inherited the grand phase protocol and durable evidence rules;
- live documentation-citer extinction, S1 exception disposition, S2 ratchet
  evidence, and alias-window closure became explicit mechanical gates;
- S4 unpaused ten named PC-5 pre-wiring gates plus the Candidate Contract,
  attributed its operator/receipt surface to CLI-operability Phase 1, and
  retained promotion ownership in S7;
- the unratified phantom third Dolt route store was demoted in OG/Dolt itself;
  route-slot/receipt tables on corpusd with vmctl as sole CAS writer are the
  settled owner topology.

Raw refinement transcripts are session diagnostics only. The adjudications and
current Definition text are the durable evidence.

## Definition-Gate Result

**VALIDATE — high confidence (2026-07-11).**

The closure panel returned five unconditional `VALIDATE` verdicts (Codex,
Devin, OMP Gemini 3.5 Flash, OMP GLM-5.2, OMP GPT-5.5). OpenCode returned
`MODIFY → then VALIDATE` for one missing S4 subordinate-spec attribution. That
attribution was added, and `scripts/doccheck -mode live` passed afterward.

Adjudication: all material authority, durable-delegation, S1 exception,
route-topology, documentation-citer, bootstrap-CAS, and S4 boundary findings
are `repaired`. `open_findings` is empty. The definition gate is `passed`.
B0—not S0—is the next executable subgoal and must persist this authority diff
to `origin/main` before any code mutation.
