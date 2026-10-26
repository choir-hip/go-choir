---
definition_version: 5

# T1a — self-develop the desktop apps to a design bar, one app per request,
# through b@c's loop with the Claude delegate. Station of
# definitions/choir-selfdev-readiness-metamission-2026-10-24.md (after SR5 and
# PD3). Owner 2026-10-25 01:4x: "I want to be more aggressive with
# self-development": after the Reader, the other apps on the list (Files with
# an Open with menu, Slides, Calendar, Podcasts). Owner 2026-10-26 00:0x: the
# rebuilt apps' design is "horrid" (not the top priority); set the Claude
# Sonnet delegate up on b@c as well. Owner 2026-10-26 00:5x: "Synthetic
# DeepSeek should be good at design ... If you give it the right prompt, the
# right demands and constrains, and it knows not to duplicate existing bad
# patterns, it can do a great job"; keep working with DeepSeek and its traces;
# "we really need to be judicious using screen real estate ... it has to be
# content everywhere"; propagation to every computer should be an internal
# mechanism, not an external agent pushing main (station SR9, design).
readiness: drafted

review:
  reviewer: none yet (one Devin seat; the file authorizes orange frontend changes)
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-26T00:45:00Z
  source:
    canonical_ref: main a03d5266a (plus docs)
    deploy_identity: staging a03d5266a
  worktrees:
    - path: /Users/wiz/Dev/go-choir
      status: dirty (M go.sum, ?? scratchpad/), the owner's; never staged
      class: unrelated_wip
      owner: owner
      touch: never
      recovery: none needed
  observed:
    - >-
      Rebuilt by self-development and on main: the Reader package
      (frontend/src/packages/reader, 942fb2415 and 7fbe45512) and Slides
      (frontend/src/lib/SlidesApp.svelte, 3321f633b). Not rebuilt: Files
      (frontend/src/lib/FileBrowser.svelte), Calendar
      (frontend/src/lib/CalendarApp.svelte), Podcasts
      (frontend/src/lib/PodcastApp.svelte); the 2026-10-24 five-app
      candidates v6 and v7 were rejected (stale base).
    - >-
      The design bar (docs/design/app-design-bar-2026-10-26.md, lane
      design-bar 87a336a09, review 255 running) rendered both rebuilt apps
      in the real shell: the Reader's toolbar collapses to 0px, app.css
      greys all document text, Slides gives the slide 42% of the window and
      breaks at phone width.
    - >-
      The Claude delegate is available on b@c (observability 00:12:54,
      claude-sonnet-5-5, until 2026-11-03); the Reader redesign round
      (selfdev-aae30fb7) ran in a claude-capable capsule but its trace shows
      no delegate call: Synthetic DeepSeek wrote it, frozen in 12 minutes, and its
      bundle reads as a binding mismatch after the refresh that followed
      (docs/problems/awaiting-candidate-unreadable-after-refresh-2026-10-26.md).

finish:
  deliver: >-
    The owner opens the Reader, Slides, Files, Calendar and Podcasts on any
    computer and finds each one well designed to one bar: theme colors in
    every preset, the content first, one toolbar, no repeated facts, a
    working phone layout; and Files opens any file with any app that can
    open it from a right-click or long-press menu.
  artifact: >-
    Five self-developed candidates, one app per request, each built on b@c
    by engineering through the Claude delegate, approved by FP0 against the
    design bar's twelve checks from screenshots rendered in the real shell
    at desktop and phone width, promoted to main with scripts/selfdev-promote
    and a tier 1 review, deployed, and upstreamed on b@c. The design bar
    lands as a reviewed doc.
  acceptance:
    - action: >-
        For each app, FP0 renders the candidate in the real shell (the
        design bar's capture harness on the candidate's patch) in London
        Salmon, Futuristic Noir and Carbon Fiber Kintsugi at 1100x750 and
        390x844, and writes pass or fail for each of the twelve checks in
        docs/evidence/t1a-design-judgments-2026-10-26.md. One fail rejects;
        a rejection is iterated with texture revise on the carried tree.
      proves: the design bar is held, not asserted
      evidence_class: rendered screenshots plus code read
    - action: >-
        Files: right-click and long-press on a .pdf offer Open with the
        Reader, PDF (classic) and any other app routing pdf; choosing one
        opens it there; Shift+F10 opens the menu from the keyboard.
      proves: the Open with menu the owner asked for
      evidence_class: deployed product path (b@c), screenshot
    - action: >-
        After each promotion deploys, b@c's source projection reports
        upstreamed for that operation, and the app renders on staging.
      proves: the loop lands for every computer
      evidence_class: deployed identity
  rollback: >-
    Per app: git revert of its promotion commit and deploy; on b@c before
    promotion, reject or restore. The design bar is a doc.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the design-bar checks each app fails in the real shell
    (baseline: the Reader and Slides fail most of the twelve; Files,
    Calendar and Podcasts unmeasured) and the apps left unrebuilt (baseline
    three), while preserving each app's contract with the desktop (registry
    ids, events, routes, recents) and the loop's authority (only engineering
    changes b@c; FP0 approves; main changes only by reviewed promotion).
  goodharting_would_be: >-
    Approving on the engineer's own report or a signed-out capture of a
    page that does not show the app; passing checks by deleting features;
    restyling one preset and leaving the others broken; a candidate built by
    FP0's hand edits.

