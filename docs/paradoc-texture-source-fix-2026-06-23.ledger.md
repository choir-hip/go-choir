# Ledger: Texture Source Citation and Style Fix

## Pass 0 — 2026-06-23

claim: source_embed removal and source_ref expansion fix the core product
regression (Q&A format, block quote citations). scope: schema, tools, prompts,
rendering, tests, core docs.
move: construct (design docs + cognitive transforms + paradoc compilation).
expected ΔV: 0 (planning pass, no code changes).
actual ΔV: 0.
receipt: docs/design-source-ref-expansion-2026-06-23.md,
  docs/prompt-revisions-needed-2026-06-23.md,
  docs/paradoc-texture-source-fix-2026-06-23.md.
edge: staging model behavior unobservable until deployed.

## Pass 1 — 2026-06-24

claim: staging QA will show inline source_ref citations and article-format
prose. scope: deployed commit 078f7018 on choir.news.
move: probe (staging acceptance proof — create Texture doc, request revision,
verify output).
expected ΔV: 46 → 0 (settle).
actual ΔV: 46 → 42 (grep clean for source_embed and WireTexture = 4 items
resolved; source entity minting still broken = 42 items remaining).
receipt:
  staging identity: PASS — 078f7018 deployed 2026-06-24T02:49:52Z.
  texture doc: c2ea5c4f-8f67-4729-9a66-3071ced22c61.
  revision: d5ffc87b-82e0-4944-b9aa-94bd043c89e3 (v2).
  source_entities_len: 0. source_ref count: 0. expanded_ref count: 0.
  article format: PASS (coherent article, not Q&A).
  grep source_embed: PASS (only guard tests/comments).
  grep WireTexture: PASS (only unrelated function names/test comments).
  screenshots: /tmp/choir-staging-source-proof-2026-06-24T03-06-04-220Z/
edge: research results (16 web search hits) not minted as source entities.
  Model named sources in prose but never called insert_source_ref. Root cause
  unknown — minting code path may be broken or unwired for this flow.
  Conjecture 1 weakened: article-format bridge works, but source citation
  requires source entities in run context, not just prompt guidance.
