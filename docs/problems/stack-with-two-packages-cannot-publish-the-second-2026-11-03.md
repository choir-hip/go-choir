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
