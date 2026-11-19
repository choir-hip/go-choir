# Calibration experiment E1: the model or the harness (protocol, rewritten 2026-11-19)

Status: design (green). Station L2 of
[the night's goal file](../definitions/choir-night-exact-vectors-learning-2026-11-19.md).
The first experiment, E1-M, can start today with no code change: it uses
routes and files that exist, plus one developer-side script (yellow). The
deletion of the 4 November arms is a separate red landing that must reach
main before 2026-11-25. Nothing here is built yet. The file keeps its old name
so that links to it, and the registry's `hold` lines, still resolve.

Owner, 2026-11-19: "if the problems are at the model level, naively changing
the harness or prompt may not be correct. but this does relate to the
calibration experiment. both are about prompt/harness optimization and
learning. the calibration experiment is something that we should do now,
actually. you can redesign it as you see fit."

How claims are marked: **[read: file:line]** or **[read: record name]** means
read for this design; **[assumed]** means not checked, and the smoke step
(section 7.2) or a named refutation (section 9) checks it.

## 1. The question and the unit

**The question.** When an agent on this computer fails in the same way again
and again, is the cause what the model does, or what the harness showed it,
allowed it and told it? And when we change the prompt or the harness, does
the change improve an outcome that neither the agent under test nor the
person who made the change can write?

**What is compared.** One thing at a time, with everything else held:

- the **model**, with the prompt, the verbs and the cell results fixed;
- the **prompt text**, with the model and the verbs fixed;
- later, **the surface**: what a cell result shows, or which verbs exist,
  with the model fixed.

**The unit is one research run on one objective.** Arms are compared on the
same objective (a paired design), so the subject of the story, which drives
most of the variation, cancels out. A computer is not the unit: putting one
arm on each computer would mix the arm with the computer's age, store and
traffic. A day is not the unit: the harness changes several times a day
under "expedite everything" (CLAUDE.md, Delivery).

**The outcome must be written by the host, not by the agent or by us.** The
measures are counts the host records about the run: web searches started,
pages fetched and read, host calls per verb, refusals, the largest number of
reads of one page, and whether a page was read from a host the story did not
already cite. The model chooses what to do; it cannot write these counts. The
experimenter fixes the objectives and the rule that turns the counts into a
verdict before the first counted run. Where quality needs a person, it is the
owner's blind verdict on a sample, and it is never on the critical path
(section 5).

### Is an experiment even the right instrument? The four cases

| Failure (record) | What it is | Instrument |
|---|---|---|
| One question reworded into 12 to 44 web searches in one run (search-volume record, item 2: 10 of 54 runs started 10 or more searches and made 56% of all searches; the four named runs sent 39, 42, 44 and 15) | Classed "model behaviour under a harness that does not stop it", confidence 0.65; not shown that another model does better [read: search-volume record, "Traces read", item 2] | **The experiment.** This is the main question of E1-M: does the tail appear with every model (the surface or the task causes it) or with one (the model does)? |
| A run reads about 5% of returned pages whatever is returned (item 3: reads stay at 7 to 25 whether a run was handed 80 or 1,680 pages) | Not established. The likeliest reading is that reads follow what the story needs and searching is the misfit (0.5) [read: same, item 3] | **Measured, not tested, in E1-M.** Reads per run and the share are recorded for every arm. The lever (titles in the cell result) is a surface arm, so it waits for E1-H. If reads per run do not move across three models either, the 5% is probably not a failure and is dropped. |
| One page read 3,090 times in one run (`c0351184`; 3 cells x 1,024 host calls) | One case ever seen on any console Node B holds [read: same, item 1]. Cause not established: 0.6 a model-written stepping mistake, 0.7 a surface that makes a long loop natural, and both can hold | **Not an experiment yet; a defect to read first.** One event cannot be rated. 160 runs would see it at least once with probability 0.91 if its true rate is 1.5% a run (1 − 0.985^160), so E1-M gives a recurrence count, but not a per-model rate (40 runs an arm: 0.45). The second trace pass (section 8) reads its three cell programs. If the program walked a huge page correctly, it is a defect in the verb and is fixed without an experiment. If it is a stepping mistake, the objective is replayed 10 times on the paper's model and 5 on each other (a reproducibility probe, 20 runs). |
| Management chooses feed or open with `candidates_shown=0` (186 of 233 choices) | **A broken instrument, not a behaviour**: the count only moves when the model calls `choir.Stories`, and the opening already holds the page; 139 runs fed a story by an id they could only have seen there [read: research-does-not-start-from-the-corpus record, `candidates_shown=0` section] | **A defect to fix.** Add the opening's story count to the line (yellow). The `choir.Stories` paging refusal (a 67-byte cursor against a 64-byte bound) is a harness defect, certain [read: recurring-agent-failures record, item 1], and is fixed (orange). Whether management chooses well is a later experiment that needs a person's reading of overlap, not this line. |

The other failures in the recurring-failures record (dropped citations, a
publication that stays a gap, refused publish-auto work, `max_tokens` with no
text, repeated searches in one run) are not established and are read from
traces before anything is varied (CLAUDE.md, rule 4).

## 2. Telling the model from the harness

### The method

Three moves, each holding everything but one thing, on a fixed set of
replayed objectives.

1. **Same model, same harness, twice.** This is the noise floor: how far two
   runs of one model on one objective land apart. Any difference between arms
   smaller than this is not a difference.
2. **Same harness, other models.** A behaviour that appears with every model
   belongs to the task or the surface. One that appears with one model and
   not the others belongs to that model. Changing the harness to fix the
   second kind would mean tuning the harness to one model's habit, which is
   the owner's warning.
3. **Same model, harness varied one way at a time** (E1-H, section 8). A
   behaviour present in move 1 and 2 and gone under one surface change is a
   harness cause, and that change is the fix.

The first experiment does moves 1 and 2. Move 3 waits until move 2 and the
cell programs say which surface change is worth building.

### Sample size, from the observed numbers

The numbers below are from the paper's research desk, 2026-11-19, 01:00Z to
05:22Z [read: search-volume record, items 2 and 3].

- Web searches per run: median 4, mean 7.5 (641 in 85 runs), largest 44, 42,
  39, 38, 27, 26; 12 of 85 runs at 14 or more.
- On the log scale, ln(1 + searches): median ln 5 = 1.61; about the 90th
  percentile at 15 searches, ln 16 = 2.77. The spread between runs is
  therefore about (2.77 − 1.61) / 1.28 = **0.9 to 1.0**.
- Share of runs with 10 or more searches: 10 of 54 = **0.19**.

How much of that spread is the objective and how much is the same model
sampled twice is not known. It is the first thing E1-M measures (claim C146).
Take the worst case, that all of it is run-to-run noise (σw = 1.0).

**Primary comparison, paired.** For each objective, the paper's model's
ln(1 + searches) (mean of its two runs) minus the mean of the two other
models. The standard deviation of that difference is about σw (each side is a
mean of two runs, so each carries σw²/2). For a two-sided 5% test with 80%
power the number of objectives is n = (2.80 × σw / Δ)².

| Effect to detect | Δ on the log scale | n at σw = 1.0 (worst) | n at σw = 0.6 |
|---|---|---|---|
| half the searches | 0.69 | 17 | 6 |
| 30% fewer | 0.36 | 60 | 22 |

With **40 objectives** the smallest difference detected in the worst case is
2.80 × 1.0 / √40 = 0.44, that is about 36% fewer searches. That is enough to
separate "this model rewords and the others do not" from noise; it is not
enough to rank small differences, and the protocol does not try.

**The tail share as a yes or no.** Moving from 0.19 to 0.05 unpaired needs
about 84 runs an arm (two proportions, 5% and 80%). The paired design does
better, but the tail share is reported, not tested.

**The read loop.** Covered in section 1: 160 runs see a 1.5% event with
probability 0.91; zero events in 160 runs put its rate under 1.9% (3/n) with
95% confidence.

**The noise floor itself.** The two runs of the paper's model on the same 40
objectives give 40 replicate differences. Their spread is the direct measure
of σw, and it is reported beside every comparison so a reader can see the
null distribution, not only a p-value.

## 3. Where it runs

Rules: never the newspaper computer, never the owner's computer, never a real
user's (the night file's constraints, and CLAUDE.md, Proof: drills on d@e
only).

