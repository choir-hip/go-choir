# Mission: Texture Hard Cutover v0 Ledger

## 2026-06-15 - Mission Creation

Claim: promoting Texture as the artifact control-plane ontology must be treated
as a hard cutover mission, not a passive docs rename.

Move: construct the initial paradoc and split the former single explanatory
draft into a short current orientation doc plus a historical/background doc.

Expected ΔV: establish the mission source program without decreasing execution
variant; V remains 10 because implementation and proof have not begun.

Actual ΔV: 0 execution obligations discharged; mission became resumable.

Receipts:
- `docs/why-texture-2026-06-15.md`
- `docs/why-texture-background-2026-06-15.md`
- `docs/mission-texture-hard-cutover-v0.md`

Open edge: the checker rule, repo-wide inventory, runtime rename, product proof,
transclusion proof, and protocol canonization remain future moves.

## 2026-06-15 - Problem Checkpoint: Retired-Name Inventory

Claim: the old V-name is system-wide current ontology residue, not a small
implementation alias, so the first admissible move is documentation and
checker-design evidence before behavior changes.

Move: probe plus documentation checkpoint. Ran read-only inventory commands and
captured the checker warning/allowlist design in the paradoc.

Expected ΔV: -1 by discharging the inventory/design obligation without touching
runtime behavior.

Actual ΔV: -1. V moves from 10 to 9. Runtime, prompts, frontend, tests, APIs,
schema, and tool affordances remain unmodified.

Receipts:
- `rg -l -i 'vtext|\.vtext|VText|VTEXT'` over the worktree: retired-name
  content in 172 docs files, 82 runtime Go files, 35 frontend source files,
  33 frontend tests, 9 store files, 9 runtime prompt files, 6 type files,
  4 command files, 2 spec files, and both root contracts.
- Retired-name path components: 44 docs paths, 22 runtime Go paths,
  18 frontend source paths, 16 frontend test paths, 2 store paths, 1 type path,
  1 runtime prompt path, and 1 command path.
- Selected affordance line counts: `/api/vtext` 505, `data-vtext` 604,
  `edit_vtext` 390, `request_super_execution` 122, V-name profile references
  417, `.vtext` 942, `vtext_` 658.
- `scripts/doccheck --report /tmp/choir-doccheck-report.md --json
  /tmp/choir-doccheck.json`: report-only complete, 212 docs, 803 warnings.

Open edge: the report-only checker rule is designed but not implemented; the
checkpoint still needs to be committed before runtime changes.
