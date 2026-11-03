# A stack carrying two packages cannot publish the second (problem 29)

Mutation class: green (this doc); the fix is orange and lands separately.

## Evidence

On the deepseek test arm the applied stack carries two packages. slides-next
was published from it at 19:16:54Z. Publishing files-next from a later applied
operation (selfdev-6be8dc5f25c72c7edfc544c90a881152) answered 409:

    package_change_not_confined: the operation changed paths outside
    frontend/src/packages/files-next that differ from the base:
    frontend/src/lib/app-sdk.ts, frontend/src/packages/slides-next/App.svelte,
    frontend/src/packages/slides-next/Icon.svelte, ...

## Defect 1: a sibling package's subtree counts as outside

`pkgrelease.DraftPackage` (changed for problem 23, ec297ef20) refuses any path
that is not under the package root, differs from the stack base and was
touched by the operation. A sibling `frontend/src/packages/<other>/` subtree
is such a path. Each package is published from its own subtree and the
release carries only that subtree, so a sibling's subtree can never leak into
the release. A stack with two packages could therefore publish only the first.

Fix: paths under another package's subtree are not outside. Shared non-package
paths (frontend/src/lib/app-sdk.ts) and the package's own tests still count.

## Defect 2 (hypothesis from code, not from the live ledger): why the operation "changes" slides-next

The restore round touched only app-sdk.ts and tests, yet the refusal lists
slides-next files. `packageSourceBefore` (internal/agentcore/api_package_release.go)
returns the previous applied operation's projection commit as the source
before, but returns the stack base itself when the previous ref's StackBase is
not the operation's base (the loop breaks "the stack restarted"), after a
re-graft (stack_rematerialized) for example. The operation's own change is
then the diff base..after, which is cumulative and includes slides-next. So the
diff is taken against the stack base, not the parent operation, whenever the
parent's ref sits on another stack base. This is the designed fallback, not a
wrong-parent bug in the diff itself; its cost was that every cumulative path
counted as this operation's. With defect 1 fixed the fallback is harmless for
sibling packages; shared-lib paths still count correctly because they are
compared to the base. Confirming the re-graft on this operation needs its
ledger (applied ref StackBase vs operation base); recorded as a residual.

## Defect 2 confirmed by code (coordinator clue, pool selfdev-c2624f40...)

A re-graft (`reconcileStack`, internal/sourceproj/sourceproj.go) leaves each
applied ref's commit on the stack's old base (`appliedCommit: commit`, trailer
Choir-Stack-Base = old base) while HEAD, and so the next operation's cumulative
bundle, is on the booted base. `packageSourceBefore` then saw
`ref.StackBase != base`, broke, and used the booted base as the source before:
the operation's "own change" became base..after, the whole cumulative stack
(files-next, app-sdk, ...). Deepseek's stack was never re-grafted, so its
refs matched and its publish passed.

Fix: the nearest earlier ref still stands in, with its stack base passed as
`BeforeBase`; `DraftPackage` grafts that change onto the operation's base
(only when the base moved at none of the touched paths, else the old base
fallback) so the parent is the previous operation, not the stack base.
