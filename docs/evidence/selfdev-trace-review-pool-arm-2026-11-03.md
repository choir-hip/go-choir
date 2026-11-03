# Pool arm trace review, 2026-11-03 (computer-4d38420c..., read-only)

Sources: trace exports of 5dc6a6ff (Files), a6b8f87c (rebase C), a475876b (Slides), abab3f23. UTC. Engineering: gpt-6-luna low for Slides rounds 1-3, DeepSeek xhigh from round 4. Judge "passed" for applied operations comes from lane C's notes; the operation records no reason.

## Rounds (bind to Freeze)
- F1 assignment-273d9f89 / selfdev-1d052791: 47.6 min, passed, applied 05:28.
- F2 assignment-146c3b6c / selfdev-c4ac7712: 21.2 min, passed, applied 06:43 (stand-in shots, allowed by the request).
- F3 07:14: Cast refused at admission (stack_base_moved). Rebase A 10:45: notice never reached engineering (mangled id, problem 16).
- Rebase B assignment-10d8ef65 / selfdev-ab2912d3: 15.1 min, approved, failed booted_base_moved (problem 22).
- Rebase C assignment-aee046fa / selfdev-5dbd9ba2: 49.9 min, applied 15:39.
- S1a assignment-285dd525: 10.6 min, ended "blocked", no Freeze. S1b assignment-7183c78e: 1.9 min, blocked.
- S2 assignment-b2eef4a5 / selfdev-f413bff3: 9.2 min, failed: "generic-only screenshot, missing captures and measurements".
- S3 assignment-5aba7f27 / selfdev-7781b241: 23.8 min, failed: "most screenshots are placeholders, measurements absent".
- S4 assignment-1d51bdc9 / selfdev-737b5314: 37.6 min, failed: HTML deck on text canvas, authored slide layout undefined, image insert never resolves.
- S5 assignment-e4fe531c: bound 19:54:46, still running at 20:07, no operation.

## Findings, ranked by time lost

**1. Rebase chain (about 4h40 idle for Slides, 11:31 to 16:14).** Symptom: the Slides Cast was refused at 11:31 ("needs a live root desk"). Notices reached a settled Files Texture only after owner nudges (revisions at 13:08 and 13:40). Rebase C spent 7 min rebasing (rejected hunk was already upstream, done 14:44), then 40 min on a failing theme-contract spec until Freeze at 15:24. Cause: no notice for a settled document (problem 21), a refresh between approve and apply, and a recipe that let "no new work" turn into spec runs. Status: fixed by 0de7c5865, 781fcfc46, d7d02400a, 23f54dd98, 059763bb0, 3f32a6ee4, 52e05f16c. Hypothesis: the 14:35 notice still needed the 13:40 nudge, so problem 21 is unproven on pool.

**2. False scope blocker: choir-shot cannot open a package app (about 90 min plus 3 failed rounds).** Symptom: S1a wrote a shot with `selector:'body'`, saw `[data-slides-next]` never appear, and called Complete "blocked: package isn't launcher-bound in desktop registry". S1b read packages.ts ("dynamic discovery") and still concluded none exists. Management escalated it as an owner scope decision; the arm sat 67 min (16:34 to 17:41). S3 grepped choir-shot for `--app`, `launcher` (all -1) and fell back to `route.fulfill` stubs ("<main>01 mobile placeholder"). S4 (DeepSeek) needed 16 min of probes (19:12 to 19:28, `/?app=slides-next` failed) before finding the app-switcher click. Cause: neither scripts/choir-shot.mjs nor the engineering overlay (rlm_engineering_runtime.yaml) mentions launcher, app switcher or openApp; the shell only opens an app by clicking `button[aria-label*="app switcher"]` then the manifest title. Proposed: add `ctx.openApp(page, title)` to choir-shot and one recipe line stating "a package with launcher menu true is opened from the app switcher; never report a launch blocker without running that click". Status: not fixed on main (7beec5d5b added only --theme and --contrast).

**3. Texture to management to engineer hand-off loses or falsifies the revision (S2 and S4 failed on proof).**
- S2: management's Cast was one sentence plus `{"parent": recordID}` (17:47:45, 3 iterations, 395 output tokens). The engineer's first cell did print the 18,159-char request (truncated:false) with the twelve-shot list, so lane C's "never saw the shot list" is unconfirmed. It froze after 9 min with twelve identical `path:'/',selector:'body'` shots.
- S3: the full verbatim text reached the engineer and it still shot stubs (finding 2).
- S4: management indexed `OwnerDirections[4]`, which is round 3's text, and pasted it as "round 4 verbatim" (19:10:50, objective 6,184 chars). Round 4's Step 0 to Step 3 order, 15/25 minute budgets and `/var/lib/artifact/contrast/*.json` pointer never arrived. Management then Noted Texture that round 4 was "carried verbatim" (false). The packet holds all six revisions oldest first (22 KB).
- Cause: the engineering prompt is whatever management types; the platform appends nothing. Proposed: the platform appends the operative revision verbatim to the assignment prompt and orders `owner_directions` newest first. Status: not fixed (management-cast-binds-wrong-parent-record is a different defect).

**4. Texture "hollow turn" (5.5 min plus an owner resend; same trap hit at 11:29).** Symptom: 17:41:27 cell did Cast, Write, Wait with `if err != nil { panic(err) }`. Wait after Write returns "this cell already called Write; Wait is a turn without a revision" (internal/yaegikernel/choir.go:1259); the panic rolled back both staged intents (`intents:0`), the model ended with prose saying it had dispatched, and the platform logged "trigger not disposed" at 17:42:03. The same error appeared in 7 Slides cells (2 in Files); in 5 of the Slides ones it was harmless. Cause: Cast help says a Cast "commits as a Wait", so models add Wait, and the error looks fatal. Proposed: make Wait after Write a no-op, or say in Write/Cast help "do not call Wait after Write". Status: 2cd70f606 closes the trigger but does not remove the trap.

**5. gpt-6-luna low gives up or stubs.** S1a: 93 iterations, 18.7K output tokens; S2 froze after 9 of 60 minutes; DeepSeek xhigh S4 wrote 49K and shot real decks. Moot: the arm is DeepSeek from 18:58.

**6. Round 5 duplicate Texture turn.** Two Texture runs (19:52, 19:53:52) each Cast to management; the first wrote unbuilt repairs in present tense ("An HTML deck now renders..."), the second corrected the document. The second Cast was refused cleanly ("assignment ... still bound ... send the change with choir.Note"), and management Noted the engineer at 19:57. Hypothesis: Write re-wakes Texture.

**7. No `.git` in capsules.** `git status` fails in S3 and rebase C, so the prompted "git diff touches only slides-next" check cannot run. Proposed: Freeze prints changed paths.

## Keep
- 15-minute rebase B from a 168-char "rebase as it is, adding no new work" prompt; mechanical `stack_rematerialized` re-graft (15:47:13, 16:01:54) with no engineering round.
- Admission refusals that quote the cause and the Note/CancelAssignment next step.
- choir-shot `--theme` and `--contrast`; the Step 0 receipt quote; Freeze's path-naming refusal.
- Texture correcting its own overstated revision within 3 minutes.

/private/tmp/claude-501/-Users-wiz-Dev-go-choir/f298736e-cabb-51ff-9cea-34588a4989cb/scratchpad/trace-review/pool-arm.md
