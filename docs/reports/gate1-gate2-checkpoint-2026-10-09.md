# Gate 1 and Gate 2 checkpoint

Written Friday evening, October ninth, after about five and a half hours of autonomous work that began around one in the afternoon Eastern time, when you handed over Node A and went to rest. It covers only that stretch.

The Texture acceptance suite now passes every check except two named holes, and self-development on staging can run code in its capsule again for the first time since October fourth.

## Where things stand

Gate 1 is close. The suite's seventh run passed listing, first draft, revise, version reads, reload, cancel, the crash check and the "what is owed after a crash" check. The crash rule you set holds on staging: after a crash the document reads as interrupted, gains no new revision, and nothing is owed or retrying. Two things still fail. Cancel ends a document for good, and a research request does not yet produce a cited revision.

Gate 2 has started. The reality probe found that the engineering desk could not run a single line of code in its capsule, and had not been able to for five days. That is fixed and verified. A fresh run is working on its change right now.

There are three things for you. Rotate the SerpAPI key. Decide how Cancel should behave. Decide when a research assignment counts as finished. Each is explained below.

## The suite, one layer at a time

I ran the suite seven times this evening. Each run failed at a different layer, and each failure was written up before it was fixed.

First, after Texture settled its own work, your next revise was refused. An owner revise now reopens the work.

Second, research found nothing because the search plane had gone dark. One cancelled search had counted as a failure against every provider at once, so all of them cooled down together. Brave had also been out for a week because we asked it for forty results when its limit is twenty. Both are fixed, and every search in the later runs succeeded.

While tracing that, I found something worse. A SerpAPI error message stored in the gateway carried the API key in its URL. The gateway returned that message to computers on every search outage, and the research tool passed it into the model's context. So the key has been in research desks' tool results and in model provider requests. Error text is now scrubbed when it is stored and again when it is read. I also cleared the stored copy. The tape is append-only, so past copies stay where they are. **Please rotate the SerpAPI key.** In the same pass I found that the gateway's search-health and breaker routes checked no caller at all, and any computer could reach them. They now answer only on the host itself, and I verified that on staging.

Third, research ran but every report it sent was rejected. The research desk was told the report's name but never its fields, and the checker refuses unknown fields. The check now runs inside the cell that writes the report, so the desk sees the error immediately, and the desk is shown the exact shape.

Fourth, once reports got through, Texture and research looped: each report woke Texture, and Texture opened more research. The document stayed on "Revising…" and grew past what you asked for. Each owner request now allows two research assignments, and further ones are dropped with a note on the turn. Run seven confirmed it: the revise landed and the document went idle in twenty-one seconds.

The lesson is about where these bugs live. Five of today's Texture problems share one cause: work closes only when a model makes the right choice, and the models are not told the rule. The research budget is the first rule the runtime enforces by itself. Two more such rules are waiting on your decisions.

## Gate 2: the capsule that could not start

The Gate 2 probe replays the self-development episode that passed on September twenty-ninth. It got through five of its sixteen steps, then stalled. Every attempt to run code in the capsule failed, and the engineering desk kept retrying, about four hundred times in twenty minutes. It had no limit on attempts, and its own rules would not let it stop without a result it could only produce inside the capsule.

The error said only that the worker had hung up. The real message was being captured and thrown away, so I made it visible first. The next run named the cause, and I misread it at first. It said the kernel lacked Landlock, the filesystem sandbox. In fact the kernel has it. The capsule's broker locks itself down with a list of allowed system calls before it starts the worker. The worker inherits that list, and the list did not allow the calls the worker needs to lock itself down further. Fixing the Landlock calls exposed the same gap for the seccomp call one step later. Both are now allowed for the broker only, and the code the model writes still cannot use them. The fourth run's capsule executed cells normally.

The worker is now hardened by design. Engineering runs also have a limit now, two hundred model calls or an hour, and on staging it stopped a dead run at exactly two hundred.

Along the way, two deploys failed. vmctl had not finished reconnecting to the running computers inside the thirty seconds the deploy script allowed. The script now waits up to three minutes. The next deploy needed forty-one seconds and passed. Your own computer reconnected cleanly both times.

## What I got wrong

I guessed at the clock while writing problem documents, and several entries carried times up to twenty minutes in the future. I corrected them from commit times and now read the clock every time. I also wrote two causes too early: the research wait first, and then the kernel. Each time, reading the trace corrected me before any code was written. That is the problem-first rule doing its job.

## What I would do next

The one thing to remember: the substrate is now honest about why things fail, and the remaining Texture failures are decisions, not mysteries.

If I were holding the phone, I would rotate the SerpAPI key first. Then I would decide two things. For Cancel, I lean toward letting a revise after a cancel or a finished document start a fresh episode on the same document, which fixes both cases with one rule. For research, I lean toward an assignment counting as finished when its run reports and goes idle. Next, I would trace why the research sources in run seven never reached the turn that wanted to cite them. Then I would let the Gate 2 probe run on to approval, apply and restore.

## Names and receipts

- Texture suite receipts: docs/evidence/texture-acceptance-2026-10-09T19-19-29-623Z.json (run 4), …T19-50-30-737Z.json (run 5), …T20-23-25-707Z.json (run 6), …T22-03-56-155Z.json (run 7). Run table: docs/texture-acceptance-suite.md.
- M11 reruns: docs/evidence/m11-rerun-2026-10-09T20-30-55Z.json, …T21-32-17Z.json, …T21-48-21Z.json (5/16 each); rerun 4 in progress on computer-beb952a2.
- Fixes: 4f9331cf (reopen on revise), 97970cda (search plane), b445fd21 (gateway ops routes), 19b7ef48 (report packet contract, internal/coagentpacket), dc3e86b6 (research budget), cb138a59 (worker stderr), 7591ca02 (engineering budget), 7922bd56 (deploy vmctl wait), e3225820 and 733bec77 (broker seccomp allows Landlock and seccomp).
- Problem docs (docs/problems/): search-plane-cooldown-on-caller-cancel, research-report-packets-rejected-by-schema, texture-research-loop-never-idles, capsule-session-worker-dies-at-start, deploy-switch-dbus-reload-and-vmctl-health-window, texture-research-sources-never-reach-the-citing-turn, texture-cancel-and-research-convergence, clustering-texture-obligation-closure (all 2026-10-09).
- Open decisions: texture-terminal-trajectory-revise (B2 leaning), texture-research-assignment-finish (A2 recommended). Residuals: search-rate-limit-cooldown-scale (Brave out 24 h after one 429), Serper credits, the dbus-broker reload stall during host deploys.
- Staging at 733bec77; owner computer computer-03335285 healthy throughout.
