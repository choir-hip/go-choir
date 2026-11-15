# Choir calibration seed: a desk's own scored record in place of the deleted prior-run seed (2026-11-01)

> **Archived 2026-11-14.** Superseded by [the four-stroke learning engine](../../learning-engine.md),
> which is the current home of this mechanism and its vocabulary (forecast,
> outcome, lesson, trust). Kept as the record of how the mechanism was reasoned
> out; where the two disagree, the learning engine wins.

Not executable. Kept so the direction is not lost.

The owner gave this direction on 2026-11-01, while deleting the desk prior-run seed
(`docs/problems/desk-runs-seeded-from-prior-run-2026-10-31.md`):

> "the whole point of the PSA system is that we can seed models with some
> kind of a calibration seed ... not just a system prompt of saying, do this,
> don't do that, but like scores ... So many models totally overestimate the
> time that it'll take them to do work. So rather than saying to the model in
> a prompt, have shorter estimates on how long it takes you to do work, you
> want to have a prompt that says, you estimated it would take 3 weeks to do
> something that you did in 3 hours. But then show, like, not just a table,
> but like a lot of statistics ... we need to do this for different models as
> well ... So the seed that we have now is a problem and we should delete it
> ... We shouldn't try to reuse that system. We should delete it ... how we're
> going to collect and process all this data from self-development, with
> pre-commitment records for a bunch of different facets of it. Like ...
> before the model runs tests, it's making a pre-commitment ... after the
> test, pass or fail, that pre-commitment gets scored. So we need to really
> think about this holistically."

```yaml
definition_version: throughline/1
readiness: intent
readiness_authority: owner, 2026-11-01 (quoted above); roadmap stage 3 (docs/roadmap-2026-10-31.md)
start:
  observed_at: 2026-11-01
  artifact: >-
    The prior-run seed is being deleted, and nothing replaces it. Precommitment
    records exist (docs/archive/learning/precommitment-records.md, the canonical mechanism and
    theory). There is no per-model, per-facet statistics pipeline over
    self-development, and no calibration seed.
finish:
  deliver: >-
    A desk starts each run with its system prompt, its REPL, and a calibration
    seed. The seed is feedback, not instructions or examples: the statistics of
    this desk's (and this model's) own scored precommitments. For example,
    "you estimated three weeks, you took three hours", shown as distributions
    across many records, per facet, with a held-out set.
  facets_to_collect_first:
    - duration estimates vs measured wall clock (plan, cell, round)
    - test outcome predictions before each test run, scored on pass/fail
    - rounds to an accepted candidate; review findings predicted vs found
    - cost/token estimates vs actual
  per_model: yes. The pool mixes synthetic DeepSeek and gpt-6-luna, and the Claude
    delegate is Sonnet. Calibration is kept separately for each model.
  prerequisite: >-
    Many self-development rounds with precommitments on these facets, so the
    statistics exist. The data comes before the seed.
value:
  better_means: >-
    Minimize each desk's miscalibration on its own predictions (Brier score and
    duration log-ratio, per facet and per model), measured on held-out records.
  goodharting_would_be: >-
    Prompt text that tells the model to estimate shorter or be confident. That
    is an instruction dressed as a statistic. Also: a seed built from too few
    records to mean anything.
homotopy:
  realism_axis: >-
    duration and test facets on b@c self-development -> every engineering
    facet -> every desk -> Autopaper desks.
boundaries:
  exclusions:
    - reusing the deleted prior-run seed's code or its compaction-entry channel
now:
  status: intent
  next_action: >-
    While self-development runs, note which facets the desks already precommit
    on, and what the traces could score. Design after the data exists.
receipts: []
```
