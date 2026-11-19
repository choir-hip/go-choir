# Calibration experiment E1: the model or the harness (protocol, rewritten 2026-11-19, round 2)

Status: design (green). Station L2 of
[the night's goal file](../definitions/choir-night-exact-vectors-learning-2026-11-19.md).
Round 2 answers seat 1's review (four blocking findings) and the lead's
eleven decisions. Tonight only a **pilot** may run (section 14). The full run
waits for the owner's word. Deleting the 4 November arms is a separate red
landing that is due on main before 2026-11-24 (section 10). The file keeps its
old name so that existing links and the registry's `hold` lines still resolve.

Owner, 2026-11-19: "if the problems are at the model level, naively changing
the harness or prompt may not be correct. but this does relate to the
calibration experiment. both are about prompt/harness optimization and
learning. the calibration experiment is something that we should do now,
actually. you can redesign it as you see fit."

How claims are marked: **[read: file:line]** or **[read: record]** means read
for this design (seat 1 checked most of these against the code); **[assumed]**
means not checked, and the smoke step checks it.

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
captures of its own.** Each arm runs on a fresh test computer. Captures are
scoped to their owner [read: internal/researchtools/researchtools.go:82], so
an arm computer starts with an empty corpus. The paper's research runs
search about 207,000 held pages. The published-stories layer is the one
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

**Primary outcome: web searches**, meaning the number of `search.served`
lines across the unit's research runs. The host writes one line "per finished
desk search", naming each provider asked and what it answered
[read: internal/types/search_served.go:8-35]. The model cannot write these
lines. The sum of the providers' `Calls` is the experiment's **search tally**,
the cost bound of section 7. On the paper, nearly every search is a single
Keenable request [read: search-volume record, Investigation], so the line
count is comparable with the record's request counts.

**Which model served the unit** comes from `model.served`. That record names
the model that actually answered each call "after any quota switch or pool
failover", and its `Switches` list each mid-run move
[read: internal/types/model_served.go:7-43]. A research run with any
`Switches` entry, or with any call served by a model other than its arm's, is
**invalid by rule**. The document route `GET /api/texture/documents/{id}/models`
gives the same facts by role [read: internal/textureowner/texture_model_provenance.go:14-17].

**A unit is valid when** all of these hold:

1. it has at least one research run;
2. every research run has ended (any terminal state) within 60 minutes of the
   submission;
3. no research run has a model switch or a foreign model;
4. its trajectory's trace bundle was exported;
5. its computer reported the same `build` in `GET /api/runtime/observability`
   [read: internal/agentcore/api.go:871-874] at submission and at the
   objective's end, and that build matches the other arms' build for the same
   objective.

A failed check 5 invalidates the objective on every arm.

**Secondary outcomes, counts only**, from the bundle's `desk.cell` and
`desk.egress` layers [read: cmd/choir/trace.go:715 onward]:

- searches by verb (`SearchAll` against plain), and searches per subject;
- pages read, and the largest number of reads of one page;
- per-cell bound refusals;
- **new-host reads**: pages read from a host the story does not cite (follow-ups only);
- whether the run read a paper story from the published layer (a covariate);
- model calls and input tokens.

Whether any of these per-search fields is present in the event payloads is
checked in the smoke step [assumed]. The gateway journal is not a source,
because a scoped key cannot read it.

## 4. Method, arithmetic, and what each outcome means

**One computer per arm.** Every arm computer is made fresh for the
experiment. All arms get the same objectives in the same order, and each
objective starts on every arm at the same moment.

- No arm sees pages another arm captured.
- Every arm runs the same release at the same moment.
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

d = mean(S-a, S-b) of ln(1 + searches) − mean(L, I) of ln(1 + searches)

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
| mean d ≥ 0.405 and the 95% interval's lower bound > 0 | The many-search count is specific to `swe-2-flash` among the models tried. The lever is the model choice. Change no harness for this tail on E1-M's strength. |
| the 90% interval lies inside ±0.405 | The models tried search alike, within 50%. The behaviour is common to them. Nothing is shown about the harness; E1-H is needed before any harness change. |
| mean d ≤ −0.405 and the 95% interval's upper bound < 0 | `swe-2-flash` searches less than the others. |
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
| S-a | `e1-sa` | `swe-2-flash` | `devin` (Devin's API through one person's CLI session); the default pool's lead and the paper's research model [read: internal/modelcatalog/catalog.go:516-523] | none until 2026-12-04 [read: same, comment] |
| S-b | `e1-sb` | `swe-2-flash` | `devin`, as S-a | none |
| L | `e1-l` | `gpt-6-luna` | `chatgpt`: ChatGPT subscription billing through Codex OAuth, the Responses endpoint [read: internal/provider/provider.go:1268-1287; catalog.go:362-368] | none (subscription) |
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
  and Luna is in the default pool today. L is kept; decision D-E2.

Texture and the conductor run on each computer's default policy, the same on
every arm, so only research differs.

## 6. Arms without switches

| | |
|---|---|
| **How an arm is set** | An overlay file `System/model-policy-overlays/e1-<arm>.toml` on that arm's computer, installed with `choir files put` (the verb exists for overlay installs [read: cmd/choir/files.go:1-5; cmd/choir/commands.go:390-396]). It names only the research role, with an end: `[overlay] expires_at = ...` and `[roles.research] provider, model, reasoning_effort` [read: internal/modelpolicy/model_policy.go:1015-1060, :1223-1229]. Every submission names it as `model_policy_overlay_id`. |
| **How it is recorded** | The route writes the overlay id into run metadata as `llm_policy_overlay_id` [read: model_policy.go:29; internal/textureowner/api_texture_prompt_eval.go:78]. The trajectory carries it as `SubjectRefs[TrajectoryRefModelPolicyOverlay]` [read: internal/textureowner/texture_handoff.go:255-257]. Research runs inherit it [read: internal/agentcore/runtime.go:1746-1749, :5547-5575]. The reader takes the arm from the run and the model from `model.served`, never from the driver's plan. |
| **How it is read back** | `choir trace export <trajectory>` (scopes `read:runtime`, `read:texture` [read: cmd/choir/commands.go:702-707]), plus `GET /api/runs/{id}/events`. |
| **How it ends** | At `expires_at` the route refuses the overlay rather than falling back [read: model_policy.go:758; api_texture_prompt_eval.go resolve check]. `scripts/e1 end` deletes the overlay files. After the readout is committed, the computers are deleted. |
| **Registry** | The five `calibration_e1_*` entries go with their code (section 10). E1-M adds no entry: it adds no setting, environment value or code path, which are what the lint scans [read: scripts/check-switch-registry:48-52, :255-260]. Its record is section 14.1 and its end is the overlay expiry, which the code enforces. E1-H's surface variant is code, so it gets an entry (section 9). |

**A prompt arm is not in this protocol.** A prompt override is stored per
owner [read: internal/promptstore/store.go:83-93, :203-204], so it cannot
differ between runs on one computer. Putting it on a second computer would
repeat the confound this design removes. The prompt arm waits for a per-run
prompt pin, which goes beside E1-H's surface variant.

## 7. Cost, and the bound on it

Search credits are the owner's money. Keenable reports no balance; the
reading is "a number a person copies from the usage page"
[read: search-volume record, Remaining error]. The last reading is 17,000
credits at 2026-11-18T12:38Z [read: scripts/status-paper-usage:59]. The
budget that reaches 5 December is about **880 searches a day**
[read: search-volume record, Investigation]. (`status-paper-usage` computes
1,032 a day from the same reading.)

| | Units | Expected searches (7.5 a unit) | Hard stop on the tally | In days of budget (880/day) |
|---|---|---|---|---|
| Pilot (tonight) | 10 objectives × 4 arms = 40 | about 300 | **450** | about 0.34 expected, 0.51 at the stop |
| Full run (owner's word) | 40 × 4 = 160 | about 1,200 | 1,800 | about 1.4 expected, 2.0 at the stop |

The hard stop is a bound the experimenter sets on the experiment's own cost.
It is not a limit on model calls.

**The refusal rule before starting.** The driver estimates the remaining
credits as the last reading minus the Keenable calls counted in the gateway
journal since that reading (read-only over SSH, as the status scripts already
do). It refuses to start when:

- the estimate is below 10 times the hard stop (4,500 for the pilot);
- the estimate cannot be made;
- `status-paper-usage` shows Keenable not answering or `quota_limited`.

At the paper's rate since the reading, the estimate may well be below 4,500.
In that case the pilot does not run tonight, and that is the correct outcome.

**Model calls:** 18.5 calls and about 0.41M input tokens per research run on
2026-11-09 [read: docs/evidence/token-efficiency-2026-11-09/research.md:21, :88].
The pilot's 40 units therefore come to about 740 research calls and 16M input
tokens, plus Texture's and the conductor's calls (not measured). All of it
goes to the three no-charge routes above.

## 8. The reader, the ledger gate, and why nothing lapses

- **The verdict is computed.** `scripts/e1 read --verdict` applies the rules
  of sections 3, 4 and 14.4 to the trace bundles and prints `true` or
  `false` for each claim. Settling a claim is copying that line.
- **Too few valid units settles false, never void.** A void is unscored, so
  it would be a lapse under another name.
- **The driver refuses to start a counted run** unless the claim ids are in
  the Claims table of `docs/calibration/claude-ledger.md` as read from
  `origin/main` (`git show origin/main:...`). They must be pushed, not only
  written. The lead enters them.
- **The validity filter and the reader are one script with a fixture**
  (synthetic bundles covering: a switch, a foreign model, a `no_research`
  submission, a build change, a missing bundle, and a unit with several
  research runs). One review seat reads the script and its fixture before the
  first counted run, because the outcome depends on code the author wrote.
- **The readings are a pure function of the bundles.** The driver's own state
  (`~/.cache/choir-status/e1/`) only lets it skip what it has already done.
  The computers are deleted only after the readout is committed.
- **`scripts/status-hourly` gains an E1 section** that shows the latest
  outcome line and flags any of these: no unit finished in three hours while
  the pilot is open; the end time passed with no verdict; a verdict exists but
  the ledger's settlements table has no row for the claim ids. The flag leads
  the status line. Settling still depends on someone running the hourly
  status; the ledger's settle-by time is the backstop.

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
  - `calibration_e1.go` (the whole file);
  - `calibration_feedback.go:580, :642` (delivery checks become the push default);
  - `calibration_reads.go:51-54` (`restateAsked`);
  - `engineering_precommit_rules.go:176`;
  - `inbox_records.go:788`;
  - `rlm_reduce.go:202, :715-720, :1239`;
  - `tools_capsule.go:1106`;
  - `tools_desk.go:922-923`.
- **The `calibrationRestatement` wrapper** (`calibration_e1.go:59-61`) moves
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
   the cost is lost units, not bias. Two S computers and the paper share
   Devin's single session. The driver reads the paper's Devin refusals in
   `status-hourly` and stops (section 14.5).
4. **Test computers publish.** The eval route opens Texture with `open_app`,
   not publish [read: api_texture_prompt_eval.go, `CompletePromptBarDecision`
   spec]. Auto-publish applies only to "a story opened with publish"
   [read: internal/textureowner/texture_cell_author.go:486]. Checked anyway in
   the smoke step and after every objective.
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
- **D-E2: the models.** Default: `swe-2-flash` twice, `gpt-6-luna` via
  ChatGPT, `iceman-contributor-free` via opencode-go. Stand-in for I:
  `recursive/flint-v1.0:free` via OpenRouter (a free model, within the
  OpenRouter rule). If neither free model passes the smoke step, run three
  arms (S-a, S-b, L).
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
- **Kept.** "Every record says what it actually got" (old F6). Fixed n, no
  early stop for success, inconclusive reported as inconclusive.

## 14. Pilot, tonight

### 14.1 The record

| | |
|---|---|
| Experiment | E1-M pilot |
| Computers and arms | `e1-sa` S-a, `e1-sb` S-b, `e1-l` L, `e1-i` I (section 5) |
| Objectives | 10: 5 `thin_event`, 5 `follow_up`, fixed and committed before the first counted run |
| Units | 40 |
| Hard stop | 450 on the search tally; at 315 (70%) no new objective starts until `scripts/e1 pilot --continue` |
| Overlays expire | 2026-11-20T12:00:00Z |
| Pilot ends | when 10 objectives are done, a stop fires, or 2026-11-20T06:00Z |
| Claims | three, entered and pushed by the lead before the first counted run (14.4) |

### 14.2 Making the computers and installing the arms

On the lead's Mac, from the primary checkout's frontend dependencies (the
script loads Playwright from `frontend/` [read: scripts/qa_test_computer.mjs:16-23]):

```
for a in sa sb l i; do node scripts/qa_test_computer.mjs --label e1-$a; done
```

Each run prints one JSON line of identifiers and the 0600 key-file path
[read: qa_test_computer.mjs:1-14]. `scripts/e1 smoke` collects them into
`~/.config/choir-qa/e1/computers.json` (mode 0600; it holds paths, not keys),
in this shape:
`{"sa": {"label", "computer_id", "key_file", "overlay": "e1-sa", "model": "devin/swe-2-flash"}, ...}`.

Overlay file for each arm (S shown; L uses `chatgpt` and `gpt-6-luna`, I uses
`opencode-go` and `iceman-contributor-free`):

```
[overlay]
expires_at = "2026-11-20T12:00:00Z"

[roles.research]
provider = "devin"
model = "swe-2-flash"
reasoning_effort = "low"
```

Installed with:
`choir files put System/model-policy-overlays/e1-sa.toml --local e1-sa.toml --api-key-file <key_file>`.
`scripts/e1 smoke` does this for all four.

### 14.3 The objective file

It lives at `docs/calibration/e1/pilot-objectives.jsonl`, committed (a docs
commit) before the first counted run. Its SHA-256 goes into the claims. Only
public news subjects appear in it.

- **Line 1, the header:**
  `{"t0": "<UTC>", "rule": "e1m-pilot-v1", "arms": ["sa","sb","l","i"], "order": "file order"}`.
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

A smoke objective `s00` (a `follow_up`, not in the file) is used only by the
smoke step.

### 14.4 The claims for the pilot (the lead enters them)

Kind `experiment`. Settle by 2026-11-20T08:00Z. Evidence: `scripts/e1 read
--verdict` on the bundles. Fewer valid units than a claim needs settles it
**false**.

- **P1, p = 0.45.** At least 2 of the 20 valid S units (S-a and S-b over the
  10 objectives) have 10 or more web searches. At the paper's rate of 0.19,
  the chance of 2 or more in 20 is 0.92. The price is low because the empty
  corpus, the Texture-written first message and the shared published layer
  all push searching down. **This is the gate.** If P1 is false, the tail does
  not reproduce on an empty computer, E1-M stops, and the research eval route
  comes first.
- **P2, p = 0.50.** On at least 7 of the 10 objectives with both S units
  valid, ln(1 + searches) of S-a and S-b differ by less than ln 2 (the same
  model on two computers lands within a factor of two).
- **P3, p = 0.50.** At least 36 of the 40 units are valid (section 3) by
  2026-11-20T06:00Z.

P1 and P2 come from the same bundles, and both depend on P3's validity rule,
so they are correlated. Each is still priced on its own.

### 14.5 The driver

`scripts/e1` (Python, developer-side, yellow) has three subcommands.
`read`'s rules and its fixture are reviewed by one seat before `pilot` runs.

**`scripts/e1 smoke`** makes or collects the four computers, installs the
overlays, submits `s00` to each, and prints one `PASS` or `FAIL` line per
check per arm. It exits 0 only if every check passes:

1. the eval route answers 202 with the arm's overlay id (a missing or
   unparsable overlay is a 400 [read: api_texture_prompt_eval.go resolve check]);
2. the trajectory resolves. `doc_id` from the 202 leads, via `GET
   /api/texture/documents/{doc_id}`, to `trajectory_id`
   [assumed field; the document carries `TrajectoryID`
   (texture_model_provenance.go:58)]. Then `GET /api/runs?trajectory_id=<id>`
   [read: internal/agentcore/api.go:569] lists the research runs. If research
   runs are not on that trajectory, the fallback is `GET
   /api/inboxes/research:<doc_id>/records`
   [read: internal/agentcore/api_inbox_records.go:29-30]. The check prints
   which path worked;
3. at least one research run ended, and its `model.served` names only the
   arm's model with no `Switches`;
4. `search.served` lines are present, and `desk.cell` and `desk.egress` carry
   the per-search and per-read fields `read` needs (it prints which are
   missing);
5. `choir trace export <trajectory_id> --out ~/.config/choir-qa/traces/e1/<arm>-s00.json`
   succeeds;
6. **no publication**: `GET /api/texture/documents/{doc_id}/published`
   [read: internal/textureowner/router.go:85] reports no published version,
   and a corpus search from `e1-sa` for the `s00` document's title returns
   no published hit from any e1 owner;
7. all four computers report the same `build`.

**`scripts/e1 pilot [--continue]`** refuses to start unless all of these hold:

- the claim ids are in the ledger on `origin/main`;
- `smoke` has passed within 6 hours (from its stamp file);
- the objective file's SHA-256 matches the one in the claims;
- the overlays have not expired;
- the credit rule of section 7 passes (it prints the estimate and the
  threshold).

Then, for each objective in file order:

- it submits to all four arms at once;
- it waits until every unit is ended or 60 minutes pass;
- it exports the bundles and runs check 6 for each document;
- it appends the outcome line to `~/.cache/choir-status/e1.tsv`;
- it prints one line, for example:
  `p03 thin_event | sa 12 sb 9 l 4 i 6 searches | runs 1/1/2/1 | valid 4/4 | tally 141/450 | build ok | published 0`.

It stops, printing why, when:

- the tally reaches 450;
- the tally reaches 315, until `--continue`;
- `status-hourly` shows the paper's searches failing, Keenable
  `quota_limited`, or the paper's Devin calls refused;
- any test document is found published: stop and record it, then unpublish
  through the operator yank route [assumed path: the `choir platform yank`
  verb, `cmd/choir/platform_yank*`];
- an arm fails 3 units in a row: that arm stops, the others go on, and the
  failure is a finding;
- the builds disagree: that objective is invalid on every arm, and the driver
  goes on.

**`scripts/e1 read [--verdict]`** recomputes every unit from the bundles. It
prints:

- the unit table (searches, research runs, validity and the reason for any
  invalid unit, position);
- per-arm medians;
- the tail share and the 90th percentile;
- d with its intervals and its sd;
- the S-a/S-b differences;
- the outcome line.

With `--verdict` it prints `P1 true|false`, `P2 ...` and `P3 ...` with the
counts behind each, ready to copy into the settlements table.

### 14.6 After the pilot

- If P1 is false, E1-M stops. The research eval route is designed next, and
  the readout says so.
- If P1 is true, the readout gives the sd of d and states the full run's n
  (40, or 60 if σ > 1.2). It waits for the owner's word (D-E3).

Either way:

- the lead settles P1, P2 and P3 from the verdict line;
- `scripts/e1 end` deletes the overlays;
- the readout is appended to this file;
- the four computers are deleted only after that commit.
