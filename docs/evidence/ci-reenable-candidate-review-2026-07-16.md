# CI Re-Enable Candidate Review — 2026-07-16

## Frozen identity

- Candidate: `8e4aa074f970b69ce59cffa07b280f164ca1c161`
- Define base: `27f03875702f503d8ef551035733eb5f40e27a1c`
- Scoped path: `.github/workflows/ci.yml`
- Frozen candidate `.github/workflows/ci.yml` SHA-256:
  `f60b63fe5e1613a9f7432f1f7c79bc577a1aee52dac7195ec4de1ad39ee4ee25`
- Pre-candidate `.github/workflows/ci.yml` SHA-256 at `27f03875`:
  `69b5de792e84446eb5802ae4f60839bbf8bfb2026e64749c3e9dac6e0f93b14c`
- Diff: restore the exact pre-pause race selector, matching `check`
  selection, and post-check main-only SBOM selector. No other workflow or
  script changed.

## Deterministic evidence

- Both restored job conditions exactly match `1b28520^:.github/workflows/ci.yml`.
- YAML parsing passed for `ci.yml` and `race.yml`.
- `ci-impact-classify-test`, `build-sboms-differential-test`,
  `deploy-impact-classify-test`, and `deploy-workflow-contract-test` passed.
- `scripts/doccheck` completed report-only over 265 documents.
- Direct CI classification of `.github/workflows/ci.yml` produced `ci=true`,
  `sbom=true`, `high_risk_race=false`, and sample bucket 17/20 at the Define
  commit, but that receipt omitted the decisive `go=false`. A direct classifier
  run with the frozen candidate SHA `8e4aa074` and the actual changed path
  `.github/workflows/ci.yml` returned `go=false`, `sbom=true`,
  `high_risk_race=false`, `sampled_race=false`, and bucket 12/20. For a
  pull-request event the sampled push branch is inapplicable; for a main push,
  `go=false` makes the sampled race condition false regardless of bucket.
- Direct deploy classification produced `deploy_needed=false` and every deploy
  class false.
- Local `actionlint` was unavailable. Historical successful run 29295978398
  proves the restored syntax executed before pause, but hosted candidate parsing
  remains required.

## Independent review

The skill-owned panel runner was bound to the frozen commit. Reviewer health:

- `opencode`: `ok`, 46 seconds, verdict `accept`, confidence high.
- `devin`, `cursor`, and three OMP reviewers: missing CLI.
- `codex`: subprocess entrypoint missing from the runner PATH.

The successful reviewer found no blocking issue. It independently compared the
candidate to `1b28520^`, confirmed that `check` fails when a selected reusable
race workflow does not succeed, confirmed SBOM remains main-push-only and after
successful `check`, and confirmed `deploy-staging` does not depend on SBOM.

## P1/P2 acceptance correction

Subsequent independent supervisory review found a P1 contradiction in the
Definition, not in the frozen workflow patch. A `.github/workflows/ci.yml`-only
main landing classifies `go=false`, `high_risk_race=false`, and `sbom=true`.
It therefore runs SBOM but does not select race, regardless of sample bucket,
because the sampled condition also requires `go=true`. A pull-request run
proves hosted parsing but runs neither main-only SBOM nor push-only sampling.
The previous acceptance text incorrectly required one canonical landing to
prove both restored signals.

The repaired evidence obligations are:

1. PR hosted parsing and honest selected/skipped job dispositions;
2. an owner-authorized serialized CI-only main landing proving SBOM,
   `deploy_needed=false`, and staging skip, without requiring race; and
3. the first naturally qualifying post-land main push proving the restored
   parent race selector, all five child jobs, and check binding. A scheduled or
   separately owner-authorized direct `race.yml` run can prove the child workflow
   but cannot substitute for parent selector/check evidence.

Main-run coordination covers both cancellation groups: parent CI uses
`ci-${github.ref}` and reusable/scheduled Race uses `race-${github.ref}`. No
app/platform change may be manufactured as a stimulus, and no push, merge, or
dispatch is authorized in this checkpoint.

## Current-main reconciliation

`origin/main` advanced from `d87bdc44` to
`a1d2f88c6a7135c8a1db916b6fb4f00acf43fb36`. The current-main movement updates
the protected Autoputer Definition and adds phase-A evidence. Reconciliation of
this branch as a whole also includes the green activation's changes to shared
registry/docs paths and deletion/rename of PR #53's source-branch draft. A
scoped diff confirms no change on current main to `.github/workflows/ci.yml` or
other admitted CI implementation paths, so the frozen workflow candidate
remains semantically current.

The branch as a whole still overlaps shared registry paths relative to current
main: `docs/ACTIVE.md`, `docs/doc-authority-manifest.yaml`, and
`docs/mission-graph.yaml`. Current main was read directly: it still states one
unqualified mission entrypoint, contains no CI-maintenance node or manifest
entry, and never contained PR #53's old draft. Rebase/landing must therefore
replay and revalidate the owner-authorized product-versus-CI entrypoint
distinction, add the live CI entries to all three current registries, and retain
the source-branch draft deletion/rename without resurrecting the draft. The
candidate workflow blob and candidate digest must remain unchanged.

## Adjudication

`repair` the Definition acceptance/concurrency contract while retaining
`accept` for the unchanged frozen workflow candidate. The missing reviewers
reduce review diversity but do not contradict the code result; deterministic
and historical evidence covers the exact restoration. This is not canonical
acceptance. Remaining gates are current-main registry reconciliation, hosted PR
parsing, and separately authorized SBOM/deploy-skip and race-selector receipts.

No cache, priming, artifact-transfer, ripgrep, deploy-test-removal, partial-race,
app/platform, deployment, or billing change was admitted.
