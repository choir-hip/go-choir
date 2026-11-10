# Pre-registration: Autopaper materiality hypotheses AL-H1 to AL-H6 (2026-11-10)

Status: committed before any reliance-link data was read and before any
reliance-link code existed. Mutation class green (docs only). Source:
section 11 of
[autopaper-learning-materiality-2026-11-04.md](../design/autopaper-learning-materiality-2026-11-04.md)
and the owner's wave 3 ruling of 2026-11-09 in
[the overnight definition](../definitions/choir-autopaper-overnight-2026-11-09.md).
This file is the precommitment: the prediction and the refuting observation
are written here first, and the ledger settles them afterwards.

The commit date of this file is 2026-11-10. That date starts every clock below.

## Standing rules (apply to all six)

1. **Settlement horizon: 28 days from the commit date**, so settlement is
   evaluated on or after 2026-12-08. Evidence read before that date is a
   progress look, never a settlement. A look may not change anything below.
2. **Weights frozen at commit.** The reliance score is the one described in
   section 6 of the note, with these parameters, fixed now and not tunable
   after data arrives: a directed graph whose nodes are published stories and
   whose edges are the typed `relies_on` links; damping 0.85; iterate to an
   L1 change under 1e-9 or 100 iterations; every edge weight 1; dangling mass
   spread uniformly; edges flow from the relying (later) story to the
   relied-on (earlier) story, so a story's score is the weight others place on
   it. A story's score is read at the horizon, counting only edges recorded
   within 28 days after that story's publication. Any change to these numbers
   is a new registration with a new date, and the old one stays on the record.
3. **Nothing ranks by the link, and the score is never shown to a story-writing
   desk.** The `relies_on` link is data only. No front-page, search, related
   story or research ordering reads it, and no prompt, packet, tool result or
   overlay given to Texture, research, management or any other desk that
   writes or chooses stories contains the link or the score (note section 7,
   point 3; class C of the metric research). Computing the score for
   settlement is an offline reading by the owner's tooling. A check that finds
   a ranker or a prompt reading the link makes AL-H1, AL-H2 and AL-H6
   unsettleable: they settle `inconclusive` with that cause recorded.
4. **Counting rule.** A `relies_on` edge counts only where the citing story
   brings at least one independent source the cited story did not cite (note
   section 7, point 2). The first recording slice does not store that
   comparison; at settlement it is computed from the two stories' cited
   captures, and an edge that fails it is dropped from the score. Edges whose
   citing and cited stories came from the same desk run on the same day are
   reported separately and also dropped from the primary score (question 7 of
   the note, the self-validating case).
5. **Sample and power.** A hypothesis whose sample falls below its stated
   minimum settles as `inconclusive`, never as confirmed. An inconclusive
   result changes no design.
6. **Falsification is stated per hypothesis.** A hypothesis settles
   `confirmed`, `refuted` or `inconclusive`. There is no fourth state and no
   post hoc redefinition of the measure.

## AL-H1 Forward-citation materiality

- **Claim.** Precommitted relevance predicts the lagged reliance score better
  than a reader-click baseline does.
- **Measure.** For stories published in the 28 days after the commit date,
  rank them (a) by the precommitted relevance probability recorded at
  publication and (b) by reader clicks plus dwell counted in the first 7 days.
  Compute the Spearman rank correlation of each ranking with the story's
  reliance score at its horizon. Bootstrap the difference of the two
  correlations over stories, 2,000 resamples, 95 percent interval.
- **Threshold.** Confirmed if the relevance correlation exceeds the click
  correlation and the interval of the difference excludes zero. Minimum
  sample: 300 stories with a recorded relevance probability, and at least 20
  incoming `relies_on` edges counted in the whole sample.
- **Falsified by.** The click ranking correlating as well or better: the
  difference interval includes zero or lies below it, on an adequate sample.

## AL-H2 The anchor matters

- **Claim.** The reliance score with reader anchoring agrees better with an
  audit sample's judgment of importance than the unanchored graph score does.
- **Measure.** Draw 100 stories uniformly from the horizon set; an audit
  judge (a human or a model seat that has seen neither score) rates each for
  importance on a 5-point scale. Spearman of each score with the rating. The
  anchored score is the reliance score multiplied by (1 + the story's reader
  return rate in the first 7 days divided by the sample median), fixed now.
