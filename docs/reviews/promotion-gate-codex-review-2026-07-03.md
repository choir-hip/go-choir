# Promotion Gate Codex Review — 2026-07-03

**Reviewer:** focused, read-only Codex reviewer  
**Scope:** `specs/promotion_protocol.tla`, `specs/promotion_protocol.cfg`, and the Mission S / promotion gate context in `docs/promotion-protocol-spec-staleness-and-redefinition-2026-07-03.md`, `docs/mission-suite-autoputer-autopaper-spec-first-v0.md`, and `docs/mission-suite-autoputer-autopaper-spec-first-v0.ledger.md` (Pass 1-7).  
**No code changes were made.**

---

## Verdict: approve with reservations

The spec is model-checked green (CI run `28648508586`, 826 states, 0 errors) and establishes the promotion gate as required by the mission suite. The safety invariants are coherent and the freshness-CAS/approval-gate/health-window shape matches the assessment doc. The reservations are about *completeness relative to the stated redefinition requirements*, not about the current model being wrong. Mission C should not encode the certificate or approval boundary until the spec captures them more precisely.

---

## Critical findings

None. TLC reports no errors for the current `.cfg`.

---

## Major findings

### 1. Promotion certificate is not modeled as a durable structured record
`docs/promotion-protocol-spec-staleness-and-redefinition-2026-07-03.md` (§Redefinition Requirements, item 3) requires the spec to model the promotion certificate as a durable record of fork point, candidate base, merge results, conflicts, verifier results, and route transition. The current spec only records `promoBase`, `candidateBase`, `promoActive`, `promoCandidate`, and `approved`/`poisoned`/`healthWindow` (`specs/promotion_protocol.tla`, lines 41-52). `CertificateCompleteness` (lines 356-359) only checks `promoBase >= 0` and `promoCandidate = c`, which is nearly tautological because `promoBase` already ranges over `0..MaxTailMoves` and `promoCandidate` is never reassigned after `ForkCandidate`. This invariant does not exercise the certificate concept as documented.

### 2. Owner approval is modeled as an internal system action, not as an external gate
`Approve(c)` in `specs/promotion_protocol.tla` (lines 163-169) is an ordinary `Next` action. The approval gate is enforced by `promoStatus = "approved"` and `approved[c] = TRUE` (lines 313-316), which is correct. However, the assessment doc frames approval as an *owner* action that is conceptually external to the autoputer. Treating it as a system step means the closed-world model can approve itself, so the spec does not prove that the system is safe *when an external owner is the only source of approval*. This is acceptable for a v1 gate, but it will need to be refined before Mission C encodes the real boundary between the autoputer and the owner/approval UI.

### 3. `Restage` is not weakly fair
`Fairness` (lines 382-390) includes `PrepareLedger`, `Verify`, `Commit`, `Abort`, `AutoRevert`, `ConfirmHealthy`, `ApplySecondary`, and `RollbackSecondary`, but it omits `Restage` (lines 139-148), `Approve` (lines 163-169), `MoveActiveTail` (lines 100-105), `ForkCandidate` (lines 110-122), `HealthCheckFail` (lines 240-247), and `PoisonedWrite` (lines 231-237). The omission of `Restage` is the most consequential: if `MoveActiveTail` advances the active base while a promotion is in `verified` or `approved`, `Restage` becomes enabled but is not guaranteed to fire. The promotion could sit stale in `verified`/`approved` forever, which contradicts the intent of the freshness-CAS story. While this is a liveness issue rather than a safety issue, the mission context explicitly says the spec must model active-computer mutation during candidacy and catch stale commits. The spec prevents stale commits but does not force stale promotions to be restaged.

### 4. Single-promotion-per-candidate model is implicit and limiting
`promoStatus`, `promoActive`, `promoCandidate`, `promoBase`, `approved`, `poisoned`, and `healthWindow` are all indexed by `CandidateComps` (lines 41-52), not by a distinct promotion ID. A candidate can only host one promotion at a time, and `ForkCandidate` requires `promoStatus[c] = "aborted"` (line 111). This is fine for a first gate, but it does not match the long-term ontology where a persistent candidate might participate in multiple promotion attempts over its lifetime. The limitation should be documented as a deliberate v1 simplification.

---

## Minor findings / nits

1. **Poisoned write does not close the health window explicitly.** `PoisonedWrite` (lines 231-237) only sets `poisoned' = TRUE`. It leaves `healthWindow[c] = "open"`. The model disables revert via the `poisoned[c] = FALSE` guard in `AutoRevert` (line 269), so the safety property holds, but it is conceptually clearer for the health window to transition to a closed/terminal state. `ConfirmHealthy` and `HealthCheckFail` both check `healthWindow[c] = "open"` and `poisoned[c] = FALSE` (lines 241-243, 253-254), so after poisoned the window is effectively closed by the conjunction of guards.

