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
    - id: M3
      path: docs/definitions/choir-storage-efficiency-2026-11-22.md
      readiness: executable
      status: working
      depends_on: []
      note: 'storage: no routine operation writes a second copy of a store, restore fits its budget, the store is measured by kind and cut. Stations S0 to S7. Added on the owner''s word after Node B''s disk refused deploys and wakes on 2026-11-22.'
    - id: M4
      path: docs/definitions/choir-autoradio-on-android-2026-11-22.md
      readiness: executable
      status: working
      depends_on: []
      note: 'Android: Autoradio running on an old Motorola from the shared codebase, and a measured table of which on-device models such a phone can run. Stations A0 to A5. Added on the owner''s word, 2026-11-22.'
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
  realism_axis: 'three missions now; another is one more station entry and its own file'
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
  - 'Landing order: the first reviewed and proved candidate takes the slot. When two are ready: the Exact train, then graph-first, then the account boundary, then first-design stations in their order. Storage stations S1 and S2 are small and go ahead of a large candidate that is not ready. A small candidate goes ahead of a large one that is not ready.'
  - 'Resources: the Mac build lock serves M1 (train proof, account drills, phone install); the build machines serve all Go work; read-only reads of the paper are free. At most about five builders at once, because the lead reads every report.'
  - 'Overlap: graph-first lands before any M2 station that changes how research or Texture opens a run.'
  - 'Adding a mission: write its goal file, add one station entry here, name it in docs/ACTIVE.md and docs/mission-graph.yaml. Run with /goal docs/definitions/choir-standing-metamission-2026-11-22.md.'
  - 'Each hourly tick reads both station files'' now cards and rewrites this one''s.'
now:
  status: working
  as_of: 2026-11-22T16:31Z
  slice: 'M1: the Exact train waits on a staging that can wake the drill computer; graph-first is reviewed and rebased, waiting for the landing slot; the account boundary is being rebuilt on Exact''s secret store. M2: F0 landed and deploying; offline slice rungs 1 and 2 running with spend granted. M3: S0, S1 and S2 building.'
  source_ref: origin/main c8d48a589
  deploy_identity: staging and the paper on c27a50d98; a deploy of main (run 45044023185) in flight
  candidate: 'graph-first (.lane/trees/gf-land, to be rebased again before landing)'
  conjecture:
    claim: 'see M1 (ledger C183, C191), M2 (C190, C192, C194) and M3 (C195); this file makes no claim of its own'
    test: the station files' tests
    edge: independence
    edge_note: 'M1 and M2 both change research and Texture prompts, and M3 changes the paper''s restarts and captures, so a result in one may be caused by another''s landing'
    observer_upgrade: one landing at a time with its own deployed proof window before the next
    scope_if_supported: none of its own
    ledger_id: C190
    status: untested
  believed_state: 'deployed today: the main-point judgment, read-first, the proxy fix. Landed and deploying: F0, a spend test fix. Nothing landed for ten hours because Node B''s disk refused deploys; it has room again (70 GiB at 16:28Z).'
  main_uncertainty: 'whether the deploy in flight restarts the paper cleanly and swaps in the staged compact copy'
  next_observation: 'the deploy''s result and Node B''s free space after it; then F0''s by-desk count (C194)'
  blocker: none
  next_action: 'after the deploy: land graph-first, then S1; prove the train against a staging that wakes d@e and land it; install on the owner''s phone'
receipts:
  - 'M1: docs/definitions/choir-exact-cutover-and-ontology-2026-11-20.md'
  - 'M2: docs/definitions/choir-first-design-2026-11-22.md'
  - 'M3: docs/definitions/choir-storage-efficiency-2026-11-22.md'
  - 'M4: docs/definitions/choir-autoradio-on-android-2026-11-22.md'
---

# The standing metamission

One file to run with `/goal`. It lists the live missions and what they share:
the landing order and the machines. Each mission's state is in its own file.
A new mission is one more entry.
