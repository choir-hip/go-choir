# Re-engineering plan review, 2026-10-13

Owner request, 2026-10-13: "Run agentic consensus reviewing the plan rn.
Lateral and then convergent. And update the plan if intelligent signals
emerge." Plan under review: docs/definitions/choir-reeng-metamission-2026-10-12.md
(with its `boundaries.pace` ruling) and the three cut files.

## Panels

- **Lateral** (ten models; glm-5.3-flash returned nothing). Prompt:
  `.agentic-consensus/reeng-plan-prompt-2026-10-13.md`; manifest
  `.agentic-consensus/reeng-plan-lateral-2026-10-13/` (session
  diagnostics, not committed).
- **Convergent** on the thirteen candidate changes the lateral panel
  produced (nine of ten answered; muse-spark timed out). Prompt:
  `.agentic-consensus/reeng-plan-convergent-prompt-2026-10-13.md`; manifest
  `.agentic-consensus/reeng-plan-convergent-2026-10-13/`.

The lateral panel converged on one reframe from many angles (protocol
conformance testing, iron bird, wafer test structures, flight test cards).
The plan treated a whole AI-news story driven by live desks as the unit of
learning, so every deterministic substrate claim paid for 5 to 11 minutes of
research, a 16 to 25% cell-error rate and model discretion (whether the
model happens to make parallel calls). Split the instrument: a short,
claim-directed scenario per landing, and the full story at cut boundaries.

## Adjudication

| | candidate | verdict | why |
|---|---|---|---|
| C1 | claim-directed tracer scenarios | **adopted**, landed | 9 of 9. The crash scenario (`--scenario crash`) crashes at the first run in flight, watches a quiet window with no owner act, reads every run open at the crash, and accepts on that claim plus I3 (exit 6 when the claim fails). The story scenario runs at cut boundaries. |
| C2 | identity fence | **adopted**, landed | 9 of 9. `--expect-sha` waits for staging and an idle CI lane, and exits 5 on a build change mid-run. |
| C3 | I3 passes an observed quiet window | **adopted**, landed with the boundary fix | 9 of 9. gpt-6.1-sol showed by a synthetic bundle that an upper-bound `boot_at` hides a dispatch between the real boot and readiness. The checker now reads from the restart request when `boot_at` is an upper bound. |
| C4 | vacuous bundles never pass | **adopted**, landed | The checker exits 3 when no rule passed; scenario acceptance names the rules its claim reads. |
| C5 | cell-error analysis now | **adopted**, done | docs/reports/reeng-desk-cell-errors-2026-10-13.md: 21.8% of 308 cells fail; 52% of failures are kernel-fixable. The top mechanism (M1, a leading type/var/const) is fixed (Cut 1 breakage B3). |
| C6 | prompt truth with each mechanism | **adopted** | 8 of 9 (one abstained). Generated verb lists and instructions that a landing makes false change in that landing; L6 keeps the full rewrite and the reverse parity test. |
| C7 | scripted, model-free desk probe | **rejected** | 8 of 9. It adds a red provider-routing surface and resembles the shadow harnesses the cutover deletes. Fallback, unused so far: a tracer-controlled slow page to pin a research cell in flight. |
| C8 | quote-in-block census now | **adopted**, offline | Report missing quotes, raw bytes and block structure separately. Today's flattened captures cannot measure the future extractor's rate. |
| C9 | batch landings that move no verdict | **adopted with change** | Split vote. Pushes are batched by dependency (L2's deletions ride L3's push as separate commits). Every push that changes behavior still gets a claim-directed tracer run; no landing skips one. Recorded as a deviation from the letter of pace rule 5. |
| C10 | thin Autopaper loop after L4 | **deferred** to Cut 2 | 8 of 9. It would build on packets L3 deletes. The first public permalink also starts the freeze. |
| C11 | collapse L1-L3 | **rejected** | 9 of 9; L1 had already landed and was teaching. |
| C12 | management in the story | **adopted** at L4 and cut boundaries | No bundle shows a management turn. |
| C13 | Cut 2 live-window wording | **adopted**, landed | At most one hour watched, then the crash proof, then the owner act that restarts the schedule. The paper is running when the receipt is written. |

## Blocking findings and what was done

1. **L1 missed old open runs** (codex, gpt-6.1-sol). The owner-scoped run list
   limits headers before it filters states. Confirmed by a failing test, fixed
   (Cut 1 breakage B2).
2. **I3 boot-boundary blind interval** (gpt-6.1-sol). Fixed in the checker.
3. **Process success weaker than acceptance** (gpt-6.1-sol, gpt-6-luna,
   glm-5.3-flash, opencode). Fixed by C1 and C4.
4. **The queued L1 run had the old shape** (devin, gemini-3.8-flash,
   deepseek-v4.1-flash). It ran anyway as a field run. The L1 acceptance
   receipt comes from the crash scenario on the next build.
5. **Two things must land in L3** (deepseek-v4.1-flash). First, the
   planned-restart marker's `work_state_epoch` (decision D7). Without it, a
   marker written before L3 resumes runs into the records-only schema.
   Second, the checker re-expressed on records: I2, I3 and `deliveries` read
   work items and packets, which L3 deletes. Both were added to Cut 1's
   landing order.
6. **Pre-boot packets survive a crash boot until L3** (claude, glm-5.3-flash,
   opencode). This is a known residual. Owner re-entry may read I3 as
   ambiguous until L3 deletes packets; it is not an L1 regression.

## Not adopted, worth keeping

- **Start Autopaper's source schedule on a staging computer now** (devin,
  deepseek-v4.1-flash). Its purpose would be a corpus clock. It was not
  adopted because every cutover deploy is a crash boot that closes the
  schedule, so it would need re-arming after each landing. Revisit at Cut 2.
- **A per-bundle novelty diff** (devin). It would report records by kind,
  runs by fate and cells by error class between consecutive bundles, and flag
  behaviour that changed where no rule looks.

## Follow-up panel: Cut 1 scope split (07:40Z)

Prompt `.agentic-consensus/c1-scope-prompt-2026-10-13.md`, manifest
`.agentic-consensus/c1-scope-2026-10-13/`. Panel: claude, codex, gpt-6.1-sol,
gpt-6-luna, deepseek-v4.1-flash; codex and the two GPT anchors hit usage
limits, claude and deepseek answered. Both: **adopt with changes**. Adopted
changes:

- C1-core keeps a Texture turn that starts and commits from records with
  zero open work items, the planned-restart work_state_epoch (D7), round
  closure with a no-op second boot, late arrival, the reverse parity test
  and generated verb lists with the prompts of Texture, research and
  management, and a checker invariant that a records route mints no packet
  or work item.
- Sub-researchers (L5) are named: moved to C1e, with the all-children
  barrier on C2's research path recorded as a risk.
- Cut 2's engineering-backed 2a acceptance moves to C1e in the same commit.

Not adopted now: a typed Report verb replacing the packet adapter (claude).
The adapter already carries final, settled, dropped and stop_reason into
the typed body; the verb change rides the prompt work in C1-core if the
traces show the packet shape getting in the way.
