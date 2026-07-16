# ACTIVE — Confirmed Work View

**Status:** curated transition view. It is narrower than the legacy mission
corpus and does not make an unverified graph status into a live work claim.

## Active Definitions

[`definitions/choir-audited-autoputer-construction-2026-07-15.md`](definitions/choir-audited-autoputer-construction-2026-07-15.md)
is the one active top-level product Definition and the sole product `/goal`
entrypoint.
It makes the production Autoputer real by constructing and booting a disposable
realization from immutable `ComputerVersion = (CodeRef, ArtifactProgramRef)`,
then proving exact typed state, destruction/reconstruction, route promotion and
rollback, and no-SSH inspection on staging.
The superseded
[`definitions/choir-autoputer-completion-2026-07-14.md`](definitions/choir-autoputer-completion-2026-07-14.md)
remains historical evidence. Its runtime and lifecycle receipts remain citable,
but its completion framing was falsified by the deployed opaque-disk outage. Do
not execute it.

The superseded
[`definitions/choir-autoputer-completion-2026-07-13.md`](definitions/choir-autoputer-completion-2026-07-13.md)
remains discoverable as historical evidence, including its references to the
[`runtime-dissolution-inventory.yaml`](runtime-dissolution-inventory.yaml).
Do not execute it.

[`definitions/og-dolt-heresy-completion-2026-07-08.md`](definitions/og-dolt-heresy-completion-2026-07-08.md)
remains load-bearing only for settled storage/D-ROUTE authority and H031
detector/deletion contracts consumed by phases B, D, and F of the active
Definition. It is not a competing `/goal`.

## Draft Successor Definitions — Not Executable

[`definitions/choir-computerversion-performance-optimization-draft-2026-07-15.md`](definitions/choir-computerversion-performance-optimization-draft-2026-07-15.md)
is an owner-authorized **draft** successor for empirically optimizing
ComputerVersion realization performance after audited construction completes.
It is blocked, is not a `/goal` entrypoint, and authorizes no implementation or
benchmark claim. Its Firecracker benchmarks must run on Node B; local macOS
timing is inadmissible. Reconcile and revise the draft from the completed
predecessor's deployed baseline, set owner-ratified numerical SLOs, and promote
it through all registries before execution.

## Independent CI Maintenance — Executable

[`definitions/choir-ci-optimization-2026-07-16.md`](definitions/choir-ci-optimization-2026-07-16.md)
is an owner-authorized, scope-disjoint CI-maintenance `/goal` entrypoint. It may
run concurrently with Autoputer because it cannot change app/platform source,
product authority, Node B, or product state. This does not create a second
product mission: Autoputer remains the sole product `/goal`. The CI mission
restores the full reusable race workflow for either classifier selection and
re-enables the already-wired host-side SBOM topology as post-`check`, non-blocking
audit evidence. GitHub Actions is its acceptance environment; CI-only changes
must prove the expected deploy-impact/Node B skip. Any main landing must be
explicitly owner-authorized and serialized because same-ref CI cancels an
in-flight run. Separate main Race observation must also coordinate the
`race-${github.ref}` cancellation group; this checkpoint authorizes no push,
merge, or workflow dispatch.

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
[`definitions/choir-audited-autoputer-construction-2026-07-15.md`](definitions/choir-audited-autoputer-construction-2026-07-15.md).
The former
[`definitions/choir-autoputer-completion-suite-2026-07-11.md`](definitions/choir-autoputer-completion-suite-2026-07-11.md)
and
[`definitions/choir-run-truth-suite-2026-07-11.md`](definitions/choir-run-truth-suite-2026-07-11.md)
are historical evidence, not `/goal` entrypoints.

[`definitions/documentation-authority-reduction-2026-07-09.md`](definitions/documentation-authority-reduction-2026-07-09.md)
is **complete**. It remains in the retained packet as the deletion receipt and
maintenance boundary; its 2026-07-14 owner correction restores
[`archive/`](archive/README.md) as searchable historical context without making
it active work or revising the product umbrella's semantics.

## Invocation

The sole product invocation is:

```text
/goal docs/definitions/choir-audited-autoputer-construction-2026-07-15.md
```

The independent CI-maintenance invocation is:

```text
/goal docs/definitions/choir-ci-optimization-2026-07-16.md
```

This index owns no current slice, next action, execution order, resumption,
completion, mutation, rollback sequencing, or escalation authority. Consult
the active Definition.

Ordinary Choir development in capsules controlled by an outside agent through
Choir CLI is an unauthorized successor until this Definition is complete.
Autopaper remains unauthorized.

The OG/Dolt/heresy contract supplies D-ROUTE's corpusd/vmctl CAS semantics,
receipt projection gates, H031 detection, and deletion bars to active phases B,
D, and F. It does not own execution order or storage topology. The owner-settled
topology has two non-conflated Dolt stores plus corpusd route-slot tables with
vmctl as sole CAS writer; route control is never a third store.

D-STORE is settled by owner authority: Choir is all-in on Dolt. Storage
questions about history latency, commit batching, rollback mechanics, ICU/build
friction, and replication are engineering verification tasks, not a renewed
database-choice gate.

## Unowned External Work

Runtime dissolution, actor/runtime extraction, broader source-system/Wire work,
and external-agent capsule development have no active Definition unless they
are strictly required by this mission's audited-construction acceptance.
Do not resume deleted plans; any new program must be grounded in current code
and staging evidence.

## Graph Rule

[`mission-graph.yaml`](mission-graph.yaml) is a minimal discovery index. A
Definition owns its own state. Beads and Git history are not executable mission
authority.

## Settled Deploy Receipt

The former `running_runs: 1` hot-refresh blockage remains a settled historical
receipt at `9dff369044c2147140782958de3e91971caed6bc`; evidence:
`docs/evidence/s1-deploy-unblock-dispatch-2026-07-12.md`.

Do not rerun its topology. A reproduced deadline/cancel/deploy regression must
be documented as a new problem and may enter this mission only if it blocks the
audited constructor's acceptance path; otherwise it requires separate authority.
The earlier `skills/*` deploy-classifier fix remains CI hygiene, not
run-lifecycle authority.
