# Instruction Substrate Prune — doccheck receipt (2026-08-11)

Mutation class: orange for `cmd/doccheck`; green for source-document wording.
This receipt records the signal correction and every baseline H-rule finding before
beads removal, corpus filtering, and detector-context repairs.

## Same-run comparison

Both runs used `go run ./cmd/doccheck --mode=full`.

| Run | Corpus | Summary | H findings |
| --- | ---: | --- | ---: |
| Baseline | 385 docs | 143 findings: 31 `warning`, 112 `info` | 31 |
| Corrected | 315 tracked docs | 38 findings: 0 `warning`, 38 `info` | 0 |

The corrected command prints `0 warnings, 38 info findings`; the Markdown report
contains separate `Findings` and `Warnings by severity` lines. The 70-document
difference is the untracked/ignored material excluded by the tracked-corpus
enumeration; no tracked Markdown document is omitted. The remaining `R3` info
findings are collection diagnostics, not actionable warnings.

## Baseline H-rule dispositions

Each row is one baseline warning. `H1` document scanning now uses only retired
prose/residue/framing families. Runtime identifiers remain covered by the code
heresy scan; they are not banned vocabulary when quoted in a Definition or
contract explanation.

| # | Finding | Disposition and authority |
| ---: | --- | --- |
| 1 | H1 `docs/choir-prompting-invariants.md:69` `update_id` | Detector over-match: runtime-owned identity field, governed by Doctrine I2c. Excluded from prose terms; runtime/code coverage remains. |
| 2 | H1 `docs/choir-prompting-invariants.md:71` `update_id` | Same runtime-schema disposition as row 1; no source claim was deleted. |
| 3 | H1 `docs/choir-prompting-invariants.md:103` `update_id` | Same runtime-schema disposition as row 1; Doctrine I2c remains the authority. |
| 4 | H1 `docs/definitions/choir-run-deploy-unblock-2026-07-11.md:41` `lease` | Source wording repaired to “time-bound admission locks”; H019 is preserved without retired vocabulary in the live claim. |
| 5 | H1 `docs/definitions/choir-run-deploy-unblock-2026-07-11.md:66` `lease` | Source wording repaired to “time-bound-control-named identifiers”; H019 remains explicit. |
| 6 | H1 `docs/definitions/choir-seam-repair-2026-07-10.md:129` `route_profile` | Detector over-match: implementation route identifier quoted in a Definition, not a retired prose ontology. Runtime/code coverage remains. |
| 7 | H1 `docs/definitions/choir-seam-repair-2026-07-10.md:134` `route_profile` | Same implementation-identifier disposition as row 6. |
| 8 | H1 `docs/definitions/choir-seam-repair-2026-07-10.md:135` `route_profile` | Same implementation-identifier disposition as row 6. |
| 9 | H1 `docs/definitions/choir-seam-repair-2026-07-10.md:655` `route_profile` | Same implementation-identifier disposition as row 6; route authority remains in the cited Definition. |
| 10 | H1 `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md:400` `lease` | Source wording repaired to “enter the materialization window”; the active effects Definition no longer uses H019 lease control vocabulary. |
| 11 | H1 `docs/definitions/og-dolt-heresy-completion-2026-07-08.md:152` `UniversalWirePlatformOwnerID` | Detector over-match: code identity symbol in a historical authority receipt. Runtime/code coverage remains. |
| 12 | H1 `docs/definitions/og-dolt-heresy-completion-2026-07-08.md:154` `route_profile` | Detector over-match: implementation route identifier in a historical receipt. |
| 13 | H1 `docs/definitions/og-dolt-heresy-completion-2026-07-08.md:159` `route_profile` | Same implementation-identifier disposition as row 12. |
| 14 | H1 `docs/definitions/og-dolt-heresy-completion-2026-07-08.md:251` `DoltPromotionAdapter` | Detector over-match: deleted adapter symbol cited as code evidence; code detector remains the enforcement seam. |
| 15 | H1 `docs/definitions/og-dolt-heresy-completion-2026-07-08.md:259` `route_profile` | Same implementation-identifier disposition as row 12. |
| 16 | H1 `docs/definitions/og-dolt-heresy-completion-2026-07-08.md:395` `route_profile` | Same implementation-identifier disposition as row 12. |
| 17 | H1 `docs/definitions/og-dolt-heresy-completion-2026-07-08.md:667` `requiredContinuationAfterTextureEdit` | Historical detector reference is explicitly labeled in the source; H3 is likewise context-qualified. The code detector still covers the symbol. |
| 18 | H1 `docs/definitions/og-dolt-heresy-completion-2026-07-08.md:880` `route_profile` | Same implementation-identifier disposition as row 12. |
| 19 | H1 `docs/definitions/og-dolt-heresy-completion-2026-07-08.md:916` `DoltPromotionAdapter` | Same deleted-adapter evidence disposition as row 14. |
| 20 | H1 `docs/texture-agentic-invariants-2026-06-13.md:225` `update_id` | Detector over-match: runtime-owned update identity, governed by Doctrine I2c. |
| 21 | H1 `docs/texture-agentic-invariants-2026-06-13.md:276` `update_id` | Same runtime-schema disposition as row 20. |
| 22 | H1 `docs/texture-agentic-invariants-2026-06-13.md:313` `initialTextureToolChoice` | Historical detector finding is labeled in the source; the implementation symbol remains code-scanned. |
| 23 | H1 `docs/texture-agentic-invariants-2026-06-13.md:339` `initialTextureToolChoice` | Historical detector requirement is labeled in the source; the implementation symbol remains code-scanned. |
| 24 | H2 `docs/definitions/og-dolt-heresy-completion-2026-07-08.md:85` `cannot fail` | Source wording repaired to “without a failure mode”; the detector contract remains unchanged. |
| 25 | H3 `docs/definitions/og-dolt-heresy-completion-2026-07-08.md:667` | Historical detector finding is explicitly labeled; this is a receipt of a retired forcing path, not a live workflow prescription. |
| 26 | H3 `docs/definitions/og-dolt-heresy-completion-2026-07-08.md:758` | Source field is labeled `historical detector reference`; the deletion and deployed proof remain citable. |
| 27 | H3 `docs/texture-agentic-invariants-2026-06-13.md:313` | Source heading is labeled `Historical detector finding`; intended invariant and required tests remain intact. |
| 28 | H3 `docs/texture-agentic-invariants-2026-06-13.md:339` | Source bullet is labeled `Historical detector requirement`; no agency invariant was weakened. |
| 29 | H5 `docs/evidence/audited-construction-phase-a-2026-07-16.md:28` | Historical evidence is not rewritten. H5 now allows the explicitly historical evidence corpus, while current implementation surfaces remain scanned. |
| 30 | H5 `docs/runtime-dissolution-inventory.yaml:3367` | The inventory is a historical disposition ledger. H5 now recognizes that named historical inventory, without allowing current source paths. |
| 31 | H5 `internal/modelpolicy/model_policy_test.go:73` | Test-only negative coverage intentionally names the retired predecessor. H5 allows `*_test.go`; production code remains scanned. |

## Detector-strength check

The correction does not delete H-rule families or their code scan. It separates
prose vocabulary from implementation symbols, labels historical detector
receipts, and adds only evidence/test path context already required by the
source documents. The corrected full-corpus run reports zero warnings against
tracked files. Doctrine invariants `I1`–`I16` and semantic laws `SEM-01`–`SEM-09`
were not edited.
