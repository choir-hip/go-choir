# Decisions waiting on you

Status 2026-10-30: this decision list is from 2026-10-20 and part of it has since been answered; do not read it as the open-decision list. Current work state is in [ACTIVE](../ACTIVE.md).

I wrote this early on October twentieth, while you are sick, from a sweep of every document written since October thirteenth. It replaces the open-question lists in older letters and status files. Several of those still list things you already decided.

There are about twenty decisions still waiting on you, and most of them take one word. Answer by number, for example "one yes, two yes, nine later". Anything you skip stays as it is. Nothing here is urgent tonight. The paper keeps publishing whatever you decide.

I already took or ruled out a few things under your standing rules. Related stories phase one is being built tonight on the design you approved. The scheduled compaction now touches only the paper and other platform computers. Tonight's dry run showed it would also have squashed the history of an account's always-on computer, so I fenced that off. That computer has a forty-three gigabyte image. It is compacted only if you ask.

## Quick yes or no

One. Delete the old corpus dump on Node B, about twenty-three gigabytes, and the roughly half a million old body files from before the October ninth teardown? I recommend yes. You chose to start from zero, and nothing reads them. The one caveat is that the design notes call the dump the only archive of the old data.

Two. Require a sign-off line on outside contributions, the lightweight kind, instead of a signed contributor agreement, and write the trademark policy later? I recommend yes. The licence is already Apache. Nothing breaks until the first outside contribution.

Three. May agents flip the security switches from observe to enforce after a clean day, and roll out the stricter VM jail, first on one test computer and then on all of them? I recommend yes. You already ruled this for host authentication. The owner-act check and the jail are still in log-only mode.

Four. May self-development run autonomously on test computers only, while the paper and your computer stay propose-only? And may the tracer approve its own changes on computers it creates? I recommend yes to both. It fits your autonomous-first ruling, and the loop proof otherwise waits on you for every drill.

Five. Should I draft the bug report for the interpreter loop bug that crashed the paper on October sixteenth, for you to post upstream? I recommend yes. Our workaround holds either way.

Six. Run a coverage-gap analysis of the paper's twenty-eight sources before adding more? I recommend yes. It is cheap and needs no deploy.

Seven. For the weekly squash of the platform database, which you already ruled, should agents build it as a small tested command rather than run it once by hand? I recommend the tested command. Tonight's paper compaction gives it a proven method. That method squashes history, checks that the content hash is identical, and runs under a memory cap.

## Needs a minute

Eight. What counts as one event? The event layer still merges different incidents too often: seven and nine and a half percent against a five percent limit. I recommend defining an event as one incident in one place at one time, with later developments as new linked events. Until this is settled, events stay in shadow and cannot start stories on their own.

Nine. Which key signs the record that proves when a capture first arrived? I recommend a new dedicated key rather than reusing the platform key. It is your security boundary, which is why I am not taking it myself. Only forecast scoring waits on it.

Ten. Old relay-style sentences that long-running stories carried over. New writing is grounded, but about half the carried sentences still credit outlets instead of sources. I recommend a one-time rewrite pass. It costs about sixty Texture turns.

Eleven. Start the primary-source registry: ministries, courts, election offices, statistics offices, and originating wires, with licences checked first? I recommend yes. Only three to ten percent of cited sources are primary today.

Twelve. Money. Several search providers and the OpenCode account are out of credit. I recommend no top-ups now. Stay on Keenable's free tier and revisit after it renews on November fifth.

Thirteen. Account recovery restores sign-in but not data that was encrypted under lost passkeys. I recommend saying that plainly in the recovery screen now, and designing a proper recovery key before outside users arrive.

Fourteen. European law has required, since August, that AI-written public-interest text be labelled. I found no label on the paper. I recommend a one-line disclosure on every published story, and a lawyer's read later.

Fifteen. The claims layer has thirteen design answers. Only the one about whom we track is ratified. I recommend accepting the rest as written. One example is that a public claim counts as the speaker's commitment, recorded by Choir. Another is that people are identified through Wikidata.

Sixteen. The guest-update design has about ten open defaults. I recommend accepting them all. Among them: updates roll to the test computer, then the paper, then your computer. Your own activity pauses an update. Only you can restart a halted rollout.

## Big direction calls

Seventeen. Off-host backups. Everything durable lives on Node B, and losing it loses history that exists nowhere else. This is the biggest risk I know of. I recommend encrypted backups to node-a as a stopgap. The key would stay on Node B and with your offline copy, and node-a would only ever hold encrypted data. Rented storage would be the real fix. The tape cleanup you approved also needs this as its archive target before any old layout is deleted.

Eighteen. May run telemetry, which is more than half of all events, be written in small batches instead of one acknowledged write each? That would cut the file count a lot. The cost is that a crash could lose up to about a second of telemetry. I recommend letting a review panel decide the details, with your yes on the principle.

Nineteen. The learning loop. Most of a story's hypotheses today are "our research desk will confirm this", which measures our own desks rather than the world. I recommend adding a separate forecast on scheduled, checkable events. Settle those by rules first. Let a desk see only its aggregate record, and leave the token-budget learner out for now.

Twenty. Red-team scope. I recommend forks only until the pre-production security slot you already scheduled.

## What can wait

The business model, the email desk's twelve questions, Autoradio's sixteen engine questions, and renaming the old World Wire names in code can all wait. You said Autoradio follows the architecture work. Telegram and the Cofacts dataset wait on you setting up their accounts when you have an hour.

## The one thing to remember

If you only answer one item, make it seventeen, the off-host backup. Every other item can be undone. Losing Node B cannot.

## Names and receipts

- Full digest with sources: docs/reports/owner-decisions-digest-2026-10-20.md (this letter).
- Compaction selection fence: problem d3e10d060, fix 06643d2bd.
- Related stories design: docs/reports/related-stories-graph-design-2026-10-19.md.
- Sources: owner-notes-2026-10-14.md (your rulings through §26), choir-last-three-days-2026-10-17.md (open questions), event-layer-false-merges-at-marginal-p-2026-10-16.md (item eight), capture-first-creation-witness-missing-2026-10-15.md (nine), articles-relay-outlets-instead-of-grounding-them-2026-10-19.md (ten and eleven), account-recovery-token-discarded-2026-10-14.md (thirteen), research/autopaper-media-and-rights-2026-10-14 (fourteen), claims-events-entities-ontology-2026-10-14.md §13 (fifteen), lu-guest-updates-design-2026-10-14.md §6 (sixteen), tape-volume-forward-fix-and-surgery-2026-10-19.md §6 (seventeen and eighteen), precommitment-records.md §8.3 (nineteen).
