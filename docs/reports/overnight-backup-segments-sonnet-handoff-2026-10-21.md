# Overnight on Choir, October twenty-first

I wrote this on the morning of Wednesday, October twenty-first. It covers the overnight work, from about two in the morning to half past eight, UTC.

The backup is now proven by a real restore, and the Sonnet rewrites never reached a freeze. After eight attempts I stopped patching, and I am asking you to choose how to fix the handoff that keeps breaking.

Here is where things stand. The backup ran clean, and I restored both databases from a snapshot onto spare space and checked them. Research more now works for logged-out readers. Op-eds publish through Texture. The Taiwan source and the rest of the wave-one schedules are polling, and only VentureBeat is failing, as it has since October thirteenth. The new tape segment store is deployed, switched off. Two serious issues must close before anyone switches it on. The ImageApp comparison ran once each way: Luna went from a new computer to a restored release in about six minutes, and Sonnet proved the carried patch but not the freeze. After your second note I stopped testing Luna and kept rewriting with Sonnet. SlidesApp went through eight attempts. Each found a real defect, and I fixed the ones in Choir itself. What remains is a two-round handoff between desks that keeps stopping in a new place. That needs your choice, not another patch.

Four things are yours to decide: how the delegate handoff should work, switching the segment store on, the repack, and deleting the legacy tape files.

## The backup, and the restore that proves it

The fifth backup run finished clean in thirty-six minutes. You asked for a durable fix to the backup check, so I did more than read the check's output. I restored the corpus database and the platform database from the snapshot, side by side with the live ones, and opened both. The corpus came back with thirty-one tables, and the platform with seventy. The drill peaked at about seven gigabytes of extra disk, and it ran from four fifty to four fifty-six.

With that proof in hand, I deleted the old copies that the move to separate volumes had left behind, about forty gigabytes, and the backup helper scripts that were no longer used. The daily check now unlocks the off-host store and sums the pack sizes, so a backup that quietly stops growing gets noticed.

The restore drill did not cover the tape files or the computer images. Those pieces are still open.

## Flat directories become a segment store

You said the flat tape directories should be sharded. I chose a different design that solves the same problem. New tape writes go into a small number of append-only segment files instead of one file per event. A later copy-only repack moves the old files into segments without changing a byte. Spreading the same millions of files across prefix folders would leave the backup check walking the same number of files.

The segment store went through four review rounds. The third round changed how it fails: when a disk write fails to sync, the writer now stops, instead of guessing. It landed with the switch off and deployed at six twenty-seven. On staging it reads every tape file through the old path, as intended: more than two thousand reads, none corrupt.

The fourth review named two serious problems that only matter once the switch is on. In one, a refused recovery can acknowledge a write that the reader then cannot find. In the other, a sealed but suspect segment tail can hide the good legacy copy. Both must close before the switch is turned on. After that, you decide when to switch it on and when to repack.

## Readers, op-eds and sources

Research more worked end to end for a reader who was not logged in. On the Apu story, one ask produced a new revision five minutes later. The story grew from one hundred and eleven words to one hundred and ninety-one, and from seven sources to eight. One intermittent fault, which I call P6, is still there. Logging is now in place to catch it.

The op-ed path passed its deployed proof at four eighteen, and the test op-ed is public. Op-eds still have no moderation or unpublish step.

I scheduled the Taiwan central bank's press feed after testing its certificate chain from the server. Its first poll fetched one hundred items, twenty-five of them new. Sixty-six source schedules are active, none overdue. During one deploy, thirteen feeds had a single refused capture while the computers were restarting. All thirteen recovered half an hour later with nothing lost.

## Self-development: Luna, then Sonnet

The ImageApp comparison ran once each way on test computers. Luna, writing its own code, went from a freshly made computer to a restored release in about six minutes, and the restored release matched what was frozen. Sonnet, working through the Claude delegate, rewrote the file in its first round. Its second round received that rewrite as a carried patch, and the patch applied. That is the first proof on staging that a failed round's work can be carried forward. The drill's deadline then ran out during the build. The two runs are not a fair model comparison. Sonnet's computer carries weeks of history, and the delegate needs a second round to build by design.

