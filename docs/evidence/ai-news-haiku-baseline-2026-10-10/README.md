# AI news research baseline: five Claude Haiku researchers (2026-10-10)

The comparison baseline for the research-only demo, "Research the news on
and around AI for the last week." (window 2026-10-03 to 2026-10-10).

How it was made, 22:37 to 22:51 UTC on 2026-10-10:

- Five Claude Haiku 5.5 researchers ran in parallel, one slice each: models
  and labs, policy and law, business and compute, products, and research,
  safety and society. Each used live web search and fetch: about 50 to 60
  tool calls each, 3 to 4 minutes of wall time.
- A sixth Haiku merged the five slices into `report.md` with no new
  searches (about 4 minutes).
- Total wall time is about 8 minutes. Two researchers reported running out
  of search budget, and all five called the search index thin for the
  window.

Use: the research fan-out goal
(`docs/archive/definitions/choir-research-fanout-2026-10-10.md`) compares Choir's
document for the same prompt against `report.md`: distinct in-window items
with correct dates and working sources, plus time to first and last
revision. Items here are not independently verified; they carry the
researchers' own confidence labels.
