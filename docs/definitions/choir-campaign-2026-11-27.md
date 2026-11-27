---
definition_version: 7
readiness: executable
authority: 'the owner, 2026-11-27: "get our big picture into a campaign file, with the campaign tracking missions, many of which are required to reach app release milestones. the first mission is the docs transform"; "from this point forward, i dont see the campaign ever stopping. i may switch orchestrator or interface agents but the campaign will continue. theres always something one can do safely. you just have to calibrate your risk tolerance to the specific conditions you operate under".'
review:
  reviewer: none (the owner's statement is the authority; each station file carries its own)
  frozen_ref: none
  verdict: none
  evidence_ref: none
metamission:
  stations:
  - id: K1
    path: docs/definitions/choir-docs-transform-2026-11-27.md
    readiness: executable
    status: working
    depends_on: []
    note: 'the docs transform and the landing process: this campaign and the plan landed, landings timed and batched, one file per change-log and ledger entry, a document filter that fits the readers that exist. The prerequisite of version 1.1. First on the owner''s word, 2026-11-27.'
  - id: K2
    path: docs/definitions/choir-standing-metamission-2026-11-22.md
    readiness: executable
    status: working
    depends_on: []
    note: 'the missions already live on 2026-11-27, kept in their own list until K1 folds that list into this file: the move to Exact (M1), the desks and engine (M2), storage (M3), Android (M4), records (M5), the voice pipeline (M6), shipping (M7), operations (M8), the desktop (M9). The owner''s order after K1: finish the move to Exact, then Android.'
  - id: K3
    path: docs/definitions/choir-finish-the-move-to-exact-2026-11-27.md
    readiness: executable
    status: working
    depends_on: []
    note: 'finishing the move of the phone app to Exact on a current Exact, and a turn that completes signed out on the phone. A prerequisite of version 1.1. Authored 2026-11-27 from a read-only count; nothing of it is built. It asks the owner (its D1) whether it takes over the phone stations of M1 under K2.'
start:
  captured_at: 2026-11-27T08:56Z
  source:
    canonical_ref: origin/main 0a213add2
    deploy_identity: staging on 323df6f3b
  what_is_true_now:
  - Autoradio 1 is on the App Store (approved 2026-11-25) and has no users yet.
  - The plan this campaign follows is docs/big-picture.md, a draft the owner has not yet cut. Its version table is an order of emphasis and a map of what needs what, not a calendar.
  - Nine missions are live under the standing metamission of 2026-11-22. About sixty lanes are open in the primary checkout; none belongs to this file.
  - 'Measured on 2026-11-27: a landing takes 12.9 minutes at the median from push to staging serving it, the wait between queued lanes is a convention the deploy does not need, and the time before the push is recorded nowhere.'
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
    owner: other agents and the lead of other sessions
    touch: forbidden
    recovery: leave in place
finish:
  deliver: The campaign does not finish. It is the standing list of missions, and it is in good order when every mission in it is working, observing, complete or superseded with named evidence, and the next mission's goal file is written before the current one closes.
  artifact: this file's station list and each station's own goal file
  acceptance:
  - action: read each station file's now.status
    proves: no mission is listed here that nobody is working, and none is called complete without its own evidence
    evidence_class: goal file state
  - action: read docs/big-picture.md beside this file's station list
    proves: every version's needs in the plan are either a station here or named in the table below as not yet a mission
    evidence_class: human inspection
  rollback: revert the commit that added this file; the standing metamission of 2026-11-22 is unchanged by it and remains runnable
  landing:
    required: false
    environment: not_applicable
    required_receipts: []
value:
  better_means: the share of the campaign's work that runs without the owner's prompting goes up, and the owner's time goes to questions only the owner can answer, approvals of irreversible acts, and sessions of design
  goodharting_would_be: a long station list nobody is working; missions marked working to look busy; questions put to the owner that had a safe default; this file growing station detail; a version counted as shipped that the App Store has not accepted
homotopy:
  realism_axis: two stations now (the docs transform, and the list of missions already live); each later mission is one more station entry and its own file
boundaries:
  mutation_class: green for this file; each mission states its own
  authority_sources:
  - the owner's words in docs/inbox.md, which is where this session's statements are kept verbatim
  - CLAUDE.md
  - each station file's authority
  must_preserve:
  - every must_preserve of each station file
  - one Mac native build at a time through the lock
  - 'anything irreversible waits for the owner: a destructive act in production, deleting a person''s data, restricting who may push to main, spending money'
  excluded:
  - 'station detail: a station''s now card is its own authority'
  - a calendar. The versions are an order of emphasis (owner, 2026-11-27).
  protected_surfaces: []
decisions_for_the_owner:
- id: D1
  question: Does docs/big-picture.md replace docs/roadmap-2026-10-31.md as the plan?
  why: Until it does, two documents state the sequence, and the roadmap is the one CLAUDE.md names.
  options: promote big-picture.md as it stands; promote it after the owner's cut; keep the roadmap
  default: the roadmap stays the named sequence and big-picture.md stays a draft; the campaign follows the owner's statements of 2026-11-27 where the two differ, because they are newer and the owner's own
- id: D2
  question: Is the version table's reading of what each version needs correct, in particular that a price and a way to pay, the platform's secrets vault and backups in a third place are needed before 1.2?
  why: It decides which platform missions are authored next and how early.
  options: as drafted; move the vault and backups later; other
  default: as drafted, since being early on these costs less than being late
owner_rulings:
- on: 2026-11-27
  decision: order
  said: "docs transform precedes into sdlc improvements, as that is the next code thing, then exact cutover finishing and android launching."
- on: 2026-11-27
  decision: authority
  said: "we should be aggressive. looking to land as much as possible always."
  read_as: a mission lands what passes its checks and review without waiting for the owner; what is irreversible still waits, on the owner's earlier word in the same session ("approvals for irreversible decisions").
- on: 2026-11-27
  decision: role
  said: "my role becomes answering questions posed by the agents that cant be answered without me, approvals for irreversible decisions, and communicating with the system naturally as part of my expression and co-understanding with it."
now:
  status: working
  slice: K1 (the closing pass and T8 to T11) and K3 (E0)
  source_ref: origin/main 5844447d4
  deploy_identity: staging on 774e87d2f
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: K-C1
    calibration_claim: C257
    claim: see K1 (ledger C257); this file makes no claim of its own
    test: the station files' tests
    edge: independence
    delta_o: one landing at a time with its own proof before the next
    scope_if_supported: none of its own
    status: active
    evidence_refs: []
  decision:
    what: none
    kind: none
    status: none
    evidence_ref: none
    owner_ratification_ref: not_applicable
  belief:
    believed_state: 'K1''s landing and filter stations are proved (a batch of two lanes landed as one push; claim C257); its closing pass, the shortened loaded file and the one plan wait on the owner. K3 is authored and unbuilt. The missions under K2 continue in their own sessions.'
    main_uncertainty: whether the owner's cut of the plan changes the order of the platform work
    next_observation: the owner's reading of docs/big-picture.md
  blocker_or_risk: none
  next_action: 'run K3: the pin (E1) and the verdicts on the lanes in flight (E3), each with its own builder; apply K1''s closing pass once the owner has read docs/evidence/mission-closing-verdicts-2026-11-27.md'
receipts: []
---

# The campaign

The campaign is the standing list of missions. It does not end. An
orchestrating agent may change; this file and the station files carry the
state across.

Run it with `/goal docs/definitions/choir-campaign-2026-11-27.md`.

## What is not yet a mission

Each row is a need from the version table in
[big-picture.md](../big-picture.md) with no goal file yet. A row becomes a
station when its file is written. The order is the version that first needs
it.

| Needed by | Mission to author | Kind of work |
| --- | --- | --- |
| 1.1 | finish the move to Exact on the latest Exact, with a signed-out turn that completes on the phone | phone |
| 1.1 | the app made nice, deeper on-device models, the onboarding interview | phone |
| 1.1 | account deletion deployed | server and phone |
| 1.2 | takes and social: record on a phone, publish from the person's computer, screen, respond, permalinks | phone and server |
| 1.2 | a price and a way to pay | server, phone, App Store |
| 1.2 | the platform's secrets vault | host |
| 1.2 | backups in a third place | host |
| 1.3 | discovery: search, explore, podcast explore, the first recommender | phone and server |
| 1.3 | monitoring as a desk, with public figures | server |
| 1.3 | Autopaper's lesson and trust strokes: a kept model of the world that a settled claim adds to, and a per-source weight moved by outcomes that decides which sources enter the context (owner, 2026-11-27; start from docs/design/autopaper-learning-materiality-2026-11-04.md) | server |
| 1.3 | the owner's statements routed from docs/inbox.md into the document that governs each subject, and each linked from the campaign row it bears on | docs |
| 1.4 | service VMs; Node A as an independent host | host |
| 1.4 | the transcription and pictures services | host |
| 1.5 | the desktop app with computer use | desktop |
| 1.6 | the second-host rearchitecture: host record, writer lease, host key, state that moves | host |
| 1.7 | metering, one unit of account, credits | server |
| 1.8 | agentic red teaming | all |
| 1.9 | the people's credentials vault, an MCP client in the computer, driving a computer from outside | host and server |
| 1.10 | the learning engine in everything | all |
| 1.11 | recalibration: a sample of what the document filter hides, read against outcomes | docs |

The second-host design starts early, in host-side lanes, because it is the
longest item and sets the date of 1.6.
