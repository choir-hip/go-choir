# The data system cutover

Written on Friday, October thirtieth, 2026, in the early afternoon UTC, and updated at about a quarter to five that afternoon. The first part covers about eight hours of work, from roughly six in the morning UTC until the paper published again just before two. The update at the end covers the three hours after that.

The fleet now runs on the new data system, and the paper is publishing on it again.

Here is where things stand. Every computer keeps its state in one SQLite file. The host keeps its control records and the Autopaper object graph in PostgreSQL. Dolt is gone from the release. The paper, your computer and the test computer all booted on the new release, and by two minutes to two the paper was publishing again. Everything you asked me to keep was counted before and after the move, and the counts match, down to a hash of every object identity. The fleet was down for about five hours, and the paper was not publishing for a little over five. Some of the old data is already deleted, and the rest is waiting for one decision step that you already authorized. Three independent reviewers are reading the whole change against the plan right now. What you might do: listen to the open items near the end, and decide whether the deletion of the remaining old data should wait for the review.

## The night: from plan to code

The plan had been reviewed many times, and you told me to build it at agentic speed and let runtime checks be the verifier. So the review step became a single fold. I took the first round of findings from Devin, Sonnet and DeepSeek, folded them into a second version of the frame, and wrote a build sheet. The build sheet split the work by package boundary, so builders would not collide.

Then I built a thin seam for database access myself, and launched six Sonnet builders, each in its own branch. One moved the guest store to SQLite. One moved the guest object graph. Two moved the host's two stores to PostgreSQL. One deleted the old projection machinery, and one wrote the migration tools. By about half past seven, all six were merged into one integration branch, and the whole tree built. Two more builders then fixed the test suites and the host leftovers.

The lesson from the night is a good one. Splitting by package boundary worked. Six builders ran in parallel, and the merges were mostly mechanical.

## The rehearsals

Before touching production, I rehearsed both migrations on copies.

The test computer's store migrated in about fifty seconds. Six hundred megabytes of Dolt became sixty-three megabytes of SQLite, and every table matched on count and content hash.

The host rehearsal went less well. Copying from the live stores made the paper unhealthy for about a minute, so I stopped it and made a rule. The largest table, the append receipts, about six million rows, would only be copied at the cutover, when the paper was stopped. That rule protected the paper. It also meant that table was never rehearsed in full, and that gap mattered later.

A rehearsal on a copy of the paper's own disk showed a second problem. The paper's store was thirty-one gigabytes of Dolt. Migrating it at boot would take longer than the half hour the host waits for a computer to come up. A boot that misses that wait deletes its partial work and tries again, so the paper would have looped. I changed the plan. The paper would be stopped first and migrated offline from the host, with nothing else running. I also raised the boot wait to ninety minutes for the cutover, so larger computers could migrate at first boot without looping.

Continuous integration found one real defect during this stretch. A memory gauge treated the new state file as a directory and reported zero bytes. I fixed it before the cutover.

## The cutover, and the table that would not copy

At twenty-four minutes to nine, I stopped the paper cleanly, then the services that write to the old stores. I took a snapshot of everything you asked me to keep. It took thirty-eight seconds and found nothing missing.

The paper's offline migration ran for about an hour and a half. It moved eleven and a half million rows across twenty-six tables, and every table matched.

The host copy was the hard part. The first attempt died in a few minutes. A health probe from the old system saw the busy database miss three deadlines and restarted it, killing the copy. I stopped the probe and started again.

The second attempt hit the gap from the rehearsal. Reading the receipts table in one long pass through the Dolt server stalled at two point seven five million rows. It recovered after five minutes, then stopped for good at five and a half million. Dolt was spinning one core and doing no input or output. Nothing outside it was wrong.

I then tried reading in pages, a fresh short query for each fifty thousand rows. My first version of the page query made Dolt scan the rest of the table every time, so it returned one page and stalled. My second version worked, but it slowed from about thirteen hundred rows a second to about one hundred and forty. At that rate the rest would have taken about nine hours.

