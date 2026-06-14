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

## 2026-06-14 - Checker Implementation And Baseline

Claim: the simplified checker can settle v0 if it produces an external
manifest, a report-only local command, manifest/link/witness/heresy reports, and
a target-doc reconciler guard probe without touching runtime behavior or CI path
filters.

Move: add `docs/doc-authority-manifest.yaml`, implement `cmd/doccheck`, add the
`scripts/doccheck` wrapper, ignore generated report outputs, run the checker
over the repo, and run an explicit `docs_reconciler` write-attempt probe against
`docs/intended-architecture-next-2026-06-06.md`.

Expected Delta V: -7 by eliminating the missing manifest, checker command,
report outputs, seeded detector config, measured runtime, reviewed baseline
warnings, and CI/manual workflow decision obligations.

Actual Delta V: -7. The checker is local/manual and warn-only; generated
reports are emitted as ignored artifacts; no docs-only CI filters were changed.

Receipts:

- `go test ./cmd/doccheck` passes.
- `scripts/doccheck` scanned 193 Markdown docs, emitted `doccheck-report.md`
  and `doccheck.json`, exited 0, and recorded runtime 819ms.
- Baseline warnings: H1=724, H3=19, H4=3, R3=50. Reviewed as discovery-only;
  no content repair is claimed.
- `go run ./cmd/doccheck -actor docs_reconciler -write-attempt
  docs/intended-architecture-next-2026-06-06.md -report
  /tmp/doccheck-r4-report.md -json /tmp/doccheck-r4.json` exited 0 and emitted
  R4=1 for the target-doc write attempt.
- Independent prover review found no blocking findings, confirmed generated
  reports are ignored, no `.github` workflow or runtime behavior files are
  dirty, and the R4 probe exits 0 with R4=1.

Open edge: the baseline still needs a successor allowlist/manual-docs pass
before warnings can become enforcement or feed an automated reconciler.

## 2026-06-14 - Report-Only CI Wiring

Claim: v0 can enter CI without weakening Choir's docs-only path-filter policy
if the job runs only when CI is already triggered, verifies report generation,
uploads artifacts, and continues to treat baseline warnings as discovery-only
rather than failures.

Move: add a `doccheck` job to `.github/workflows/ci.yml`, wire it into the
aggregate `check` job, upload `doccheck-report.md` and `doccheck.json`, and
preserve the existing `docs/**` and top-level `*.md` `paths-ignore` filters.

Expected Delta V: -1 by eliminating the unresolved CI/manual workflow decision
without turning doc warnings into a blocking quality gate.

Actual Delta V: -1. CI now exercises the command and report generation when CI
already runs. Docs-only CI behavior remains unchanged.

Receipts:

- `.github/workflows/ci.yml` has a `Docs Truth Check` job.
- The job runs `scripts/doccheck`, asserts both reports are non-empty, and
  uploads them as a `doccheck-report` artifact.
- The aggregate `check` job now requires `doccheck` success, meaning command
  success and report generation, not zero warnings.

Open edge: automatic doc-only PR execution still requires explicit
operating-contract reconciliation before changing path filters.

## 2026-06-14 - Completion Audit

Claim: the v0 checker mission can be closed only if current evidence proves the
manifest, checker command, generated reports, warn-only behavior, R4 target-doc
guard probe, baseline review, runtime budget, and no-runtime-change boundary.

Move: audit the current worktree, fix the stale Suggested Goal String in the
paradoc, rerun compile/report/R4 acceptance, and inspect dirty paths plus
ignored generated artifacts.

Expected Delta V: -1 by converting settlement from a plausible handoff claim
to current-state evidence.

Actual Delta V: -1. The mission remains settled for v0.

Receipts:

- `nix develop -c go test ./cmd/doccheck` passes.
- `nix develop -c scripts/doccheck` scans 193 Markdown docs, emits
  `doccheck-report.md` and `doccheck.json`, exits 0, and records runtime under
  the 10-second budget.
- The baseline warning shape remains discovery-only: H1 retired vocabulary,
  H3 VText agency-collapse candidates, H4 current/target collapse candidates,
  and R3 reachability or collection-candidate findings. Introduced heresy is
  empty and repaired heresy is not claimed for content.
- `nix develop -c go run ./cmd/doccheck -actor docs_reconciler -write-attempt
  docs/intended-architecture-next-2026-06-06.md -report
  /tmp/doccheck-r4-report.md -json /tmp/doccheck-r4.json` exits 0 and emits
  one R4 warning for a `claim_scope: target` write attempt.
- Dirty tracked paths are limited to `.gitignore`, this paradoc/ledger, and
  the new docs checker artifacts; no `.github` workflow or runtime behavior
  file is dirty. Generated `doccheck-report.md` and `doccheck.json` are ignored.

Open edge: successor work still needs a reviewed allowlist/manual-docs cleanup
plan before warnings can become policy, and no reconciler should be implemented
until its write path carries the R4 actor guard.
