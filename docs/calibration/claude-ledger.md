# Claude's Calibration Ledger (Level 1)

Purpose: record the checkable claims Claude (the lead and its subagents)
makes while building Choir, with a probability where one is stated or
implied, and settle each against independent evidence (CI, staging,
deployed behaviour, later traces). This is the first, Markdown-only form
of scoring Claude; level 2 writes the same records through the Choir CLI
or MCP into a Choir computer, so the pattern works for any agent outside
this repo (owner, 2026-10-14). It doubles as an experiment: does showing
an agent its own track record in context improve its calibration?

Rules:
- A claim is recorded before its settlement evidence exists; settled rows
  are never edited, only appended to.
- Implied confidence for an unqualified assertion ("tests pass", "no
  behaviour change") is 0.95 unless the claim says otherwise.
- Settlement cites evidence (commit, CI run, staging check, problem doc).
- Scoring: Brier score per claim (p - outcome)^2; reported per agent kind
  and per claim type, with counts.

Level 2 (`choir precommit`, `choir mcp`):
[docs/archive/reports/choir-mcp-precommitments-2026-10-14.md](../archive/reports/choir-mcp-precommitments-2026-10-14.md).
`choir precommit import-ledger` reads the two tables below, so keep their
columns as they are.

## Settled claims, 2026-10-14 (reconstructed the same day from the session)

| # | Who | Claim | p | Outcome | Evidence |
|---|---|---|---|---|---|
| 1 | subagent (S2a build) | S2a delegation is ready: all package and shard tests pass | 0.95 | false in production: every task refused by the V2 vocabulary fence | docs/archive/problems/s2a-management-tasks-refused-by-vocab-fence-2026-10-14.md |
| 2 | subagent (search benching) | the bench fix is correct; `go test -race` passes | 0.95 | false: overflow to a negative bench on amd64 (passed on arm64) | CI 38579173174; bdc996432 |
| 3 | lead | "the search fix is live" (said before CI finished) | 0.95 | false: CI had failed | lead correction at ~15:00 UTC |
| 4 | lead | the claims lab package will not trigger a deploy | 0.9 | false at first: the classifier flagged it; fixed by a rule | 023ab7312 |
| 5 | lead | the yaegikernel test failure is a timing flake unrelated to the deploy | 0.9 | true: passed 8/8 locally; the next CI run passed | CI 38565529909 |
| 6 | subagent (LF0) | missing scores leave the front page exactly as before | 0.95 | true | deployed 9a0da89ba; front page checked |
| 7 | subagent (L2a trace) | Jev labels will flow once the peer re-pin deploys | 0.9 | true: 25 labelled, 0 unlabelled after deploy | lead check ~17:35 UTC |
| 8 | subagent (firewall) | live guests keep egress through the chain swap | 0.95 | true: 6,224 packets accepted, 0 dropped; paper kept publishing | lead check ~18:40 UTC |
| 9 | subagent (object graph gate) | registering every written kind means no live write is refused | 0.9 | true so far: 0 refused writes after deploy | lead check ~18:05 UTC |
| 10 | lead | the self-development drill reaches Freeze via the goproxy | 0.7 | false: the capsule could not build bin/autoputer | docs/archive/problems/selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md |
| 11 | subagent (ranking pass) | new criteria move the bugging story from 23rd to 9th | 0.8 | partly: scored 7 as predicted, then aged out of the newest-100 window | front list ~15:00 UTC |
| 12 | subagent (storage leak) | GC dry-run will report ~7.7 GB chunks + 0-15.9 GB bases | 0.8 | true: 7.74 GB chunks, 15.9 GB bases | corpusd journal 13:05 UTC |

First reading (n = 12): unqualified "tests pass / ready" claims failed in
production 2 of 3 times when production differed from the test
environment (vocabulary cutover flag; amd64 vs arm64). Lesson for the
prompt context: state p below 0.95 for "ready" claims unless the claim
was checked under production conditions.

## Open claims (recorded before settlement)

| # | Who | Claim | p | Settles by |
|---|---|---|---|---|
| 13 | lead | the jailer flag-off deploy causes no VM launch failure in the first hour | 0.85 | vmctl journal + verifier `--expect direct` |
| 14 | lead | with the jailer on for one test computer, boot, hibernate/wake and vmctl reattach all work first try | 0.5 | staging rollout step 2-3 |
| 15 | lead | admission + sustained reclaim causes no reclaim of a non-test computer in its first 24 h | 0.9 | vmctl journal |
| 16 | lead | guest store indexes cut the paper's CPU below 0.5 core | 0.6 | scripts/cpu_host_sample.py after deploy |
| 17 | lead | prompt-cache fix raises the paper's cached-input share above 50% within a day | 0.5 | /api/costs |
| 18 | lead | the capsule-build fix lets the drill reach Freeze for a frontend-only change | 0.6 | reeng tracer rerun |
| 19 | subagent (ICRL build) | the calibration digest gives evidence *for* H-ICRL on management: all three conditions of the precommitment hold at 500 settled hypotheses per arm (ECE gain ≥ 0.03, interval excluding 0; Brier not worse by > 0.005; resolution not down by > 0.01) | 0.45 | docs/archive/reports/icrl-calibration-experiment-2026-10-14.md §3, `scripts/icrl_calibration.py measure` |