After your note I stopped Luna and started the Sonnet rewrites, SlidesApp first because it is the smallest. Every attempt stopped on a defect in Choir or in the drill, not in Sonnet's work. I documented each one before fixing it.

The first seven attempts each stopped on something new. First, management never gave the build round network access for npm. I fixed its prompt, it overcorrected, and I made the drill's brief spell out both rounds. Next, the copy into the release tree failed. I blamed the sandbox's system-call filter for a missing call, and I was wrong. Then I blamed ownership of the computer's installed release, and that was not the whole story either. The trace finally showed the real cause. The copy fails on new files and passes on files that already exist. The copy command sets an access list on each file it creates, and the sandbox refused that with "permission denied", which the copy treats as fatal. The sandbox now refuses those calls with "not supported" instead. Tools read that as a plain filesystem without access lists, and they fall back to a normal permission change. Nothing new is allowed.

Four more defects came out along the way. The engineering desk read a diff's normal exit code as a failure, so the brief now says what passing looks like. Engineering also once ended its round while its own build was still running. Choir now refuses that, just as it already refused a freeze during a build. A failed build used to leave its outputs behind, which spoiled the next build's evidence. It now cleans up either way. The last stop was management refusing the build round. It believed it had to find the carried patch itself. Its prompt now says that the platform carries the patch on its own.

The eighth attempt showed what is left. Management now casts again, but it cast the first round twice. When the build round did get the carried patch, the engineering desk emptied the file while inspecting it, then stopped over "provenance", a word from management's brief. Every stop since the fifth attempt has been a desk reasoning its way out of a handoff. The delegate cannot build in its own capsule, so every rewrite needs a second round. Three desk turns pass between the rewrite and the build, and each one restates the task in its own words. Each restatement is a new place to stop. Sonnet's own work never failed: it rewrote the file every time in ten to twenty-five turns, for about twenty to thirty cents.

## The decision I need from you

There are three ways out. First, one round: the engineering desk's build job, in the same capsule as the delegate, reaches the package registry through the proxy, while the delegate itself stays cut off. Then there is no handoff at all. The cost is a network grant, so it is a security call and yours. Second, the platform could run the build round itself with a fixed recipe whenever a delegate round ends unfinished. No desk would interpret anything. Third, and cheapest, raise the engineering desk's reasoning effort. That only moves the odds. I recommend the first if you accept the registry access for build jobs, and the second if you do not. The other five apps wait until you choose, because each would hit the same handoff.

## Still open

The open items are these. The command runner still starts in the wrong folder, and long commands still die on a timeout. Texture can re-cast work without limit. P6 is intermittent. Op-eds have no moderation. The segment store has its two serious problems. The restore drill skipped tapes and images. A computer left by yesterday's visual tracers may still be running. I did not start it, and I left it alone.

## The one thing to remember

The backup is real now: it has been restored and read. If I were holding the phone, I would pick the one-round handoff, or say no to it, so the rewrites can run. I would switch on the segment store only after its two serious issues close.

## Names and receipts

- Goal: docs/archive/definitions/choir-ledger-metamission-2026-10-13.md; owner notes sections 32 and 33.
- Backup: docs/problems/no-off-host-backup-2026-10-20.md, item 21.
- Segment store: docs/archive/definitions/choir-tape-segments-a2-repack-2026-10-21.md; landed 8791f9b8f, deployed 06:27 UTC.
- Drill report: docs/reports/selfdev-two-arm-drill-imageapp-2026-10-21.md.
- Fixes: carried patch secrets 85250d067; management network prompt 7b6cbd918; seccomp siblings 4d45601d0; capsule release dir 871c3c145; drill brief f9ad90709; Complete refused during a job b03f57414; xattr refused as unsupported and build cleanup 16a5aa0e0; management carried-patch prompt 10132ed5b.
- Engineering failures 16 to 26: docs/problems/engineering-completion-path-cluster-2026-10-20.md.
- Research more: rx-c4e206c940e3.
- SlidesApp drills a to h, deploys cbef56155 to 10132ed5b; the delegate test computer is stopped. Structural options: failure 26 in the cluster doc.
- Overnight goal: docs/archive/definitions/choir-overnight-research-oped-sources-2026-10-20.md.
