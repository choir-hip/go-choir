# deepseek arm trace review, 2026-11-03 (computer-e80d229b...)

Sources: trace exports of 6 trajectories, `self-dev show`, repo main at e8adf487a. Times UTC. "bind to Freeze" in minutes.

## Rounds
- Files R1 eee45597: 50. No operation; self-dev mode off at Freeze.
- Files R2 f0934b0d: 30. op 35d61e7e; FP0 rejected: one preset in all 12 shots, shots 08/10/11 missing, bare page not in the window, Folder 0 B.
- R2b 4bd58941: cancelled 3.5 (FP0 build cap).
- Files R3 e4146ed6: 40. op 574077b3 applied; 4 UI checks skipped, stand-in file API.
- Files R4 45bd487c: 30. op 4ee90af9 rejected base_moved 07:05 (problem 12).
- Files R5: no assignment. Texture committed 07:17:42 with a Write only (receipts: texture_turn, no cast); dead until the 10:47 restart.
- Rebase 09852284: 6. op 152fe036 applied.
- Files R6: never ran. Texture desk-worker "permission denied", 78 loops 11:02-11:40.
- Files R7 7a80bd65: 49. op 0edfc337 applied 14:21.
- R7 re-check 17a659c2: cancelled at 60 ("context deadline exceeded"), no report.
- Re-check 22b89f80: 29. No op; contrast 12.13-17.27 all pass; diff-scope unresolved.
- Slides S0 8c4c1bc5: cancelled at spawn, host memory (14:39).
- S1 9db10554: 52 (opened 15:22, bound 15:42). op 5a1f90f0 failed: no shots, Settings theme stall.
- S2 e1d0a799: 46 (queued 27). op 763adb58 failed: futuristic-noir 3.04:1.
- S3 fdf9b071: 46. op d42c1646 failed: no contrast numbers, 2 of 12 shots.
- S4 bc3618ee: 22. op 5f442f29 applied, published 19:16.
- Restore 65637c64: cancelled at spawn, host memory (19:19).
- Restore 090e8233: 15. op 3e3c28f2 approved, apply refused base_moved_since_freeze. No re-run assignment seen at export (unconfirmed).

## Findings, ranked by time lost

### 1. Engineering opens wait for the previous assignment, and no desk can see it (~84 min queued, 3 rounds)
**Symptom:** S1 opened 15:22:06 and bound 15:42:36, 9 s after the stuck re-check was cancelled. The re-check 22b89f80 opened 16:02:29 and bound 16:39:48, 4 min after S1 froze. S2 opened 16:42:14 and bound 17:09:11, 11 s after 22b89f80 froze. Texture's 16:04:33 turn: "assignment is open but dormant with no operation or spawn attempt, it cannot be released from this run".
**Cause:** the ledger shows one live engineering slot per computer (mechanism is a hypothesis). Nothing records "queued behind <assignment>", and a hung occupant holds it for the full 60 minutes.
**Change:** write a ledger record on open ("queued behind X, bound at Y") that Texture can read; let Texture or management cancel the occupant.
**Status:** open. Problem 24 (6e4658891) covers only memory waits.

### 2. A "prove it" re-check burned the 60-minute deadline with no deliverable
**Symptom:** 17a659c2 made 52 read-only calls at about 70 s each: theme.ts, files.go, state.db, a git ls-remote. No choir-shot run, no contrast number. `git` answered "not a git repository", so the requested "file-by-file diff of the pre-apply generation" had no tool. Five cells panicked on `r.Stdout[:4000]` with the opaque `1:28: panic: main(...)`.
**Cause:** management's cast (14:42:21) asked for three unverifiable proofs and replaced the owner's words with its own objective. The capsule has no .git and no base tree. The panic message carries no line.
**Change:** (a) the recipe states "/workspace/platform has no .git; the base tree is at <path>"; (b) cell panics print line:col and the failing expression; (c) a 25-minute "no deliverable yet" nudge; (d) management must not ask for a diff the capsule cannot compute.
**Status:** open. choir-shot --contrast (7beec5d5b) covers only the contrast part.

