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

## Ledger, second form (from 2026-11-10)

Owner, 2026-11-10: "we should have a ledger of precommitments that we score
over time." The tables above stay as they are for the importer. From here
the ledger is two append-only tables and a scorecard.

Changes from the first form, and why:

- **Claims and settlements are separate tables.** A claim row is never
  touched again; settling appends a row to the second table. Nothing moves,
  so nothing can be quietly reworded after the fact.
- **Every claim states its own test.** The claim is written so that a
  stranger with the named evidence could mark it true or false. "Works" and
  "is ready" are not claims; a number, a count or an observable event is.
- **Every claim has a settle-by time.** A claim not settled by then is
  recorded as `lapsed`. Lapsed claims are counted and reported: a ledger
  that only settles its easy claims is not a track record.
- **Each claim has a kind,** so the score can be read by kind: `ready` (it
  passes where it has not run yet), `effect` (a change moves a number),
  `estimate` (a quantity not yet measured), `diagnosis` (the cause of
  something seen), `schedule` (done by a time), `judgment` (what a person
  will decide).
- **Pairs are encouraged.** The same number claimed at two thresholds (below
  500, below 300) says more about calibration than one claim.
- **Unqualified statements still count.** A bare "tests pass" or "this is
  safe" from the lead or a builder is entered at 0.95, as before.

Rules: a claim is entered before its evidence exists, with the time. `p` is
the probability the claim is true. Outcome is `true`, `false`, `void` (the
test became impossible; say why) or `lapsed`. Score is Brier,
(p − outcome)², over true and false only. The scorecard is recomputed when
settlements are appended and reports count, mean Brier, and for each
probability band the share that came true. Who: `lead` is the orchestrating
session; a builder or reviewer is named by its lane.

The claims of the first form numbered 13 to 19 were never settled. They are
entered below as lapsed, not scored.

### Claims

