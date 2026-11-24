---
definition_version: 7
readiness: intent
authority: 'the owner, 2026-11-23: "We can''t lose logs"; "we want to have some measure of perplexity of our corpus versus the world"; "We should do experiments before we decide on the design"; on records and measurement as its own track: "Yes, that matches as a third track".'
review:
  reviewer: none (an intention recorded from the owner's words; reviewed before it becomes executable)
  frozen_ref: none
  verdict: none
start:
  captured_at: 2026-11-23T22:02Z
  source:
    canonical_ref: origin/main bcea7eefd
    deploy_identity: staging and the paper on a2ba96767
  what_is_true_now:
  - The paper computer's own decision lines (story choice, research cast, search summaries) exist only in a console file that keeps five files of 1 MiB, under two hours on 2026-11-23. Nothing ships them elsewhere.
  - The run ledger is durable and does not hold the prompt a model saw, what woke the run, or its search queries.
  - No record joins a capture's id to the choice made for it, so the density measurement (ledger C203) could not be run.
  - 'A voice turn logs counts only: not the ranked list given to the model and not the queue changes it wrote.'
  - There is no measure of the paper against the world. The proposed one (the gap between the world's attention over happenings and the front page's, with its cost beside it) is in docs/design/ideal-form-draft-2026-11-23.md and has never been computed.
  - 'unknown: whether vector groups of arrivals are happenings; whether happenings grow more slowly than sources; whether a cheap detector finds a turn in a running story.'
finish:
  deliver: Every decision the paper makes about a capture and every voice turn can be read a week later, and the paper's gap against the world is a number computed continuously with its cost beside it, which the experiments that decide the engine's design are scored by.
  artifact: 'on main: a durable decision record (one row per capture and stage), turn records, the measure computed over a replayed week and then live, and the results of the replay experiments in docs/evidence'
  acceptance:
  - action: 'ask of any capture from five days ago: what was decided about it, by whom, from which candidates'
    proves: the decision record is durable and joins a capture to its choice
    evidence_class: deployed proof, rerunnable
  - action: compute the measure over the replayed week and read the hour of 2026-11-23T19:00Z
    proves: the measure shows the failure that started this (the Huang happening high in the world, low on our page)
    evidence_class: replay, rerunnable; a ledger claim entered first
  - action: run the replay experiments E1 to E5 and settle each ledger claim
    proves: each design assumption is supported or dropped by an outcome the lead cannot write
    evidence_class: ledger settlements
  rollback: each station is a commit that reverts
  landing:
    required: true
    environment: staging and the owner's phone where the station touches the app
    required_receipts:
    - pushed_commit
    - ci
    - deploy
    - deployed_acceptance
value:
  better_means: the share of the system's decisions that can be read back with their inputs rises to all of them, and the design's open questions are settled by measurement and not by argument
  goodharting_would_be: a log nobody can join to anything; a measure tuned until it shows what we hoped; an experiment whose pass line was chosen after the data; a dashboard
homotopy:
  realism_axis: from a replay over data already held (no spending, no reader sees a change), to shadow tables beside the live mechanisms, to live records the root reads
boundaries:
  mutation_class: orange for records the runtime writes; red where the tape or run ledger gains fields; yellow for replay scripts
  authority_sources:
  - the owner's words in docs/design/design-session-2026-11-23.md; CLAUDE.md
  owner_rulings:
  - decision: logs are never lost
    said: Wtf. The log from before the deploy is gone? ... We can't lose logs. We're storing so much
    'on': '2026-11-23'
  - decision: the measure is perplexity of the corpus against the world, by maximal coverage, with importance matching the world's
    said: anything that happened more than 1 minute ago that is consequential should be in Choir, and the importance of that story in the world should be roughly the importance of it in Choir
    'on': '2026-11-23'
  - decision: no source is withheld to make a test set
    said: the point is not that we should be not covering something so then we can test it
    'on': '2026-11-23'
  - decision: 'reading less at the same coverage is an improvement; the lead''s reading: the measure is reported with its cost and the frontier between them is what improves'
    said: maybe reading less with the same coverage should improve the measure??
    'on': '2026-11-23'
  - decision: experiments precede the design
    said: We should do experiments before we decide on the design
    'on': '2026-11-23'
  - decision: the paper's root knows its own metrics and may call research to report on the paper against the news
    said: It's important that root texture desk for paper computer knows its own metrics. It can / should be able to call research for this metareporting
    'on': '2026-11-23'
  - decision: the world's attention starts with equal weight per independent origin; the weights are learned over time by the learning engine
    said: 4) yes, start with equal weight. we can learn weights over time with the learning engine
    'on': '2026-11-24'
  - decision: 'groups and origin counts: host code as a named graph custodian, run as a host service'
    said: 2) yes, can run this as a host service
    'on': '2026-11-24'
  must_preserve:
  - records hold ids, choices, counts and probabilities, never a user's private content beyond what the tape already holds
  - 'one custodian per data kind: the record''s custodian is named before it is built'
  - no experiment changes what a reader or listener sees
  - every experiment enters its claim in the calibration ledger with its pass line before it runs
  excluded:
  - the engine's redesign itself (M2)
  - dashboards
