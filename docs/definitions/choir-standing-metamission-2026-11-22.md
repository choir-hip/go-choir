---
definition_version: 7
readiness: executable
authority: 'the owner, 2026-11-22: "we are still working on exact train arent we. we should of course continue that"; "whats the right goalstring for running both missions at once? maybe make a metamission which covers them both, and any 3rd or nth mission we add. so we can keep /goal running continuously".'
review:
  reviewer: none (the owner's statement is the authority; each station file carries its own)
  frozen_ref: none
  verdict: none
metamission:
  stations:
    - id: M1
      path: docs/definitions/choir-exact-cutover-and-ontology-2026-11-20.md
      readiness: executable
      status: working
      depends_on: []
      note: 'the phone: every page outside voice in shared Exact code, and the account boundary; and the search changes in flight (graph-first). Its ontology track (events and entities as separate kinds) is replaced by M2.'
    - id: M2
      path: docs/definitions/choir-first-design-2026-11-22.md
      readiness: executable
      status: working
      depends_on: []
      note: 'the first design: subjects and mentions, the hot path for captures, predictions that subscribe, the root Texture, the readers. Stations F0 to F6. No red station starts before the design panel has read the draft.'
start:
  captured_at: 2026-11-22T05:02Z
  source:
    canonical_ref: origin/main 9f10b7477 (docs); the last code commit is 92f068980
    deploy_identity: staging and the paper on 92f068980, read from the CI run's deploy job; the owner's iPhone on the build of 2026-11-20 at bfc1492c6
  what_is_true_now:
    - 'Two missions are live and each file holds its own state. This file holds only what they share: the order of landings, the shared resources and the list of missions.'
    - 'One landing and one deploy run at a time and a landing takes about two hours; one Mac native build runs at a time through the lock; Go builds on the build machines, several at once.'
  branches_in_flight:
    - 'see each station file; none belong to this file'
finish:
  deliver: 'Every mission listed here is complete or superseded, and the list is empty of working missions.'
  artifact: 'the station files, each settled by its own acceptance list'
  acceptance:
    - action: read each station file's now.status
      proves: every listed mission is complete or superseded with its own named evidence
      evidence_class: goal file state
  rollback: none of its own; each mission names its own
  landing:
    required: false
    environment: staging
    required_receipts: []
value:
  better_means: 'the landing slot is never idle while a reviewed candidate waits, and no mission stalls because another holds the session''s attention'
  goodharting_would_be: 'landings batched across missions to save a deploy; a mission marked working here that nobody has touched in a day; this file growing station detail'
homotopy:
  realism_axis: 'two missions now; a third is one more station entry and its own file'
boundaries:
  mutation_class: green for this file; each mission states its own
  authority_sources:
    - 'the owner''s words above; CLAUDE.md; each station file''s authority'
  owner_rulings:
    - '"autoradio is already a compelling product imo. now we are sprinting towards production grade, scalable"; "and above all, software quality. craft matters" (2026-11-22). Reading (the lead''s): when speed and quality pull apart, quality wins; a station is done when its replacement is proved and the old mechanism is deleted, not when the new one runs beside it.'
    - '"we are still working on exact train arent we. we should of course continue that" (2026-11-22). Reading: the phone cutover is not displaced by the redesign.'
  must_preserve:
    - 'one landing and one deploy at a time; one Mac native build at a time through the lock; every must_preserve of each station file'
  excluded:
    - 'station detail: a station''s now card is its own authority'
operating_notes:
  - 'Landing order: the first reviewed and proved candidate takes the slot. When two are ready: the Exact train, then graph-first, then the account boundary, then first-design stations in their order. A small candidate goes ahead of a large one that is not ready.'
  - 'Resources: the Mac build lock serves M1 (train proof, account drills, phone install); the build machines serve all Go work; read-only reads of the paper are free. At most about five builders at once, because the lead reads every report.'
  - 'Overlap: graph-first lands before any M2 station that changes how research or Texture opens a run.'
  - 'Adding a mission: write its goal file, add one station entry here, name it in docs/ACTIVE.md and docs/mission-graph.yaml. Run with /goal docs/definitions/choir-standing-metamission-2026-11-22.md.'
  - 'Each hourly tick reads both station files'' now cards and rewrites this one''s.'
now:
  status: working
  as_of: 2026-11-22T05:02Z
  slice: 'M1: the train''s proof, graph-first in rework after two send-backs, the account boundary in rework. M2: F0 and the read-only half of F1 building; the design panel not yet run.'
  source_ref: origin/main 9f10b7477
  deploy_identity: staging and the paper on 92f068980
  candidate: none frozen
  conjecture:
    claim: 'see M2 (ledger C190) and M1 (ledger C183, C188); this file makes no claim of its own'
    test: the station files' tests
    edge: independence
    edge_note: 'both missions change research and Texture prompts, so a result in one may be caused by the other''s landing'
    observer_upgrade: one landing at a time with its own deployed proof window before the next
    scope_if_supported: none of its own
    ledger_id: C190
    status: untested
  believed_state: 'nothing has landed since read-first; three candidates are in rework or proof; two redesign builders started at 05:00Z'
  main_uncertainty: 'which candidate reaches the landing slot first'
  next_observation: 'the train''s proof line; graph-first''s reworked commit; F1''s pair file'
  blocker: none
  next_action: 'review and land whichever candidate is ready first; run the design panel on M2''s draft'
receipts:
  - 'M1: docs/definitions/choir-exact-cutover-and-ontology-2026-11-20.md'
  - 'M2: docs/definitions/choir-first-design-2026-11-22.md'
---

# The standing metamission

One file to run with `/goal`. It lists the live missions and what they share:
the landing order and the machines. Each mission's state is in its own file.
A new mission is one more entry.
