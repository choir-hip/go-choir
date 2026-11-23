---
definition_version: 7
readiness: drafted
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
  - '2026-11-23T22:24Z: the development build "Autoradio 1" (news.choir.autoradio.lab.one, the same code as before, dcc56f2cd) is on the owner''s phone signed by the paid team; iOS refused the upgrade across teams, so that app alone was removed and installed again and starts signed out. The first app was not touched.'
  - '2026-11-23T22:50Z: the owner registered the bundle id news.choir.autoradio (Associated Domains enabled, nothing else), created the app record "Autoradio 1" in App Store Connect, an API key (the file is outside the repo) and an Apple Distribution certificate, which is on the Mac. The feedback address and the reviewer''s contact number are kept outside the repo. Still needed from the owner: the API key''s issuer id.'
  - '2026-11-23 late: the owner gave the API key''s issuer id; the key reads the app record and the bundle id through Apple''s API. The lead created the App Store provisioning profile "Autoradio 1 App Store" for news.choir.autoradio through the API (active, to 2027-11-23, tied to the distribution certificate); the profile file, the key, the ids and the contact details are in a folder outside the repo. Signing material for an upload is complete.'
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
  - decision: the app's name is Autoradio 1
    said: i have chosen the name Autoradio 1 btw (i like "A1" vs "AI" as it has slang connotations of trust and it implies there will be Autoradios 2..n which i expect competition)
    'on': '2026-11-23'
  - decision: 'Apple''s requirements are met before applying: an account made inside the app and deleted inside the app come first (this settles Q6)'
    said: we should prioritize making all the apple requirements like in-app account creation and account deletion before we apply. gotta put your best foot forward
    'on': '2026-11-23'
  - decision: this mission goes first in the metamission; other work continues while Apple reviews
    said: metamission wise, we can prioritize this, as we will have a couple days to wait on apples review. we will be busy working during that time
    'on': '2026-11-23'
  - decision: a new privacy policy is written; the owner reads it, the screenshots and the descriptions before anything is submitted
    said: yes, we need to make a new privacy policy. ill need to read that in addition to seeing the screenshots and descriptions you make
    'on': '2026-11-23'
  - decision: CarPlay is a priority once the iPhone app is on TestFlight
    said: i didnt select any capability requests but yes, carplay is a priority once we get ios working on testflight
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
  scope: 'done by the owner on 2026-11-23: the bundle id, the app record, the API key and its issuer id, the distribution certificate; the App Store profile made by the lead through the API'
  class: green
  review: none
  depends_on: []
  status: proved
- id: P1
  name: A build Apple accepts
  scope: the release build with no lab or trace pages, its version and build number set, uploaded; the first proof that Exact's archive passes Apple's processing. The upload is the same for internal and external; no internal group is a goal. Lane store-build was sent back by both review seats on 2026-11-23 (a lower-level build script let a development build take the production id); fixes in progress, then re-review
  class: yellow for scripts; orange for the build's contents
  review: one seat
  depends_on:
  - P0
  status: working
  lane: lane/store-build
- id: P2
  name: The first run, signed out
  scope: open the app and hear the public paper at once, never blank and never waiting; asking prompts sign-in or sign-up
  class: orange
  review: one seat
  depends_on: []
  status: pending
- id: P3
  name: An account made and deleted in the app
  scope: creation with a passkey inside the app, the same account as on choir.news; deletion in the app covering the account, the computer and published objects. Design first (a read-only proposal is being written 2026-11-23); then its stations
  class: red
  review: two seats
  depends_on: []
  status: working
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
  scope: scripted slim-simulator runs with seeded content, captured at Apple's current sizes, previews of captured footage only, the review notes and a short walk-through; with a new privacy policy, the privacy label answers and the store texts, drafted for the owner to read
  class: yellow
  review: none
  depends_on:
  - P2
  status: working
- id: P7
  name: External testers, as soon as possible
  scope: 'beta review passed and a testers group with a public link; the beta description, feedback contact, review information and what-to-test notes. On the owner''s word nothing is submitted until Apple''s requirements are met: the signed-out radio, an account made and deleted in the app, consent and permissions, and a privacy policy, screenshots and descriptions the owner has read'
  class: yellow
  review: none
  depends_on:
  - P0
  - P1
  - P2
  - P3
  - P4
  - P6
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
now:
  status: working
  as_of: 2026-11-23T22:50Z
  slice: P1 (the release build, lane store-build), P3 (design of in-app creation and deletion), P6 (privacy policy and texts drafted for the owner)
  source_ref: origin/main bcea7eefd
  deploy_identity: staging on a2ba96767
  candidate: none
  believed_state: signing material is complete; the release build is in review (sent back once); nothing is uploaded
  main_uncertainty: whether Exact's archive passes Apple's processing, and what a reviewer makes of AI-written news
  next_observation: the three agents' reports; the first upload's processing result
  blocker: none
  next_action: re-review and land the release build; the first archive and upload (no submission) to learn whether Apple accepts Exact's archive; the owner reads the drafts in docs/ship and rules on the account design's decisions
operating_notes:
- The before-we-ship list is kept in docs/design/design-session-2026-11-23.md until this file is executable; then it lives here as stations.
receipts:
- docs/design/design-session-2026-11-23.md
- docs/research/app-store-automation-2026-11-23.md
---

# Shipping the app: TestFlight and the App Store

Track T5. The owner renewed the developer account. This mission takes the app to testers and then to the store, and holds the list of things that must be true before it ships: the radio signed out, an account made and deleted inside the app, consent and permissions asked at the right moment, and materials a reviewer can follow.