That was my third iteration on one mechanism, and I stopped tuning. You checked in around then, and I told you the plan. Instead of asking the Dolt server for rows, I stopped both servers and read their files directly, with the same embedded engine that had just moved the paper's eleven and a half million rows without a stall. I built that version on the agents machine in a few minutes. It copied everything in ten and a half minutes. Control came to thirty-three tables and six point two million rows, including all six point one seven million receipts. The graph came to seven tables and one point one eight million rows. Every count and every content hash matched.

Then I took the second snapshot of what you keep. Ninety-five of ninety-six lines matched at once. The last one, a hash over every object identity in the graph, differed. I dumped both lists and compared them. Every difference was the same thing: the old client prints an empty value as two quote marks, and the new one prints nothing. With that normalized, the lists are identical.

The lesson here is the one I most want to keep. The rehearsal could not cover what it skipped. The one table the rehearsal left out caused the morning's longest delay. A smaller lesson is that three tuning rounds was the right point to change approach, and the change was to reuse a path that was already proven.

## Landing the release

I squashed the work into one commit and pushed it to main.

The first run of continuous integration failed on one test. That test looked for the second of two passes in a log by splitting the lines in half. Its progress lines are timed, so under the slower race detector the first pass wrote more of them, and its own last line crossed the midpoint. That was a defect in the test, not the product. I fixed it and pushed again.

The second run passed, but the deploy stopped at its disk check. Deploys require sixty gibibytes free, and there were about forty. The paper's new thirty-gigabyte file was sitting beside its old Dolt store. Your condition for deleting old data was met, because both the copy check and the keep check were green. So I deleted the most thoroughly checked pieces first: the old graph store, eighteen gigabytes, and the rehearsal scratch databases. I kept the old control store and the paper's old store. That freed enough room, and the deploy went out at half past one.

That deletion has a cost you should know. Without the old graph store, the platform can no longer roll back to the old release. From there on, problems get fixed forward, and the last full copy of the old data is the off-host backup from the morning of October twenty-ninth.

## The running system

The paper booted on its new file, found it already migrated, and came up healthy. The restart rules behaved as written. One hundred and forty-two source checks that had been in flight at the stop were closed, not replayed, and every source schedule stayed active. The first scheduled checks came due about fifteen minutes after boot. By two minutes to two, the paper had added three publications, eleven versions and nearly a thousand objects. The story gate was still refusing stories that cite only one outlet.

Your computer migrated its own store at first boot in about four minutes and came up ready.

The test computer was held back by something older than the cutover. It has a self-development rollback that has been stuck since yesterday evening, because its apply attempts are spent. The host never restarts a computer in the middle of self-development, so it kept deferring the update. The test computer is the one we are allowed to break, so I forced the update. It migrated in seven seconds. Then I killed it hard with kill dash nine. It came back, reopened its store without migrating again, and closed its open work instead of resuming it.

I took the keep snapshot once more on the live system, and all ninety-six lines matched.

## What is still open

Several things are not yet proven.

Mail has not been seen arriving on the new system. The last inbound message came in at about one in the morning, before the cutover. Your stored mail came through intact. Only a new arrival will prove the path.

Two drills have not run on the real system. One is that a write acknowledged just before a crash is never lost. The other is that a writer with a stale epoch is refused. Neither has a harness yet. Hibernate and wake on the test computer are blocked by its stuck rollback, which needs an owner action to clear.

The paper validates a set of derived keys at every boot. That takes about an hour and briefly stalled its store once. The old release did the same, so it is not new, but it is worth fixing by remembering what was already validated.

The paper's new file is thirty gigabytes, about the size of the Dolt it replaced. The test computer shrank ten times. I do not yet know why the paper did not, and it is worth looking at.

Three reviewers, Devin, DeepSeek and Sonnet, are reviewing the whole change against the plan and against this runtime record. Each is splitting the work across its own subagents.

## The one thing to remember

The runtime checks did the verifying, as you intended. They caught the memory gauge defect, the test defect, the stalled copy and the empty-value artifact, and none of those needed a reviewer to find. The weak spot was what the checks never touched.

