# Choir Vocabulary Cutover

## Subordinate Invocation Semantics

This document is the R7 rename specification of:

```text
/goal docs/definitions/choir-autoputer-completion-2026-07-13.md
```

Do not invoke it independently. It runs only after R1-R6 are complete, so
rename work cannot preserve code that runtime dissolution should delete.

## Why this mission exists

Doctrine terms and code names drifted (`loop.*` events for runs, lease-shaped
identifiers, `universal-wire` vs world-wire, host `sandbox` vs autoputer).
Folding those renames into the run-lifecycle correctness mission blocked
tangible completion. This Definition owns rename cutovers as their own exit
receipt: staging green under new names with aliases drained.

## Mission kind

**Rename cutover only.** No behavior changes. Every caller moves under one
clean-cutover mutation; compatibility aliases are forbidden. If a sweep needs
behavior change, stop, return it to the owning phase, and resume rename later.

## Source Authority Order

1. `docs/definitions/choir-autoputer-completion-2026-07-13.md`.
2. This subordinate Definition within R7 scope.
3. `docs/choir-doctrine.md` and
   `docs/definitions/og-dolt-heresy-completion-2026-07-08.md` Phase E.
4. `docs/standing-questions.md`, `AGENTS.md`.
5. Observed baselines (2026-07-11, refresh at Phase 0):
   - `universal wire` ~619 non-docs matches; `world wire` ~0 in code
   - host `sandbox` packages + ~77 CI refs
   - H019 lease detectors ~90±10 non-`release` in `internal/`+`cmd/`
   - `"loop.` wire strings: 28
   - `prompt-bar`/`PromptBar`: ~271 in `internal/`+`cmd/` (~333 with frontend)

## Settled Inputs

- R1-R6 are complete on staging before R7 starts.
- og-dolt Phase E supplies surface-deletion detector contracts. R7 executes
  each rename once and updates the active mission capsule.
- Continuation, parent-child, and result-channel deletion must already have
  occurred in R1 where production callers proved it safe.
- Every rename and all callers cut over atomically; old names and compatibility
  aliases are deleted in the same landing.

## Mission Purpose

Clean rename cutovers until non-allowlisted residue is empty and staging is
green under the new names:

1. `universal wire` → `world wire` (all casings), including an atomic
   `/api/world-wire/*` route and caller cutover with no old HTTP alias.
2. Choir-host `sandbox` → `autoputer` (not OS/browser/test sandboxing; those
   stay allowlisted). Same-commit `deploy-impact-classify` updates.
3. H019 lease vocabulary → worker handle / activation budget / progress
   deadline / trajectory obligation (detectors: `lease`, `leased`,
   `worker lease`, `lease_seconds`). Beware `release` / `please` false positives.
4. `loop.*` → `run.*` event wire strings, with producers and consumers moved in
   one landing; consider `internal/types/task.go` → `run.go`.
5. Prompt-bar vs run submission: keep `prompt-bar` as UI input surface name;
   submission object/API → `/api/runs/*` in one clean cutover.

## Mission Non-Purpose

- No run admission, retry, artifact, or Deploy-unblock work (settled Deploy
  receipt and R4).
- No continuation / parent-child / H025 deletion (R1).
- No VM instance rename requiring reprovision (`vm-universal-wire-platform`
  allowlisted with follow-on pointer).
- No wire-store behavior changes (settled Wire receipt; R7 runs only after R1-R6).

## Completion Semantics

Complete when:

1. Each sweep's detector has zero non-allowlisted hits.
2. Independent review of each immutable candidate and residue allowlist finds
   no hidden behavior change or compatibility alias.
3. Staging green under new names (health, wire read path, one processor run
   e2e).
4. Coordination note landed in og-dolt Phase E ledger (sweeps not to be
   re-executed there).

## Sequencing and Gates

Iterative per sweep. Each iteration:

1. Enumerate + classify (rename / delete / allowlist).
2. Apply the rename and every caller atomically; run build, vet, focused tests,
   and the frontend build where affected.
3. Apply the active mission's assurance profile to the immutable candidate.
4. Run the landing loop and staging product proof.
5. Stop when a full pass finds zero non-allowlisted hits.

**Rollback ref:** SHA before each sweep landing. Halt-on-red; documented
failure is an accepted outcome.

## Compatibility rules

- Compatibility aliases are forbidden; route, event, API, package, and caller
  names move together and old names are deleted in the same landing.
- Package/dir renames update `deploy-impact-classify` in the same commit
  without changing classifications.
- Artifact/unit/systemd renames prove themselves via their own deploy.

## Out of scope (registry)

- `continuation` / parent-child **control deletion** → og-dolt B/C
- H025 result-channel **deletion** → og-dolt E
- Browser (H029), Terminal→Super Console, docs-side "platform Dolt" → owners
  named in the active mission.

## Supersession Record

- Receives former run-lifecycle Phase F + completion criterion 7 (2026-07-11
  suite foliation).
- Owner-directed naming scope preserved; ownership made exclusive vs racing
  og-dolt E by coordination receipt.

## Red-Class Ceremony

- **Mutation class:** green for this doc; renames touching routes/deploy/CI are
  red and use full gate protocol.
- **Autonomous:** defaults + recorded deviations; no owner pause.
- **Heresy delta:** `discovered` — vocabulary drift across code/CI; `repaired`
  when detectors quiet under successor names.
