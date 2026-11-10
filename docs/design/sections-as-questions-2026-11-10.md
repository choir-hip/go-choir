# Sections as questions (lane B, 2026-11-10)

Mutation class: orange (runtime behavior of the host scorer and the front read
path). Rollback: revert the lane B commits; a scores file written by the new
scorer is read by the old corpusd (it ignores the new fields and keeps the
legacy `section`), and the old scorer rescores sections on its next run.

Inputs: handoff 4.B; step 0 contracts 3.6, D10 and ruling 8.1;
`docs/evidence/jev-measurement-2026-11-10.md` (call site 11);
`docs/evidence/autopaper-source-audit-2026-11-10/README.md` finding 3.

## Problem

What the scorer does today (`internal/frontimportance/scorer.go`):

- One Jev call per story with three questions: `importance` (choice 1..10),
  `promo` (yes/no) and `section` (a single choice among four), built in
  `scoreStory` (the section question and its choices at `sectionChoices`,
  line 340 before this change).
- The section question says "Pick exactly one" and "Everything else,
  including politics, conflict and society outside the United States, is
  world", and the `world` criterion repeats "anything not in another
  section".
- A row is kept while `old.Section != ""`, its revision and its criteria
  version are current, so a section can never be added without a full rescore
  of everything (the criteria version bump is the only lever).
- corpusd checks the stored section against a second hardcoded list
  (`internal/platform/publication_front_importance.go:41`), and the frontend
  against a third (`frontend/src/lib/autopaper.ts` `AUTOPAPER_SECTIONS`).

Why the sections are lopsided (audit, 7 days: world 1,159 of 2,108 = 55%,
67% in the last 24 hours; us_politics 303, tech_ai 270, business 175; none
201, all on 11-07 and 11-08):

1. The question set is too small and the fallback is a catch-all. A forced
   single choice among four, with "everything else is world" written into the
   instruction, sends every culture, sport, science, health and climate story
   to World, along with all non-U.S. politics. The paper's source mix
   (Guardian, Al Jazeera, BBC lead the cites) is mostly international, so
   World is also the true plurality; but 55 to 67% is the catch-all's
   signature.
2. It is not a threshold or tie rule. There is no threshold: the model's one
   choice is taken as is, and an answer outside the four is dropped (the
   story then has no section). No tie occurs because a single choice question
   has one answer.
