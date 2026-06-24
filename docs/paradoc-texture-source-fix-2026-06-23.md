# Texture Source Citation and Style Fix

## Suggested Goal String

```text
/goal Run docs/paradoc-texture-source-fix-2026-06-23.md as a Parallax mission: remove source_embed, add expanded_ref to source_ref, add mark_source_unused, remove the WireTexture prompt branch (preserving {{else}} research guidance as unconditional text), register style textures as toolbar-only source entities, remove negative prompt phrasing, and update core docs (AGENTS.md, choir-doctrine.md, texture-agentic-invariants, runtime-invariants, platform-os-app-state) in the same PR. Hard cutover, no migration, no users yet. Orange mutation class; protected surfaces: Texture canonical writes, prompt generation, tool schemas. Rollback: revert PR. Invariants: tri-state source handling (cited, toolbar-only, marked-unused), no prompt control flow, display_mode string enum (numbered_ref, expanded_ref), core docs update with code. Design doc: docs/design-source-ref-expansion-2026-06-23.md. Prompt revisions: docs/prompt-revisions-needed-2026-06-23.md. Ledger: docs/paradoc-texture-source-fix-2026-06-23.ledger.md. Settle when staging QA shows inline citations and article-format prose, grep clean for source_embed and WireTexture, all tests pass, and the PR is landed through the landing loop.
```

## Design References

- `docs/design-source-ref-expansion-2026-06-23.md` — full design document with
  schema, tool, renderer, prompt, and test change specifications.
- `docs/prompt-revisions-needed-2026-06-23.md` — prompt overlay revision plan.
- `docs/triage-next-steps-2026-06-23.md` — triage context (Priority 1).

## Parallax State

status: blocked
mission conjecture: if the source_embed removal and source_ref expansion
satisfy the staging witness under the tri-state source invariant and the
no-prompt-control-flow invariant, then the deeper goal of style-shaped
Texture writing is materially advanced.
deeper goal (G): a Texture system where the model's writing behavior is shaped
by style texture source entities in the run context, not by prompt control-flow
branches. Simpler, more general, more honest.
witness/spec (A/S): a Texture document on staging with inline source_ref
citations, article-format prose, expanded_ref display mode, no source_embed
references in the codebase, no WireTexture branch, style texture in toolbar
sources tab, mark_source_unused for immaterial sources.
invariants (I):
  1. Tri-state source handling: every source entity is cited (source_ref in
     body), toolbar-only (style texture), or marked-unused
     (mark_source_unused with rationale in revision metadata). No source is
     silently ignored.
  2. No prompt control flow for product behavior: prompts provide data and
     invariants, not boolean branches. Unconditional text is not control flow.
  3. Canonical meaning is Texture-owned: model priors are not evidence.
  4. Reader toggle: expanded_ref and numbered_ref are reader-toggleable display
     modes on the same source_ref node. Toggle is local UI state, does not
     mutate canonical document.
  5. Style texture before branch removal: styles/default.style.texture must
     exist and be registered as a source entity before the WireTexture branch
     is removed.
  6. {{else}} content preserved as unconditional: research guidance becomes
     unconditional text. Only the {{if}}/{{else}}/{{end}} wrapper and
     Wire-specific text are removed.
  7. display_mode is a string enum: "numbered_ref" | "expanded_ref". Not
     boolean.
  8. Media source display: style texture decides. Not a schema decision.
  9. Core docs update with code: AGENTS.md, choir-doctrine.md,
     texture-agentic-invariants-2026-06-13.md, runtime-invariants.md,
     platform-os-app-state.md updated in the same PR.
qualities (Q): article-format prose, not Q&A; inline citation points, not
block quote cards; grep clean for source_embed and WireTexture; all tests
pass; landing loop complete.
domain ramp (D): local unit tests → frontend tests → staging QA. No fake
island — staging is the acceptance environment.
variant (V):
  V = source_embed references in codebase (current: ~30+)
    + source_ref display_mode enum items missing (current: 1, expanded_ref)
    + WireTexture branch references (current: 2, run_system.yaml + tool_profiles.go)
    + mark_source_unused operation missing (current: 1)
    + style texture source entity registration missing (current: 1)
    + negative prompt phrasing instances (current: ~5)
    + core docs not updated (current: 5)
    + prompt control-flow branches in scope (current: 1, WireTexture branch)
  current V ≈ 46; target V = 0
