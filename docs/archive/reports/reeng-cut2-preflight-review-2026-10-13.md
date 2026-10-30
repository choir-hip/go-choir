# Cut 2 pre-flight review, 2026-10-13

Convergent panel on Cut 2 at 76ab40fd with the pre-flight drift
(reeng-cut2-preflight-2026-10-13.md), the quote census
(reeng-c8-quote-census-2026-10-13.md) and eight orchestrator amendments.
Prompt `.agentic-consensus/cut2-preflight-prompt-2026-10-13.md`, manifest
`.agentic-consensus/cut2-preflight-2026-10-13/` (session diagnostics).
Answered: claude opus, devin, opencode, muse-spark, gemini-3.8-flash,
deepseek-v4.1-flash. codex, gpt-6.1-sol and gpt-6-luna hit usage limits;
glm-5.3-flash had not answered when this was written.

**Verdict: ready with amendments**, six of six. No blocking finding for
2a. Two findings bind before 2b (A6, and the opening-Cast schema).

| | amendment | ruling |
|---|---|---|
| A1 | matcher outcomes | **adopted with change.** Outcomes are matched (exact or tolerant), not found, and not captured. Not captured has its own badge ("unread source"), distinct from pending, and is never memoized: it is recomputed when a capture of the cited page lands (claude, deepseek). |
| A2 | Read before cite | **adopted with change.** Research's prompt teaches it, with no runtime scoring. Texture's Write refuses, synchronously and line-located, a `{{cite}}` handle that names no captured block. That is a shape check like C1's typed bodies, so a snippet quote cannot reach a published page (devin, deepseek). The tracer and live receipts report the share of citations not captured (claude). |
| A3 | table rows | **adopted.** A row is a block of trimmed cells joined by " | ". The header row and the caption are their own blocks. The join lives in the extractor and normtext is shared. |
| A4 | ellipsis quotes | **adopted, strict.** Segments match in order inside one block, because a Ref names one cid (claude, devin, gemini). muse-spark and deepseek would allow cross-block matches; that is rejected for now. A cross-block ellipsis quote is refused at Write with a message to split it into two cites. The census rerun counts these misses. This is a recorded divergence from mike's cascade and goes in the freeze receipt. |
| A5 | captures layer | **adopted with change.** The layer carries URL, extractor version, doc_hash, block count, raw-bytes presence and each block's cid and normtext, served through an owner-scoped product route. Ids and counts alone cannot rerun the census (claude, muse-spark, deepseek). |
| A6 | how management opens a story | **decided at 2b under a pinned constraint.** Majority (devin, gemini, deepseek, muse-spark, opencode): the ProductAPI prompt-bar route. Claude's code-cited objection stands as the constraint: `EnsureTextureHandoff` mints revision 1 as AuthorUser with management's text, mints a work item, and starts a Texture run that races the opening Cast. Opening a story must create the document with no owner-authored revision, no work item and no racing run, and record desk origin. If the prompt-bar route can be changed to that without a second creation path, it is used; otherwise a `texture:new` Cast creates it in the reducer. |
| A6' | opening-Cast schema | **adopted (deepseek, blocking for the freeze).** The opening Cast's fields (hypotheses, capture refs, publish auto) are defined in 2a and enter the freeze receipt, so 2b changes no record schema after the freeze. A 2a tracer scenario exercises management opening a story. |
| A7 | autonomous Universal Wire path | **adopted.** Texture's Write (iv) drops the commit-time hook (`tools_texture.go:455`) deliberately. The rest of the cross-section (proxy route, runtime field and config, wire_platform_publish.go, wire_publication.go, wirepublish, tests, nix, prompts, specs) is deleted in 2b before the paper computer's first schedule run, behind a same-commit grep gate. |
| A8 | landing order | **adopted with change** (every answer). (i) normtext, quotematch and Ref landed first (b5059bd9). Then (ii) block captures and Read run in parallel with (iii) async search. (iv) Write follows (ii); (v) support judgement follows (iv); (vi) the translation contract can go anytime. Integration on the shared module-set and surface files (`yaegikernel/choir.go`, `surface.go`, the parity test) is serialized, and deploys and tracers run one at a time under `--expect-sha`. |

## Risks carried into the plan

- Management load in the live window: one wake per poll with captures,
  plus one per Texture commit with unjudged pairs. The per-interval
  latency is reported and read from the trace.
- Real outlets may extract worse than the census's 91% upper bound, so
  (ii)'s tracer reads a real outlet page, not only the fixture.
- Search streams cancelled in release or reclaim while a cell is mid-Get
  need a race test.
- A crash early in the live window stalls the paper silently until the
  owner restarts it, which is the rule. The live receipt names the time
  of any crash.
- choir.Research's all-children barrier stays on C2's research path until
  C1e and may delay the first Report.
- The broker handle's lifetime was fixed in C0 (287734c5, reissue at
  every cell); the live window re-checks it.