3. The 201 "none" stories are the unscored backlog of 11-07 and 11-08 (the
   audit's hypothesis: scorer backlog; the serial 80-call cap), which lane A
   removed as a cause by running the scorer concurrently. Rows also lose their
   section when the 14-day retention drops them (handoff residual).

## Design

- A section is a yes/no question asked of Jev about a story. Jev answers with
  a distribution (`answers{name:{choice, probabilities}}`); the stored value is
  P(yes).
- A section list is a document (`internal/sectionlist`): `version`, `id`, and
  sections each with `id`, `label`, `question`, `threshold`. The house list is
  `internal/sectionlist/house.json`, embedded; editing it and deploying is how
  the house list changes. It is data, not code: no section name appears in Go.
- A story belongs to every section whose P(yes) reaches the threshold
  (multi-membership). A story with no section above threshold is placed in its
  highest-probability section and marked `section_fallback` in the data. There
  is no "no section" bucket: the reader sees the story in a section page, and
  always on the front page.
- Thresholds live in the list document and are applied at read time over the
  stored probabilities (by corpusd at file load), so a threshold change
  re-sorts without asking Jev again.
- Per D10 (ruling 8.1): the house list is scored on the host and cached in
  the host's scores file, which corpusd reads; only the host writes it. Every
  other list runs on the reader's own computer (wave 2, not built tonight).
  The host door has no per-scope limiter (lane A's limiter is keyed by
  computer and scope on the guest door only), so no scope is added here.

The 10 house sections: World, U.S., Politics, Business, Technology, Science,
Health, Climate, Culture, Sports, all at threshold 0.5 to start (no
hand-labelled set in pass 1; calibrate from the `jev-calls.jsonl` record,
which carries each answer's distribution and the threshold in force). The
World question is "outside the United States", not "anything else".
`us_politics` is split into U.S. and Politics; `tech_ai` keeps its id with
label Technology.

## Cache

Key: (story version canonical id, SHA-256 of the normalized question text,
judge version).

- Story version canonical id: `publication_id@revision`, the id the scorer
  already uses as the decision id. The row stores `revision`; a row whose
  revision differs from the story's latest is not a cache hit for any
  question.
- Question: hex SHA-256 over the question text with whitespace collapsed
  (`sectionlist.Hash`). The section id and label are not in the key.
- Judge version: `sectionJudgeVersion` in the scorer, stored as `judge` in
  the row. Bump it when the framing around the questions changes.
- Storage: the scores file row carries `section_p: {hash: P(yes)}`. The file
  carries the published list (`section_list`) so corpusd maps hashes to ids
  and thresholds.

What invalidates: a new revision (all answers); an edited question (that
question: its hash is new); a judge version bump (all). Not invalidating: a
renamed section (same text), a label edit, a threshold edit, reordering.
Removed questions' answers are pruned from rows scored in the window.

## Cost physics

Wire: one call carries one story and up to 12 questions (the host cap
`System1MaxQuestions`; the measurement saw 782 sampled receipts with exactly
12 and about 500 with more). Importance and promo take two, so a list of up
to 10 sections fits one call: `MaxSections = 10` and a longer list is refused.
Measured call latency is about 110 ms p50 and 210 ms p90 for 3 questions;
lane A runs the scorer 12 at a time.

- Backfill of the first 100 stories: 100 calls (importance and promo are
  kept at their current criteria; 10 section questions), about 1 to 2 s at
  concurrency 12. Bounded by `MaxCalls` (200) and the 3-minute budget per
  run; stories are asked in front-list order, so the visible page is first
  and the archive fills over later runs.
- A new story: 1 call, 12 questions.
- A new revision: 1 call, 12 questions.
- Adding one section: 1 question per story, over the visible stories first
  (one call each, 1 question), then older stories on the later runs as the
  budget allows; the rest of the cached answers are untouched.
- Editing a threshold: 0 calls.

## Failures and their tests

| Failure | Guard | Test |
|---|---|---|
| A question matches everything | the run logs any section answered yes (>= threshold) by 95% or more of 20 or more scored stories; the log is the signal for the list's editor | `TestScorerFlagsBroadSection` |
| A section is empty | not an error; the front read path lists the section from the published list with no stories (the frontend shows its empty state) | `TestListFrontStoriesInSectionMulti` (empty case), frontend empty-state test is existing |
| A story is in no section | placed in its top section, `section_fallback` set | `TestPlace` (sectionlist), `TestAttachFrontImportanceSections` |
| Threshold change re-sorts without asking | placement is computed at read time from stored probabilities | `TestPlace`, `TestThresholdChangeAsksNothing` |
| Cache poisoning | only the host job writes the scores file; probabilities outside [0,1] or not numbers are refused; corpusd drops rows whose hash is not in the list | `TestScorerRefusesBadProbability`, `TestAttachFrontImportanceSections` |
| A renamed question loses its cache | key is the question text, not id or label | `TestRenamedSectionKeepsCache` |
| An edited question keeps a stale answer | new hash is asked, old pruned | `TestNewQuestionAsksOnlyThatQuestion` |
| Jev unavailable | failure keeps the previous row and the previous file; the file still carries the list, so the front page renders from the last scores | `TestScorerJevDownKeepsLastScores` |
| Reorder flapping between runs | answers are cached; the stored probabilities change only when asked; placement is deterministic (list order breaks ties) | `TestScorerSecondRunIsStable` |

## Not built tonight

- The reader-side half (D10 option C, wave 2): a reader's own list scored on
  their computer under the per-computer `judgments:sections` scope.
- A reader-editable list Texture. The list is a versioned file and a
  deploy; the format is the one a Texture would hold.
- `internal/maild/digest_render.go:26` still has its own section list (not
  lane B's file).
- Retention: a story older than the 14-day retention loses its row. The
  probabilities are kept as long as the row is.
