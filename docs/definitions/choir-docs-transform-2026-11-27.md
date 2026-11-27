---
definition_version: 7
readiness: executable
authority: 'the owner, 2026-11-27: "the first mission is the docs transform. im nearly going to bed, so i want to get that running so i can wakeup to a new sdlc"; "docs transform precedes into sdlc improvements, as that is the next code thing, then exact cutover finishing and android launching. we need to be ready to think about how to merge code in batches appropriately, to use github actions optimally, to be resource efficient with our infrastructure"; "we should be aggressive. looking to land as much as possible always".'
review:
  reviewer: none (the owner's statement is the authority)
  frozen_ref: none
  verdict: none
  evidence_ref: none
start:
  captured_at: 2026-11-27T08:56Z
  source:
    canonical_ref: origin/main 0a213add2
    deploy_identity: staging on 774e87d2f (x-choir-build-commit of https://choir.news)
  what_is_true_now:
  - 'Docs (measured 2026-11-26): 1,641 Markdown files and 4.4 million words. In docs/design, docs/definitions and the top level, a skim on 2026-11-27 sorted 281 files: 51 governing, 126 built, 30 superseded, 27 undecided, 47 unsorted. Twelve designs that carry an owner ruling are linked from none of CLAUDE.md, the system map, current-architecture.md, the roadmap or ACTIVE.md. The per-file list is not in the repository.'
  - 'Landing (measured 2026-11-27 over 21 runs that deployed): push to the CI gate 6.0 minutes at the median, the deploy 6.8, push to staging serving 12.9 (8.1 to 24.9). scripts/land records no timings; three failed runs show 10 to 22 minutes before the push. scripts/land-queue makes lane N+1 wait until lane N is green and served, which the deploy does not need: its job lets a newer commit supersede a queued one.'
  - 'In 14 days 120 commits touched docs/changes.md and 255 touched docs/calibration/claude-ledger.md, of 1,613. scripts/land joins conflicts in those two files by hand (union_docs).'
  - 'The Vocabulary Gates job is about fifteen structural lints. On 2026-11-26 it failed ten runs on main because five switch-registry entries passed an expiry date; no change caused it. 49 other entries are past expiry and only warn.'
  - 'cmd/doccheck is 1,918 lines; its default mode reports and always exits 0. docs/doc-authority-manifest.yaml lists 86 documents by declaration; the other 1,558 are inferred.'
  - 'Readers: an outside harness loads one file (CLAUDE.md or AGENTS.md) and searches for the rest; a review seat reads what it is handed; a Choir desk queries through functions. None can be made to read a set.'
  worktrees:
  - path: .lane/trees/big-picture
    status: clean
    class: goal_candidate
    owner: the lead of this session
    touch: goal_owned
    recovery: lane rm big-picture after it lands
  - path: every other lane under .lane/trees
    status: unknown
    class: other_agent_wip
    owner: other agents
    touch: forbidden
    recovery: leave in place
finish:
  deliver: An agent that has read only the one file its harness loads can find the governing document for any subject with one query, and a dozen approved lanes land in one verified push with their timings recorded.
  artifact: on main, the batched landing scripts with timing output; change-log and ledger entries as one file each; a first line of kind and standing on every governing and superseded document in scope; a query that lists documents by kind and standing; a shorter CLAUDE.md that points at the query
  acceptance:
  - action: land two or more approved lanes with the batched queue in one push, and read the CI run for that commit
    proves: a batch is verified once and pushed once, and CI is green on it at the first attempt
    evidence_class: a CI run on main, with the landing's own timing lines
  - action: read the timing lines of five landings
    proves: where a landing's time goes before the push is known step by step
    evidence_class: recorded timings
  - action: land two lanes in one batch that each add a change-log entry and a ledger claim
    proves: neither file conflicts, with no hand join
    evidence_class: the batch's cherry-pick log
  - action: move a system date past a switch-registry expiry in the lint's own selftest
    proves: an expiry warns ahead and does not fail a run by the calendar alone
    evidence_class: selftest
  - action: run the query for each of the twelve forgotten designs' subjects from a shell
    proves: each governing design is returned first for its subject, and no superseded design is returned as governing
    evidence_class: command output, checked against the triage list by a seat that did not write the query
  - action: give a fresh agent the task of naming the governing design for five subjects, with nothing but the repository
    proves: the filter works for a reader that was not told about it
    evidence_class: a subagent's answers, scored against the owner's rulings
  rollback: each station is its own commit and reverts alone; the earlier scripts stay in history; no document is deleted by this mission
  landing:
    required: true
    environment: not_applicable
    required_receipts: [pushed_commit, ci]
value:
  better_means: 'two gaps close: the time an approved lane waits to reach main, and the chance that an agent works from a document that an owner ruling has replaced'
  goodharting_would_be: a batch that lands by skipping a suite; timings recorded and never read; a header on every file that no outcome settled, so the old manifest again under another name; fewer files with the same contradictions; a shorter CLAUDE.md that drops a rule nobody then follows
homotopy:
  realism_axis: 'the filter: from a first line on about eighty documents and a shell query, to every document, to the same index served to desks through their cells. The lander: from one batch pushed from the Mac, to the same script run on rented compute with the Mac as the exercised backup.'
boundaries:
  mutation_class: yellow (scripts, gates, lints) and green (docs); no runtime behaviour
  authority_sources:
  - the owner's words of 2026-11-26 and 27 in docs/inbox.md
  - CLAUDE.md
  - docs/big-picture.md, a draft, for the measurements it records
  must_preserve:
  - 'every check a landing runs today still runs on every landed commit: a batch changes how many times, not whether'
  - no document is deleted or moved by this mission; standing is marked, and what loses standing stays where it is
  - a lane in flight on the old change-log and ledger files still lands
  - the docs-only filters that skip CI and deploy are not weakened
  - review seats stay from different providers, two for security-relevant changes
  - no secret, token or owner address in any file or output
  excluded:
  - restricting who may push to main (the owner's act)
  - changing the deploy job or what a deploy does
  - rewriting doctrine, the ontology or the learning engine
  - the move to Exact and every app change
  protected_surfaces: []
stations:
- id: T0
  name: the plan and the campaign on main
  scope: docs/big-picture.md, docs/inbox.md, the campaign file, this file, the three registries
  class: green
  review: none
  depends_on: []
  carrier: 9d75196f4
  status: proved
- id: T1
  name: landings are timed
  scope: scripts/land and scripts/land-queue print and append the seconds each step took
  class: yellow
  review: none
  depends_on: []
  carrier: 6d85de5a7
  status: proved
- id: T2
  name: an expiry warns and does not fail
  scope: the switch-registry lint warns from fourteen days before an expiry and fails only when a change adds or extends a switch past one; its selftest covers the calendar case
  class: yellow
  review: one seat
  depends_on: []
  carrier: 7c2ea0572
  status: landed
- id: T3
  name: one file per entry
  scope: a change-log entry and a ledger claim are each one new file in a directory, with the two long files kept as history and still joined for lanes in flight; CLAUDE.md's two rules point at the directories
  class: yellow
  review: one seat
  depends_on: []
  carrier: e4cca635f
  status: landed
- id: T4
  name: the batched queue
  scope: scripts/land-queue stacks every approved lane, verifies the stack once and pushes once; a failing stack is halved to find the lane that broke it, which is set aside and named; the wait for staging between lanes is removed
  class: yellow
  review: one seat
  depends_on: [T1]
  carrier: lane batch-queue (80252a1a8), sent back once by seat 1, with seat 2
  status: working
- id: T5
  name: heavy suites run once
  scope: from T1's timings, decide whether the build machine or CI is the gate for the race and scale suites, and remove the other run
  class: yellow
  review: one seat
  depends_on: [T1, T4]
  carrier: none yet
  status: pending
- id: T6
  name: the first line
  scope: 'a one-line header on each document in docs/design, docs/definitions and the top level that the triage sorted as governing or superseded: its kind, its standing, and the ruling or outcome that settled it, with the date. The triage list is checked in. Nothing is moved.'
  class: green
  review: one seat reads a sample against the rulings
  depends_on: [T0]
  carrier: 7ff273046 and 2dd849e6d
  status: proved
- id: T7
  name: the query
  scope: scripts/docs lists documents by kind, standing and subject from their first lines; a small check fails a new design or definition with no first line; scripts/doccheck leaves scripts/gates
  class: yellow
  review: one seat
  depends_on: [T6]
  carrier: b80278aaa
  status: landed
- id: T8
  name: the loaded file
  scope: CLAUDE.md shortened to the rules that must never be missed and the instruction to query before relying on a design; the rest moves to documents the query returns
  class: green, and the owner's contract
  review: the owner
  depends_on: [T7]
  carrier: none yet
  status: pending
- id: T9
  name: one plan
  scope: the campaign file takes the place of the standing metamission, ACTIVE.md and the mission graph as the list of missions; the roadmap is marked as replaced by the plan
  class: green, and the owner's sequence
  review: the owner
  depends_on: [T0]
  carrier: none yet
  status: pending
- id: T10
  name: what exists, in one place
  scope: 'the facts of the 126 built designs that current-architecture.md lacks are added to it; the three vault designs, the dozen storage designs and the four SwiftUI-era app designs are each given a governing member or marked undecided'
  class: green
  review: one seat
  depends_on: [T6]
  carrier: none yet
  status: pending
- id: T11
  name: checks named for what they protect
  scope: the Vocabulary Gates job and its lints are renamed after the rule each enforces, with one line each on the decision or incident behind it
  class: yellow
  review: none
  depends_on: [T2]
  carrier: none yet
  status: pending
decisions_for_the_owner:
- id: D1
  question: May pushes to main be restricted to the landing identity, with a bypass for the owner?
  why: It makes the lander the only path, which is what keeps main green. It changes who can push and is the owner's to set.
  options: restrict now; restrict after the batched queue has landed a week of work; leave main open
  default: main stays open; the mission does not touch the setting
- id: D2
  question: Is the shortened CLAUDE.md accepted?
  why: It is the one file every outside agent loads, and the owner's contract.
  options: accept the draft; edit it; keep the present file
  default: the draft waits in a lane and the present file stays; T3 makes only the two edits its change needs
- id: D3
  question: Does the campaign file replace the standing metamission, ACTIVE.md and the mission graph (T9)?
  why: Other sessions run /goal on the standing metamission today.
  options: replace now; replace when the missions under it next change state; keep both
  default: keep both; the campaign lists the standing metamission as one station
- id: D4
  question: Which of the 49 switch-registry entries past their expiry are retired, deleted or extended?
  why: Each is a setting that was meant to be temporary.
  options: a list with a proposed verdict per entry, for one reading
  default: none is changed; T2 only stops a date from failing a run
owner_rulings:
- on: 2026-11-27
  decision: scope
  said: "The problem record is somewhat useful if verbose, and probably could be improved upon rather than scrapped. While the key issue is I think that we have too many, and conflicting, theory, architecture, and planning docs, and should consolidate those. And also streamline the sdlc significantly."
- on: 2026-11-27
  decision: method
  said: "thinking functionally, it should be fine that we accumulate docs, if we have the right way to filter them"
  read_as: the mission marks standing and builds the filter; it does not delete or move documents.
- on: 2026-11-27
  decision: readers
  said: "'always read set' is implying more control over the agent and the harness than we can safely. need to recalibrate to actually existing agents, external agents like claude code, and the yaegi rlm repl design of choir desks"
  read_as: the filter lives in the one loaded file, in names and first lines, in a query, and in checks at landing.
- on: 2026-11-27
  decision: landing
  said: "I don't want pull requests and to be more dependent on GitHub."
  read_as: lanes stay the unit of work; GitHub Actions is used as compute, with our own machine as the exercised backup.
now:
  status: working
  slice: T4, with the second review seat; T5 and T11 next
  source_ref: origin/main 7c2ea0572
  deploy_identity: staging on 774e87d2f
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: DT-C1
    calibration_claim: C257
    claim: batching is mostly a change to the queue script, so a batch of two or more approved lanes lands in one push with CI green at the first attempt within two days
    test: the CI run on the batch's pushed commit, and the queue's own log naming the lanes in it
    edge: resource
    delta_o: T1's timing lines, which show whether the time before the push or the wait between lanes was the larger cost
    scope_if_supported: the queue script and the lanes it landed; nothing about the deploy or about CI on lane branches
    status: active
    evidence_refs: []
  decision:
    what: mark standing and build a filter; do not delete or move documents
    kind: operational
    status: settled
    evidence_ref: the owner's words of 2026-11-27 in docs/inbox.md
    owner_ratification_ref: not_applicable
  belief:
    believed_state: 'T0, T1, T2, T3, T6 and T7 are on main. A landing of a scripts-only change takes 250 to 590 seconds before CI, nearly all of it scripts/gates (247 to 489 s for one run, by how busy the Mac is); the build is 7 to 38 s.'
    main_uncertainty: whether the batched queue is safe on a real gh and a real build machine; it has run only against fakes
    next_observation: seat 2's verdict on T4, then the first real batch of two lanes, which settles C257
  blocker_or_risk: 'a docs push to main while a landing runs makes the lander run the gates twice (seen on T3: 474 s); three lanes in a row conflicted on one spot in scripts/gates. Both go to T5 and T11.'
  next_action: read seat 2 on T4 and land it or send it back; then T5 (gates are the cost, not suites; a docs-only move of main must not re-run them) and T11 (one file per gate check); follow-ups are the doccheck CI job and the three readers of the old ledger
receipts:
- id: T0
  boundary: define
  identity: 9d75196f4
  proof_refs: ['scripts/check-goal-files: 94 files scanned, all goal files pass (run in the lane before the push)']
  rollback_ref: revert 9d75196f4
  disposition: the plan, the inbox, the campaign and this file are on main; docs only, so no CI gate or deploy ran by design
  landing:
    source_commit: 9d75196f4
    ci_ref: not_applicable
    deploy_ref: not_applicable
    environment_identity: not_applicable
    deployed_acceptance: not_applicable
---

# The docs transform

Two things, one mission, because the second is the next code the first
unblocks (owner, 2026-11-27).

**Documents.** They may accumulate. What changes is that each says, in its
first line, what kind it is and what standing it has, and that standing is
settled by an owner ruling or an outcome its author cannot write. A query
returns documents by standing. The readers that exist cannot be made to read
a set, so the filter sits where they look: the one file a harness loads,
the first line a search shows, the query, and the checks at landing.

The four kinds, by how a statement changes:

| Kind | Changes when |
| --- | --- |
| what exists | code lands |
| what is decided | the owner rules |
| what is wanted | any time; it adds |
| what happened | never; it is appended |

**Landing.** Approved lanes are stacked, verified once and pushed once. A
date never turns main red. Two lanes never conflict on a log file. Every
step is timed.

What this mission does not do: restrict main, change the deploy, or delete a
document. The recalibration that reads a sample of what the filter hides is
its own recurring mission in the campaign.