homotopy:
  realism_axis: >-
    one app restyled against the bar on b@c (the Reader) -> the shared
    content opt-out in app.css lands with it -> each app judged in the real
    shell at three presets and two widths -> promoted and deployed for
    every computer -> the bar's screenshot set produced inside the capsule
    by a tracked harness, so verify judges design and FP0 confirms.

boundaries:
  mutation_class: orange (frontend app behavior and styling, self-developed)
  review_tier: >-
    tier 1 (Devin) per promotion; the design bar doc, a design a build
    follows, one Devin seat (review 255).
  authority_sources:
    - owner 2026-10-25 and 2026-10-26 statements (above)
    - owner 2026-10-25 01:4x "you approve the reader" (FP0 approves on b@c with the owner's standing authority)
    - owner 2026-10-26 "We should set it up on b@c.com as well" (the delegate)
  must_preserve:
    - only engineering changes b@c's platform tree; FP0 never hand-edits a candidate's patch
    - each app's registry id, props, events and payloads, routes and recents
    - PdfApp and EpubApp stay as the classic apps
    - one app per request
    - content first: the window title bar and the prompt bar are the only standing bars; an app adds at most one toolbar, and no empty bands (owner 2026-10-26)
    - a trace review after every round (docs/evidence/selfdev-trace-reviews-2026-10-25.md)
  excluded:
    - the desktop redesign and its three presets (T1c)
    - Mail and other apps beyond the five
  protected_surfaces: [frontend/src/app.css shared overrides, frontend/src/lib/Desktop.svelte media routing, self-development approval]

now:
  status: working
  slice: >-
    The Reader redesign: candidate selfdev-aae30fb7 awaits approval on b@c;
    its embedded capture (/read.html, 1280x800) shows one toolbar, dark body
    text and the default theme. Its patch and visual routes answer 409
    after a refresh; a read-only investigation is running. Then Slides,
    Files, Calendar, Podcasts (requests in _runs/design/).
  source_ref: a03d5266a
  deploy_identity: staging a03d5266a
  candidate: {id: selfdev-aae30fb75aa73afab4e6897823facfdc, state: awaiting_approval, ref: b@c, base: a03d5266a's predecessor (built before the refresh), digest: ed0ee483, scope: [frontend/src/packages/reader, frontend/src/app.css, frontend/src/reader.js, frontend/reader-chrome.css, frontend/read.html, frontend/src/lib/theme.ts]}
  conjecture:
    id: T1a-1
    claim: >-
      With a design bar in the request (constraints, the bad patterns not to
      copy, content first) and in the approval, a self-developed app passes
      the bar in one or two rounds, whether Synthetic DeepSeek or the Claude
      delegate writes the code; the earlier "horrid" designs came from
      briefs without a bar, not from the model.
    test: >-
      rounds per app to a candidate that passes all twelve checks, compared
      between DeepSeek rounds and delegate rounds, read from the traces
    edge: missing_oracle
    delta_o: >-
      FP0 renders each candidate in the real shell at three presets and
      two widths instead of trusting a signed-out capture
    scope_if_supported: frontend apps on b@c with the delegate available
    status: active
    evidence_refs: []
  decision:
    what: >-
      FP0 judges design from screenshots rendered locally in the real shell
      against the candidate's patch, because the capsule's capture sees only
      signed-out pages
    kind: operational
    status: settled
    evidence_ref: docs/design/app-design-bar-2026-10-26.md, "How the screenshots were made"
    owner_ratification_ref: not_applicable
  belief:
    believed_state: >-
      The delegate is available on b@c but was not called; DeepSeek, given
      the design bar, built the Reader redesign in 12 minutes and it looks
      right in its one capture; its diff is unreadable until the binding
      mismatch is understood.
    main_uncertainty: >-
      Whether a refresh while a candidate awaits approval breaks the
      candidate for good, or only its read routes.
    next_observation: the investigation's cause and whether approval would also fail
  blocker_or_risk: >-
    The binding mismatch above; until it is fixed, avoid deploys while a
    candidate awaits approval on b@c.
  next_action: >-
    Diagnose the binding mismatch; if the candidate cannot be read, repair
    by texture revise (a fresh freeze on the carried tree) and judge that.

receipts: []
---

# T1a — the apps, to a design bar

The Reader proved the self-development loop on b@c. It also proved that a
loop without a design bar ships designs the owner calls horrid. This station
self-develops the five apps, one per request, with the bar in each request
and in FP0's approval, and with engineering handing the code to Claude
Sonnet through the delegate.

Order: the Reader redesign, then Slides, then Files (with Open with), then
Calendar and Podcasts. Requests: `_runs/design/reader-request.txt`,
`slides-request.txt`, `files-request.txt`; Calendar and Podcasts follow the
same shape with the 2026-10-24 v7 brief's feature lists.
