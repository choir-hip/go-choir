# ACTIVE — Confirmed Work View

**Status:** curated transition view. It is narrower than the legacy mission
corpus and does not make an unverified graph status into a live work claim.

## Active Definitions

[`definitions/choir-autoputer-completion-suite-2026-07-11.md`](definitions/choir-autoputer-completion-suite-2026-07-11.md)
is the one active top-level product Definition and the only `/goal` entry point
for current product work. It owns the resumable sequence from Deploy repair
through Wire cutover, runtime dissolution, audited-computer proof, external
operator truth, self-development, containment, Choir-in-Choir, and vocabulary
cutover. The superseded product-completion and run-truth documents supply
subordinate contracts/evidence only.

[`definitions/og-dolt-heresy-completion-2026-07-08.md`](definitions/og-dolt-heresy-completion-2026-07-08.md)
remains load-bearing for storage, promotion, and heresy detector/deletion
contracts, but overlapping mutations execute as grand-suite subgoals rather
than through a competing top-level `/goal`.

## Supporting Maintenance

[`definitions/choir-seam-repair-2026-07-10.md`](definitions/choir-seam-repair-2026-07-10.md)
is the active maintenance Definition for service-scoped deployment identity,
RouteProfile format repair, compiled-only source-workspace identity, dead-code
excision, and product-completion Definition state refresh. It is **settled**.

[`definitions/choir-autopaper-activation-2026-07-10.md`](definitions/choir-autopaper-activation-2026-07-10.md)
is **superseded in topology** (2026-07-11): the post-mortem
[`definitions/choir-autopaper-activation-attempt-report-2026-07-11.md`](definitions/choir-autopaper-activation-attempt-report-2026-07-11.md)
showed its Real Artifact contradicted the settled D-WIRE decision. Its
evidence ledger remains valid history; do not execute its topology sections.
Successor authority:
[`definitions/choir-autoputer-completion-suite-2026-07-11.md`](definitions/choir-autoputer-completion-suite-2026-07-11.md),
the single executable and resumable mission suite for Deploy restoration,
Wire authority cutover, runtime dissolution, audited-computer proof, external
CLI operability, run truth, self-development, contained Choir-in-Choir
authority, and the final vocabulary cutover. The former
[`definitions/choir-run-truth-suite-2026-07-11.md`](definitions/choir-run-truth-suite-2026-07-11.md)
is now a subordinate historical foliation record, not a `/goal` entry point.

[`definitions/documentation-authority-reduction-2026-07-09.md`](definitions/documentation-authority-reduction-2026-07-09.md)
is **complete**. It remains in the retained packet as the deletion receipt and
maintenance boundary; it is not active work and cannot revise the product
umbrella's semantics.

## Next Executable Work

The seam-repair Definition is **settled** after staging acceptance at `944d4d94`
proved per-service `/health` identity and RouteProfile promotion/rollback on
`choir.news`.

The next and only suite-level invocation is:

```text
/goal docs/definitions/choir-autoputer-completion-suite-2026-07-11.md
```

The same command resumes after intentional or accidental interruption. The
suite orchestrator reconciles durable state and executes these ordered subgoals:

1. persist the suite authority, registry cutover, subordinate demotions, and
   consensus evidence on `origin/main`;
2. reconcile the suite and install runtime-dissolution ratchets;
3. restore Deploy by draining the stuck active run;
4. cut Wire authority to corpusd and delete boot/runtime-local paths;
5. iteratively dissolve `internal/runtime`, with atomic caller cutover and
   independent phase verification, until the directory is absent;
6. prove the audited computer and CLI-visible observation/receipts;
7. establish run truth and artifact-verified completion on the extracted core;
8. prove candidate self-development, receipted promotion, and rollback;
9. prove contained co-super authority and open Choir-in-Choir;
10. perform the alias-free vocabulary cutover and hand off to a newly defined
    Autopaper successor, if still wanted.

Member Definitions are subordinate specifications. They are not invoked as
separate `/goal` runs and cannot reorder the grand suite. Autopaper editorial
work remains blocked until the suite reports `complete`.

The OG/Dolt/heresy contract supplies Texture semantic-forcing detectors,
history/audit evidence, promotion CAS/receipt semantics, and deletion gates as
grand S2/S3/S6/S7/S9 inputs. It does not own execution order or storage
topology. The owner-settled route topology is corpusd sql-server route-slot
tables with vmctl as sole CAS writer, never a third Dolt domain.

D-STORE is settled by owner authority: Choir is all-in on Dolt. Storage
questions about history latency, commit batching, rollback mechanics, ICU/build
friction, and replication are engineering verification tasks, not a renewed
database-choice gate.

## Unowned External Work

Runtime dissolution and actor/runtime extraction are owned by grand S3.
Broader source-system/Wire follow-ups outside grand S2/S3/S6/S9 and outside a
future explicitly authorized Autopaper successor have no active Definition.
Do not resume deleted plans; any new program must be grounded in current code
and staging evidence.

## Graph Rule

[`mission-graph.yaml`](mission-graph.yaml) is a minimal discovery index. A
Definition owns its own state. Beads and Git history are not executable mission
authority.

## Remaining Error

`Deploy to Staging (Node B)` on `main` is failing because `vm-universal-wire-platform`
has an active run (`running_runs: 1`) and the `Deploy` hot-refresh waits until the
sandbox `/health` reports the new commit. The `sourcecycled` `blocked` fix
(`f1ceba58`) treats a `blocked` processor run as terminal, but the stuck run is
still `running`, so the refresh cannot complete.

Evidence: runs `29154725145` (failed `Deploy` after `f2d0af69`), `29155456035`
(cancelled `Merge` run), and `29155509641` (failed `Deploy` after `894eaf2c`);
`Deploy` logs show `Timed out waiting for vm-universal-wire-platform` and
`running_runs: 1` in the diagnostic ownership snapshot.

**Fix:** the grand suite's S1 subgoal, using
[`definitions/choir-run-deploy-unblock-2026-07-11.md`](definitions/choir-run-deploy-unblock-2026-07-11.md)
as its bounded subordinate specification. Full run-lifecycle work occurs only
after Wire cutover, runtime extinction, audited-computer proof, and observation
receipts.

Note: `skills/*` → sandbox deploy classify was addressed on `main` by
`d8fe4336` (non-deployed workflow artifact). If a later push reintroduces that
classify, treat it as CI hygiene, not run-lifecycle work.
