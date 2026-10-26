# A Release Carries the Publisher's Whole Unpromoted Stack

Date: 2026-10-26. Found by the SR9 investigation (read-only). Problem
first; no fix in this commit. Status: open; fixed by Library step S2
(docs/design/library-package-propagation-2026-10-26.md, sections 3.1
and 3.2, failure mode L1).

A release is meant to be one change. It is the publisher's whole stack of
changes that no one has promoted to main yet.

## What happens

When U1 (the source projection) is on, a candidate's patch is rewritten at Freeze
into the patch from the stack base. `composeFreezePatch`
(internal/agentcore/source_projection.go:323-358) replaces the staged
`source.patch` with the result of `sourceproj.Cumulative`
(internal/sourceproj/compose.go:42-80), and the bundle then names that
patch and the stack base. The comment at compose.go:39-41 says why: the host
build is named from the bundle's patch and base.

The release draft reads that bundle. `releaseDraft`
(internal/agentcore/api_release.go:45-120) takes `SourcePatchSHA256` and
`SourcePatchBaseCommit` from it and records them as `Source.PatchSHA256` and
`Source.BaseCommit` (api_release.go:97-99). So the published patch is
cumulative. `ChangedPaths` is read from the same patch
(`patchChangedPaths`, api_release.go:99), so it lists every path of every
change in the stack.

A computer with two unpromoted changes, one to the Reader and one to the
shell, publishes both as one release. An adopter that wants the Reader
change gets the shell change with it. Adoption hands the whole patch to the
desks (internal/agentcore/release_adoption.go:336-351).

## Evidence

- The Reader candidate on the test computer (selfdev-aae30fb7) has a scope
  of six entries, the Reader package and five files outside it
  (docs/definitions/choir-t1a-app-self-development-2026-10-26.md, the `now`
  card). With U1 on, a release of it would carry that whole scope, and
  every earlier unpromoted change under it.
- `release.Record.Validate` (internal/release/record.go:194-200) checks only
  that `changed_paths` is non-empty and clean. Nothing limits the paths, and
  nothing compares the patch to the publisher's own change.

## Why it matters

- An owner who publishes one change publishes every earlier unpromoted
  change on the computer, including ones not meant to leave it.
- A release cannot be adopted on its own: it depends on the stack under it,
  which the adopter may not have.
- The patch is larger than the change, so it conflicts more and costs more
  to review.

Fix: Library step S2. A package release carries only the package
directory's tree, and refuses with `package_parent_unpublished` when the
operation sits on an earlier unpublished change to the same package.
