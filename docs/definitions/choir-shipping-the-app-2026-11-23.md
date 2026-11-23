---
definition_version: 7
readiness: intent
authority: 'the owner, 2026-11-23: "i just renewed my apple developer account. so, another new mission is doing our app store/testflight application... we need to do this and also make screenshots and video previews... and improve the onboarding (voice onboarding as mentioned above) and explanatory materials for the apple reviewers"; "you should be able to make an account (which also applies to choir.news) first inside autoradio app. so, we may have a list of things to do like this before we can ship".'
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
  - The app has never been submitted. The owner's developer account was renewed on 2026-11-23.
  - The app signs in to an existing account (the device flow); it creates none and has no in-app account deletion.
  - 'Signed out, the paper reads and the radio reads the public front page aloud: built, proven on a simulator, not seen by the owner on a phone.'
  - 'Research of 2026-11-23 (docs/research/app-store-automation-2026-11-23.md, read from Apple''s and the tools'' own pages that day): the first submission is mostly automatable; a person must sign agreements, do tax and banking, create the app record and the API key and enter the privacy label. Its ranked rejection risks: a reviewer who cannot sign in; no in-app account deletion; spoken requests reaching a third-party model without consent; completeness; model downloads.'
  - 'unknown: whether Exact''s archive passes Apple''s processing (never uploaded); how Exact sets the version and build number on iOS.'
  - '2026-11-23 evening: the owner signed in to Xcode with the renewed account. On the Mac: a development identity and a wildcard development profile for the paid team (S47GERGJZ3, to 2027-11-23) that covers the owner''s phone. No Apple Distribution identity yet, and no App Store profile.'
finish:
  deliver: Autoradio is on TestFlight for testers and then in the App Store, with a release an agent can run end to end except where Apple requires a person.
  artifact: an app record, a build accepted by Apple's processing, internal then external TestFlight groups, a submitted version with screenshots, previews and review notes, and a release script with its metadata as files in the repo
  acceptance:
  - action: upload a build from main
    proves: Exact's archive passes Apple's processing
    evidence_class: Apple's processing result
  - action: a new person installs from TestFlight, hears the radio signed out, creates an account in the app and deletes it
    proves: the first run, in-app creation and in-app deletion work for a stranger
    evidence_class: device drive and the owner's verdict
  - action: Apple's review of the first version
    proves: the submission is accepted
    evidence_class: Apple's verdict
  - action: ship an update by the release script
    proves: an update needs no person except where Apple requires one
    evidence_class: a second accepted version
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
  better_means: the time and human minutes from a commit on main to that build in a tester's hands falls, and the first review passes
  goodharting_would_be: screenshots of features that do not work; review notes that describe an app we have not built; a reviewer path that is a hidden switch; shipping the lab build
homotopy:
  realism_axis: external TestFlight first (beta review), then the App Store; the same build and scripts at each step
boundaries:
  mutation_class: yellow for release scripts and metadata; orange for app behaviour; red for account creation, deletion and sign-in
  authority_sources:
  - the owner's words in docs/design/design-session-2026-11-23.md; CLAUDE.md
  owner_rulings:
  - decision: an account is created inside the app and is the same account as on choir.news
    said: you should be able to make an account (which also applies to choir.news) first inside autoradio app
    'on': '2026-11-23'
  - decision: the paper and the radio work signed out
    said: and yes, autopaper and autoradio should work logged out
    'on': '2026-11-23'
  - decision: signed out is listening only; asking prompts sign-in or sign-up
    said: logged out should be listening only. if you ask it should prompt you to log in/ sign up
    'on': '2026-11-23'
  - decision: screenshots, preview videos, better onboarding and reviewer materials are part of this mission
    said: we need to do this and also make screenshots and video previews... and improve the onboarding (voice onboarding as mentioned above) and explanatory materials for the apple reviewers
    'on': '2026-11-23'
  - decision: several slim simulators may run at once
    said: the one-simulator rule came before we started using simslim. we can do multiple simulators now if they are slim
    'on': '2026-11-23'
  - decision: external TestFlight as soon as possible; internal TestFlight is not a goal, only the upload it shares
    said: idc about internal testflight. i want external testflight asap. internal testflight is just for the dev team, which is just me
    'on': '2026-11-23'
  - decision: 'bundle ids are in the news.choir family; the lead''s reading: the production id is news.choir.autoradio unless the owner names another before the app record is made'
    said: we should use news.choir
    'on': '2026-11-23'
  - decision: Autoradio 1's development build moves to the paid team
    said: yes, move autoradio 1 to the paid team
    'on': '2026-11-23'
  must_preserve:
  - sending a build to testers and submitting to Apple are the owner's acts until the owner delegates them in words
  - signing keys and API keys never enter the repo or an agent's output; key files are passed by path
  - no automation through Apple's private web session
  - the owner's current app is never reinstalled or refreshed without the owner's word
  - 'no switches: the reviewer''s way in is the signed-out radio everyone gets'
  excluded:
  - Android's store (M4)
  - paid tiers and in-app purchase
stations:
- id: P0
  name: The owner's account steps
  scope: agreements, tax and banking, the app record, an API key with the smallest role that works, the bundle id and name (Q1)
  class: green
  review: none
  depends_on: []
  status: pending
