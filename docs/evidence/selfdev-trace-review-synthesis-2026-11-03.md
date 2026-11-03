# Self-development trace reviews, 2026-11-03: synthesis

Owner, 2026-11-03 ~20:00Z: "we should have sonnet agents that review the agent traces and report back on them. then we can improve the agents affordances ... we can do it retrospectively and live."

Four Sonnet reviewers each read one test arm's whole day of traces, read-only, from the brief at the end. Their reports sit beside this file as selfdev-trace-review-{claude,deepseek,luna,pool}-arm-2026-11-03.md. This file groups their findings by the affordance at fault, ranks them by rounds and minutes lost across arms, and names the fix and its state. A finding one reviewer marked a hypothesis stays a hypothesis here.

## Ranked clusters

1. **The owner's words do not reach the engineer.** Texture restates the revision, management restates the Cast, and scope, shot lists, step order, budgets and "use the delegate" fall out.
   - Seen on pool r2: a one-sentence Cast, after which the engineer froze with twelve identical shots.
   - Seen on pool r4: management indexed OwnerDirections[4], round 3's text, called it "verbatim", and reported it as carried.
   - Seen on deepseek's restore round: Texture reframed it as a publish retry.
   - Seen on claude's Files pass: "start the delegate" became "report the delegate's call count".
   - Fix: the platform appends the operative owner revision, verbatim and labeled, to every engineering prompt it serves, and the OwnerDirections indexing is corrected. Building.
2. **The Claude delegate is under-equipped.**
   - The delegate sees only the brief's objective; gpt-6-luna squeezed it to about 700 characters and dropped the shot list.
   - Its environment strips CHOIR_ variables and it cannot start capsule jobs, so it cannot build, run choir-shot or measure.
   - The overlay's example max_turns of 40 is copied; R12 hit it at 13 of 40 minutes.
   - resume_session cannot work, because each run has its own HOME.
   - "Delegate first" covers only source edits, so screenshot work stays with the engineer.
   - Management twice chose a network mode ("forbidden", then "ecosystem_proxy") that shut the delegate out.
   - Fix: a delegate affordance pass covering all of these. Building.
3. **Proof tooling gaps.**
   - choir-shot has no open-app step. Pool r1 reported a false "not reachable" scope blocker and r3 shot route-stubbed placeholders; pool r4 spent 16 minutes finding the click.
   - A harness written in one round does not carry to the next.
   - A low contrast reading does not name the element; deepseek r3 spent 46 minutes "fixing" a badge outside the package.
   - Capsules have no .git, so a scope or diff proof against the base has no tool.
   - Fixed already: --theme and --contrast (7beec5d5b), and real /computer content in delegate capsules (b495fd61a).
   - Fix: `choir-shot --open <app>`, contrast failures that name the element and its selector, and a `choir-diff` helper against the base. Building.
4. **Early Freeze without the asked evidence.** About nine Luna operations froze after 2 to 5 minutes, saying "no screenshots"; the judge rejects them, but each costs a judge run. Candidate fix: Freeze refuses when the Cast names screenshots and screens/ is empty. Deferred until cluster 2 lands, because a delegate that can take shots removes most cases.
5. **Texture's Wait-after-Write trap.** A cell that does Cast, then Write, then Wait inside panic(err) loses both staged intents, and the trigger is refused as not disposed (pool 17:41Z; the same error appears in 7 Slides cells, at choir.go:1259). Fix: Wait after Write should not error, or the error should not roll back staged intents. Building.
6. **Invisible single build slot.** About 84 minutes of deepseek queueing behind its own earlier assignment, with no desk able to see it. This is a hypothesis: one live engineering slot per computer. Fix: name the queue position in the Cast's status. Open.
7. **Provider and circuit behaviour.**
   - A gateway circuit-open cancels an assignment with no retry: luna lost about 100 minutes.
   - Synthetic DeepSeek ran at a 70 s median per call from 14:26 to 15:50.
   - Open: retry the circuit-open after its cooldown, as a bounded wait like problem 24's.

## Fixed tonight and seen in these traces
Problems 17, 21–28; the lean rebase notice (52e05f16c); the mechanical re-graft (stack_rematerialized), which cost pool no engineering round at two refreshes; choir-shot's autoputer helper (031dd6839).

## Kept
- Luna R12's single parked wait with zero model turns, and its honest summary of three delegate runs.
- The judge rejecting mocks and placeholders every time.
- Lane C's ids line on every judged round.

## Live reviews
From now on, each judged round gets its own Sonnet review, run against the same brief.
