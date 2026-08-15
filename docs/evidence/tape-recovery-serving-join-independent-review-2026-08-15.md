# Tape-recovery serving-join independent review

- **Date:** 2026-08-15
- **Mode:** convergent
- **Candidate:** deployed staging `4ac90583` plus uncommitted serving-join evidence
- **Panel dir:** `.agentic-consensus/tape-recovery-serving-join-20260815/`
- **Disposition:** `ACCEPT` (product receipts); docs-only terminal-closure land stamps Definition `complete`

## Panel receipt

Eight default-panel routes were attempted. Five completed:

| Reviewer | Status | Verdict |
|---|---|---|
| Devin | ok | REPAIR (docs/registry land only; serving_join paid) |
| OMP Gemini 3.6 Flash | ok | ACCEPT |
| OMP Cursor Grok 4.5 | ok | ACCEPT |
| OMP GPT-5.6 Sol | ok | ACCEPT |
| Cursor Agent | ok | ACCEPT |
| Codex CLI | failed | `--autoputer` flag rejected by current `codex exec` |
| opencode | timed-out | no verdict |
| OMP DeepSeek v4 Flash | timed-out | no verdict |

The repair vote does not unpaid a receipt. Devin agreed serving_join is paid and the floor passes; it refused an immediate `complete` stamp until `docs/ACTIVE.md`, `docs/mission-graph.yaml`, and the uncommitted evidence files land. Four ACCEPT votes said the same thing with `STAMP_COMPLETE: no` (Gemini said `yes` only as part of that same land commit).

## Consensus that survived

1. **`serving_join` is paid.** Live 2026-08-15T17:10Z re-probe on node-b matched the receipt: unsigned `https://choir.news/` sha `4e2d1954` (`index-YTmyLpSn.js`, proxy) ≠ retained `computer-033352…` epoch 268 sha `2c74a7b0` (`index-BH09hKq-.js`, autoputer) ≠ secondary `computer-bb0f4fa…` epoch 12 sha `1e62d8b9` (`index-BgRdleu6.js`, autoputer). Hop is guest-static ComputerSurface after vmctl resolve (`internal/proxy/computer_surface.go`, `internal/autoputer/computer_surface.go`).
2. **SSH restage of the secondary SPA does not unpaid the hop.** It repaired a pre-rename `sandbox_id` BIOS loop. `not_done_when` forbids restore reachable only through SSH; restore already used `choir computer restore`. The serving-join receipt `not_claimed` RestagePinnedRelease / owner restore.
3. **Bundled floor item passes.** Checkpoint + rematerialization + serving join were reviewed together against the six deployed receipts. Prior panels (replace-workspace, owner-recovery publication, store reopen) are not this floor item; this panel is.
4. **Do not stamp `complete` until the docs land.** Uncommitted Definition/evidence plus ACTIVE/mission-graph still saying `blocked` on one interactive VM is a registry-hygiene blocker, not a product blocker.
5. **Owner-recovery security review is correctly deferred** (owner 2026-08-14). Not a tape-recovery complete gate. Effects stay OFF.

## Adjudication

| Question | Decision |
|---|---|
| Q1 serving_join | paid |
| Q2 completion floor | pass |
| Q3 stamp complete now | no |
| Q4 next_action | docs-only terminal-closure commit |
| Q5 owner-recovery security review | deferred post-mission |

## Next action

Land one docs-only commit: this review, `tape-recovery-serving-join-2026-08-15.json`, `tape-recovery-secondary-bootstrap-incident-2026-08-15.json`, Definition `now`/`receipts`, `docs/ACTIVE.md`, `docs/mission-graph.yaml`. Do not rematerialize. Do not invent `choir computer create`. Do not enable effects. Stamp Definition `complete` in that commit, not before.

## Landed

This review authorized the docs-only terminal-closure commit that stamps Definition `complete`.
