# Self-Development Operations Retry Silently No-Ops on Passivated Bound Super

<problem_id: selfdev-wake-passivated-super-silent-noop-2026-08-28>
<first_observed: 2026-08-28T17:20Z>
<mutation_class: red>
<deployed_commit: 42d476044ec80efe8a31d043af577ad77ba7572b>
<affected_surfaces: [internal/agentcore/api_self_development.go, internal/agentcore/runtime.go, guest systemd go-choir-autoputer]>

## 1. Problem Description

The mission-defined recovery for an `executing` self-development operation whose
Super died is the idempotent operations POST (`ensureSelfDevelopmentRun`):
unbind the terminal op-bound Super and start a fresh persistent Super. Live on
restored staging (epoch 831, `42d47604`), the retry POST returned **200 with the
operation unchanged and no new Super run** - a silent no-op.

Mechanism: `ensureSelfDevelopmentRun` (StateExecuting branch) wakes only when
the single op-bound run satisfies `selfDevelopmentSuperRunTerminal`, which
accepts `completed | failed | cancelled | blocked` but **not `passivated`**.
The 08-28 boot-outage recovery passivated every interrupted run at each boot
(`passivateInterruptedActivations`, `passivated_reason=runtime_restarted`),
including at least one Super bound to live operation
`selfdev-ccf0f1ec0e851750f253fe5f5ed97974` (state `executing`, empty bundle,
retry owed). That passivated bound run makes the wake branch return
`(operation, nil)` forever: no error, no 409, no Super, no log. The defined
product-path recovery is unreachable.

This compounds `held-computer-*-crash` family damage: the crash loop's
mass-passivation quietly bricked the self-development retry surface.

## 2. Evidence (live, 2026-08-28, choir.news)

- Computer `computer-03335285269bdba4f94377e56879f9e6` active epoch 831 on
  `42d47604` (force-deploy CI `33187442230`), serving owner API 200s.
- `POST /api/computers/{id}/self-development/operations` with the operation's
  exact original prompt (request commitment recomputed locally =
  `ef1bca527ddc4914b25cebbdf4a80db4716185d9e5e1570d332b93f5c7dce555`, verified)
  and idempotency `effects-solitaire-start-2026-08-19T17:44Z` returned **200**
  with `state=executing`, `updated_at` frozen at `2026-08-19T17:45:05Z`.
- No new run appeared (newest run update stayed `2026-08-28T14:27:34Z`); no 409.
- Code: `internal/agentcore/api_self_development.go`
  `ensureSelfDevelopmentRun` StateExecuting branch returns `(operation, nil)`
  when `len(runs)==1` and `!selfDevelopmentSuperRunTerminal(runs[0].State)`;
  `selfDevelopmentSuperRunTerminal` omits `types.RunPassivated`.
- The 200-vs-401/409 elimination: mode receipt valid, commitment matched
  (otherwise 409 `idempotency key reused with different prompt`), unbind/start
  errors would surface as 409 (lines 252-256), a successful wake would mint a
  run (none exists). Only the non-terminal-bound-run branch yields a 200 no-op.
- Ops-bound run list depth: bound Supers predate 2026-08-22; `/api/runs` caps
  at 100 rows (newest-first), so the blocking run is not enumerable via the
  list API - by-design observability gap noted, not re-opened here.

## 3. Non-fixes

- Do not mint a fresh operation with a new idempotency key (violates the
  ratified single-request scheduling contract; duplicates the live op).
- Do not SQL-edit or SSH-mutate the run metadata.
- Do not widen passivation or change boot behavior.
- Do not raise caps or cancel anything.

## 4. Fix (general product path)

Treat `passivated` as terminal for the self-development wake: add
`types.RunPassivated` to `selfDevelopmentSuperRunTerminal`. A passivated run is
boot-restarted-interrupted by definition (`passivated_reason=runtime_restarted`)
and is never auto-resumed; unbinding it and starting the fresh persistent Super
is the same recovery the 08-16 landing defined for terminal Supers.
`unbindSelfDevelopmentSuper` performs a metadata-only update and needs no
transition change. Rollback: revert restores the silent no-op.

## 5. Acceptance

Retry POST on the live operation returns 200 **and** a new persistent Super run
is minted; the Super proceeds to `assign_co_super` of a fresh CoSuper
assignment (candidate A authorship resumes). Tests:
`go test ./internal/agentcore -run 'SelfDevelopment'` covers the wake branch;
extend to assert a passivated bound Super is unbound and replaced.
