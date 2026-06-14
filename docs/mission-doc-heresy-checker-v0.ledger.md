# Doc/Heresy Checker v0 - Parallax Mission Ledger

## 2026-06-13 - Import And Repo-Grounded Review

Claim: the imported draft has the right checker shape but assumed repo-wide
frontmatter and a four-kind doc type system that the actual corpus does not
support.

Move: copy the draft into `docs/mission-doc-heresy-checker-v0.md`, run repo
probes, and edit the spec in place.

Expected Delta V: -3 by grounding the draft in frontmatter reality, link graph
reality, and CI policy.

Actual Delta V: -3. The revised spec now says external manifest first, no mass
frontmatter migration in v0, doc roles plus claim scope, Markdown links plus
bare filename mentions, and report-only integration that respects docs-only CI
filters.

Receipts:

- Markdown files found: 193.
- YAML frontmatter files found: 3, all skill files.
- Markdown links found: 463 across 71 files.
- Bare Markdown filename mentions found: 1396 across 137 files.
- Current command directories include `auth`, `gateway`, `maild`, `platformd`,
  `proxy`, `sandbox`, `sourcecycled`, `vmctl`, and `zot`.
- CI and FlakeHub workflows ignore `docs/**` and top-level `*.md`.

Open edge: the originating agent has not reviewed this edited draft yet; no
checker implementation exists.
