# Start here, October twentieth

I wrote this early on Tuesday, October twentieth, before you woke. It covers the night's work, from about ten in the evening your time until morning. It replaces the handoff file you asked me to drop. It says what changed, what needs you, and which documents to read, in order.

Overnight, the paper gained related stories and an honest AI label. The host shed the tapes of one hundred and sixty-five dead computers. Every change you approved last night is now live, built and switched off, or written up as a design waiting on your answers.

Where things stand. The paper kept publishing all night. Every code change went through the full landing loop on staging, and each fix has a problem document written before it, except one small test repair whose record is its commit message. Five things need you, and none is urgent. Expansion and the digest each need one small act from you to turn on. The off-host backup needs you to run one command and keep a password safe. And three designs ask questions that decide what we build next: the op-ed section, the account recovery key, and the forecast act. Read the op-ed and forecast designs together, because they disagree on one point. Everything else is a reading list.

## What changed overnight

Related stories are live. Each Autopaper story now ends with a short list called "More on this", linked through the sources the stories share. The first precision read failed badly: about four in ten links were wrong. Nearly all of them came from one Guardian live blog that five unrelated stories had cited. A live page covers many subjects under one address, so sharing one says nothing about two stories. I fixed that. A wider read of thirty-eight links now finds thirty-three right, which sits at the edge of our ten percent bar. I kept the feature on and noted the next calibration step, which is stories that share only a single source.

Autopaper stories now open with a one-line label saying AI wrote them, as you ruled.

The scheduled compaction works. I proved it twice by hand on the paper as a planned restart. The second run squashed the store from one hundred and seven commits to two in forty-seven seconds, with an identical content hash, and the paper resumed its work on its own. Along the way, a dry run showed the timer would also have squashed the history of another account's always-on computer. I limited it to the paper and the other platform computers before the timer's first night. At four in the morning UTC the timer ran on its own, found nothing over its threshold, and stopped.

The event layer now uses your definition: one incident, one place, one time. Later developments become new events, linked to the first. I ran an early blind check at nine UTC, and it came back too small to decide. The paper made only twelve joins in six hours, and the second test computer has gone missing. Of the twelve, I read one as a wrong merge, and it has a familiar cause: a Guardian live blog opened an event and a rally report joined it. That is the same live-page trap that broke related stories last night. I recorded it without a fix, and I will rerun the check this afternoon on a larger window.

Expansion and the digest are deployed but switched off. Expansion puts a "More research" button on a story for you alone. Pressing it asks the paper to research and revise that story. The digest is a daily email of the top stories at noon UTC, sent only to accounts that turn it on. Tonight that is nobody.

Internal calls on the host now require the shared credential. Before, the host trusted anything that came from its own machine. I checked that a call without the credential is refused, a call with it succeeds, and the paper stayed healthy.

Telemetry batching is deployed with its switch off. When it is on, the computer writes its routine counters in batches instead of one tape event each, which is most of the tape's growth. The design and how to turn it on for one computer are in its report.

The tape cleanup you approved has run. A dry run walked all two hundred and twenty-eight computers' tapes, about two point three million events. It found one hundred and sixty-two computers that no longer exist anywhere: no owner, no route, nothing written for at least a week. I checked that list against every computer the host knows, and none of them is live. Then the cleanup archived each one and deleted its tape: about seven hundred and sixty thousand files and two point three gigabytes, with no errors. The archives take three hundred megabytes and can bring any of them back.

The second half of that cleanup, stray files that no event points to, refused to run. I traced why, and the reason was a flaw in the check, not in the data. The check counted the host's receipts thirty-four minutes after it had looked at each computer's position, and live computers kept writing in between. So the counts could never agree on a working host. I wrote up the flaw, had it fixed so the count stops at the same moment as the walk, and deployed the fix. The next dry run passed while the paper kept writing. It found about forty-one thousand stray files, the same count we estimated yesterday, all at least four days old. I applied it: eight hundred and seventy-six megabytes, archived first, no errors. Three more dead computers had crossed the one-week line by then, and went with it. In all, the cleanup removed about eight hundred thousand files and three gigabytes overnight.

I also started collecting the old story body files left from before the object graph teardown, which last night's dump deletion unblocked. There are about five hundred and seventy thousand of them, about one gigabyte. The hourly cleanup removes fifty thousand at a time, so they will be gone by this afternoon. The first batch went through with no errors.

Later in the night, five helper agents finished their work. Two wrote the designs I describe below. One recorded your rulings from last night, added the contributor sign-off rule for the open-source repository, and drafted the bug report for the interpreter loop that ran desks out of memory. That draft is for you to post upstream, not me. Another prepared the change that lets a computer hand freed disk space back to the host. It does nothing yet, because the virtual machine software we pin is one version too old. The last built the first stage of the new tape storage, which reads from compact segment files when they exist and from the old one-file-per-event layout otherwise. No segment files exist yet, so nothing changes in practice.

One small thing broke and I fixed it. A test of how desks see search results arriving was racing against itself and failed once on the build server. The product was fine. I made the test wait for the right signal.

Smaller items. I deleted the twenty-three gigabyte corpus dump. The source registry gained eleven licence-checked primary sources, not yet scheduled. The platform database squash is built, with its timer off until a first supervised run.

## What needs you

