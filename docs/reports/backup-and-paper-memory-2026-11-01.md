# Backup and paper memory, November first

I wrote this on Sunday, November first, in the afternoon, and updated it at about half past five UTC. It covers the overnight and morning memory work, about eleven hours, starting a little after five in the morning UTC. The update adds a chapter on capping the paper's cache, which you asked for this afternoon.

The nightly backup now takes about twelve minutes instead of nearly five hours, and it fits in its own memory limit without any raise.

Here is where things stand. The eleven million old loose tape files are gone from Node B. They were copied into tape segments, the copy was checked, and only then were the originals deleted. That freed about twenty-four gigabytes on disk. The backup no longer splits into shards, and the temporary memory raise I made at five thirty is removed. Node B is using about twelve of its thirty-two gigabytes, with almost twenty available. The paper computer still holds about seven and a half gigabytes, almost all of it file cache inside the guest. One decision is yours: whether to cap that cache. I did not add memory anywhere.

## Why the backup was slow

The backup was slow because it walked eleven million tiny files every night. Those were the old loose tape files, one file per event, payload and receipt. Restic had to hold a picture of all of them in memory, which pushed it against its limit, and it thrashed. Last night's run took four hours and forty minutes and almost twenty-seven hours of CPU.

You said that if those old files were hard to back up, we should not keep backing them up. That was the right instinct. The new structure already stores tape in segments, so the loose files were a leftover. First I made sure nothing writes new loose files. Every pin now goes to segments, except one small class of public files that a verifier still opens by path.

## Moving the files out

Next I copied every loose file into segments with the repack tool, then ran a fresh census to prove each one was covered. The first census read about five hundred gigabytes to check thirty gigabytes of segments, because it read records in the wrong order on a compressed disk. I fixed the order, and the copy read only thirty gigabytes.

The delete ran from eleven in the morning to about half past three, at the lowest priority. It removed ten point eight six million files. It kept two hundred and fifty-two that the database still names. It left about thirty-six hundred more by rule, because they are either the public class or have no receipt. The counts matched exactly.

## The new backup

Then I pushed the change that runs the backup as one pass, and ran it once by hand under its normal limit. It finished in eleven minutes and forty seconds, using twenty-four minutes of CPU. It never hit its hard memory limit. Its own processes used a few megabytes. The rest was disk cache from the database dump, which the kernel gives back freely.

That run added forty-five gigabytes to the repository. That is the segments arriving for the first time, and it will not repeat. The repository is at a hundred and thirty-eight gigabytes of its four hundred and fifty.

One side effect from last night: this morning's weekly prune missed its window. It waits for the backup, the backup ran past seven, and node-a's prune window closes at seven. With a twelve-minute backup, next Sunday's prune fits easily.

## The paper computer

On the paper I found two things. First, the guest re-checked its derived keys from scratch at every boot, reading the whole object store each time, for hours. That kept the guest's cache hot and churning. Now each key family stores a mark tied to the store's head, so a later boot can trust the mark and skip the pass. Both reviewers accepted it. The first pass on the new code finished at eleven forty. I have not yet seen a boot that skips the pass, because the paper has not rebooted since.

Second, I tried the cold-cache release on the paper. The guest freed a gigabyte, but the host got back only a sixth of it, because the guest refilled its cache almost at once. I put the paper back to observe-only. The cold-cache release stays active on the test computer.

## Capping the paper's cache

This afternoon you chose the trade: more disk reads, less memory. Here is what I am doing with it.

First, some background on why the paper holds so much. Inside the paper computer, Linux keeps every file it has read in spare memory, as page cache. That is normally free speed. But the host cannot tell cached memory from memory in use, so every gigabyte the guest caches is a gigabyte the host cannot give to another computer. A guest that has read a lot looks full from outside, forever.

