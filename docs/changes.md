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
| Phone: the rest of the pages moved to shared code ("the train") | built; fails 7 checks of its own real-host proof; never on a phone | `lane/exact-train1` |
| Phone: dead native code removed | built, reviewed once; waits on the train | `lane/native-dead-code` |
| Phone: one data folder per account, sign-out wipes it | 12 review rounds, 5 failed drill runs; stopped for a structural assessment | `lane/account-partition-s1` |
| Research prompts: one source count, a stop rule, a real gap measure | sent back by its review (four blocking findings); rework pushed, re-review owed | `lane/research-frame` |
| Phone: "Autoradio 1", the train as a second app beside the first | **on the owner's phone since 2026-11-23 01:33Z**; seven defects found there | `lane/exact-side-by-side` |
| Phone: position moves while playing; a closed answer card stays closed; the queue is described to the server as it is; the microphone opens on a first launch | built; one review seat accepted with six fixes, being applied; not on the phone | `lane/exact-ar1-next` |
| Phone: sign-in in a sheet over the app | built (red: the approval route and page change); seat 1 accepted with three fixes, seat 2 running | `lane/signin-sheet` |
| A voice ask searches the paper | not started. Today the answering model has no tools and sees at most 40 front-page titles | none |
| `choir trace run`: one run, step by step, with the reasons its record holds | built and run on the drill account; shows 0 model calls for a run that made some; not landed | `lane/trace-run` |
| Research prompts: one source count, a stop rule, a real gap measure | built; under test; not reviewed | `lane/research-frame` |
| `choir trace run`: one run, step by step, with the reasons its record holds | built; tested on fixtures only; not run against a real computer; not landed | `lane/trace-run` |
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

## 2026-11-22

### `choir trace run <run_id>` shows one run step by step with its reasons (`lane/trace-run`, yellow)
- **What:** a command that reads a run through routes that already exist and prints its model calls, cells, module-surface calls, records, turn commits, restarts and resumes in time order, with the reason the record holds for each call (turn-commit Reason, else the model's own text, else "not recorded"), what retirement has taken, and a closing list of what that run's record does not contain. `choir trace runs` lists runs. `--json` and `--full`.
- **Why:** a run's steps and reasons could only be rebuilt by hand from console lines (owner, 2026-11-22: "show me this run, step by step, with its reasons").
- **Proof:** 24 tests on fixtures in the route handlers' own types, whole `cmd/choir` suite and gates pass. Not run against a real computer.
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