**E1-M runs on one fresh test computer, `e1-m`, and every model arm lives on
that one computer, assigned per run.** This is possible because a route that
does exactly this already exists: `POST /api/evals/texture-prompt` starts the
same conductor to Texture path as the prompt bar but pins a model-policy
overlay onto the trajectory; the overlay "covers the Texture run ... and any
researcher it spawns (via coagent overlay inheritance)"
[read: internal/textureowner/api_texture_prompt_eval.go:38-43]. The route
refuses an unknown or expired overlay instead of falling back
[read: same file, the resolve check after the request decode; internal/modelpolicy/model_policy.go:734-762, expiry at :758].
So the model is the only thing that differs between two runs on `e1-m`: same
release, same store, same corpus view, same minute.

Why a fresh computer and not the others:

- **Not the five computers of the 4 November design** (claude, deepseek,
  luna, pool, crouton). Each was pinned to one model, so model and computer
  were the same thing; F7 of the old protocol said so ("E1-E only prunes").
  They differ in age and history, crouton's model was retired on 2026-11-09
  [read: internal/modelcatalog/catalog.go, the comment above `Pools()`], and
  their current state was not read for this design. They hold no arm.
- **Not d@e as the main computer.** It is the proof ring (R0) and carries
  other proofs and drills (Autoradio proofs ran on it all of 2026-11-17 and
  18 [read: docs/current-architecture.md, Autoradio rows]); their runs would
  share its slots and corpus view with the experiment's. d@e is the fallback
  if a fresh computer cannot be created, and the place a smoke run may go.