If I were holding the phone, I would let the review land, then delete the remaining old data, which frees about ninety gigabytes on a nearly full disk. After that come the performance benchmarks and the scoring of our precommitments, which you asked for next.

## Update: the afternoon

The old data is gone, the review's findings are fixed and deployed, and the next design is written and under review.

Here is where things stand at about a quarter to five. The paper kept publishing all afternoon: twenty-eight new publications and ninety-nine new versions in the last three hours. The review is done. Every finding it called real is either fixed and deployed or written down as owed work. The old Dolt data is deleted, on the host and inside the computers, after a fresh copy of the host databases was taken. Your computer took its first snapshot. The paper cannot yet, because its store is too big for the way snapshots copy today, and a builder is changing that. The test computer is current again, and the drills it owes are running on it now. You made four decisions this afternoon, and they are recorded. What you might do: nothing tonight. Tomorrow, read the design changes when the review panel finishes, and decide when the sealed data gets a copy off this machine.

### The review

Devin, DeepSeek and Sonnet read the whole cutover against the plan. Their verdict was that the system is fit to serve, but not yet fit to lose data over, because nothing in production had ever taken a snapshot.

Two findings were urgent, and all three reviewers found the first. The counter that numbers the key escrow audit log had restarted at one, while the log already held about four thousand entries. The next request to unlock a computer's key would have failed. I fixed it in the live database within minutes. The second was in the same place. An operator unlock marked the request as done before it wrote the audit entry, so a failed write could leave an unlock with no record. Both now happen in one transaction, and the key is opened only after the record commits.

I fixed the rest over the afternoon, each one checked on staging. Snapshots now have a producer. A check meant to notice a lost disk had always answered no, and now it looks at the right file. The checkpoint fingerprint no longer pauses the computer's writes while it reads the whole store. A database conflict on an event write is retried on the host instead of reaching the computer as a permanent refusal. The host stops asking a computer that keeps refusing an update every minute forever, and backs off to once an hour. The script that counts what you keep now runs as written. And about twelve hundred lines of dead code from the old projection system are deleted.

### A hole found while designing

While reading the graph code for the redesign, I found a security hole that had nothing to do with the cutover. A computer could write public routes and publications through the host's generic object routes. In principle, a guest could have changed what a public address shows. I checked every route in production. All three thousand four hundred and eighty-eight were written by the host, and none pointed across owners, so there is no sign it was ever used. The fix closes the generic routes to guests entirely, at the host and at the proxy. It has been live since a quarter to three, and nothing legitimate was refused afterwards.

### Benchmarks, and why the paper did not shrink

You saw that the benchmarks improved everywhere except the paper's store. The store is thirty point seven gigabytes, and the reason is simple: nothing in it has ever been retired. Not one object of any kind is marked deleted or superseded. Almost five gigabytes is a single kind, the event records, about two and a half million of them. Another five million rows index the computer's own events. Retention was in the plan but never built. So the precommitment that predicted the paper's size is falsified. The scoring round for the precommitments is still owed. I have the numbers, and it comes next after tonight's builds.

### The redesign

You asked how the system had been shaped by Dolt, and what we would build now with a new storage stack. I wrote that up as a redesign of both graphs and the bridge between them. It keeps the logical model. It makes data types into package data that an app installs, so the engineering desk can ship a new app with new data types without a platform release. It stores each class of data in its own shape. It makes the state fingerprint logical, so adding an index never breaks a restore. It carries the tape as batched, signed segments. And it makes the Autopaper graph a store of immutable, signed versions. The route fix was its first concrete result.

Then you asked the question that mattered most. Should a snapshot be the host copying the computer's disk, or the computer streaming out its own sealed copy? I chose the computer's own sealed copy. The host never sees plaintext. The host never has to open a file a guest controls. And the copy carries state, not the computer's identity. Your later point made the choice decisive. You plan to run computers on your Mac and on enterprise hosts, under whichever hypervisor wins the bake-off. A host copy of one hypervisor's disk is useless to another. A logical, sealed copy moves anywhere.

