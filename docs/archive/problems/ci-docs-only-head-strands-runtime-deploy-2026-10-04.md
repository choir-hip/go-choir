# CI: a docs-only head push after a failed runtime deploy strands the runtime code undeployed

**Date:** 2026-10-04
**Status:** open. Reproduced live while deploying the S2 CAS-ref transport.
**Mutation class of this record:** green (problem documentation only).
**Subsystem:** `.github/workflows/ci.yml` deploy gating.

## Finding

Staging deploy is gated on `plan.outputs.non_docs`, computed by
`ci-impact-classify` over the **push delta** `github.event.before →
github.sha` (ci.yml:34, plan step classify). The `deploy-impact` job itself
correctly diffs against the *deployed* identity (ci.yml:604-620) so
intermediate failed pushes do not shrink the delta — but `deploy-impact` only
runs when `non_docs == 'true'` (ci.yml:552).

So when a runtime commit's CI run fails or is superseded and a **docs-only
commit lands on main next**, `plan.non_docs=false` for that push →
`deploy-impact` never runs → the earlier runtime code is never deployed.
Staging continues serving the last deployed commit while main carries
undeployed runtime.

## Reproduction (live, 2026-10-04)

- `ef2e607d` (CAS-ref transport: platform/agentcore/selfdevprotocol Go)
  ran; its `actorruntime` race shard flaked
  (`TestAdapterRestartDeliversRunningLifecycleActivationFromDurableBacklog`,
  passes locally) → deploy never reached.
- `25ed50ff`, `4a3c6991`, `5458c090` — docs/report commits on top — each
  computed `non_docs=false` and skipped deploy-impact + Deploy.
- Result: `main` carried undeployed runtime code with no scheduled deploy;
  `choir.news` kept serving `3142979b`.
- Workaround applied: `gh workflow run ci.yml --ref main
  -f force_staging_deploy=true` (run 37210230559) — forces the deploy lane.

## Why it matters

The landing loop treats "main pushed → CI → deployed" as monotonic. This gap
breaks that: a green docs push can quietly freeze staging one runtime commit
behind, and an agent watching `x-choir-build-commit` will wait forever for a
deploy that was never scheduled. It is the same class of hazard the
metamission already names under CI deploy-cancellation.

## Fix direction (not implemented yet)

Make the deploy gate depend on the **deployed-vs-head** delta, not the
push delta: `deploy-impact` should run whenever the head differs from the
deployed commit and the delta contains deployed-artifact paths —
independent of whether the immediate push was docs-only. Concretely: the
`non_docs` computation should diff `deployed → head` (as deploy-impact
already does internally), or `deploy-impact` should not be gated on
`plan.non_docs` at all and decide purely on its own deployed-base diff.

## Verification

Push a runtime change, let its deploy fail; push a docs-only commit; assert
the deploy lane still schedules for the head (or that a later non-docs push
deploys the full `deployed→head` delta). Current workaround is the manual
`force_staging_deploy` dispatch.
