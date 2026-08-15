# Effects invoke-readiness — 2026-08-15

**Question:** may the owner invoke `/goal docs/definitions/choir-supervised-self-development-effects-2026-08-11.md` now?

**Panel:** `.agentic-consensus/effects-invoke-readiness-20260815/` (gitignored diagnostics; this receipt is the durable projection)

**Mode:** lateral. Required verdict: `GO` | `GO_WITH_CAVEATS` | `NO_GO` | `ESCALATE`

**Baseline observed at invoke:** `HEAD` = `origin/main` = `3c12a9bb`; worktree clean; staging product `4ac90583`; tape-recovery `complete`.

## Verdict

**GO_WITH_CAVEATS** (4/4 completed panelists). Runner exit 0 with `--keep-going`.

| Reviewer | Lens | Status | Verdict |
|---|---|---|---|
| Devin | first-session `must_preserve` | ok | GO_WITH_CAVEATS |
| OMP Gemini 3.6 | OwnerRecovery leak into promotion | ok | GO_WITH_CAVEATS |
| OMP Cursor Grok 4.5 | candidate-readiness assumption | ok | GO_WITH_CAVEATS |
| OMP GPT-5.6 Sol | effects-OFF integrity | ok | GO_WITH_CAVEATS |
| Codex CLI | stale-unknowns | failed | `--autoputer` flag rejected |
| Cursor Agent | epoch-8253 | timed out | no verdict |
| opencode | irreversible-email bundling | timed out | no verdict |
| OMP DeepSeek v4 Flash | completion_cutover as false-start | timed out | no verdict |

Hidden assumption all four broke: **registry promotion + predecessor complete ≠ invoke-safe.** External pointers name which file to load. They do not make every sentence inside that file currently executable.

## First slice required by the panel

Docs-only reconciliation of this Definition:

- Pin `now.reconciliation` to staging `4ac90583` / `HEAD 3c12a9bb`.
- Consume tape-recovery receipts; do not re-prove restore.
- Preserve `start.unknowns` as historical observation; add a dated correction so they are not the live backlog.
- Retire route-map item 1 as a live "do first."
- Annotate `completion_cutover.doctrine-promote` SPA language as closed.
- Do not touch policy code, restore, or effects in that slice.

## Forbidden on invoke

- Rematerialize, or independently green restore
- Invent `choir computer create`
- Enable effects / run actuators
- Delete `external-owner:` / `accept_once` / `awaiting_approval` before a consensus receipt and reducer exist
- Use an `OwnerRecovery` checkpoint for promotion (route projection already refuses it)
- Open the irreversible-email path in session one
- Treat epoch `8253` as the current retained epoch (paid restore is epoch **268**)
- Let a CoSuper packet open privileged Super execution

## Not blockers

| Item | Disposition |
|---|---|
| Epoch `8253` / `ak_45ce1796` | Named residual. Classify before red mutation; do not reopen tape-recovery |
| Irreversible email | Same Definition, not first session |
| Kill-loop problem | Repaired at `db265d1e`; not current readiness proof |
| Owner-recovery security review | Post-mission; not an effects gate |
| Completion cutover | Part of `goal.complete`, not a reason to delay invoke |

Effects remain OFF until decision-policy rehearsal actually passes.
