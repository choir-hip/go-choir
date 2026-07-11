# Choir Vocabulary Cutover

## Harness Invocation Semantics

```text
/goal docs/definitions/choir-vocabulary-cutover-2026-07-11.md
```

Read this document as executable semantic authority for **rename-only**
cutovers after run-truth correctness is staging-proven. It is **member 4** of
`docs/definitions/choir-run-truth-suite-2026-07-11.md` (Rename). It must not
run until suite members 1–3 are complete.

## Why this mission exists

Doctrine terms and code names drifted (`loop.*` events for runs, lease-shaped
identifiers, `universal-wire` vs world-wire, host `sandbox` vs autoputer).
Folding those renames into the run-lifecycle correctness mission blocked
tangible completion. This Definition owns rename cutovers as their own exit
receipt: staging green under new names with aliases drained.

## Mission kind

**Rename cutover only.** No behavior changes except those required to keep
aliases working during cutover. If a sweep needs behavior change, stop, file it
under the owning Correctness or Deletion mission, and resume rename later.

## Source Authority Order

1. This Definition.
2. `docs/definitions/choir-run-truth-suite-2026-07-11.md`.
3. `docs/choir-doctrine.md` (H019 and naming notes),
   `docs/definitions/og-dolt-heresy-completion-2026-07-08.md` Phase E
   (surface cleanup coordination).
4. `docs/standing-questions.md` (Q3 citers), `AGENTS.md`.
5. Observed baselines (2026-07-11, refresh at Phase 0):
   - `universal wire` ~619 non-docs matches; `world wire` ~0 in code
   - host `sandbox` packages + ~77 CI refs
   - H019 lease detectors ~90±10 non-`release` in `internal/`+`cmd/`
   - `"loop.` wire strings: 28
   - `prompt-bar`/`PromptBar`: ~271 in `internal/`+`cmd/` (~333 with frontend)

## Settled Inputs

- Suite members 1–3 complete on staging before this `/goal` starts.
- og-dolt Phase E already lists Universal→World Wire, sandbox→autoputer, and
  H019–H029 surface work. **This mission executes the rename sweeps once** and
  records coordination in both evidence ledgers so E does not redo them.
- Continuation / parent-child **deletion** remains og-dolt B/C — renaming an
  event string that contains `continuation` is allowed; deleting continuation
  machinery is forbidden here.
- H025 unqualified result-channel deletion is og-dolt E deletion work, not a
  rename sweep here.

## Mission Purpose

Iterative rename sweeps until non-allowlisted residue is empty and staging is
green under the new names:

1. `universal wire` → `world wire` (all casings), with `/api/world-wire/*`
   canonical and `/api/universal-wire/*` alias until cutover proven.
2. Choir-host `sandbox` → `autoputer` (not OS/browser/test sandboxing; those
   stay allowlisted). Same-commit `deploy-impact-classify` updates.
3. H019 lease vocabulary → worker handle / activation budget / progress
   deadline / trajectory obligation (detectors: `lease`, `leased`,
   `worker lease`, `lease_seconds`). Beware `release` / `please` false positives.
4. `loop.*` → `run.*` event wire strings; read-side aliases until frontend
   cutover proven; consider `internal/types/task.go` → `run.go`.
5. Prompt-bar vs run submission: keep `prompt-bar` as UI input surface name;
   submission object/API → `/api/runs/*` with serving alias.

## Mission Non-Purpose

- No run admission, retry, artifact, or Deploy-unblock work (members 1–3).
- No continuation / parent-child / H025 deletion.
- No VM instance rename requiring reprovision (`vm-universal-wire-platform`
  allowlisted with follow-on pointer).
- No wire-store behavior changes (member 2 Non-Purpose: no rename ceremonies
  during conformance — this mission runs *after* that conformance).

## Completion Semantics

Complete when:

1. Each sweep's detector has zero non-allowlisted hits.
2. Per-iteration consensus concurs residue allowlist legitimacy and finds no
   hidden behavior change.
3. Staging green under new names (health, wire read path, one processor run
   e2e).
4. Coordination note landed in og-dolt Phase E ledger (sweeps not to be
   re-executed there).

## Sequencing and Gates

Iterative per sweep. Each iteration:

1. Enumerate + classify (rename / alias / allowlist).
2. Apply renames only; `go build ./...`, `go vet ./...`, tests, frontend build.
3. Consensus on diff + residue list.
4. Landing loop + staging QA.
5. Stop when a full pass finds zero non-allowlisted hits and panel concurs.

**Rollback ref:** SHA before each sweep landing. Halt-on-red; documented
failure is an accepted outcome.

## Compatibility rules

- HTTP aliases until staging-proven cutover; remove alias in a later iteration.
- Package/dir renames update `deploy-impact-classify` in the same commit
  without changing classifications.
- Artifact/unit/systemd renames prove themselves via their own deploy.

## Out of scope (registry)

- `continuation` / parent-child **control deletion** → og-dolt B/C
- H025 result-channel **deletion** → og-dolt E
- Browser (H029), Terminal→Super Console, docs-side "platform Dolt" → owners
  named in suite index

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
