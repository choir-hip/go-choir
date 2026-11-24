# What changed

One entry per thing built, newest first. This is the page to read to catch up
(owner, 2026-11-22: "its hard to catch up on what claude has done since i last
looked"). Every landing that changes behavior adds or updates its entry in the
same change. An entry says what changed, why, how it was proved, what it
replaced, and how to undo it. "Proof: not re-checked" means the entry was
backfilled from the commit and nobody re-read the evidence for this page.

Backfilled on 2026-11-23 from 2026-11-20. In that span main took 241 commits:
169 docs-only and 72 that change behavior.

## Not landed (in flight or dropped)

| What | State | Where |
| --- | --- | --- |
| Phone: one data folder per account, sign-out wipes it | 12 review rounds across three approaches; frozen; [structural assessment](problems/account-boundary-three-approaches-no-convergence-2026-11-23.md) written 11-23, the choice of boundary is the owner's | `lane/account-partition-s1` |
| Phone: "Autoradio 1", the train as a second app beside the first | **on the owner's phone**: installed 2026-11-23 13:08Z from `dcc56f2cd` (the train with its review fixes); the owner's tests are owed: a locked phone, a first-launch microphone, the sign-in sheet | main |
| Phone: sign-in in a sheet over the app | both parts are on main (`977442ff8`, `0a397a684`); never run on a phone | main |
| Phone model benchmark on the Motorola | first rows; not landed | `lane/model-bench` |
| Snapshot capture in one pass (no store-sized scratch copy) | not started; the reader half landed 11-22 | `lane/snapshot-one-pass` |
| Redesign of capture (subjects and mentions), offline slice | **failed all four of its kill lines** (claim C192); design panel not yet run | `lane/offline-slice-r0` |
| "Feeds reported by accounts" | removed after three failed reviews; the phone still posts to the 404 route | main |
| Phone: onboarding and settings, the reader, the edition picks as shared code | built; not reviewed, or sent back twice | `lane/exact-onboarding-settings`, `lane/exact-reader`, `lane/exact-edition-picks` |
| Phone: the Texture editor as shared code | stopped for the owner after a third send-back; part landed | `lane/exact-convo-edit` |
| Phone: an earlier account boundary mechanism, and a settle fix | red; one review seat outstanding | `lane/account-boundary-l1`, `lane/account-settle-memo` |
| Snapshot capture streaming pages (no scratch copy) | red; stopped for the owner 11-20; main has moved 22 commits in its files | `lane/snapshot-stream` |
| The old root desk on every computer | red; **superseded by the first design**, which retires it | `lane/root-desk-on`, `lane/root-desk-r3` |
| Owner acts always enforced | red; held for the owner; two of three commits already on main | `lane/owner-act-enforce` |
| A planned restart cancels an in-flight snapshot capture | red; parked after five rounds | `lane/restart-cancels-capture` |
| Restore installs at boot (design and failure list) | parked | `lane/restore-boot-install` |
| A management reply does not wake research | unreviewed | `lane/reply-wake-fix` |
| Dispatcher: a log line per slow wake | its suite fails on the build machine | `lane/dispatch-deadlines` |
| Search starting from Ceramic | kept by the owner's word; main has moved 80 commits in its files | `lane/search-ceramic` |
| Full-band voice capture while the radio plays | one commit, not merged | `lane/voice-fullband` |

Counted 2026-11-23: 520 lane branches on the remote; 468 are landed, 13
superseded, 11 evidence only, 26 are distinct live work (the rows above and
the first table's). Seven designs exist only on lanes.

## 2026-11-24

### A per-account usage report (`scripts/status-account-usage`, yellow)
- **What:** one read-only ssh to Node B, then per account (by user id, no email): computers, calls and input tokens over 24 hours and 7 days, dollars today and yesterday, disk, memory and the top providers, most active first. No flag, no bound.
- **Why:** owner, 2026-11-24: monitoring of usage, not a spend bound, for friendly testers.
- **Proof:** `scripts/test-status-account-usage.sh` (24 checks, seen failing first); run against Node B on 2026-11-24: 1,097 accounts, 4 with a computer, 29,103 calls in the journal's reach.
- **What it cannot say:** [problem record](problems/per-account-usage-is-recorded-thinly-2026-11-24.md): no history beyond the journal and a two-day spend file, no output tokens per computer, no computer hours.
- **Replaced:** nothing. **Undo:** delete the script.

### Phone: the listening models come from the model list; Fluid Speech Mini is the recogniser (`lane/voice-default`, orange)
- **What:** (1) a recognition stage for Fluid Speech Mini (stage `fluid-speech-mini`, ported from Mike's `FluidSpeechMini.swift`: warm, one pass over the speech-bounded audio), its model-list entry (repository, pinned commit, every file's size and SHA-256, CC BY 4.0 with its credit) and a store that fetches a folder of files, each checked against its digest. (2) a test that runs it on the fixture speech through Core ML on the Mac. (3) the listener configuration is no longer a fixed list: speech detection, live words and the final transcript are chosen from the model list (`Voice/ListenerPipeline`): Silero, Parakeet EOU for live words and Fluid Speech Mini for the transcript when their models are on the phone and ready, the energy detector and Apple's recogniser until then. A better model is a new row in `CandidateTable`.
- **Why:** owner, 2026-11-24: Apple's recogniser "isnt good enough"; "we are only using that currently because of a coding bug. (we shuold be using fluid speech mini etc)"; "we will keep updating the models as better ones come".
- **Proof:** Swift package tests on the Mac (numbers in `docs/reference/ON-DEVICE-MODELS.md` section 11, marked Mac, not phone). **Not proved on a phone**: the list the lead must check is in the lane's report.
- **Also:** `Voice/ModelList` (the model list as JSON-decodable data with a built-in copy and per-device recommendations; the server route is not built), and the live-then-final stage names no model (its stage ids come only from the choice). Measured on the Mac over 73 real-reader clips: Parakeet TDT v2 2.57%, Fluid Speech Mini 3.93%, Parakeet EOU 8.13%, Apple 12.66% word error (section 11).
- **Review fixes (same lane, after the first review):** the chosen listener is applied to the running app when voice setup ends (it was fixed at launch and hands-free kept its stages for the life of the process); the live-words model that comes with a batch recogniser is part of the configuration (consent size, memory, what is kept; before, it was fetched then deleted in the same run) and a model is ready only when its companions are in; a folder model is found and deleted by the configurator; paths from a served list may not leave the model's folder; a copy is replaced on a pin bump; the composite finishes (not cancels) the live session and falls back to its words when the final is empty. Proof: Swift package tests on the Mac; a 33 s real-speech turn through Fluid Speech Mini scored 1.2% word error. Not proved on a phone.
- **Second review fixes (same lane):** the Fluid row's size, progress bar and memory need count the live-words model that comes with it (503 MB and 800 MB, not 253 and 550); a turn no longer waits for the live recogniser once the final transcript is in hand (the join happens before the next turn's `begin`, for at most two seconds); a final with no letter or digit counts as empty; Cancel on an answer ends it for hands-free, so a listener change is not held back; a served repository or path can no longer trap (percent-encoded, repository and commit validated, never force-unwrapped); served sizes are bounded (4 GiB a file, 8 GiB and 256 files a model, overflow-safe) and each file's size is checked after its digest; the model folder records its pin (`.pin`, written last, removed first) so a pin bump with the same sizes or an interrupted replace is not "ready"; the store refuses file names that leave its root in `ensure` and `register`; the engine's configuration hops are ordered. Checked and not a defect: the recognisers are marked ready by the provider as each finishes and read from `voice-ready.json` at launch (a test now covers the relaunch). **Not proved on a phone.**
- **Replaced:** the first-commit fixed list (energy detector, Apple's recogniser) in `ChoirApp.listenerConfiguration`; voice setup's transcriber ladder now starts at Fluid Speech Mini with its live words (one row, two downloads). **Undo:** revert the commits; nothing is migrated.
### choir.news serves the app association file for passkeys

- **What:** `GET /.well-known/apple-app-site-association` on choir.news answers 200 with JSON naming two app ids of team S47GERGJZ3 (`news.choir.autoradio` and `news.choir.autoradio.lab.one`) under `webcredentials`. It is a public route with no sign-in, served by the proxy (`internal/proxy/apple_app_site_association.go`).
- **Why:** iOS lets an app create or use a site's passkeys only if the site names the app in this file. It is the first piece of passkey account creation and sign-in inside the phone app (owner, 2026-11-23), landed ahead of the rest because Apple's servers fetch and cache it.
- **Proof:** the Go tests drive the real route table and handler chain (200, content type, length, the exact JSON shape, no body on HEAD, 405 for other methods, neighbouring and child paths do not serve it) and check that the development variant the app's manifest produces is in the list. After deploy: `curl -sS -D- https://choir.news/.well-known/apple-app-site-association`. Not proved: that a phone accepts it. The app's entitlement is not on main yet, and the development id works only in a build signed with its own App ID carrying Associated Domains (none exists yet; the app on the owner's phone today will not match). The first development app is on a free team, which cannot use passkeys for a site at all, so it is not listed.
- **Replaced:** nothing; the path used to fall to the computer-surface catch-all.
- **Undo:** revert the commit. Apple's servers cache the file, so a phone may keep honouring a removed app id for about a day after the revert deploys.

## 2026-11-23

### Phone: the radio never waits (lane `exact-flip-player`; orange)
- **What:** two defects of the shared player, found on the phone on 2026-11-23, fixed in `radio-director.ts`, `radio-writer.ts` and `radio-bridge.ts`. (1) Opening the radio no longer throws the saved queue away: the old `open` removed every desk and phone entry ahead whenever no opening had been heard for half an hour; now they stay and play after the opening, in order (a transition whose neighbours are gone is still dropped when its turn comes). (2) With nothing ahead and the radio wanted, the director asks the phone for what to play: the paper's front stories not yet on the sheet (up to three, the paper's weight first, placed at the end by the phone with a reason, their words fetched when each starts). So after the opening the radio goes on to the front page with no ask of the desk. The paper's list now travels to native with the queue snapshot (`radioSet` carries `candidates`) and is the turn's `candidates`; the Swift story list (`ChoirApp.loadStories`, `storyRows`, `ChoirFront.stories`) is deleted. Also deleted: the radio view's item window (`RadioItem`, `items`), the `radioJump` send and the `jumpTo` action (nothing called them), `RadioPage.length`.
- **Review repairs (second commit):** a refused opening with nothing ahead (the app reopened inside the half hour) used to air the second front story and mark the first heard unplayed, because the fill's own store changes started the first and `advance` then aired another; the fill no longer re-enters the director and an entry already on air is never taken over (N8, N8b). A fill story the listener removes, or clears with Up next, is not filled again for this listening (N9); the front page otherwise returns when nothing is ahead. The voice's candidate key now covers every route. Not done: while the phone is locked the deck cannot hold a fill story ahead (its words are fetched when it starts), so the gap between two fill stories rests on shared code running as the sound ends; a saved queue is not aged out and may be days old.
- **Why:** the owner, 2026-11-23: "get all this converted to exact overnight ... its been dragging for too long". The run sheet, queue, player decisions, opening, the bar's run-sheet facts, the radio page, the voice turn's queue and the server's queue operations were already shared code (the train); what was left was these two defects and dead Swift.
- **Proof:** `radio-never-waits.test.ts` (N1 to N7 and the restart case, written first and run red), `radio-front-fill.test.ts` (a launch through the bridge with a desk that never answers: the opening, then the first front story, handed to a fake deck; fails with the fill removed), `radio-director.test.ts` 14 rewritten. `bun test` in `mobile/AutoradioLab`: whole suite pass; `swift build --build-tests` and `swift test` of `mobile/AutoradioKit`: 661 tests, 0 failures; `contract build app.contract`, `test web` and an `agent web` walk of the radio page and the bar: pass. Not proved: nothing was run on a phone or a simulator; native files under `mobile/exact-package/apple` were edited but are not compiled by the Kit's package (the lead's build compiles them).
- **Replaced:** the Swift story list and a branch of the radio view. Native non-voice code 5,844 to 5,741 lines, 27 to 26 interface names. **Undo:** revert the commit; the saved queue is then discarded again after half an hour and nothing follows the opening but the desk.

### A voice ask searches the paper (`2d44744e0`, `95ce70334`, `42fa0d9f4`, orange)
- **What:** before its one model call, a voice turn searches the paper's published stories for the listener's words (keyword and vector, fused, then weighted by age, stored importance and distinct sources) and gives the model the best ten as fenced untrusted data whose routes join the queue candidates. One host read with a 1.5 s wait; a failure is told to the model as a search that did not run, an empty or half-answered search as incomplete.
- **Why:** the owner: "it should do keyword and vector search over the corpus and rank by new/important/citations". Before this the model saw at most 40 front-page titles.
- **Proof:** deployed 2026-11-23 13:58Z; the first deployed proof at 14:04Z failed one condition (the vector half missed its budget, so the answer was keyword only: [problem record](problems/ask-vector-half-misses-its-deadline-2026-11-23.md)). Fixed by `f95d6f28b` (at most 80 passages, the two halves side by side), deployed 17:04Z. `scripts/proofs/autoradio-ask-paper.sh` on the drill computer then passed three times of three at 17:05Z: found 28, gave 10, vector answered, search 228 to 448 ms.
- **Replaced:** nothing. **Undo:** revert the three commits.

### Phone: the train as one branch (`0a397a684`, a merge; orange with red parts)
- **What:** the phone's pages and their logic moved from Swift to shared Exact code, as one branch cut from `lane/exact-ar1-next` (the train and the radio fixes) with three lanes merged in: reader, search, imported documents and the computer page (`lane/train-fix-ac`); the native review fixes (old-build files wiped, audio tests restored, dead Swift deleted, the earlier native-dead-code lane, the voice list made true, the native ceiling set to the true 49 files) (`lane/train-fix-d`); and the sign-in sheet's app part (below). Added here: the page the voice holds (`pageText`) carries the account generation it was read under and native drops a stale one, the same rule `radioSet` and `editionMirror` have; the unread `level` verb case and `LiveAudio.preparedSpeech` are deleted. Areas on shared code: reader, search, imported documents, the computer page, mail, files, podcasts, takes, the notebook, the run sheet, the radio and its queue, sign-in, sync.
- **Why:** the owner's rule: only voice is native ("get everything in exact, as much as possible").
- **Proof:** bun suite 1408 tests pass on the merged tree with the generation guard's fixes (`lane/train2-gen`, `lane/train2-churn`). Simulator proof `train2p6` at `dcc56f2cd`: 167 checks pass, one fails (Files could not reach the computer during that drive). On the owner's phone as Autoradio 1 since 2026-11-23 13:08Z; the owner's tests are owed.
- **Replaced:** the Swift of those pages and the dead native code (native non-voice code is 49 files, 5844 lines, 27 names; the ceiling rose to 5844 for the account-generation guard: a boot announcement (`SharedGeneration.boot`, native adopts a JS context that starts again below the newest), the one-lock `admit` (check and write are one step) and the wipe's last act, a floor above anything computed before it, announced with a deadline). **Undo:** revert the merge.
- **The radio's churn, fixed here (`lane/train2-churn`):** while the programme played, native's 0.4 s progress timer woke the radio's pulse (it touched `radioView` and `barFacts`, which also moved the radio note), so the pulse ran about 180 times a minute and each beat asked the three screens and what reads them again. Now the beat moves once per device second and the progress timer and the microphone's level no longer wake the pulse: 60 pulses a minute playing in front (12 paused), native calls 840 to 480 a minute counting native's own `barFacts` and `radioView` (pulse and screens alone 540 to 180). The simulated minute is `radio-churn.test.ts`; the proof's `rpulse` and `asked again` counts should fall by about a third. **The sheet's posts now wait for an in-flight desk ask** (up to the ask's deadline, 240 s): the pulse calls for no second `radioWrite` while one waits, because a mutation sent again replaces the first and the first is the one waiting for the reply. The cleaner remedy, a separate post mutation, is not done.

### Phone: sign-in opens the approval page in the system sign-in sheet (app part of `lane/signin-sheet`, orange)
- **What:** Sign in opens the device-flow approval page in Exact's system sign-in sheet (`openAuthSession`), from shared code; one sign-in attempt is one state machine in shared code (`signin-attempt.ts`); native keeps the device code, one poll and the key write. The return from the approval page is only a signal: the poll learns the key. `app.json` declares `SIGNIN_SCHEME` per variant. The server half (the approval route and page return, red) lands from `lane/signin-server`, not here.
- **Why:** sign-in left the app for a full browser.
- **Proof:** shared-code tests only (`signin-sheet.test.ts`, `signin-attempt.test.ts`); **owed on the owner's phone.** The stricter `TestDeviceReturnSchemesAreTheAppsInAppJSON` (equality, each variant's `SIGNIN_SCHEME`) replaces the relaxed one on this branch.
- **Replaced:** Swift's browser call and the `choirOpenApproval` op. **Undo:** revert the commits.

