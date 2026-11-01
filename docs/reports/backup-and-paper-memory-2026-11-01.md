# Backup and paper memory, November first

I wrote this on Sunday, November first, in the afternoon. It covers the overnight and morning memory work, about eleven hours, starting a little after five in the morning UTC.

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

## What is left for you

The one lever left on the paper is a ceiling on its guest cache. That could be a memory limit inside the guest, or a balloon. Either one trades some re-reads of the state database for a few gigabytes back on the host. That is your call.

Self-development on the b@c test computer is still waiting behind this work. Slides, Files, Calendar and Podcasts remain.

The thing to remember: we didn't make the backup faster by tuning it. We deleted the eleven million files it was struggling with. If I were holding the phone, I would decide on the paper cache ceiling, and then let self-development resume.

## Names and receipts

- Goal: docs/definitions/choir-memory-optimization-2026-11-01.md, ledger beside it, checkpoint 53ca87c54.
- Backup unsharded: 06731745d, deployed 15:46Z. Runtime drop-in reverted; unit at MemoryHigh 4G, MemoryMax 6G.
- Proof run: 15:46:57 to 15:58:37Z, snapshot eea3daea, 45,531,568,875 bytes added, memory.max events 0.
- Segment rule 16f7b5d01; derived-key marks c99e5ce3c; census read order 2c2d3515e.
- Copy receipt 4d49c40e; census e0f76416; delete verify 36f8f6bf. Deleted 10,858,631, freed 24,300,741,722 bytes.
- Problems: docs/problems/offhost-backup-shard-thrashes-at-4g-2026-11-01.md, docs/problems/paper-busy-cold-freed-cache-refills-2026-11-01.md, docs/problems/derived-key-validation-rereads-og-objects-every-boot-2026-11-01.md.