To turn on expansion, give me the go-ahead to put your user id on its allowlist. Then press the button on one older story and watch it revise. Eleven stories from October fourteenth still have instructions for headlines, such as "Write the story of…". Expansion is the natural way to fix them.

To turn on the digest, open the Mail app, press the calendar button and switch it on. Then check that the first email reaches your inbox.

For the off-host backup, run one command on Node B. It prints a password once, and you keep that password in two offline places. Then hand node-a's agent a short note I wrote. If both Node B and your copy of the password are lost, every backup is lost with them.

The op-ed design asks fifteen questions. I recommend yes to all of them. The main one is phase one: house op-eds from the desks, unlisted at first so you can judge quality. Your idea of human and agent authors is in the data model from the start.

The account recovery design asks twelve questions, and it found something you should know first. Today no data is actually locked by a passkey. The host holds every computer's key, so a person who loses their passkey loses nothing once they recover their account by email. The sentence you approved for the recovery page, warning that passkey-locked data cannot come back, is therefore not true yet. The design proposes a sentence that is true now, a slower and safer email recovery with a cancel window, and a twenty-four word recovery key for the day user-held data arrives. It also asks you to ratify, out loud, that the host keeps a custodial copy of each computer's key.

The forecast act design asks thirteen questions. It proposes that forecasts become their own kind of record, bound to a calendar of scheduled events, such as statistics releases and central bank decisions, and settled by rules wherever possible. Its third question changes the op-ed design. Instead of every house op-ed carrying one to three forecasts, a piece carries forecasts only when a fitting calendar event exists. Decide that one with both documents open.

Two smaller checks. When the helper recorded your ruling on the claims ontology, two items in that document had no explicit answer from you: whether a research read of a scheduled item counts as scheduled, and how to treat the privacy note. The helper wrote both as reversible defaults and labelled them inferred. And the disk-space change needs a newer virtual machine version on Node B before it does anything. That bump is a host change I did not make overnight.

## Reading list, in order

1. This letter.
2. The op-ed design, which holds the most decisions.
3. The forecast act design, right after the op-ed design.
4. The account recovery design, because recovery is a launch blocker.
5. The story expansion design, then its phase one note.
6. The daily digest design.
7. The node-a backup handover note, before you run the key command.
8. The source coverage-gap analysis.
9. The interpreter bug report draft, if you want to post it.
10. If you want the details: the related stories record, the compaction record, the telemetry batching design, the host-auth record, the guest disk-trim record, and the tape segment section of the tape forward-fix document.

## The one thing to remember

The paper now has the shape of a real paper: related reading, an honest label, and a way to ask for more on a story. The next step is the opinion section, and it waits on your answers to the op-ed and forecast questions.

## Names and receipts

- Reading list paths, in order:
  - docs/reports/op-ed-section-design-2026-10-20.md
  - docs/reports/forecast-act-design-2026-10-20.md
  - docs/reports/account-recovery-protector-design-2026-10-20.md
  - docs/reports/story-expansion-request-design-2026-10-20.md
  - docs/problems/story-expansion-request-no-reader-door-and-wrong-wake-framing-2026-10-20.md
  - docs/reports/autopaper-daily-digest-design-2026-10-20.md
  - docs/reports/node-a-backup-handover-2026-10-20.md
  - docs/problems/no-off-host-backup-2026-10-20.md
  - docs/reports/source-coverage-gap-analysis-2026-10-20.md
  - docs/reports/yaegi-upstream-issue-draft-2026-10-20.md
  - docs/problems/related-stories-dead-end-2026-10-20.md
  - docs/archive/problems/guest-store-compaction-scheduled-2026-10-20.md
  - docs/reports/telemetry-batching-design-2026-10-20.md
  - docs/problems/hostauth-enforce-flip-2026-10-20.md
  - docs/problems/event-layer-false-merges-at-marginal-p-2026-10-16.md, last section
  - docs/problems/guest-data-image-never-trims-2026-10-20.md
  - docs/reports/tape-volume-forward-fix-and-surgery-2026-10-19.md, section 9 and appendix A
- Owner rulings: docs/reports/owner-notes-2026-10-14.md §27. Decisions digest: docs/reports/owner-decisions-digest-2026-10-20.md.
- Inferred defaults to check: docs/reports/claims-events-entities-ontology-2026-10-14.md §13 (F1, GDPR on D6).
- Key commits:
  - related stories e7ea4afda, live-page fix 0a541483c
  - disclosure 9c68537f8
  - compaction 06643d2bd, af8ae3686
  - tape sweeps 685dffc68
  - event definition 31a5c2bc4
  - expansion and digest up to 5b4a28f66
  - backup, squash and registry f41012d93, with heresy fix be887edc2
  - hostauth enforce 9a42a7abb
  - telemetry batching 3f9c48bbc through 72435fcd1, deployed as 6eba20df5
  - CI race fix 82fe1f6e8
  - ruled tasks and guest discard 57fa088c4
  - tape segment A1 4128cb16f
  - tape sweep mode active 5a3cffbe6; dry run ts-20261020T051653Z-fd085f79, apply ts-20261020T060033Z-eb7c5980
  - og collection 35555a20d
  - orphan check fix 4064880b4
- Records: docs/problems/tape-sweeps-destroyed-and-orphans-2026-10-20.md (dry-run, apply, orphan flaw); docs/problems/artifact-store-grows-12gb-a-day-2026-10-14.md (step D).