### The edges index retry waits for its build lock (`3b5f6fd34`, `6229cd73a`, orange)
- **What:** the retry that repairs an invalid edges index waits for the build lock instead of returning as if done when another builder holds it, re-reads the index after the wait, and gives the build its own deadline after a long wait.
- **Why:** CI failed twice on `bd643bc55` in the platform suite. It looked like a flaky test and was a real defect: a retry that lost the lock race left the index invalid and reported success.
- **Proof:** CI green on both commits; the three index tests ran 90 times under CPU load with no failure and the new deadline test fails on the old code. No deployed proof beyond the deploy's health checks: the path runs only when an index build is interrupted.
- **Replaced:** `pg_try_advisory_lock` and its silent return. **Undo:** revert both.

### REVERTED 2026-11-23: Research prompts: one source count, a stop rule, a real gap measure, a corpus search that says whether it matched (`3dcddbe86` to `33fc8ff10`, orange)
- **Reverted 2026-11-23 by the commit that carries this line (orange; rollback: revert):** after it deployed (2026-11-23 07:58Z) research runs went from 13.2 an hour (11-22 17Z to 11-23 07Z) to 62.3 an hour (08Z to 10Z), web searches from 40 to 132 an hour, model calls to 1,792 an hour, and the provider refused 82 calls for quota in an hour; captures, management calls and Texture runs stayed flat and 41 of 48 newly opened stories got a first-round research Cast. The mechanism is not proven; the problem record is `docs/problems/research-runs-quadrupled-after-research-frame-2026-11-23.md`. The prompt text and code below were restored to their state before `3dcddbe86` (role prompt v22 and Texture v5 carry the old text under higher numbers). The entry below is kept as the record of what was tried.
- **What:** the "read at least 8 independent sources" target and its pick-list are gone; one statement of the two-outlet gate (`MinPublishOutlets`, unchanged) in the research role, the overlay and Texture, with the event record named as a source of the second; Texture's Cast asks for the one thing the frame lacks, names the page to fetch and never says corroborate as a goal; a stop rule (a detail you cannot confirm is a finding; a reworded query that returns pages you already hold says this wording does not find it) beside "search the web whenever the corpus does not hold what the question needs"; `held` in web results means an earlier search of this run returned the page; the first turn names the digest and asks for an interim packet when there is a claim to cite. `SearchWireCorpus` opens its result with `captures_matched_words` and `captures_near_by_meaning_only` and a `words_note` (silent when the text index is partial or events, published pieces or a window explain the zero). The report log counts a gap only for a `missing:` line that says something and logs its words; `scripts/status-search-reads` reports the mean over all runs, the heaviest-five share, the no-read share and real gaps. The worked cell only prints the counts: whether pages answer is the model's judgement.
- **Why:** in 3 hours of traces 50 of 77 hand-labelled searches were reformulations, runs told to corroborate sent 7.4 searches each, the prompts held three source counts, the corpus search returned captures 224 of 224 times whether or not the graph held the subject, and 55 of 56 Reports "named a gap" because the log counted a placeholder. Not a cap: runs still go to the web when the corpus lacks the answer (owner, 2026-11-23).
- **Proof:** deployed 2026-11-23 07:58Z (staging serves `33fc8ff10`). Deployed proof owed: the gap measure and the corpus-share line in `scripts/status-hourly` after a day. Built and tested on the build VM: researchtools, runtimeprompts, promptstore, textureprompts, yaegikernel, the whole agentcore and textureowner shards, internal/platform (with pgvector), the status-script fixtures.
- **Replaced:** the 8-source target, "do not search again", the placeholder `missing:` line, the unset `SearchHit.CaptureID` claim (the field stays, commented). Not done: waking research on a record with no objective, capturing the page a lead links to, logging the domains read, a worked cell that reads the event record. **Undo:** revert.

