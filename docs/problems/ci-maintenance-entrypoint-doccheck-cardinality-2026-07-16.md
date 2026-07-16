# Scope-Disjoint CI Entrypoint Is Rejected by the Doc Truth Checker

**Date:** 2026-07-16
**Status:** observed; bounded repair admitted; main landing blocked pending publication authority
**Classification:** substrate — documentation-governance validation contract
**Mutation class of the exposing change:** green Definition/registry activation
**Mutation class of the planned repair:** yellow documentation-truth tooling

## Problem

The live mission graph deliberately permits one working product spine and an
owner-authorized, scope-disjoint CI-maintenance `/goal` entrypoint. The manifest
keeps the CI Definition entry-only rather than an authority root. The strict
doc truth checker still encodes the older rule that there must be exactly one
graph entrypoint in total, so it rejects the valid live registry.

This prevents the independently authorized CI mission from obtaining the live
doccheck proof required before its PR landing, even though it neither creates a
second product authority nor overlaps the Autoputer product mission.

## Evidence

- Command: `GOCACHE=/tmp/choir-gocache ./scripts/doccheck --mode=live --report /tmp/doccheck.md --json /tmp/doccheck.json`
- Observed result: `L5: expected exactly one mission graph entrypoint, found 2`
- Declared graph rule: `docs/mission-graph.yaml` `scoped-entrypoints`
- Product node: `choir-audited-autoputer-construction-2026-07-15` — working
  `spine` / `mission_orchestrator`
- Maintenance node: `choir-ci-optimization-2026-07-16` — working
  `ci_maintenance` / `scope_disjoint_maintenance`
- Checker sites: `cmd/doccheck/main.go` live L5 entrypoint validation and the
  generic R5 mission-graph validation both require one total entrypoint.

## Root Cause Belief

The graph and manifest were extended to express a bounded maintenance
exception, but `cmd/doccheck` was not extended at the same time. The checker
therefore validates an obsolete cardinality rule rather than the current
authority topology.

## Existing Replacement Opportunity

The declarative replacement already exists in the graph and manifest: one
product authority-root entrypoint plus an entry-only scope-disjoint maintenance
Definition. The repair is to wire that topology into doccheck, not to hide the
CI mission by clearing its executable entrypoint.

## Bounded Repair Contract

The yellow repair may touch only `cmd/doccheck/main.go` and
`cmd/doccheck/main_test.go`. It must:

1. require exactly one working product `spine` / `mission_orchestrator`
   entrypoint that matches the authority-root Definition;
2. allow a working `ci_maintenance` / `scope_disjoint_maintenance` entrypoint
   only when its current manifest entry is a Definition, entry-root, and not an
   authority root;
3. reject any other `entrypoint: true` shape, stale maintenance entrypoint,
   second product spine, or maintenance Definition that becomes an authority
   root; and
4. leave application behavior, workflow semantics, Node B, and product
   authority unchanged.

## 2026-07-16 Landing Side Effect

The bounded source repair is not a docs-only change. Running the unchanged CI
classifier with `cmd/doccheck/main.go` as the changed path yields:

```text
docs_only=false
go=true
sbom=true
flakehub=true
high_risk_race=false
ci=false
```

The normal main workflow therefore skips Node B deployment but may run the
rolling FlakeHub publication after `check` succeeds. The owner's authorization
to merge the CI mission and deploy Node B if needed does not clearly authorize
this separate public publication side effect. The repair may be validated and
opened as a draft PR, but its merge is blocked until the owner either grants
narrow FlakeHub-publication authority or changes the requested landing route.

This is an authority boundary, not a reason to weaken the classifier or remove
the existing FlakeHub relationship from CI.

## Belief State

- Supported: the live registry has one product authority root and one
  scope-disjoint maintenance entrypoint.
- Rejected: “exactly one graph entrypoint” accurately expresses the current
  registry's authority model.
- Pending: focused tests and a live doccheck pass prove the checker correctly
  enforces the bounded exception without admitting a second product mission.

## Remaining Error Field

Until the checker is repaired, a strict live doccheck fails. The CI candidate
must not be pushed or PR-landed as fully validated until the yellow repair has
focused tests and a live doccheck receipt. After that, PR 1's main merge still
requires specific authority for the classifier-selected FlakeHub publication;
Node B remains expected to skip.

## Rollback

Revert only the checker/test repair if it admits an invalid product or
maintenance entrypoint shape. The mission graph, manifest, CI workflow, Node B,
and product state are untouched by this problem record.

## Heresy Delta

- `discovered`: the documentation topology and its executable checker diverged.
- `introduced`: none by this evidence record.
- `repaired`: pending the bounded checker repair and live validation.