- **Threshold.** Confirmed if the anchored correlation exceeds the unanchored
  one by at least 0.05 and the paired bootstrap interval excludes zero.
  Minimum sample: 100 rated stories.
- **Falsified by.** The two agreeing equally well (difference under 0.05 or an
  interval including zero): the anchor is not earning its cost.

## AL-H3 Self-citation does not launder errors

- **Claim.** After self-citation, the audit-found unsupported-claim rate on
  stories that cite a prior story of the paper is not higher than on matched
  stories that do not, by more than one percentage point.
- **Measure.** An audit sample of stories that carry a `relies_on` link
  against matched stories without one (matched on desk, beat and publication
  day), each audited by the existing support judge plus a human spot check of
  20 percent. The measure is the rate of claims judged unsupported.
- **Threshold.** Confirmed if the cited-story rate minus the matched rate is
  at most 1.0 percentage point and the upper 95 percent bound is under 2.0.
  Minimum sample: 150 stories with the link and 150 matched.
- **Falsified by.** The cited-story rate higher by 2.0 percentage points or
  more (point estimate), on an adequate sample.

## AL-H4 Search recall

- **Claim.** After the scan cap is lifted and the search ranks by BM25 over the
  whole archive, the share of stories for which research finds at least one
  relevant earlier story rises.
- **Measure.** A labelled sample of 60 subjects, 30 older than the old cap's
  reach (outside the newest 2,000 stories at the time of the change) and 30
  newer, each with a human-labelled set of relevant earlier stories. Recall
  at 10 of the search results, old method (replayed from the previous commit
  on the same archive snapshot) versus new method. This one may be settled
  from a snapshot taken at the change, without waiting for the horizon. The
  live companion measure, read at the horizon, is the share of research runs
  whose packet cites an earlier story of the paper.
- **Threshold.** Confirmed if recall at 10 on the older 30 subjects rises by
  at least 0.20 absolute and does not fall on the newer 30. Minimum: all 60
  subjects labelled.
- **Falsified by.** Recall on subjects older than the old cap not improving
  (a rise under 0.05): the cap was not the limit.

## AL-H5 Preference over behaviour

- **Claim.** Where a reader's stated "irrelevant to me" and the same reader's
  later behaviour on similar stories disagree, the stated preference predicts
  the reader's later explicit ratings better.
- **Measure.** Over the window, for each reader with at least one stated
  preference and at least 5 later explicit ratings on similar stories, take
  the disagreeing cases. Accuracy of predicting the direction of the later
  rating from the stated preference versus from behaviour (clicks, dwell).
- **Threshold.** Confirmed if stated-preference accuracy exceeds behaviour
  accuracy with a 95 percent interval excluding zero. Minimum: 40 disagreeing
  cases. At Choir's reader count this is likely to fall short.
- **Falsified by.** Behaviour predicting as well or better. If reader signals
  are not collected by the horizon, this settles `inconclusive` with that
  cause.

## AL-H6 The quantilizer on materiality

- **Claim.** Ranking above the floors by a diversity-weighted draw from the
  top of the materiality ranking does not lower the later reliance score of
  the front page compared with argmax, and raises topical diversity.
- **Measure.** A live materiality ranking is forbidden by rule 3 before AL-H1
  settles, so this is counterfactual replay only: on the horizon set,
  simulate front pages by argmax and by the draw (top 20 percent,
  diversity-weighted, 1,000 draws per day) and compare the mean reliance score
  and topic entropy.
- **Threshold.** Confirmed if the draw's mean reliance score is within 5
  percent of argmax and its topic entropy is higher. Minimum: 28 simulated
  days.
- **Falsified by.** The draw lowering the mean reliance score by more than 10
  percent. A difference between 5 and 10 percent is inconclusive. The note
  says this one is underpowered, so an inconclusive result will not change the
  design.

## What would invalidate this registration

A change to the frozen parameters in rule 2 without a new dated registration;
a ranker or prompt that reads the link or the score (rule 3); recording of the
link by a path other than the one that publishes the citing story. Each is
recorded in the mission ledger with its cause, and the affected hypotheses
settle `inconclusive` with that cause named. An invalidation is not a pass.
