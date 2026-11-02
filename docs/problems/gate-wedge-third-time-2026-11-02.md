# A gate-file commit wedged main a third time (2026-11-02)

Mutation class: green (record) with a yellow guard. Owner-authorized repair:
force-push of main ("Yes force push but this is the second time (at least)
this happened so we should also look into preventing it happening again").

## What happened

9c823862c (the deploy-refresh fix, built by a Sonnet builder, landed by the
orchestrator after only the classifier test) added an `actions/setup-go` step
to `.github/workflows/ci.yml`. Any ci.yml change is a gate change for
`scripts/check-guest-security-epoch` (rule 6) and needs `Security-Effect: gate
change` and `Security-Effect-Reviewed-By` trailers in its own message; an
epoch raise does not excuse it. CI runs 41369048312 and 41369591335 failed
Vocabulary Gates; nothing deployed after 61772f741.

Repair: Devin review 383 approved the hunk
(docs/evidence/devin-review-383-ci-setup-go-2026-11-02.md). Main rebuilt from
f332ee465: 9c823862c amended with the trailers (now 3bfc88b42), 3a83cc9e1 and
95ea10801 re-applied unchanged, the review evidence on top; both the head's
and the deployed base's copy of the gate passed; force-pushed with lease
95ea10801 -> bb9686d5d. Old main at origin backup/main-pre-rewrite-2026-11-02.

## Why it keeps happening

Third time: 2026-10-28 (switch registry, review 377), 2026-10-31 (docs
citer rewrite, review 382), 2026-11-02. Each time the rule was known and
documented, and each time the landing step skipped the one check that knows
it. Cause, by trace:

- Nothing runs the gate before a push. The main checkout's
  `core.hooksPath` points at `/Users/wiz/go-choir/.beads/hooks`, which does
  not exist, so no hook of any kind runs on push from any worktree.
- The CI mirror is the documented pre-push check, but it is a step an agent
  must remember; under the Operating Tempo it is the first thing dropped.
- The rule is invisible at the edit: a one-line ci.yml step looks like CI
  plumbing, not a security surface.
- After the push there is no repair short of a history rewrite (the 10-28
  follow-up, an exemption path, is still open).

## What changes

1. A tracked pre-push hook, `.githooks/pre-push`, runs the head's and the
   deployed base's copy of the epoch gate over live-staging..pushed-head for
   every push to main and refuses on failure. `.envrc` points
   `core.hooksPath` at `.githooks` (idempotent), so every direnv-loaded
   worktree gets it. Tested: the old main (95ea10801) is refused, the
   rewritten main passes. `git push --no-verify` still bypasses it; it guards
   the honest mistake, which is all three incidents.
2. Residual, open: a forward recovery path (an exemption commit naming the
   SHA, reviewed like a gate change) so a wedge costs one commit, not a
   force-push. It is a gate change itself and needs Devin review and the
   owner's yes.
3. Residual, open: the gate's remedy text ("a follow-up commit that raises
   security_epoch covers the whole range") misleads for gate files (Devin
   383 nit); fixing it is a gate-file change and rides with item 2.
