# Guest Isolation Checkpoint

Written late on Sunday, November first, about a day into the guest isolation work. It is a checkpoint: the closing numbers report comes on Monday afternoon, after a full day of the memory meter, and replaces this one.

Every guest on Node B now has its own fence for memory, processor and disk, and a test computer hammering the disk or the processor no longer hurts the paper.

Here is where things stand. Each running computer now sits in its own control group with three limits. Its memory may go one gigabyte past its size before the host pushes back, it can never swap, and it gets its processor count plus one. Disk time is shared by weight, with latency targets, so one guest can no longer starve another. A computer will not start when the host disk is short of space. Each guest is held to eight thousand new connections, and the uplink is shared fairly between guests. Along the way, I found that the paper was rereading gigabytes from disk every time it cleaned out old telemetry, and I fixed that. What remains is a day of quiet data, and the report.

## What changed today

This afternoon the paper moved into the jail, the confined launch every other guest already used. That gave it a control group of its own, which is what every later limit needs. Then I turned on the disk scheduler. My first claim that it worked was wrong: with its automatic settings it measured nothing and did nothing. A drill caught that. With explicit latency targets, a reader next to a noisy neighbour went from one hundred forty megabytes a second to about five hundred, and a lone reader still got its full speed.

In the evening the memory, processor and disk limits went onto every guest. Two reviewers checked that change, Devin and DeepSeek, and both accepted it.

## The rereads

You asked me to make sure we were not doing extra reads, now that we lean on disk instead of memory. The paper's cleanup pass was the worst offender. It removes old event lines and finished run memory, which the sealed tape still holds. To find a few hundred rows, it read every event body in the store, because the fields it filtered on are stored after the body. One pass read about eight and a half gigabytes. My first fix made it worse, about nineteen gigabytes, because it exposed a second scan of the same kind. Four indexes later, the pass reads under half a gigabyte, in twenty four seconds. Between passes the paper now reads about three megabytes a minute.

The lesson is that the measurement after the fix mattered more than the fix. Each round, the meter showed what was still being read.

## The drill on the test computer

I gave the guest a small, host-only drill that makes it a noisy neighbour on purpose, and ran it on the test computer only. While it read, wrote, or spun every processor, the paper stalled for at most a quarter of one percent of the time. A stand-in second guest kept most of its speed under a read storm or a processor storm. A write storm is the one place it still hurts: the neighbour's reads halve, and its slowest reads get about five times slower. The writer pays most of that itself, stalled for forty two percent of the time.

## The numbers so far

These cover about four and a half hours, with seven deploys and so seven reboots inside them. Treat them as a first look. The paper's own memory on the host sits around one point six gigabytes, down from three to seven before this mission. It peaked near eight gigabytes only while building a new index at boot. Your computer sits at about one point three gigabytes. The host never swapped more than a quarter of a gigabyte, and it always had at least fifteen of its thirty one gigabytes free. The disk is flat at about two hundred thirty three gigabytes used, one hundred ninety eight free.

## What is still open

The network limits are live, but I did not attack them, because a flood needs an outside target, and every outside target is someone else. The disk floor refusal is covered by tests, not drilled. Every guest deploy still reboots every guest. One continuous integration test fails now and then on a timing race.

The one thing to remember is that the paper is now a well-behaved neighbour with its own fence, and it reads only what it needs. If I were you, I would let tomorrow's full day of numbers confirm that before starting the self-development load test.

## Names and receipts

Goal file: docs/definitions/choir-guest-isolation-2026-11-01.md, with its ledger beside it. Deployed: 43c070756 for the memory, processor and disk limits plus the disk floor and network limits; efb712d0f, 218e8fd98 and f7a560fb3 for the read fixes; 61772f741 for the drill route and security epoch 30. Drill results on Node B: /var/lib/go-choir/drill-scratch/results-20261101T234609Z.txt. Memory meter: /var/lib/go-choir/growth-meter/mem-2026-11.jsonl, from 2026-11-01T19:32Z. Test computer: computer-59c902c5d46caecc0a180a8557164e72.
