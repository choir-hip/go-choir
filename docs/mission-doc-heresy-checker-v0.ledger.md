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

## 2026-06-13 - Simplification Pass

Claim: the reviewed draft still carried too much grammar because doc roles,
authority/lifecycle annotations, and updater intent could influence warning
logic.

Move: simplify the checker kernel so rules branch only on `claim_scope`,
`is_root`, and `is_evidence`; demote doc roles to report annotations; rebind
target-doc protection from semantic edit intent to future reconciler actor
identity; and add checker-before-reconciler ordering.

Expected Delta V: doc-role branches 9 -> 0, decision cells 36 -> 4, semantic
intention firing conditions 1 -> 0, non-kernel manifest fields gating checks
3+ -> 0.

Actual Delta V: matched expected. No code was implemented.

Receipts:

- `docs/mission-doc-heresy-checker-v0.md` rule audit now states field reads for
  R1-R4 and H1-H4.
- `doc_role`, `authority`, and `lifecycle` remain only as manifest
  annotations.
- R4 is an actor-identity guard for the future `docs_reconciler`, not an
  inferred-intent rule.
- Ordering constraint added: checker ships before reconciler, and acceptance
  must catch a planted target-doc-converging edit by the reconciler actor.

Open edge: independent prover review still needs to check whether the grammar
regrew elsewhere in the spec.

## 2026-06-13 - Independent Prover Cleanup

Claim: the simplification pass should not hand off until a fresh prover checks
whether the spec regrew grammar through payload fields, incomplete rule audits,
or unsupported settlement language.

Move: run an independent prover review, accept its findings, and patch the spec
so `witnesses` is clearly evidence payload selected by `claim_scope`, R3's read
audit includes `claim_scope`, and `refresh_triggers` is explicitly report
metadata.

Expected Delta V: preserve the simplification pass' claimed result by keeping
non-kernel manifest-field gates at 0 and semantic-intention firing conditions at
0.

Actual Delta V: preserved. The checker kernel remains `claim_scope`, `is_root`,
and `is_evidence`; no code was implemented.

Receipts:

- Independent prover found three grammar-regrowth risks: witness payload as an
  accidental gate, R3's incomplete field-read audit, and top-level
  `refresh_triggers` without a non-gating note.
- `docs/mission-doc-heresy-checker-v0.md` now states that `annotations` and
  `refresh_triggers` are report metadata, while `witnesses` is evidence payload.
- R2 now fires from `claim_scope: current|mixed`, not from the mere presence of
  `witnesses`.
- R3 now audits all kernel fields it reads: `claim_scope`, `is_root`, and
  `is_evidence`.

Open edge: originating-agent critique is still pending; checker implementation
is explicitly deferred.
