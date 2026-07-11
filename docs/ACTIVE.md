# ACTIVE — Confirmed Work View

**Status:** curated transition view. It is narrower than the legacy mission
corpus and does not make an unverified graph status into a live work claim.

## Active Definitions

[`definitions/choir-product-completion-2026-07-10.md`](definitions/choir-product-completion-2026-07-10.md)
is the one confirmed active top-level product Definition. It owns Choir CLI,
Wails desktop, Choir Base, and Autopaper recovery, including auth, data-
integrity, activation, packaging, and staging evidence. It inherits promotion
semantics from the OG/Dolt/heresy Definition rather than creating a competing
protocol.

[`definitions/og-dolt-heresy-completion-2026-07-08.md`](definitions/og-dolt-heresy-completion-2026-07-08.md)
is the active spine Definition for the remaining OG/Dolt/heresy completion
program. It is discoverable and load-bearing for promotion/storage protocol
authority, but it is not a second top-level product Definition.

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
Successors:
[`definitions/choir-wire-store-conformance-2026-07-11.md`](definitions/choir-wire-store-conformance-2026-07-11.md)
(wire state onto the world-wire store; legacy migration deletion),
[`definitions/choir-autoputer-cli-operability-2026-07-11.md`](definitions/choir-autoputer-cli-operability-2026-07-11.md)
(canonical sequence: audited computer → choir-CLI autoputer → choir-in-choir → autopaper), and
[`definitions/choir-run-lifecycle-and-completion-authority-2026-07-11.md`](definitions/choir-run-lifecycle-and-completion-authority-2026-07-11.md)
(unified run status, idempotency/retry, and artifact-verified completion).

[`definitions/documentation-authority-reduction-2026-07-09.md`](definitions/documentation-authority-reduction-2026-07-09.md)
is **complete**. It remains in the retained packet as the deletion receipt and
maintenance boundary; it is not active work and cannot revise the product
umbrella's semantics.

## Next Executable Work

The seam-repair Definition is **settled** after staging acceptance at `944d4d94`
proved per-service `/health` identity and RouteProfile promotion/rollback on
`choir.news`.

The next executable focus is
[`definitions/choir-wire-store-conformance-2026-07-11.md`](definitions/choir-wire-store-conformance-2026-07-11.md):
move wire state to the corpusd-served world-wire store, delete the boot-time
legacy migration, and decouple `/api/universal-wire/stories` from VM
lifecycle. It is Phase 0 of the autoputer sequence in
[`definitions/choir-autoputer-cli-operability-2026-07-11.md`](definitions/choir-autoputer-cli-operability-2026-07-11.md);
PC-5 (Base exact-byte kernel) and audited-computer candidate-materialization work
are owned by [`definitions/choir-product-completion-2026-07-10.md`](definitions/choir-product-completion-2026-07-10.md)
and [`docs/computer-ontology.md`](docs/computer-ontology.md), and
[`definitions/choir-run-lifecycle-and-completion-authority-2026-07-11.md`](definitions/choir-run-lifecycle-and-completion-authority-2026-07-11.md)
resolves run lifecycle and completion truth (Phase 3). Autopaper editorial work waits
for the whole sequence per the restored autoputer-before-autopaper dictum.

Resume product-completion work at PC-2 (Wails token containment) and PC-3 (CLI
request-budget) once Autopaper activation is stable, with Base product wiring
still gated on the exact-byte stable-identity kernel. Typed Autopaper handoff
idempotency remains open after projection-triggered activation deletion.

The OG/Dolt/heresy spine still owns Texture semantic-forcing residue removal,
Dolt history/audit load-bearing reads, and ComputerVersion promotion protocol
gates. Milestone shorthand in doctrine maps to that Definition as follows:
M3.1/M3.2 work is Phase B; M4/M3.3 work is Phase C; ComputerVersion promotion is
Phase D; surface/vocabulary cleanup is Phase E.

D-STORE is settled by owner authority: Choir is all-in on Dolt. Storage
questions about history latency, commit batching, rollback mechanics, ICU/build
friction, and replication are engineering verification tasks, not a renewed
database-choice gate.

## Unowned External Work

Broader source-system/Wire follow-ups outside Autopaper's single-activation
slice and actor/runtime extraction have no active successor Definition. Do not
resume their deleted plans. Any resumed program needs a fresh Definition
grounded in current code and staging evidence.

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

In addition, `.github/scripts/deploy-impact-classify` classifies `skills/*` as a
`sandbox` runtime-package change, so agent-skill docs/scripts (e.g. the
`agentic-consensus` runner and `SKILL.md` note) trigger `Deploy to Staging` and
active-VM refresh. Each `main` push that touches `skills/*` therefore re-attempts
the refresh and fails. `skills/` are agent-facing docs and scripts; they are not
installed in the `sandbox` runtime or on Node B, so they should not select a
sandbox host-service deploy.

Evidence: runs `29154725145` (failed `Deploy` after `f2d0af69`), `29155456035`
(cancelled `Merge` run), and `29155509641` (failed `Deploy` after `894eaf2c`);
`Deploy` logs show `Timed out waiting for vm-universal-wire-platform` and
`running_runs: 1` in the diagnostic ownership snapshot.

The structural fix is the `choir-run-lifecycle-and-completion-authority` mission
(`docs/definitions/choir-run-lifecycle-and-completion-authority-2026-07-11.md`),
which will define run authority and artifact-verified completion. The immediate
CI bypass is to stop routing `skills/*` to a sandbox host-service deploy.