Two things were filling the paper's cache. The first was the derived-key check I fixed this morning. The paper restarted at eleven past four, and its boot trusted the stored marks and skipped the check. That fix is now proven. For ten minutes afterward the paper sat at about three gigabytes on the host, not eight.

Then a second reader appeared. Ten minutes after every boot, and once a day after that, the paper retires old telemetry. Each step of that job re-reads a large share of the store, about fourteen and a half gigabytes in four minutes. The paper's host memory went straight back to eight gigabytes. I wrote this up as its own problem. A cheaper query would fix it at the source, but a cap fixes the whole class.

The cap uses a device the paper already has, called a balloon. The host tells the balloon to inflate. The guest has to hand over that much memory, and it gives up cache first. The host then gets those pages back. If the guest ever truly runs short, it can take memory back from the balloon instead of crashing. None of this changes the guest's code.

I built a small controller in vmctl, the host service that runs the computers. You name a computer and a cap, and once a minute it sets that computer's balloon to fit. Dropping the name lets the balloon go. Devin reviewed it twice. The first review found a real bug: after a restart, the controller could have used the wrong memory size for a computer. I fixed that, and the second review accepted it.

## The benchmark

Before deploying the controller, I ran the paper through a ladder of caps by hand, fifteen minutes each. A watchdog would let the balloon go if the guest stopped answering or its cache got too small. It never fired.

With no cap, the paper held eight gigabytes. At a three gigabyte cap, it held three, and it read about six megabytes a minute from disk. At two and a half gigabytes, it held two and a half, with about the same reads. At two gigabytes, it held two, but reads jumped to hundreds of megabytes a minute. That is the start of thrashing. At one and a half, the guest re-read about two gigabytes a minute, mostly from the host's own cache. So the bend sits between two and two and a half gigabytes.

I am setting the paper's cap at three gigabytes. That leaves room for the telemetry job's surges and for the paper's own peak, and it gives about five gigabytes back to Node B. On a thirty-two gigabyte host, that is room for two or three more small computers from the paper alone. The same cap can go on other computers one at a time, after each one has been measured.

## What is left for you

You made the cache call this afternoon, and the cap is going in. What remains is widening it to other computers, and making the telemetry job cheaper so the cap rarely has to push.

Self-development on the b@c test computer is still waiting behind this work. Slides, Files, Calendar and Podcasts remain.

The thing to remember: we didn't make the backup faster by tuning it. We deleted the eleven million files it was struggling with. If I were holding the phone, I would watch the paper hold at three gigabytes for a day, and then let self-development resume.

## Names and receipts

- Goal: docs/definitions/choir-memory-optimization-2026-11-01.md, ledger beside it, checkpoint 53ca87c54.
- Backup unsharded: 06731745d, deployed 15:46Z. Runtime drop-in reverted; unit at MemoryHigh 4G, MemoryMax 6G.
- Proof run: 15:46:57 to 15:58:37Z, snapshot eea3daea, 45,531,568,875 bytes added, memory.max events 0.
- Segment rule 16f7b5d01; derived-key marks c99e5ce3c; census read order 2c2d3515e.
- Copy receipt 4d49c40e; census e0f76416; delete verify 36f8f6bf. Deleted 10,858,631, freed 24,300,741,722 bytes.
- Cap controller: VMCTL_GUEST_MEMORY_CAP_COMPUTERS, internal/vmctl/guest_balloon.go and internal/vmmanager/balloon_target.go (bfa9c4e6d, review fix 9fa570157).
- Ladder: /var/lib/go-choir/taperepack-work/paper-cap-ladder-1101.log and paper-mem-1101.csv on Node B.
- Problems: docs/problems/telemetry-retirement-reads-14gb-per-run-2026-11-01.md, docs/problems/offhost-backup-shard-thrashes-at-4g-2026-11-01.md, docs/problems/paper-busy-cold-freed-cache-refills-2026-11-01.md, docs/problems/derived-key-validation-rereads-og-objects-every-boot-2026-11-01.md.
