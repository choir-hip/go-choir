# A rebase whose right answer is "the new base supersedes my change" has no outcome (2026-11-04)

Status: fix landed on branch noop-rebase (hypothesis 1), not yet deployed;
documented before the fix. Mutation class of the fix: red
(self-development Freeze, the U4 rebase content gate at apply). Blocks
every ordinary engineering Cast on the crouton arm.

## Problem

Commit 8d78b7e78 (deployed 11:04Z) added `frontend/src/packages/library-next`
to main: an export of pool's Library release 8810c72d. Test computers that
had applied their own, independently written `library-next` updated onto
8d78b7e78 and their applied stack went `stack_base_moved`. While it is,
`sourceProjectionCastGateFor` refuses every ordinary engineering Cast; only
a U4 rebase Cast is admitted.

The rebase has two honest answers: re-apply the change on the new base, or
drop it because the new base supersedes it. U4 built only the first. A
rebase round that concludes "drop" has no way to say so:

- Freeze refuses an empty patch (`freezeSourceChangeError`, failure 15 of
  docs/problems/engineering-completion-path-cluster-2026-10-20.md: "this
  capsule changed no /workspace/platform source");
- the materializer's rebase content gate refuses a rebase operation with
  no source patch as `candidate_does_not_apply`
  (`verifyRebaseContentForOperation`), and `sourceproj.VerifyRebase`
  refuses an empty candidate the same way;
- Complete without a candidate leaves the pair claimed and the stack where
  it was; no further notice is minted for a claimed pair.

So the stack stays `stack_base_moved` and the computer's self-development
is wedged until the next base (or forever, if the base never takes the
arm's version).

The U4 design does not contain this outcome. It names the neighbour as a
residual: a conflict path the candidate leaves as the new base has it is
refused `kept_base_unverifiable` ("content cannot tell resolved by keeping
B''s version from dropped ... there is no escape in U4",
docs/archive/design/u4-stack-rebase-build-2026-10-26.md, question 7). A whole
change superseded by the base is that residual on every path at once.

## Evidence

crouton arm, computer-848b21d55df1dd1a953ae798b8edc696:

- Applied stack top: selfdev-5b70a932cf065e299c1e5eca9506774f (trajectory
  26f41d68), stack base 4a6ab66bf. Its cumulative patch
  (`choir self-dev patch`, sha256 2485a56d…, 40162 bytes) adds three files:
  `library-next/App.svelte` (567 lines), `choir-package.json`, `library.css`.
  The prior applied op selfdev-227b2366 (same trajectory) adds the same three
  paths.
- 8d78b7e78 adds five files at the same directory: `App.svelte` (1363 lines),
  `Icon.svelte`, `catalog.js`, `choir-package.json`, `library.css`. Same
  package name and title ("Library (new)"), different implementation: the
  manifests differ (description, launcher order 115 vs 12, singleton true vs
  false, heavy false vs true, auth preview public-preview vs private).
- Every path of P is add/add with different bytes, so `sourceproj.Contained`
  is false (not `stack_upstreamed`), and `git apply` of P on 8d78b7e78 fails on
  every path. In `VerifyRebase`'s terms each path is a conflict path whose
  "keep the new base" resolution is `kept_base_unverifiable`.
- Rebase notice to Texture doc cb80d63e (trajectory 26f41d68): "Rebase the
  applied change selfdev-5b70a932 onto base 8d78b7e78". Engineering bound
  11:20:31; the engineer judged the new base already holds a newer Library
  and made no edit; Freeze refused 11:21:41 (failure-15 text); operation
  selfdev-57b6e558 failed "assignment completed before its candidate
  froze"; Texture settled "no code change was needed". Round r9 (trajectory
  f07451ef) is stuck at Cast.

Answer to "was the engineer right that it is already contained": no, not
literally. The base does not contain crouton's change; it contains a
different app under the same package path. The engineer's real judgment is
"the platform's export supersedes the arm's version", which is a legitimate
product outcome (the export is the release the platform chose), and the one
the rebase could not express.

claude arm, computer-a875023a6379c41f6ddb438f02e76a02 (differs from crouton):

- Applied stack top selfdev-a66db1c8 (trajectory 5e79072c), stack base
  1104c9890. Its cumulative patch (301833 bytes) touches files-next (12
  paths), library-next (7) and slides-next (8). Dropping it would drop
  files-next and slides-next, which the new base does not supersede. Its
  right answer is a re-apply.
- It did re-apply: selfdev-62f30e70 (same trajectory, 11:26) froze a rebase
  candidate ("Rebase applied cleanly with the conflicted files reconciled to
  the patch versions"), and independent judgment failed it for "0 of 12
  shots" the original request names. That is a separate defect (the per-shot
  review is applied to a rebase); the coordinator is fixing it in
  `rlm_reduce.go`. Not this problem.

## Cause (the export collision)

A Library release was exported to main verbatim under the same package
directory that test arms had grown their own version in. Package identity
is the directory name, so two independent apps collided at one path. Any
export of a package into main can do this to a computer that has a local
version of the same package; the platform's version should win, and the
arm's computer needs a way to accept that.

## Hypotheses for the fix

1. (Chosen.) A rebase round's Freeze with an empty source patch is a
   "superseded" candidate, admitted only when the content proves the base
   took every path over: run the same per-path rebase check with the
   candidate equal to the new base, and pass only when no path of P applies
   cleanly to the new base with a differing result (`clean_path_differs`),
   none lacks a hunk the merge would carry (`hunk_missing`), and every
   remaining path is a conflict the base resolved its own way
   (`kept_base_unverifiable`) or already carries (`Resolved`). It goes
   through Verify and owner approval like any candidate; the owner's apply
   record says the applied change is dropped and names the paths; once
   applied the stack has no source patch, so the projection sits on the base.
   Rollback to the previous operation restores the old stack (failure mode 5
   of U4).
2. An owner act "drop the stack" (rollback to the base). Rejected for now:
   no genesis operation is listed on these arms, so there is no rollback
   target that carries no source; and it takes the decision out of the
   rebase round that already reached it.
3. Treat add/add of the same package directory as contained. Rejected: it
   would silently discard a change the owner approved.

## Unblock today

No existing owner act clears it. `self-dev rollback --to-operation` needs an
applied target: on crouton both applied operations add the same
library-next paths, so a rollback leaves the stack `stack_base_moved`.
`abandon` settles open operations, not the applied stack. After the fix
deploys, the owner's `rebase-retry` (POST
/api/computers/<id>/self-development/rebase-retry, an owner act) re-opens the
spent pair with one notice; the round's engineer can then Freeze the empty
capsule as superseded.

Review r1 residuals (Devin nits 3 and 5): a superseded Freeze proves its base only through a reused running binary, so a no-source capsule that stages its own `bin/autoputer` (the autoputer build job run for a receipt) is refused `base_mismatch`, and one whose running build is not B' with differing binary inputs fails earlier at staging (`PlanReleaseBinaryReuseWith`), the same wedge as before; the common shape, a computer already running B', needs no proof.