- **Forks of one computer would be cleanest** (identical state, own
  identity; the term is defined in docs/computer-ontology.md under "forked
  computer"), but no computer fork route is built (the only fork route found
  is `/api/texture/fork` [read: internal/textureowner/router.go:39]). A fresh
  computer from today's release is the next best thing: every arm starts
  from the same empty state.

Fresh computers are made by `node scripts/qa_test_computer.mjs --label e1-m`,
which creates a test owner and computer on staging and writes a key scoped to
that one computer to a 0600 file under `~/.config/choir-qa/test-computers/`
[read: scripts/qa_test_computer.mjs:1-14, :31-44]. The key's scopes include
`read:runtime` and `write:runtime`; whether they cover the eval route and
`choir trace export` is checked in the smoke step [assumed].

A second fresh computer, `e1-p`, holds the prompt arm (E1-P, section 7.8),
because a prompt override is stored per owner
[read: internal/promptstore/store.go:83-93, :203-204; internal/agentcore/prompts.go:178-210]
and each test computer has its own owner.

## 4. Arms without switches

The contract forbids feature switches; an experiment arm is allowed when it
is recorded, dated and ended. E1's arms are not settings at all.

| | Model arm (E1-M) | Prompt arm (E1-P) |
|---|---|---|
| **How it is set** | An overlay file per model, `System/model-policy-overlays/e1m-<model>.toml` on `e1-m`, written with `choir files put` (the files verb's stated purpose is overlay install [read: cmd/choir/files.go:1-5]). It names only the research role: `[overlay] expires_at = "2026-11-24T00:00:00Z"` and `[roles.research] provider = ..., model = ..., reasoning_effort = "low"` [read: model_policy.go:1015-1060 for the sections, :1223-1229 for the keys]. Texture and the conductor stay on the computer's default policy. | `PUT /api/prompts/research` on `e1-p` with the arm's text. |
| **How it is recorded** | The overlay id rides the run's metadata (`model_policy_overlay_id`, `eval_kind=texture_prompt`) [read: api_texture_prompt_eval.go metadata block]. The reader takes the arm from the run, never from the driver's plan, and takes the model actually served from each call's provenance; a run with any call served by another model is invalid. This keeps the one good idea of the old protocol (F6: "every record says which delivery it actually got"). | The prompt version stamped on records today hashes only the shared overlays and the desk's runtime overlay, not the role prompt or an owner's override [read: internal/runtimeprompts/prompts.go:149-155]. So the reader records the override's SHA-256 from `GET /api/prompts/research` before and after each block, and a block whose two readings differ is invalid. (Stamping the effective prompt hash on the run is a small yellow change listed in section 7.9; E1-P does not wait for it.) |
| **How it is read back** | `choir trace export <trajectory>` on `e1-m` [read: cmd/choir/trace.go:3-7]; bundles stay outside the repo. | The same, on `e1-p`. |
| **How it ends** | The overlay's `expires_at` is the end. After it, the route refuses the overlay (model_policy.go:758), so no arm run can start by mistake. `scripts/e1 end` then deletes the overlay files. | `scripts/e1 end` sends `DELETE /api/prompts/research` (reset to the default) and reads it back. |
| **What is left behind** | Nothing on any other computer. After the readout is committed, the test computers are deleted. The trace bundles stay in `~/.config/choir-qa/traces/e1/` on the lead's machine (owner data, outside the repo). | Same. |

**The registry.** The five `calibration_e1_*` entries
[read: switches/registry.yaml:2009-2120] are deleted with their settings and
code (section 6). E1-M and E1-P add **no** registry entry, because they add
no setting, no environment value and no code path; the registry lint scans
those and nothing else [read: scripts/check-switch-registry:48-52, :255-260].
The experiment's record is section 7.1 of this file plus the ledger claims,
and its end is enforced by the overlay expiry in code. When E1-H adds a
run-scoped surface variant (code), that variant gets a registry entry with
`life: experiment`, `expires` at its end date, and `retire: delete`, and the
code itself refuses the variant after that date (section 8). Decision with a
default in section 10, D-E3.

## 5. The reader, and why nothing can lapse

The lead's ledger has 31 lapsed claims of 125 in its second form
[read: docs/calibration/claude-ledger.md, Scorecard, recounted 2026-11-19T00:15Z];
three of them were found only by a recount. A claim lapses when settling it
needs someone to remember, to fetch evidence, and to judge. E1 removes all
three.

1. **The verdict is computed, not judged.** `scripts/e1 read` turns trace
   bundles into one row per run and the rows into the precommitted
   statistics and a verdict word (`true`, `false`, `incomplete`) by the rule
   written in the claim. Settling a claim is copying that line.
2. **The readings are a pure function of the traces.** The script keeps no
   state that matters: it re-reads every bundle and recomputes. Lost cache,
   a restarted driver or a sleeping Mac lose nothing while the test computer
   holds the runs. The test computers are deleted only after the readout is
   committed.
3. **The driver runs without a session.** `scripts/e1 run` is a restartable
   process (its state is the list of submitted run ids in
   `~/.cache/choir-status/e1/`); a restart skips submitted runs and exports
   any missing traces. It reads after each block of objectives and appends
   the outcome line to `~/.cache/choir-status/e1.tsv`.
4. **The hourly status shows it until it is settled.** `scripts/status-hourly`
   (which already appends its other readings under `~/.cache/choir-status/`
   [read: scripts/status-hourly:1-15]) gains one section: the latest E1 line,
   and a flag when (a) no new run has finished in three hours while E1 is
   open, (b) the end time has passed with no verdict line, or (c) a verdict
   line exists and the ledger's settlements table has no row for C145, C146
   and C147 (the section greps the ledger). The flag leads the status line, so
   an unsettled E1 is the first thing the next session reads.
5. **Short n is a settlement, not a lapse.** Each claim says what happens if
   too few runs are valid by the end: C147 settles false and C145 and C146
   settle `void` with the count named. Nothing is left to judge.

**Stopping rule.** Fixed n: 40 objectives, every arm. No early stop for
success. The reader reports inconclusive as inconclusive. Stops for harm, any
one of which pauses the driver (it backs off; it does not cap anyone's calls):

- a provider refuses an arm's calls (quota, revoked session): that arm
  pauses until the provider answers again; the others go on;
- the search plane reports Keenable `quota_limited`, or `status-hourly` shows
  the paper's own searches failing: the whole experiment pauses, because
  the paper comes first;
- an arm's runs fail to finish in more than a quarter of cases: that arm
  stops, and its failure rate is the finding (a model that cannot drive the
  cell surface is a model-level result);
- the Devin provider refuses any of the paper's calls during an E1 block (one
  person's CLI session carries both [read: catalog.go, the Devin paragraph
  above `Pools()`]): the paper's-model arm moves to the paper's quiet hours.

