# Choir Run-Truth Suite

## Harness Invocation Semantics

```text
/goal docs/definitions/choir-run-truth-suite-2026-07-11.md
```

Read this document as the **suite index** for run-truth work. It does not
authorize code mutation by itself. It sequences member Definitions and forbids
mixing mission kinds inside one `/goal`.

Member harness entry points (run one at a time, in order):

```text
/goal docs/definitions/choir-run-deploy-unblock-2026-07-11.md
/goal docs/definitions/choir-wire-store-conformance-2026-07-11.md
/goal docs/definitions/choir-run-lifecycle-and-completion-authority-2026-07-11.md
/goal docs/definitions/choir-vocabulary-cutover-2026-07-11.md
```

## Why this suite exists

Owner direction 2026-07-11: stop bolting **correctness**, **deletion**, and
**rename** into one mega-mission. A month of mixed refactors produced little
tangible staging proof. Foliate by exit receipt:

| Kind | Done when | Owned here |
|------|-----------|------------|
| **Correctness** | Staging operator/product proof | Members 1–3 |
| **Deletion** | Heresy detector quiet + net lines down | `og-dolt-heresy-completion` Phases B/C (and related E deletions) — **not** this suite |
| **Rename cutover** | New names live, aliases drained, staging green | Member 4 (coordinates with og-dolt Phase E; does not delete continuation/parent-child) |

## Suite sequence (strict)

1. **Deploy unblock** (`choir-run-deploy-unblock-2026-07-11`) — Correctness.
   Drain the stuck `running` run; restore `Deploy to Staging (Node B)`.
   Tangible first win. Executable now.
2. **Wire-store conformance** (`choir-wire-store-conformance-2026-07-11`) —
   Correctness. World-wire on corpusd; delete boot migration; no VM fate-share
   on stories. No rename ceremonies.
3. **Run lifecycle and completion authority**
   (`choir-run-lifecycle-and-completion-authority-2026-07-11`) — Correctness.
   Single `RunRecord.State` authority, retry, artifact-verified completion,
   `choir run status`. **No naming sweeps.** Depends on members 1–2.
4. **Vocabulary cutover** (`choir-vocabulary-cutover-2026-07-11`) — Rename.
   `loop.*`→`run.*`, prompt-bar submission API, and coordinated
   universal-wire / sandbox / H019 sweeps. Runs only after member 3 is
   complete. Does not reopen continuation or parent/child deletion.

**Do not** chain-execute a later member from inside an earlier one. Advance the
suite by completing the current member's completion semantics, updating
`docs/ACTIVE.md`, then invoking the next `/goal`.

## Atomic coupling rules

- Behavior change XOR rename of the same symbols — never both in one landing.
- New correctness code is born in successor vocabulary (progress deadline, not
  lease; run, not loop-as-product) without purging old names in the same mission.
- Deleting production callers travels with deleting the API (Standing Q3).
- Deploy wait + stuck `running` drain stay together (member 1).
- Wire stories read path + boot-migration deletion stay together (member 2).

## Out of suite (do not absorb)

- Continuation deletion (H006–H008) — og-dolt Phase C.
- Parent/child deletion (H001–H005) — og-dolt Phase B.
- H025 result-channel API deletion — og-dolt Phase E (deletion), not B/C.
- Browser / Super Console / docs-only "platform Dolt" renames — their owning
  missions.
- Autoputer CLI phases after run truth (deploy receipts, promotion, keys) —
  `choir-autoputer-cli-operability-2026-07-11`.

## Suite completion

The suite is complete when members 1–4 each report `complete` on staging under
their own Definition. Deletion heresies remaining on og-dolt are tracked there;
they are not suite exit criteria.

## Supersession Record

- Owner-ratified foliation 2026-07-11: separate correctness / deletion / rename.
- Supersedes the 2026-07-11 attempt to fold naming Phase F and completion
  criterion 7 into `choir-run-lifecycle-and-completion-authority-2026-07-11`
  (those contents move to `choir-vocabulary-cutover-2026-07-11`).
- Does not supersede `og-dolt-heresy-completion-2026-07-08` Phase E ownership of
  surface cleanup; member 4 must record coordination rather than silently
  duplicate or race it.

## Red-Class Ceremony

- **Mutation class:** green (this index only).
- Member Definitions carry their own red/orange ceremony.
- **Heresy delta:** `discovered` — mega-mission mixing kinds blocked tangible
  progress; `repaired` — suite foliation by exit receipt.
