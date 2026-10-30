# The data system cutover

Written on Friday, October thirtieth, 2026, in the early afternoon UTC. It covers about eight hours of work that same day, from roughly six in the morning UTC, after you said good night, until the paper published again just before two in the afternoon.

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