2. **`AutoRevert` leaves `healthWindow` as `"failed"`.** After `AutoRevert` (lines 266-281), `healthWindow` stays `"failed"`. This is harmless because the promotion is in `reverted` and no further health-window actions apply, but it is slightly odd that a reverted promotion's health window is not reset to `"open"` or a terminal state.

3. **`HealthCheckFail` and `ConfirmHealthy` can both be enabled at the same time.** When `promoStatus = "committed"`, `healthWindow = "open"`, `poisoned = FALSE`, and all ledgers are `applied`, both `HealthCheckFail` and `ConfirmHealthy` are enabled. TLC will explore both branches. This is intentional non-determinism, but it means the model does not distinguish "healthy" from "unhealthy" by any state predicate other than the external choice. This should be documented so readers do not assume the health check is deterministic.

4. **`ForkCandidate` cannot change a candidate's parent.** `ForkCandidate` requires `candidateParent[c] = a` (line 112) and leaves `candidateParent` unchanged (line 122). The initial `candidateParent` is chosen once in `Init` (line 86). Therefore each candidate can only ever fork from the active computer it was assigned at model initialization. If the intent is to model candidates being forked from *any* active computer, `ForkCandidate` should either set `candidateParent'` or `candidateParent` should not be part of the candidate identity.

5. **`.cfg` ledger names do not match the assessment doc ledger split.** `specs/promotion_protocol.cfg` (line 7) uses `Ledgers = {source, data, index}`. The assessment doc (§Redefinition Requirements, item 2) names `source_build`, `dolt_app`, `vm_os`, `blob_content`, `artifact_graph`, and `route_identity`. The generic names are not wrong for a model, but they should be documented or updated to reflect the intended ledger split.

6. **No sabotage/counterexample tests are visible in the review scope.** The assessment doc (§What the New Spec Will Prove) says dropping the freshness CAS, approval gate, etc. should reproduce known failure modes as short counterexamples. The review scope did not include any `.cfg` or scripts for sabotage variants. Before Mission C encodes the logic, it would be valuable to have at least one sabotage `.cfg` (e.g., one that drops the freshness check from `Commit`) that TLC flags with a counterexample.

---

## Questions for the author

1. Is the single-promotion-per-candidate limitation an intentional v1 simplification, or should the next revision introduce a distinct `Promotions` set indexed by promotion IDs?
2. Should `Approve` be modeled as an external action (e.g., an environment/owner process) rather than a system action, to prove the autoputer is safe *when it cannot self-approve*?
3. Should `Restage` be weakly fair so that stale `verified`/`approved` promotions cannot sit indefinitely?
4. Should `PoisonedWrite` explicitly set `healthWindow` to a closed/terminal state, or is the current `poisoned` flag the intended design?
5. How will the durable promotion certificate be encoded in Go? Will it be a new variable in the next spec revision, or will Mission C synthesize it from the existing `promoBase`/`promoActive`/`promoCandidate` fields?
6. Are there sabotage `.cfg` files or test scripts planned to verify the counterexamples listed in the assessment doc?

---

## Recommended next steps before Mission C encodes the promotion logic

1. **Promote the certificate to a real variable.** Add a `promoCertificate` variable (or equivalent) recording fork base, candidate base, merge results, conflicts, verifier results, and route transition. Strengthen `CertificateCompleteness` to assert that every committed-or-terminal promotion has a complete certificate.
2. **Clarify the approval model.** Decide whether `Approve` stays as a system action or becomes an external environment action. If it stays internal, document the closed-world assumption; if it moves external, add the owner/environment process to the spec.
3. **Add weak fairness to `Restage`.** This closes the liveness gap where a stale verified/approved promotion is never restaged.
4. **Add sabotage model-check variants.** Create `.cfg` files that deliberately weaken a guard (e.g., drop the freshness CAS from `Commit`) and confirm TLC produces a short counterexample. This satisfies the assessment doc's claim that the spec catches the known failure modes.
5. **Align ledger names or document the mapping.** Either update the `.cfg` to use the ledger names from the assessment doc or add a comment explaining that `{source, data, index}` is a generic stand-in for the six ledger types.
6. **Document the single-promotion-per-candidate assumption.** Add a note to `specs/README.md` or the spec header stating that v1 models one promotion per candidate and that multi-promotion candidates are future work.

---

*Review written to `/Users/wiz/go-choir/docs/reviews/promotion-gate-codex-review-2026-07-03.md` by the Codex reviewer. No files were modified.*