- id: P1
  name: A build Apple accepts
  scope: the release build with no lab or trace pages, its version and build number set, uploaded; the first proof that Exact's archive passes Apple's processing. The upload is the same for internal and external; no internal group is a goal
  class: yellow for scripts; orange for the build's contents
  review: one seat
  depends_on:
  - P0
  status: pending
- id: P2
  name: The first run, signed out
  scope: open the app and hear the public paper at once, never blank and never waiting; asking prompts sign-in or sign-up
  class: orange
  review: one seat
  depends_on: []
  status: pending
- id: P3
  name: An account made and deleted in the app
  scope: creation with a passkey inside the app, the same account as on choir.news; deletion in the app covering the account, the computer and published objects
  class: red
  review: two seats
  depends_on: []
  status: pending
- id: P4
  name: Consent, permissions and models
  scope: the microphone asked only when a signed-in listener turns voice on; one consent before the first request that reaches a model on a server; model files downloaded with their size stated, hosted by us; the privacy label and policy to match
  class: orange
  review: one seat
  depends_on: []
  status: pending
- id: P5
  name: Spoken onboarding
  scope: 'with the voice track (V7): the listener says what they are interested in'
  class: orange
  review: one seat
  depends_on:
  - 'M6: V7'
  status: pending
- id: P6
  name: Screenshots, previews and the reviewer's notes
  scope: scripted slim-simulator runs with seeded content, captured at Apple's current sizes, previews of captured footage only, the review notes and a short walk-through
  class: yellow
  review: none
  depends_on:
  - P2
  status: pending
- id: P7
  name: External testers, as soon as possible
  scope: 'the first aim of the mission: beta review passed and a testers group with a public link; the beta description, feedback contact, review information and what-to-test notes. What must be true first is the shortest list that passes beta review and serves a stranger: P0, P1, P2, and whichever of P3 and P4 the owner rules are needed before the first outside tester (Q6)'
  class: yellow
  review: none
  depends_on:
  - P0
  - P1
  - P2
  status: pending
- id: P8
  name: The first version submitted
  scope: metadata as files, age rating, export answer, submission; a rejection is answered and recorded
  class: yellow
  review: none
  depends_on:
  - P6
  - P7
  status: pending
- id: P9
  name: The release an agent runs
  scope: version and build numbers, change log to what's new, upload, groups, submission, phased release, reading crashes and feedback
  class: yellow
  review: one seat
  depends_on:
  - P8
  status: pending
decisions_for_the_owner:
- id: Q1
  question: What are the production bundle id and name?
  why: the id cannot change after the first upload
  options:
  - news.choir.autoradio, "Autoradio"
  - another
  default: nothing is uploaded until the owner names them
- id: Q2
  question: What is in version 1?
  why: each page shown to a reviewer must work; mail, files and takes have had little use on a phone
  options:
  - radio, paper, reader, voice and podcasts
  - everything
  default: radio, paper, reader, voice and podcasts; the rest when each has been used on the owner's phone
- id: Q3
  question: What does deleting an account remove?
  why: Apple requires deletion in the app; published stories and the computer are the question
  options:
  - the account, the computer and published objects
  - the account only
  default: designed and not built until the owner rules
- id: Q4
  question: Which territories first, and is the app free?
  why: the EU needs trader status; a price brings purchase rules
  options:
  - United States, free
  - wider
  default: United States, free
- id: Q5
  question: Which tool?
  why: the research recommends the App Store Connect CLI, pinned, with Apple's own uploader as fallback, and no private-session automation
  options:
  - that CLI
  - fastlane
  - the raw API
  default: that CLI, pinned, its telemetry turned off, after its reference page is written
- id: Q6
  question: Does the first external build wait for an account made and deleted inside the app?
  why: beta review can see the app through the signed-out radio; a tester who wants more needs an account. In-app creation and deletion are red work with two review seats; without them a first tester makes an account at choir.news and signs in with the device flow
  options:
  - go to beta review with the signed-out radio and sign-in to an existing account; in-app creation and deletion follow in the next build
  - wait for in-app creation and deletion
  default: wait for in-app creation, as the owner listed it first among the things before we can ship; the owner may choose the faster path
now:
  status: working
  as_of: 2026-11-23T22:02Z
  slice: not started; the research is in and the owner's account steps come first
  source_ref: origin/main bcea7eefd
  deploy_identity: staging on a2ba96767
  candidate: none
  believed_state: the rules and tools were read on 2026-11-23; nothing is built
  main_uncertainty: whether Exact's archive passes Apple's processing, and what a reviewer makes of AI-written news
  next_observation: the owner's answers on Q1 to Q5
  blocker: none
  next_action: the owner's account steps (P0) and Q1 can start at once; in parallel the release build and the first upload (P1) once the plan is defined; external beta review as soon as Q6's list is true
operating_notes:
- The before-we-ship list is kept in docs/design/design-session-2026-11-23.md until this file is executable; then it lives here as stations.
receipts:
- docs/design/design-session-2026-11-23.md
- docs/research/app-store-automation-2026-11-23.md
---

# Shipping the app: TestFlight and the App Store

Track T5. The owner renewed the developer account. This mission takes the app to testers and then to the store, and holds the list of things that must be true before it ships: the radio signed out, an account made and deleted inside the app, consent and permissions asked at the right moment, and materials a reviewer can follow.