## 2026-11-22

### `choir trace run <run_id>` shows one run step by step with its reasons (`99338db60`, `bd643bc55`, yellow)
- **What:** a command that reads a run through routes that already exist and prints its model calls, cells, module-surface calls, records, turn commits, restarts and resumes in time order, with the reason the record holds for each call (turn-commit Reason, else the model's own text, else "not recorded"), what retirement has taken, and a closing list of what that run's record does not contain. `choir trace runs` lists runs. `--json` and `--full`.
- **Why:** a run's steps and reasons could only be rebuilt by hand from console lines (owner, 2026-11-22: "show me this run, step by step, with its reasons").
- **Proof:** 24 tests on fixtures in the route handlers' own types, whole `cmd/choir` suite and gates pass. Not run against a real computer.
- **Replaced:** nothing. **Undo:** revert.

### Device approval can name the app to return to (`977442ff8`, red)
- **What:** `POST /auth/device/approve` takes optional `return` (an app id from a fixed list: `news.choir.autoradio.lab`, `news.choir.autoradio.lab.one`) and `state` (a nonce) and answers `return_to` only after a successful approval (a denial answers none, and the page does not navigate; the app learns of it by its poll); the decision is recorded first, and a bad hint just omits `return_to`. The page sends the hint only when it has the server's shape (a long or odd hint in a crafted link is dropped). A body over `readJSON`'s 1 KiB cap is a 400 before the code is looked up (no decision, no miss counted, no oracle); a test pins that. The `/device` page follows `return_to` back to the phone's sign-in sheet. Clients that send neither field see no change. The CI frontend job now runs the page's return unit test.
- **Why:** the phone's sign-in leaves the app for a full browser; the sheet needs a way back. The server and page must be deployed before the app that uses them, so this part lands alone, ahead of the app part (`lane/signin-sheet`).
- **Proof:** deployed 2026-11-23 07:58Z; `scripts/proofs/device-approve-return-deployed.sh` passed all six checks against staging at 08:12Z (a stranger's checks only: 401 with and without the fields and identical bodies, 415 for a form, the page served). The approval answering `return_to` is covered by Go tests, not yet seen on staging or a phone.
- **Replaced:** nothing. **Undo:** revert.

### The store can be measured by kind (`0f4421aae`, orange)
- **What:** a background reading of a computer's store: rows and bytes per table and per object kind, current against superseded, duplicates, run memory by age.
- **Why:** the paper's store is 50 GiB, grows about 2 GiB a day, and nobody knew what was in it.
- **Proof:** deployed; ran on the paper 2026-11-23 00:08Z to 00:18Z, all 30 tables complete. The file is 49.7 GiB and 29.4 GiB of it is stored values: object bodies 15.7, agent run memory 7.3 (2.4 of it older than a week), object headers 5.6. By kind the largest bodies are events 3.6, Texture revisions 1.8, lifecycle commands 1.7, source references 1.6, records between desks 1.3, runs 1.2. Superseded objects: 0. Duplicates: 0.24 GiB. So the store is mostly the machinery's own log, and about 20 GiB is index and page overhead of 8 million small objects.
- **Replaced:** nothing. **Undo:** revert.

### Restore can verify a snapshot by its pages (`9e3e2060f`, red)
- **What:** restore accepts a snapshot record that carries a digest of its pages and checks that, without recomputing an hour-long digest of every row.
- **Why:** first half of removing the store-sized copy and the 55-minute check from the daily snapshot.
- **Proof:** deployed. **No capture writes the new record yet**, so nothing in production uses it. No restore has ever run on a store larger than about 1 GB.
- **Replaced:** nothing yet. **Undo:** revert.

### Compaction no longer copies a store for its age (`06c49c1b8`, red)
- **What:** the weekly "layout" trigger is deleted. A compact copy is made only when the store has free pages to reclaim.
- **Why:** on 11-22 a copy that reclaimed nothing held Node B under the deploy floor and refused wakes.
- **Proof:** deployed. The claim that it does not hurt snapshot reuse (C197) is open until 2026-11-30.
- **Replaced:** the layout trigger and its marker file. **Undo:** revert.

### Research is given the paper's graph first (`3fc71ee89`, orange)
- **What:** a research run starts with what the paper already holds on its subject and is told to use it before the web. The read-first refusals and `SearchFresh` are removed.
- **Why:** the owner: "we shouldnt need rules... prompts to encourage models to search the object graph first".
- **Proof:** deployed. **It did not cut web searches** (C191 false: 2.85 per run over all runs, five runs sending 46%).
- **Replaced:** the two 11-21 search changes below. **Undo:** revert.

### Every model call names its desk and run (`34bc283ee`, `aefdfbbc4`, `507f23055`)
- **What:** the gateway's log line for each call carries the desk and the run; `scripts/status-calls-by-desk` reads it. A racy test of the spend ledger was fixed.
- **Why:** model calls could not be attributed, so cost per desk was a guess.
- **Proof:** deployed and read (C194 true). Calls before 16:58Z on 11-22 are unattributed.
- **Undo:** revert.

### The paper's story bundle is relayed whole (`d7d276b55`, orange)
- **What:** the proxy passes the resolve bundle through uncut and bounds decoded bodies explicitly.
- **Proof:** not re-checked. **Undo:** revert.

## 2026-11-21

### A capture joins a story by its main point (`a887e478e` and three more, red)
- **What:** management decides open-or-feed by judging the capture's main point against candidate stories. A story's research continues with what it holds; a refused research request never drops the page write.
- **Why:** the title-word matcher opened duplicate stories.
- **Proof:** deployed; 46 of 46 opens in one window carried a main point. The "this continues that story" answer was given 0 times, so no successor links exist.
- **Replaced:** the title-word matcher. **Undo:** revert.

### Research reads what it holds before searching again (`2543187a8`, `92f068980`)
- **What:** refusals that made a run read held pages before another web search.
- **Proof:** deployed, then **removed the next day** by the graph-first change.

### Story publish loads sources by id and retries a timeout (`8578a1aa8`, orange)
- **Why:** 10 to 48 publishes an hour failed loading the private revision.
- **Proof:** a deployed proof script exists (`scripts/proofs/publish-load-deployed.sh`); result not re-checked. **Undo:** revert.

### Phone: the takes page is shared code (`358afba01`, orange)
- **Proof:** on main; **not on the owner's phone**, which runs the build of 11-20 10:42Z. **Undo:** revert.

## 2026-11-20

### Phone pages moved from Swift to shared code
Themes, the conversation page, the Textures list, the bar and the Autoradio
page (`53bfafc64`, `2f82f7703`, `3b0bdaae7`, `e1e6d0342`, `1b5de2c2e` and
their review answers). About shows the commit the app was built from
(`bfc1492c6`, `fc3ef9176`). The Exact pin moved (`5124c0a65`).
- **Why:** the owner's rule: only voice is native.
- **Proof:** simulator proofs at landing, not re-checked. The owner's phone runs `bfc1492c6`; **the bar, the Autoradio page and everything after are not on a phone.**
- **Replaced:** the Swift theme loader, row formatter, Textures list. **Undo:** revert each.

### Phone: demo mode and two dead syncs deleted (`a7e0e6535`, `6bdddb530`, `c00a7bb22`, `12d8f3bab`)
- **Proof:** not re-checked. **Undo:** revert.

### Management sees the whole 8-hour window of stories, and held stories per arrival (`1b8c7682c`, `d8367e2c8`, `2b862a00e`, `e33533850`)
- **Proof:** not re-checked. **Undo:** revert.

### Search reuse inside a run (`87137215e`, `f51ea5c50`, `7b1a63183`, `20af586d3`, `3f3e10ba1`)
- **What:** a re-asked question is answered from what the run holds; every web search passes a need test.
- **Proof:** deployed. Searches per run did not fall to target (claims on this were false three times). **Undo:** revert.

### A research run with no objective is briefed from its conversation (`03c84a441` and three more)
- **Proof:** on 11-22, 21 of 55 runs were still woken with no objective. **Not fixed.**

### Entities: a ready list, candidates and a public read (`2becc9673`, `dc584af3b`, `0eef16ec7`, `e33d9df5f`)
- **Proof:** deployed. A hand read found 29% of story entity rows are not entities and 24% are duplicates.

### corpusd keeps every web-search result for 30 days; captures get vectors before the event flush (`b1257d04a`, `303239085`)
- **Proof:** not re-checked. **Undo:** revert, drop the table.

### Three settings deleted: twelve dispatcher slots, the event layer and production by event are unconditional (`7f5ad41e1`, `6a873149b`, `605436fdc`, `8c296cde6`)
- **Why:** no switches.
- **Proof:** deployed. On 11-22 the twelve slots were seen full with 33 agents waiting.

### The gateway's spend files are safe for two writers (`f1aa4df9d`, `f883ea88d`, red)
- **Proof:** not re-checked. **Undo:** revert.