## 6. The old five arms: delete them, with their code

**Decision: delete** `calibration.e1_restate`, `e1_pull`, `e1_stub`,
`e1_two_stage` and `e1_baseline`, their readers and request builders in
`internal/agentcore/calibration_e1.go` (the delivery choice at :64-87, the
restatement paths at :166 and :288), the `choir.Restate` verb, their constants
and defaults in `internal/store/computer_setting.go:181-198, :222-226`, and the
five registry entries.

Why:

- **They answer a smaller question than the one now asked.** The old E1
  asked whether showing a desk its own calibration record makes its stated
  probabilities better. The learning engine's own reading of 341 settled
  claims is that perfect recalibration could gain at most about 0.016 Brier,
  and that the gap is discrimination, which "a number about the self can fix
  only" in the small term [read: docs/learning-engine.md, section 9.2]. The
  owner's question now is about behaviour failures and whether a change
  helps.
- **They cannot start.** Their precondition was a 12-hour window with no
  prompt, model or calibration change on five computers, and E1-M needed 112
  Casts (about 28 hours) and E1-E's two-stage arm about 140 hours, on
  computers whose round rate today is not known. Under "expedite everything"
  no 12-hour quiet window comes.
- **They are switches in all but name**, off everywhere for fifteen days,
  and walked once past the gate already
  [read: docs/problems/dated-switch-gate-turned-main-red-at-midnight-2026-11-19.md].

What stays: the shipped digest push to half of engineering's rounds (the
cut-2 A/B) is shipped behaviour, not an E1 arm, and is untouched. The claims
route `GET /api/calibration/claims` and the read marks stay; E1-M does not
read them. The type-level p1 filter (`types.RestatementRulePrecommit`,
read by `calibration_feedback.go:194, :390, :685`) stays until it is proved
that no p1 record exists on any tape; it costs nothing and removing it is the
only part of the deletion that could miscount a digest (old F1).

