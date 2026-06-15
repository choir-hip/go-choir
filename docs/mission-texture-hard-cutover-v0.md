# Mission: Texture Hard Cutover v0

## Summary

Texture is the promoted ontology and product language for Choir's versioned,
transclusive artifact control plane. The old V-name is now migration residue,
not current doctrine.

This mission is not a cosmetic rename. It is an ontology cutover. The codebase,
prompts, UI, APIs, tests, docs checker, high-read docs, and product-path proofs
must teach the same object: Texture as the artifact layer that turns autonomous
activity into directed results and compounding learning.

The protocol spec is deliberately not written first. The mission must make the
product path work, delete accidental complexity, prove the minimal surface, and
only then canonize a Texture Protocol v0.

## Source Documents

- [why-texture-2026-06-15.md](./why-texture-2026-06-15.md)
- [why-texture-background-2026-06-15.md](./why-texture-background-2026-06-15.md)
- [choir-doctrine.md](./choir-doctrine.md)
- the M3.4 first-draft regression paradoc linked through
  [mission-graph.yaml](./mission-graph.yaml)
- [mission-portfolio-2026-06-11.md](./mission-portfolio-2026-06-11.md)

## Problem

The system currently carries a split ontology. The product object has outgrown
its old internal name, but code, prompts, docs, tests, route names, tool names,
and acceptance language still teach the old object. That split invites shallow
patches: route fixes that preserve wrong concepts, prompt fixes that encode
workflow decisions, and docs that describe a control plane while the runtime
still names it like an internal text widget.

The current urgent regression is also a warning. A prompt can open the artifact
surface but fail to create the first useful revision. That failure is easier to
miss when acceptance overweights route topology and underweights browser-driven
proof of the actual artifact loop.

## Problem Checkpoint: Retired-Name Inventory

Mutation class: `green` documentation and evidence only. No runtime behavior,
schema, API, prompt default, UI, or test surface changed in this checkpoint.

Read-only search on 2026-06-15 confirms that the old V-name is not isolated
implementation residue. It is still the dominant artifact-control-plane name
across current docs, runtime, frontend, tests, API routes, data attributes,
tool names, prompt defaults, and storage identifiers.

Receipts:

- `rg -l -i 'vtext|\.vtext|VText|VTEXT'` over the worktree found retired-name
  content in 172 docs files, 82 runtime Go files, 35 frontend source files,
  33 frontend tests, 9 store files, 9 runtime prompt files, 6 type files,
  4 command files, 2 spec files, and both root contracts.
- The same inventory found retired-name path components in 44 docs paths,
  22 runtime Go paths, 18 frontend source paths, 16 frontend test paths,
  2 store paths, 1 type path, 1 runtime prompt path, and 1 command path.
- Selected affordance line counts: `/api/vtext` 505, `data-vtext` 604,
  `edit_vtext` 390, `request_super_execution` 122, V-name profile references
  417, `.vtext` 942, and `vtext_` 658.
- `scripts/doccheck --report /tmp/choir-doccheck-report.md --json
  /tmp/choir-doccheck.json` completed in report-only mode with 212 docs and
  803 warnings before any Texture-specific checker rule was added.

Checker warning design:

- Add a report-only Texture retired-name warning family to `cmd/doccheck`
  rather than failing docs-only CI in the same pass.
- Scan current and mixed non-evidence docs plus code/prompt/frontend/test
  surfaces for retired-name terms: `VText`, `vtext`, `.vtext`, `/api/vtext`,
  `data-vtext`, `edit_vtext`, and `vtext_`.
- Treat `docs/why-texture-background-2026-06-15.md` as the standing historical
  background allowlist entry.
- Allow historical mission/evidence occurrences only when the manifest marks
  the file `claim_scope: historical` or `is_evidence: true`, or when a mixed
  mission line explicitly labels the occurrence as historical evidence,
  retired-name evidence, migration residue, or a deletion target.
- Current docs, prompts, UI labels, tests, API affordances, storage-facing names,
  and tool names should warn until renamed or explicitly classified as temporary
  cutover residue with a deletion receipt. Warning silence is not settlement;
  final settlement still requires the retired-name search to show only allowed
  historical/background occurrences.

## Non-Goals

- Do not write a full protocol cold.
- Do not preserve compatibility aliases as indefinite dual paths.
- Do not implement semantic phrase matching in runtime to make the cutover pass.
- Do not weaken docs-only CI filters.
- Do not resume M3 or source/news work until the core prompt-bar artifact loop
  has product-path proof under the Texture ontology.

