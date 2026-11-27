# Half-started work inventory, 2026-11-27

Standing: governing | kind: happened | settled-by: a read-only count on 2026-11-27 against origin/main 4ac73d992, station T12 of docs/definitions/choir-docs-transform-2026-11-27.md | note: the stalled-lane counts are an upper bound, because lanes landed by a squashed train look unlanded

Measured against origin/main 4ac73d992. Plain words; inference is marked.

## A. Lanes: summary

Refs examined: 502 (local lane trees, local lane/* branches, origin/lane/* branches). Distinct lane names: 400. Lane tree directories: 108 (plus 5 log files).

How each ref was classified (measured signals, in order): ahead 0 of main; or `git cherry` shows every commit already on main; or every commit subject already appears on main; or a trial merge changes nothing: LANDED. Otherwise last commit on or after 2026-11-25: ACTIVE; else STALLED. Inference layer, added because this repo lands lanes by squash and rebase and the signals above miss that: of a STALLED ref's changed files, the share whose final version (blob) appears anywhere in main history. 80 percent or more: LIKELY-LANDED (inference). 50 to 80 percent: STALLED-UNCLEAR. Under 50 percent: STALLED (the strongest candidate for never landed; later edits on main also lower the share, so it is not proof). DIRTY: the lane tree has uncommitted files.

Counts by lane name (excluding this session's 9 lanes): ACTIVE 4, LANDED 134, LIKELY-LANDED 69, STALLED 96, STALLED-UNCLEAR 89.

Counts by ref (same exclusion): ACTIVE 6, LANDED 139, LIKELY-LANDED 90, STALLED 138, STALLED-UNCLEAR 121.

Lane trees: 108; with uncommitted files: 24.

### This session's lanes (not judged)

| lane | ahead | unlanded by cherry | last commit | files | class (measured) | last subject |
|---|---|---|---|---|---|---|
| batch-queue | 2 | 2 | 2026-11-27 | 8 | ACTIVE | scripts: land-queue review fixes: result file, CI states, bash 3.2, clean tree (yellow; rollback: revert) |
| doc-first-line | 0 | 0 | 2026-11-27 | 0 | LANDED | docs: the standing triage index with overlap groups and questions for the owner (green; rollback: revert) |
| docs-query | 1 | 1 | 2026-11-27 | 8 | LANDED | scripts: a document is found by its first line; check-doc-standing; doccheck leaves the local gates (yellow; r |
| expiry-warns | 1 | 0 | 2026-11-27 | 4 | LANDED | switches: a passed date warns; a new switch is refused while one is due with no verdict (yellow; rollback: rev |
| gates-cost | 0 | 0 | 2026-11-27 | 0 | LANDED +DIRTY | docs: the docs transform after six stations landed (green; rollback: revert) |
| inbox-notes | 0 | 0 | 2026-11-27 | 0 | LANDED | docs: the docs transform after six stations landed (green; rollback: revert) |
| land-timing | 2 | 1 | 2026-11-27 | 8 | LANDED | docs: the change-log entry for timed landings (green; rollback: revert) |
| one-file-per-entry | 1 | 0 | 2026-11-27 | 12 | LANDED | docs: one file per change-log entry and per ledger claim (yellow; rollback: revert) |

### Lane trees (every directory under .lane/trees)

| tree | branch | ahead | not on main by cherry | last commit | files vs base | dirty files | class | class (inferred) |
|---|---|---|---|---|---|---|---|---|
| .trash | main | 0 | ? | 2026-11-26 | ? | 0 | EMPTY | |
| aasa | aasa | 1 | 1 | 2026-11-23 | 3 | 0 | STALLED | STALLED |
| account-delete | rebased | 38 | 38 | 2026-11-24 | 101 | 0 | STALLED | STALLED |
| acct-old | acct-old | 1 | 1 | 2026-11-22 | 171 | 1 | STALLED + DIRTY-UNCOMMITTED | STALLED +DIRTY |
| acctdel-land | acctdel-land | 3 | 3 | 2026-11-24 | 111 | 0 | STALLED | LIKELY-LANDED |
| android-1-goal | HEAD | 0 | ? | 2026-11-25 | ? | 0 | EMPTY | |
| android-a0 | HEAD | 0 | ? | 2026-11-26 | ? | 0 | EMPTY | |
| android-auth | android-auth | 0 | 0 | 2026-11-26 | 0 | 0 | LANDED |  |
| android-build-harness | HEAD | 0 | ? | 2026-11-26 | ? | 0 | EMPTY | |
| android-deck | android-deck | 1 | 1 | 2026-11-26 | 2 | 0 | ACTIVE |  |
| android-embed-bench | android-embed-bench | 0 | 0 | 2026-11-26 | 0 | 0 | LANDED |  |
| android-model-bench | HEAD | 0 | ? | 2026-11-26 | ? | 0 | EMPTY | |
| android-model-research | android-model-research | 0 | 0 | 2026-11-26 | 0 | 0 | LANDED |  |
| android-model-runtime | android-model-runtime | 0 | 0 | 2026-11-26 | 0 | 0 | LANDED |  |
| android-package | HEAD | 0 | ? | 2026-11-26 | ? | 0 | EMPTY | |
| android-parity-matrix | android-parity-matrix | 0 | 0 | 2026-11-26 | 0 | 0 | LANDED |  |
| android-silero-bench | HEAD | 0 | ? | 2026-11-26 | ? | 0 | EMPTY | |
| android-web-proof | android-web-proof | 0 | 0 | 2026-11-26 | 0 | 0 | LANDED |  |
| autoradio-design | autoradio-design | 100 | 74 | 2026-11-16 | 177 | 1 | STALLED + DIRTY-UNCOMMITTED | STALLED +DIRTY |
| autoradio-reader | autoradio-reader | 44 | 30 | 2026-11-15 | 135 | 0 | STALLED | STALLED |
| batch-queue | batch-queue | 2 | 2 | 2026-11-27 | 8 | 0 | ACTIVE |  |
| big-picture | HEAD | 0 | ? | 2026-11-27 | ? | 0 | EMPTY | |
| corpus-repl | HEAD | 3 | ? | 2026-11-24 | ? | 0 | DETACHED HEAD | |
| decision-record | decision-record | 6 | 5 | 2026-11-24 | 28 | 0 | STALLED | STALLED-UNCLEAR |
| density-2a | density-2a | 1 | 1 | 2026-11-23 | 3 | 0 | STALLED | STALLED |
| dev-profile | dev-profile | 1 | 1 | 2026-11-24 | 6 | 0 | LANDED |  |
| dev-profile-fix | dev-profile-fix | 3 | 2 | 2026-11-24 | 9 | 0 | LANDED |  |
| disk-leftovers | disk-leftovers | 11 | 2 | 2026-11-24 | 12 | 0 | LANDED |  |
| disk-report | disk-report | 10 | 1 | 2026-11-23 | 10 | 0 | LANDED |  |
| dispatch-deadlines | dispatch-deadlines | 2 | 2 | 2026-11-20 | 12 | 0 | STALLED | STALLED |
| dl-consent | dl-consent | 50 | 41 | 2026-11-24 | 178 | 5 | STALLED + DIRTY-UNCOMMITTED | LIKELY-LANDED +DIRTY |
| doc-first-line | doc-first-line | 0 | 0 | 2026-11-27 | 0 | 0 | LANDED |  |
| docs-query | docs-query | 1 | 1 | 2026-11-27 | 8 | 0 | LANDED |  |
| drop-zai | drop-zai | 2 | 1 | 2026-11-24 | 28 | 0 | LANDED |  |
| exact-edition-picks | exact-edition-picks | 4 | 4 | 2026-11-21 | 30 | 0 | STALLED | STALLED-UNCLEAR |
| exact-editor | exact-editor | 6 | 6 | 2026-11-24 | 111 | 0 | STALLED | STALLED-UNCLEAR |
| exact-rest | exact-rest | 8 | 8 | 2026-11-23 | 132 | 0 | STALLED | STALLED |
| exact-rest-a | exact-rest-a | 2 | 2 | 2026-11-24 | 81 | 0 | LANDED |  |
| expiry-warns | expiry-warns | 1 | 0 | 2026-11-27 | 4 | 0 | LANDED |  |
| exr-fix2 | exr-fix2 | 4 | 3 | 2026-11-24 | 83 | 0 | LANDED |  |
| exr-stack | exr-r | 13 | 13 | 2026-11-24 | 76 | 12 | STALLED + DIRTY-UNCOMMITTED | STALLED-UNCLEAR +DIRTY |
| first-open | first-open | 8 | 8 | 2026-11-23 | 31 | 0 | STALLED | STALLED |
| gates-cost | gates-cost | 0 | 0 | 2026-11-27 | 0 | 4 | LANDED + DIRTY-UNCOMMITTED |  |
| inbox-notes | inbox-notes | 0 | 0 | 2026-11-27 | 0 | 0 | LANDED |  |
| intake-starve | intake-starve | 3 | 2 | 2026-11-24 | 15 | 0 | LANDED |  |
| jev-no-halve-docs | jev-no-halve-docs | 3 | 3 | 2026-11-15 | 8 | 0 | STALLED | STALLED |
| land-s3 | land-s3 | 13 | 13 | 2026-11-24 | 32 | 0 | STALLED | STALLED |
| land-timing | land-timing | 2 | 1 | 2026-11-27 | 8 | 0 | LANDED |  |
| lane | main | 0 | ? | 2026-11-26 | ? | 0 | EMPTY | |
| model-list-route | model-list-route | 4 | 4 | 2026-11-23 | 10 | 0 | STALLED | STALLED |
| move-a | move-a | 48 | 35 | 2026-11-24 | 242 | 0 | STALLED | LIKELY-LANDED |
| move-b | move-b | 48 | 35 | 2026-11-24 | 276 | 0 | STALLED | STALLED-UNCLEAR |
| native-op-log | native-op-log | 14 | 10 | 2026-11-24 | 137 | 0 | STALLED | LIKELY-LANDED |
| offline-experiments | offline-experiments | 8 | 8 | 2026-11-24 | 38 | 0 | STALLED | STALLED |
| omni-ears | omni-ears | 11 | 9 | 2026-11-23 | 53 | 0 | STALLED | STALLED |
| onboard-consent | onboard-consent | 33 | 2 | 2026-11-24 | 110 | 0 | LANDED |  |
| one-file-per-entry | one-file-per-entry | 1 | 0 | 2026-11-27 | 12 | 0 | LANDED |  |
| podcast-mac | podcast-mac | 1 | 1 | 2026-11-24 | 3 | 0 | STALLED | STALLED |
| privacy-v2 | privacy-v2 | 6 | 1 | 2026-11-24 | 3 | 0 | LANDED |  |
| qa-mac | deck-build | 1 | 1 | 2026-11-26 | 28 | 38 | ACTIVE + DIRTY-UNCOMMITTED |  |
| requeue-left | requeue-left | 5 | 4 | 2026-11-24 | 17 | 0 | STALLED | STALLED-UNCLEAR |
| responder-omni | responder-omni | 6 | 5 | 2026-11-23 | 33 | 0 | STALLED | STALLED-UNCLEAR |
| rev-cap5 | HEAD | 51 | ? | 2026-11-24 | ? | 4 | DETACHED HEAD DIRTY-UNCOMMITTED | |
| rev-corpus | HEAD | 13 | ? | 2026-11-24 | ? | 1 | DETACHED HEAD DIRTY-UNCOMMITTED | |
| rev-corpus-narrow | main | 0 | ? | 2026-11-26 | ? | 0 | EMPTY | |
| rev-decrec | HEAD | 4 | ? | 2026-11-24 | ? | 5 | DETACHED HEAD DIRTY-UNCOMMITTED | |
| rev-delete2 | HEAD | 21 | ? | 2026-11-24 | ? | 0 | DETACHED HEAD | |
| rev-editor | HEAD | 13 | ? | 2026-11-24 | ? | 0 | DETACHED HEAD | |
| rev-intake | HEAD | 4 | ? | 2026-11-24 | ? | 0 | DETACHED HEAD | |
| rev-logs6 | HEAD | 8 | ? | 2026-11-24 | ? | 5 | DETACHED HEAD DIRTY-UNCOMMITTED | |
| rev-mlist2 | rev-mlist2 | 3 | 3 | 2026-11-23 | 10 | 3 | STALLED + DIRTY-UNCOMMITTED | STALLED +DIRTY |
| rev-onboard8 | HEAD | 32 | ? | 2026-11-24 | ? | 3 | DETACHED HEAD DIRTY-UNCOMMITTED | |
| rev-open2 | rev-open2 | 7 | 7 | 2026-11-23 | 30 | 3 | STALLED + DIRTY-UNCOMMITTED | STALLED +DIRTY |
| rev-pk8 | HEAD | 23 | ? | 2026-11-24 | ? | 2 | DETACHED HEAD DIRTY-UNCOMMITTED | |
| rev-privacy | HEAD | 1 | ? | 2026-11-24 | ? | 0 | DETACHED HEAD | |
| rev-pro | rev-pro | 4 | 4 | 2026-11-23 | 17 | 1 | STALLED + DIRTY-UNCOMMITTED | STALLED +DIRTY |
| rev-purge | rev-purge | 36 | 12 | 2026-11-24 | 103 | 1 | STALLED + DIRTY-UNCOMMITTED | STALLED-UNCLEAR +DIRTY |
| rev-purge2 | HEAD | 56 | ? | 2026-11-24 | ? | 3 | DETACHED HEAD DIRTY-UNCOMMITTED | |
| rev-resta | rev-resta | 1 | 1 | 2026-11-23 | 73 | 1 | LANDED + DIRTY-UNCOMMITTED |  |
| rev-s3 | rev-s3 | 12 | 12 | 2026-11-23 | 32 | 3 | STALLED + DIRTY-UNCOMMITTED | STALLED +DIRTY |
| rev-spacer | HEAD | 49 | ? | 2026-11-24 | ? | 2 | DETACHED HEAD DIRTY-UNCOMMITTED | |
| rev-store5 | rev-store5 | 9 | 7 | 2026-11-23 | 38 | 4 | LANDED + DIRTY-UNCOMMITTED |  |
| rev-texnew2 | rev-texnew2 | 2 | 2 | 2026-11-23 | 9 | 3 | STALLED + DIRTY-UNCOMMITTED | STALLED +DIRTY |
| rev-tmerge | HEAD | 1 | ? | 2026-11-24 | ? | 0 | DETACHED HEAD | |
| rev-voicemore | rev-voicemore | 29 | 4 | 2026-11-24 | 121 | 1 | STALLED + DIRTY-UNCOMMITTED | LIKELY-LANDED +DIRTY |
| search-ceramic | search-ceramic | 1 | 1 | 2026-11-18 | 24 | 0 | STALLED | STALLED |
| settle-once | settle-once | 6 | 6 | 2026-11-24 | 22 | 0 | STALLED | STALLED |
| ship-asc | ship-asc | 2 | 2 | 2026-11-24 | 4 | 0 | STALLED | STALLED-UNCLEAR |
| signin-sheet | signin-sheet | 16 | 6 | 2026-11-23 | 36 | 0 | STALLED | STALLED-UNCLEAR |
| snapshot-capture-s3 | snapshot-capture-s3 | 12 | 12 | 2026-11-23 | 32 | 0 | STALLED | STALLED |
| spacer-out | spacer-out | 5 | 2 | 2026-11-24 | 38 | 0 | LANDED |  |
| store-build | store-build | 10 | 7 | 2026-11-23 | 39 | 0 | LANDED |  |
| store-by-kind | store-by-kind | 1 | 0 | 2026-11-22 | 10 | 0 | LANDED |  |
| store-int | HEAD | 3 | ? | 2026-11-25 | ? | 0 | DETACHED HEAD | |
| story-events-e5 | story-events-e5 | 1 | 1 | 2026-11-20 | 28 | 0 | STALLED | STALLED |
| support-page | support-page | 5 | 5 | 2026-11-25 | 9 | 0 | ACTIVE |  |
| sweep-fix | sweep-fix | 16 | 3 | 2026-11-24 | 19 | 0 | LANDED |  |
| tester-capacity | tester-capacity | 54 | 1 | 2026-11-24 | 141 | 0 | LANDED |  |
| texture-merge | texture-merge | 1 | 1 | 2026-11-24 | 11 | 0 | STALLED | STALLED |
| texture-new-doc | texture-new-doc | 3 | 3 | 2026-11-23 | 9 | 0 | STALLED | STALLED |
| tiny-voca | tiny-voca | 60 | 51 | 2026-11-25 | 199 | 0 | ACTIVE |  |
| unified-voice-download | unified-voice-download | 51 | 42 | 2026-11-24 | 183 | 22 | STALLED + DIRTY-UNCOMMITTED | STALLED-UNCLEAR +DIRTY |
| usage-view | usage-view | 1 | 1 | 2026-11-23 | 6 | 0 | STALLED | STALLED |
| voice-default | voice-default | 5 | 4 | 2026-11-23 | 32 | 0 | LANDED |  |
| voice-models | voice-models | 21 | 21 | 2026-11-24 | 95 | 0 | STALLED | STALLED |
| voice-more | voice-more | 29 | 4 | 2026-11-24 | 121 | 0 | STALLED | LIKELY-LANDED |
| voice-pro | voice-pro | 7 | 6 | 2026-11-23 | 33 | 0 | STALLED | STALLED-UNCLEAR |
| voice-pro-rb | voice-pro-rb | 4 | 4 | 2026-11-23 | 17 | 0 | STALLED | STALLED |

### Stalled, active and likely-landed lanes by name (excluding this session's)

Biggest first. "commits" counts commits not matched on main by cherry (squash-landed work still counts here, see the class column). "docs" lists where docs/ names the lane (lane/<name>), measured by text match, not by judgment. Last three subjects are the lane's own words.

| lane | class | commits | files | last commit | on-main share | trees | dirty | docs naming it | last three commit subjects |
|---|---|---|---|---|---|---|---|---|---|
| tiny-voca | ACTIVE | 60 | 199 | 2026-11-25 | 91% | tiny-voca | 0 |  | mobile: the search embedding model is fetched by its press and part of the first // mobile: a pure C header over Dizione, so Swift imports the text frontend (orange // mobile: Dizione modules as C++, so Swift can import the port (orange; rollback:  |
| support-page | ACTIVE | 5 | 9 | 2026-11-25 | 78% | support-page | 0 |  | web: the support page names no policy section, and drops "we confirm by email",  // docs: the support page exists in lane support-page; the checklist and the URLs s // docs: model files are recommended and agreed in voice setup, Settings is for try |
| android-deck | ACTIVE | 1 | 2 | 2026-11-26 | 0% | android-deck | 0 |  | docs: record the Android deck's landing (green; rollback: revert) |
| deck-build | ACTIVE | 1 | 28 | 2026-11-26 | 64% | qa-mac | 38 |  | wip: salvage the Android deck before rebase (not for landing) |
| autoradio-background-audio | STALLED | 104 | 185 | 2026-11-16 | 43% |  | 0 |  | fix(autoradio): audio plays in the background and Play works after return: one s // fix(autoradio): remove press-and-hold talk (Lab talk button, choirTalkDown/Up, l // fix(autoradio): voice is off unless the listener taps it; no launch, foreground  |
| autoradio-content-fullscreen | STALLED | 104 | 185 | 2026-11-16 | 42% |  | 0 |  | feat(autoradio): Reader documents and embedded web pages fill the screen above t // fix(autoradio): remove press-and-hold talk (Lab talk button, choirTalkDown/Up, l // fix(autoradio): voice is off unless the listener taps it; no launch, foreground  |
| autoradio-qa | STALLED | 103 | 182 | 2026-11-16 | 43% |  | 0 | definitions/choir-autoradio-full-experience-overnight-2026-11-15.md, problems/autoradio-four-lists-decide-what-plays-next-and-there-is-no-desk-2026-11-16.md, problems/autoradio-owner-phone-review-2026-11-15.md | fix(autoradio): remove press-and-hold talk (Lab talk button, choirTalkDown/Up, l // fix(autoradio): voice is off unless the listener taps it; no launch, foreground  // feat(autoradio): demo mode puts a short sample book on the Reader's shelf (orang |
| autoradio-typography | STALLED | 103 | 182 | 2026-11-16 | 44% |  | 0 |  | feat(autoradio): a typographic system, so the face says headline, reading text,  // fix(autoradio): voice is off unless the listener taps it; no launch, foreground  // feat(autoradio): demo mode puts a short sample book on the Reader's shelf (orang |
| voice-off-default | STALLED | 103 | 182 | 2026-11-16 | 43% |  | 0 |  | fix(autoradio): remove press-and-hold talk (Lab talk button, choirTalkDown/Up, l // fix(autoradio): voice is off unless the listener taps it; no launch, foreground  // feat(autoradio): demo mode puts a short sample book on the Reader's shelf (orang |
| autoradio-reader | STALLED | 101 | 179 | 2026-11-16 | 41% | autoradio-reader | 0 | definitions/choir-autoradio-full-experience-overnight-2026-11-15.md | feat(autoradio): demo mode puts a short sample book on the Reader's shelf (orang // feat(autoradio): no pills, sweep 7: one prompt line, sections on one line, one f // feat(autoradio): no pills, sweep 6: the first-run prompt is one quiet line, equa |
| autoradio-design | STALLED +DIRTY | 100 | 177 | 2026-11-16 | 40% | autoradio-design | 1 | definitions/choir-autoradio-full-experience-overnight-2026-11-15.md | feat(autoradio): no pills, sweep 7: one prompt line, sections on one line, one f // feat(autoradio): no pills, sweep 6: the first-run prompt is one quiet line, equa // Autoradio queue: queueing what is playing is a no-op; the desk is sent only what |
| autoradio-queue | STALLED | 98 | 177 | 2026-11-16 | 40% |  | 0 | definitions/choir-autoradio-full-experience-overnight-2026-11-15.md | Autoradio queue: queueing what is playing is a no-op; the desk is sent only what // feat(autoradio): no pills, sweep 5: one rule for filled blocks, Podcasts on the  // feat(autoradio): no pills, sweep 4: the player (quiet card, one-line speed, hone |
| autoradio-joins | STALLED | 95 | 177 | 2026-11-15 | 39% |  | 0 |  | feat(autoradio): the desk is sent at most 50 upcoming queue entries; the tour's  // feat(autoradio): Hear it on a story's page and Clips on a show's page, with Play // feat(autoradio): a question can be answered from a book in the Reader, naming it |
| autoradio-dj-clips | STALLED | 47 | 133 | 2026-11-15 | 39% |  | 0 | definitions/choir-autoradio-full-experience-overnight-2026-11-15.md | feat(autoradio): the DJ plays podcast clips; Play hands the list to the DJ (oran // wip preflight // merge queue lane |
| autoradio-podcasts | STALLED | 43 | 137 | 2026-11-15 | 41% |  | 0 | definitions/choir-autoradio-full-experience-overnight-2026-11-15.md | feat(autoradio): podcasts: platform routes (report feeds, popular, shared transc // feat(autoradio): the Podcasts app: search, subscribe, show and episode pages, cl // Autoradio queue: remote audio, ops and snapshot, Up next, URL in the bar, statio |
| autoradio-talk | STALLED | 42 | 145 | 2026-11-15 | 42% |  | 0 | definitions/choir-autoradio-full-experience-overnight-2026-11-15.md | Autoradio talk: the answer is kept and shown, a first answer from saved stories, // Autoradio queue: remote audio, ops and snapshot, Up next, URL in the bar, statio // feat(autoradio): design system components in ui.contract: Emerge, PageHeader, Se |
| rebased | STALLED | 38 | 101 | 2026-11-24 | 43% | account-delete | 0 |  | account deletion round 3: each backup run copies the deletion ledger off-host, t // account deletion round 3: the state, not the clock, is the cancel boundary; the  // auth: a recovery link is not mailed for an account whose deletion is pending; te |
| account-delete | STALLED | 36 | 94 | 2026-11-24 | 36% |  | 0 | definitions/choir-shipping-the-app-2026-11-23.md, design/account-create-delete-2026-11-23.md | auth: a recovery link is not mailed for an account whose deletion is pending; te // corpusd: the route slots live in the control database, not the corpus one (red;  // docs: account deletion round 2: the change entry, the one statement in the polic |
| autoradio-paper-views | STALLED | 29 | 103 | 2026-11-15 | 42% |  | 0 |  | feat(autoradio): the paper's front has views: front page, my edition, each categ // feat(autoradio): edit the edition after onboarding with the same topic tree; the // Merge remote-tracking branch 'origin/lane/autoradio-topic-tree' into lane/autora |
| autoradio-edit-edition | STALLED | 28 | 96 | 2026-11-15 | 37% |  | 0 |  | feat(autoradio): edit the edition after onboarding with the same topic tree; the // Merge remote-tracking branch 'origin/lane/autoradio-topic-tree' into lane/autora // feat(autoradio): the topic screen is a tree of ten categories with shared subtop |
| autoradio-handsfree-toggle | STALLED | 28 | 98 | 2026-11-15 | 34% |  | 0 |  | fix(autoradio): hands-free is one on/off switch; listening shows only for a real // Merge remote-tracking branch 'origin/lane/autoradio-topic-tree' into lane/autora // feat(autoradio): the topic screen is a tree of ten categories with shared subtop |
| autoradio-phonon | STALLED | 26 | 98 | 2026-11-15 | 40% |  | 0 |  | feat(autoradio): Phonon 31M as an alternative voice beside Kokoro, downloaded fr // Merge remote-tracking branch 'origin/lane/autoradio-voice' into lane/autoradio-q // feat(autoradio): the neural voice is the default and speaks everywhere; voice ca |
| offline-slice-r0 | STALLED | 26 | 40 | 2026-11-22 | 0% |  | 0 | design/first-design-2026-11-22.md | docs: offline slice rungs 1 and 2 (System 1 scoring, group naming, kill lines) ( // offline slice rung 2: river naming runner (yellow; rollback: revert) // offline slice rung 2: capped, relay-collapsed groups for naming (yellow; rollbac |
| autoradio-topic-tree | STALLED | 24 | 80 | 2026-11-15 | 34% |  | 0 |  | feat(autoradio): the topic screen is a tree of ten categories with shared subtop // fix(autoradio): selecting a topic no longer changes the chip's size (orange; rol // Merge remote-tracking branch 'origin/lane/autoradio-onboarding' into lane/autora |
| ar3-shell | STALLED | 21 | 19 | 2026-11-11 | 28% |  | 0 |  | test(autoradio-app): lens variants shot at the top of the long front page (yello // test(autoradio-app): lens variants shot over the long front page, mid-scroll (ye // test(autoradio-app): three lens variants (native glass, plain fill, ultra-thin)  |
| voice-models | STALLED | 21 | 95 | 2026-11-24 | 48% | voice-models | 0 | reports/last-day-on-the-metamission-2026-11-24.md | phone: no nil gate: the Test gate is required at every effect entry, the voice s // phone: a Test is an obligation of one account generation: one gate (TrialGate) i // docs: problem record and structural assessment: a Test on Voice and models is an |
| autoradio-player | STALLED | 18 | 67 | 2026-11-15 | 31% |  | 0 |  | fix(autoradio): the DJ gets the paper's stories; the radio never announces there // feat(autoradio): player: speed slider to 3x, next and previous story, autoplay d // Merge remote-tracking branch 'origin/lane/autoradio-search' into lane/autoradio- |
| autoradio-stack-load | STALLED | 18 | 59 | 2026-11-15 | 36% |  | 0 | problems/autoradio-owner-phone-review-2026-11-15.md | fix(autoradio): the microphone is hands-free: a tap opens it, the turn ends when // fix(autoradio): stack cards load ahead, show the real title at once and emerge i // Merge remote-tracking branch 'origin/lane/autoradio-search' into lane/autoradio- |
| autoradio-onboarding | STALLED | 17 | 73 | 2026-11-15 | 33% |  | 0 |  | feat(autoradio): edition setup is a screen of topic chips, not a spoken Q&A; fin // Merge remote-tracking branch 'origin/lane/autoradio-search' into lane/autoradio- // Merge remote-tracking branch 'origin/lane/autoradio-webview' into lane/autoradio |
| autoradio-voice | STALLED | 17 | 72 | 2026-11-15 | 36% |  | 0 |  | feat(autoradio): the neural voice is the default and speaks everywhere; voice ca // Merge remote-tracking branch 'origin/lane/autoradio-search' into lane/autoradio- // Merge remote-tracking branch 'origin/lane/autoradio-webview' into lane/autoradio |
| autoradio-webview-embed | STALLED | 17 | 58 | 2026-11-15 | 38% |  | 0 |  | feat(autoradio): a source opens in place above the bar, in one tap from the inli // Merge remote-tracking branch 'origin/lane/autoradio-search' into lane/autoradio- // Merge remote-tracking branch 'origin/lane/autoradio-webview' into lane/autoradio |
| updater-prune | STALLED | 15 | 18 | 2026-11-13 | 33% |  | 0 | definitions/choir-storage-solutions-3-store-once-2026-11-14.md, problems/updater-directory-fills-the-data-volume-2026-11-13.md, problems/updater-prune-three-rounds-structural-assessment-2026-11-14.md | fix(updater,agentcore): keep the applied and restaged predecessors, bound the re // test(updater,agentcore): failing tests for round two of the review (yellow; roll // test(updater): an unstaged payload goes past the window, not before (yellow; rol |
| account-boundary-l1 | STALLED | 15 | 65 | 2026-11-21 | 0% |  | 0 | changes.md | mobile: exempt sources get a device storage handle; Texture themes only through  // mobile: the Texture status line is the world's, the files cache path names the a // mobile: themes are two stores (device, and Texture themes as account data in the |
| land-s3 | STALLED | 13 | 32 | 2026-11-24 | 0% | land-s3 | 0 |  | snapshot: the guest-security effect of the three capture commits, as a review se // snapshot: a seam at the stream's end so the success-path deadline test is determ // snapshot: the two-tick producer test moves the head between ticks (yellow; rollb |
| restore-boot-install | STALLED | 13 | 41 | 2026-11-15 | 0% |  | 0 | problems/restore-act-three-review-rounds-structural-assessment-2026-11-15.md | docs(design): restore installs at boot, design section and failure list before c // fix(restore): the act's id and computer are fixed fields, not read from a record // fix(restore): one repair routine driven by the durable record; the engine undoes |
| snapshot-capture-s3 | STALLED | 13 | 32 | 2026-11-24 | 0% | snapshot-capture-s3 | 0 | definitions/choir-storage-efficiency-2026-11-22.md | snapshot: the guest-security effect of the three capture commits, as a review se // snapshot: a seam at the stream's end so the success-path deadline test is determ // snapshot: the two-tick producer test moves the head between ticks (yellow; rollb |
| restore-act | STALLED | 12 | 40 | 2026-11-15 | 0% |  | 0 | definitions/choir-storage-solutions-3-store-once-2026-11-14.ledger.md, problems/restore-act-three-review-rounds-structural-assessment-2026-11-15.md | fix(restore): the act's id and computer are fixed fields, not read from a record // fix(restore): one repair routine driven by the durable record; the engine undoes // test(restore): review round 2, failing: kill after the in-process undo, a live - |
| autoradio-search | STALLED | 12 | 49 | 2026-11-15 | 37% |  | 0 |  | docs(autoradio): the prompt bar says it asks the radio about this page (green) // feat(autoradio): launcher search covers the whole paper through the public searc // chore(mobile): a simulator made by sim-device.sh is slimmed before first use (ye |
| tester-capacity-r2 | STALLED | 12 | 31 | 2026-11-23 | 6% |  | 0 |  | docs: changes: remove conflict markers left by the rebase onto main (green; roll // auth: the sweep test covers the window after the challenge is consumed; the dupl // nix: the comment on VM_MEM_MIB says a new account's computer is 4 GiB (green; ro |
| rev-s3 | STALLED +DIRTY | 12 | 32 | 2026-11-23 | 0% | rev-s3 | 3 |  | snapshot: a seam at the stream's end so the success-path deadline test is determ // snapshot: the two-tick producer test moves the head between ticks (yellow; rollb // snapshot: the retry test crawls after the third chunk, not at a fixed page (yell |
| omni-ears | STALLED | 11 | 53 | 2026-11-23 | 43% | omni-ears | 0 | reference/VOICE-MODEL-CATALOG.md | docs, scripts: LFM3-Omni-2B as the ears: word error against Fluid Speech on nois // merge voice-pro: Fluid Speech Pro, for the like-for-like noisy-speech comparison // docs: LFM3-Omni-2B as a larger responder: what it is, the row, the Mac measureme |
| autoradio-webview | STALLED | 11 | 49 | 2026-11-15 | 35% |  | 0 |  | feat(autoradio): source and article links open in an in-app browser sheet, not S // chore(mobile): a simulator made by sim-device.sh is slimmed before first use (ye // Merge remote-tracking branch 'origin/lane/autoradio-handsfree' into lane/autorad |
| restart-cancels-capture | STALLED | 10 | 42 | 2026-11-13 | 0% |  | 0 | definitions/choir-storage-solutions-2-2026-11-12.ledger.md, definitions/choir-storage-solutions-2-2026-11-12.md, problems/snapshot-loop-blocks-its-own-fix-2026-11-12.md | fix(snapshot): the marker's owner enforces ownership and release, review round 4 // test(snapshot): failing tests for review round 4 of the restart-cancel change (y // fix(snapshot): one owner for the planned-restart marker, review round 3 (red; ro |
| exact-rest | STALLED | 8 | 132 | 2026-11-23 | 47% | exact-rest | 0 | definitions/choir-exact-cutover-and-ontology-2026-11-20.md, definitions/choir-shipping-the-app-2026-11-23.md | mobile: fix round 3 of the exact-rest review, seat 2 and the rest of seat 1: the // mobile: fix round 3 of the exact-rest review: the keystroke is on disk before it // mobile: fix round 2 of the exact-rest review: ONE queue per Texture document, ON |
| first-open | STALLED | 8 | 31 | 2026-11-23 | 0% | first-open | 0 |  | phone: second review round: laps keep two route segments; the show page never dr // phone: review fixes: the kept copy reaches the podcast and notebook pages; a sav // phone: laps log the page kind and a route pattern, never an address, an id or a  |
| s4-stage0-roots | STALLED | 8 | 13 | 2026-11-13 | 23% |  | 0 | problems/unappended-snapshot-root-evicts-the-on-tape-root-2026-11-13.md | fix(platform,vmctl): the named snapshot roots are ordered by their naming event; // test(snapshot roots): four in-flight registrations keep the first; named roots o // docs(problems): Stage 0 build notes, the vmctl base_missing correction, the in-f |
| offline-experiments | STALLED | 8 | 38 | 2026-11-24 | 0% | offline-experiments | 0 | definitions/choir-records-and-measurement-2026-11-23.md | docs: E5 results: the batch model pass, arms A, B, D, E, A5 and A2 on the 3,529  // docs: E4 results: novelty relative to the subject; ledger C221 to C231 settled ( // offline experiments: E5 arm E calls, digest struct and runner (yellow; rollback: |
| rev-open2 | STALLED +DIRTY | 7 | 30 | 2026-11-23 | 0% | rev-open2 | 3 |  | phone: review fixes: the kept copy reaches the podcast and notebook pages; a sav // phone: laps log the page kind and a route pattern, never an address, an id or a  // phone: the radio's open asks the front list beside the work that does not need i |
| jev-gather-design | STALLED | 6 | 1 | 2026-11-15 | 0% |  | 0 |  | docs(design): intake gather, review round 2: an arrival is held only when its en // docs(design): the gather holds the arrival and releases it with its labels; the  // docs(design): review round 1 repairs the gather by subtraction; the intake task' |
| account-partition-s1 | STALLED | 6 | 179 | 2026-11-22 | 0% |  | 0 | definitions/choir-first-design-2026-11-22.md, problems/account-boundary-three-approaches-no-convergence-2026-11-23.md, problems/account-wipe-seal-six-reviews-and-tests-wipe-the-mac-2026-11-22.md | account partition: native is told `ready` and handed the key by whichever answer // account partition: the store exists only in an answer's first turn, so the key i // account partition: the interface generator stops declaring the sign-in the share |
| exact-account-s1 | STALLED | 6 | 179 | 2026-11-22 | 0% |  | 0 |  | account partition: native is told `ready` and handed the key by whichever answer // account partition: the store exists only in an answer's first turn, so the key i // account partition: the interface generator stops declaring the sign-in the share |
| root-desk-on | STALLED | 6 | 37 | 2026-11-20 | 31% |  | 0 | definitions/choir-exact-search-entities-2026-11-20.md, definitions/choir-first-design-2026-11-22.md | tests: the boot test advances the clock so the post-boot digest's nonce differs  // textureowner, store: one hourly root digest for every document; a digest names n // textureowner, store: the root desk batches story documents by the hour, has no h |
| settle-once | STALLED | 6 | 22 | 2026-11-24 | 0% | settle-once | 0 |  | mobile: no waiter deadline; a run that does not publish fails; a store read that // mobile: a settle whose key read threw is not published; waiting has a deadline;  // mobile: one answer owns the launch settle; others wait for its value; no promise |
| ar-core | STALLED | 5 | 46 | 2026-11-10 | 48% |  | 0 |  | feat(mobile): build-lab and sim-launch scripts, Exact app builds and launches on // feat(mobile): ModelStore, Exact package and AutoradioLab app (Contract screens), // feat(mobile): live audio engine, programme player, lab runtime for hosts (green; |
| t2-run-writes-design | STALLED | 5 | 1 | 2026-11-15 | 0% |  | 0 |  | docs(design): round 2 repairs to the T2 source-entities design, consumers, marke // docs(design): the source-entities list is not stored in the run record, round 1  // docs(design): the source-entities list leaves the run record, design and failure |
| exact-convo-edit | STALLED | 5 | 39 | 2026-11-20 | 16% |  | 0 | changes.md | autoradio: the Texture editor never reverts another writer, never posts a text w // autoradio: the Texture editor carries the old build's unsent edit over, flushes  // autoradio: the Texture editor is shared Exact code; the Swift editor session, it |
| event-index-diff | STALLED | 4 | 12 | 2026-11-14 | 33% |  | 0 | problems/replay-of-de-differs-on-the-event-index-only-2026-11-14.md, problems/restore-cannot-finish-inside-its-ten-minute-request-2026-11-14.md | docs(problems): the event index replay difference, code investigation and repair // fix(agentcore,store,computerversion): the probe compares the event index without // fix(store): Prepare writes "[]" for an event with no payload pins, never null (r |
| incremental-snapshots-design | STALLED | 4 | 1 | 2026-11-15 | 0% |  | 0 | problems/incremental-snapshots-design-three-review-rounds-structural-assessment-2026-11-15.md | docs(design): incremental snapshots, review round 2: the existing floor stays an // docs(design): incremental snapshots, review round 1 repaired by subtraction (gre // docs(design): incremental snapshots, two line anchors corrected (green) |
| model-list-route | STALLED | 4 | 10 | 2026-11-23 | 0% | model-list-route | 0 |  | wip: model list wording: the CI check is a checksum lock (green; rollback: rever // proxy: model list review fixes: the route test applies the phone's exact rules,  // wip: model list review fixes (yellow; rollback: revert) |
| rev-pro | STALLED +DIRTY | 4 | 17 | 2026-11-23 | 12% | rev-pro | 1 |  | phone: a model row carries its credit; Parakeet TDT 0.6B v2 and Pocket TTS, both // phone: setup's first transcriber row is Pro on this list; the test names both ro // docs: Fluid Speech Pro: clean-set word error re-run, per-process memory (green) |
| voice-pro-rb | STALLED | 4 | 17 | 2026-11-23 | 12% | voice-pro-rb | 0 |  | phone: a model row carries its credit; Parakeet TDT 0.6B v2 and Pocket TTS, both // phone: setup's first transcriber row is Pro on this list; the test names both ro // docs: Fluid Speech Pro: clean-set word error re-run, per-process memory (green) |
| jev-no-halve-docs | STALLED | 3 | 8 | 2026-11-15 | 0% | jev-no-halve-docs | 0 |  | docs(reference): the Jev pages say what a failed call does now (green) // fix(intake): a flush whose story index could not be read leaves its captures unl // fix(jev): a failed call that is not a size refusal is never halved (red; rollbac |
| forward-restore-design | STALLED | 3 | 1 | 2026-11-15 | 0% |  | 0 |  | docs(design): forward restore event accepted in round 2, should-fixes taken (gre // docs(design): forward restore event, review round 1 repaired (green) // docs(design): the forward restore event, a restore that survives boot and append |
| jev-gather | STALLED | 3 | 8 | 2026-11-15 | 0% |  | 0 |  | fix(sweep): the held-capture sweep also serves a computer with only intake label // fix(intake): a flush whose story index could not be read leaves its captures unl // fix(jev): a failed call that is not a size refusal is never halved (red; rollbac |
| node-a-memory | STALLED | 3 | 5 | 2026-11-14 | 0% |  | 0 | reports/overnight-autoradio-and-capture-2026-11-15.md | docs(node-a): the memory patch is applied on Node A; Mike's VMs and the runner u // docs(node-a): memory patch after review; victim order holds at any size, every p // docs(node-a): memory budget for a second build VM without overcommit; problem re |
| t1-carry-design | STALLED | 3 | 1 | 2026-11-15 | 0% |  | 0 | problems/carry-by-reference-design-three-review-rounds-structural-assessment-2026-11-15.md | docs(design): carry by reference round 2 repair, seq rule, resolution position,  // docs(design): carry by reference round 1 repair, apply-time retire guard, epoch  // docs(design): carry by reference, design section and failure list for T1 (green) |
| account-settle-memo | STALLED | 3 | 3 | 2026-11-21 | 0% |  | 0 | changes.md | account: the remembered settle answer is valid only for the state it was compute // account: tests from the table for the state of an unclean sign-out, red on the c // docs: the state table for an unclean sign-out, and the frame that replaces the e |
| embed-pass | STALLED | 3 | 8 | 2026-11-19 | 0% |  | 0 | definitions/choir-night-exact-vectors-learning-2026-11-19.md, problems/first-vectors-file-refused-equal-text-under-another-title-2026-11-19.md | embed-pass: equal text under another title is embedded again; a resumed run reus // embed-pass: the export confines every read under the artifacts root, orders stor // embed-pass: the Mac's first-pass tool (embed-trial pass), the read-only public e |
| owner-act-enforce | STALLED | 3 | 58 | 2026-11-20 | 42% |  | 0 | changes.md | platform, nix: owner acts always refuse; the observe mode and CHOIR_OWNER_ACT_MO // agentcore, store, runtimeprompts: production by event is deleted; intake.event_w // vmmanager, nix: the DAMON guest reclaim experiment is deleted; VM_GUEST_RECLAIM_ |
| switches-on | STALLED | 3 | 30 | 2026-11-19 | 41% |  | 0 |  | textureowner, agentcore, store: the Texture root desk runs on every computer; te // actor, agentcore, store: every computer runs twelve shared dispatcher slots; dis // agentcore, store: the event layer runs with no setting; intake.events is gone (o |
| texture-new-doc | STALLED | 3 | 9 | 2026-11-23 | 0% | texture-new-doc | 0 |  | mobile: New document: the test leaves Textures, and an answer under the launcher // mobile: New document opens straight in the editor; one create at a time; the ope // mobile: New document makes a document and opens it, top right of Textures; a fai |
| rev-mlist2 | STALLED +DIRTY | 3 | 10 | 2026-11-23 | 0% | rev-mlist2 | 3 |  | proxy: model list review fixes: the route test applies the phone's exact rules,  // wip: model list review fixes (yellow; rollback: revert) // proxy: choir.news serves the phone's model list at /api/phone/model-list, the ap |
| aasa | STALLED | 2 | 4 | 2026-11-23 | 25% | aasa | 0 | changes.md | proxy: the association file is built from one list; tests pin the headers, HEAD  // proxy: serve /.well-known/apple-app-site-association (webcredentials for the pho |
| dispatch-deadlines | STALLED | 2 | 12 | 2026-11-20 | 0% | dispatch-deadlines | 0 | changes.md | dispatcher: a deadline whose cell or run has ended is consumed at due time witho // dispatcher: one line per slow activation and a per-minute summary of the rest, w |
| autoradio-nav | STALLED | 2 | 14 | 2026-11-15 | 0% |  | 0 |  | fix(autoradio): no page scrolls sideways, menu rows wrap, tables and code fit th // feat(autoradio): paper articles are a card stack, swipe goes to the next and pre |
| autoradio-speech-fixes | STALLED | 2 | 6 | 2026-11-15 | 17% |  | 0 |  | fix(autoradio): review repair: the loader alone starts the interview, the end of // fix(autoradio): the interview asks once, speech does not stop at an empty buffer |
| capture-bodies-gc | STALLED | 2 | 4 | 2026-11-13 | 0% |  | 0 | problems/capture-bodies-sweep-admission-2026-11-14.md, problems/node-b-residue-after-the-sweep-2026-11-13.md | feat(platform): capture bodies join the artifact collector's reachability sweep  // test(platform): failing tests for capture bodies under the artifact collector (y |
| jev-judges-serial | STALLED | 2 | 8 | 2026-11-10 | 25% |  | 0 |  | fix(jev): the event layer, support, sub-call, cast, inert and story-analysis jud // docs(jev): the judges' serial form and the ways it can fail (green; rollback: re |
| jev-no-halve | STALLED | 2 | 6 | 2026-11-15 | 0% |  | 0 | calibration/claude-ledger.md | fix(intake): a flush whose story index could not be read leaves its captures unl // fix(jev): a failed call that is not a size refusal is never halved (red; rollbac |
| acct-webtest | STALLED | 2 | 177 | 2026-11-22 | 0% |  | 0 |  | mobile: the mail frame test puts the mailbox on top itself (the launch is no lon // account partition: the key is Exact's store; sign-in, sign-out and the launch ve |
| search-leads-l2 | STALLED | 2 | 41 | 2026-11-20 | 47% |  | 0 | definitions/choir-exact-cutover-and-ontology-2026-11-20.md | scripts/gates runs the replay script's test and the CI classifier selects the Go // corpusd's ranked answer says how strong and recent it is and returns held leads; |
| rev-texnew2 | STALLED +DIRTY | 2 | 9 | 2026-11-23 | 0% | rev-texnew2 | 3 |  | mobile: New document opens straight in the editor; one create at a time; the ope // mobile: New document makes a document and opens it, top right of Textures; a fai |
| acct-old | STALLED +DIRTY | 1 | 171 | 2026-11-22 | 0% | acct-old | 1 |  | account partition: one launch verdict, made once by native; sign-out wipes nothi |
| density-2a | STALLED | 1 | 3 | 2026-11-23 | 0% | density-2a | 0 |  | scripts: status-density, step 2a's density measurement; part 1 blocked at the pe |
| autoradio-search-listen | STALLED | 1 | 10 | 2026-11-15 | 20% |  | 0 |  | feat(autoradio-app): Listen to one story, a Texture, an EPUB chapter or a text f |
| autoradio-voice-engine | STALLED | 1 | 1 | 2026-11-15 | 0% |  | 0 | problems/autoradio-voice-pipeline-not-the-one-chosen-2026-11-24.md | docs(design): Autoradio's voice engine, hands-free turn, dictation and reading a |
| bases-bench | STALLED | 1 | 1 | 2026-11-15 | 0% |  | 0 | reports/overnight-autoradio-and-capture-2026-11-15.md | test(store): page-churn bench measures p at four unit sizes, index/table split a |
| take-writer-land | STALLED | 1 | 5 | 2026-11-14 | 0% |  | 0 | ACTIVE.md, reports/overnight-autoradio-and-capture-2026-11-15.md | fix(texture): every owner-route commit picks its writer from the write set in on |
| crash-guard-proof | STALLED | 1 | 1 | 2026-11-16 | 0% |  | 0 | calibration/claude-ledger.md | test(frontend): deployed proof of the desktop crash guard on a fresh account (ye |
| exact-audio-research | STALLED | 1 | 3 | 2026-11-21 | 0% |  | 0 |  | docs: what of audio and voice Exact can carry at f53347503 (green) |
| jev-one-state | STALLED | 1 | 21 | 2026-11-12 | 14% |  | 0 | problems/jev-misuse-structural-assessment-2026-11-15.md | feat(jev): intake labels ask one state with every question, sized by the vendor' |
| podcast-mac | STALLED | 1 | 3 | 2026-11-24 | 0% | podcast-mac | 0 |  | scripts: the podcast worker as a launchd agent on a Mac (yellow; rollback: rever |
| reply-wake-fix | STALLED | 1 | 6 | 2026-11-20 | 17% |  | 0 | definitions/choir-exact-cutover-and-ontology-2026-11-20.md | agentcore: a management reply to a stale escalation does not wake a research des |
| root-desk-r3 | STALLED | 1 | 37 | 2026-11-20 | 0% |  | 0 | definitions/choir-first-design-2026-11-22.md | textureowner, store, objectgraph, agentcore, actor: the Texture root desk runs o |
| search-ceramic | STALLED | 1 | 24 | 2026-11-18 | 0% | search-ceramic | 0 | definitions/choir-exact-cutover-and-ontology-2026-11-20.md, definitions/choir-exact-search-entities-2026-11-20.md, definitions/choir-night-exact-vectors-learning-2026-11-19.md | search: Ceramic starts every search; Keenable joins on shortfall under a cap of  |
| slides-next | STALLED | 1 | 5 | 2026-11-02 | 0% |  | 0 | definitions/choir-selfdev-apps-feature-complete-2026-11-03.md, problems/slides-html-deck-markup-runs-in-shell-origin-2026-11-03.md | slides-next: Slides (new) beside Slides, built by self-development on b@c with t |
| snapshot-stream | STALLED | 1 | 27 | 2026-11-20 | 4% |  | 0 | changes.md | snapshot: the capture streams the store's pages from its one read transaction, w |
| texture-merge | STALLED | 1 | 11 | 2026-11-24 | 0% | texture-merge | 0 |  | texture: the server derives a revision's body from the owner's typed text: textu |
| usage-view | STALLED | 1 | 6 | 2026-11-23 | 0% | usage-view | 0 |  | scripts: status-account-usage, a read-only per-account usage report from what No |
| story-events-e5 | STALLED | 1 | 28 | 2026-11-20 | 0% | story-events-e5 | 0 | definitions/choir-exact-cutover-and-ontology-2026-11-20.md | corpusd, agentcore, proxy: a published story carries the ids of the world events |
| autoradio-integ23 | LIKELY-LANDED | 223 | 447 | 2026-11-18 | 98% |  | 0 | definitions/choir-night-exact-vectors-learning-2026-11-19.md | Merge remote-tracking branch 'origin/lane/audio-held' into autoradio-integ23 // autoradio: remove the block wait bound; the lock screen's play and toggle reach  // Merge remote-tracking branch 'origin/lane/no-withholding' into autoradio-integ22 |
| autoradio-integ22 | LIKELY-LANDED | 221 | 447 | 2026-11-18 | 97% |  | 0 |  | Merge remote-tracking branch 'origin/lane/no-withholding' into autoradio-integ22 // Merge remote-tracking branch 'origin/lane/audio-held' into autoradio-integ22 // autoradio: the phone no longer withholds mail, file passages or real titles from |
| no-withholding | LIKELY-LANDED | 217 | 447 | 2026-11-18 | 96% |  | 0 |  | autoradio: the phone no longer withholds mail, file passages or real titles from // autoradio: the press-and-hold scan reads the app's source, not Exact's build out // Merge remote-tracking branch 'origin/lane/start-gap' into autoradio-integ21 |
| autoradio-integ21 | LIKELY-LANDED | 216 | 447 | 2026-11-18 | 96% |  | 0 | definitions/choir-autoradio-exact-cutover-2026-11-18.md | autoradio: the press-and-hold scan reads the app's source, not Exact's build out // Merge remote-tracking branch 'origin/lane/start-gap' into autoradio-integ21 // autoradio: a place-now dropped as duplicate_id airs the server's copy; an ahead  |
| audio-held | LIKELY-LANDED | 214 | 446 | 2026-11-18 | 97% |  | 0 | definitions/choir-autoradio-exact-cutover-2026-11-18.md, problems/autoradio-first-listening-session-2026-11-18.md | autoradio: remove the block wait bound; the lock screen's play and toggle reach  // autoradio: a pause by the listener holds an entry held at its start too; foregro // autoradio: a held entry tells the system's pause from the listener's; one Play r |
| phone-main | LIKELY-LANDED | 211 | 396 | 2026-11-18 | 100% |  | 0 | definitions/choir-autoradio-exact-cutover-2026-11-18.md | docs: the phone code is on main; three server fixtures the phone never reads (gr // Merge remote-tracking branch 'origin/main' into phone-main // autoradio: the phone work of 2026-11-15 to 2026-11-18 joins main, 162 commits fr |
| start-gap | LIKELY-LANDED | 211 | 446 | 2026-11-18 | 97% |  | 0 | definitions/choir-autoradio-exact-cutover-2026-11-18.md | autoradio: a place-now dropped as duplicate_id airs the server's copy; an ahead  // autoradio: a kept story plays over its restored copy; a repeat dropped from an e // autoradio: extend at launch when the phone has its own opening; a dropped answer |
| android-probe | LIKELY-LANDED | 209 | 445 | 2026-11-18 | 97% |  | 0 | definitions/choir-autoradio-exact-cutover-2026-11-18.md, definitions/choir-autoradio-on-android-2026-11-22.md | probe: app.json names Android, as bun exact.mjs android wrote it (not for landin // autoradio: a file copied for a page goes to the folder the page reads; PDFs and  // autoradio ui: remove the Why this button from the radio page; tray glyphs carry  |
| exact-loop | LIKELY-LANDED | 209 | 448 | 2026-11-18 | 97% |  | 0 | definitions/choir-autoradio-exact-cutover-2026-11-18.md, definitions/choir-night-exact-vectors-learning-2026-11-19.md | autoradio: the mark compiles in a fifth of the time and a thirtieth of the plan; // autoradio: a file copied for a page goes to the folder the page reads; PDFs and  // autoradio ui: remove the Why this button from the radio page; tray glyphs carry  |
| exact-paper | LIKELY-LANDED | 209 | 457 | 2026-11-18 | 95% |  | 0 | definitions/choir-autoradio-exact-cutover-2026-11-18.md, definitions/choir-night-exact-vectors-learning-2026-11-19.md | autoradio: the Paper page's data, ranking and layout move from Swift to app.ts;  // autoradio: a file copied for a page goes to the folder the page reads; PDFs and  // autoradio ui: remove the Why this button from the radio page; tray glyphs carry  |
| autoradio-integ20 | LIKELY-LANDED | 208 | 445 | 2026-11-18 | 98% |  | 0 | definitions/choir-autoradio-exact-cutover-2026-11-18.md, design/autoradio-in-exact-2026-11-18.md, problems/autoradio-app-logic-is-swift-not-exact-2026-11-18.md | autoradio: a file copied for a page goes to the folder the page reads; PDFs and  // autoradio ui: remove the Why this button from the radio page; tray glyphs carry  // autoradio: one local keep for what a local-only entry may not post; neutral post |
| ui-morning | LIKELY-LANDED | 207 | 445 | 2026-11-18 | 97% |  | 0 |  | autoradio ui: remove the Why this button from the radio page; tray glyphs carry  // autoradio: one local keep for what a local-only entry may not post; neutral post // Merge remote-tracking branch 'origin/lane/fix-content' into HEAD |
| autoradio-integ19 | LIKELY-LANDED | 206 | 445 | 2026-11-18 | 96% |  | 0 | definitions/choir-autoradio-learning-loop-2026-11-18.md, problems/autoradio-first-listening-session-2026-11-18.md | autoradio: one local keep for what a local-only entry may not post; neutral post // Merge remote-tracking branch 'origin/lane/fix-content' into HEAD // Merge remote-tracking branch 'origin/lane/fix-player' into HEAD |
| fix-player3 | LIKELY-LANDED | 206 | 445 | 2026-11-18 | 96% |  | 0 |  | autoradio: one local keep for what a local-only entry may not post; neutral post // Merge remote-tracking branch 'origin/lane/fix-content' into HEAD // Merge remote-tracking branch 'origin/lane/fix-player' into HEAD |
| autoradio-integ18 | LIKELY-LANDED | 205 | 443 | 2026-11-18 | 95% |  | 0 |  | Merge remote-tracking branch 'origin/lane/fix-content' into HEAD // Merge remote-tracking branch 'origin/lane/fix-player' into HEAD // autoradio: the microphone check ignores a stale transcript, drops a spoken turn  |
| autoradio-integ17 | LIKELY-LANDED | 200 | 443 | 2026-11-18 | 93% |  | 0 | problems/autoradio-phone-code-unreviewed-for-three-days-2026-11-18.md | Merge remote-tracking branch 'origin/lane/fix-account' into HEAD // autoradio phone: one account generation; in-flight sync, desk stream, feed load, // autoradio: the meter type the onboarding microphone check drives exists (orange; |
| autoradio-integ16 | LIKELY-LANDED | 198 | 442 | 2026-11-18 | 90% |  | 0 | reference/AUTORADIO-TAKES.md | autoradio: the meter type the onboarding microphone check drives exists (orange; // Merge remote-tracking branch 'origin/lane/fix-account' into HEAD // autoradio phone: one account wipe at sign-out, background revoke, notebook creat |
| fix-content | LIKELY-LANDED | 195 | 437 | 2026-11-18 | 89% |  | 0 |  | autoradio: the microphone check ignores a stale transcript, drops a spoken turn  // autoradio: the meter type the onboarding microphone check drives exists (orange; // autoradio: the microphone check listens and can pass; feed notes keep nested tex |
| fix-account | LIKELY-LANDED | 194 | 440 | 2026-11-18 | 93% |  | 0 |  | autoradio phone: one account generation; in-flight sync, desk stream, feed load, // autoradio phone: one account wipe at sign-out, background revoke, notebook creat // Merge remote-tracking branch 'origin/lane/reducer-twin' into HEAD |
| fix-player | LIKELY-LANDED | 194 | 436 | 2026-11-18 | 87% |  | 0 |  | autoradio: a real pause inside the hold window is rechecked, a pause during a ho // autoradio: mail words stay on the phone, clip replay, previous in a lead-in, hol // Merge remote-tracking branch 'origin/lane/reducer-twin' into HEAD |
| autoradio-integ15 | LIKELY-LANDED | 192 | 435 | 2026-11-18 | 87% |  | 0 | current-architecture.md | Merge remote-tracking branch 'origin/lane/reducer-twin' into HEAD // autoradio takes: stop 0.5 s before the ceiling, claim clamped to 3..60 from the  // autoradio: the Swift run sheet reducer mirrors the server's takeover: an aired o |
| takes-wire | LIKELY-LANDED | 190 | 434 | 2026-11-18 | 87% |  | 0 | problems/autoradio-two-sides-of-a-wire-built-apart-2026-11-18.md | autoradio takes: stop 0.5 s before the ceiling, claim clamped to 3..60 from the  // autoradio takes: the phone matches the server's wire; publish is one multipart P // autoradio: a Take control beside Next starts the take flow, whose panel sits und |
| autoradio-integ14 | LIKELY-LANDED | 189 | 434 | 2026-11-18 | 86% |  | 0 |  | autoradio takes: the phone matches the server's wire; publish is one multipart P // autoradio: a Take control beside Next starts the take flow, whose panel sits und // mobile: the mark gallery is a known page, so the not-yet page no longer draws un |
| reducer-twin | LIKELY-LANDED | 189 | 433 | 2026-11-18 | 86% |  | 0 | problems/autoradio-two-sides-of-a-wire-built-apart-2026-11-18.md | autoradio: the Swift run sheet reducer mirrors the server's takeover: an aired o // autoradio: a Take control beside Next starts the take flow, whose panel sits und // mobile: the mark gallery is a known page, so the not-yet page no longer draws un |
| autoradio-integ13 | LIKELY-LANDED | 188 | 432 | 2026-11-18 | 85% |  | 0 |  | autoradio: a Take control beside Next starts the take flow, whose panel sits und // mobile: the mark gallery is a known page, so the not-yet page no longer draws un // integ: merge lane/learn-phone (orange; rollback: revert) |
| phone-polish | LIKELY-LANDED | 188 | 432 | 2026-11-18 | 85% |  | 0 |  | autoradio: a Take control beside Next starts the take flow, whose panel sits und // mobile: the mark gallery is a known page, so the not-yet page no longer draws un // integ: merge lane/learn-phone (orange; rollback: revert) |
| autoradio-integ12 | LIKELY-LANDED | 187 | 429 | 2026-11-18 | 85% |  | 0 |  | mobile: the mark gallery is a known page, so the not-yet page no longer draws un // integ: merge lane/learn-phone (orange; rollback: revert) // autoradio: the phone learns from listening: settle keys kept, crash is not leavi |
| autoradio-integ11 | LIKELY-LANDED | 186 | 429 | 2026-11-17 | 85% |  | 0 |  | integ: merge lane/learn-phone (orange; rollback: revert) // autoradio: the phone learns from listening: settle keys kept, crash is not leavi // integ: merge lane/page-motion (orange; rollback: revert) |
| autoradio-integ10 | LIKELY-LANDED | 184 | 409 | 2026-11-17 | 82% |  | 0 |  | integ: merge lane/page-motion (orange; rollback: revert) // integ: merge lane/tetramark-motion (orange; rollback: revert) // integ: merge lane/takes-phone (orange; rollback: revert) |
| learn-phone | STALLED-UNCLEAR | 179 | 412 | 2026-11-17 | 79% |  | 0 |  | autoradio: the phone learns from listening: settle keys kept, crash is not leavi // autoradio: takes on the phone: record, review, publish, list, delete, mute, my r // integ: merge lane/files-open (orange; rollback: revert) |
| integ9-fix | STALLED-UNCLEAR | 178 | 376 | 2026-11-17 | 77% |  | 0 | problems/autoradio-two-sides-of-a-wire-built-apart-2026-11-18.md | autoradio: the page module answers the shape the contract declares (builtOn on t // integ: merge lane/files-open (orange; rollback: revert) // integ: merge lane/radio-page2 (orange; rollback: revert) |
| page-motion | STALLED-UNCLEAR | 178 | 385 | 2026-11-17 | 79% |  | 0 |  | feat(mobile): the frame never leaves; a screen change animates from the tap, no  // integ: merge lane/files-open (orange; rollback: revert) // integ: merge lane/radio-page2 (orange; rollback: revert) |
| takes-phone | STALLED-UNCLEAR | 178 | 392 | 2026-11-17 | 76% |  | 0 |  | autoradio: takes on the phone: record, review, publish, list, delete, mute, my r // integ: merge lane/files-open (orange; rollback: revert) // integ: merge lane/radio-page2 (orange; rollback: revert) |
| tetramark-motion | STALLED-UNCLEAR | 178 | 384 | 2026-11-17 | 77% |  | 0 |  | autoradio: the tetramark moves: a pure motion model (3D projection, per-blade of // integ: merge lane/files-open (orange; rollback: revert) // integ: merge lane/radio-page2 (orange; rollback: revert) |
| autoradio-integ9 | STALLED-UNCLEAR | 177 | 376 | 2026-11-17 | 76% |  | 0 | definitions/choir-autoradio-learning-loop-2026-11-18.md | integ: merge lane/files-open (orange; rollback: revert) // integ: merge lane/radio-page2 (orange; rollback: revert) // autoradio: opening a file from Files: a library row without its original is fetc |
| autoradio-integ8 | STALLED-UNCLEAR | 173 | 372 | 2026-11-17 | 73% |  | 0 | design/autoradio-learned-document-2026-11-17.md | integ: merge lane/clip-relocate onto lane/voice-coexist (orange; rollback: rever // autoradio: voice on does not pause the radio (its own route change is not a lost // autoradio: the clip is located in a window centred on the fingerprint record's s |
| clip-relocate | STALLED-UNCLEAR | 171 | 371 | 2026-11-17 | 72% |  | 0 |  | autoradio: the clip is located in a window centred on the fingerprint record's s // autoradio: a podcast clip is found in the served file by its fingerprint and pla // mobile: Exact, the model libraries and the app are built in one tree per Mac (ye |
| files-open | STALLED-UNCLEAR | 170 | 368 | 2026-11-17 | 73% |  | 0 |  | autoradio: opening a file from Files: a library row without its original is fetc // mobile: Exact, the model libraries and the app are built in one tree per Mac (ye // autoradio: a quick relaunch plays what was left ahead at once; every finish and  |
| radio-page2 | STALLED-UNCLEAR | 170 | 368 | 2026-11-17 | 73% |  | 0 |  | autoradio: the time is computed when the app opens; no Places row, Built on inst // mobile: Exact, the model libraries and the app are built in one tree per Mac (ye // autoradio: a quick relaunch plays what was left ahead at once; every finish and  |
| voice-coexist | STALLED-UNCLEAR | 170 | 367 | 2026-11-17 | 73% |  | 0 |  | autoradio: voice on does not pause the radio (its own route change is not a lost // mobile: Exact, the model libraries and the app are built in one tree per Mac (ye // autoradio: a quick relaunch plays what was left ahead at once; every finish and  |
| build-one-place | STALLED-UNCLEAR | 169 | 366 | 2026-11-17 | 72% |  | 0 | definitions/choir-autoradio-learning-loop-2026-11-18.md | mobile: Exact, the model libraries and the app are built in one tree per Mac (ye // autoradio: a quick relaunch plays what was left ahead at once; every finish and  // autoradio: a session failure holds the entry and retries instead of failing it;  |
| audio-bg | STALLED-UNCLEAR | 168 | 365 | 2026-11-17 | 71% |  | 0 | calibration/claude-ledger.md | autoradio: a quick relaunch plays what was left ahead at once; every finish and  // autoradio: a session failure holds the entry and retries instead of failing it;  // autoradio: one floor per ranking source, stated where the retrievers meet (orang |
| autoradio-integ7 | STALLED-UNCLEAR | 168 | 365 | 2026-11-17 | 71% |  | 0 |  | autoradio: a quick relaunch plays what was left ahead at once; every finish and  // autoradio: a session failure holds the entry and retries instead of failing it;  // autoradio: one floor per ranking source, stated where the retrievers meet (orang |
| autoradio-integ6 | STALLED-UNCLEAR | 166 | 363 | 2026-11-17 | 70% |  | 0 |  | autoradio: one floor per ranking source, stated where the retrievers meet (orang // autoradio: vector search exists: the fused retriever, the embedding job wired to // autoradio: EmbeddingGemma 2 on device: the GGUF through llama.cpp b12259 runs on |
| embed-trial | STALLED-UNCLEAR | 166 | 363 | 2026-11-17 | 70% |  | 0 | definitions/choir-autoradio-overnight-2026-11-17.md | autoradio: one floor per ranking source, stated where the retrievers meet (orang // autoradio: vector search exists: the fused retriever, the embedding job wired to // autoradio: EmbeddingGemma 2 on device: the GGUF through llama.cpp b12259 runs on |
| autoradio-integ5 | STALLED-UNCLEAR | 163 | 327 | 2026-11-17 | 66% |  | 0 |  | integ: merge lane/rag-fix (orange; rollback: revert) // autoradio: one path for every turn: phrase matching removed (LocalCommands, Retr // autoradio: mail and local-only files are never sent to the server desk; a turn w |
| rag-fix | STALLED-UNCLEAR | 160 | 326 | 2026-11-17 | 67% |  | 0 |  | autoradio: one path for every turn: phrase matching removed (LocalCommands, Retr // autoradio: mail and local-only files are never sent to the server desk; a turn w // autoradio: an unnamed page address keeps the caller's on-air title (orange; roll |
| mac-proof2 | STALLED-UNCLEAR | 158 | 326 | 2026-11-17 | 64% |  | 0 |  | autoradio: proof-mac.sh for the Autoradio page; the launch line says when Play w // autoradio: the Say something row is removed (owner); the page stays below the st // autoradio: Play starts the want (the radio no longer stops after the opening whe |
| radio-page | STALLED-UNCLEAR | 157 | 326 | 2026-11-17 | 64% |  | 0 | definitions/choir-autoradio-overnight-2026-11-17.md | autoradio: the Say something row is removed (owner); the page stays below the st // autoradio: Play starts the want (the radio no longer stops after the opening whe // autoradio: one opening rule: an opening is placed only when none is on air and n |
| autoradio-integ4 | STALLED-UNCLEAR | 156 | 325 | 2026-11-17 | 64% |  | 0 | definitions/choir-autoradio-overnight-2026-11-17.md | autoradio: Play starts the want (the radio no longer stops after the opening whe // autoradio: one opening rule: an opening is placed only when none is on air and n // autoradio: demo test affordance AutoradioDemoSheet loads a bundled fixture run s |
| autoradio-integ3 | STALLED-UNCLEAR | 150 | 310 | 2026-11-17 | 62% |  | 0 |  | integ: merge lane/mac-app (orange; rollback: revert) // exact: the pin is a file; setup fetches it into an old clone; every build refuse // autoradio: offline, the stories held on the phone play after the cached opening  |
| instant-start | STALLED-UNCLEAR | 148 | 307 | 2026-11-16 | 62% |  | 0 | definitions/choir-autoradio-overnight-2026-11-17.md | autoradio: offline, the stories held on the phone play after the cached opening  // autoradio: the launch waits for its condition, not two seconds; the voice is war // autoradio: the opening is written and spoken before the launch that plays it; on |
| autoradio-integ2 | STALLED-UNCLEAR | 145 | 304 | 2026-11-16 | 62% |  | 0 |  | integ: merge lane/phone-retrieval; the generated apple/exact.json stays untracke // autoradio: the first Play no longer pauses what it just started; the Mac proof s // autoradio: every pause names who asked; no microphone request with voice off on  |
| mac-app | STALLED-UNCLEAR | 143 | 303 | 2026-11-17 | 62% |  | 0 | definitions/choir-autoradio-overnight-2026-11-17.md | exact: the pin is a file; setup fetches it into an old clone; every build refuse // autoradio: the first Play no longer pauses what it just started; the Mac proof s // autoradio: every pause names who asked; no microphone request with voice off on  |
| phone-retrieval | STALLED-UNCLEAR | 135 | 282 | 2026-11-16 | 61% |  | 0 | definitions/choir-autoradio-overnight-2026-11-17.md | autoradio: the content store opens off the launch path in a background task, a s // autoradio: one local content store: mail, stories, documents, files, podcasts an // autoradio: retrieval follow-ups: story bodies fetched in the background, mail/fi |
| autoradio-integ | STALLED-UNCLEAR | 133 | 278 | 2026-11-16 | 60% |  | 0 | definitions/choir-autoradio-overnight-2026-11-17.md | autoradio: retrieval follow-ups: story bodies fetched in the background, mail/fi // autoradio: the on-device model searches everything the phone holds with BM25 bef // autoradio: remote audio behind the single-voice gate with a test that names ever |
| voice-fullband | STALLED-UNCLEAR | 132 | 276 | 2026-11-16 | 60% |  | 0 | definitions/choir-autoradio-overnight-2026-11-17.md, definitions/choir-voice-pipeline-2026-11-23.md, problems/autoradio-voice-pipeline-not-the-one-chosen-2026-11-24.md | autoradio: full-band capture while the radio plays: the listener over the radio  // autoradio: remote audio behind the single-voice gate with a test that names ever // autoradio: one voice at a time: a voice line holds the programme and it goes on  |
| ar-app-repair | LIKELY-LANDED | 120 | 251 | 2026-11-11 | 100% |  | 0 |  | test(autoradio-app): proof-real lets the list settle after scrolling the second  // test(autoradio-app): proof-real scrolls the second story card clear of the bar b // test(autoradio-app): proof-real drags the front page to its top before tapping a |
| ar-app | LIKELY-LANDED | 116 | 248 | 2026-11-11 | 92% |  | 0 | calibration/claude-ledger.md, reference/EXACT.md, reports/autoradio-app-built-overnight-2026-11-11.md | fix(autoradio-app): real proof picks the first visible story and play button (th // fix(autoradio-app): touch is main-thread only (a background caller crashed); REA // fix(autoradio-app): typed ask answers on the existing LabState mutation (orange; |
| ar-app-real | LIKELY-LANDED | 116 | 248 | 2026-11-11 | 92% |  | 0 | reports/autoradio-app-built-overnight-2026-11-11.md | fix(autoradio-app): real proof picks the first visible story and play button (th // fix(autoradio-app): touch is main-thread only (a background caller crashed); REA // fix(autoradio-app): typed ask answers on the existing LabState mutation (orange; |
| exact-integ | LIKELY-LANDED | 110 | 264 | 2026-11-19 | 100% |  | 0 | design/autoradio-exact-integration-2026-11-19.md | host fact: a probe that finishes after the native signal can no longer write non // host fact, sign-out counter, shared deck saves: seat 2's six should-fixes on 301 // mobile: proof-real starts with a throwaway warm-up launch (the first launch afte |
| runsheet-phone | STALLED-UNCLEAR | 106 | 217 | 2026-11-16 | 53% |  | 0 |  | autoradio: views and host adapters onto the run sheet; Up next is ahead(3), the  // autoradio: the phone plays one run sheet; the opening comes first and the single // runsheet: the run sheet types, the reducer and the shared fixtures in Swift (ora |
| runsheet-swift | STALLED-UNCLEAR | 104 | 210 | 2026-11-16 | 50% |  | 0 |  | runsheet: the run sheet types, the reducer and the shared fixtures in Swift (ora // fix(autoradio): remove press-and-hold talk (Lab talk button, choirTalkDown/Up, l // fix(autoradio): voice is off unless the listener taps it; no launch, foreground  |
| ar-app-engine | STALLED-UNCLEAR | 103 | 243 | 2026-11-11 | 77% |  | 0 |  | feat(autoradio-app): engine round 2 complete: proven by proof-app.sh (216 Swift  // wip(autoradio-app): engine round 2: rate, render position, DeskTurn.interrupted, // docs(autoradio-app): integration notes: proof state, lane proofs after the merge |
| phone-on-main-land | LIKELY-LANDED | 57 | 107 | 2026-11-14 | 100% |  | 0 |  | docs(problems): the deck swipe stalls on Textures; deckdiag proof mode (green; r // merge: main into the phone lane so the app builds on current main (orange; rollb // fix(autoradio-app): drop the ReaderBlock alt field, the bridge shape is declared |
| phone-on-main | LIKELY-LANDED | 56 | 106 | 2026-11-14 | 100% |  | 0 |  | merge: main into the phone lane so the app builds on current main (orange; rollb // fix(autoradio-app): drop the ReaderBlock alt field, the bridge shape is declared // test(golden): known differences after the native fixes are web-side or by design |
| golden-docs | LIKELY-LANDED | 55 | 123 | 2026-11-11 | 96% |  | 0 | definitions/choir-autoradio-learning-engine-2026-11-14.md, definitions/choir-autoradio-pass1-2026-11-10.md | fix(autoradio-app): drop the ReaderBlock alt field, the bridge shape is declared // test(golden): known differences after the native fixes are web-side or by design // test(golden): the native proof also runs the parity and reader tests (yellow; ro |
| unified-voice-download | STALLED-UNCLEAR +DIRTY | 51 | 183 | 2026-11-24 | 75% | unified-voice-download | 22 |  | mobile: use pinned packs and offline loaders for Silero, Phonon and Parakeet (or // mobile: a lane's Exact loop links mobile/.exact-src to EXACT2, so @exact/passkey // Merge origin/main into lane/store-int (the reviewer sign-in, privacy policy, Z.A |
| dl-consent | LIKELY-LANDED +DIRTY | 50 | 178 | 2026-11-24 | 84% | dl-consent | 5 |  | mobile: a lane's Exact loop links mobile/.exact-src to EXACT2, so @exact/passkey // Merge origin/main into lane/store-int (the reviewer sign-in, privacy policy, Z.A // mobile: move-a after seat 2: an unreadable sheet is given up after a bound and k |
| tiny-voca-assets | LIKELY-LANDED | 50 | 178 | 2026-11-24 | 84% |  | 0 |  | mobile: a lane's Exact loop links mobile/.exact-src to EXACT2, so @exact/passkey // Merge origin/main into lane/store-int (the reviewer sign-in, privacy policy, Z.A // mobile: move-a after seat 2: an unreadable sheet is given up after a bound and k |
| tiny-voca-native | LIKELY-LANDED | 50 | 178 | 2026-11-24 | 84% |  | 0 |  | mobile: a lane's Exact loop links mobile/.exact-src to EXACT2, so @exact/passkey // Merge origin/main into lane/store-int (the reviewer sign-in, privacy policy, Z.A // mobile: move-a after seat 2: an unreadable sheet is given up after a bound and k |
| store-int | LIKELY-LANDED | 50 | 178 | 2026-11-24 | 84% |  | 0 |  | mobile: a lane's Exact loop links mobile/.exact-src to EXACT2, so @exact/passkey // Merge origin/main into lane/store-int (the reviewer sign-in, privacy policy, Z.A // mobile: move-a after seat 2: an unreadable sheet is given up after a bound and k |
| ar3-app-deck | LIKELY-LANDED | 49 | 101 | 2026-11-11 | 92% |  | 0 | definitions/choir-autoradio-pass1-2026-11-10.md | docs(problems): the deck swipe stalls on Textures; deckdiag proof mode (green; r // Revert: a revising Texture shows a still glyph (the spinner was not the cause of // test(autoradio-app): deckdiag compares the pages (yellow; rollback: revert) |
| ar3-app-repair | LIKELY-LANDED | 49 | 101 | 2026-11-11 | 99% |  | 0 |  | fix(autoradio-app): round 3 review repairs: Send after Cancel, hand-over race an // Revert: a revising Texture shows a still glyph (the spinner was not the cause of // test(autoradio-app): deckdiag compares the pages (yellow; rollback: revert) |
| corpus-brief | LIKELY-LANDED | 49 | 30 | 2026-11-18 | 93% |  | 0 | calibration/claude-ledger.md | tests, comments, docs: the late-call test drops comparisons that could never fai // docs: the searches run at the same time, a panic is a thread that did not answer // agentcore: a panic in a thread's search is that thread's did-not-answer; slowest |
| ar3-app | LIKELY-LANDED | 48 | 100 | 2026-11-11 | 92% |  | 0 |  | Revert: a revising Texture shows a still glyph (the spinner was not the cause of // test(autoradio-app): deckdiag compares the pages (yellow; rollback: revert) // test(autoradio-app): the proof fails if model weights ship in the built app (yel |
| move-a | LIKELY-LANDED | 48 | 242 | 2026-11-24 | 88% | move-a | 0 | calibration/claude-ledger.md | mobile: move-a after seat 2: an unreadable sheet is given up after a bound and k // mobile: the run sheet is restored by radioHold and kept as a value; the pulse re // docs: problem record: the radio starts empty after a quick relaunch (a let-go re |
| move-b | STALLED-UNCLEAR | 48 | 276 | 2026-11-24 | 59% | move-b | 0 |  | mobile: the clock and the app version from Exact and the build, not native (oran // mobile: no round trips through Swift for mail review, the reader's aim, the pick // mobile: one reader of "signed in", one key read per answer, the host fact from n |
| phone-int | LIKELY-LANDED | 45 | 227 | 2026-11-24 | 86% |  | 0 |  | mobile: share the launch settle and the host decision as values, not as a kept p // docs: problem record: one let-go answer fails every shared-code request on the p // Merge remote-tracking branch 'origin/lane/voice-models' into HEAD |
| event-shortlist | LIKELY-LANDED | 40 | 69 | 2026-11-19 | 91% |  | 0 | reference/EMBEDDINGGEMMA.md | corpusd: a refused capture is counted as its own reason and is never a neighbour // docs: the event layer row, after the rebase onto the vector index rows (green) // scripts: status-corpus-reuse reads op=event_flush: flushes, questions, joined, m |
| rev-purge | STALLED-UNCLEAR +DIRTY | 36 | 103 | 2026-11-24 | 75% | rev-purge | 1 |  | docs: purge-unproved, the tables it touches and the key of each; the change log  // operator: purge-unproved, a manual command that deletes the accounts whose addre // docs: the register-finish refusals as they are (green; rollback: revert) |
| ar-l8 | STALLED-UNCLEAR | 33 | 152 | 2026-11-11 | 56% |  | 0 |  | fix(autoradio-app): lane 8 proof waits longer for the radio and asserts the fron // fix(autoradio-app): edition buttons above the edition, clear of the bar (orange; // fix(autoradio-app): edition fixture fails the first write so Retry is proven; fi |
| ar-l6 | STALLED-UNCLEAR | 32 | 159 | 2026-11-11 | 58% |  | 0 |  | fix(autoradio-app): mail buttons are theme-coloured custom buttons (orange; roll // fix(autoradio-app): mail bordered buttons read in dark (orange; rollback: revert // fix(autoradio-app): mail proof: loading screen by screenshot, dark after the app |
| ar-l2 | STALLED-UNCLEAR | 31 | 155 | 2026-11-10 | 57% |  | 0 |  | fix(autoradio-app): paper proof switches to dark through Settings (green; rollba // fix(autoradio-app): paper proof scrolls back with in-screen drags (green; rollba // fix(autoradio-app): paper proof settles before edition taps (green; rollback: re |
| ar-l3 | STALLED-UNCLEAR | 31 | 167 | 2026-11-10 | 59% |  | 0 |  | docs(autoradio-app): lane 3 README (green; rollback: revert) // test(autoradio-app): side-note step uses the already open card (yellow; rollback // test(autoradio-app): proof order keeps the menu reachable (yellow; rollback: rev |
| rev-voicemore | LIKELY-LANDED +DIRTY | 29 | 121 | 2026-11-24 | 80% | rev-voicemore | 1 |  | phone: Phonon is in the voice download group (Settings > Downloads only, withhel // phone: 53 English voices from the packs' own metadata and LFM2.5-350M as a quick // phone round 6: a check that ran keeps its own words; only a failed load is expla |
| voice-more | LIKELY-LANDED | 29 | 121 | 2026-11-24 | 80% | voice-more | 0 |  | phone: Phonon is in the voice download group (Settings > Downloads only, withhel // phone: 53 English voices from the packs' own metadata and LFM2.5-350M as a quick // phone round 6: a check that ran keeps its own words; only a failed load is expla |
| ar-l5 | STALLED-UNCLEAR | 28 | 148 | 2026-11-10 | 57% |  | 0 |  | fix(autoradio-app): file grid as two flex columns (orange; rollback: revert) // fix(autoradio-app): file grid by flex-wrap, audio card margins, search title and // fix(autoradio-app): Files header compact, grid columns minmax, proof reaches dee |
| ar-l7 | STALLED-UNCLEAR | 28 | 154 | 2026-11-10 | 56% |  | 0 |  | feat(autoradio-app): border-box rows so cards do not overflow, left-aligned acti // feat(autoradio-app): settings and Activity scroll above the bar so every row is  // feat(autoradio-app): Talk and Swipes screens inject th again (orange; rollback:  |
| ar-l4 | STALLED-UNCLEAR | 27 | 148 | 2026-11-10 | 57% |  | 0 |  | fix(autoradio-app): opaque underlay for glass cards, border-box card text, stamp // fix(autoradio-app): drag start written as a value (actions read state as it was) // test(autoradio-app): retry when the touch runner is slow to start (yellow; rollb |
| ar-l1 | STALLED-UNCLEAR | 25 | 144 | 2026-11-10 | 53% |  | 0 |  | fix(autoradio-app): bar is idle until the radio has started; paused needs a star // fix(autoradio-app): bar error glyph is the alert role (orange; rollback: revert) // feat(autoradio-app): lane 1 bar states, expanded player, radio view, talk status |
| ar-l9 | STALLED-UNCLEAR | 24 | 145 | 2026-11-10 | 54% |  | 0 |  | fix(autoradio-app): drop keyframe 3D fins (axis refused) (orange; rollback: reve // feat(autoradio-app): pure-Contract TetraMark, alive wait, motion gallery (orange // feat(autoradio-app): spine proof, glass on the content node, virtual engine on t |
| ar-spine | STALLED-UNCLEAR | 22 | 142 | 2026-11-10 | 54% |  | 0 | reference/EXACT.md, reference/exact/build-and-ship.md | feat(autoradio-app): spine proof, glass on the content node, virtual engine on t // feat(autoradio-app): contract spine: page host, bar, layers, cards, themes bound // feat(autoradio-app): module dispatch table, per-lane interface fragments and lan |
| receipts-first-steps | STALLED-UNCLEAR | 22 | 62 | 2026-11-13 | 77% |  | 0 | problems/old-receipts-do-not-compact-2026-11-14.md | fix(receipts): the index migration runs in its own goroutine, not as a retrieval // perf(receipts): drop computer_event_append_receipt_id_idx, which has no reader ( // fix(checkpoints): the current head's receipt is found by (computer, sequence), t |
| podcast-clips | LIKELY-LANDED | 21 | 43 | 2026-11-15 | 100% |  | 0 | definitions/choir-autoradio-full-experience-overnight-2026-11-15.md, problems/podcast-clips-review-did-not-converge-structural-assessment-2026-11-16.md | docs(design): podcast clips: matching by hand, and automatic matching NOT BUILT  // chore(gates): scripts/gates runs check-jev-entry, with --base when the base ref  // refactor(podcast): the automatic match job and its gateway entry are removed; th |
| vec-inbox | LIKELY-LANDED | 19 | 28 | 2026-11-19 | 89% |  | 0 | design/corpus-first-retrieval-and-research-2026-11-18.md | corpusd vectors inbox: a file that waits is not work (the pass keeps its interva // corpusd vectors: a file that cannot be opened for a reason other than permission // corpusd vectors: one function maps an embedder outcome to one of three effects a |
| ar-integrate | STALLED-UNCLEAR | 18 | 77 | 2026-11-10 | 71% |  | 0 | reference/EXACT.md, reference/exact/audio.md, reference/exact/build-and-ship.md | Merge remote-tracking branch 'origin/lane/ar-models' into lane/ar-integrate // fix(autoradio-app): Trace is a pushed screen of the Components tab, and the proo // feat(mobile): model results table, phone measurement commands, LFM3 run on the M |
| stored-result | LIKELY-LANDED | 18 | 50 | 2026-11-13 | 88% |  | 0 |  | fix(store): a lifecycle command names the revision its batch writes instead of e // docs(computerevent): AppendSequentialRequests is an upper bound now that the bin // fix(store): read the appender's revalidation count through an atomic source, not |
| autoradio-lab | STALLED-UNCLEAR | 18 | 77 | 2026-11-10 | 71% |  | 0 | definitions/choir-autoradio-pass1-2026-11-10.md | Merge remote-tracking branch 'origin/lane/ar-models' into lane/ar-integrate // fix(autoradio-app): Trace is a pushed screen of the Components tab, and the proo // feat(mobile): model results table, phone measurement commands, LFM3 run on the M |
| exact-runsheet | STALLED-UNCLEAR | 18 | 72 | 2026-11-19 | 55% |  | 0 |  | runsheet: the signed-in requester over the seam and the sign-out wipe of the sha // Merge remote-tracking branch 'origin/lane/exact-signin' into exact-runsheet // edition: edition setup (topics, selection, speech, tree, finishing) in TypeScrip |
| arl-p1 | LIKELY-LANDED | 17 | 21 | 2026-11-15 | 100% |  | 0 | problems/p1-player-three-review-rounds-structural-assessment-2026-11-15.md | docs(autoradio): P1 texts left by the hand-over removal: README bullet, a test h // mobile: P1 round 3: the hand-over is removed; at the deadline the article plays  // mobile: P1 review round 3, tests first: the hand-over tests go, sign-out clears  |
| exact-files | STALLED-UNCLEAR | 16 | 93 | 2026-11-19 | 57% |  | 0 |  | tests: the Epub tests keep their helpers and read the demo book from the shared  // REGEN: interface and native ceiling after the Files page moved (yellow; rollback // files: Swift consumers of the removed Files service cut or fed another way (oran |
| exact-signin-sheet | STALLED-UNCLEAR | 16 | 36 | 2026-11-23 | 61% |  | 0 |  | fixup autoradio: TIMEOUT from begun is ignored (as the table says); the Cancel c // fixup ci: every mobile/scripts shell script the identity test reads selects the  // docs: the judgement on the two limits that remain, a superseded code approved, a |
| signin-sheet | STALLED-UNCLEAR | 16 | 36 | 2026-11-23 | 61% | signin-sheet | 0 | calibration/claude-ledger.md, changes.md | fixup autoradio: TIMEOUT from begun is ignored (as the table says); the Cancel c // fixup ci: every mobile/scripts shell script the identity test reads selects the  // docs: the judgement on the two limits that remain, a superseded code approved, a |
| exact-deck | STALLED-UNCLEAR | 15 | 94 | 2026-11-19 | 56% |  | 0 |  | REGEN: exact.json from the interface fragments, native ceiling after the deck (y // scripts: the deck's playback engine glue joins the voice-and-deck list (yellow;  // deck: the Swift deck under the run sheet player: journal of started and ended wi |
| jev-one-state-r2 | LIKELY-LANDED | 14 | 45 | 2026-11-14 | 98% |  | 0 | problems/jev-misuse-structural-assessment-2026-11-15.md | fix(jev): a gateway 413 halves at once; only deterministic statuses fail once; a // fix(jev): a deterministic failure fails once; comments and residuals say what th // fix(jev): failure handling independent of the vendor's reason, tested through th |
| native-op-log | LIKELY-LANDED | 14 | 137 | 2026-11-24 | 83% | native-op-log | 0 |  | mobile: a ledger of native calls and a shared-code heartbeat, to name the call a // mobile: New document is a button at the top right of Textures: the server's crea // mobile: the editor, seat 2 round 2: the wipe waits for the set-aside copy of an  |
| exact-settings | STALLED-UNCLEAR | 14 | 90 | 2026-11-19 | 66% |  | 0 |  | REGEN: exact.json and the native ceiling after the lab page, card names and acco // mobile: settings account in shared code; the dead card names and the swipe-meani // Merge remote-tracking branch 'origin/lane/exact-signin' into exact-settings |
| exact-editor | STALLED-UNCLEAR | 13 | 76 | 2026-11-24 | 50% | exact-editor | 0 |  | mobile: a document the phone cannot write exactly (an embed, a source_ref, sourc // docs: problem: the phone's Texture save builds body_doc by inference and the ser // mobile: the editor saves cited and embedded documents: sources travel with the r |
| exr-r | STALLED-UNCLEAR +DIRTY | 13 | 76 | 2026-11-24 | 50% | exr-stack | 12 |  | mobile: a document the phone cannot write exactly (an embed, a source_ref, sourc // docs: problem: the phone's Texture save builds body_doc by inference and the ser // mobile: the editor saves cited and embedded documents: sources travel with the r |
| restore-witness | LIKELY-LANDED | 13 | 37 | 2026-11-14 | 100% |  | 0 | problems/replay-of-de-differs-on-the-event-index-only-2026-11-14.md, problems/restore-cannot-finish-inside-its-ten-minute-request-2026-11-14.md | Merge origin/main into lane/restore-witness // docs(problems): rollback by order of rollback, diagnostics peak, probe residual  // fix(platform,selfdevprotocol,agentcore,store): serve stored checkpoints verbatim |
| arl-l1-fix | LIKELY-LANDED | 13 | 43 | 2026-11-15 | 98% |  | 0 |  | test(autoradio): programme rows test the checks they name; reference page and de // docs(autoradio): section 12 matches the L1 code, index status, bounds, residuals // fix(autoradio): L1 review round 1, replay streams only segments with a claim, de |
| exact-content | STALLED-UNCLEAR | 13 | 75 | 2026-11-19 | 54% |  | 0 |  | autoradio: takes, the notebook with its scorecard, and activity as shared TypeSc // Merge the signed-in seam of lane exact-signin; the provisional copy goes (yellow // autoradio: search over the local copy and the content store with its sync, as sh |
| inbox-retry | STALLED-UNCLEAR | 13 | 8 | 2026-11-19 | 75% |  | 0 | problems/first-vectors-file-refused-equal-text-under-another-title-2026-11-19.md | docs: the inbox rules for a short sample, the three-draw count and a pass that y // corpusd vectors: a running pass yields to a file in the inbox; a short draw coun // tests: the spin test counts the passes that try the waiter by their log line (ye |
| ar-models | STALLED-UNCLEAR | 12 | 69 | 2026-11-10 | 64% |  | 0 | reference/EXACT.md, reference/exact/build-and-ship.md, reference/exact/native-modules.md | feat(mobile): model results table, phone measurement commands, LFM3 run on the M // feat(mobile): model proof runs from a launch argument, proof-models script, xcfr // feat(mobile): on-device model stages (Silero, Parakeet, Kokoro, PocketTTS, LFM3- |
| arl-l1 | LIKELY-LANDED | 12 | 42 | 2026-11-15 | 83% |  | 0 | calibration/claude-ledger.md | docs(autoradio): section 12 matches the L1 code, index status, bounds, residuals // fix(autoradio): L1 review round 1, replay streams only segments with a claim, de // test(autoradio): L1 review round 1, failing tests for the repairs (red: tape and |
| exact-podcasts | STALLED-UNCLEAR | 12 | 93 | 2026-11-19 | 60% |  | 0 |  | REGEN: the interface and the native ceiling after podcasts and clips (yellow; ro // autoradio: podcasts and clips move to shared code; the Swift service, feed parse // autoradio: podcast and clip sources in shared code, first piece (orange; rollbac |
| exact-mail | STALLED-UNCLEAR | 11 | 91 | 2026-11-19 | 53% |  | 0 |  | REGEN: interface and native ceiling after the Mail page moved (yellow; rollback: // mail: the mailbox, messages, drafts and the phone's sealed copy move from Swift  // api seam: keep ChoirClient untouched so the native ceiling does not rise; REGEN: |
| exact-signedout | LIKELY-LANDED | 11 | 35 | 2026-11-19 | 97% |  | 0 | calibration/claude-ledger.md | autoradio: the signed-out card sits whole above the bar, is the paper's only sig // autopaper: the masthead date is the reader's local date, as the radio's is (oran // autoradio: every Sign in action is the account state's; a late handle, a refusal |
| ar-ui | STALLED-UNCLEAR | 10 | 56 | 2026-11-10 | 52% |  | 0 | design/autoradio-app-ui-2026-11-10.md | docs(design): Autoradio app plan: one bar, many pages; onboarding interview; dat // fix(mobile): a desk turn ends after quiet, discovery leaves the turn path, Syste // feat(mobile): the listener's app: sign-in screen, Player with a real programme,  |
| ar3-config | STALLED-UNCLEAR | 10 | 60 | 2026-11-11 | 75% |  | 0 |  | feat(autoradio-app): configurator docs; plain wording for a network floor (green // fix(autoradio-app): type-check fixes in the configurator (yellow; rollback: reve // fix(autoradio-app): isOnDisk awaits outside the autoclosure (yellow; rollback: r |
| search-gate | LIKELY-LANDED | 10 | 21 | 2026-11-19 | 90% |  | 0 | definitions/corpus-first-search-one-verb-2026-11-19.md | tests: the run-events test starts its three concurrent searches with fresh (yell // yaegikernel: a search started while another of the run is running is refused at  // yaegikernel: a refused SearchAll wording puts the unread pages in Results and a  |
| ar-choir-client | STALLED-UNCLEAR | 9 | 55 | 2026-11-10 | 51% |  | 0 |  | fix(mobile): a desk turn ends after quiet, discovery leaves the turn path, Syste // feat(mobile): the listener's app: sign-in screen, Player with a real programme,  // feat(mobile): Choir client stages (sign-in, System 1, front page, desk), stub-se |
| snapshot-by-replay-time | LIKELY-LANDED | 9 | 16 | 2026-11-14 | 81% |  | 0 | problems/restore-cannot-finish-inside-its-ten-minute-request-2026-11-14.md | fix(agentcore,snapshot): the production restore tape probes the host; partitions // test(agentcore,snapshot): the restore probe through the production tape type, fa // fix(snapshot,agentcore,platform,computerevent): the restore plan reaches the roo |
| exact-base | STALLED-UNCLEAR | 9 | 59 | 2026-11-19 | 51% |  | 0 | definitions/choir-night-exact-vectors-learning-2026-11-19.md, design/autoradio-exact-integration-2026-11-19.md | api seam: keep ChoirClient untouched so the native ceiling does not rise; REGEN: // api seam: authedJSON, authedBytes, SignedOutError over the stored sign-in, read  // scripts: the native-boundary ceiling falls to what the Paper move and the galler |
| exact-signin | STALLED-UNCLEAR | 9 | 59 | 2026-11-19 | 51% |  | 0 |  | api seam: keep ChoirClient untouched so the native ceiling does not rise; REGEN: // api seam: authedJSON, authedBytes, SignedOutError over the stored sign-in, read  // scripts: the native-boundary ceiling falls to what the Paper move and the galler |
| exact-takes | STALLED-UNCLEAR | 9 | 32 | 2026-11-20 | 77% |  | 0 | definitions/choir-exact-cutover-and-ontology-2026-11-20.md | autoradio: takes review 6: the account-layer line leaves this lane, the native m // autoradio: takes review 5 follow-ups: a mirror in flight at an account change is // autoradio: takes, fourth round: nothing is carried from the pre-release build; a |
| exact-reader | STALLED-UNCLEAR | 8 | 46 | 2026-11-21 | 77% |  | 0 | definitions/choir-exact-cutover-and-ontology-2026-11-20.md | mobile: the reader drive's text check reads the sentence runs under each block,  // mobile: the reader drive scrolls to the citation by its own frame, with a real t // mobile: HeldPages drops the generation floor (it never healed after a failed sig |
| ar-audio | STALLED-UNCLEAR | 7 | 53 | 2026-11-10 | 57% |  | 0 |  | docs(mobile): phone measurement status and how to drive the phone (green; rollba // feat(mobile): listening/talking audio states, session coordinator, virtual audio // feat(mobile): build-lab and sim-launch scripts, Exact app builds and launches on |
| capture-static-copy | LIKELY-LANDED | 7 | 29 | 2026-11-15 | 86% |  | 0 | design/capture-static-copy-2026-11-15.md | test(snapshot): the producer tests run the wired shape; cross-check errors keep  // feat(snapshot): capture witnesses and uploads a static copy, so the WAL pin last // test(snapshot): failure-list tests of the static-copy capture, seen failing, wit |
| default-themes | STALLED-UNCLEAR | 7 | 19 | 2026-11-11 | 79% |  | 0 |  | fix(autoradio-app): the Default card previews the default pair, not the chosen t // fix(web): a scheme change keeps a pending pack choice (orange; rollback: revert) // fix(autoradio-app): a bundled theme is never replaced, and Settings gets a Defau |
| embed-query | STALLED-UNCLEAR | 7 | 7 | 2026-11-19 | 57% |  | 0 | problems/search-waits-four-seconds-behind-the-embed-worker-2026-11-19.md | corpusd: log whether a query embedder is set; the slow-embedder test releases it // nix: comments say the bulk unit serves documents and the query unit serves searc // nix: the query unit runs the flags that were measured: two slots of 512 tokens,  |
| exact-onboarding-settings | STALLED-UNCLEAR | 7 | 50 | 2026-11-21 | 57% |  | 0 | changes.md | REGEN: native ceiling after the first-run move (yellow; rollback: revert) // Merge remote-tracking branch 'origin/main' into exact-onboarding-settings // autoradio: first run, Settings > Computer, Activity and About's version are shar |
| voice-pro | STALLED-UNCLEAR | 7 | 33 | 2026-11-23 | 52% | voice-pro | 0 | definitions/choir-voice-pipeline-2026-11-23.md | phone: setup's first transcriber row is Pro on this list; the test names both ro // docs: Fluid Speech Pro: clean-set word error re-run, per-process memory (green) // phone: Fluid Speech Pro as a recogniser row, recommended from the 8 GB class, Mi |
| decision-record | STALLED-UNCLEAR | 6 | 28 | 2026-11-24 | 57% | decision-record | 0 | definitions/choir-records-and-measurement-2026-11-23.md | platform: the front render test rescoring moves the right story: a thin story ne // platform: the front render test gives each scores write a later mtime so the cac // platform: the front render record: the page's ranked order, each score and its i |
| ar3-voice | STALLED-UNCLEAR | 6 | 55 | 2026-11-11 | 60% |  | 0 | design/autoradio-on-device-voice-configurations-2026-11-11.md | fix(autoradio-app): setup's total counts a shared model once (yellow; rollback:  // fix(autoradio-app): restore VoiceSetup.swift (a bad slice cut the file); the res // feat(autoradio-app): responder ladder ordered by the Mac sample runs; docs (oran |
| autoradio-desk | LIKELY-LANDED | 6 | 34 | 2026-11-11 | 97% |  | 0 | design/gateway-streaming-for-every-model-2026-11-11.md | fix(autoradio): multi-byte space in the forced cut, closing quotes after non-Lat // fix(autoradio): bind replay to the words, honour reasoning_off only where it bel // feat(autoradio): reasoning off for the fast role only, a cache-aware frame, and  |
| responder-omni | STALLED-UNCLEAR | 6 | 33 | 2026-11-23 | 64% | responder-omni | 0 | definitions/choir-voice-pipeline-2026-11-23.md | docs: LFM3-Omni-2B as a larger responder: what it is, the row, the Mac measureme // phone: LFM3-Omni-2B (text path, Q8_0) as a larger responder row for 8 GB phones; // phone: review fixes: the chosen listener reaches the running app, the live-words |
| receipts-step2-r3 | STALLED-UNCLEAR | 5 | 26 | 2026-11-14 | 54% |  | 0 |  | test(receipts): a fresh database has neither old index; the plan test loses a de // perf(receipts): build the partial kind index and drop the two old ones, one at a // feat(receipts): the backfill writes format 2 from its own progress row; it waits |
| requeue-left | STALLED-UNCLEAR | 5 | 17 | 2026-11-24 | 53% | requeue-left | 0 |  | wip: fmt // wip: older test follows the host re-queue // wip: requeue left captures |
| exact-edition-picks | STALLED-UNCLEAR | 4 | 30 | 2026-11-21 | 66% | exact-edition-picks | 0 | definitions/choir-exact-cutover-and-ontology-2026-11-20.md | autoradio: edition picks: no carry-over: the old Swift file is never read and no // autoradio: edition picks: a picks file written while the proof about the old fil // autoradio: edition picks review 1: adopt the notebook's own lines, decide the ca |
| ar3-mark | LIKELY-LANDED | 4 | 11 | 2026-11-11 | 91% |  | 0 |  | feat(autoradio-app): the mark is three independent blades between the flat symbo // feat(autoradio-app): the mark takes body, a bounded camera and a bolder dance (o // fix(autoradio-app): icon from app.json icons only; proof-lane9 follows the new m |
| inline-body-reader | STALLED-UNCLEAR | 4 | 7 | 2026-11-15 | 57% |  | 0 | calibration/claude-ledger.md | feat(tape): the reader accepts object bodies carried inline as JSON, store epoch // docs(design): inline bodies accepted in round 2, should-fixes taken (green) // docs(design): inline bodies, review round 1 repaired by subtraction (green) |
| jev-intake-flush | LIKELY-LANDED | 4 | 10 | 2026-11-10 | 90% |  | 0 |  | fix(intake): group commit, no timer; the poll waits for its labels and then anno // fix(intake): one flusher per runtime, no deadline shared across calls, a stop th // fix(intake): the flush survives a panic, drains on a graceful stop, and asks at  |
| receipts-step2 | STALLED-UNCLEAR | 4 | 22 | 2026-11-14 | 59% |  | 0 |  | perf(receipts): build the partial kind index and drop the two old ones, one at a // feat(receipts): the backfill writes format 2 and revisits rows the canonical cod // perf(receipts): readers are index-backed with either kind index; head receipts a |
| exact-demo-off | LIKELY-LANDED | 4 | 15 | 2026-11-20 | 87% |  | 0 | definitions/choir-exact-search-entities-2026-11-20.md | mobile: the demo cleanup needs a positive trace, never removes real state, and m // mobile: the demo cleanup runs once on any phone, whatever the stored flag was (o // mobile: the Demo mode row is gone, and a phone that was in demo mode loses what  |
| exact-x12 | STALLED-UNCLEAR | 4 | 48 | 2026-11-19 | 55% |  | 0 | calibration/claude-ledger.md | autoradio: the motion gallery, mark gallery and wait pages are deleted (orange;  // docs: the pattern as built, as a recipe for the next page (green; rollback: reve // autoradio: the Paper page's data, ranking and layout move from Swift to app.ts;  |
| acctdel-land | LIKELY-LANDED | 3 | 111 | 2026-11-24 | 89% | acctdel-land | 0 | changes.md | protect the reviewer at the deletion and purge boundary (red; rollback: revert) // mobile, policy: the App Review account's Delete page says it cannot be deleted i // account deletion inside the app and on choir.news: 72-hour hold, cancel at sign- |
| receipts-step2a | STALLED-UNCLEAR | 3 | 26 | 2026-11-14 | 65% |  | 0 | problems/old-receipts-do-not-compact-2026-11-14.md | feat(receipts): the backfill writes format 2 from its own progress and restarts  // perf(receipts): readers are index-backed with either kind index; head receipts a // feat(receipts): every reader expands format 2, the jsonb-spelled compact form; n |
| receipts-step2b | STALLED-UNCLEAR | 3 | 21 | 2026-11-14 | 76% |  | 0 |  | feat(receipts): scripts/receipts-index-swap, the supervised swap of the live tab // perf(receipts): readers are index-backed with either kind index; head receipts a // feat(receipts): every reader expands format 2, the jsonb-spelled compact form; n |
| native-dead-code | STALLED-UNCLEAR | 3 | 45 | 2026-11-22 | 78% |  | 0 |  | mobile: restore the account-generation tests of take-session, drop stale proof c // mobile: delete the stores written and never read (NotebookCache, TakeOwn/TakeMut // mobile: delete dead non-voice Swift (RemoteSystem1, RemoteProgramme, RouteStretc |
| f0-desk-on-calls | LIKELY-LANDED | 2 | 42 | 2026-11-22 | 100% |  | 0 |  | gateway: the desk and the run on every model call's log line, guest and provider // gateway: the desk and the run on every model call's log line, guest strings in t |
| exact-dead-code | STALLED-UNCLEAR | 2 | 43 | 2026-11-22 | 72% |  | 0 | calibration/claude-ledger.md | mobile: delete the stores written and never read (NotebookCache, TakeOwn/TakeMut // mobile: delete dead non-voice Swift (RemoteSystem1, RemoteProgramme, RouteStretc |
| mac-arm-guest | LIKELY-LANDED | 2 | 2 | 2026-10-31 | 100% |  | 0 | definitions/choir-lean-system-2026-10-30.lanes.md, definitions/choir-lean-system-2026-10-30.ledger.md, definitions/choir-mac-sync-stage2-2026-11-01.md | nix: aarch64 guest skips pdfplumber's checks so duckdb is not compiled from sour // nix: aarch64 guest image for the Mac (Apple Virtualization), layered base/app er |
| mgmt-stories-fixes | STALLED-UNCLEAR | 2 | 8 | 2026-11-19 | 75% |  | 0 |  | docs: the paging bound and the shown count are fixed in both records (green) // management: story cursor fits the broker bound; story choice line counts the ope |
| ship-asc | STALLED-UNCLEAR | 2 | 4 | 2026-11-24 | 50% | ship-asc | 0 | reports/last-day-on-the-metamission-2026-11-24.md | docs: model files are recommended and agreed in voice setup, Settings is for try // docs: final App Store texts, review notes, privacy answer sheet and the submissi |
| autoradio-handsfree | STALLED-UNCLEAR | 1 | 21 | 2026-11-15 | 52% |  | 0 |  | feat(autoradio): hands-free voice turns, the engine's turn policy drives the mic |
| bundle-authenticity | STALLED-UNCLEAR | 1 | 19 | 2026-10-31 | 63% |  | 0 | definitions/choir-lean-system-2026-10-30.lanes.md, definitions/choir-lean-system-2026-10-30.ledger.md, design/bundle-authenticity-2026-10-31.md | bundle authenticity: verify checks every receipt signature against the importing |

### Landed lanes (nothing left; safe to remove, after the owner agrees)

134 names. With a lane tree still on disk: android-auth, android-embed-bench, android-model-research, android-model-runtime, android-parity-matrix, android-web-proof, dev-profile, dev-profile-fix, disk-leftovers, disk-report, drop-zai, exact-rest-a, exr-fix2, intake-starve, onboard-consent, privacy-v2, rev-resta, rev-store5, spacer-out, store-build, store-by-kind, sweep-fix, tester-capacity, voice-default.

Remote branches of landed lanes: 49 refs.

android-auth, android-embed-bench, android-model-research, android-model-runtime, android-parity-matrix, android-web-proof, appender-wedge, ar-server, ar-system1-route, arl-l1a, autoradio-review-investigation, boot-closure-batch, brief-objective, capture-bodies-gc-r2, capture-fulltext, capture-fulltext-2, capture-search-latency, capture-trigger, capture-trigger-design, ci-no-apt, csl-land, data-volume-observability, design-rules-fix, dev-profile, dev-profile-fix, disk-leftovers, disk-report, dispatch-backlog, drop-zai, e1-join-read, embed-kick, enter-newline, entity-e2, event-envelope-record, event-land, exact-design-rules, exact-rest-a, exr-fix2, fix-ackcount, flake-fix, gates-cover-ci, gateway-no-inference-bucket, golden-web, graph-first, heard-memory, inline-body-design, intake-starve, jev-site-scorer, keenable-auth, l5-gates, land-03e618ecfb, land-5ed8870bdc, land-6010ac62ec, land-789267adfb, land-876cb5416f, land-9ae1512b45, land-b2ffe0cb93, land-b485c160bb, land-b6bb8a22da, land-c0850dc468, land-cbba69e7ad, land-d35a2284e2, land-f26f70b30d, land-shard-timeout, last-switches, late-take, late-take-land, layered-fleet, learn-notebook, onboard-consent, opp-goal, paging-skip, phone-squash-land, privacy-v2, programme-opening, prune-at-boot, publish-load-502, ranked-search, receipts-step1, receipts-step2ab, red-main-records, research-frame, retire-census, retire-r1, rev-resta, rev-store5, rewrite-counters, rewrite-counters-land, runsheet-server, s4-design, s4-design-r2, s4-roots-prune, s5-coalesce, s7-sweep-schedule, search-cooldown, search-for-need, search-gauge, search-leads, simslim, snapshot-loop, snapshot-one-pass, snapshot-root-verifies-chunks, spacer-out, status-gaps, status-scripts, storage-guards, store-build, store-by-kind, store-churn, store-only, store-only-land, store-rank, store-size, store-size-land, stored-result-step1, stored-result-step2, sweep-fix, sweep-survives-restart, sweep-walk-digest-index, sweep-walk-one-view, switches-land, system1-route-fix, t2-list-not-stored, t2-run-write-counter, takes-server, tape-write-counters, tester-capacity, vec-core, vec-go, vec-import, vec-land, vec-train, vectors-pg, voice-default

## B. Missions (docs/definitions on origin/main)

Files: 74 (the .ledger.md files excluded). `now.status` counts: working 27, checkpoint_incomplete 16, superseded 8, no now block 7, complete 3, blocked_incomplete 2, reviewed 2, checkpoint 2, blocked 2, ready_for_fp0 1, checkpoint_complete 1, waiting_on_fp0 1, done 1, waiting_for_FP0 1.

Complete (complete, checkpoint_complete, done): 5. Superseded: 8. Working: 27. Working and stale (no substantive commit in more than 5 days): 15. Live-status missions (working, checkpoint_incomplete, blocked_incomplete, observing): 45.

Date caveat: the commit "a first line of standing on 87 governing, superseded and undecided documents" (2026-11-27) touched many mission files without changing them. The column "last substantive commit" skips that commit; "last commit (raw)" is `git log -1`. Staleness uses the substantive date. Station statuses are as recorded in each file's `stations:` list, which many files do not keep current; "stations not landed" counts every station whose status is not landed, proved, superseded or complete, so it is an upper bound on what is open.

### Live-status missions

| mission | status | listed in ACTIVE | in mission-graph | stations total / not landed | last substantive commit (age days) | last commit raw | stale while working | next_action | blocker_or_risk |
|---|---|---|---|---|---|---|---|---|---|
| choir-autoradio-on-android-2026-11-22 | working | yes | yes | 8 / 7 | 2026-11-27 (0) | 2026-11-27 |  | freeze and review the android-auth candidate with two seats; after the lead lands and deploys it, run scripts/proofs/android-a1-account-boundary.py in qa-mac with the owner present for the biometric action | the system passkey/biometric prompt may not be drivable by adb and the Motorola had no enrolled fingerprint/face at the A0 read; A1 is locally built but not deployed or device-proved; CoreML model bun |
| choir-campaign-2026-11-27 | working | yes | yes | 2 / 2 | 2026-11-27 (0) | 2026-11-27 |  | work K1; when its landing stations are proved, author the goal file for finishing the move to Exact on the latest Exact | none |
| choir-docs-transform-2026-11-27 | working | yes | yes | 12 / 6 | 2026-11-27 (0) | 2026-11-27 |  | read seat 2 on T4 and land it or send it back; then T5 (gates are the cost, not suites; a docs-only move of main must not re-run them) and T11 (one file per gate check); follow-ups are the doccheck CI job and the three readers of  | a docs push to main while a landing runs makes the lander run the gates twice (seen on T3: 474 s); three lanes in a row conflicted on one spot in scripts/gates. Both go to T5 and T11. |
| choir-standing-metamission-2026-11-22 | working | yes | yes | 9 / 9 | 2026-11-25 (2) | 2026-11-27 |  | land each phone lane as its review accepts, one at a time with its deployed proof; merge the landed phone lanes into the build tree, one native build, install on the owner's phone with a QA list, first archive and upload (submissi |  |
| choir-shipping-the-app-2026-11-23 | working | yes | yes | 15 / 14 | 2026-11-24 (3) | 2026-11-27 |  | land each lane as its review accepts, one at a time with its deployed proof: capacity, passkey, voice-default, onboarding, exact-rest, demo account, account deletion, store build; then merge into the build tree, one native build,  |  |
| choir-autoradio-in-the-desktop-2026-11-23 | working | yes | yes | 3 / 3 | 2026-11-23 (4) | 2026-11-27 |  | nothing until the owner says |  |
| choir-exact-cutover-and-ontology-2026-11-20 | working | yes | yes | 27 / 18 | 2026-11-23 (4) | 2026-11-27 |  | land exact-rest when accepted; run X9 in the first device build; start X10 when voice-default and onboard-consent have landed |  |
| choir-first-design-2026-11-22 | working | yes | yes | 13 / 8 | 2026-11-23 (4) | 2026-11-27 |  | review G1 and G2 with two seats each when reported; start G3; choose G4's context recipe from E5; keep shipping (M7) in the landing slot |  |
| choir-operations-podcasts-and-providers-2026-11-23 | working | yes | yes | 4 / 4 | 2026-11-23 (4) | 2026-11-27 |  | wait for the plan; O2 when the owner gives the numbers |  |
| choir-records-and-measurement-2026-11-23 | working | yes | yes | 9 / 6 | 2026-11-23 (4) | 2026-11-27 |  | settle C232 to C243; pick the capture desk's first context recipe from E5; rerun E4's burst-without-copies form on another day, precommitted |  |
| choir-storage-efficiency-2026-11-22 | working | yes | yes | 12 / 9 | 2026-11-23 (4) | 2026-11-27 |  | two-seat review of lane disk-leftovers; show the owner the S11 dry-run list after deploy; review S3 when its builder reports |  |
| choir-voice-pipeline-2026-11-23 | working | yes | yes | 12 / 12 | 2026-11-23 (4) | 2026-11-27 |  | land V2 when its review accepts; the first device build proves Fluid Speech Mini and enrolment on the owner's phone; recordings from that phone with the radio playing are the first real-condition accuracy measure |  |
| choir-exact-search-entities-2026-11-20 | working | yes | yes | 15 / 9 | 2026-11-20 (7) | 2026-11-27 | STALE | settle C176 after 16:30Z from the paper's run lines and the gateway's search counts; land the themes if the bounded fix is accepted; the owner's four decisions (snapshot budget, root desk, editor, owner acts) |  |
| choir-autoradio-exact-cutover-2026-11-18 | working | yes | yes | 17 / 15 | 2026-11-19 (8) | 2026-11-19 | STALE | read the reports of lanes exact-loop, exact-paper and phone-main; build each in the build tree; settle C125 and C126; land K3 | the owner's iPhone is away, so nothing about sound or a locked screen can be checked today; two builders share one Contract compiler, which takes a minute a run until X1 lands; the phone work is not o |
| choir-night-exact-vectors-learning-2026-11-19 | working | yes | yes | 15 / 15 | 2026-11-19 (8) | 2026-11-27 | STALE | start the first wave of builders (Exact lanes rebased and measured, the boundary gate, the trace read, pgvector and the embedder) and land the faster corpus search | the builder machine has 4 GB, so Go builders run two at a time; every server landing takes about 75 minutes of suites and they run one at a time, so the twelve hours hold eight or nine; the Mac carrie |
| choir-autoradio-overnight-2026-11-17 | working | yes | no | 9 / 9 | 2026-11-17 (10) | 2026-11-17 | STALE | device check of fbf7e84f8 with the console attached (play, background, return; ledger C118), which needs the phone unlocked; then review and land the mobile lanes; the phone embedding model download and first vector query; the ful | the installed build is unheard; a launch speaks at once, so the lead did not launch it at night; rollback is lane/autoradio-integ 5a7813642 |
| choir-autoradio-full-experience-overnight-2026-11-15 | working | yes | no | 9 / 9 | 2026-11-16 (11) | 2026-11-16 | STALE | a second matching pass near 09:30Z, the morning report, then the owner's decisions: the Jev entry point for automatic matching, where the worker and its key live |  |
| choir-autoradio-learning-engine-2026-11-14 | working | yes | yes | 9 / 9 | 2026-11-15 (12) | 2026-11-15 | STALE | > | > |
| choir-storage-solutions-3-store-once-2026-11-14 | working | yes | yes | 8 / 6 | 2026-11-15 (12) | 2026-11-15 | STALE | land the run-write counter and read it on the paper (C101); choose T2's mechanism from that reading; read what choir.event writes (17% of tape bytes). T1 is parked after three review rounds; carried messages are 12.8% of tape byte | > |
| choir-modular-architecture-2026-11-04 | working | yes | yes | 9 / 9 | 2026-11-14 (13) | 2026-11-14 | STALE | >- | >- |
| choir-storage-solutions-2-2026-11-12 | working | yes | yes | 10 / 5 | 2026-11-13 (14) | 2026-11-13 | STALE | > | > |
| choir-autopaper-overnight-2026-11-09 | working | yes | yes | 6 / 6 | 2026-11-09 (18) | 2026-11-09 | STALE | >- | >- |
| choir-selfdev-apps-feature-complete-2026-11-03 | working | yes | yes | 0 / 0 | 2026-11-03 (24) | 2026-11-03 | STALE | land S1's builds as each passes review; start the luna and pool arms; then the first Files round on all arms | >- |
| choir-t1a-app-self-development-2026-10-26 | working | yes | yes | 0 / 0 | 2026-11-02 (25) | 2026-11-27 | STALE | >- | >- |
| choir-launch-fp7-gateway-identity-2026-10-21 | working | no | no | 0 / 0 | 2026-10-30 (28) | 2026-10-30 | STALE | >- | >- |
| choir-lp-s6-commit-gate-2026-10-20 | working | yes | yes | 0 / 0 | 2026-10-30 (28) | 2026-10-30 | STALE | >- | >- |
| choir-launch-fp3-host-floor-2026-10-21 | working | no | no | 0 / 0 | 2026-10-21 (37) | 2026-10-21 | STALE | FP0 re-reviews group 1 round 3; groups 2 and 3 after FP0 sends the lane back. | >- |
| choir-autoradio-learning-loop-2026-11-18 | checkpoint_incomplete | yes | yes | 10 / 10 | 2026-11-18 (9) | 2026-11-18 |  | none here. Mission incomplete; checkpoint landed. The owner cleared this goal on 2026-11-18 and opened docs/definitions/choir-autoradio-exact-cutover-2026-11-18.md, which carries the open work: K1 (the iPhone repairs), K2 (the not | the phone lanes (about 150 commits) are not on main and were first reviewed on 2026-11-18 (docs/problems/autoradio-phone-code-unreviewed-for-three-days-2026-11-18.md); TestFlight needs the owner's pai |
| choir-selfdev-readiness-metamission-2026-10-24 | checkpoint_incomplete | yes | yes | 19 / 17 | 2026-11-14 (13) | 2026-11-14 |  | >- | >- |
| choir-autoradio-pass1-2026-11-10 | blocked_incomplete | yes | yes | 0 / 0 | 2026-11-11 (16) | 2026-11-11 |  | > | > |
| choir-launch-fp10-net-05-2026-10-21 | checkpoint_incomplete | no | no | 0 / 0 | 2026-11-09 (18) | 2026-11-09 |  | FP0 freezes and reviews 953836789 in the next bunch; lands it (CI only, inert) | >- |
| choir-launch-fp10-net10-2026-10-22 | checkpoint_incomplete | no | no | 0 / 0 | 2026-11-09 (18) | 2026-11-09 |  | >- | >- |
| choir-lean-system-2026-10-30 | checkpoint_incomplete | yes | yes | 0 / 0 | 2026-10-31 (27) | 2026-10-31 |  | >- | >- |
| choir-launch-fp10-arch-08-2026-10-21 | checkpoint_incomplete | no | no | 0 / 0 | 2026-10-30 (28) | 2026-10-30 |  |  |  |
| choir-launch-fp10-net-03-04-2026-10-21 | checkpoint_incomplete | no | no | 0 / 0 | 2026-10-30 (28) | 2026-10-30 |  | >- | >- |
| choir-launch-fp10-res-20-2026-10-21 | checkpoint_incomplete | no | no | 0 / 0 | 2026-10-30 (28) | 2026-10-30 |  | >- | >- |
| choir-launch-fp2-desk-boundary-2026-10-21 | checkpoint_incomplete | no | no | 0 / 0 | 2026-10-30 (28) | 2026-10-30 |  | >- | >- |
| choir-v-boot-vocab-pass-removal-2026-10-27 | checkpoint_incomplete | no | no | 0 / 0 | 2026-10-28 (30) | 2026-10-28 |  | observe b@c's next start; then settle complete |  |
| choir-launch-fp10-arch-09-2026-10-21 | checkpoint_incomplete | no | no | 0 / 0 | 2026-10-22 (36) | 2026-10-22 |  |  |  |
| choir-launch-fp10-arch-11-12-2026-10-21 | checkpoint_incomplete | no | no | 0 / 0 | 2026-10-22 (36) | 2026-10-22 |  | >- | >- |
| choir-launch-fp10-net01-host-fetch-2026-10-21 | checkpoint_incomplete | no | no | 0 / 0 | 2026-10-22 (36) | 2026-10-22 |  | FP0 reviews 07f34670b and 9fba08b0b (red; were 2a73810d4 and 759544b18) with the batch panel, lands, runs the proof steps | >- |
| choir-launch-fp10-sec14-deploy-user-2026-10-21 | checkpoint_incomplete | no | no | 0 / 0 | 2026-10-22 (36) | 2026-10-22 |  | FP0 rebases the four floor commits into the deploy-1 group and cc944c3f8 into the step-3 group, re-reviews the delta, lands deploy 1 and runs the probe | >- |
| choir-launch-fp6-network-2026-10-21 | checkpoint_incomplete | no | no | 0 / 0 | 2026-10-22 (36) | 2026-10-22 |  | >- | >- |
| choir-launch-fp10-arch-06-2026-10-21 | blocked_incomplete | no | no | 0 / 0 | 2026-10-21 (37) | 2026-10-21 |  | after both land, rebase this lane and build S1 (tests first) | FP2 and RES-17 not on main; both edit the pool |
| choir-launch-fp10-net02-mirrors-2026-10-21 | checkpoint_incomplete | no | no | 0 / 0 | 2026-10-21 (37) | 2026-10-21 |  | FP0 sends the red commits to a panel, then lands and proves them in the order under "Landing and proof | >- |

### Stations not landed, by mission (live-status missions with station lists)

| mission | station | name | status |
|---|---|---|---|
| autoradio-on-android-2026-11-22 | A0 | Physical baseline and parity inventory | working |
| autoradio-on-android-2026-11-22 | A2 | Kotlin deck and background audio | pending |
| autoradio-on-android-2026-11-22 | A3 | Android platform shell | working |
| autoradio-on-android-2026-11-22 | A4 | Android mail secrecy | pending |
| autoradio-on-android-2026-11-22 | A5 | Model portability matrix and old-phone benchmark | pending |
| autoradio-on-android-2026-11-22 | A6 | Kotlin hands-free voice and model execution | pending |
| autoradio-on-android-2026-11-22 | A7 | Full parity proof and Autoradio 1 APK | pending |
| campaign-2026-11-27 | K1 |  | working |
| campaign-2026-11-27 | K2 |  | working |
| docs-transform-2026-11-27 | T4 | the batched queue | working |
| docs-transform-2026-11-27 | T5 | heavy suites run once | pending |
| docs-transform-2026-11-27 | T8 | the loaded file | pending |
| docs-transform-2026-11-27 | T9 | one plan | pending |
| docs-transform-2026-11-27 | T10 | what exists, in one place | pending |
| docs-transform-2026-11-27 | T11 | checks named for what they protect | pending |
| standing-metamission-2026-11-22 | M1 |  | working |
| standing-metamission-2026-11-22 | M2 |  | working |
| standing-metamission-2026-11-22 | M3 |  | working |
| standing-metamission-2026-11-22 | M4 |  | working |
| standing-metamission-2026-11-22 | M5 |  | working |
| standing-metamission-2026-11-22 | M6 |  | working |
| standing-metamission-2026-11-22 | M7 |  | working |
| standing-metamission-2026-11-22 | M8 |  | pending |
| standing-metamission-2026-11-22 | M9 |  | pending |
| shipping-the-app-2026-11-23 | P1 | A build Apple accepts | working |
| shipping-the-app-2026-11-23 | P2 | The first run, signed out | working |
| shipping-the-app-2026-11-23 | P3 | Native passkey account creation and sign-in, and account deletion in the app | working |
| shipping-the-app-2026-11-23 | P4 | Consent, permissions and models | working |
| shipping-the-app-2026-11-23 | P5 | Spoken onboarding | working |
| shipping-the-app-2026-11-23 | P6 | Screenshots, previews and the reviewer's notes | working |
| shipping-the-app-2026-11-23 | P7 | External testers, as soon as possible | pending |
| shipping-the-app-2026-11-23 | P8 | The first version submitted | pending |
| shipping-the-app-2026-11-23 | P9 | The release an agent runs | pending |
| shipping-the-app-2026-11-23 | P10 | Computers for many testers | working |
| shipping-the-app-2026-11-23 | P11 | The rest of the Exact cutover, before the first build | working |
| shipping-the-app-2026-11-23 | P12 | A demo account for Apple's reviewers | working |
| shipping-the-app-2026-11-23 | P13 | Test accounts deleted and usage watched | pending |
| shipping-the-app-2026-11-23 | P14 | Thirty days of host logs, enforced | pending |
| autoradio-in-the-desktop-2026-11-23 | W0 | Can Exact live in a desktop window | pending |
| autoradio-in-the-desktop-2026-11-23 | W1 | Speech in the browser | pending |
| autoradio-in-the-desktop-2026-11-23 | W2 | The desktop app, listening only | pending |
| exact-cutover-and-ontology-2026-11-20 | X0 | Exact moved to upstream | working |
| exact-cutover-and-ontology-2026-11-20 | X1 | Edition picks and takes in shared code, no carry-over | working |
| exact-cutover-and-ontology-2026-11-20 | X2 | The About page's Build row | working |
| exact-cutover-and-ontology-2026-11-20 | X3 | The reader | working |
| exact-cutover-and-ontology-2026-11-20 | X4 | The bar and the run sheet page models | working |
| exact-cutover-and-ontology-2026-11-20 | X5 | Mail and files leftovers and the open-pages set | working |
| exact-cutover-and-ontology-2026-11-20 | X6 | Imported documents (the code's "shelf"), onboarding and settings models | working |
| exact-cutover-and-ontology-2026-11-20 | X7 | The Texture editor on what Exact's editor already holds | working |
| exact-cutover-and-ontology-2026-11-20 | X8 | What is left native, counted and justified | working |
| exact-cutover-and-ontology-2026-11-20 | P1 | Stories that cannot publish | working |
| exact-cutover-and-ontology-2026-11-20 | R1 | Operations | working |
| exact-cutover-and-ontology-2026-11-20 | X9 | The device probe | pending |
| exact-cutover-and-ontology-2026-11-20 | X10 | Model configuration and choice in Exact | pending |
| exact-cutover-and-ontology-2026-11-20 | X11 | Speaker scoring, enrolment and kept turns in Exact | pending |
| exact-cutover-and-ontology-2026-11-20 | X12 | Turn policy and hands-free state in Exact | pending |
| exact-cutover-and-ontology-2026-11-20 | X13 | The turn pipeline in Exact | pending |
| exact-cutover-and-ontology-2026-11-20 | X14 | Deck rules and remote audio in Exact | pending |
| exact-cutover-and-ontology-2026-11-20 | X15 | The lab harness retired | pending |
| first-design-2026-11-22 | F0 | The desk on every model call | pending |
| first-design-2026-11-22 | G1 | The spacer and the serial switch are removed | working |
| first-design-2026-11-22 | G2 | The corpus in the REPL | working |
| first-design-2026-11-22 | G3 | Creating an RLM is a protocol act with a prediction | pending |
| first-design-2026-11-22 | G4 | The capture desk in shadow | pending |
| first-design-2026-11-22 | G5 | The capture desk takes over | pending |
| first-design-2026-11-22 | G6 | The root as designed | pending |
| first-design-2026-11-22 | F6 | The readers | pending |
| operations-podcasts-and-providers-2026-11-23 | O0 | The podcast worker on a durable host | pending |
| operations-podcasts-and-providers-2026-11-23 | O1 | Routing by reported headroom | pending |
| operations-podcasts-and-providers-2026-11-23 | O2 | The search budget on the new key | pending |
| operations-podcasts-and-providers-2026-11-23 | O3 | After the free month | pending |
| records-and-measurement-2026-11-23 | R0 | The decision record | working |
| records-and-measurement-2026-11-23 | R1 | What a run saw | pending |
| records-and-measurement-2026-11-23 | R2 | Turn records | pending |
| records-and-measurement-2026-11-23 | E4 | The turn detector against real turns | working |
| records-and-measurement-2026-11-23 | E5 | Batching against the staged funnel | working |
| records-and-measurement-2026-11-23 | R3 | The measure, live | pending |
| storage-efficiency-2026-11-22 | S3 | The capture in one pass | working |
| storage-efficiency-2026-11-22 | S4 | Restore and rebuild inside their budgets | pending |
| storage-efficiency-2026-11-22 | S5 | The deploy floor from real growth | pending |
| storage-efficiency-2026-11-22 | S6 | What leaves the store | pending |
| storage-efficiency-2026-11-22 | S7 | The host's leftovers | pending |
| storage-efficiency-2026-11-22 | S8 | Snapshot chunks that are shared between days | pending |
| storage-efficiency-2026-11-22 | S9 | The tape's payload bytes | pending |
| storage-efficiency-2026-11-22 | S10 | Self-development leftovers measured, then collected | working |
| storage-efficiency-2026-11-22 | S11 | The removed computers' remains deleted | working |
| voice-pipeline-2026-11-23 | V0 | The listener configuration kept and read at launch | pending |
| voice-pipeline-2026-11-23 | V1 | Speech detection by model | pending |
| voice-pipeline-2026-11-23 | V2 | The recogniser the owner named | working |
| voice-pipeline-2026-11-23 | V3 | End of turn by model | pending |
| voice-pipeline-2026-11-23 | V4 | Echo and the same sound | pending |
| voice-pipeline-2026-11-23 | V5 | The turn judged on the phone | pending |
| voice-pipeline-2026-11-23 | V6 | The handover | pending |
| voice-pipeline-2026-11-23 | V7 | Spoken onboarding | working |
| voice-pipeline-2026-11-23 | V8 | Fluid Speech Pro for phones that can hold it | working |
| voice-pipeline-2026-11-23 | V9 | A larger answering model for newer phones | working |
| voice-pipeline-2026-11-23 | V10 | The model list served | pending |
| voice-pipeline-2026-11-23 | V11 | The conversation logic in Exact | pending |
| exact-search-entities-2026-11-20 | X2 | The remaining page logic out of Swift, smallest first | working |
| exact-search-entities-2026-11-20 | S3 | Search leads and the one search verb (fixes 2 and 3, L1 to L4) | working |
| exact-search-entities-2026-11-20 | S4 | The day's number | pending |
| exact-search-entities-2026-11-20 | E1 | Is the event join rate right | working |
| exact-search-entities-2026-11-20 | E2 | The entity layer | working |
| exact-search-entities-2026-11-20 | W2 | The Texture root desk on everywhere | working |
| exact-search-entities-2026-11-20 | W3 | The last three settings | working |
| exact-search-entities-2026-11-20 | H1 | Host problems recorded and not fixed | working |
| exact-search-entities-2026-11-20 | R1 | Operations the old thread held in its session | pending |
| autoradio-exact-cutover-2026-11-18 | X0 | Baseline, review and the device tests | working |
| autoradio-exact-cutover-2026-11-18 | X1 | The loop is fast | working |
| autoradio-exact-cutover-2026-11-18 | X2 | The paper page, as the pattern | working |
| autoradio-exact-cutover-2026-11-18 | XG | The boundary gate, the test map and Bun in CI | working |
| autoradio-exact-cutover-2026-11-18 | X3 | Sign-in in shared code | pending |
| autoradio-exact-cutover-2026-11-18 | X4 | The deck, under the Swift player | pending |
| autoradio-exact-cutover-2026-11-18 | X5 | The run sheet and the radio's decisions in shared code | pending |
| autoradio-exact-cutover-2026-11-18 | X6 | Background play on Android | pending |
| autoradio-exact-cutover-2026-11-18 | X7 | Podcasts and clips | pending |
| autoradio-exact-cutover-2026-11-18 | X8 | Files, the reader and books | pending |
| autoradio-exact-cutover-2026-11-18 | X9 | Mail | pending |
| autoradio-exact-cutover-2026-11-18 | X10 | Content, search, takes, the notebook pages | pending |
| autoradio-exact-cutover-2026-11-18 | X11 | Settings, onboarding, themes, cards, the bar | pending |
| autoradio-exact-cutover-2026-11-18 | X12 | Only voice is native | pending |
| autoradio-exact-cutover-2026-11-18 | K1 | The owner's iPhone keeps working (carried from the learning loop) | working |
| night-exact-vectors-learning-2026-11-19 | V0 | The corpus brief and the faster corpus search, landed and measured | working |
| night-exact-vectors-learning-2026-11-19 | V1 | pgvector and an EmbeddingGemma 2 embedder on Node B | pending |
| night-exact-vectors-learning-2026-11-19 | V2 | Vector tables and the embed worker in corpusd | pending |
| night-exact-vectors-learning-2026-11-19 | V3 | The first pass, on the Mac | working |
| night-exact-vectors-learning-2026-11-19 | V4 | 'One ranked answer: vector order with full-page BM25''s top three kept, and the  | pending |
| night-exact-vectors-learning-2026-11-19 | V5 | The event layer reads research's pages, with a vector shortlist | pending |
| night-exact-vectors-learning-2026-11-19 | V6 | Stories carry event and entity ids at publish | pending |
| night-exact-vectors-learning-2026-11-19 | V7a | The neighbourhood read | pending |
| night-exact-vectors-learning-2026-11-19 | V7b | The fold check at first publish | pending |
| night-exact-vectors-learning-2026-11-19 | V7c | Entity merges | pending |
| night-exact-vectors-learning-2026-11-19 | V7d | The front and the radio deduped by event | pending |
| night-exact-vectors-learning-2026-11-19 | L1 | Read the traces of every recurring failure | pending |
| night-exact-vectors-learning-2026-11-19 | L2 | The calibration experiment, redesigned | pending |
| night-exact-vectors-learning-2026-11-19 | L3 | E1 runs | pending |
| night-exact-vectors-learning-2026-11-19 | L4 | The other dated switches | pending |
| autoradio-learning-loop-2026-11-18 | L1 | Outcomes reach the server | working |
| autoradio-learning-loop-2026-11-18 | L2 | Settlement by rule | working |
| autoradio-learning-loop-2026-11-18 | L3 | Scorecard and ledger | working |
| autoradio-learning-loop-2026-11-18 | L4 | The document and its writer | pending |
| autoradio-learning-loop-2026-11-18 | L5 | The DJ reads the document | pending |
| autoradio-learning-loop-2026-11-18 | L6 | The phone | pending |
| autoradio-learning-loop-2026-11-18 | L7 | Takes | working |
| autoradio-learning-loop-2026-11-18 | L8 | The evening's fixes reach the phone and main | working |
| autoradio-learning-loop-2026-11-18 | L9 | TestFlight | pending |
| autoradio-learning-loop-2026-11-18 | L10 | Android experiment | pending |
| autoradio-overnight-2026-11-17 | S1 | One voice at a time | working |
| autoradio-overnight-2026-11-17 | S2 | Local retrieval for the on-device model | working |
| autoradio-overnight-2026-11-17 | S3 | Latest Exact on both platforms | working |
| autoradio-overnight-2026-11-17 | S4 | The Autoradio Mac app | working |
| autoradio-overnight-2026-11-17 | S5 | EmbeddingGemma 2 on device | working |
| autoradio-overnight-2026-11-17 | S6 | The Autoradio page | working |
| autoradio-overnight-2026-11-17 | S7 | Opening-first programme | working |
| autoradio-overnight-2026-11-17 | S8 | More podcast clips | working |
| autoradio-overnight-2026-11-17 | S9 | Talking at once | working |
| autoradio-full-experience-overnight-2026-11-15 | Q | "the play queue, remote audio, Up next, a URL in the bar" | working |
| autoradio-full-experience-overnight-2026-11-15 | P | "the podcast app" | working |
| autoradio-full-experience-overnight-2026-11-15 | R | "the Reader app, EPUB and PDF" | working |
| autoradio-full-experience-overnight-2026-11-15 | T | "asking fixed, the on-device first answer, voice on and off, the reader's voice" | working |
| autoradio-full-experience-overnight-2026-11-15 | D | "clips in the programme, Play hands the list to the DJ" | working |
| autoradio-full-experience-overnight-2026-11-15 | V | "the design pass, with screenshot rounds from the lead" | working |
| autoradio-full-experience-overnight-2026-11-15 | S | "podcast clips on the server and the transcription worker" | working |
| autoradio-full-experience-overnight-2026-11-15 | K | "the desk sees the queue and returns ordered changes" | working |
| autoradio-full-experience-overnight-2026-11-15 | M | "merges, builds, installs and the report" | working |
| autoradio-learning-engine-2026-11-14 | L0 | design review | working |
| autoradio-learning-engine-2026-11-14 | L1a | reserved namespace, forecast constructor, wire fixtures | pending |
| autoradio-learning-engine-2026-11-14 | L1 | programme call with forecasts | pending |
| autoradio-learning-engine-2026-11-14 | L2 | air log, heard route, settlement | pending |
| autoradio-learning-engine-2026-11-14 | L3 | the paper back, and lessons | pending |
| autoradio-learning-engine-2026-11-14 | L4 | notebook route | pending |
| autoradio-learning-engine-2026-11-14 | P1 | phone, programme player | pending |
| autoradio-learning-engine-2026-11-14 | P2 | phone, playback reports | pending |
| autoradio-learning-engine-2026-11-14 | P3 | phone, the notebook screen | pending |
| storage-solutions-3-store-once-2026-11-14 | T1 | stop copying forward | pending |
| storage-solutions-3-store-once-2026-11-14 | T2 | stop writing mutable objects whole | pending |
| storage-solutions-3-store-once-2026-11-14 | T3 | the guest store, from the reading | pending |
| storage-solutions-3-store-once-2026-11-14 | T4 | receipts, with the retained set named | pending |
| storage-solutions-3-store-once-2026-11-14 | T5 | compression, measured first | pending |
| storage-solutions-3-store-once-2026-11-14 | T8 | find the next one | pending |
| modular-architecture-2026-11-04 | P0_evidence |  | working |
| modular-architecture-2026-11-04 | P1_records_only_channel |  | pending |
| modular-architecture-2026-11-04 | P2_kernels_manifest |  | pending |
| modular-architecture-2026-11-04 | P3_custody_labels |  | pending |
| modular-architecture-2026-11-04 | P4_desks_packages |  | pending |
| modular-architecture-2026-11-04 | P5_apps_services_packages |  | pending |
| modular-architecture-2026-11-04 | P6_isolation |  | pending |
| modular-architecture-2026-11-04 | P7_bayesian_trust |  | pending |
| modular-architecture-2026-11-04 | P8_selfdev_restart |  | pending |
| selfdev-readiness-metamission-2026-10-24 | CI0 |  | working |
| selfdev-readiness-metamission-2026-10-24 | SR0 |  | working |
| selfdev-readiness-metamission-2026-10-24 | SR1 |  | working |
| selfdev-readiness-metamission-2026-10-24 | SR2 |  | working |
| selfdev-readiness-metamission-2026-10-24 | SR3 |  | working |
| selfdev-readiness-metamission-2026-10-24 | SR4 |  | working |
| selfdev-readiness-metamission-2026-10-24 | SR6 |  | working |
| selfdev-readiness-metamission-2026-10-24 | SR7 |  | pending |
| selfdev-readiness-metamission-2026-10-24 | SR8 |  | pending |
| selfdev-readiness-metamission-2026-10-24 | TE |  | working |
| selfdev-readiness-metamission-2026-10-24 | T1a |  | working |
| selfdev-readiness-metamission-2026-10-24 | SR9 |  | pending |
| selfdev-readiness-metamission-2026-10-24 | T1c |  | pending |
| selfdev-readiness-metamission-2026-10-24 | T1 |  | pending |
| selfdev-readiness-metamission-2026-10-24 | T1b |  | pending |
| selfdev-readiness-metamission-2026-10-24 | T2 |  | pending |
| selfdev-readiness-metamission-2026-10-24 | T3 |  | pending |
| storage-solutions-2-2026-11-12 | S3 | measure | working |
| storage-solutions-2-2026-11-12 | S4 | snapshots cost the change, keep two | working |
| storage-solutions-2-2026-11-12 | S5 | fewer events for the same record | working |
| storage-solutions-2-2026-11-12 | S7 | the collector reaps by itself | working |
| storage-solutions-2-2026-11-12 | S8 | a smaller store, and the archive half of cold tape | pending |
| autopaper-overnight-2026-11-09 | W0_contracts_and_audit |  | pending |
| autopaper-overnight-2026-11-09 | W1_first_lanes |  | pending |
| autopaper-overnight-2026-11-09 | W2_second_lanes |  | pending |
| autopaper-overnight-2026-11-09 | W3_materiality_clock |  | pending |
| autopaper-overnight-2026-11-09 | X_side_lanes |  | pending |
| autopaper-overnight-2026-11-09 | E_extra_credit |  | pending |

### Every mission file

| file | now.status | substantive date | in ACTIVE | in graph | stations | not landed |
|---|---|---|---|---|---|---|
| choir-autopaper-improvement-2026-11-09.md | superseded | 2026-11-09 | y | y | 5 | 5 |
| choir-autopaper-overnight-2026-11-09.md | working | 2026-11-09 | y | y | 6 | 6 |
| choir-autoradio-exact-cutover-2026-11-18.md | working | 2026-11-19 | y | y | 17 | 15 |
| choir-autoradio-full-experience-overnight-2026-11-15.md | working | 2026-11-16 | y |  | 9 | 9 |
| choir-autoradio-in-the-desktop-2026-11-23.md | working | 2026-11-23 | y | y | 3 | 3 |
| choir-autoradio-learning-engine-2026-11-14.md | working | 2026-11-15 | y | y | 9 | 9 |
| choir-autoradio-learning-loop-2026-11-18.md | checkpoint_incomplete | 2026-11-18 | y | y | 10 | 10 |
| choir-autoradio-on-android-2026-11-22.md | working | 2026-11-27 | y | y | 8 | 7 |
| choir-autoradio-overnight-2026-11-17.md | working | 2026-11-17 | y |  | 9 | 9 |
| choir-autoradio-pass1-2026-11-10.md | blocked_incomplete | 2026-11-11 | y | y | 0 | 0 |
| choir-campaign-2026-11-27.md | working | 2026-11-27 | y | y | 2 | 2 |
| choir-consolidate-and-scale-metamission-2026-10-26.md | superseded | 2026-10-30 | y | y | 0 | 0 |
| choir-data-architecture-selection-2026-10-28.md | reviewed | 2026-11-14 | y | y | 0 | 0 |
| choir-data-architecture-selection-2026-10-28.precommit.md | none | 2026-11-14 |  |  | 0 | 0 |
| choir-data-system-build-2026-10-30.md | superseded | 2026-10-30 | y | y | 0 | 0 |
| choir-docs-transform-2026-11-27.md | working | 2026-11-27 | y | y | 12 | 6 |
| choir-exact-cutover-and-ontology-2026-11-20.md | working | 2026-11-23 | y | y | 27 | 18 |
| choir-exact-search-entities-2026-11-20.md | working | 2026-11-20 | y | y | 15 | 9 |
| choir-first-design-2026-11-22.md | working | 2026-11-23 | y | y | 13 | 8 |
| choir-guest-isolation-2026-11-01.md | superseded | 2026-11-01 | y | y | 0 | 0 |
| choir-launch-first-pass-metamission-2026-10-21.md | superseded | 2026-10-30 | y | y | 11 | 10 |
| choir-launch-fp0-density-bench-2026-10-21.md | ready_for_fp0 | 2026-10-30 |  |  | 0 | 0 |
| choir-launch-fp0-proof-scripts-2026-10-21.md | checkpoint_complete | 2026-10-22 |  |  | 0 | 0 |
| choir-launch-fp0-vm-retention-2026-10-22.md | waiting_on_fp0 | 2026-10-22 |  |  | 0 | 0 |
| choir-launch-fp1-a1-boot-alter-2026-10-21.md | done | 2026-10-21 |  |  | 0 | 0 |
| choir-launch-fp10-arch-06-2026-10-21.md | blocked_incomplete | 2026-10-21 |  |  | 0 | 0 |
| choir-launch-fp10-arch-08-2026-10-21.md | checkpoint_incomplete | 2026-10-30 |  |  | 0 | 0 |
| choir-launch-fp10-arch-09-2026-10-21.md | checkpoint_incomplete | 2026-10-22 |  |  | 0 | 0 |
| choir-launch-fp10-arch-11-12-2026-10-21.md | checkpoint_incomplete | 2026-10-22 |  |  | 0 | 0 |
| choir-launch-fp10-net-03-04-2026-10-21.md | checkpoint_incomplete | 2026-10-30 |  |  | 0 | 0 |
| choir-launch-fp10-net-05-2026-10-21.md | checkpoint_incomplete | 2026-11-09 |  |  | 0 | 0 |
| choir-launch-fp10-net01-host-fetch-2026-10-21.md | checkpoint_incomplete | 2026-10-22 |  |  | 0 | 0 |
| choir-launch-fp10-net02-mirrors-2026-10-21.md | checkpoint_incomplete | 2026-10-21 |  |  | 0 | 0 |
| choir-launch-fp10-net10-2026-10-22.md | checkpoint_incomplete | 2026-11-09 |  |  | 0 | 0 |
| choir-launch-fp10-p2-small-2026-10-21.md | none | 2026-10-22 |  |  | 0 | 0 |
| choir-launch-fp10-res-08-2026-10-21.md | checkpoint | 2026-10-30 |  |  | 0 | 0 |
| choir-launch-fp10-res-10-2026-10-21.md | none | 2026-10-22 |  |  | 0 | 0 |
| choir-launch-fp10-res-13-22-2026-10-21.md | blocked | 2026-10-30 |  |  | 0 | 0 |
| choir-launch-fp10-res-14-2026-10-22.md | blocked | 2026-10-22 |  |  | 0 | 0 |
| choir-launch-fp10-res-20-2026-10-21.md | checkpoint_incomplete | 2026-10-30 |  |  | 0 | 0 |
| choir-launch-fp10-res17-reaper-2026-10-21.md | none | 2026-10-21 |  |  | 0 | 0 |
| choir-launch-fp10-sec14-deploy-user-2026-10-21.md | checkpoint_incomplete | 2026-10-22 |  |  | 0 | 0 |
| choir-launch-fp2-desk-boundary-2026-10-21.md | checkpoint_incomplete | 2026-10-30 |  |  | 0 | 0 |
| choir-launch-fp3-host-floor-2026-10-21.md | working | 2026-10-21 |  |  | 0 | 0 |
| choir-launch-fp4-selfdev-architecture-2026-10-21.md | checkpoint | 2026-11-09 |  |  | 0 | 0 |
| choir-launch-fp6-network-2026-10-21.md | checkpoint_incomplete | 2026-10-22 |  |  | 0 | 0 |
| choir-launch-fp7-gateway-identity-2026-10-21.md | working | 2026-10-30 |  |  | 0 | 0 |
| choir-lean-system-2026-10-30.lanes.md | none | 2026-10-31 |  |  | 0 | 0 |
| choir-lean-system-2026-10-30.md | checkpoint_incomplete | 2026-10-31 | y | y | 0 | 0 |
| choir-lp-s6-commit-gate-2026-10-20.md | working | 2026-10-30 | y | y | 0 | 0 |
| choir-mac-sync-stage2-2026-11-01.md | reviewed | 2026-10-31 |  |  | 0 | 0 |
| choir-memory-optimization-2026-11-01.md | superseded | 2026-11-01 | y | y | 0 | 0 |
| choir-modular-architecture-2026-11-04.md | working | 2026-11-14 | y | y | 9 | 9 |
| choir-modular-architecture-2026-11-04.precommit.md | none | 2026-11-14 |  |  | 0 | 0 |
| choir-night-exact-vectors-learning-2026-11-19.md | working | 2026-11-19 | y | y | 15 | 15 |
| choir-operations-podcasts-and-providers-2026-11-23.md | working | 2026-11-23 | y | y | 4 | 4 |
| choir-ordinary-prompt-path-2026-11-09.md | complete | 2026-11-09 | y | y | 0 | 0 |
| choir-overnight-pd-then-selfdev-metamission-2026-11-03.md | superseded | 2026-11-04 | y | y | 0 | 0 |
| choir-progressive-delivery-2026-11-03.md | complete | 2026-11-04 | y | y | 0 | 0 |
| choir-records-and-measurement-2026-11-23.md | working | 2026-11-23 | y | y | 9 | 6 |
| choir-selfdev-apps-feature-complete-2026-11-03.md | working | 2026-11-03 | y | y | 0 | 0 |
| choir-selfdev-apps-side-by-side-2026-11-02.md | superseded | 2026-11-02 |  | y | 0 | 0 |
| choir-selfdev-readiness-metamission-2026-10-24.md | checkpoint_incomplete | 2026-11-14 | y | y | 19 | 17 |
| choir-shipping-the-app-2026-11-23.md | working | 2026-11-24 | y | y | 15 | 14 |
| choir-standing-metamission-2026-11-22.md | working | 2026-11-25 | y | y | 9 | 9 |
| choir-storage-efficiency-2026-11-22.md | working | 2026-11-23 | y | y | 12 | 9 |
| choir-storage-solutions-2-2026-11-12.md | working | 2026-11-13 | y | y | 10 | 5 |
| choir-storage-solutions-3-store-once-2026-11-14.md | working | 2026-11-15 | y | y | 8 | 6 |
| choir-store-header-body-split-2026-11-02.md | complete | 2026-11-03 | y | y | 0 | 0 |
| choir-t1a-app-self-development-2026-10-26.md | working | 2026-11-02 | y | y | 0 | 0 |
| choir-v-boot-vocab-pass-removal-2026-10-27.md | checkpoint_incomplete | 2026-10-28 |  |  | 0 | 0 |
| choir-voice-pipeline-2026-11-23.md | working | 2026-11-23 | y | y | 12 | 12 |
| ci-test-cooptimization-2026-10-24.md | waiting_for_FP0 | 2026-10-30 |  |  | 0 | 0 |
| corpus-first-search-one-verb-2026-11-19.md | none | 2026-11-20 |  |  | 0 | 0 |
## C. Designs partly built

Source: `scripts/docs --standing governing` lists 50 governing documents. Only those that carry their own marks are listed. Line numbers are in the file on origin/main. Words in quotes are the design's own.

### docs/design/desks-and-engine-2026-11-24.md (175 lines; status words built, partly, not built)

| line | part | the design's own mark |
|---|---|---|
| 24 | Texture desk | "partly: stories built; the root as designed is not built" |
| 25 | Capture desk | "not built" |
| 26 | Research | "built, but still shapes stories through management" |
| 27 | Autoradio desk taking the voice back | "partly" |
| 29 | Management | "built as the old overloaded form" |
| 122-126 | RLM substrate: any desk creates another RLM; a child creates a child; parent and child message while running; the corpus as values in the REPL; one plain model call returning text | "Only research spawns children"; "Forbidden by rule"; "Not built"; "no iterator ... (lane/corpus-repl)"; "Not built" |
| 109 | Scoring | management's delegation "carries no forecast and is never scored"; spacer removal "is in lane/spacer-out" |
| 151-164 | Order of work, six steps | 1 spacer and serial switch (lane/spacer-out); 2 corpus verbs in the REPL (lane/corpus-repl); 3 one creation verb; 4 capture desk in shadow; 5 learned context, management scoring, cutover and deletion of the management task role; 6 the root as designed. None marked done in the file. |
| 166-175 | Open | what measures novelty; who names a subject; union of captures' symbols; how far back history goes; a separate voice for the phone model |

### docs/design/first-design-2026-11-22.md (413 lines; "Status: idea; nothing here is built unless a line says so", line 8)

| line | part | the design's own mark |
|---|---|---|
| 276 | Step 0, the desk on every model call | "in build, lane f0-desk-on-calls" (current-architecture row 87: built, not deployed) |
| 277 | Step 1, custodian of subjects and mentions | not marked done |
| 280-305 | Step 2, the offline slice | "the first run failed all four lines ... steps 3 to 7 do not start" (lane/offline-slice-r0) |
| 306-322 | Step 2a, density measurement | "C203 remains open" |
| 323-331 | Steps 3 to 7: hot path live beside old; predictions and settlement; stories woken; the root; the readers | none started |
| 339-344 | Open questions | custodian; the root's read of the public graph; mention roles; escalation model; candidate-rule tuning |
| 5 | Supersession | "Superseded where it conflicts (owner, 2026-11-24)" by desks-and-engine and the ideal-form draft |

Mission file `choir-first-design-2026-11-22` records stations F0 pending, G1 and G2 working, G3 to G6 pending, F6 pending; F1 to F5 superseded.

### docs/design/autoradio-in-exact-2026-11-18.md (258 lines; "Status: proposed", line 5)

The design has no built marks. Its order (lines 80-111) is stations 0 to 10; the problem record below carries the progress. Parts the problem record still lists as Swift: see next table.

### docs/problems/autoradio-app-logic-is-swift-not-exact-2026-11-18.md (216 lines; "Status: open", line 3)

| line | part | the record's own words |
|---|---|---|
| 100 | after 2026-11-19 | "Still Swift ...: the run sheet, the radio page, the queue, the voice" (since moved, per later updates) |
| 147 | first run, computer, activity | "Not moved, and why: Configurator, LiveConfigBackend, VoiceSetup" (voice list) |
| 176-184 | run sheet and queue | "the shared runtime exists; the native flip is the open part" (line 201 says the flip has landed) |
| 205-212 | still Swift 2026-11-23 | RunSheetEntries, RadioBridge, RadioPage, PodcastClips and the Joins/Podcasts handlers; SegmentTracker kept for about 30 tests; BarState; PlaybackContinuity, Playhead, PlayerControls, Deck (audio session, voice list); ChoirApp engine controller; ChoirClient "the exact-rest lane's" |
| 216 | 2026-11-24 | "Still Swift on purpose, and the next two changes: the Texture editor ... and the mail seal"; they "wait for a redesigned editor state model and mail identity (four review rounds found new defects in each fix)"; gate count 40 files, 4,657 lines, 22 names |

### docs/design/account-create-delete-2026-11-23.md (98 lines)

| line | part | mark |
|---|---|---|
| 3, 5 | deletion half (section C) | "built on lane/account-delete ... Not deployed"; creation half open |
| 84-91 | S0 device test of creating a passkey; S1 register view and Create account action; S2 lazy computer (no VM at sign-in); S6 sweep of abandoned users; S7 AASA, capability, profile, Exact passkeys | no mark; the note says creation is open; a calibration claim C231 (deletion on the drill account) is open |
| 96-98 | "What I could not confirm" | registration cap value; whether passkey creation works in the sheet; Apple's deletion wording; footprint of an empty computer |

Measured: `account-delete`, `rebased` (tree account-delete), `acctdel-land` and `rev-purge` have commits not matched on main by patch; the App Review train commit ed89268fa (2026-11-25) names account deletion. Whether the deletion server is deployed was not determined here.

### docs/design/onboarding-voice-tour-2026-11-25.md (42 lines)

| line | part | mark |
|---|---|---|
| 3 | whole design | "deferred; not built" |
| 9-14 | radio-first tour | "Status: deferred design ... The radio-first tour is not in that build" |
| 16-35 | play radio first with Tiny Voca; tour as programme segments; one Download action on the shared queue; voice questions when listening is ready | all unbuilt |
| 37-42 | replaces `onboarding.contract` and the step machine in `onboarding.ts` | not done |

### docs/design/resource-fabric-2026-10-31.md (1,276 lines)

| line | part | mark |
|---|---|---|
| 5 | whole | "design exploration of the end state. Nothing in it is built." |
| 1217-1243 | section 10, ten proposals | only proposal 1 is marked "Accepted (owner, 2026-10-31)"; proposals 2 to 10 (home per computer, writer lease, writer_handoff, home-durable precommitments, Choir in Choir by grant, native capability model, passkey enrollment, offboarding, voice) carry no ruling in the text |
| 337-345 | workstation push route | "Already built: bundle format with delta export, verifier..."; "New: the push route; the workstation's host key in the trust set; writer_epoch and branch_id in the signed head receipt" |
| 1036-1049 | section 7.3 what is missing | org principal, org graph scope with RBAC "named in doctrine, unbuilt", addressed publication, effect policies "designed; not live", site enrollment, a home that is not Node B, org kill switch, MDM guidance |
| 1078-1135 | 8.2 near-term builds | not marked |

### docs/design/modular-architecture-plan-2026-11-04.md (2,085 lines)

| line | part | mark |
|---|---|---|
| 3 | standing | "the mission is paused; the rulings are not replaced" |
| 5 | whole | "Nothing here has been built, reviewed or landed" (dated 2026-11-04; a later reader should check main) |
| 170, 567, 774, 1103, 1655, 1730, 1849, 1871 | phases 1 to 8: precommitment records as the only inter-desk channel; kernels and manifest; custody and labels; every desk a package; apps and services as packages; isolation; Bayesian trust; resume self-development | phases; the mission file lists P0 working and P1 to P8 pending |
| 1810 | isolation phase | "Not built here. The installed surface must mount in the shell" |
| 1974 | open questions | section 18 |

Note: CLAUDE.md says every desk is a package desk registered the same way; current-architecture.md rows 60 to 63 show engineering "partial (section 8)" and the autoradio desk "manifest only", so parts of phase 4 exist. This was not traced further.

### Other governing documents with their own marks

| file | line | mark |
|---|---|---|
| docs/design/autoradio-dj-programme-2026-11-11.md | via current-architecture 122 | slices 3 to 7 "Not built": mail headers; air log and /api/autoradio/heard; hand-offs and the go cell; station and write-ahead; podcast notes |
| docs/design/autoradio-queue-desk-2026-11-15.md section 6 (not governing, cited by arch row 123) | 123 | "autoradio.queue cell module; queue on the programme route; claims for reorders" |
| docs/design/autoradio-learned-document-2026-11-17.md | not read in depth | the learning-loop mission checkpoint_incomplete, 10 stations open |
| docs/design/forced-modularity-build-2026-11-04.md, library-app-store-security-intent-2026-11-04.md, capsule-overlay-computer-state-2026-11-03.md, one-tape-durability-2026-10-31.md | not read | governing; marks not extracted (not determined) |

### docs/current-architecture.md rows whose status is not built or proven

The file uses proven, built, partial, specified, idea, unknown. Of its tables, 12 rows are proven and 12 built by first-word count; the rows below are the rest.

| line | row | status in the file |
|---|---|---|
| 34, 36 | on-device models; Autoradio desk | partial; "no cell" |
| 42, 60 | engineering | partial (section 8) |
| 46-47 | Autoradio desk takeover; mail and podcast notes in the programme | "idea" for the server desk; "specified" for mail and podcast notes |
| 62 | autoradio package desk | manifest only; run sheet "not yet written by a desk" |
| 63 | mail desk | built; "live use unknown: settle by a Mail lease record on d@e" |
| 83 | search providers | partial (five paid providers out of quota) |
| 87 | the desk on every model call (F0) | built, not deployed (lane f0-desk-on-calls) |
| 88 | podcast clips | partial |
| 100 | management story page and main-point judgment | built, not deployed (lane story-main-point) |
| 122 | autoradio DJ programme slices 3 to 7 | specified, not built |
| 123 | autoradio.queue cell module | specified, not built |
| 189 | how long a fresh deploy takes to show on a computer | unknown |
| 196 | engineering in capsules | proven (older), "Not re-proven after the modular..." |
| 198 | prompt-only change on one computer | partial (write route refused on deployed computers) |
| 210 | store readings | built, not yet run on the paper (lane store-by-kind) |

## D. Obligations recorded elsewhere

### Calibration ledger (docs/calibration/claude-ledger.md)

Rows whose last column is "open": 18. Settle-by passed as of 2026-11-27T10:00Z, where the row carries a settle-by time: 8 (C120, C165, C178, C210, C231, C250, C251, C255). C206 and C209 settle by a relative event ("the deploy plus ...", "the first run after the deploy") and cannot be dated. C254 and C257 are not yet due. Claims C1 to C257 live in this file; from 2026-11-27 new claims are files under docs/calibration/claims (none yet beyond the README).

| id | line | settle by | claim (start) |
|---|---|---|---|
| C120 | 206 | 2026-11-25T06:00Z | The lesson stroke works for one listener: once the listener's document is read by the DJ (station L5 of `docs/ |
| C127 | 213 | 2026-12-09T06:00Z | The cutover's conjecture E1 (`docs/definitions/choir-autoradio-exact-cutover-2026-11-18.md`): by the settle ti |
| C178 | 263 | 2026-11-21T16:00Z | The themes as shared Exact code (`lane/exact-themes` dbd791a82, four commits; accepted on a fourth review with |
| C183 | 268 | 2026-12-04T00:00Z | The conjecture of the goal `docs/definitions/choir-exact-cutover-and-ontology-2026-11-20.md`: once station C2  |
| C195 | 280 | 2026-11-30T00:00Z | Storage efficiency (`docs/definitions/choir-storage-efficiency-2026-11-22.md`, stations S1 and S3): with the l |
| C197 | 282 | 2026-11-30T00:00Z | Storage station S1, the layout trigger deleted (`lane/layout-land`, 392d749cd; both seats accepted with fixes, |
| C165 | 284 | 2026-11-21T12:00Z | The Texture root desk on every computer with no setting (`lane/root-desk-r3`, the restructured commit `cf31b8d |
| C206 | 361 | relative | `capsule-artifacts` on the drill computer holds at least 4 GiB, and `capsule-artifacts/subjects` alone at leas |
| C207 | 362 | 2026-12-08T00:00Z | After an idle-time collector of candidate source copies and receipts (owner, 2026-11-24: not at boot, some tim |
| C208 | 363 | 2026-12-08T00:00Z | The first tester's computer (2 GiB memory) holds at most 3 GiB in `data.img` on the host after seven days of o |
| C210 | 364 | 2026-11-25T12:00Z | One hour after the decision-record build (`lane/decision-record`) is deployed to the paper, `GET /api/decision |
| C209 | 365 | relative | The operator command that deletes the tape segments and snapshot chunks of removed computers, run in its defau |
| C250 | 426 | 2026-11-26T12:00Z | One day after the front render record is deployed, the paper's Store B holds `choir.page_render_record` rows ( |
| C251 | 427 | 2026-11-26T12:00Z | Lane `drop-zai` (Z.AI provider removed) passes CI on main and deploys to staging with no rollback, and in the  |
| C254 | 431 | 2026-11-28T00:00Z | The current shared Exact app plus a narrow Kotlin native layer can pass every required shipped-iOS journey on  |
| C231 | 432 | 2026-11-27T00:00Z | After `lane/account-delete` deploys, on the drill account only (never the paper or the owner's computers): a p |
| C255 | 433 | 2026-11-27T00:00Z | Station A1's frozen candidate passes the whole uncached `internal/appids`, `internal/auth`, `internal/proxy` a |
| C257 | 435 | 2026-11-29T09:00Z | A batch of lanes lands in one push. After the batched queue of the docs-transform mission (`docs/definitions/c |

Broader heuristic (measured, noisy): 209 claim rows exist; 136 have no matching settlement row (id in a 5-column settlement table). Of those, 74 carry a parseable settle-by before 2026-11-27T10:00Z; 54 cannot be dated by a simple column parse because the table columns differ. Most are landing claims from 2026-11-16 to 2026-11-24 whose settlement may be recorded in prose. The full list is in the scratch file ledger_open.tsv (id, line, recorded, settle-by, due flag, claim).

### Problem records dated 2026-11-15 or later

Heuristic only: a record is listed when none of "fixed", "resolved", "landed" or "closed" appears in its first 15 lines. Records dated by file name. 107 records examined: 29 with such a word, 78 without. Many of the 78 may be fixed in a later update that the first 15 lines do not show.

| path | title |
|---|---|
| account-boundary-three-approaches-no-convergence-2026-11-23.md | The phone's account boundary: three approaches, twelve rounds, no landing |
| android-native-runtime-archive-2026-11-26.md | The pinned llama.cpp Android archive is not a complete 16 KiB runtime source |
| antigravity-login-suspended-2026-11-17.md | The Antigravity product login is suspended, and it served consensus review seat 1 |
| autoradio-app-logic-is-swift-not-exact-2026-11-18.md | Autoradio: the app's logic is a Swift package, not Exact, so nothing is shared with Android (2026-11-18) |
| autoradio-asking-went-quiet-2026-11-15.md | Autoradio: asking the radio could go quiet, lose the answer, or show nothing (2026-11-15) |
| autoradio-four-lists-decide-what-plays-next-and-there-is-no-desk-2026-11-16.md | Autoradio: four lists decide what plays next, and there is no Autoradio desk |
| autoradio-owner-phone-review-2026-11-15.md | Autoradio on the owner's phone: the first review (2026-11-15) |
| autoradio-repeats-what-the-listener-already-heard-2026-11-18.md | Autoradio repeats what the listener already heard (2026-11-18) |
| autoradio-voice-pipeline-not-the-one-chosen-2026-11-24.md | Autoradio's voice pipeline is not the one the owner chose (2026-11-24) |
| broker-payload-accepts-a-number-no-decoder-holds-2026-11-23.md | The broker's payload check accepts a number no decoder holds |
| capture-read-serves-any-capture-id-to-any-guest-2026-11-24.md | GET /internal/platform/captures/<id> serves any capture id to any bound guest |
| captures-dropped-and-stalled-2026-11-24.md | Captures are dropped by expiry and by tasks that close without deciding them (2026-11-24) |
| carry-by-reference-design-three-review-rounds-structural-assessment-2026-11-15.md | Carry by reference: three review rounds without convergence |
| cell-interpreter-drops-a-map-index-in-a-two-target-assignment-2026-11-22.md | The cell interpreter drops a map-index target in a two-target assignment |
| ci-relevant-script-names-reached-main-unseen-2026-11-19.md | Scripts that CI must run for reached main classified as docs (2026-11-19) |
| compact-copy-grows-the-host-image-and-restarts-discard-it-2026-11-15.md | The paper's compact copy grows the host image, and a restart discards it |
| computer-disks-hold-self-development-leftovers-2026-11-24.md | Two test computers hold 4 to 6 times a normal computer's disk, and no code ever removes what self-development leaves t |
| deploy-marked-incomplete-while-the-paper-was-refreshed-twice-2026-11-22.md | A deploy was marked incomplete after one health read timed out, and the paper took two planned restarts in eleven minu |
| entity-read-lists-no-stories-in-production-2026-11-20.md | The public entity read lists no stories in production, and every candidate weighs 0 (2026-11-20) |
| every-landing-restarts-the-owners-computer-2026-11-25.md | Every landing restarts the owner's computer, and the phone calls it "Offline" (2026-11-25) |
| first-vectors-file-refused-equal-text-under-another-title-2026-11-19.md | The first vectors file was refused whole: equal text under another title carried the wrong vector |
| gateway-log-lines-carry-raw-guest-strings-2026-11-22.md | Gateway log lines carry raw guest strings (2026-11-22) |
| gateway-refuses-guests-while-it-drains-on-deploy-2026-11-19.md | The gateway refuses every guest while it drains on a deploy (2026-11-19) |
| gateway-spend-bad-file-two-movers-lose-a-call-2026-11-21.md | Two gateway processes that both find a corrupt spend file can lose one process's calls (cause: the test, not the ledge |
| guest-reach-file-is-not-a-security-surface-2026-11-23.md | The file that decides what a guest can reach is not a security surface |
| host-logs-kept-a-day-or-for-ever-2026-11-24.md | Host logs are kept under a day in one place and with no time bound in others |
| incremental-snapshots-design-three-review-rounds-structural-assessment-2026-11-15.md | Incremental snapshots design: three review rounds without convergence (2026-11-15) |
| intake-flushes-single-captures-as-their-own-jev-call-2026-11-15.md | Intake flushes single captures as their own Jev call |
| jev-misuse-structural-assessment-2026-11-15.md | Why we keep using Jev wrongly: a structural assessment |
| landing-script-runs-cheap-gates-after-the-suites-2026-11-15.md | The landing script runs the cheap gates after the suites |
| native-pushes-without-an-account-generation-2026-11-23.md | Native pushes that carry account data and no account generation (2026-11-23) |
| node-a-store-fills-with-unrooted-guest-images-2026-11-15.md | Node A's Nix store fills with unrooted guest images |
| offhost-backup-prune-has-failed-for-twelve-days-2026-11-20.md | The off-host backup has not pruned for twelve days, and its freshness check fails unseen |
| owner-phone-stuck-in-demo-mode-2026-11-20.md | The owner's iPhone was stuck in demo mode (2026-11-20) |
| paper-data-image-grew-40-gib-in-eleven-minutes-2026-11-16.md | The paper's data image grew 40 GiB in eleven minutes after a deploy |
| paper-dispatcher-twelve-slots-full-ninety-due-2026-11-20.md | The paper's dispatcher: twelve slots full, about ninety agents due (2026-11-20) |
| paper-publishes-stories-whose-headline-is-a-failed-search-2026-11-16.md | The paper publishes stories whose headline is a failed search |
| paper-snapshot-aborts-at-its-time-budget-and-its-scratch-copy-blocks-deploys-2026-11-22.md | The paper's store snapshot aborts at its time budget, and its scratch copy takes Node B under the deploy floor (2026-1 |
| parallel-mobile-builds-exhausted-the-mac-2026-11-15.md | Parallel mobile builds exhausted the Mac and cost the first QA build (2026-11-15) |
| passkey-claims-an-unproved-address-2026-11-24.md | A passkey can claim an address nobody has proved (2026-11-24) |
| phone-pages-stall-placard-2026-11-24.md | Phone pages stall on a placard; the radio stops after a segment (2026-11-24) |
| podcast-clipping-work-was-never-written-down-2026-11-15.md | The podcast clipping work of November 10 was never written down |
| podcast-clips-review-did-not-converge-structural-assessment-2026-11-16.md | Podcast clips: three review rounds did not converge. Structural assessment |
| podcast-transcripts-refused-and-downloads-stuck-on-first-address-2026-11-17.md | Long podcast transcripts refused whole, and downloads that never left the first address |
| podcast-worker-cannot-fetch-from-the-build-vm-proxy-only-egress-2026-11-16.md | The podcast worker cannot fetch from the build VM: its only way out is a proxy |
| podcast-worker-transcripts-refused-for-an-invalid-segment-2026-11-16.md | The podcast worker's transcripts are refused for one invalid segment |
| radio-restore-empty-after-let-go-2026-11-25.md | The radio starts empty after a quick relaunch, and its writer waits 30 s at every launch |
| read-first-reads-more-but-searches-as-much-2026-11-22.md | Read-first makes research runs read more but not search less |
| recurring-agent-failures-read-from-traces-2026-11-19.md | Recurring agent failures read from traces, and a clustering assessment (2026-11-19) |
| remote-audio-is-recorded-heard-without-sound-2026-11-19.md | Remote audio that reaches its end is recorded as heard, sound or not (2026-11-19) |
| research-child-report-drops-the-capture-label-2026-11-18.md | A research child's report reaches its parent without the capture label (2026-11-18) |
| research-desks-woken-by-management-replies-loop-2026-11-20.md | Research desks are re-woken by management replies that say Relate refuses "central", which no ledger shows (2026-11-20 |
| research-does-not-start-from-the-corpus-2026-11-18.md | Research does not start from the corpus (2026-11-18) |
| research-runs-quadrupled-after-research-frame-2026-11-23.md | Research runs rose from 13 to 62 an hour right after the research-frame prompt change |
| research-runs-send-reworded-web-searches-2026-11-22.md | Research runs send reworded web searches because their prompts ask for sources the frame does not need |
| restore-bases-design-three-review-rounds-structural-assessment-2026-11-15.md | Restore bases design: three review rounds without convergence |
| reviewer-account-deletion-boundary-2026-11-25.md | A missing reviewer ID is not permission to delete |
| search-sufficiency-measurement-three-reviews-no-landing-2026-11-20.md | The search sufficiency measurement (L2) was sent back three times and is stopped |
| search-volume-outruns-keenable-credits-2026-11-18.md | Search volume outruns the Keenable credits (2026-11-18) |
| search-waits-four-seconds-behind-the-embed-worker-2026-11-19.md | A corpus search waits four seconds behind the embed worker and comes back without its vector half |
| shared-settle-promise-fails-every-request-2026-11-25.md | One let-go answer fails every shared-code request until the app restarts |
| sign-up-bound-was-a-constant-on-accounts-2026-11-24.md | The sign-up bound was a constant on accounts, with an exemption bolted on (2026-11-24) |
| status-hourly-reports-zero-search-answers-while-search-answers-2026-11-16.md | The hourly status script says no search was answered while search is answering |
| store-schema-floor-guard-is-not-run-2026-11-15.md | The store schema floor guard is not run by anything |
| story-list-paging-can-skip-rows-in-one-second-2026-11-19.md | Paging over published stories would skip rows that share a second, if any row carried a fraction (2026-11-19) |
| story-research-context-main-point-empty-under-enforce-2026-11-22.md | The deployed story research context loses its main point under the enforced reader gate |
| sub-call-spacer-outlived-its-bucket-2026-11-24.md | The sub-call spacer outlived the bucket it protected (2026-11-24) |
| takes-in-exact-three-reviews-no-landing-2026-11-20.md | The takes page in Exact was sent back three times; the approach to old data is changed |
| tape-object-ops-carry-more-than-twice-their-payload-2026-11-15.md | Tape object ops carry more than twice their payload |
| texture-editor-in-exact-three-reviews-no-landing-2026-11-20.md | The Texture editor in Exact was sent back three times and is stopped for the owner |
| texture-root-desk-six-reviews-no-landing-2026-11-20.md | The Texture root desk was sent back six times and is stopped for the owner |
| texture-story-ask-path-writes-no-fast-v1-and-research-delivers-once-2026-11-16.md | A topic prompt gets no fast v1, and research delivers once at the end |
| the-days-top-story-ranked-low-2026-11-23.md | The day's top story was published and ranked near the bottom |
| the-paper-is-not-restorable-by-any-path-2026-11-15.md | The paper is not restorable by any path |
| train-landed-red-objectgraph-registry-and-vendor-hash-2026-11-20.md | A train went to main red: an unregistered edge kind and a stale vendor hash |
| two-landings-in-one-tree-pushed-an-unverified-train-2026-11-19.md | Two landing runs in one tree pushed a train whose suites had not run (2026-11-19) |
| vector-worker-gap-count-never-ends-on-the-paper-2026-11-19.md | The vector worker's first statement never ends on the paper (2026-11-19) |
| web-searches-are-sent-without-asking-what-is-held-2026-11-19.md | Web searches are sent without asking what is held (2026-11-19) |