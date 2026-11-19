# Calibration experiment E1: the model or the harness (protocol, rewritten 2026-11-19, round 3)

Status: design (green). Station L2 of
[the night's goal file](../definitions/choir-night-exact-vectors-learning-2026-11-19.md).
Round 2 answered seat 1's review (four blocking findings) and the lead's
eleven decisions. Round 3 answers the second review (nineteen findings): the
driver `scripts/e1` and its self-test were **written by this change**, and
section 14 was rewritten against the code the driver calls. Tonight only a
**pilot** may run (section 14), **on three arms: S-a, S-b and I. Arm L is dropped
for the pilot** (section 5). The full run
waits for the owner's word. Deleting the 4 November arms is a separate red
landing that is due on main before 2026-11-24 (section 10). The file keeps its
old name so that existing links and the registry's `hold` lines still resolve.

Owner, 2026-11-19: "if the problems are at the model level, naively changing
the harness or prompt may not be correct. but this does relate to the
calibration experiment. both are about prompt/harness optimization and
learning. the calibration experiment is something that we should do now,
actually. you can redesign it as you see fit."

How claims are marked: **[read: file:line]** or **[read: record]** means read
for this design (seat 1 checked most of these against the code, and the round 3
author opened every route and field named in section 14 and in sections 3 and
7); **[assumed]** means not checked, and the smoke step checks it
(`scripts/e1 smoke` prints one PASS or FAIL line per check).

## 1. What E1-M is, and what it cannot conclude

**E1-M is a screen.** It runs the research desk on the same objectives with
the harness fixed and only the research model varied. It can show one of two
things: that the many-search tail is **specific to the paper's model** among
the models tried, or that it is **common to them**. It cannot show that the
harness causes the tail. A behaviour shared by every model is still
model-level behaviour in general, and that is the owner's warning. Only a
surface arm can name a harness cause, and that is E1-H (section 9), which
cannot start until the research eval route exists.

**The scope of every claim: a research run on a computer that holds no
captures from the paper, and only those of its own earlier objectives.** Each
arm runs on a fresh test computer. Captures are scoped to their owner
[read: internal/researchtools/researchtools.go:82], so an arm computer starts
with an empty corpus: true of objective 1 only. Objectives 2 to 10 on a
computer see the captures its own earlier objectives made; the objective's
position is a covariate and the readout shows it. The paper's corpus holds
about 207,000 pages; a run searches that corpus and the web and gets 40 results
a web search, so the comparison is between a large corpus and a nearly empty
one. The published-stories layer is the one
thing an arm computer shares with the paper: it is searched "from every
computer" [read: researchtools.go:105-107]. This difference is a confound, not
a risk. It narrows what the result means; it does not invalidate it. The
pilot compares the paper-model arms with the paper's own rates. If the tail
does not appear at all on an empty computer, E1-M stops there, and the
research eval route is built first (it replays a real run's first message and
held captures). Seeding a test computer from the paper's captures is out for
now.

## 2. The four failures, and whether an experiment is the right instrument

| Failure (record) | What it is | Instrument |
|---|---|---|
| One question reworded into 12 to 44 web searches (search-volume record, item 2: 10 of 54 runs started 10 or more searches and made 56% of all searches) | "Model behaviour under a harness that does not stop it", 0.65; not shown that another model does better [read: search-volume record, "Traces read", item 2] | **E1-M screens it.** Is the tail specific to `swe-2-flash`? |
| A run reads about 5% of the pages it is given (reads stay at 7 to 25 per run whether 80 or 1,680 pages come back) | Not established; the likeliest reading is that reads follow what the story needs (0.5) [read: same, item 3] | **Measured in E1-M, not tested.** Its lever (titles in the result) is E1-H arm H-a. |
| One page read 3,090 times (`c0351184`) | Seen once on any console Node B holds; the cause is not established [read: same, item 1] | **A defect to read first, not an experiment.** The second trace pass reads its cell programs (section 9 says what each finding changes). |
| Management chooses with `candidates_shown=0` (186 of 233 choices) | A broken instrument: the count moves only when the model calls `choir.Stories`, and the opening already holds the page [read: research-does-not-start-from-the-corpus record, last section] | **A defect to fix** (add the opening's count, yellow). The 67-byte cursor against the 64-byte bound is also a defect to fix (orange) [read: recurring-agent-failures record, item 1]. |

## 3. The unit and the outcome

**The unit is one objective on one arm.** One submission to the Texture eval
route produces one trajectory. Texture may cast to research zero, one or
several times. The unit counts **every research run of that trajectory,
summed**, and records the number of research runs beside the total.

**The rule for a submission that produces no research run**, fixed now:

- such an objective on such an arm is recorded as `no_research`;
- if any arm has `no_research` for an objective, that objective leaves every
  paired comparison, so the pairing holds;
- the `no_research` count per arm is reported. Texture runs on the same model
  on every arm, so an imbalance points at the research model's effect on
  Texture's casting, and is reported as that.

**Primary outcome: started web searches**, meaning the number of
`search_start` rows of the desk's egress that were not errors, over every
research run of the unit. The host writes the row when a desk cell starts an
asynchronous search and a `search_done` row when it ends
[read: internal/agentcore/tools_desk.go:665-717]; the start row rides the open
cell's `desk.cell` event as an `egress` element, the done row is its own
`desk.egress` event [read: cmd/choir/trace.go:715-768, which folds both]. The
reader counts distinct `(run_id, request_id)` start rows and distinct
`(run_id, search_id)` done rows, and takes the larger count (a run whose start
row was lost still counts). **This is a count of searches started, the
measure of the search-volume record** ("started 10 or more"
[read: search-volume record, "Traces read", item 2]). It includes a search the
gateway's result cache answered.

**The cost tally is a different number.** The host writes one `search.served`
line per finished search *that a provider answered*: the writer is guarded by
`len(done.providers) > 0` [read: internal/agentcore/tools_desk.go:699-703], so
a cache-answered search has no line (the record counts 44 of 332). The sum of
the providers' `Calls` over those lines [read: internal/types/search_served.go:8-35]
is the experiment's **tally**, the cost bound of section 7. The unit also
reports `cache_answered`, started less served lines, which is zero or more. The
model cannot write either row. A search made by a `choir.Research` child is not
a run and emits no desk row to the parent [read: cmd/choir/trace.go:413 and
internal/agentcore/research_children.go:382-404, which forwards only
`model.served`]; the pilot does not see those, and its count is of the research
desk's own searches.

**Which model served the unit** comes from `model.served` lines of the unit's
research runs: the lines with purpose `tool_loop`, leaving out the decision
model's (role `system1`). The record names the model that actually answered
each call "after any quota switch or pool failover", and its `Switches` list
each mid-run move [read: internal/types/model_served.go:7-43]. A research run
with any `Switches` entry, or with any call served by a model other than its
arm's (provider and catalogued name), is **invalid by rule**. The run's own
metadata names the overlay (`llm_policy_overlay_id`); a research run without
the arm's overlay id is invalid too. Texture's and the conductor's lines are on
their own runs and are not checked against the arm. The document route
`GET /api/texture/documents/{id}/models` gives the same facts by role plus the
providers' calls by role [read: internal/textureowner/texture_model_provenance.go:14-17];
the driver does not need it.

**A unit is valid when** all of these hold:

1. it has at least one research run;
2. every research run has ended (completed, failed or cancelled) within 60
   minutes of the submission (a run cancelled after its 60 minutes is invalid,
   and its searches stay in the tally);
3. no research run has a model switch or a foreign model;
4. its trajectory's trace bundle was exported with the runs and run events
   served;
5. its computer reported the same `build.commit` in
   `GET /api/runtime/observability` [read: internal/agentcore/api.go:871-874]
   at submission and at the objective's end, and that build matches the other
   arms' build for the same objective;
6. no document of its trajectory is published (section 11, risk 4).

A failed check 5 invalidates the objective on every arm. A unit with no
research run is recorded `no_research` (section 3, above).

**Secondary outcomes, counts only.** The reader prints, per unit: research
runs, started searches, `search_done` rows, `search.served` lines, provider
calls, cache-answered searches, failed starts (a `search_start` row with
`is_error`), `read` rows, model calls and input tokens, and the largest number
of started searches in one run. The egress rows carry the verb as `search_start`,
`search_done` or `read`; they do not say whether the cell called `SearchAll` or
the plain verb (that is in the cell source, which only E1-H's program analysis
reads), so **the design no longer reports searches by verb**. Per-subject
searches, new-host reads, per-cell bound refusals, and whether a run read a
paper story from the published layer need fields the driver does not read; they
are left to the second trace pass and to E1-H. The gateway journal is not a
source, because a scoped key cannot read it.

## 4. Method, arithmetic, and what each outcome means

**One computer per arm.** Every arm computer is made fresh for the
experiment. All arms get the same objectives in the same order. What is aligned
is the **submission**: the driver posts an objective to the four eval routes
together (four requests sent at once), and records each request's time. What
the route delivers after that is not the same input: the eval route runs the
conductor and Texture, and **Texture writes the research run's first message**
[read: internal/agentcore/runtime.go:4108-4116], so the research message, the
moment research starts and the number of research runs differ by arm, and each
arm has its own test owner (`scripts/qa_test_computer.mjs` registers
`<label>@example.com`; owner ids differ). The S-a/S-b pair measures that
variation on one model only; the other arms' variation from Texture's choice
is part of what the contrast measures, and the readout reports it (research
runs per unit, `no_research` per arm).

- No arm sees pages another arm captured.
- Every arm runs the same release, checked by the build at submission and at
  the end of each objective.
- What remains is symmetric by design: each computer accumulates its own
  captures over successive objectives. The objective's position (1 to n) is
  reported as a covariate, and the primary difference is also shown by
  position.

Within one computer the objectives run one after another. That is for
validity: a later objective must see only the captures of earlier ones on its
own computer. It is not spacing of model calls; no model call anywhere waits
on it.

**The noise floor is two computers on the paper's model** (S-a and S-b): the
same model on the same objective on two identical computers.

**The primary statistic.** For each objective:

d = mean(S-a, S-b) of ln(1 + searches) − mean(the other arms present: L and I, or I alone tonight) of ln(1 + searches)

The table reports the mean of d over valid objectives, with a 95% t interval
(and a bootstrap interval beside it as a check). The sd of d includes the
run-to-run noise and an objective-by-model interaction that the S-a/S-b
floor does not measure. The pilot estimates the sd of d directly.

**Sample size, from the observed numbers** [read: search-volume record, items 2 and 3]:

- median 4 searches a run, mean 7.5;
- on the log scale, a spread of (2.77 − 1.61) / 1.28 = 0.91, taken as 1.0;
- formula n = ((t₀.₉₇₅,ₙ₋₁ + t₀.₈,ₙ₋₁) × σ / Δ)², solved by iteration (z in
  brackets).

| Effect | Δ | n at σ = 1.0 | n at σ = 0.6 |
|---|---|---|---|
| half the searches | 0.69 | 19 (z: 17) | 9 (z: 6) |
| 30% fewer | 0.36 | 63 (z: 61) | 24 (z: 22) |

At n = 40 the smallest difference detected is (2.023 + 0.851) × 1.0 / √40
= 0.45 on the log scale, about 36% fewer searches, if σ is 1.0. The pilot's
10 objectives estimate σ. If σ is above 1.2, the full run needs 60
objectives to keep the same detectable difference. That is the full run's
decision, not the pilot's.

**Outcome and conclusion (the full run).** "No difference" uses an
equivalence margin of ln 1.5 = 0.405 (up to half again as many searches or a
third fewer), tested two one-sided at 5% each, which is the 90% interval.

| Outcome | Conclusion |
|---|---|
| mean d ≥ 0.405 and the 95% interval's lower bound > 0 | The mean number of started searches is higher on `swe-2-flash` than on the models tried. Whether the many-search tail is specific to it is for the tail share below, which is underpowered. The lever is the model choice. Change no harness for this tail on E1-M's strength. |
| the 90% interval lies inside ±0.405 | The models tried search alike, within 50%. The behaviour is common to them. Nothing is shown about the harness; E1-H is needed before any harness change. |
| mean d ≤ −0.405 and the 95% interval's upper bound < 0 | `swe-2-flash` starts fewer searches on average than the others. |
| anything else | Inconclusive, reported as such with the interval. |

At d = 0 and σ = 1.0, the 90% interval falls inside ±0.405 with probability
about 0.6 at n = 40. A failed equivalence test is therefore weak evidence of
a difference.

**Pre-registered secondary outcomes, with their power stated honestly:**

- **The tail share**: the share of units with 10 or more searches, S against
  the others, with a paired test over objectives. At the paper's 0.19 against
  0.05, an unpaired test needs about 84 per arm. With 40 paired objectives the
  power is roughly 0.5 [assumed; it depends on concordance]. A difference
  here alone is not a verdict.
- **The 90th percentile** of searches per arm, with a bootstrap interval. At
  40 units per arm the 90th percentile is the fourth-largest value, so it is
  descriptive.
- Thin against thick objectives, new-host reads, reads per run, the read
  loop: reported only. Any reading drawn from them is a hypothesis for E1-H,
  not a verdict. No multiple-comparison correction is applied because none of
  them decides anything.

## 5. Computers, models and provider routes

Rules: never the newspaper computer, never the owner's computer, never a real
user's. The five computers of the 4 November design hold nothing: each was
pinned to one model, so their arms confounded model with computer (old F7).
d@e holds nothing either: it is the proof ring and runs other proofs.

| Arm | Computer | Research model | Provider route | Marginal charge |
|---|---|---|---|---|
| S-a | `e1-sa` | `swe-2-flash`, effort `low` | `devin` (Devin's API through one person's CLI session); the default pool's lead and the paper's research model [read: internal/modelcatalog/catalog.go:516-523]. The provider maps `low` to the `swe-2-flash-medium` model id [read: internal/provider/devin.go:770-795] | none until 2026-12-04 [read: same, comment] |
| S-b | `e1-sb` | `swe-2-flash` | `devin`, as S-a | none |
| L (**dropped for the pilot**) | `e1-l` | `gpt-6-luna` | `chatgpt`: ChatGPT subscription billing through Codex OAuth, the Responses endpoint [read: internal/provider/provider.go:1268-1287; catalog.go:362-368] | none (subscription) |
| I | `e1-i` | `iceman-contributor-free` | `opencode-go` free model, probed with a tool call on 2026-11-05 at cost 0; the default pool's first fallback [read: catalog.go:249-260, :524] | none |

Why these, under the rules the lead gave:

- **Synthetic is out.** It is reserved for self-development
  [read: catalog.go, AutoradioPool comment, "Synthetic is reserved for self-development"].
- **River is out.** Every River route spends against a paid budget
  [read: catalog.go, SelfDevPool comment, "$10 a day"], and the owner is asleep.
- **Bedrock Haiku is out tonight.** It is paid.
- **OpenRouter is allowed only for Jev and free models**
  [read: catalog.go, "only use Jev via openrouter, and free models"]. Its free
  models (`flint`, `nemotron`) qualify, but they are the default pool's later
  fallbacks and showed quota refusals on 2026-11-09
  [read: docs/evidence/paper-token-budget-2026-11-09.md:355]. They are the
  stand-in if I fails the smoke step.
- **`beam-free`** is the other opencode-go free model; the catalogue says it
  "is often parked" [read: catalog.go, AutoradioPool comment].
- An owner note of 2026-10-21 said "stop testing luna". It was about that
  night's app rewrites [read: docs/reports/owner-notes-2026-10-14.md:905-916],
  and Luna is in the default pool today. L was kept in round 2 and **is dropped
  for the pilot of 2026-11-19** (decision D-E2, below). The gateway's journal
  shows why: the first Luna call of the day got `chatgpt: auth: refresh chatgpt
  auth via http: status 401`, the gateway marked the model a hard failure, and
  its circuit has stayed open since; the journal holds only two Luna calls in a
  week. That is the gateway's ChatGPT login, which only the owner can renew. The
  three smoke runs of 2026-11-19 saw the same breaker (`circuit open (model
  unhealthy)`) on every L research run. Until the owner renews the
  login, **the comparison tonight is S against I only**: the contrast is two
  Devin computers against one opencode-go computer, and it says nothing about
  a third model.

Texture and the conductor run on each computer's default policy, the same on
every arm, so only research differs. That default policy is led by Devin
[read: internal/modelcatalog/catalog.go:516-523], so the Texture and conductor
calls of **all four** arms use Devin's single session beside the paper's; the
two S arms' research calls come on top of that.

**The research reasoning effort.** The repo's policy default is `low` for every
role including research [read: internal/modelpolicy/model_policy.go:796-808,
the `[roles.research]` block and `fallbackPolicy`'s comment "every role runs at
low effort (owner, 2026-11-09)"], and all four overlays say `low`. That is
**assumed from the repo**: the paper's policy file may be owner-edited, and the
evidence of 2026-11-09 says research then ran `high`
[read: docs/evidence/paper-token-budget-2026-11-09.md:349-350]. The live value
cannot be read without touching the paper computer. The line that shows it is
in Node B's gateway journal (`journalctl -u go-choir-gateway`): `gateway:
inference request from autoputer <the paper's vm> (provider=devin model=... reasoning=...)`
[read: internal/gateway/handlers.go:511]. The lead can read it with the status
scripts' ssh and a `grep reasoning=` on the paper's vm id. If it says `high`,
the S arms' overlay (`reasoning_effort`) is changed in `scripts/e1`
(`EFFORT`) before the pilot, and the smoke prints the reported model id of the
arm for comparison with the paper's `model=` value.

## 6. Arms without switches

| | |
|---|---|
| **How an arm is set** | An overlay file `System/model-policy-overlays/e1-<arm>.toml` on that arm's computer, installed with `choir files put` (the verb exists for overlay installs [read: cmd/choir/files.go:1-5; cmd/choir/commands.go:390-396]). It names only the research role, with an end: `[overlay] expires_at = ...` and `[roles.research] provider, model, reasoning_effort` [read: internal/modelpolicy/model_policy.go:1015-1060, :1223-1229]. Every submission names it as `model_policy_overlay_id`. The directory is made one level at a time (`System`, then `System/model-policy-overlays`; `files put` into a missing parent answered 404 "parent directory not found" on 2026-11-19). The route resolves the overlay against the **Texture** role [read: api_texture_prompt_eval.go:99-108], so its 202 (and the `provider`/`model` it echoes, which are Texture's) does not show that the research section parsed; `model.served` does. |
| **How it is recorded** | The route writes the overlay id into run metadata as `llm_policy_overlay_id` [read: model_policy.go:29; internal/textureowner/api_texture_prompt_eval.go:78]. The trajectory carries it as `SubjectRefs[TrajectoryRefModelPolicyOverlay]` [read: internal/textureowner/texture_handoff.go:255-257]. Research runs inherit it [read: internal/agentcore/runtime.go:1746-1749, :5547-5575]. The reader takes the arm from the run and the model from `model.served`, never from the driver's plan. |
| **How it is read back** | `choir trace export <trajectory>` (scopes `read:runtime`, `read:texture` [read: cmd/choir/commands.go:702-707]), plus `GET /api/runs/{id}/events`. |
| **How it ends** | At `expires_at` the route refuses the overlay rather than falling back [read: model_policy.go:758; api_texture_prompt_eval.go resolve check]. `scripts/e1 end` removes the overlay files with `DELETE /api/files/System/model-policy-overlays/<id>.toml` [read: internal/autoputer/files.go:247]; there is no `choir files` delete verb. After the readout is committed, the computers are deleted (by the lead, not by the driver). |
| **Registry** | The five `calibration_e1_*` entries go with their code (section 10). E1-M adds no entry: it adds no setting, environment value or code path, which are what the lint scans [read: scripts/check-switch-registry:48-52, :255-260]. Its record is section 14.1 and its end is the overlay expiry, which the code enforces. The driver is a developer script with no environment value or setting. E1-H's surface variant is code, so it gets an entry (section 9). |

**A prompt arm is not in this protocol.** A prompt override is stored per
owner [read: internal/promptstore/store.go:83-93, :203-204], so it cannot
differ between runs on one computer. Putting it on a second computer would
repeat the confound this design removes. The prompt arm waits for a per-run
prompt pin, which goes beside E1-H's surface variant.

## 7. Cost, and the bound on it

Search credits are the owner's money. Keenable reports no balance; the
reading is "a number a person copies from the usage page"
[read: search-volume record, Remaining error]. The last reading is **17,000
credits read by the owner at 2026-11-18T12:38Z** [read: scripts/status-paper-usage:59].
The budget that reaches 5 December is about **880 searches a day**
[read: search-volume record, Investigation]. (`status-paper-usage` computes
1,032 a day from the same reading.)

| | Units | Expected started searches (7.5 a unit) | Hard stop on the tally | In days of budget (880/day) |
|---|---|---|---|---|
| Pilot (tonight) | 10 objectives x 3 arms = 30 (4 arms: 40) | about 225 (3 arms; 300 for 4) | **450** | about 0.34 expected, 0.51 at the stop |
| Full run (owner's word) | 40 x 4 = 160 | about 1,200 | 1,800 | about 1.4 expected, 2.0 at the stop |

The smoke objective `s00` (four more units, one per arm) is **outside** the 40
and outside the 450: it is a handful of searches, counted and printed by the
smoke, and the tally for the stop starts at zero with objective `p01`.

**The tally and the stop.** The hard stop is a bound the experimenter sets on
the experiment's own cost. It is not a limit on model calls. The tally is the
sum of the providers' `Calls` in the `search.served` lines of the pilot's
research runs (section 3); started searches, the outcome, are a few percent
more because the gateway's cache answers some. **450 is the hard stop**: the
driver adds the live tally of the running objective (read from the runs'
events every three minutes) to the finished objectives', and at 450 it cancels
every active run of the objective and ends the pilot. **315 is a printed
warning**, never a stop, so on a tail-heavy night the ten objectives P1 needs
still run. The most that can overshoot is what runs in the three minutes
between two live reads.

**The refusal rule before starting.** Estimated remaining credits is "17,000
read at 2026-11-18T12:38Z, minus the provider's answered searches since then".
**Where that count can be read without acting on any computer:**
`scripts/status-paper-usage --json` (one read-only ssh to Node B, the status
scripts' own reading, the only one the driver runs and never twice in three
minutes) gives `search.keenable.today_calls`, the gateway's Keenable calls
today UTC [read: scripts/status-paper-usage:208-220 and
internal/gateway/search_plane.go:300-323, the `today=[keenable=N]` field]. It
reads the last 60 minutes of the journal [read: scripts/status-paper-usage:381, `--since '-60 min'`]; no status line gives
the calls of the days between the reading and today. So the driver cannot
compute the estimate (it can only say today's part: `scripts/e1 credits`), and
**the lead supplies it**: `scripts/e1 pilot --credits N --credits-basis "..."`,
both required. The driver prints both and refuses to start when N is below
**4,500** (ten times the hard stop), when `--credits-basis` is empty, or when
the one status read fails. It also refuses when the read shows the
newspaper's searches getting no provider answer, its Devin calls refused for
quota, or Keenable `quota_limited` (the last may never fire: Keenable is the
uncapped provider [read: scripts/status-paper-usage:51]; the credit figure is
the real guard).

At the paper's rate since the reading, the estimate may well be below 4,500.
In that case the pilot does not run tonight, and that is the correct outcome.

**Model calls:** 18.5 calls and about 0.41M input tokens per research run on
2026-11-09 [read: docs/evidence/token-efficiency-2026-11-09/research.md:21, :88].
The pilot's 40 units (four arms; about three quarters of that with three) come to about 740 research calls and 16M input
tokens, plus Texture's and the conductor's calls (not measured). All of it
goes to the three no-charge routes above.

## 8. The reader, the ledger gate, and why nothing lapses

- **The verdict is computed, and the forecaster does not judge.**
  `scripts/e1 read --verdict` applies sections 3, 4 and 14.4 to the exported
  bundles and prints `true`, `false` or `void` per claim, with the counts
  behind it. The computed line is the teacher (learning engine, rule 1a: the
  one who forecast does not settle [read: docs/learning-engine.md:39-50]); the
  lead only copies the line into the ledger's settlements table, with the
  command and the bundles' hash as evidence.
- **Too few valid units settles `void` with the reason, not `false`.** The
  ledger defines `void` as a real settlement ("the test became impossible; say
  why"), distinct from `lapsed`, and not scored
  [read: docs/calibration/claude-ledger.md:89-90; precedents C101, C128].
  `false` is a completed test whose event is absent. A stop (cost, publication,
  the newspaper's status, the deadline) that cuts the pilot short therefore
  settles the claims it starves `void`, and never reads as "the tail does not
  reproduce" (finding 1 of the second review).
- **The driver refuses to start a counted run** unless the three claim ids are
  rows of the **Claims** table (not the settlements) of
  `docs/calibration/claude-ledger.md` as read from `origin/main` (`git fetch`
  then `git show origin/main:...`, never the working tree), each row says
  "E1-M pilot P1", "E1-M pilot P2" or "E1-M pilot P3", and the P1 row holds the
  SHA-256 of the objective file, which itself must be the file on `origin/main`.
  They must be pushed, not only written. The lead enters them.
- **The validity filter and the reader are one script with a fixture.**
  `scripts/e1` holds `read_unit`, `link_builds` and `verdicts` as pure
  functions of the exported bundles; `scripts/selftest-e1` runs them over the
  invented bundles in `scripts/testdata/e1/` (a normal run, a many-search run, a
  cache-answered search with no `search.served` line, a run with no research, a
  timed-out run, a two-research-run trajectory, a wrong served model, a model
  switch) and the stub flow, and is wired into `scripts/gates`. One review seat
  reads the script, its fixture and the self-test before the first counted run,
  because the outcome depends on code the author wrote. That review has not
  happened: **no counted run starts until it has.**
- **The readings are a pure function of the bundles.** The driver's own state
  (`~/.cache/choir-e1/`, outside the repo) holds the submissions and the
  bundles; `read` recomputes every unit from the bundles and the recorded
  submission times. The computers are deleted only after the readout is
  committed.
- **`scripts/status-hourly` gains no E1 section in this change.** It is not
  built. The settle-by time in the ledger remains the backstop, and the lead
  runs `scripts/e1 read --verdict` by hand. (The round 2 text promised the
  section; the promise is withdrawn until someone builds it.)

## 9. E1-H: the harness arms (not startable until the research eval route exists)

E1-H is where a harness cause can be shown. It holds the model fixed
(`swe-2-flash`) and changes one surface fact per arm. The arms come from the
trace record's last section [read: recurring-agent-failures record, "What a
controlled comparison would have to vary"]:

- **H-a** the cell result after a search shows the top ten titles with hosts
  and held marks, not only "12 of 40 pages new";
- **H-b** only `SearchAll` exists (plain `SearchStart` absent);
- **H-c** `ReadCapture` returns `block_count` and the position read, and a
  whole-page read up to a cap exists.

**Its unit** is a replayed real run: its first message and its held captures,
on an arm. That needs **the research eval route** (orange): `POST
/api/evals/research` on a test computer starts a research desk run from a
given first message, with the same overlay pinning as the Texture eval route,
and with that run's held public captures available. The first messages come
from the paper's run ledger, which needs a read-scoped key for the paper (the
owner mints it). Seeding the corpus is the open question the lead deferred,
and it is decided when the route is designed.

**How a variant is pinned without becoming a product switch.**

- The variant is a closed enum carried in the run's metadata.
- Only the research eval route sets it, and only on a test-class computer.
  That refusal is a correctness rule: the paper must never run a variant.
- The kernel reads it when building cell results.
- The code refuses it after an end date compiled beside the enum.
- It is stamped on every `desk.cell` event.
- It gets a registry entry with `life: experiment`, `expires` at the end date
  and `retire: delete`.
- A per-run prompt pin is built the same way.

Class: orange, or red if a seat finds it changes what a run records. One seat,
or two if red.

**Sample size.** Each variant against a control on the same replays, one
computer per arm as in E1-M, with two control computers. By section 4: 19
objectives detect a halving at σ = 1.0, 63 detect 30%. Replays of tail runs
(the many-search runs) make the effect larger and the n smaller. For H-c, the
outcome is reads of one page and bound refusals, on replays of long-page runs.

**What the second trace pass changes:**

| If the cell programs show | Then |
|---|---|
| The many-search runs wrote one cell looping over a list of wordings | H-b first. |
| Many cells, each a new wording after reading results | H-a first, on thin-kind replays. If E1-M shows the tail in one model only, no harness arm: it is that model's choice. |
| The read loop advanced and nearly finished a huge page | Not a model failure. The whole-page read of H-c ships as a defect fix, not as an arm. |
| The read loop reset or stepped by one block | Replay it 10 times on S and 5 times on each other model once the route exists; H-c is tested as an arm. |
| Runs never call `Summary` or `Get` before reading | H-a is the arm for the read share. |
| They do, and still read 7 to 25 | The 5% is what stories need; it is dropped as a failure. |

## 10. Deleting the 4 November arms (red, two seats, due on main before 2026-11-24)

Why: the old question (does showing a desk its own record improve its stated
numbers) has a ceiling of about 0.016 Brier [read: docs/learning-engine.md
section 9.2]; its precondition, a 12-hour window with no change, never came;
and it has sat as five switches off everywhere, already walked once past the
gate [read: docs/problems/dated-switch-gate-turned-main-red-at-midnight-2026-11-19.md].
The deletion does not block the pilot.

The complete list [read: git grep at this commit]:

- **Settings**: `internal/store/computer_setting.go:181-198, :222-226`; the
  five registry entries (`switches/registry.yaml` from line 2010).
- **Runtime** (`internal/agentcore/`):
  - `calibration_e1.go`, **not the whole file**: the file also holds helpers the
    kept push digest and `choir.Calibration` still read, and they move first
    (the table below this list says where each goes); the rest of the file
    goes, with `calibration_e1_test.go` (its tests are the five arms' and the
    restatement protocol's, F1 to F9; any test that reads a kept helper moves
    with it);
  - `calibration_feedback.go:580` (the push gate `rt.calibrationDeliveryOfRun(ctx, rec) == calibrationDeliveryPush` becomes the desk test alone) and `:642-645` (the pull/stub branch that calls `calibrationComputedDigest` goes, the push branch stays);
  - `calibration_reads.go:51-54` (`restateAsked`);
  - `engineering_precommit_rules.go:176`;
  - `inbox_records.go:788`;
  - `rlm_reduce.go:202, :715-720, :1239`;
  - `tools_capsule.go:1106`;
  - `tools_desk.go:922-923`.
- **The helpers the kept code still uses, and where each goes:**

  | Helper (`internal/agentcore/calibration_e1.go`) | Read by | Fate |
  |---|---|---|
  | `calibrationRestatement` (:60-62) | `calibration_feedback.go:194, :390, :685` | moves to `calibration_feedback.go` (next row) |
  | `calibrationDeliveryFor` (:67), `calibrationArmForDelivery` (:102) | the helpers below only | deleted: with no setting held, delivery is always `push` and the arm is the thread's arm |
  | `calibrationDeliveryOfRun` (:91) | `calibration_feedback.go:580, :642` | deleted; both call sites test the desk (`IsAssignmentPerformer`) alone |
  | `calibrationComputedDigest` (:114) | `calibration_feedback.go:645` (pull/stub branch), `calibrationReadableForRun` | deleted with that branch |
  | `calibrationReadableForRun` (:137) | `tools_desk.go:792`, `tools_capsule.go:1024` | reduced to `calibrationDigestForRun` plus the literal delivery `"push"` those two callers record, so the callers keep their signature |
  | `stampEngineerDelivery` (:153) | `engineering_precommit_rules.go:169` | reduced to the two stamps a push claim keeps, `calibrationArmRef` from `calibrationArmOfThread` (already on line 167) and `RuleRefStage = first`; the `calibrationDeliveryRef` stamp stops (the kept reader `calibration_claims_api.go:106` reads `Delivery` from it, so an old record still shows its delivery and a new one shows none) |
  | the constants `calibrationDeliveryPush` and `calibrationDeliveryRef` (:33-48) | the rows above | `Push` stays beside the reduced `calibrationReadableForRun`; the other four deliveries go |

  These rows are read from `git grep` at this commit and from the file; the
  review of the deletion confirms each against the compiler.
- **The `calibrationRestatement` wrapper** (`calibration_e1.go:60-62`) moves
  to `calibration_feedback.go`, beside its three callers (:194, :390, :685),
  still calling `types.RestatementRulePrecommit`. It stays until it is shown
  that no p1 record exists on any tape.
- **Kept for the readers**: `calibration_claims_api.go:84-106` (`Stage`,
  `Restates`, `Delivery`, `RecordShown`), so any historical record still reads
  correctly. The constants they name move with the wrapper.
- **Grant**: `internal/deskmanifest/grants.go:101` (`calibration.restate`) and
  `manifest.go:349, :417`. **The impact table**: no exact row was found by
  grep, so `calibration.restate` may resolve through a family row
  (`trust.GrantImpact`, `internal/trust/impact.go:130`); run
  `TestEveryCatalogueGrantHasAnImpactRow` and
  `TestEveryImpactRowNamesARealGrant` (`internal/trust/impact_grants_test.go`),
  and remove an exact row if one exists.
- **Catalogue**: `internal/deskmcp/catalogue.go:103` (`desk_restate`).
- **Kernel surface**: `internal/yaegikernel/choir.go:491, :1885-1899`;
  `intent.go:70-74, :805-813`; `surface.go:59`; `surface_examples.go:55`.
  Removing a verb moves the desk protocol surface, so a new lock file goes in
  `internal/deskprotocol/lock/` with the version bump it classifies as
  (`internal/deskprotocol/version.go`).
- **Capsule protocol**: `internal/capsule/types.go:223-226`
  (`CalibrationRestate`).
- **Cell output keys**: `internal/toolregistry/noprogress.go:85`
  (`calibration_restate`).
- **Guest-security trailers** (`Security-Effect`,
  `Security-Effect-Reviewed-By`) are needed on every commit touching
  `internal/agentcore/` or `internal/toolregistry/`, which are listed in
  `nix/guest-security-surfaces.txt`. `internal/store`, `deskmanifest`,
  `deskmcp`, `yaegikernel`, `capsule` and `trust` are not on that list at this
  commit.
- **Before landing**: read each computer's stored settings for the five keys
  (d@e, the five old arm computers, the paper) and confirm none is stored.

Rollback: revert.

## 11. Risks, and how to refute this design in a day

1. **The empty computer does not show the tail** (scope, section 1). Refuted
   or confirmed by the pilot itself: forecast P1 and the gate in 14.4.
2. **A Texture-cast follow-up is not the paper's research task.** Texture
   writes the research run's first message, not management
   [read: internal/agentcore/runtime.go:4108-4116]. Half the pilot's
   objectives are thin local events of the tail's kind, labelled before any
   run, so the pilot shows whether either form reproduces the tail.
3. **The pinned model does not serve every call, or the experiment competes
   with the paper.** `ContinuationChain` switches model on quota
   [read: model_policy.go, near :225]. Every switch makes a unit invalid, so
   the cost is lost units, not bias. **All four arms'** Texture and conductor
   calls, and the two S arms' research calls, share Devin's single session with
   the paper (the default policy is Devin-led). The driver reads the
   newspaper's Devin refusals through the one status read before each
   objective and waits when they appear (section 14.5). L and I use
   subscription and free-tier quotas that can also refuse: the evidence of
   2026-11-09 counts 54 ChatGPT quota failures and 113 opencode-go circuit
   opens in a day [read: docs/evidence/paper-token-budget-2026-11-09.md:355-356].
   A refusal there is a model switch or a failed run, so a lost unit, not a
   measurement. D-E2's stand-in covers I only.
4. **Test computers publish.** The eval route opens Texture with `open_app`,
   not publish [read: api_texture_prompt_eval.go, `CompletePromptBarDecision`
   spec]. Auto-publish applies only to "a story opened with publish"
   [read: internal/textureowner/texture_cell_author.go:486]. Checked anyway in
   the smoke step and after every objective with a real read:
   `GET /api/platform/texture/publications?doc_id=<id>` is the caller's own
   publication of one document, whatever its mode: 404 when there is none, 200
   with the publication when there is
   [read: internal/proxy/platform_publish.go:781-846;
   internal/platform/handlers.go:196-222]. (`GET /api/texture/documents/{id}/published`,
   which round 2 named, is a POST-only notice and answers 405
   [read: internal/textureowner/published_notice.go:13-34].) The check covers the
   submission's document and every document id the trace bundle names. **There
   is no story-unpublish verb**: `choir platform yank` yanks a package release
   [read: cmd/choir/package.go:193-241], and the platform yank route is
   operator-only. So the response to a publication by a test computer is:
   **stop the pilot, print the document and the arm, and tell the owner.** Test
   computers must not be public-platform computers; these are fresh test
   owners made by `scripts/qa_test_computer.mjs`, ordinary computers on no
   platform ring.
5. **The paper's story is findable from the arm computers** through the
   shared published layer. Every arm sees it equally. Whether a run read it is
   recorded as a covariate. It lowers searching against the paper's rates,
   which is a reason the pilot's reproduction gate is low (two tail units of
   twenty).

## 12. Decisions taken by default

- **D-E1 (the night file's D2): which computers hold an arm.** Default: four
  fresh test computers, one per arm (`e1-sa`, `e1-sb`, `e1-l`, `e1-i`). The
  paper, the owner's computer, users' computers, d@e and the old arm computers
  hold nothing.
- **D-E2: the models (updated 2026-11-19).** Default for the pilot: `swe-2-flash`
  twice and `iceman-contributor-free` via opencode-go, **three arms**; `gpt-6-luna`
  via ChatGPT is dropped until the owner renews the gateway's ChatGPT login (the
  Luna decision of round 2, "L is kept", is superseded by the journal's 401). The
  full run's four-arm design waits for that login. Stand-in for I:
  `recursive/flint-v1.0:free` via OpenRouter (a free model, within the
  OpenRouter rule). If neither free model passes the smoke step, run three
  arms (S-a, S-b, L) would be the other three-arm pilot. The driver keeps a
  table of the four arms; **the pilot's arms are the objective file's header
  `arms`** (section 14.3), so a pilot of three arms is a header, not an edit; a
  stand-in model is an edit to the table and a new `setup`.
- **D-E3: what runs before the owner wakes.** Default: **the pilot only.**
  The full 40-objective run (and its expansion to 60 if σ > 1.2), the
  read-loop replays and any E1-H work wait for the owner's word at the
  check-in, about 17:30Z today.
- **D-E4: no registry entry for E1-M.** Default: none (section 6).
- **D-E5: delete the old arms with their code.** Default: yes (section 10).

## 13. What changed from the 4 November protocol, and why

- **The question.** It was whether a desk's own calibration record improves
  its stated probabilities. It is now whether a recurring failure belongs to
  the model, screened by holding the harness fixed. The old ceiling was small,
  and the owner's 2026-11-19 question is the new one.
- **The desk.** From management's and engineering's forecasts to the research
  desk's behaviour.
- **The unit.** From a Cast on five differently pinned, differently aged
  computers to an objective on an arm, with one fresh computer per arm, the
  same objectives at the same moment, and a two-computer noise floor.
- **The arms.** From five settings off everywhere to an overlay per computer
  that expires on its own.
- **The precondition.** From a 12-hour quiet window that never came to
  same-moment starts and a build check per objective.
- **The reader.** From the owner reading a route by hand to a reviewed script
  whose verdict is computed, a driver that will not start without the claims
  on `origin/main`, and the hourly status chasing the settlement.
- **Round 3 (the second review's nineteen findings).** The outcome is started
  searches, the cost tally is the providers' calls; the unit's validity rules
  and the verdicts are code with a fixture; too few valid units is `void`; the
  publication read and the response to a publication are real; the driver and
  its self-test exist; the credit figure is supplied, not computed; 315 is a
  warning; a timed-out run is cancelled and re-exported; the deletion list
  names the helpers the kept code uses.
- **Kept.** "Every record says what it actually got" (old F6). Fixed n, no
  early stop for success, inconclusive reported as inconclusive.

## 14. Pilot, tonight

**The driver exists.** `scripts/e1` (Python standard library, developer side,
yellow) and its self-test `scripts/selftest-e1` with the invented bundles in
`scripts/testdata/e1/` were written by the change that rewrote this section,
and the self-test runs in `scripts/gates`. Every route below is one read in the
code (cited) or one seen to answer (marked **seen**). `setup` and `smoke` ran
once on staging on 2026-11-19; their output is in 14.6. `pilot` has **not**
run, and it does not start until the review in section 8 has read the reader
and its fixtures.

### 14.1 The record

| | |
|---|---|
| Experiment | E1-M pilot |
| Computers and arms | S-a `e1-sa`, S-b `e1-sb`, I `e1-i`; the header's `arms` are `["sa","sb","i"]`. L's computer `e1-l` exists and stays as it is (section 5; ids in 14.6) |
| Objectives | 10: 5 `thin_event`, 5 `follow_up`, fixed and committed before the first counted run (14.3) |
| Units | 30 (10 objectives x 3 arms; 10 x the header's arms), plus the smoke's units outside the bound |
| Outcome | started web searches, summed over the unit's research runs (section 3) |
| Cost tally | the providers' calls in `search.served`; **hard stop 450**; 315 is a printed warning only (section 7) |
| Overlays expire | set by `setup` (now plus 30 hours; 2026-11-20T12:50Z for these four); the pilot refuses with under 90 minutes left |
| Pilot ends | when 10 objectives are done, a stop fires, or no new objective starts after 2026-11-20T06:00Z |
| Act and bound | the owner's message of 2026-11-19 ("the calibration experiment is something that we should do now, actually") is the act; the 450-call tally, the 4,500-credit floor and the 0 published documents are its bound; the claims name both |
| Claims | three, entered and pushed by the lead before the first counted run (14.4) |
| Start | all four requests of an objective are sent together; the start time is each request's. Research start, first message and number of research runs vary by arm (section 4) |

### 14.2 Making the computers and installing the arms

`scripts/e1 setup` (print the plan with `--plan`). It runs
`node scripts/qa_test_computer.mjs --label <label>` four times in turn, from the
checkout whose `frontend/node_modules` holds Playwright
[read: scripts/qa_test_computer.mjs:16-23]. That script needs no credential
from the person running it: it registers a fresh owner `<label>@example.com`
with a virtual passkey on `https://choir.news`, waits for the computer, and
mints a key scoped to that one computer
[read: scripts/qa_test_computer.mjs:122-148]. It prints one JSON line with the
computer id and `credential_file`, a JSON file under
`~/.config/choir-qa/test-computers/` holding `api_key`
[read: scripts/qa_test_computer.mjs:151-160]. A label that already has a
credential file is refused by the script; `setup --label-suffix` takes new
labels, and a rerun of `setup` keeps the computers it already recorded.

The driver derives `~/.cache/choir-e1/keys/<label>.key` (mode 0600, the
`api_key` and a newline: the CLI's `--api-key-file` reads the whole file as the
key and requires 0600 [read: cmd/choir/main.go:99-126]) and records, in
`computers.json` (0600, outside the repo), per arm: label, computer id, key
file, credential file, overlay id, provider, model, reasoning effort, host.
No key is printed.

The overlay for each arm (S shown; L uses `chatgpt` and `gpt-6-luna`, I uses
`opencode-go` and `iceman-contributor-free`; `reasoning_effort` is assumed from
the repo, section 5):

```
[overlay]
expires_at = "<setup time + 30 h>"

[roles.research]
provider = "devin"
model = "swe-2-flash"
reasoning_effort = "low"
```

It is installed with `choir files mkdir System`, `choir files mkdir
System/model-policy-overlays` and `choir files put --local <file>
--api-key-file <key> --host <host> System/model-policy-overlays/<id>.toml`
(flags before the path: the CLI's parser stops at the first positional
[read: cmd/choir/registry.go:352-378, 380]; mkdir and put answered on
2026-11-19, **seen**), and read back with `choir files get`.

### 14.3 The objective file

It lives at `docs/calibration/e1/pilot-objectives.jsonl`, committed and pushed
(a docs commit) before the first counted run. Its SHA-256 goes into the P1
claim. Only public news subjects appear in it. **It is not written by this
change**: choosing the stories is the lead's act at T0, from the public front
(`GET /api/platform/publications/front`, which needs no key
[read: internal/proxy/platform_public.go:88-96]). The driver validates the
file's shape (`scripts/testdata/e1/pilot-objectives.sample.jsonl` is an invented
example): one header and ten objectives, five of each kind.

- **Line 1, the header:**
  `{"t0": "<UTC>", "rule": "e1m-pilot-v1", "arms": ["sa","sb","i"], "order": "file order"}`.
  **`arms` is the pilot's arm list.** It must be a non-empty list of distinct
  arms from the driver's table (`sa`, `sb`, `l`, `i`) that includes `sa` and
  `sb`; everything the driver counts (units = 10 x arms, the per-objective
  line, the tally, the drop-an-arm rule, P3's denominator, d's pairing) comes
  from it. The tonight header is exactly:
  `{"t0": "<UTC>", "rule": "e1m-pilot-v1", "arms": ["sa","sb","i"], "order": "file order"}`.
- **Lines 2 to 11, one per objective:**
  `{"id": "p01", "kind": "thin_event"|"follow_up", "story_url", "headline", "cited_hosts": [...], "label_basis", "prompt"}`.

The selection rule, applied once at T0 to the paper's public front, newest
first:

- **`thin_event`**: a story that cites two distinct hosts or fewer and whose
  headline names a place below the country (a city, a district or a line),
  the kind of the record's tail runs. The prompt is `Research this event and
  write a short news story about it: <headline>. <first 300 characters of
  the story>`. No URL is given.
- **`follow_up`**: the next five stories not chosen above. The prompt is
  `Write a follow-up to this published story. Find reporting, primary
  documents and outlets it does not cite yet, and read them. <headline>
  <story_url> <first 1,500 characters>`.
- `label_basis` records why each label was given (for example, "2 cited
  hosts; a district named"). Labels are written before any run.

The smoke objective `s00` is a `follow_up`-style prompt inside `scripts/e1`,
not in the file; its four units are outside the pilot's 40 and 450.

### 14.4 The claims for the pilot (the lead enters them)

Kind `experiment`. Settle by 2026-11-20T08:00Z. Evidence: `scripts/e1 read
--verdict` on the bundles. Each row begins "E1-M pilot P1" (P2, P3), names the
act and its bound (14.1), and the P1 row holds the objective file's SHA-256.
Each claim is restated so that **one line of the verdict** settles it, over
**valid units** (section 3), with a stated **minimum count**, on **started
searches**. Fewer valid units than the minimum, or a stop that cut the pilot
short, settles **`void` with the reason**; `false` is a completed test whose
event is absent. The probabilities are the design's.

- **P1, p = 0.45.** Among the valid S units (S-a and S-b over the ten
  objectives), at least 2 started 10 or more web searches, **given at least 14
  valid S units**. With fewer than 14, `void`. The event changed in two small
  ways from round 2: the count is of started searches (a few percent higher
  than `search.served` lines, because cache-answered searches count), and 14
  valid units suffice where round 2 asked for 20. The two move the chance in
  opposite directions by less than the forecast's precision, so the number is
  kept. At the paper's rate of 0.19, the chance of 2 or more in 20 is 0.92 (and
  0.78 in 14); the price is low because the empty corpus, the Texture-written
  first message and the shared published layer all push searching down. **This
  is the gate.** If P1 is false, the tail does not reproduce on an empty
  computer, E1-M stops, and the research eval route comes first. If P1 is void
  nothing is concluded.
- **P2, p = 0.50.** On at least 7 of the objectives on which S-a and S-b are
  both valid, ln(1 + started) of S-a and S-b differ by less than ln 2, **given
  at least 7 such objectives**. With fewer than 7, `void`. (Round 2 said "at
  least 7 of the 10 ... with both S units valid", which a count under 7 settled
  false; it is now void, and the probability is kept.)
- **P3, p = 0.50.** All 10 objectives were submitted (no stop cut the pilot)
  and at least **90% of the units, rounded up, are valid** (section 3): 10
  objectives times the header's arms is the number of units, so 27 of 30 for
  three arms and 36 of 40 for four. `read --verdict` prints the derived counts
  ("N of 30 units valid (the claim needs 27, 90% rounded up)"). The event of
  round 2 was 36 of 40; with three arms it is stated on the derived number and
  the probability is kept (the share, 0.9, is the same). If a stop cut the
  pilot short, **`void`**: the test of validity did not run. A completed pilot
  with fewer valid units than that is `false`. The unit's 60-minute rule bounds
  the wait, so no unit is waited on past 2026-11-20T06:00Z plus an hour.

P1 and P2 come from the same bundles, and both depend on the validity rule,
so they are correlated. Each is still priced on its own. The driver's output
for each is one line, `P1 (C...) true|false|void  <counts and reason>`.

### 14.5 The driver

```
scripts/e1 setup   [--plan] [--label-suffix S]
scripts/e1 smoke
scripts/e1 credits
scripts/e1 pilot   --claims C1,C2,C3 --credits N --credits-basis "..." [--objectives FILE]
scripts/e1 read    [--verdict] [--claims C1,C2,C3]
scripts/e1 end
```

Common: `--state-dir` (default `~/.cache/choir-e1`), `--choir` (default `choir`
on PATH; the CLI runs with a scrubbed environment so no stray key reaches it).

**What the driver calls** (each route read in the code; the ones marked
**seen** answered during the smoke):

| Step | Call | Read |
|---|---|---|
| submit | `POST /api/evals/texture-prompt` `{text, model_policy_overlay_id}`; 202 with `submission_id`, `doc_id`, `model_policy_overlay_id` | internal/apihandler/routes.go:82; api_texture_prompt_eval.go:21-36, :133-143; scope `write:runtime` (internal/proxy/handlers.go:591-615) |
| unknown overlay | the same route answers 400 and starts nothing | api_texture_prompt_eval.go:99-108 |
| trajectory | `GET /api/prompt-bar/submissions/{id}` → `decision.trajectory_id` | prompt_bar.go:87-95, :426-484; texture_handoff.go:277 |
| runs | `GET /api/runs?trajectory_id=&limit=500` → `runs[]` with `run_id`, `agent_role`, `state`, `created_at`, `finished_at`, `metadata` | internal/agentcore/api.go:569-600 |
| live tally | `GET /api/runs/{id}/events?after=&limit=1000` → `events[]` (`loop_id`, `kind`, `payload`), `next_after` | api_run_events.go:38-78; types/task.go:422-460 |
| cancel | `POST /api/runs/{id}/cancel` | api.go:643-653 |
| build | `GET /api/runtime/observability` → `build.commit`, `computer_id` | api.go:871-874; cmd/choir/trace.go:441-470 |
| bundle | `choir trace export --api-key-file K --host H --timeout 120s --out P <trajectory>` | cmd/choir/trace.go:1003-1064; scopes `read:runtime`, `read:texture` (commands.go:702-707) |
| publication | `GET /api/platform/texture/publications?doc_id=` → 404 none, 200 published | platform_publish.go:781-846; platform/handlers.go:196-222 |
| overlay | `choir files mkdir|put|get`; `DELETE /api/files/System/model-policy-overlays/<id>.toml` for `end` | commands.go:374-423; autoputer/files.go:247 |
| newspaper status | `scripts/status-paper-usage --json` (ssh to Node B, read only), at most once in three minutes | scripts/status-paper-usage:381 |

**`scripts/e1 smoke [--arms sa,sb,i] [--restamp]`** reads the computers from
`computers.json` (default all four; `--arms` a comma list), submits `s00` to the
chosen arms together, and prints one `PASS` or `FAIL` line per check per
arm, then one summary line per arm, then `SMOKE OK` (exit 0) or `SMOKE FAILED
at step N (arm): ...`. A pass writes a stamp naming the four computer ids.
Steps:

1. the computer answers observability with its recorded id and a build;
2. the overlay file is on the computer, as put;
3. an unknown overlay id is refused with 400 and no run;
4. the eval route answers 202 with the arm's overlay id;
5. the submission names its trajectory;
6. every run of the trajectory ends within 30 minutes (else cancelled and re-exported);
7. `choir trace export` succeeds with runs and run events served;
8. the trajectory has a research run (it prints the roles seen);
9. `model.served` of the research runs names only the arm's model, no switch, and the run carries the arm's overlay (it prints the reported model id for comparison with the paper's);
10. `search_start` rows and `search.served` lines are present (it prints started, done, served lines, provider calls and cache-answered searches);
11. no document of the trajectory is published (14.5, publication read);
12. the chosen computers report one build before and after.

The stamp is **per arm**: `smoke.json` records, for each arm smoked, `passed`,
the computer id, the time, the build, the failing step if any and the run
errors, and a smoke of some arms keeps what an earlier smoke recorded for the
others. `--restamp` re-evaluates the stored bundles of the last smoke
(`bundles/s00-<arm>.json`) with no new run: steps 8 to 11 on the bundle (with a
fresh publication read of its documents) and step 12 on the bundles' builds; the
submission time is the earliest run's creation in the bundle, the build is the
bundle's boot-layer build, and the stamp's time is the bundle's export time, so
the six-hour clock starts when the evidence was taken. Its `source` says
"restamp".

A cache-answered search cannot be forced; the reader's handling of one is
tested on the invented bundle only, and the pilot shows whether it occurs.

**`scripts/e1 pilot`** takes `--claims`, `--credits` and `--credits-basis`
(required). It refuses to start, listing every reason, unless **all** hold:

- the three claim ids are rows of the Claims table on `origin/main`, P1 to P3 (section 8);
- the objective file validates, is the file on `origin/main`, and its SHA-256 is in the P1 row;
- every arm of the header has a passed smoke on its own recorded computer within 6 hours (an arm outside the header, or a failed L, does not count; the header is not required to equal the arms the smoke covered; the refusal names the arm: "smoke has not passed for l");
- the overlays have at least 90 minutes left;
- `--credits` is at least 4,500, and `--credits-basis` says where it came from (section 7);
- the newspaper status read succeeds and shows no stop.

It then prints the block it starts from (claims, file hash, smoke stamp,
overlay expiry, credits and basis, the status read time, the stops) and, for
each objective in file order: reads the build of each computer, submits to all
arms together, waits (a poll every 20 s; the trajectory has ended when it holds
runs, none is pending, running or blocked, and the set held for 45 s), exports
the bundles, runs the publication read, reads the build again, appends the
outcome line to `outcomes.tsv` and prints:
`p03 thin_event | sa 12 sb 9 i 6 started | runs 1/1/1 | valid 3/3 | tally 141/450 | build ok | published 0` (one entry per header arm).

A unit that has not ended 60 minutes after its submission is **timed out**:
its active runs are cancelled (`POST /api/runs/{id}/cancel`), the driver waits
up to five minutes for a terminal state, and **re-exports before the next
objective starts**. Its searches are in the tally; the unit is invalid.

It stops, printing why and writing the stop to `pilot.json`, when:

- the live tally of the running objective plus the finished ones reaches **450**: every active run of the objective is cancelled; terminal;
- a test document is found **published**: it prints the document and the arm and says there is no story-unpublish verb and the owner must be told; terminal. A publication read that answers anything but 404 or 200 stops it too;
- the status read, made before each objective, shows the newspaper's searches getting no provider answer, its Devin calls refused for quota, or Keenable `quota_limited`, or fails: a pause, not terminal; running `pilot` again re-reads it;
- 2026-11-20T06:00Z passes before an objective starts: terminal;
- an arm's last three units are invalid: that arm is left out of later objectives, the others go on, and the failure is a finding;
- the builds disagree: that objective is invalid on every arm, and the driver goes on.

315 prints a warning once. A terminal stop is final for this pilot: a second
`pilot` run refuses. `pilot` is resumable: it reads `pilot.json`, skips
finished objectives, and an objective whose submissions it recorded is waited
on, not submitted again.

**`scripts/e1 read [--verdict]`** recomputes every unit from the bundles and
the recorded submission times and prints: the unit table (research runs,
started searches, provider calls, cache-answered, valid, reasons, position);
per arm the valid count, median, 90th percentile (nearest rank), tail share,
`no_research` count and calls; d over the objectives valid on every arm of the pilot with its mean, sd, 95% t
interval and a bootstrap interval; the S-a against S-b counts; and the outcome
line, which states the full run's n (40, or 60 if sd of d is above 1.2). With
`--verdict` it prints `P1`, `P2`, `P3`, each `true`, `false` or `void` with the
counts and the reason.

**`scripts/e1 credits`** runs the one status read and prints what the status
scripts can give (today's Keenable calls) and what they cannot (section 7).

**`scripts/e1 end`** removes the four overlay files and prints what is left on
the test computers (documents, runs, captures, key and credential files,
bundles). It deletes no computer.

### 14.6 What `setup` and `smoke` showed on 2026-11-19
Run on the lead's Mac against `https://choir.news`, three smoke runs (the
limit set for the builder), the third at 07:21Z. The four computers (ids; no key
is in the repo or the output):

| Arm | Computer id |
|---|---|
| S-a `e1-sa` | `computer-e4e37fe18240fbd17853e3538973f5a9` |
| S-b `e1-sb` | `computer-745b8716c3f3d7332568fec3b128d65c` |
| L `e1-l` | `computer-1b48a948cbfd539ba5b3f067983bd236` |
| I `e1-i` | `computer-bbb984346f27ff99ca3f72c831925374` |

All four ran build `a0e852f56e292a81826a4934a3b6fc9c71bb079c` before and after.

**What worked (seen):** setup (minting, key derivation, overlay `mkdir` of
`System` then the overlay directory, `put`, `get`); the 400 for an unknown overlay
with no run started; the 202 with the overlay id; the trajectory id in the
submission status; `GET /api/runs?trajectory_id`; every trajectory ending
inside 30 minutes; `choir trace export` with `runs` and `run_events` served;
the publication read answering 404 for every document of every arm (nothing
published); the roles seen in every trajectory (`conductor`, `texture`,
`research`); the overlay inherited by research runs (research runs carry the
arm's `llm_policy_overlay_id` and `llm_provider`/`llm_model`, Texture's run the
default `devin/swe-2-flash`).

**Run 3, per arm:**

| Arm | Research runs | Started searches | Provider calls | Served model | Switches | Unit |
|---|---|---|---|---|---|---|
| S-a | 1 | 2 | 2 | `devin/swe-2-flash` | 0 | valid |
| S-b | 1 | 3 | 3 | `devin/swe-2-flash` | 0 | valid |
| L | 5 | 0 | 0 | none | 0 | **not valid**: `no_model_served_record` |
| I | 1 | 7 | 7 | `opencode-go/iceman-contributor-free` | 0 | valid |

**What failed: arm L.** Every research run of L failed in about 45 seconds with
`tool loop iteration 3: gateway call failed: gateway client: provider chatgpt
model gpt-6-luna: circuit open (model unhealthy)`. The
gateway's breaker for `chatgpt/gpt-6-luna` was open on 2026-11-19 from 06:51Z to
07:21Z, so no research call was served and the arm has no model record, no
search. The smoke fails at step 9 for L. This is a finding about the L route, not
about the driver: **L cannot run until that breaker is closed** (the breaker
routes are operator-only; the driver does not read or reset them). The pilot
no longer blocks a pilot whose header leaves L out; a header that names `l`
refuses with "smoke has not passed for l" until L passes a smoke.

**The decision after the smoke (2026-11-19).** The journal explains L (section
5): the gateway's ChatGPT login returned 401 and the breaker has been open since,
which only the owner can renew. The pilot runs on three arms (header `arms`
`["sa","sb","i"]`). The driver's smoke stamp became per arm, and the third run's
stored bundles were **re-stamped without a new run** with
`scripts/e1 smoke --restamp --arms sa,sb,l,i`: S-a, S-b and I passed (stamp
time 07:20Z, the bundles' export time, source "restamp: bundles/s00-<arm>.json"),
L failed at step 9 with the circuit-open run error recorded. The stamps expire
six hours after the export (about 13:20Z on 2026-11-19); after that the pilot
needs one more smoke of the three arms (`smoke --arms sa,sb,i`).

**What the first two runs showed, fixed in the driver:** run 1 read run ids from
`run_id` where the bundle's run records say `loop_id` (every research run was
invisible); `System` had to be made before its overlay directory. Run 2's step 10
failed on all four arms because the research runs that night searched the corpus
and not the web (started 0): the smoke objective was changed to ask for
web reporting, and run 3 shows 2, 3 and 7 started searches. A model that
chooses not to search leaves a unit valid with started 0; the pilot's
objectives are real stories, and `no_research` and started 0 are reported.

**Not exercised:** a cache-answered search (the invented bundle only); the
`reported_model` field (empty on all three arms: no provider reported a
different id); a timed-out run's cancel and re-export; `pilot`, `read` and
`end` against the real routes (they ran against the stub only).


### 14.7 After the pilot

- If P1 is false, E1-M stops. The research eval route is designed next, and
  the readout says so.
- If P1 is true, the readout gives the sd of d and states the full run's n
  (40, or 60 if sd of d is above 1.2). It waits for the owner's word (D-E3).
- If P1 is void, nothing is concluded; the readout says which stop starved it.

Either way:

- the lead settles P1, P2 and P3 from the verdict lines, copying them; the
  computed line is the teacher and the forecaster does not judge (section 8);
- `scripts/e1 end` removes the overlays;
- the readout is appended to this file;
- the four computers are deleted only after that commit.

### 14.8 What the pilot cannot show

- Anything about the harness: it varies the model only (section 1).
- Anything about the paper's corpus: the arms start empty (section 1).
- The searches of `choir.Research` children (section 3).
- The paper's live reasoning effort, assumed `low` from the repo (section 5).
- The run-to-run noise of the I arm: only S has a second computer.
- Anything about `gpt-6-luna`: arm L is dropped for the pilot (section 5); tonight's contrast is S against I only.
- Whether a cache-answered search reproduces at the paper's rate: the 13% is the record's, and the pilot's rate is whatever `cache_answered` shows.
- A real tail: ten objectives on two S computers give 20 S units; the tail share is descriptive.
