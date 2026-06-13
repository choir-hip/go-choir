# Doctrine Conformance Update Report — 2026-06-13

## Scope

This report records the docs-only doctrine update pass that touched the
originally flagged doctrine-sensitive files plus one doctrine-surface
instruction file:

- `skills/parallax/SKILL.md`

No runtime behavior code or prompt-default routing text was changed.

## Validation

```text
rg --files -g '*.md' | wc -l => 186
post-edit doctrine term scan => 35 hit files / 151 no-hit files
```

The hit-file count did not drop because this pass preserved truthful historical
evidence and detector vocabulary. The objective here was not "zero terms"; it
was "stop these docs from re-normalizing retired ontology as current doctrine."

## Heresy accounting

- `discovered`: 0 new doctrine heresies in this edit pass
- `introduced`: 0
- `repaired`: the 35 originally flagged docs were all touched so that legacy
  terms now read as one of:
  - explicit retired vocabulary
  - historical evidence
  - transitional compatibility residue
  - active successor-scope cleanup item

Discovery from the earlier sweep is not re-counted as repair here. Repair in
this pass means the document now labels or routes the legacy language instead of
quietly inheriting it.

## Files updated in this pass

- `AGENTS.md`
- `README.md`
- `docs/README.md`
- `docs/choir-architecture-review-next-moves-2026-06-11.md`
- `docs/choir-deck-treatment-and-faq-2026-06-09.md`
- `docs/choir-doctrine.md`
- `docs/choir-master-spec-review-2026-06-13.md`
- `docs/choir-rearchitecture-durable-actors-2026-06-11.md`
- `docs/current-architecture.md`
- `docs/heresy-detectors.md`
- `docs/mission-agentic-debugging-vtext-stability-v0.md`
- `docs/mission-apps-and-changes-store-sweep-v0.md`
- `docs/mission-campaign-compiler-selfdev-v0.md`
- `docs/mission-choir-doctrine-upgrade-v0.md`
- `docs/mission-choir-grand-deformation-v0.md`
- `docs/mission-choir-in-choir-platform-pr-accelerator-v0.md`
- `docs/mission-global-wire-style-vtext-collaborative-storygraph-v0.md`
- `docs/mission-lifecycle-cutover-v0.ledger.md`
- `docs/mission-lifecycle-cutover-v0.md`
- `docs/mission-messaging-cutover-v0.ledger.md`
- `docs/mission-portfolio-2026-06-11.md`
- `docs/mission-source-system-simplify-secure-smart-v0.md`
- `docs/mission-surface-ontology-cleanup-h027-h029-v0.md`
- `docs/mission-vtext-client-ready-source-transclusion-pretext-v0.md`
- `docs/mission-vtext-source-viewer-reader-mode-hardening-v0.md`
- `docs/mission-web-surface-rationalization-v0.md`
- `docs/news-econ-publishing-synthesis-2026-06-04.md`
- `docs/overnight-vtext-super-console-zot-mega-report-2026-05-31.md`
- `docs/platform-os-app-state.md`
- `docs/project-goals.md`
- `docs/runtime-invariants.md`
- `docs/vtext-mission-current-system-hard-review-2026-06-06.md`
- `docs/vtext-regression-review-2026-05-31.md`
- `skills/parallax/SKILL.md`

## What changed

The edit pattern was intentionally narrow:

- current doctrine docs got precise wording fixes where legacy terms still read
  like endorsed target state;
- historical mission/review/report docs got short doctrine notes clarifying that
  Trace app, raw Terminal, Browser app, StoryGraph, and `continuation-level`
  language is preserved as evidence or transition residue, not current doctrine;
- source-opening docs were re-routed toward Source Viewer/reader artifacts
  first, with explicit Web Lens live/original inspection as the follow-on
  surface;
- acceptance-language docs were tightened so `continuation-level` reads as
  transitional residue rather than a permanent goal term.

## Remaining contradictions and deferrals

These remain real after the doc pass:

- `docs/platform-os-app-state.md` still truthfully records a shipped Features
  `Open Trace` stub. The doc now labels it as residue, but the product surface
  still exists. That is code-bearing cleanup.
- `docs/mission-vtext-client-ready-source-transclusion-pretext-v0.md`,
  `docs/mission-source-system-simplify-secure-smart-v0.md`, and related source
  missions still preserve `BrowserApp` as an implementation name in historical
  evidence. That is acceptable for now, but renaming/quarantining the code/test
  surface is a successor task.
- `continuation-level` remains present across cutover and acceptance docs as
  truthful transitional evidence residue. This pass did not and should not
  pretend those architecture/code dependencies are already gone.

## Next code-bearing paramissions

1. H027 cleanup: remove or replace the remaining Features `Open Trace` product
   stub. Concretely, audit `frontend/src/lib/FeaturesApp.svelte`,
   `frontend/src/lib/Desktop.svelte`, Features tests, and any copy that still
   presents "Open Trace" or "Trace UI" as a user action. The replacement should
   be one of: a trace-evidence/provenance action, a run-acceptance/evidence
   artifact link, or a Super Console diagnosis action. Non-goal: deleting
   Trace evidence APIs, trace moments, run bundles, or machine-readable
   evidence.
2. H029 cleanup: quarantine or rename remaining `BrowserApp` /
   `browser_sessions` implementation surfaces where they still leak into
   user-facing routes, tests, or copy. Concretely, classify
   `frontend/src/lib/BrowserApp.svelte`, `frontend/src/lib/apps/registry.ts`,
   browser-named Playwright specs, `internal/store/browser.go`,
   `internal/types/browser.go`, `internal/runtime/prompt_defaults/conductor.md`,
   and `browser_sessions` schema/table names as either hidden compatibility
   implementation names or rename candidates. The user-facing doctrine target
   is Source Viewer/reader artifacts first, then explicit Web Lens live/original
   inspection. Non-goal: changing routing prompts or breaking existing source
   opening, iframe fallback, Web Lens inspection, or publication-carried reader
   snapshots inside this doctrine landing commit.
3. M4 continuation deletion: finish the architectural removal/re-pointing so
   `continuation-level` can leave current acceptance language entirely.
