---
definition_version: 7
readiness: intent
authority: 'the owner, 2026-11-23: "we should also have an autoradio app inside choir.news... but that should be deferred til after android i think. not urgent. we need to figure out our web voice stack. the obv answer is cloud based voice but i dont like that for cost reasons... but maybe we can do listening only in autoradio on web choir.news, using a local model"; "exact web target, yes, but i mean an app inside our desktop".'
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
  - The choir.news desktop has no Autoradio app.
  - The radio's pages, queue and director are shared Exact code, and Exact has a web target the shared code's tests already run against.
  - 'unknown: whether Exact''s web build can mount inside one desktop window or must be framed; whether a local speech model speaks fast enough in a browser on an ordinary laptop.'
finish:
  deliver: A person at choir.news opens Autoradio as a desktop app and listens, with speech made in the browser.
  artifact: an Autoradio app in the desktop, listening only
  acceptance:
  - action: open the desktop app signed out and signed in and listen for ten minutes
    proves: the radio plays in the desktop from the same shared code, with no cloud voice
    evidence_class: browser proof and the owner's verdict
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
  better_means: the share of the radio that is one codebase across phone and desktop stays whole, and voice on the web costs nothing per listener
  goodharting_would_be: a second radio written for the web; a cloud voice behind a local-sounding name
homotopy:
  realism_axis: listening only first; asking by voice on the web is a later question
boundaries:
  mutation_class: orange
  authority_sources:
  - the owner's words in docs/design/design-session-2026-11-23.md; CLAUDE.md
  owner_rulings:
  - decision: Autoradio is an app inside the desktop, after Android
    said: we should also have an autoradio app inside choir.news... but that should be deferred til after android i think. not urgent
    'on': '2026-11-23'
  - decision: no cloud voice for cost; listening only with a local model is the first form to consider
    said: the obv answer is cloud based voice but i dont like that for cost reasons... but maybe we can do listening only in autoradio on web choir.news, using a local model
    'on': '2026-11-23'
  - decision: it is an app in the desktop, not a separate page
    said: exact web target, yes, but i mean an app inside our desktop
    'on': '2026-11-23'
  must_preserve:
  - the phone app is Exact and shared; the desktop app does not fork the radio's logic
  excluded:
  - voice asks on the web
stations:
- id: W0
  name: Can Exact live in a desktop window
  scope: read Exact's reference; mount or frame its web build in a desktop app window; sign-in, theme and audio focus shared with the desktop
  class: green for the finding; orange after
  review: one seat
  depends_on: []
  status: pending
- id: W1
  name: Speech in the browser
  scope: 'a local speech model measured in a browser on an ordinary laptop: load, speed, memory'
  class: yellow
  review: none
  depends_on: []
  status: pending
- id: W2
  name: The desktop app, listening only
  scope: the radio in the desktop from the shared code
  class: orange
  review: one seat
  depends_on:
  - W0
  - W1
  status: pending
decisions_for_the_owner:
- id: Q1
  question: Host the shared Exact code in a desktop window, or write the app in the desktop's own frontend?
  why: the first keeps one codebase; the second fits the desktop better
  options:
  - host Exact
  - native to the desktop
  default: host Exact, decided after W0's finding
now:
  status: working
  as_of: 2026-11-23T22:02Z
  slice: deferred until after Android (M4), which is itself deferred
  source_ref: origin/main bcea7eefd
  deploy_identity: staging on a2ba96767
  candidate: none
  believed_state: an intention; nothing investigated
  main_uncertainty: whether Exact's web build can live inside a desktop window
  next_observation: none until M4
  blocker: none
  next_action: nothing until the owner says
operating_notes:
- Recorded so the metamission holds every plan.
receipts:
- docs/design/design-session-2026-11-23.md
---

# Autoradio in the choir.news desktop

Standing: governing | kind: wanted | settled-by: owner 2026-11-23, quoted in this file's authority field (line 4); deferred in docs/ACTIVE.md | note: deferred until after Android (owner 2026-11-23)

Deferred. The owner wants the radio as an app in the choir.news desktop, listening only at first, with speech made in the browser by a local model and no cloud voice.