## Parallax State

status: open_handoff

mission conjecture: if Choir hard-cuts the artifact control-plane ontology to
Texture across docs, code, prompts, UI, tests, tool names, acceptance, and
checker warnings, while preserving the core prompt-bar -> conductor -> Texture
revision loop under deployed product proof, then the M3 lifecycle portfolio can
resume from a cleaner ontology with less route confusion and fewer hidden
workflow gates.

deeper goal (G): make Texture the stable semantic substrate for directing
autonomous results and compounding learnings, so safe self-development,
source/news articles, style, research, super evidence, and future media
projections all share one artifact-native control plane.

witness/spec (A/S):
- replace current user-facing, agent-facing, code-facing, and docs-facing uses
  of the retired V-name with Texture;
- preserve historical explanation only in
  `docs/why-texture-background-2026-06-15.md` and explicitly historical
  mission evidence;
- repair or preserve the deployed prompt-bar -> conductor -> Texture first
  revision loop;
- split the overloaded edit affordance into a common patch tool and an
  exceptional whole-document recovery rewrite, unless investigation proves a
  smaller surface is clearer;
- add report-only docs checker coverage for retired-name drift and later
  promote it to CI failure after the warning baseline is burned down;
- canonize `docs/texture-protocol-v0.md` only after implementation proof shows
  the minimal protocol surface.

invariants / qualities / domain ramp (I/Q/D):
- I: Texture owns canonical artifact meaning and learning; super owns
  privileged execution.
- I: among agents, one Texture writer writes canonical Texture state; other
  agents produce evidence, proposals, receipts, faults, diffs, source packets,
  and promotion claims.
- I: human direct edits remain canonical revisions.
- I: every Texture version is immutable, addressable, comparable, restorable,
  and forkable.
- I: transclusions pin version refs by default and the UI shows when newer
  versions exist.
- I: runtime protects mechanical invariants, not semantic decision trees.
- I: no indefinite dual path. Compatibility shims, if unavoidable for one
  deploy, must have deletion receipts before settlement.
- Q: names should teach distributional expectations. The common edit tool
  should sound common; the whole-document rewrite tool should sound
  exceptional.
- Q: product proof must use browser/computer-driven interaction on staging, not
  only API probes or local tests.
- D ramp: docs and detector warnings -> focused local tests -> staging deploy
  identity -> browser product proof -> protocol canonization.

variant (ranking function) V: current V=6; last ΔV=-1:
1. discharged: old-name inventory across code, docs, prompts, API routes,
   database tables, frontend labels, tests, scripts, and checker manifests is
   documented in the Problem Checkpoint above;
2. discharged: docs checker retired-name warning rule is implemented in
   report-only mode as H5 with the documented allowlist;
3. current V includes: high-read doctrine and index docs still require Texture
   reconciliation; the operating contract and Texture invariant doc have been
   renamed/reworded, but doctrine/README/mission graph still carry H5 warnings
   for live old filenames and detector symbols;
4. current V includes: internal runtime, storage, file, UI data-attribute, and
   compatibility-shim symbols still use the old ontology; product-facing API
   routes and tool affordance names now expose Texture locally;
5. current V includes: UI labels and browser proof still target the old
   artifact name;
6. current V includes: edit affordance surface still needs common-vs-
   exceptional naming proof;
7. discharged for local scope: prompt register and registered tool names now
   use Texture-oriented wording and `edit_texture` /
   `record_texture_decision` affordances without adding runtime semantic
   decision trees;
8. discharged for the current product-facing slice: deployed prompt-bar ->
   conductor -> Texture first-revision proof passed under `/api/texture` and
   `edit_texture`, with no super-before-Texture trace;
9. current V includes: transclusion pinned-ref plus newer-version indicator
   proof is not yet present;
10. current V includes: Texture Protocol v0 is intentionally unwritten until
    the working minimal surface is proven.

budget: one broad red-surface cutover mission before M3 resumes. If the rename
reveals a distinct product regression, split the regression into a child
paradoc only after documenting it here.

authority / bounds: mutation class target is `red`; this document creation is
`green`. Protected surfaces for execution: canonical artifact writes, prompt
bar routing, conductor route materialization, Texture prompts/tools, Trace and
acceptance projection, UI labels, docs checker, deployment routing, and any
database migrations. Apply Problem Documentation First before behavior fixes.

