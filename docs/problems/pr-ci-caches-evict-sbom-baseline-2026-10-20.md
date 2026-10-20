# PR CI caches evict main's SBOM baseline (2026-10-20)

## Evidence

- CI run 39410862161 for d7f6ecae3 (visual evidence) deployed to staging
  successfully (staging reports d7f6ecae3; vmctl carries the visual capture
  grant), then **Accept Differential SBOMs** failed: "SBOM manifest failed
  package, source, status, or required-package validation".
- The candidate manifest is complete: 11 packages, every required one
  `built` or `reused` with an SBOM digest, `source_commit` d7f6ecae3,
  `previous_commit` ec2dc7b7d.
- The acceptance job's "Restore accepted baseline" step logged "Cache not
  found for input keys: sbom-bundle-v2-d7f6…, sbom-bundle-v2-". With no
  baseline, `.github/scripts/verify-sbom-candidate` expects
  `previous_commit == ""`, so the ec2dc7b7d the build step had read from the
  baseline (11:02 UTC) no longer matched at acceptance (11:05 UTC).
- `gh cache list` at 11:10 UTC: no `sbom-bundle-v2-` entry; about 9.9 GB
  in use against GitHub's 10 GB repository limit, 6.9 GB of it from the
  draft PRs 77 and 78 (CI-only drafts opened to run the Linux and full
  agentcore suites before landing).

## Cause (confirmed by the logs above; the eviction itself is inferred)

GitHub evicts the least recently used caches when a repository passes 10 GB.
PR runs save large caches under `refs/pull/<n>/merge`; the main-only SBOM
baseline is one small, rarely read entry, so it is evicted first. The
build step and the acceptance step read the baseline at different times, so
an eviction between them makes a correct candidate fail acceptance.

## Effect

The deploy had already happened (acceptance runs after it), so staging is
correct; the failure is a false red on main. The next deploying run finds
no baseline in both steps and should accept a fresh one.

## Response now

Deleted the PR caches (refs/pull/76, 77, 78; regenerable), bringing use to
3.0 GB. Opening CI-only draft PRs is how this session runs the full suite
before landing; each one now costs main's baseline unless its caches are
deleted after the checks finish.

## Fix options (not chosen yet)

- A. Read the baseline once, in the build step, and carry it to acceptance
  as an artifact, so both steps judge against the same baseline (removes the
  race).
- B. Do not save the heavy caches from `pull_request` runs (restore only).
- C. Keep the baseline outside the Actions cache (a release asset or a
  branch), where eviction cannot reach it.

A is the substrate fix for the false red; B keeps the quota for main.
Class: yellow (CI), no product behavior.