budget: 1 focused implementation session / 1 PR. Not overnight. Solvency: V=46
fits one session if batched (route is unambiguous, all constructs foreseeable).
authority / bounds:
  Edit: internal/texturedoc/schema.go, projection.go, tools_texture.go,
    run_system.yaml, revision_source_entities_intro.yaml, tool_profiles.go,
    tools_coagent.go, frontend rendering files, internal/types/texture.go,
    styles/default.style.texture (new), core docs (AGENTS.md,
    choir-doctrine.md, texture-agentic-invariants-2026-06-13.md,
    runtime-invariants.md, platform-os-app-state.md).
  Do not edit: revision_policy.yaml, revision_worker_findings.yaml
    (follow-up PR). Do not edit other run_system.yaml control flow beyond
    WireTexture branch (follow-up).
mutation class / protected surfaces: orange (runtime behavior, product APIs,
  app state). Protected: Texture canonical writes, prompt generation, tool
  schemas. Rollback: revert PR. No data migration (hard cutover, no users).
evidence packet: unit tests (schema, tools, projection), frontend tests
  (expanded_ref rendering, source toolbar), prompt generation tests (no
  source_embed string), staging QA (inline citations, article format), grep
  clean for source_embed and WireTexture, landing loop proof.
heresy delta: discovered (source_embed as citation crutch, WireTexture as
  missing-style workaround, prompt control flow as antipattern); introduced:
  none expected; repaired: source_embed removal, WireTexture removal, prompt
  flattening.
position / live conjectures / open edges:
  Position: code landed (078f7018), CI green, staging deployed. Staging QA
  revealed: article-format guidance works (no Q&A), but source entities are
  not being minted from research results. source_entities_len=0,
  source_ref count=0. Model named sources in prose (Reuters, BBC) but never
  called insert_source_ref. Grep clean for source_embed and WireTexture.
  Can now see staging model behavior. The gap is in source entity minting,
  not in prompt guidance or schema.
  Conjecture 1 (weakened): unconditional article-format guidance produces
    article-format output (supported on staging). But the bridge does not
    produce source citations — the model needs source entities in the run
    context to cite them. The style texture registration deferral is not the
    only gap; research result → source entity minting is also missing.
  Conjecture 2 (untested): expanded_ref not tested because no source_ref
    was produced.
  Conjecture 3 (untested): mark_source_unused not tested because no source
    entities were stored.
  Conjecture 4 (supported): {{else}} research guidance as unconditional text
    did not cause non-Wire regression. Research ran successfully (16 results).
  Open edge: why are research results not minted as source entities? Is the
    minting code path broken, or was it never wired for this flow?
next move: investigate root cause — why are research results not converted
  to source entities in the Texture revision? Check
  internal/runtime/texture_evidence_sources.go, tools_worker_update.go,
  and the coagent revision flow. The model cannot cite sources that are not
  registered as source entities. Predicted ΔV = unknown until root cause
  identified.
ledger file: docs/paradoc-texture-source-fix-2026-06-23.ledger.md
version / lineage: spawned from triage session 2026-06-23. Parent:
  docs/triage-next-steps-2026-06-23.md (Priority 1). Related:
  docs/worktree-review-2026-06-23.md (worktree #5).
learning state:
  - WireTexture branch was a workaround for missing default style guidance.
  - Prompt control flow is an antipattern: behavior depends on hidden runtime
    conditions rather than data in run context.
  - Model defaults to Q&A when no style guidance is present. Context issue,
    not capability issue.
  - source_embed and source_ref were redundant; display mode flag collapses
    them.
  - Qdrant and object service prototypes are not blocking. Deferred.
settlement: staging QA shows inline citations and article-format prose; grep
  clean for source_embed and WireTexture; all tests pass; PR landed through
  landing loop (commit → push → CI → staging → verify).