stations:
- id: R0
  name: The decision record
  scope: one durable row per capture and stage (capture id, stage, decision, who decided, candidates shown, probability, time), written first for today's management choices; the story-choice line gains its capture ids
  class: orange; red if it is written to the tape
  review: one seat; two if red
  depends_on: []
  status: pending
- id: R1
  name: What a run saw
  scope: the run ledger gains the prompt a model saw, what woke the run and its search queries; guest log lines reach the host journal before the console rotates
  class: red
  review: two seats
  depends_on: []
  status: pending
- id: R2
  name: Turn records
  scope: a voice turn records the ranked list it was given and the queue changes it wrote; the phone records what un-paused playback and why
  class: orange
  review: one seat
  depends_on: []
  status: pending
- id: E1
  name: Does the measure see the failure
  scope: the measure, version 0, over the replayed week; the claim that the hour of 2026-11-23T19:00Z shows the Huang happening in the world's top three and our page holding it low
  class: yellow
  review: none
  depends_on:
  - R0
  status: pending
- id: E2
  name: Do happenings grow more slowly than sources
  scope: the replayed week with 10, 25, 50 and 100 per cent of sources; count happenings reported by two or more independent origins; the claim is an exponent under 0.85
  class: yellow
  review: none
  depends_on: []
  status: pending
- id: E3
  name: Are vector groups happenings
  scope: a hand read of 100 groups at two and three origins, and whether the Huang group forms and bursts in replay
  class: yellow
  review: none
  depends_on: []
  status: pending
- id: E4
  name: The turn detector against real turns
  scope: '20 hand-collected cases where a story''s point moved, today''s among them, against a detector with no count in it: does a rival main point explain recent arrivals better than the story''s own'
  class: yellow
  review: none
  depends_on:
  - E3
  status: pending
- id: E5
  name: Batching against the staged funnel
  scope: four arms on one replayed day (the staged funnel; batches of about 25 with no retrieval; batches with each item's nearest stories; what management did), scored by what the world did next, a blind reader and the turn set, priced at the paid rate, kill lines set first
  class: yellow; the model calls it makes are counted and reported
  review: one seat on the harness
  depends_on:
  - R0
  - E3
  status: pending
- id: R3
  name: The measure, live
  scope: the measure and its cost as a host fold the root reads, with the daily outside check research runs; density step 2a (ledger C203) settles here
  class: orange
  review: one seat
  depends_on:
  - E1
  status: pending
decisions_for_the_owner:
- id: Q1
  question: Who is custodian of the decision record, of groups and of origin counts?
  why: the hot path is host code and no desk; the contract says one custodian per data kind
  options:
  - the host holds them as derived folds and the Texture desk holds the subject index
  - a desk holds all of it
  default: the host holds derived folds; nothing canonical is added until the owner rules
now:
  status: working
  as_of: 2026-11-23T22:02Z
  slice: nothing building; the first three experiments read only captures and may start on the owner's word
  source_ref: origin/main bcea7eefd
  deploy_identity: staging on a2ba96767
  candidate: none
  believed_state: the holes are known from today's reads; nothing is built
  main_uncertainty: whether the measure, computed on our own sources, tracks what the owner means by the world
  next_observation: the owner's word on starting E1 to E3
  blocker: none
  next_action: on the owner's word, R0 and E1 to E3; then write this file up to executable
operating_notes:
- 'This track goes first: the voice and engine tracks are settled by its records.'
receipts:
- docs/design/design-session-2026-11-23.md
- docs/design/ideal-form-draft-2026-11-23.md
- docs/learning-engine.md
---

# Records and measurement

Track T3. The paper cannot say why it made a choice two hours ago, and nothing measures it against the news. This mission makes both true and runs the experiments the engine's design waits on. The four-stroke learning engine needs outcomes that are written down; this is where they are written.