| id | recorded (UTC) | who | kind | claim, with its test | p | settle by | evidence source |
|---|---|---|---|---|---|---|---|
| C20 | 2026-11-10T22:10Z | lead | ready | The System 1 client route lands on main through `scripts/land` on its first run (suites pass, pushed). | 0.80 | 2026-11-11T02:00Z | land log |
| C21 | 2026-11-10T22:10Z | lead | effect | On staging, `scripts/proofs/system1-route.sh` against d@e returns both answers among the offered choices with `total_ms` under 1000 on its first run. | 0.65 | 2026-11-12T00:00Z | proof output |
| C22 | 2026-11-10T22:10Z | lead | estimate | Over 20 consecutive calls of that route on d@e, median `total_ms` is under 500. | 0.60 | 2026-11-12T00:00Z | proof output |
| C23 | 2026-11-10T22:10Z | lead | effect | In the first three whole hours after the intake flush is deployed on the paper, the paper's Jev calls per hour average under 500 (about 890 before). | 0.70 | 2026-11-12T00:00Z | gateway `jev call` lines |
| C24 | 2026-11-10T22:10Z | lead | effect | Same window: under 300. | 0.30 | 2026-11-12T00:00Z | gateway `jev call` lines |
| C25 | 2026-11-10T22:10Z | lead | effect | Same window: no hour has a same-second burst of four or more Jev calls from the paper (bursts of six to eight were routine before). | 0.60 | 2026-11-12T00:00Z | gateway `jev call` lines |
| C26 | 2026-11-10T22:10Z | lead | ready | The intake flush branch, after its repair, passes the whole agentcore suite under `scripts/land` on the first run. | 0.60 | 2026-11-11T12:00Z | land log |
| C27 | 2026-11-10T22:10Z | builder (intake) | ready | "Focused tests, build, vet and gates pass" implies the branch is sound: no blocking-real defect is found in it by review or by staging within 48 hours of deploy. | 0.95 | 2026-11-13T00:00Z | review outputs, ledger |
| C28 | 2026-11-10T22:10Z | lead | effect | In the 48 hours after the gateway request bucket is removed, Devin quota refusals (gateway `status 429` lines for devin) per day are no higher than 1.5 times the count for 2026-11-10. | 0.70 | 2026-11-13T12:00Z | gateway journal |
| C29 | 2026-11-10T22:10Z | lead | effect | In those 48 hours no provider's circuit opens because of one computer's burst, as shown by the hourly calls line (a computer over 3 times its own median hour immediately before the trip). | 0.85 | 2026-11-13T12:00Z | gateway journal |
| C30 | 2026-11-10T22:10Z | lead | estimate | In the 7 days after the Jev limiter's rate caps are removed, the gateway logs at least one upstream 429 from the Jev provider. | 0.15 | 7 days after that deploy | gateway `jev hour summary` |
| C31 | 2026-11-10T22:10Z | lead | schedule | Within two hours of the owner signing in to Xcode, the lab app is installed and launched on the owner's iPhone with no further owner action except trusting the developer certificate. | 0.45 | two hours after sign-in | device install output |
| C32 | 2026-11-10T22:10Z | lead | diagnosis | The simulator exit right after the audio claim in the Exact spike is caused by microphone permission (a missing usage string or an unanswered prompt), not by the engine or the driver. | 0.50 | 2026-11-12T00:00Z | lane K report |
| C33 | 2026-11-10T22:10Z | lead | estimate | On the iPhone 14 Pro Max, with a capture claim in default mode, A2DP allowed and no voice processing, playback to AirPods stays on A2DP while the phone's own microphone is captured. | 0.55 | 2026-11-13T00:00Z | device log of the route |
| C34 | 2026-11-10T22:10Z | lead | estimate | Releasing and re-claiming the audio session to switch between listening and talking takes under 300 ms on that phone (median of 10). | 0.50 | 2026-11-13T00:00Z | kit events |
| C35 | 2026-11-10T22:10Z | lead | estimate | LFM3-Audio-2B through FluidAudio's Core ML build loads and transcribes one minute of speech on that phone without the app being killed for memory. | 0.40 | 2026-11-14T00:00Z | device log, turn record |
| C36 | 2026-11-10T22:10Z | lead | estimate | Its live transcription runs faster than real time on that phone. | 0.30 | 2026-11-14T00:00Z | turn record |
| C37 | 2026-11-10T22:10Z | lead | estimate | Qwen3-0.6B as a one-token scorer answers a prompt of about 1,500 tokens in under 500 ms to first token on that phone, warm. | 0.45 | 2026-11-14T00:00Z | turn record |
| C38 | 2026-11-10T22:10Z | lead | estimate | That scorer agrees with Jev on at least 85% of a fixed set of at least 50 of our own questions. | 0.35 | 2026-11-17T00:00Z | replay records |
| C39 | 2026-11-10T22:10Z | lead | schedule | By 2026-11-11T12:00Z the whole loop has run once on the simulator or the phone against staging: sign in, fork a story, hear it, speak, and hear the desk's revision. | 0.35 | 2026-11-11T12:00Z | turn record, document id |
| C40 | 2026-11-10T22:10Z | lead | judgment | In the owner's first voice QA session, at least one mode (button, word, open or hybrid) is judged above the usability threshold. | 0.30 | the session, or lapse 2026-11-17 | owner's verdict |
| C41 | 2026-11-10T22:10Z | lead | estimate | On the word test bench, "tetra" reaches 90% recall with under one false accept per hour of programme audio, with the first-stage spotter and the confirmation step. | 0.30 | 2026-11-17T00:00Z | bench output |
| C42 | 2026-11-10T22:10Z | lead | estimate | When the scorer packing measurement runs (100 stories, packed three to a call against one to a call), importance agrees within one point on at least 90% of stories. | 0.60 | 2026-11-24T00:00Z | measurement output |
| C43 | 2026-11-10T22:10Z | lead | effect | The first deploy with no expedite anywhere in its range walks R0 to R5 with no ring held more than 20 minutes and no rollback. | 0.60 | 2026-11-24T00:00Z | vmctl journal |
| C44 | 2026-11-10T22:10Z | lead | diagnosis | `TestResearchChildModelCallsShareOneComputerGate` fails again in the next ten whole-suite runs of agentcore (land or CI). | 0.35 | after ten runs | land logs, CI |
| C45 | 2026-11-10T22:10Z | lead | effect | With the judges converted to one call at a time, no judge site's work is dropped for a deadline in the paper's first 24 hours (no new "budget expired" or deadline lines from those sites beyond the day before). | 0.65 | 24 hours after deploy | paper log lines via host, ledger |
| C46 | 2026-11-11T10:10Z | lead | ready | The Gemini review seat, run on the four parts of the Autoradio app diff (`lane/ar-app` at 3d1b1934c), reports at least one blocking finding that I judge real after reading the code. | 0.60 | 2026-11-11T14:00Z | review outputs |
| C47 | 2026-11-11T10:10Z | lead | ready | After any repairs, the app branch lands on main through `scripts/land` on its first run. | 0.60 | 2026-11-11T18:00Z | land log |

### Settlements

| id | settled (UTC) | outcome | evidence | Brier |
|---|---|---|---|---|
| 13 | 2026-11-10T22:10Z | lapsed | never settled; no settle-by time in the first form | n/a |
| 14 | 2026-11-10T22:10Z | lapsed | same | n/a |
| 15 | 2026-11-10T22:10Z | lapsed | same | n/a |
| 16 | 2026-11-10T22:10Z | lapsed | same | n/a |
| 17 | 2026-11-10T22:10Z | lapsed | same | n/a |
| 18 | 2026-11-10T22:10Z | lapsed | same | n/a |
| 19 | 2026-11-10T22:10Z | lapsed | same | n/a |
| C46 | 2026-11-11T10:40Z | true | four review outputs, verdict repair on each; the repair builder judged the key import, sanitizer, host check, audio continuations, shared state and stray tasks real | 0.16 |
| C47 | 2026-11-11T10:48Z | true | `scripts/land land/ar-app` exit 0 on its first run, main at 29e01354f (the staging deploy was then refused for disk headroom, which the claim did not cover) | 0.16 |

### Scorecard

Second form: 26 claims open, 2 settled. First form: 12 settled (7 true, 4
false, 1 partial), 7 lapsed.

| band | claims settled | came true |
|---|---|---|
| 0.0 to 0.3 | 0 | n/a |
| 0.3 to 0.6 | 0 | n/a |
| 0.6 to 0.8 | 2 | 2 |
| 0.8 to 1.0 | 0 | n/a |