### 3. Slides proof failures: three failed operations from capture tooling (~145 min build)
**Symptom:** S1's report said the page "never exposed [data-theme-preset] within 20 s" after driving the Settings window, and zero PNGs resulted. S2 fixed it by seeding choir.theme.boot.v2 from the page, in its own /tmp harness. S3 re-wrote a 15 KB harness from scratch and logged six capsule_job_ended retries (8 min gaps), leaving 2 of 12 shots. S3 also rewrote slides.css tokens for 46 min against a 3.04:1 figure that S4 traced to the shell's window-count badge, outside the package (S4 measured per element). S4's `choir-shot --help` answered "unknown flag --help" and it found no --contrast flag.
**Cause:** the recipe still sent engineers through Settings. The harness lived in /tmp and the carried patch moves only source. The measure did not name the element. The S4 capsule's choir-shot appears to predate 7beec5d5b (hypothesis; deploy time not checked).
**Change:** carry /tmp/shots.mjs (or ship a Slides spec in the repo); choir-shot --contrast prints the element and its owner (package or shell); add --help.
**Status:** --theme and --contrast already fixed by 7beec5d5b. Carry and element naming are open.

### 4. Rounds 5 and 6 produced nothing (~3.6 h)
**Symptom (R5):** the revision said "call choir.Cast now". Texture read the doc for 2.5 min and made one Write with no cast receipt. No wake followed.
**Symptom (R6):** after the 11:02 layered apply every desk_go_eval failed with `fork/exec .../desk-worker: permission denied`. Texture re-ran the turn 78 times until the 11:40 restart.
**Cause:** R6 is problem 17. R5 matches no numbered problem I found (hypothesis).
**Change (R5):** reject a Texture commit whose owner revision asks for a Cast but whose receipts hold none.
**Status:** R6 fixed by 2cd70f606. R5 open.

### 5. Texture re-framed the restore request, and the cast packet lost the owner's scope (R8)
**Symptom:** 19:26:32 Texture cast "Retry the publication ... refused at spawn by memory", and management's 19:27:40 cast read "Publish the combined stack to the Library now". The owner's scope, verify commands and 20-minute limit were not in either. The engineer's 19:42 report said the premise was wrong, and it did the restore anyway.
**Cause:** `owner_revision` was in the packet only for the Slides casts at 16:42 and 18:53, not at 19:26. Texture summarized the failed first attempt, not the revision.
**Change:** when the trigger is an owner revision, the cast packet must carry owner_revision, and management's engineering objective must quote it verbatim.
**Status:** open.

### 6. Problem 28: the build began on a stale base and the apply was refused (~15 min plus a judge run)
Opened 19:27:40, the VM refreshed 19:27:57, bound 19:28:08, froze 19:39 on 2cd70f60 against booted e4f1c685. The same class hit R4 at 07:05 (problem 12). Change: check booted base against release base at bind and Freeze, before the judge. Status: documented in e8adf487a, not fixed.

### 7. Host-memory cancels at spawn
S0 at 14:39 and the restore at 19:19. Status: 6e4658891 adds a wait; deepseek ran the old binary at 19:19.

## Keep
- Engineer reports were honest about gaps (S1, S3) and corrected a wrong premise (R8).
- Judge reasons were concrete and carried the next-run instruction ("measure first, then shoot"), and S4 followed it in 22 min.
- The rebase notice path took 6 min. Texture's post-report rewrites were accurate.
- Every contrast and restore claim cites a ledger receipt.

/private/tmp/claude-501/-Users-wiz-Dev-go-choir/f298736e-cabb-51ff-9cea-34588a4989cb/scratchpad/trace-review/deepseek-arm.md