A review panel is reading the next phase of the design. Two of three seats have reported. Both agree with the snapshot choice. Both say to split the next phase: the fingerprint, the package format and enforced writers first, the new storage tables later. They disagree on how to build the fingerprint. One reviewer found that nothing in the store itself makes a precommitment record immutable once written. An overwrite would still show on the tape, so it would be visible, not silent. But the store should refuse it, and the next phase will make it.

### The old data, deleted

With the blocking findings fixed, I took the cutover's last step. First I made a fresh compressed copy of both host databases, so the host keeps a current copy instead of a stale Dolt one. Then I deleted the old host control store, fifty-six gigabytes. Free space on the host went from fifty-seven gigabytes to a hundred and nine. Inside the computers, a boot after a verified migration now deletes the old Dolt database and keeps the files the new store uses. The paper did that at eleven minutes to four.

That change failed its first deploy on a gate I had forgotten. A change to the guest's security surface must either raise a security epoch or carry a reviewed statement that it has no security effect. My commits carried neither, and I could not honestly claim a review that never happened. So I raised the epoch. That is the documented route, and its only effect is that a hibernated computer boots fresh instead of resuming an older release.

### Snapshots

The snapshot producer went live at ten to four. Your computer took its first snapshot four minutes later: two gigabytes, sealed, in forty-four seconds, and recorded on its tape.

The paper is refused. Today's method makes a full temporary copy of the store, and I capped that copy at eight gigabytes to protect the host's disk. The paper's store is thirty. My own design said the snapshot should stream instead of copying. The builder did not follow that, and I should have caught it before pushing. A new builder is now making the snapshot stream straight from the store in page-sized chunks. The sealing is deterministic, so an unchanged range of pages seals to the same chunk, and later snapshots upload only what changed.

Until then, the paper's only safety net is a copy of its disk on the same machine, labelled as not a snapshot. You decided to accept that risk for tonight.

### The paper's slow boot check

The paper re-checks a set of derived index rows at every boot. I measured it at forty-seven rows a second. Two hours after boot, it was still on the third of four families. I first thought the cause was waiting for the paper's single write connection, so I moved the comparison onto the read connections. That helped, to eighty-one rows a second, but my diagnosis was only partly right. The rest of the cost is the main table's layout. The timestamp the check compares is stored after each object's body, so every row it checks reads the whole body. Changing the layout changes the schema, and today's fingerprint includes the schema, so the layout fix waits for the redesign, which deletes this check entirely. It no longer delays the paper's writes.

### The test computer

The test computer had been stuck behind a held rollback since yesterday, and you gave the go-ahead to do whatever it takes. The cleanest path was an operator restart. I had planned a code fix as well, but the boot log showed that an existing rule already handled it. A settlement that fails on two crash boots is closed as degraded. It stays visible for you, and it no longer blocks updates. The test computer is on the current release and idle. The drills it owes are running on it now: a hard kill under write load with no acknowledged write lost, a stale writer refused, hibernate and wake, and its peak memory compared with the paper's.

### Your decisions, and the words we untangled

You made four decisions, and they are in the ledger.

First, the event records split by kind. The ones that acceptance and the Texture verifier read stay. Trace and progress records leave the working store. They do not leave the tape. Every write already goes onto the tape, sealed with the computer's key, so trimming the working copy deletes nothing of the record, as long as the trim is itself recorded on the tape. Second, the test computer may be reset as needed. Third, the paper runs without a backup tonight. Fourth, the tape must become immutable and tamper-proof, and the data system must sync a computer between the host, your Mac, and later enterprise hosts and workstations.

We also untangled the words. The tape is the record. The store is the working copy computed from it. A snapshot is a sealed copy of the store at one point, and its only job is speed. A checkpoint is a published fingerprint of the state at one point, so a restore can prove it landed in the right place. Undoing a bad self-development is a rebuild aimed at an earlier checkpoint. And none of these is a backup yet, because they all live on one machine.

The sync plan changes the next phases. The signed tape becomes a replication protocol, with one writer at a time and a handoff between places. The computer's key travels with the computer, so a host keeps only ciphertext. And every replica is a party that would notice a rewritten history, which is what turns tamper-evident into tamper-proof, together with signatures and anchoring of the tape somewhere outside our control.