Before the deletion lands, read each computer's stored settings for the
five keys (the settings route on d@e and on the five arm computers, and the
store on the paper) and confirm none is stored. The old protocol says no
computer ever held an arm [read: old Status line]; that is a statement, not
a reading.

Class: **red** (it removes a writer of tape records and changes what the
digest readers filter). Two seats. Rollback: revert. It must be on main
before 2026-11-25, when the walk-by date in each entry fails main
[read: registry.yaml, `grandfathered_off_until` of each entry]. It does not
block E1-M.

## 7. The first experiment, E1-M: the model screen on the research desk

### 7.1 The record

| Field | Value |
|---|---|
| Experiment | E1-M |
| Question | Is the many-search tail (and the read count, and the read loop) the model's behaviour or the task's and the surface's? |
| Computer | `e1-m`, a fresh test computer made today; nothing else runs on it |
| Arms (per run, by overlay) | **S**: `devin/swe-2-flash` (the paper's research model; the paper's role names the default pool, whose lead it is [read: catalog.go:516-523]), twice (S1, S2); **L**: `chatgpt/gpt-6-luna`; **D**: DeepSeek through Synthetic (the catalog's Synthetic DeepSeek entry) |
| Effort | low for all, as the paper's policy |
| Objectives | 40, fixed before the first counted run (7.3) |
| Runs | 40 x 4 = 160, plus up to 20 for the read-loop probe if the second trace pass names its objective |
| Order | For each objective, the four arm runs back to back in an order drawn from a seed (the SHA-256 of the objective file), one run at a time on the computer |
| Start | today, after the smoke step passes and the claims are entered |
| End | 2026-11-23T23:59Z (the overlays expire at 2026-11-24T00:00Z) |
| Ledger | C145, C146, C147 (7.6), entered before the first counted run |

### 7.2 The smoke step (does not count)

Before any claim is entered and any counted run is made, one submission per
arm on one objective that is not in the set. It must show, from the traces:

1. the eval route accepts the key and starts a Texture run that casts to a
   research run on `e1-m` [assumed until seen];
2. the research run's calls are served by the overlay's model and only that
   model [assumed];
3. the research run's first message has the same shape as the paper's (the
   v17 research prompt [read: internal/promptstore/defaults/research.yaml:1],
   the corpus brief block if it is deployed, the same verbs) [assumed];
4. `choir trace export` on `e1-m` returns the cells, the egress and the
   search starts [assumed];
5. the computer reports one release for all four runs.

If 1 fails, E1-M moves to d@e with the same design (one computer, arms by
run). If 2 or 3 fails, the route is not a faithful replay; build the research
eval route of section 7.9 item 5 first and start a day later.

### 7.3 The objectives

A follow-up research request on a published story is a real research flow
in the product ("research more" [read: internal/store/story_research_request.go:16-37])
and it has no leak: the story is given, so finding it proves nothing.

Rule, applied once at a time T0 written in the objective file:

- the 40 newest stories on the paper's public front at T0;
- each objective is the story's headline, its first 1,500 characters and its
  public URL, with the instruction "Write a follow-up. Find reporting,
  primary documents and outlets this story does not cite yet, and read them."
  (the wording of the product's own request, shortened);
- each objective is labelled before any run: **thin** if the story cites two
  distinct hosts or fewer, **thick** if three or more. The search-volume
  record's hard kind ("a local event with thin coverage, a non-English
  locale") is what thin is meant to catch;
- the file (objectives, labels, T0, the seed) is committed with its SHA-256
  before the first counted run.

Only public news subjects are used; no other computer's queries are copied.

### 7.4 What a run records, and the outcome line

From each run's trace and the gateway lines for its run id, counts only:

- web searches started (all verbs), and by verb (`SearchAll`, plain);
- searches per subject (the share of a run's queries that contain one of
  its two commonest terms, as in the trace record);
- pages returned, new to the run, and read; the largest number of reads of
  one capture; host calls per cell; per-cell bound refusals;
- **new-host reads**: pages read whose host the story does not cite (the
  quality guard; the host fetched them, so the model cannot write the count);
- whether the run finished with a Report; model calls and input tokens;
- validity: finished or ended, served by one model, trace exported, release
  unchanged within the objective's block.

The cell programs come with the bundle, so every loop and every burst of
searches in E1-M can be read as a program or as a sequence of cells, which
is what the paper's traces could not show.

`scripts/e1 read` prints one line per readout, for example:

```
E1-M 2026-11-21T14:00Z valid 88/160 | searches median S1 4 S2 4 L 3 D 5 | >=10: S .18 L .10 D .12 | dS-other +0.31 [+0.05,+0.58] | S1-S2 sd 0.62 | new-host reads S 3 L 4 D 3 | loops 0 | releases 2 | verdict pending (fixed n)
```

### 7.5 The analysis, fixed now

- **Primary (C145).** Per objective, d = mean over S1, S2 of ln(1 + searches)
  minus mean over L, D. The mean of d over objectives with all four runs
  valid, and a 95% bootstrap interval over objectives (10,000 resamples,
  seed fixed in the script).
- **Noise (C146).** σw from the S1 and S2 pairs (the sd of their difference
  over √2); σb, the between-objective sd of the S mean.
- **Reported beside them, not tested:** tail share per arm; the same d for
  thin and thick objectives separately; reads per run and the read share per
  arm; new-host reads per arm; loops and bound refusals per arm; model calls
  and tokens per arm; invalid runs per arm and why.
- **Reading the result.**
  - d > 0 with the interval above 0: the tail is (at least partly) the
    paper's model. Do not change the harness for it; the lever is the model
    choice, or a model-specific instruction tested as a prompt arm.
  - d near 0 and the tail present in all arms: the task or the surface
    causes it. If the tail sits in thin objectives for every model, the lever
    is what a thin result says to the model (E1-H arm H-a or H-b).
  - d near 0 and no tail in any arm: the tail does not reproduce on a
    replayed objective (see risk A2), or it is gone since the 18 November
    prompt changes; either way, no harness change for it.
  - σw close to σb: one model sampled twice differs as much as two
    objectives do. Then every future prompt or harness comparison needs at
    least two runs an objective an arm, and single-run before-and-after
    comparisons on the paper (like C140) cannot be read.

### 7.6 The forecasts, entered before the first counted run

Each is one test, so none is a conjunction. Kind: `experiment`. Settle by
2026-11-24T12:00Z. Evidence: the verdict line of `scripts/e1 read --verdict`
and the trace bundles on `e1-m`.

- **C145, p = 0.45.** On `e1-m`, over the objectives with all four runs
  valid (at least 32), the mean paired difference in ln(1 + web searches
  started) between `swe-2-flash` (mean of its two runs) and the mean of
  `gpt-6-luna` and DeepSeek is above 0 and its 95% bootstrap interval
  excludes 0. Void if fewer than 32 objectives have four valid runs.
  (Why 0.45: the trace record put "model behaviour" at 0.65, but that
  meant "the model's choices under this surface", not "other models choose
  differently"; rewording a thin search is common to models.)
- **C146, p = 0.50.** The within-objective sd of ln(1 + searches) for
  `swe-2-flash`, from its two runs, is less than half the between-objective
  sd of its mean. Void as C145.
- **C147, p = 0.60.** At least 144 of the 160 planned runs are valid (7.4)
  by 2026-11-23T23:59Z. (The lead's 0.3 to 0.6 claims have come true more
  often than priced [read: ledger scorecard]; this one has three untested
  dependencies, the route, the overlay inheritance and three providers.)

### 7.7 Cost

- **Runs:** 160 (plus up to 20).
- **Model calls:** a research run made 18.5 calls on 2026-11-09 and used
  about 0.41M input tokens (33.07M over 80 runs) [read:
  docs/evidence/token-efficiency-2026-11-09/research.md:21, :88]; the
  numbers today may differ. So about 3,000 research calls and 66M input
  tokens, plus the conductor's and Texture's calls for each submission (not
  measured; assumed 5 to 10 a submission, 800 to 1,600 more).
- **Money:** Devin is free until 2026-12-04 [read: catalog.go, comment at the
  default pool's first member]; ChatGPT and Synthetic are subscriptions
  [assumed: no marginal charge]. Bedrock and River are not used; if
  Synthetic refuses, the D arm pauses and River is not used without the
  owner (a grant that spends the owner's money is an owner act).
- **Web searches:** about 7.5 a run, so about 1,200, almost all on Keenable.
  About 17,000 Keenable credits remained on 2026-11-18 against the paper's
  several thousand a day [read: search-volume record, Problem]. 1,200 is
  about a third of a day of the paper's searching. The owner's R8 ("wait a
  day") was about buying or capping, not about this; it is named as D-E4.
- **Time:** at three to seven minutes a run (the four named runs took 3.7 to
  6.9 minutes of searching), one run at a time is 8 to 19 hours, inside the
  four-day window with room for pauses.

### 7.8 E1-P, the prompt arm, beside it

Startable the same day with no code: on `e1-p`, the research prompt
override set to the v13 text (2026-11-09, before the four 18 November
changes, v14 to v17 [read: research.yaml comments]), the model pinned to
`swe-2-flash` by the same kind of overlay, run on the same 40 objectives
once, alongside E1-M's S1 runs on `e1-m`. It asks whether the 18 November
prompt changes changed searching with the model and verbs fixed (C140 read
it before and after on the paper, which mixes the prompt with the day). It
starts only after E1-M's first ten objectives read valid, and it shares
E1-M's stops. Its own claim is entered when it starts. It costs 40 runs and
probably more searches than an E1-M arm. Decision D-E5.

### 7.9 What must be built, smallest first

1. **green**: this protocol; the three ledger claims (entered by whoever
   starts the run, before the first counted run); the objective file
   committed with its hash.
2. **yellow**: `scripts/e1` with `plan` (applies 7.3 at T0 and writes the
   file), `run` (the driver: overlays, submissions in seeded order, waits,
   trace export, pauses on the stops), `read` (rows, statistics, outcome
   line, verdict) and `end` (deletes overlays, resets the prompt, reads both
   back). A developer-side script, so it ships without CI; it gets a fixture
   test in `scripts/test-status-scripts.sh` style before its first real run.
3. **yellow**: the E1 section and its three flags in `scripts/status-hourly`.
4. **operations**: create `e1-m` (and `e1-p`), install the overlays, the
   smoke step. Nothing to land.
5. **red, by 2026-11-24**: the deletion of section 6. Two seats. Independent
   of E1-M.
6. **Only if E1-M or the trace pass calls for move 3** (section 8):
   **orange** `POST /api/evals/research`, which starts a research desk run on
   a test computer from a fixed first message (with the same overlay pinning
   as the Texture eval route), so a surface arm replays the paper's exact
   first message, not Texture's restatement of it; and the run-scoped surface
   variant. One seat each, two if the reviewer classes the variant red
   because it changes what a run records.
7. **yellow, any time**: stamp the effective role-prompt hash (default or
   override) on each run's records, so a prompt arm is read from the run.

Items 1 to 4 start E1-M today. Item 5 is the one landing with a deadline.

## 8. E1-H, the harness arms, and how the second trace pass slots in

A second trace pass is reading the paper's cell programs on the drill
computer now. Its result decides which harness arm, if any, is built, and
E1-H is written once it and E1-M's first readout are in. The candidates,
each one change with the paper's model fixed, from the trace record's list
[read: recurring-agent-failures record, last section]:

- **H-a** the cell result after a search shows the top ten titles with hosts
  and held marks, not only "12 of 40 pages new";
- **H-b** only `SearchAll` exists (plain `SearchStart` absent);
- **H-c** `ReadCapture` returns `block_count` and "you have read blocks a to
  b of n", and a whole-page read up to a cap exists;
- **H-d** the per-cell bound refusal names the verb and the capture that
  made the calls.

Each arm is a run-scoped surface variant: carried in the run's metadata from
the research eval route only, refused after its end date by the code, and
stamped on every cell event. Outcome for H-a and H-b: searches per run and
new-host reads; for H-c and H-d: reads of one capture and bound refusals.

What the trace pass would change:

| If the cell programs show | Then |
|---|---|
| The many-search runs wrote **one cell that loops over a list of wordings** (a program) | The surface can bound a program's shape. H-b first, and E1-M says whether other models write the same loop. |
| They wrote **many cells, each a new wording after reading results** (reasoning) | The lever is what a thin result tells the model: H-a first, analysed on thin objectives. If E1-M shows it in one model only, no harness arm: it is a model choice. |
| The read loop **advanced `fromBlock` and nearly finished a huge page** | Not a model failure: the verb has no whole-page read. Fix it as a defect (H-c becomes the fix, not an arm), no experiment. |
| The read loop **reset `fromBlock` each cell, or stepped by one block** | A model mistake met by a silent surface. Replay its objective (10 on S, 5 on L, 5 on D) inside E1-M's window; if it recurs on S only, it is the model's, and H-c and H-d are tested as arms, not shipped as fixes. |
| Runs **never call `Summary` or `Get`** before choosing what to read | H-a is the arm for the read share. |
| Runs **do call them and still read 7 to 25** | The 5% is what the stories need; drop it as a failure. |
| Every `candidates_shown=0` management run had the page in its opening | Pure instrument fix (section 1); nothing for E1. |

## 9. Risks, and how to refute this design in a day

The three assumptions it leans on hardest:

- **A1. A run started through the Texture eval route on a fresh test
  computer is the paper's research run in all that matters** (same prompt,
  verbs, corpus view, a research run that actually starts). Refutation, day
  0: the smoke step's five checks, and the S runs of the first ten objectives
  set beside the paper's numbers (median 4 searches, reads 7 to 25). If the
  test computer searches far less (for instance because it holds no corpus
  of its own, or the corpus brief differs), the replay is not the paper's
  condition, and the research eval route (7.9 item 6) moves ahead of E1-M.
- **A2. The tail reproduces on replayed objectives.** If none of the first
  20 S runs reaches 10 searches (against 0.19 on the paper; the chance of 0
  of 20 at 0.19 is about 1.5%), either the objectives (follow-ups of
  published stories) are not the condition that makes the tail, or the
  tail is gone since v17. Refutation, day 1: read the first block. Answer:
  add 20 objectives built from the paper's newest public captures that no
  published story cites yet, labelled thin and thick the same way, and say
  in the readout which set produced the tail.
- **A3. The pinned model serves every call, and the experiment does not
  compete with the paper.** The overlay may not reach the research run
  (inheritance not exercised for a follow-up), a provider may fall back, and
  Devin's single session carries both the paper and the S arm. Refutation,
  day 0 and 1: per-run model provenance on the first ten runs (any foreign
  call makes the run invalid), and the paper's Devin refusals in
  `status-hourly` before and during the first block. If Devin refuses the
  paper's calls, the S arm moves to quiet hours (section 5).

Other risks, named:

- **Deploys in the window.** The harness will change during four days.
  Arms are interleaved inside each objective's block, so a deploy touches all
  arms of all blocks but one; the reader stamps each run's release and drops
  a block that straddles a release change. Results are reported per release
  if the release changed more than twice.
- **The corpus moves.** Later objectives see a larger corpus than earlier
  ones; the paired design cancels this within a block.
- **Texture's cast adds noise.** Texture restates the objective to research
  differently each time. This is equal across arms and is part of σw; the
  research eval route removes it if it turns out large.
- **The quality guard is weak.** New-host reads say the run looked beyond
  the story, not that it found something good. E1-M's decision is a
  classification (model or not), for which behaviour counts are enough. A
  change meant to improve outcomes (E1-H) needs a stronger measure; there the
  owner's blind pairwise verdict on 12 pairs per arm is offered, never
  waited for, and a known-item objective set (a question whose answer is in a
  named capture) is the alternative.

## 10. Decisions taken by default

- **D-E1 (D2 of the night file): which computers hold an arm.** Default: one
  fresh test computer `e1-m` holds every model arm, by run; `e1-p` holds the
  prompt arm; d@e is the fallback; the paper, the owner's computer, real
  users' computers and the five old arm computers hold nothing.
- **D-E2: the three models.** Default: `swe-2-flash`, `gpt-6-luna`,
  Synthetic DeepSeek, all with no marginal charge. Claude Haiku on Bedrock is
  the fourth if the owner wants a paid model in the screen.
- **D-E3: no registry entry for E1-M and E1-P.** Default: none, because they
  gate no code; the record is section 7.1 and the end is the overlay's
  expiry. Alternative: a `life: experiment` entry naming the eval route with
  `expires: 2026-11-24`.
- **D-E4: spend about 1,200 Keenable searches.** Default: yes, with the stop
  on `quota_limited` and on the paper's own failures. Alternative: start
  with 20 objectives (600 searches; the halving is still detectable, the
  36% difference is not).
- **D-E5: E1-P beside E1-M.** Default: yes, after E1-M's first ten
  objectives read valid.
- **D-E6: delete the old arms and their code.** Default: yes (section 6);
  the conservative alternative, keeping the code and deleting only the
  settings, leaves code nothing can reach.

## 11. What changed from the 4 November protocol, and why

- **The question.** It was whether showing a desk its own calibration record
  improves its stated probabilities (management's restatement and four ways
  to deliver engineering's digest). It is now whether a recurring failure is
  the model's or the harness's, and whether a change improves an outcome the
  changer cannot write. The learning engine's reading of 341 claims showed
  the old question's ceiling was small (about 0.016 Brier), and the owner's
  2026-11-19 question is the new one.
- **The desk.** From management and engineering's forecasts to the research
  desk's behaviour, where the recurring failures are.
- **The unit.** From a Cast or an engineer thread on five differently pinned
  computers to one run on one objective, paired across arms on one computer.
  The old design confounded model with computer and said so (F7).
- **The arms.** From five computer settings, off everywhere, to a model
  overlay per run that expires by itself and a prompt override on a test
  computer. The settings and their code are deleted.
- **The precondition.** From a 12-hour window with no change, which never
  came, to interleaving within each objective and stamping the release.
- **The reader.** From the owner reading `GET /api/calibration/claims` by
  hand to a script whose verdict is computed by a rule fixed now, shown by the
  hourly status until the ledger holds the settlement.
- **Kept.** "Every record says what it actually got" (old F6): the arm and
  the model are read from the run. Fixed n and no early stop for success.
  Inconclusive reported as inconclusive.