evidence packet:
- retired-name inventory and allowlist;
- docs checker report with new warning family in report-only mode;
- focused tests for route, tool, prompt, and revision behavior;
- local sharded runtime tests when runtime changes land;
- pushed commits with CI run ids;
- Node B staging deploy identity for behavior-changing commits;
- browser/computer-use proof of prompt-bar submission creating a Texture,
  non-empty first appagent revision, history navigation, sources panel, and no
  super-before-Texture route;
- proof of pinned transclusion with newer-version indicator, or an explicit
  blocker if the UI surface is absent;
- final retired-name search showing only allowed historical/background
  occurrences;
- protocol v0 created only after the preceding proof.

heresy delta: discovered: the old ontology is now visible as a system-wide
drift source rather than a harmless implementation name. Introduced: none
accepted. Repaired target: delete dual-path naming, direct-super ingress
ambiguity, workflow-forcing prompts, and overloaded edit affordances where this
mission proves them.

position / live conjectures / open edges:
- C1 active: the hard rename is a vocabulary shift that should change route
  choice and acceptance quality, not just labels.
- C2 active: a common patch tool plus an exceptional whole-document recovery
  rewrite will better orient the Texture writer than one overloaded edit tool.
- C3 supported for report-only scope: the docs checker now carries H5
  retired-name warnings without failing docs-only CI. Current baseline:
  `scripts/doccheck --report /tmp/choir-doccheck-report.md --json
  /tmp/choir-doccheck.json` reports 1,155 total warnings, including 352 H5
  file-level warnings across AGENTS.md, README.md, cmd, docs, frontend,
  internal, and specs. Promotion to fail-closed remains future work after the
  baseline burns down.
- C4 active: some old mission docs may be cheaper and clearer to delete or
  leave only in git history than to rewrite under the new ontology.
- C5 active: protocol design before proof risks cathedral-building. The
  protocol should be the last deliverable, distilled from the working minimal
  surface.
- C6 supported for deployed product-route scope: `/api/texture` is registered
  and exercised by focused tests, frontend API callers, and staging
  Playwright product proof. `/api/vtext` remains an explicitly temporary
  compatibility shim; this does not authorize shim deletion.
- C7 repaired and CI-green: CI exposed a Universal Wire publication compatibility
  regression. The route/tool slice made new Texture revisions write
  `source=edit_texture`, but the `internal/wirepublish` autonomous publication
  eligibility package still accepted only the retired edit-source metadata.
  Result: runtime shards 2 and 3 failed before staging deploy, with missing
  edition transclusion and missing in-flight publication work item evidence.
  The repair accepts current Texture metadata plus deletion-receipted legacy
  metadata in the wire publish/read predicates; the rerun passed CI and staged.

next move: continue the broader UI/internal symbol cutover and the
transclusion pinned-ref/newer-version proof before protocol v0.

ledger file: `docs/mission-texture-hard-cutover-v0.ledger.md`

version / lineage: spawned from M3.4 readiness review and the 2026-06-15
Texture rename discussion. Blocks M3 until either settled or explicitly scoped
as a narrower dependency.

learning state: Texture exists to direct results with autonomy and facilitate
learnings. The rename must preserve that reason, not collapse into branding or
API churn.

settlement: settled only when the repo has no non-allowed retired-name
occurrences, Texture docs and doctrine agree, warning-only checker coverage is
landed, deployed product proof shows the core Texture revision loop, the
transclusion UI rule is proven or blocked with a successor, and a minimal
Texture Protocol v0 is canonized from the working surface.

## Suggested Goal String

```text
Use Parallax on docs/mission-texture-hard-cutover-v0.md. Treat it as the source
program for the Texture hard cutover before M3 resumes. Texture is the promoted
ontology for Choir's versioned, transclusive artifact control plane; the old
V-name is migration residue allowed only in the historical background doc and
explicit historical mission evidence. Current status is open_handoff with V=6.
The read-only retired-name inventory, Problem Documentation First checkpoint,
report-only H5 docs checker, operating-contract Texture reconciliation, and a
deployed product-facing route/tool/prompt slice are landed. Continue renaming
docs/code/prompts/UI/tests/tool affordances toward Texture, preserve one Texture
writer among agents, keep human direct edits canonical, keep super downstream of
Texture for privileged execution, and avoid runtime semantic decision trees.
Transclusions should pin version refs by default and show newer versions when
available. Do not canonize a Texture Protocol upfront; make protocol v0 the last
deliverable after the working minimal product surface is proven. Append moves
to docs/mission-texture-hard-cutover-v0.ledger.md and settle only with CI,
staging identity, deployed acceptance, retired-name search receipts, checker
report, and a minimal protocol distilled from proof.
```