### Still open

Mail has still not been seen arriving on the new system. The newest inbound message is from one in the morning, before the cutover. The precommitment scoring round is owed. The paper's snapshot waits for the streaming builder. An off-host copy of the sealed data is the next durability step. And the design doc needs your points written in: computer signatures, outside anchoring, and sync with one writer at a time.

### The one thing to remember from the afternoon

Two of my calls this afternoon were wrong in ways only the running system could show. The snapshot cap left out exactly the computer that most needs a snapshot. And my diagnosis of the slow boot check was only half right. Both were caught within the hour by reading what the system actually did.

If I were holding the phone, I would let tonight's builders land, watch the paper record its first snapshot, and then make the first off-host copy of the sealed data. That step turns tonight's accepted risk into a closed one.

## Names and receipts

- Goal file: docs/definitions/choir-data-system-build-2026-10-30.md. Ledger: docs/definitions/choir-data-system-build-2026-10-30.ledger.md.
- Build sheet: docs/design/data-system-build-sheet-2026-10-30.md. Frame: docs/design/choir-data-system-frame-2026-10-30.md.
- Integration branch: cutover/data-system (PR 108, draft, CI only). Follow-up branch for guest Dolt deletion: cutover/guest-dolt-delete at 159b0f999.
- Squashed cutover on main: 871ab6531. Test fix: 16e6d80d7 (deployed 13:29:52 UTC, CI run 41059427856, attempt 2). Ledger entry: 635f512eb.
- Fixes during the cutover: guestmem file root 550531ff2; rowcopy keyset paging 125aa70bc and follow-ups; hostcopy embedded-Dolt source 200af7867.
- Host copy report: /var/lib/go-choir/migration-reports/host-c5.json on Node B. Earlier attempts: host-c5-attempt1 to attempt4.
- Paper offline migration log: /var/lib/go-choir-rehearsal/reports/paper-migrate-c5.log. Its report: state.db.migration.json inside the paper's image.
- Keep-set snapshots: /root/cutover/keepset-old, keepset-new and keepset-post on Node B.
- Test computer's held operation: rollback-4dca8039-97fe-4615-a048-36f1662efb66.
- Review panel output: the session scratchpad, review/run1.
- Afternoon, design and problem records: docs/design/object-graph-redesign-2026-10-30.md (9adadd947, amended 03f11f5ec); docs/problems/guest-can-write-public-routes-2026-10-30.md.
- Afternoon fixes on main: route fix 3a1232242 (deployed 14:42); escrow reveal 7812e5571 inside 03f11f5ec (deployed 15:01:47); escrow sequence set live at 14:13; guest Dolt deletion 337c659c2 and zero-byte guard 63b41761a; security epoch 4 to 5, 2ae10c421; conflict retry 6fce4951d; checkpoint witness on the read pool c8814dfe1; snapshot producer e2be9bdad (deployed 15:50:11); validator on the read pool f82f9ad2e (deployed 16:05:08); refresh backoff 5f997b071 (deployed 16:19:51); keep-set script 3c1d0767a; dead projection jobs 423b8bfbf; owner rulings in the ledger 28d2d2e3f.
- Host database copy before deleting Store A: /var/lib/go-choir/backups/pg-2026-10-30 on Node B. Paper's interim disk copy: /var/lib/go-choir/backups/paper-interim.
- Your computer's first snapshot: event 01a18d84-bd14-707e-a9b2-d3544f6af18a at head 596106.
- Phase-one review panel output: the session scratchpad, panel-phase1/run1.


## Merged from docs/reports/data-architecture-state-and-paths-2026-10-28.md (2026-10-28)

**The data architecture, state and paths.**
I wrote this on Wednesday, October twenty-eighth, in the early afternoon UTC. It covers a three-round review panel that ran for about an hour and a half today, with Devin and DeepSeek on the panel. It also covers where the data architecture stands after this week.

(Full text in git history at the path above.)

